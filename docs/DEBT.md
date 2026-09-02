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
