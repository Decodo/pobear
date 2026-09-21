# pobear known debt

Things we know are wrong, missing, or incomplete, that we've deliberately chosen
not to fix at the time they were found. This is not a backlog of features — it's
a record of specific gaps discovered mid-work, so they don't get silently
forgotten or rediscovered from scratch later. Each entry says what's wrong, why
it wasn't fixed on the spot, and what fixing it actually requires.

An entry is removed once it's fixed — this file tracks open debt, not a history
of resolved debt (git history already has that).

---

## The wizard's tier-row model has no way to un-configure the Local tier

**What's wrong:** `Proxy` and `Provider` tier rows go back to "— not set" the
moment their one identifying field (`Proxy URL`, `Provider`) is cleared back to
blank — that's what makes them "unconfigured" again. `Local` has no such field:
its only content is `Profile`, which always has a valid value (`chrome` by
default) and can't be blanked out. So once the `Local` tier screen is opened and
left — even just by pressing `esc` immediately — `fc.localConfigured` is `true`
for the rest of the session, with no action in the UI that sets it back to
`false`.

**Why not fixed now:** flagged in the final review of the fetch-ladder plan as
a real but low-severity gap (worst case, the user reopens the wizard or deletes
the `Local` row's summary manually in the generated spec); adding a "clear tier"
action is a UI decision — what key, what it does to fields further down the
form — better made deliberately than folded into a bug-fix pass.

**What fixing it requires:** likely a `ctrl+d`-style clear-tier action on the
tier rows, mirroring the pattern this plan already uses for removing a CSS
extraction field pair (`services/cli/internal/wizard/scraper_form.go`'s
`ctrl+d` handling).

**Blocks:** nothing load-bearing — `Local` can always still be the sole
configured tier by simply never opening its screen; this only affects the case
where a user opens it, wants to back out to "unconfigured", and currently can't.

---

## `pobear docs --json`'s `itemFields` omits `"headers"`

**What's wrong:** `services/cli/internal/command/docs.go`'s hardcoded
`itemFields` contract list names `["url", "finalUrl", "parentUrl", "status",
"body", "depth", "data", "tier"]`, but the actual output envelope
(`services/core/internal/sink/sinks/sink.go`'s `Item`) also has a `Headers`
field (`json:"headers,omitempty"`). A caller reading the contract to know what
fields an item can carry won't see `headers` listed at all.

**Why not fixed now:** found during the fetch-ladder plan's final review while
fixing that same list's separate, unrelated omission of the new `"tier"`
field — genuinely pre-existing (predates this plan) and orthogonal to it, so
out of that plan's scope.

**What fixing it requires:** adding `"headers"` to the same 3-4 pinned copies
that were just kept in sync for `"tier"` — `services/cli/internal/command/docs.go`,
`services/cli/internal/command/docs_test.go`, `docs/CLI_REFERENCE.md`'s embedded
`pobear docs --json` example, and `cmd/core/testdata/docs/json-contract.yaml`.
Search each for `itemFields`/`want :=` to find the exact spots.

---

## No integration-level test exercises a real laddered spec end-to-end

**What's wrong:** every test covering fetch-tier escalation
(`services/core/internal/fetch`) runs at the `Fetcher`-unit level against fake
`Tier` implementations. Nothing proves that `services/core/build.go`'s real
wiring — an actual `Fetcher`, real tier clients, `sinks.Item.Tier` — agrees with
that unit-level behavior when a `ladder: true` spec runs through the production
`core.New(...)` path end to end.

**Why not fixed now:** flagged in the final review as low-risk given how
thorough the `Fetcher`-unit coverage already is; adding a full integration
harness case (a fixture HTTP server returning a blocked/rate-limited response on
the first rung and success on the next) was judged a reasonable scope boundary
to leave for a deliberate follow-up rather than bolt onto an already-large
plan's fix wave.

**What fixing it requires:** a case in `services/core/internal/integrationtest`
(or a similarly-shaped new package) with a fixture HTTP server that returns,
say, a `403` on the first rung and a `200` on the next, exercising a real
`ladder: true` spec through `core.New(...)` and asserting the emitted item's
`tier` matches which rung actually succeeded.
