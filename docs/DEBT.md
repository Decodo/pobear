# pobear known debt

Things we know are wrong, missing, or incomplete, that we've deliberately chosen
not to fix at the time they were found. This is not a backlog of features — it's
a record of specific gaps discovered mid-work, so they don't get silently
forgotten or rediscovered from scratch later. Each entry says what's wrong, why
it wasn't fixed on the spot, and what fixing it actually requires.

An entry is removed once it's fixed — this file tracks open debt, not a history
of resolved debt (git history already has that).

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

**Why not fixed now:** this plan's job is wiring + CLI test coverage, not new
`services/core` feature work; none of the three has a spec'd real implementation
to wire in yet.

**What fixing it requires:** real implementations of `workers.LinkExtractor`
(HTML `<a href>` walking + relative-URL resolution — needs a base-URL parameter
added to `Extract`, per `PROJECT_STRUCTURE.md`'s existing open item),
`crawl.RobotsChecker` (fetch + parse `robots.txt`), and `crawl.SitemapFetcher`
(fetch + parse `/sitemap.xml`) — then rewiring `core.New`'s `liveExecutor` to use
the real ones instead of the no-op/nil stand-ins.

**Blocks:** any CLI test case that exercises actual link-following,
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
test coverage (e.g. `crawl/print-spec-verbose`'s `"rps": 2`) only checks that the
value round-trips into the printed spec, not that it's enforced.

---

## `pobear auth`'s terminal prompt echoes the secret it promises not to

**What's wrong:** `CLI_REFERENCE.md`'s `pobear auth <provider>` section says the
secret prompt is "not echoed," but `cmd/core/main.go`'s `termPrompt` reads with a
plain `bufio.Reader` over `os.Stdin` — nothing disables terminal echo, so a real
interactive `pobear auth decodo` prints the token in the clear as it's typed.
`providerator.Auth` now really calls this `prompt` function (`services/cli`'s real
implementation), so a real interactive `pobear auth` run hits this bug today, not
just once `providerator.Auth` gets implemented.

**Why not fixed now:** found during the `cli-goldencli-tests` plan's final review;
`termPrompt` is real, already-shipped code, but suppressing terminal echo portably
needs a new dependency (`golang.org/x/term`'s `ReadPassword`, which also handles
Windows), and adding a new module dependency is a decision beyond that plan's
test-fixture scope.

**What fixing it requires:** add `golang.org/x/term` to `go.mod`, replace
`termPrompt`'s read with `term.ReadPassword(int(os.Stdin.Fd()))`, and confirm it
degrades sanely when stdin isn't a real terminal (as it never is under `clitest`,
which always feeds a piped `stdin` fixture file).

**Blocks:** nothing test-visible — `clitest`'s piped stdin never exercises real
terminal echo either way — but affects every real interactive `pobear auth` run.

---

## `limits.concurrency` never gets real parallel load

**What's wrong:** `services/core/internal/fetch.Fetcher` builds a real
semaphore sized to `limits.concurrency` and its own tests
(`fetch_test.go`) confirm it caps concurrent `Fetch` calls correctly, but
nothing ever calls it concurrently in practice: `job.Executor`'s URL-list
loop and `job/workers/crawl.Crawl`'s frontier loop both dispatch
`workers.Scrape.Run` synchronously, one URL at a time, inside a plain
`for` loop. `limits.concurrency` round-trips correctly into the parsed
spec and the semaphore is exercised in isolation, but a real multi-URL or
multi-page job never has more than one fetch in flight regardless of its
value.

**Why not fixed now:** found during the post-implementation code review of
`services/core`. Making either loop genuinely concurrent is real
synchronization work — a worker pool or bounded goroutine fan-out, plus
deciding how per-URL/per-page errors and sink writes serialize — and would
very likely break existing tests that assert exact dispatch order/count
against non-mutex-protected fakes (e.g.
`TestExecutorURLListDispatchesEachURLAndTalliesSuccesses`'s exact
`scrape.gotURLs` order, and the crawl package's frontier fakes). That's a
larger, riskier change than the code-review fix pass this was found in was
scoped for.

**What fixing it requires:** redesign `job.Executor`'s URL loop and
`crawl.Crawl`'s frontier loop to dispatch `workers.Scrape.Run` across a
bounded pool of goroutines sized by `limits.concurrency`, decide how
results/errors are collected back (a results channel, or a mutex-guarded
accumulator), make the existing order-sensitive fakes/tests either
tolerant of reordering or explicitly synchronized, and make `sinks.Sinker`
writes concurrency-safe if they aren't already (`services/core/internal/sink/sinks`'s
buffered sinks currently assume single-writer use).

**Blocks:** nothing currently — small jobs work correctly, just serially.
Blocks realistic throughput on any job with `limits.concurrency` > 1 and
more than one URL/page.

---

## `fetch.profile` (TLS fingerprint preset) is parsed but never consulted

**What's wrong:** `fetch.profile` (`chrome`/`firefox`/`safari`) is parsed
and validated by `pkg/jobspec/fetch.go` for `local`/`proxy` tiers — including
defaulting to `chrome` when unset — but nothing in
`services/core/internal/fetch/tiers` (the `local`/`proxy` tier
implementations, both built on `doHTTPFetch` in `tiers/http.go`) ever reads
`Fetch.Profile` or configures a TLS ClientHello/JA3 fingerprint to match the
named browser. A real fetch always goes out with Go's default `net/http`
TLS fingerprint, regardless of which profile the job spec names.

**Why not fixed now:** found during the post-implementation code review of
`services/core`. TLS fingerprinting needs a real ClientHello-spoofing
library (e.g. `utls`), which isn't a current dependency, and picking one
plus wiring per-profile fingerprint tables is a scoped piece of design work
on its own, well beyond a review fix pass.

**What fixing it requires:** add a TLS fingerprinting dependency (most
likely `github.com/refraction-networking/utls`), build or source a
ClientHello spec per supported profile (`chrome`/`firefox`/`safari`), thread
`Fetch.Profile` from the parsed spec into `tiers.Local`/`tiers.Proxy`'s
construction, and have `doHTTPFetch` dial through a `utls`-wrapped
connection using the selected spec instead of the stdlib default transport.

**Blocks:** any real anti-bot-evasion use case that depends on
`fetch.profile` actually changing the TLS fingerprint — currently the field
is inert past validation.

---

## `pobear wizard` (bare `pobear`) is entirely unimplemented

**What's wrong:** `CLI_REFERENCE.md`'s wizard section describes a full-screen
reactive terminal UI (`services/cli/internal/wizard/tui.go`, per
`PROJECT_STRUCTURE.md`), but `services/cli/internal/wizard` currently holds only
a `doc.go` — no types, no tests, no TUI dependency in `go.mod`. `services/cli.New`
doesn't register a `"wizard"` command at all. Bare `pobear` now prints a
clearer "interactive mode is not yet available" message (rather than the
confusing `unknown command "wizard"` it used to print), but there's still no
actual wizard to launch; `pobear wizard` explicitly still prints
`pobear: unknown command "wizard"` (exit 3), since nothing registers that name.

**Why not fixed now:** found while implementing the rest of `services/cli`
(providerator/jobgen/command) this session — every other package already had
real stub files and a full test-first suite to implement against; wizard has
neither, and is a categorically bigger, differently-shaped task (a reactive TUI,
needing its own design/brainstorm pass and a TUI library choice like
`bubbletea`) rather than "implement this stub to green."

**What fixing it requires:** a brainstorm/design pass picking a TUI approach,
writing the test-first spec for the form-reactivity rules `CLI_REFERENCE.md`
describes (field visibility changing with `Fetch via`/`Extract`/`Template`,
inline `Test`/`Save`/`Run` actions), adding the chosen TUI dependency, then
implementing `wizard.go`/`tui.go` against that spec, plus registering
`"wizard"` in `services/cli/build.go`'s command map.

**Blocks:** any real interactive `pobear`/`pobear wizard` invocation — no
fixture currently exercises it either, for the same reason.
