# Fetch ladder (`--ladder` / `fetch.ladder`) design

## Goal

Resilience against blocks. Today, a job picks exactly one fetch tier
(`local`, `proxy`, or `provider`) and lives or dies by it — if `local` gets
blocked, the whole job fails and the user has to notice, then manually
re-run with `--provider` (or edit the job spec) to escalate. The ladder
feature lets a job configure more than one tier up front and have the
engine automatically climb from the starting tier to a higher one when it
gets blocked or rate-limited, instead of failing outright or requiring a
manual re-run.

Non-goals (explicitly out of scope for this design, revisit later if
needed): cost-based tier selection, "sticky" escalation state shared across
URLs in a job, escalating on failures other than `blocked`/`rateLimit`, and
a way to configure a tier but exclude it from a particular run's ladder
without un-configuring it.

## Terminology

The three fetch tiers have a fixed hierarchy, already implied by today's
docs ("`--proxy` is tier 1, `--provider` is tier 2" on top of the free
`local` tier): **`local` < `proxy` < `provider`**. A ladder only ever
climbs upward through this fixed order — it never climbs down, and it
never skips downward past its own starting tier.

A tier is **configured** if the job spec provides what that tier needs to
run at all: `local` needs nothing beyond `fetch.profile` and is always
available; `proxy` is configured when `fetch.proxy` is set; `provider` is
configured when `fetch.name` is set. The **ladder** for a job is the
ordered list of configured tiers at-or-above `fetch.use`.

## Job spec changes

One new field on `Fetch` (`pkg/jobspec/types.go`):

```go
type Fetch struct {
	Use         string
	Name        string
	Template    string
	Proxy       string
	Profile     string
	Options     map[string]any
	Timeout     string
	NoRedirects bool
	Headers     map[string]string
	Ladder      bool `yaml:"ladder,omitempty" json:"ladder,omitempty"`
}
```

Example spec, `local` escalating to `provider` (skipping `proxy` — it's
simply never configured):

```yaml
fetch:
  use: local
  ladder: true
  name: decodo
  template: universal
```

### Validation changes (`pkg/jobspec/fetch.go`)

Today, `tierScopedString`/`requireTier` reject a tier-scoped field
(`fetch.proxy`, `fetch.name`, `fetch.template`, `fetch.options`) unless it
belongs to the exact tier named in `fetch.use`. That rule is what currently
makes it impossible to express "I'm using `local`, but also have `provider`
configured as a fallback" in a spec at all.

New rule, replacing the exact-match check for these four fields:
- when `fetch.ladder` is `false` (the default): unchanged, exact match
  required, precisely today's behavior.
- when `fetch.ladder` is `true`: a tier-scoped field is valid as long as
  its own tier is **at or above** `fetch.use` in the fixed hierarchy (so
  `fetch.proxy` is invalid when `fetch.use: provider`, since there's
  nowhere above provider to climb from proxy — same direction-only rule
  the ladder itself follows).

`pkg/jobspec` exports the canonical tier order so both the validator here
and the fetch engine (below) share one source of truth instead of each
hand-rolling their own list:

```go
// pkg/jobspec, alongside the Fetch type
var FetchTierOrder = []string{"local", "proxy", "provider"}
```

After the four fields are parsed, one more check: if `fetch.ladder` is
`true`, at least one tier strictly above `fetch.use` must be configured —
`ladder: true` with nothing to climb to is a validation error (e.g.
`fetch.use: provider` with `ladder: true`, or `fetch.use: local` with
`ladder: true` but neither `proxy` nor `name` set).

## Escalation triggers and retry budget

- Escalates only on `fault.Blocked` and `fault.RateLimit` — the "you got
  detected" signals. A real `404`, a timeout, a provider auth failure, or
  a `provider` fault kind (bad manifest/response shape) don't get fixed by
  switching tiers, so those stay terminal at whichever rung produced them.
- Each rung gets its own full `limits.retries` budget, run with its
  existing backoff schedule — unchanged from today's per-tier retry loop.
  Moving to the next rung after a tier's retries are exhausted happens
  immediately, with no extra backoff between rungs: the whole point of
  escalating to a different tier (different IP, possibly different TLS
  fingerprint) is that it doesn't need the same "wait it out" treatment a
  same-tier retry does.
- Stateless per URL: every URL in the job independently starts its climb
  at `fetch.use`. A job doesn't remember that `local` got blocked on URL 1
  and skip straight to `proxy` for URL 2 — each URL pays for its own climb.
  Simple, requires no shared/concurrency-safe state in the `Fetcher`, and
  is the one to build first; revisit only if the repeated low-tier
  attempts turn out to be a real cost problem in practice.

## Fetch engine changes (`services/core/internal/fetch`)

`Fetcher.Fetch` (`fetch.go`) changes from "run the retry loop against one
named tier" to "run the retry loop against each rung in the ladder, in
order, until one succeeds or the ladder runs out":

```go
func (f *Fetcher) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error) {
	release, err := f.acquire(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()

	var result *tiers.Result
	var kind fault.Kind

	for _, tierName := range ladderFor(cfg) {
		tier, ok := f.tiers[tierName]
		if !ok {
			return nil, "", ErrUnknownTier
		}

		result, kind, err = f.fetchTierWithRetries(ctx, tier, cfg, url)
		if result != nil {
			result.Tier = tierName
		}
		if err == nil && kind == "" {
			return result, kind, nil
		}
		if !escalates(kind) {
			return result, kind, err
		}
	}
	return result, kind, err
}

func ladderFor(cfg jobspec.Fetch) []string {
	if !cfg.Ladder {
		return []string{cfg.Use}
	}
	start := slices.Index(jobspec.FetchTierOrder, cfg.Use)
	var ladder []string
	for _, t := range jobspec.FetchTierOrder[start:] {
		if configured(cfg, t) {
			ladder = append(ladder, t)
		}
	}
	return ladder
}

func configured(cfg jobspec.Fetch, tier string) bool {
	switch tier {
	case "local":
		return true
	case "proxy":
		return cfg.Proxy != ""
	case "provider":
		return cfg.Name != ""
	default:
		return false
	}
}

func escalates(k fault.Kind) bool {
	return k == fault.Blocked || k == fault.RateLimit
}
```

`fetchTierWithRetries` is today's existing per-tier attempt/backoff loop
(lines 51-77 of the current `fetch.go`), extracted unchanged into its own
method so it can run once per rung instead of once per whole `Fetch` call.
The semaphore acquire/release still wraps the entire ladder climb for one
URL, exactly as it wraps the single-tier retry loop today — one URL still
occupies exactly one concurrency slot regardless of how many rungs it
climbs. Throttle (`f.throttle.Wait`) is unchanged too, applying to every
attempt on every rung the same way it applies to every retry today.

When `cfg.Ladder` is `false`, `ladderFor` returns `[cfg.Use]` — a
single-element ladder — so the loop body runs exactly once, identically to
today's behavior. **This is intentionally the whole safety net for this
change**: every job that doesn't set `ladder: true` gets byte-for-byte the
same fetch behavior as before this feature existed.

### A real bug this design has to close: tier clients in `services/core/build.go`

`liveExecutor.createFetcher` currently only builds a fingerprinted client
for whichever tier `spec.Fetch.Use` happens to be:

```go
switch spec.Fetch.Use {
case "local":
	client, err := tiers.NewFingerprintedClient(spec.Fetch.Profile, "")
	localClient = client
case "proxy":
	client, err := tiers.NewFingerprintedClient(spec.Fetch.Profile, spec.Fetch.Proxy)
	proxyClient = client
}
```

If `spec.Fetch.Use == "local"` and the ladder escalates to `proxy`, the
`proxy` tier would silently run on the bare `http.DefaultClient` — no TLS
fingerprint, no configured proxy routing applied through the fingerprint
layer — defeating the entire point of escalating to it. This has to change
to build each client based on whether that tier's config is *present*,
not on whether it's the active `Use`:

```go
localClient, err := tiers.NewFingerprintedClient(spec.Fetch.Profile, "")
if err != nil {
	return nil, err
}

proxyClient := http.DefaultClient
if spec.Fetch.Proxy != "" {
	proxyClient, err = tiers.NewFingerprintedClient(spec.Fetch.Profile, spec.Fetch.Proxy)
	if err != nil {
		return nil, err
	}
}
```

`spec.Fetch.Profile` is guaranteed present whenever `local` or `proxy`
appears anywhere in a *valid* ladder (the validation above requires
`fetch.use` to be `local` or `proxy` for either tier to be reachable at
all, and both of those already require `profile`), so this is safe with no
new nil/empty-profile case to handle.

## Tier visibility in output

`tiers.Result` (`services/core/internal/fetch/tiers/tier.go`) gets a new
field, set by the `Fetcher` itself right after a rung produces the result
(not by each tier implementation — the `Fetcher` is what already knows
which named tier key it's currently on):

```go
type Result struct {
	Status   int
	Body     []byte
	FinalURL string
	Tier     string
}
```

That flows into `sinks.Item` (`services/core/internal/sink/sinks/sink.go`),
which is the actual envelope shape written to NDJSON/stdout/the wizard's
captured output — one new field, always populated:

```go
type Item struct {
	URL       string              `json:"url" yaml:"url"`
	FinalURL  string              `json:"finalUrl" yaml:"finalUrl"`
	ParentURL string              `json:"parentUrl" yaml:"parentUrl"`
	Status    int                 `json:"status" yaml:"status"`
	Headers   map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body      string              `json:"body" yaml:"body"`
	Depth     int                 `json:"depth" yaml:"depth"`
	Data      map[string]any      `json:"data,omitempty" yaml:"data,omitempty"`
	Tier      string              `json:"tier" yaml:"tier"`
}
```

`workers.Scrape.Run` (`services/core/internal/job/workers/scrape.go`) just
needs one more line copying `result.Tier` into the `sinks.Item{...}`
literal it already builds.

`Tier` is always populated, even for a plain non-ladder job — it's
trivially just whatever `fetch.use` was in that case. A consistent shape
(the field is always there) beats a field that's sometimes present and
sometimes not depending on whether ladder happened to be used.

On a **total failure** (every rung in the ladder exhausted without
success), the error and fault kind returned are whatever the *last*
attempted rung produced — so `pobear scrape ... [blocked]: tiers: provider
status 403` on a laddered job tells you it climbed all the way to
`provider` and even that got blocked, not just that `local` (the starting
tier) failed.

### CLI

`pobear scrape`/`pobear crawl` gain a `--ladder` boolean flag
(`services/cli/internal/command/flags.go`, alongside `--proxy`/
`--provider`). Validation in `crawlRun`/the scrape equivalent
(`services/cli/internal/command/scrape.go`, `crawl.go`):

- `--ladder` with neither `--proxy` nor `--provider` set: error
  (`--ladder requires --proxy and/or --provider to be set`) — nothing
  configured above `local` to climb to.
- More than one of `--proxy`/`--provider` set **without** `--ladder`: now
  an error too (`both --proxy and --provider are set — add --ladder to
  escalate between them, or remove one`), replacing today's silent
  pick-the-highest-tier behavior. There's no released version of pobear to
  keep this backward compatible with, so this is a straight cleanup rather
  than something that needs its own opt-in.
- The provider credential check that already runs upfront for a
  provider-tier job (before the job starts, so a missing credential fails
  fast) runs the same way whenever `provider` is anywhere in the ladder,
  even if it's just the top fallback rung and might never actually get
  reached. Finding out about a missing credential immediately beats
  discovering it deep into a long-running crawl the moment `local`/`proxy`
  finally get blocked.

### TUI

The wizard's current `Fetch via` field is a single mutually-exclusive
choice (`local`/`proxy`/`provider`), and `fetchFields` (`form.go`) branches
hard on it — only one tier's fields are ever shown. That model can't
express "configure `local` *and* `provider` at once," and cramming all
three tiers' fields inline (worse for Crawl, which already has its own
long field list: Depth, Max pages, Scope, Sitemap, Robots, rps...) doesn't
scale.

Replacing `Fetch via` + its conditional fields with **three fixed rows**,
one per tier, each a navigation entry into its own sub-screen rather than
an inline field:

```
┌─ Scrape ───────────────────────────────────────────────────────── esc: back to menu ─┐
│  URL            (one per line)                                                       │
│  Local          ✓ chrome                     (enter to configure)                    │
│  Proxy          — not set                     (enter to configure)                   │
│  Provider       ✓ decodo / universal          (enter to configure)                   │
│  Extract        bare                                                                 │
│  Concurrency    8                                                                    │
│  Retries        3                                                                    │
│  [ Save as job.yaml ]   [ Run ]                                                      │
└────────────────────────────────────────────────────────────────────────────────────┘
```

- All three start **unconfigured** — pressing `enter` on a row pushes a
  new screen scoped to just that tier (`Profile` for Local; `Proxy URL` +
  `Profile` for Proxy; the existing provider search/Template/options
  fields for Provider), `esc` pops back to the main form, and the row then
  shows a one-line summary instead of the raw fields.
- Whichever tiers end up configured become the ladder, in fixed order,
  automatically — filling in a second or third tier's screen **is** the
  opt-in, no separate "Ladder" toggle row. The wizard sets `fetch.ladder:
  true` on the generated spec whenever 2+ tiers end up configured, and
  `fetch.use` is the lowest configured tier. With exactly one configured,
  `fetch.ladder` stays `false`/omitted and `fetch.use` is that one tier —
  byte-for-byte what the wizard produces today.
- On Save/Run with all three unconfigured: same class of error as today's
  "no URLs to scrape" — `✗ configure at least one of Local/Proxy/Provider`.
- Results list (`services/cli/internal/wizard/results.go`,
  `formatResultRow`): shows the tier inline, e.g. `✓ 200 (proxy)`, so
  which rung handled a URL is visible without opening item detail. Item
  detail (`itemdetail.go`) shows it too, since it's just part of the same
  envelope map already rendered there.

This is a real restructuring of `form.go`'s field computation (new sub-screen
type modeled on the existing `providerDetailScreen` pattern, `fetchConfig`
gaining per-tier "configured" state) and touches both `scraper_form.go` and
`crawl_form.go` (shared via `fetchFields`, so one implementation serves
both), but it's built on navigation primitives (`pushScreen`/`popScreen`)
already used throughout the wizard.

## Testing

- `pkg/jobspec`: validation tests for the relaxed tier-scoping rule (valid
  and invalid combinations of `use`/`ladder`/`proxy`/`name` — including the
  "ladder true but nothing above use" and "ladder false but two tiers set"
  cases), plus that `ladder: false`/omitted keeps every existing validation
  test passing unchanged.
- `services/core/internal/fetch`: unit tests against a fake multi-tier
  `Tier` map — asserts the ladder climbs on `Blocked`/`RateLimit` and stops
  cold on any other fault kind, that each rung gets its own full retry
  count, that a non-ladder `cfg` runs exactly the old single-tier path, and
  that `Result.Tier` matches whichever tier actually produced the result.
- `services/core/build.go`: a test that a laddered spec with `use: local`
  and a configured `proxy` produces a `proxy` tier client that's actually
  fingerprinted/routed through the configured proxy URL, not
  `http.DefaultClient` — this is the regression test for the client-wiring
  bug called out above.
- `services/cli/internal/command`: CLI-level tests for the new `--ladder`
  flag and its two new validation errors (missing config to climb to;
  ambiguous multi-tier config without `--ladder`).
- `services/cli/internal/wizard`: form tests for the three-tier-row model
  (each row's sub-screen, the summary text once configured, `ladder`/`use`
  derivation from however many tiers ended up configured, the "configure
  at least one" validation), plus results/item-detail tests asserting the
  tier shows up in both.

## Docs to update alongside implementation

- `docs/JOB_SPEC_REFERENCE.md` — new `fetch.ladder` field, the relaxed
  tier-scoping rule, the new `tier` envelope field.
- `docs/CLI_REFERENCE.md` — `--ladder` flag, its two validation errors, the
  updated wizard walkthrough mockups for the three-tier-row Scrape/Crawl
  forms and their sub-screens.
- `docs/PROVIDER_REFERENCE.md` — no changes expected; a provider manifest's
  shape is unaffected by which tier reaches it.
