# pobear known debt

Things we know are wrong, missing, or incomplete, that we've deliberately chosen
not to fix at the time they were found. This is not a backlog of features — it's
a record of specific gaps discovered mid-work, so they don't get silently
forgotten or rediscovered from scratch later. Each entry says what's wrong, why
it wasn't fixed on the spot, and what fixing it actually requires.

An entry is removed once it's fixed — this file tracks open debt, not a history
of resolved debt (git history already has that).

---

## `Stager`'s data-key-collision warning never reaches any visible output

**What's wrong:** `JOB_SPEC_REFERENCE.md` §6 says when two stages write the same
`data` key, the engine prints a warning naming the two stages involved, on top of
keeping the last write. `services/core/internal/stage.Stager.Run` already returns
both the merged `data` and a `warnings []string` (locked by
`services/core/internal/stage/stager_warnings_test.go`), but
`services/core/internal/job/workers.Scrape.Run` only forwards `data` into the
`sinks.Item` it builds — `warnings` is dropped on the floor. Nothing between
`Stager` and the CLI (or any other output) currently has a path to surface it.

**Why not fixed now:** first found while scoping the `core-integration-tests`
batch-2 plan (hermetic integration tests for `services/core`) — surfacing
warnings would mean changing `workers.Scrape`'s return shape or `sinks.Item`'s
fields, which is real design work outside that plan's scope. Found again while
scoping CLI golden tests (`cli-tooling-tests`) — same reason, still out of scope
there: a golden CLI test can't assert on a warning that never reaches stdout/stderr.

**What fixing it requires:** decide how `workers.Scrape.Run` communicates
warnings to its caller (an added return value, or a field on `sinks.Item`), thread
it through `job.Executor`, and decide where the CLI prints it (stderr, presumably,
same as the `fetched N, errors M, skipped K` summary line) and in what format.
Once that lands, `services/core` gets a `key-collision-last-write-wins`-style
integration case asserting the warning text, and the CLI plan gets its own golden
case for the same behavior.

**Blocks:** a golden CLI test case for `stages` key-collision (see
`docs/superpowers/plans/*cli*.md`'s "What this plan does not do").

---

## Crawl has no URL dedupe — a re-discovered link can be fetched/queued twice

**What's wrong:** `PROJECT_STRUCTURE.md`'s file tree names
`services/core/internal/job/workers/crawl/dedupe.go` — tracking URLs already
fetched or queued during a crawl so a link discovered from two different parent
pages isn't fetched or queued twice — but the file doesn't exist, and nothing in
`crawl.Crawl`'s design currently tracks this. No `source.crawl` job spec field
says how dedup is controlled (always on? opt-out? exact-string match or a
normalized URL?), and no test in
`services/core/internal/job/workers/crawl/crawl_test.go` exercises a URL
discovered from two different parent pages, so the current red suite doesn't
even pin whether dedup should happen, let alone how.

**Why not fixed now:** found while surveying `services/core/internal/job/workers/crawl`
for the real-implementation phase (not yet started) — implementing `Crawl.Run`
without a spec'd dedup behavior means inventing undocumented behavior no test
locks down.

**What fixing it requires:** a `JOB_SPEC_REFERENCE.md` §3.2 decision (a new
`source.crawl` field, or a documented always-on default) for how dedup is
controlled and what "same URL" means, a test fixture in `crawl_test.go`
exercising a URL discovered from two different parents, then the real
`dedupe.go` (or inline logic) implementing it.

**Blocks:** the real implementation of
`services/core/internal/job/workers/crawl.Crawl.Run`.

---

## Crawl never discovers links, never consults robots.txt, never seeds from a sitemap

**What's wrong:** `services/core.New`'s wiring (this plan) passes a permanent no-op
`workers.LinkExtractor` (`Extract` always returns `nil`) and `nil, nil` for
`crawl.RobotsChecker`/`crawl.SitemapFetcher` to `crawl.NewCrawl`, because no real
implementation of any of the three exists anywhere in the repo yet. This means a
real `pobear crawl` — once `Crawl.Run`/`Fetcher.Fetch`/etc. are actually
implemented — will only ever fetch its seed URLs, regardless of `--depth`,
`--sitemap`, or the default `robots: respect` behavior JOB_SPEC_REFERENCE.md §3.2
documents.

**Why not fixed now:** this plan's job is wiring + golden test coverage, not new
`services/core` feature work; none of the three has a spec'd real implementation
to wire in yet.

**What fixing it requires:** real implementations of `workers.LinkExtractor`
(HTML `<a href>` walking + relative-URL resolution — needs a base-URL parameter
added to `Extract`, per `PROJECT_STRUCTURE.md`'s existing open item),
`crawl.RobotsChecker` (fetch + parse `robots.txt`), and `crawl.SitemapFetcher`
(fetch + parse `/sitemap.xml`) — then rewiring `core.New`'s `liveExecutor` to use
the real ones instead of the no-op/nil stand-ins.

**Blocks:** any golden CLI test case that exercises actual link-following,
robots.txt enforcement, or sitemap seeding — this plan's crawl coverage is
scoped to `depth: 0` (seed-only) cases only, for exactly this reason.

---

## `limits.rps` has no wiring seam — the real binary will silently ignore it

**What's wrong:** `PROJECT_STRUCTURE.md` says `services/core/internal/fetch/throttle.go`
"enforces `limits.rps`; crawl's own frontier scheduling paces through this too," and
`fetch.NewThrottle` exists (with its own `throttle_test.go`), but nothing constructs
one anywhere in the repo. `services/core/build.go`'s `liveExecutor.Run` builds a
`Fetcher` from `tiers`/`concurrency`/`retries`/`backoff` only — `fetch.Fetcher` has
no throttle field at all (`services/core/internal/fetch/fetch.go`) — so there is no
seam through which a real `Fetcher.Fetch` implementation could apply rate limiting
even once written.

**Why not fixed now:** found while wiring `core.New` for the `cli-goldencli-tests`
plan; adding a throttle seam to `Fetcher` is a `services/core` design change outside
that plan's wiring-only scope, and `Fetcher.Fetch` itself isn't implemented yet either.

**What fixing it requires:** add a throttle field/parameter to `fetch.Fetcher`
(or thread one through its constructor), have `liveExecutor.Run` construct a
`fetch.NewThrottle(spec.Limits.RPS)` per job alongside the rest of the per-job
graph, and have `Fetcher.Fetch`'s real implementation consult it before each request.

**Blocks:** no test currently asserts `--rps` actually throttles — `limits.rps`'s
golden coverage (e.g. `crawl/print-spec-verbose`'s `"rps": 2`) only checks that the
value round-trips into the printed spec, not that it's enforced.

---

## `--output-format raw` has no seam in the real composition root

**What's wrong:** `cmd/core/main.go` calls `core.New(providersDir, credentialsPath,
os.Stdout)` once, before argv is parsed, so `services/core`'s `sinks.Stdout` always
writes the full JSON envelope straight to the process's stdout. `command.Scrape.Run`
gets its own `stdout io.Writer` parameter but has no way to intercept or transform
the items `core` emits, so once `Scrape.Run` is really implemented it has no path to
print "just the extracted value, no envelope" as `CLI_REFERENCE.md`'s
`--output-format raw` section requires.

**Why not fixed now:** found while writing golden fixtures for
`scrape/output-format-raw*` in the `cli-goldencli-tests` plan; the fix is a
`core.New`/`cli.New` composition-root design change (an item-callback or
alternate-writer seam), outside that plan's wiring-only scope.

**What fixing it requires:** decide how `core` hands raw items to a caller that
wants to reformat them instead of writing a fixed envelope — likely an
item-callback parameter on `core.New` (or a `core.Service` method `command.Scrape`
calls directly instead of going through the standard sink path) — then implement
`--output-format raw`'s single-value extraction/validation logic in `command.Scrape`
against that seam.

**Blocks:** the real implementation of `scrape/output-format-raw`,
`output-format-raw-two-fields`, and `output-format-raw-zero-fields`'s golden cases.

---

## `pobear auth`'s terminal prompt echoes the secret it promises not to

**What's wrong:** `CLI_REFERENCE.md`'s `pobear auth <provider>` section says the
secret prompt is "not echoed," but `cmd/core/main.go`'s `termPrompt` reads with a
plain `bufio.Reader` over `os.Stdin` — nothing disables terminal echo, so a real
interactive `pobear auth decodo` prints the token in the clear as it's typed.

**Why not fixed now:** found during the `cli-goldencli-tests` plan's final review;
`termPrompt` is real, already-shipped code, but suppressing terminal echo portably
needs a new dependency (`golang.org/x/term`'s `ReadPassword`, which also handles
Windows), and adding a new module dependency is a decision beyond that plan's
test-fixture scope. `providerator.Auth` doesn't call `prompt` yet either
(`_ = prompt`), so nothing currently exercises this path.

**What fixing it requires:** add `golang.org/x/term` to `go.mod`, replace
`termPrompt`'s read with `term.ReadPassword(int(os.Stdin.Fd()))`, and confirm it
degrades sanely when stdin isn't a real terminal (as it never is under `goldencli`,
which always feeds a piped `stdin` fixture file).

**Blocks:** nothing currently — `providerator.Auth` doesn't call `Prompt` yet, so
this has no test-visible effect until that real implementation lands.
