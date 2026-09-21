# Fetch Ladder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a job spec configure more than one fetch tier and have the engine automatically climb from `local` to `proxy` to `provider` when it hits `fault.Blocked`/`fault.RateLimit`, instead of failing outright — surfaced through `pkg/jobspec`, the fetch engine, the CLI (`--ladder`), and the TUI wizard (a three-tier-row redesign).

**Architecture:** One new `fetch.ladder` bool threads through every layer: `pkg/jobspec` validates it and relaxes its tier-scoping rules when it's set; `services/core/internal/fetch.Fetcher` derives an ordered list of configured tiers from a `jobspec.Fetch` and climbs it on `blocked`/`rateLimit`; `services/core/build.go` gets a real bug fix so an escalated-to tier is never silently unfingerprinted; a new `Tier` field threads through the output envelope so every surface can show which tier actually served a URL; the CLI gets an explicit `--ladder` flag; the TUI replaces its single `Fetch via` field with three independent tier rows, each its own sub-screen, where configuring more than one tier *is* the opt-in.

**Tech Stack:** Go, bubbletea/lipgloss (TUI), cobra (CLI), `gopkg.in/yaml.v3`.

**Spec:** `docs/superpowers/specs/2026-09-21-fetch-ladder-design.md`

## Global Constraints

- Escalates only on `fault.Blocked` and `fault.RateLimit` — no other fault kind triggers a climb.
- Each rung gets its own full `limits.retries` budget; no extra backoff is added between rungs.
- Escalation is stateless per URL — every URL independently starts its climb at `fetch.use`; no shared "last successful rung" state.
- Tier hierarchy is fixed: `local` < `proxy` < `provider`. A ladder only ever climbs upward, never past its own starting tier.
- `fetch.ladder` defaults to `false`/omitted; every job that doesn't set it gets byte-for-byte today's existing single-tier behavior.
- There is no released version of pobear — CLI flag-combination behavior is free to change (no backward-compat constraint), per explicit instruction from the project owner.
- `Tier` in the output envelope (`sinks.Item.Tier`) is always populated, even for a non-ladder job.
- Go code in this repo carries near-zero comments — only `//nolint:...` directives required for lint, package doc comments only when `staticcheck`'s `ST1000` requires one, and rare non-obvious-WHY comments. Do not add explanatory comments to the code you write in this plan.
- After every task: `gofmt -l .`, `go build ./...`, `go vet ./...`, `golangci-lint run ./...` (`make lint`), and `go test -race -count=1 ./...` (`make test`) must all be clean before committing.

---

### Task 1: `fetch.ladder` field parses, `FetchTierOrder` exported

**Files:**
- Modify: `pkg/jobspec/types.go` (the `Fetch` struct)
- Modify: `pkg/jobspec/fetch.go` (`parseFetch`)
- Test: `pkg/jobspec/testdata/validation/fetch/ladder-parses-true.yaml`, `pkg/jobspec/testdata/validation/fetch/ladder-defaults-false.yaml` (new fixtures, consumed by the existing `pkg/jobspec/validation_test.go` fixture runner)

**Interfaces:**
- Produces: `jobspec.Fetch.Ladder bool`; `jobspec.FetchTierOrder = []string{"local", "proxy", "provider"}` (exported var).

- [ ] **Step 1: Write the failing fixtures**

`pkg/jobspec/testdata/validation/fetch/ladder-parses-true.yaml`:
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: local, ladder: true}
wantErr: false
wantSpec:
  version: 1
  source:
    urls: [https://a.test]
  fetch:
    use: local
    profile: chrome
    ladder: true
    timeout: 30s
    noRedirects: false
    headers: {}
  limits:
    concurrency: 8
    rps: 0
    retries: 3
    backoff: exponential
    pluginConcurrency: 0
    pluginTimeout: 30s
  stages: []
  sink:
    use: stdout
```

`pkg/jobspec/testdata/validation/fetch/ladder-defaults-false.yaml`:
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: local}
wantErr: false
wantSpec:
  version: 1
  source:
    urls: [https://a.test]
  fetch:
    use: local
    profile: chrome
    ladder: false
    timeout: 30s
    noRedirects: false
    headers: {}
  limits:
    concurrency: 8
    rps: 0
    retries: 3
    backoff: exponential
    pluginConcurrency: 0
    pluginTimeout: 30s
  stages: []
  sink:
    use: stdout
```

Note `ladder-defaults-false.yaml`'s `fetch.ladder: true` up top in the *input* is intentionally absent — this fixture checks the omitted-field default.

- [ ] **Step 2: Run the fixtures to verify they fail**

Run: `go test ./pkg/jobspec/... -run TestValidationFixtures/fetch/ladder -v`
Expected: FAIL — both fixtures currently fail because `fetch: {use: local, ladder: true}` isn't yet an allowed key in the mapping (`AsMapping` rejects unknown keys), and neither `wantSpec` has a `Ladder` field to compare since it doesn't exist yet on the `Fetch` struct (this will actually fail to compile/match rather than a clean assertion failure — that's expected at this stage).

- [ ] **Step 3: Add the field and the shared tier-order var**

In `pkg/jobspec/types.go`, add `Ladder` to the `Fetch` struct (keep it as the last field, matching the spec's exact tag style):
```go
type Fetch struct {
	Use         string            `yaml:"use" json:"use"`
	Name        string            `yaml:"name,omitempty" json:"name,omitempty"`
	Template    string            `yaml:"template,omitempty" json:"template,omitempty"`
	Proxy       string            `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	Profile     string            `yaml:"profile,omitempty" json:"profile,omitempty"`
	Options     map[string]any    `yaml:"options,omitempty" json:"options,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	NoRedirects bool              `yaml:"noRedirects,omitempty" json:"noRedirects,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Ladder      bool              `yaml:"ladder,omitempty" json:"ladder,omitempty"`
}

var FetchTierOrder = []string{"local", "proxy", "provider"}
```

(Match whatever the existing field tags in your copy of `types.go` actually look like — the point is `Ladder bool` goes on the end with `yaml:"ladder,omitempty" json:"ladder,omitempty"`, and `FetchTierOrder` is a package-level var next to the `Fetch` type.)

In `pkg/jobspec/fetch.go`, add `"ladder"` to the allowed-keys list and parse it right after `use`, before the tier-scoped fields:
```go
m, err := utils.AsMapping(raw, "fetch", "use", "name", "template", "proxy", "profile", "options", "timeout", "noRedirects", "headers", "ladder")
if err != nil {
	return Fetch{}, err
}

useRaw, hasUse := m["use"]
if !hasUse {
	return Fetch{}, utils.Errorf("fetch.use", "`fetch.use` is required")
}
use, _ := useRaw.(string)
if err := utils.EnumField("fetch.use", use, "local", "proxy", "provider"); err != nil {
	return Fetch{}, err
}

ladder, err := utils.GetBool(m, "ladder", "fetch.ladder", false)
if err != nil {
	return Fetch{}, err
}

fetch := Fetch{Use: use, Ladder: ladder}
```

Leave the rest of `parseFetch` (the `fetch.Name`/`fetch.Template`/`fetch.Proxy`/`parseFetchProfile`/`parseFetchOptions` calls) untouched for this task — Task 2 changes those.

- [ ] **Step 4: Run the fixtures to verify they pass**

Run: `go test ./pkg/jobspec/... -run TestValidationFixtures/fetch/ladder -v`
Expected: PASS

- [ ] **Step 5: Run the full jobspec suite to confirm no regressions**

Run: `go test ./pkg/jobspec/... -v`
Expected: PASS — every existing fixture's `wantSpec.fetch` block that doesn't mention `ladder` still matches, since the Go zero value for the new bool field is `false` on both sides of the comparison.

- [ ] **Step 6: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l . && go build ./... && go vet ./...
git add pkg/jobspec/types.go pkg/jobspec/fetch.go pkg/jobspec/testdata/validation/fetch/ladder-parses-true.yaml pkg/jobspec/testdata/validation/fetch/ladder-defaults-false.yaml
git commit -m "jobspec: parse fetch.ladder, export FetchTierOrder"
```

---

### Task 2: Relaxed tier-scoping validation when `ladder: true`

**Files:**
- Modify: `pkg/jobspec/fetch.go` (`tierScopedString`, `requireTier`, `parseFetch`, `parseFetchOptions`)
- Test: several new fixtures under `pkg/jobspec/testdata/validation/fetch/`

**Interfaces:**
- Consumes: `jobspec.FetchTierOrder` (Task 1).
- Produces: `requireTierAtOrAbove(present bool, field, use, ownTier string, ladder bool) *ValidationError` — the new validation helper other tasks don't need to call directly, but its error message shape (`wantMessage` below) is what future fixtures should expect.

- [ ] **Step 1: Write the failing fixtures**

`pkg/jobspec/testdata/validation/fetch/ladder-local-to-provider-valid.yaml`:
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: local, ladder: true, name: acme}
wantErr: false
wantSpec:
  version: 1
  source:
    urls: [https://a.test]
  fetch:
    use: local
    name: acme
    profile: chrome
    ladder: true
    timeout: 30s
    noRedirects: false
    headers: {}
  limits:
    concurrency: 8
    rps: 0
    retries: 3
    backoff: exponential
    pluginConcurrency: 0
    pluginTimeout: 30s
  stages: []
  sink:
    use: stdout
```

`pkg/jobspec/testdata/validation/fetch/ladder-proxy-to-provider-valid.yaml`:
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: proxy, proxy: "http://p.test", ladder: true, name: acme}
wantErr: false
wantSpec:
  version: 1
  source:
    urls: [https://a.test]
  fetch:
    use: proxy
    name: acme
    proxy: "http://p.test"
    profile: chrome
    ladder: true
    timeout: 30s
    noRedirects: false
    headers: {}
  limits:
    concurrency: 8
    rps: 0
    retries: 3
    backoff: exponential
    pluginConcurrency: 0
    pluginTimeout: 30s
  stages: []
  sink:
    use: stdout
```

`pkg/jobspec/testdata/validation/fetch/ladder-still-rejects-tier-below-use.yaml` (proxy is *below* `use: provider`, so even with `ladder: true` it's invalid — the ladder only climbs up):
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: provider, name: acme, ladder: true, proxy: "http://p.test"}
wantErr: true
wantField: fetch.proxy
wantMessage: "`fetch.proxy`'s tier (proxy) is below `fetch.use` (provider) -- with `fetch.ladder: true`, a configured tier must be at or above `fetch.use`"
```

`pkg/jobspec/testdata/validation/fetch/ladder-false-still-rejects-mixed-tiers.yaml` (regression: without `ladder: true`, today's exact-match rule is unchanged):
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: local, name: acme}
wantErr: true
wantField: fetch.name
wantMessage: "`fetch.name` requires `fetch.use: provider`, got \"local\""
```

`pkg/jobspec/testdata/validation/fetch/ladder-nothing-to-climb-use-provider.yaml` (the "nothing above `use`" check — this one belongs to Task 3, but write it now alongside the others since it's the same file; it's expected to fail in this task and start passing once Task 3 lands):
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: provider, name: acme, ladder: true}
wantErr: true
wantField: fetch.ladder
wantMessage: "`fetch.ladder: true` requires a tier above `fetch.use: provider` to be configured (proxy and/or provider)"
```

- [ ] **Step 2: Run the fixtures to verify the new ones fail**

Run: `go test ./pkg/jobspec/... -run TestValidationFixtures/fetch/ladder -v`
Expected: FAIL for `ladder-local-to-provider-valid`, `ladder-proxy-to-provider-valid`, and `ladder-still-rejects-tier-below-use` (today's code rejects any tier-scoped field that isn't an exact match for `use`, ladder or not). `ladder-false-still-rejects-mixed-tiers` should already PASS (it's exercising unchanged behavior). `ladder-nothing-to-climb-use-provider` will also FAIL for now — Task 3 makes it pass; leave it red at the end of this task, that's expected.

- [ ] **Step 3: Implement the relaxed validation**

In `pkg/jobspec/fetch.go`, change `tierScopedString`'s signature from a variadic `allowedTiers ...string` to a single `ownTier string` plus a `ladder bool`, and add the new `requireTierAtOrAbove` helper (the old `requireTier` stays exactly as-is, still used by `parseFetchProfile`, which doesn't need this relaxation — see Global Constraints and the spec's Terminology section for why):

```go
func tierScopedString(m map[string]any, key, field, use, ownTier string, ladder bool) (string, *ValidationError) {
	raw, present := m[key]
	if err := requireTierAtOrAbove(present, field, use, ownTier, ladder); err != nil {
		return "", err
	}
	if !present {
		return "", nil
	}
	s, _ := raw.(string)
	return s, nil
}

func requireTierAtOrAbove(present bool, field, use, ownTier string, ladder bool) *ValidationError {
	if !present {
		return nil
	}
	if !ladder {
		if use == ownTier {
			return nil
		}
		return utils.Errorf(field, "`%s` requires `fetch.use: %s`, got %q", field, ownTier, use)
	}
	if slices.Index(FetchTierOrder, ownTier) >= slices.Index(FetchTierOrder, use) {
		return nil
	}
	return utils.Errorf(field, "`%s`'s tier (%s) is below `fetch.use` (%s) -- with `fetch.ladder: true`, a configured tier must be at or above `fetch.use`", field, ownTier, use)
}
```

Update the three `tierScopedString` call sites in `parseFetch` to pass `ladder`:
```go
if fetch.Name, err = tierScopedString(m, "name", "fetch.name", use, "provider", ladder); err != nil {
	return Fetch{}, err
}
if fetch.Template, err = tierScopedString(m, "template", "fetch.template", use, "provider", ladder); err != nil {
	return Fetch{}, err
}
if fetch.Proxy, err = tierScopedString(m, "proxy", "fetch.proxy", use, "proxy", ladder); err != nil {
	return Fetch{}, err
}
```

Update `parseFetchOptions` the same way — add a `ladder bool` parameter and use `requireTierAtOrAbove` instead of `requireTier`:
```go
func parseFetchOptions(m map[string]any, use string, ladder bool, fetch *Fetch) *ValidationError {
	_, present := m["options"]
	if err := requireTierAtOrAbove(present, "fetch.options", use, "provider", ladder); err != nil {
		return err
	}
	switch {
	case present:
		optMap, ok := m["options"].(map[string]any)
		if !ok {
			return utils.Errorf("fetch.options", "`fetch.options` must be a mapping")
		}
		fetch.Options = optMap
	case use == "provider":
		fetch.Options = map[string]any{}
	}
	return nil
}
```

And its call site in `parseFetch`:
```go
if err := parseFetchOptions(m, use, ladder, &fetch); err != nil {
	return Fetch{}, err
}
```

`parseFetchProfile`'s call site (`parseFetchProfile(m, use, &fetch)`) is unchanged — do not touch it.

- [ ] **Step 4: Run the fixtures to verify the ones this task covers pass**

Run: `go test ./pkg/jobspec/... -run TestValidationFixtures/fetch/ladder -v`
Expected: PASS for `ladder-local-to-provider-valid`, `ladder-proxy-to-provider-valid`, `ladder-still-rejects-tier-below-use`, `ladder-false-still-rejects-mixed-tiers`. `ladder-nothing-to-climb-use-provider` is still expected to FAIL — that's Task 3.

- [ ] **Step 5: Run the full jobspec suite to confirm no regressions**

Run: `go test ./pkg/jobspec/... -v`
Expected: PASS on everything except `ladder-nothing-to-climb-use-provider`.

- [ ] **Step 6: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l . && go build ./... && go vet ./...
git add pkg/jobspec/fetch.go pkg/jobspec/testdata/validation/fetch/
git commit -m "jobspec: relax tier-scoping validation when fetch.ladder is true"
```

---

### Task 3: `fetch.ladder: true` requires a tier above `use` to be configured

**Files:**
- Modify: `pkg/jobspec/fetch.go` (`parseFetch`)
- Test: `pkg/jobspec/testdata/validation/fetch/ladder-nothing-to-climb-use-provider.yaml` (already written in Task 2), plus one more new fixture below

**Interfaces:**
- Consumes: `jobspec.FetchTierOrder`, `fetch.Name`/`fetch.Proxy` as parsed earlier in `parseFetch` (Task 2).

- [ ] **Step 1: Write the second failing fixture**

`pkg/jobspec/testdata/validation/fetch/ladder-nothing-to-climb-use-local.yaml`:
```yaml
input: |
  version: 1
  source: {urls: [https://a.test]}
  fetch: {use: local, ladder: true}
wantErr: true
wantField: fetch.ladder
wantMessage: "`fetch.ladder: true` requires a tier above `fetch.use: local` to be configured (proxy and/or provider)"
```

- [ ] **Step 2: Run both fixtures to verify they fail**

Run: `go test ./pkg/jobspec/... -run TestValidationFixtures/fetch/ladder-nothing-to-climb -v`
Expected: FAIL — nothing currently rejects `ladder: true` with no higher tier configured.

- [ ] **Step 3: Implement the check**

In `pkg/jobspec/fetch.go`'s `parseFetch`, add this right after the `parseFetchOptions` call (so `fetch.Name`/`fetch.Proxy` are both already populated) and before the `timeout` parsing:

```go
if ladder {
	hasHigherTier := (use == "local" && (fetch.Proxy != "" || fetch.Name != "")) ||
		(use == "proxy" && fetch.Name != "")
	if !hasHigherTier {
		return Fetch{}, utils.Errorf("fetch.ladder", "`fetch.ladder: true` requires a tier above `fetch.use: %s` to be configured (proxy and/or provider)", use)
	}
}
```

- [ ] **Step 4: Run both fixtures to verify they pass**

Run: `go test ./pkg/jobspec/... -run TestValidationFixtures/fetch/ladder-nothing-to-climb -v`
Expected: PASS

- [ ] **Step 5: Run the full jobspec suite**

Run: `go test ./pkg/jobspec/... -v`
Expected: PASS across every fixture, including all of Tasks 1-2's.

- [ ] **Step 6: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l . && go build ./... && go vet ./...
git add pkg/jobspec/fetch.go pkg/jobspec/testdata/validation/fetch/ladder-nothing-to-climb-use-local.yaml
git commit -m "jobspec: reject fetch.ladder: true with nothing above use to climb to"
```

---

### Task 4: `Tier` threads through `tiers.Result` → `sinks.Item` (no escalation logic yet)

**Files:**
- Modify: `services/core/internal/fetch/tiers/tier.go` (`Result`)
- Modify: `services/core/internal/fetch/fetch.go` (`Fetcher.Fetch` sets `result.Tier`)
- Modify: `services/core/internal/sink/sinks/sink.go` (`Item`)
- Modify: `services/core/internal/job/workers/scrape.go` (`Scrape.Run` copies it across)
- Test: `services/core/internal/fetch/fetcher_test.go` (existing file — add a case), `services/core/internal/job/workers/scrape_test.go` (existing file — add a case)

**Interfaces:**
- Produces: `tiers.Result.Tier string`; `sinks.Item.Tier string`.
- Consumes (later tasks): Task 6's escalation algorithm sets `result.Tier` to whichever rung actually produced the result, not just `cfg.Use` — this task only covers the current single-tier case.
- The existing fakes this task reuses (already defined in those two files, do not redefine them): `services/core/internal/fetch/fetcher_test.go` has `fakeTier{results []fakeTierResult, calls int}` implementing `fetch.Tier`, where `fakeTierResult{result *tiers.Result, fault fault.Kind, err error}`; `services/core/internal/job/workers/scrape_test.go` has `fakeFetcher{result *tiers.Result, fault fault.Kind, err error, gotURL string}`, `fakeStager`, `fakeSinker{err error, gotItem sinks.Item, calls int}`, `fakeLinkExtractor`. Both test files are external test packages (`package fetch_test` / `package workers_test`), so every call below is package-qualified (`fetch.NewFetcher`, `workers.NewScrape`, etc.).

- [ ] **Step 1: Write the failing tests**

In `services/core/internal/fetch/fetcher_test.go`, add:
```go
func TestFetchSetsResultTierToTheConfiguredUseTier(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local}, 0, 1, noBackoff, nil)

	result, kind, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil || kind != "" {
		t.Fatalf("Fetch() = %v, %q, %v, want a clean success", result, kind, err)
	}
	if result.Tier != "local" {
		t.Fatalf("result.Tier = %q, want %q", result.Tier, "local")
	}
}
```
(`noBackoff` is the existing helper already defined at the top of this file — reuse it, same as every other test in the file already does.)

In `services/core/internal/job/workers/scrape_test.go`, add:
```go
func TestScrapeRunCopiesResultTierIntoTheSinkItem(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Tier: "proxy"}}
	sinker := &fakeSinker{}
	s := workers.NewScrape(fetcher, &fakeStager{data: map[string]any{}}, sinker, &fakeLinkExtractor{})

	_, _, kind, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test", "", 0, nil)

	if err != nil || kind != "" {
		t.Fatalf("Run() kind = %q, err = %v, want a clean success", kind, err)
	}
	if sinker.gotItem.Tier != "proxy" {
		t.Fatalf("sink item Tier = %q, want %q", sinker.gotItem.Tier, "proxy")
	}
}
```

- [ ] **Step 2: Run both tests to verify they fail**

Run: `go test ./services/core/internal/fetch/... ./services/core/internal/job/workers/... -run "TestFetchSetsResultTierToTheConfiguredUseTier|TestScrapeRunCopiesResultTierIntoTheSinkItem" -v`
Expected: FAIL to compile — `Result`/`Item` don't have a `Tier` field yet.

- [ ] **Step 3: Add the field and wire it through**

`services/core/internal/fetch/tiers/tier.go`:
```go
package tiers

type Result struct {
	Status   int
	Body     []byte
	FinalURL string
	Tier     string
}
```

`services/core/internal/fetch/fetch.go`, in `Fetcher.Fetch`, set `result.Tier` right after the tier's `Fetch` call returns (this task keeps the rest of `Fetch` exactly as it is today — Task 6 is what turns this into a real multi-tier loop):
```go
func (f *Fetcher) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error) {
	tier, ok := f.tiers[cfg.Use]
	if !ok {
		return nil, "", ErrUnknownTier
	}

	release, err := f.acquire(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()

	retries := max(f.retries, 1)

	var result *tiers.Result
	var kind fault.Kind

	for attempt := 1; attempt <= retries; attempt++ {
		if f.throttle != nil {
			if err := f.throttle.Wait(ctx); err != nil {
				return nil, "", err
			}
		}

		result, kind, err = tier.Fetch(ctx, cfg, url)
		if result != nil {
			result.Tier = cfg.Use
		}

		if err == nil && kind == "" {
			return result, kind, nil
		}
		if attempt == retries {
			return result, kind, err
		}
		if err := f.waitBackoff(ctx, attempt); err != nil {
			return nil, "", err
		}
	}

	return result, kind, err
}
```

`services/core/internal/sink/sinks/sink.go`:
```go
package sinks

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

`services/core/internal/job/workers/scrape.go`, in `Scrape.Run`, add `Tier: result.Tier` to the `sinks.Item{...}` literal:
```go
item := sinks.Item{
	URL:       url,
	FinalURL:  result.FinalURL,
	ParentURL: parentURL,
	Status:    result.Status,
	Body:      string(result.Body),
	Depth:     depth,
	Data:      data,
	Tier:      result.Tier,
}
```

- [ ] **Step 4: Run both tests to verify they pass**

Run: `go test ./services/core/internal/fetch/... ./services/core/internal/job/workers/... -run "TestFetchSetsResultTierToTheConfiguredUseTier|TestScrapeRunCopiesResultTierIntoTheSinkItem" -v`
Expected: PASS

- [ ] **Step 5: Run the full fetch/workers/sink suites**

Run: `go test ./services/core/... -v 2>&1 | tail -80`
Expected: PASS — check in particular `services/core/internal/sink/sinks` (any golden/fixture tests there comparing full `Item` JSON output need the new `"tier":""` key accounted for) and `services/core/internal/integrationtest` (its `wantItem`/assertion helpers in `harness.go` don't check `Tier`, so nothing there should need updating, but confirm the full `services/core` test run is green).

- [ ] **Step 6: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l . && go build ./... && go vet ./...
git add services/core/internal/fetch/tiers/tier.go services/core/internal/fetch/fetch.go services/core/internal/fetch/fetcher_test.go services/core/internal/sink/sinks/sink.go services/core/internal/job/workers/scrape.go services/core/internal/job/workers/scrape_test.go
git commit -m "core: thread a Tier field through the fetch/sink pipeline"
```

---

### Task 5: The ladder escalation algorithm

**Files:**
- Modify: `services/core/internal/fetch/fetch.go`
- Test: `services/core/internal/fetch/fetcher_test.go`

**Interfaces:**
- Consumes: `jobspec.FetchTierOrder`, `jobspec.Fetch.Ladder`/`.Proxy`/`.Name` (Tasks 1-3); `tiers.Result.Tier` (Task 4).
- Produces: `ladderFor(cfg jobspec.Fetch) []string`, `configured(cfg jobspec.Fetch, tier string) bool`, `escalates(k fault.Kind) bool` — package-private to `fetch`, not consumed outside it.
- Reuses the same `fakeTier{results []fakeTierResult, calls int}`/`fakeTierResult{result *tiers.Result, fault fault.Kind, err error}` fakes and the `noBackoff` helper from Task 4/the file's existing tests — `fakeTier.Fetch` already returns one scripted result per call (clamped to the last entry once `results` is exhausted), which is exactly the "distinct sequence per call, with a call counter" shape these tests need. Do not add a second fake type.

- [ ] **Step 1: Write the failing tests**

Add to `services/core/internal/fetch/fetcher_test.go`:

```go
func TestFetchClimbsTheLadderOnBlocked(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{fault: fault.Blocked, err: errors.New("blocked")}}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 1, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Ladder: true, Proxy: "http://p.test"}
	result, kind, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if err != nil || kind != "" {
		t.Fatalf("Fetch() = %v, %q, %v, want success after climbing to proxy", result, kind, err)
	}
	if result.Tier != "proxy" {
		t.Fatalf("result.Tier = %q, want %q", result.Tier, "proxy")
	}
	if local.calls != 1 || proxy.calls != 1 {
		t.Fatalf("local calls = %d, proxy calls = %d, want 1 and 1", local.calls, proxy.calls)
	}
}

func TestFetchStopsOnNonEscalatingFaultKind(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{fault: fault.Auth, err: errors.New("bad credential")}}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 1, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Ladder: true, Proxy: "http://p.test"}
	_, kind, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if kind != fault.Auth || err == nil {
		t.Fatalf("Fetch() kind = %q, err = %v, want fault.Auth and a non-nil error, no escalation", kind, err)
	}
	if proxy.calls != 0 {
		t.Fatalf("proxy.calls = %d, want 0 -- fault.Auth must never escalate", proxy.calls)
	}
}

func TestFetchEachRungGetsItsOwnFullRetryBudget(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{
		{fault: fault.Blocked, err: errors.New("1")},
		{fault: fault.Blocked, err: errors.New("2")},
		{fault: fault.Blocked, err: errors.New("3")},
	}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 3, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Ladder: true, Proxy: "http://p.test"}
	_, _, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if err != nil {
		t.Fatalf("Fetch(): %v", err)
	}
	if local.calls != 3 {
		t.Fatalf("local.calls = %d, want 3 -- retries=3 should give local its own full budget before escalating", local.calls)
	}
	if proxy.calls != 1 {
		t.Fatalf("proxy.calls = %d, want 1", proxy.calls)
	}
}

func TestFetchWithoutLadderRunsExactlyTheOldSingleTierPath(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{fault: fault.Blocked, err: errors.New("blocked")}}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 1, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Proxy: "http://p.test"}
	_, kind, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if kind != fault.Blocked || err == nil {
		t.Fatalf("Fetch() kind = %q, err = %v, want fault.Blocked returned directly -- Ladder is false, must never escalate", kind, err)
	}
	if proxy.calls != 0 {
		t.Fatalf("proxy.calls = %d, want 0 -- Ladder is false", proxy.calls)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./services/core/internal/fetch/... -run "TestFetchClimbsTheLadderOnBlocked|TestFetchStopsOnNonEscalatingFaultKind|TestFetchEachRungGetsItsOwnFullRetryBudget|TestFetchWithoutLadderRunsExactlyTheOldSingleTierPath" -v`
Expected: FAIL — `Fetch` doesn't climb yet.

- [ ] **Step 3: Implement the escalation algorithm**

Replace `Fetcher.Fetch` in `services/core/internal/fetch/fetch.go` entirely (this supersedes Task 4's version), and add the three new helper functions. Add `"slices"` to the import block.

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

func (f *Fetcher) fetchTierWithRetries(ctx context.Context, tier Tier, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error) {
	retries := max(f.retries, 1)

	var result *tiers.Result
	var kind fault.Kind
	var err error

	for attempt := 1; attempt <= retries; attempt++ {
		if f.throttle != nil {
			if err := f.throttle.Wait(ctx); err != nil {
				return nil, "", err
			}
		}

		result, kind, err = tier.Fetch(ctx, cfg, url)

		if err == nil && kind == "" {
			return result, kind, nil
		}
		if attempt == retries {
			return result, kind, err
		}
		if err := f.waitBackoff(ctx, attempt); err != nil {
			return nil, "", err
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

`acquire`/`waitBackoff` are unchanged from today — leave them exactly as they are.

- [ ] **Step 4: Run the new tests, then Task 4's test, to verify they all pass**

Run: `go test ./services/core/internal/fetch/... -v`
Expected: PASS on all of Task 4's and this task's tests.

- [ ] **Step 5: Run the full core suite**

Run: `go test -race ./services/core/... -v 2>&1 | tail -100`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l . && go build ./... && go vet ./...
git add services/core/internal/fetch/fetch.go services/core/internal/fetch/fetcher_test.go
git commit -m "core: climb the fetch ladder on blocked/rate-limited"
```

---

### Task 6: `services/core/build.go` client-wiring bug fix

**Files:**
- Modify: `services/core/build.go`
- Test: `services/core/build_internal_test.go` (new file, `package core` — matches the existing internal-test naming convention already used elsewhere in this repo, e.g. `pkg/provider/template_internal_test.go`)

**Interfaces:**
- Produces: `buildFetchClients(fetch jobspec.Fetch) (local, proxy *http.Client, err error)` — unexported, package-private to `core`, called from `(*liveExecutor).createFetcher`.

- [ ] **Step 1: Write the failing test**

`services/core/build_internal_test.go`:
```go
package core

import (
	"net/http"
	"testing"

	"github.com/pobear/pobear/pkg/jobspec"
)

func TestBuildFetchClientsFingerprintsProxyEvenWhenUseIsLocal(t *testing.T) {
	fetch := jobspec.Fetch{Use: "local", Ladder: true, Profile: "chrome", Proxy: "http://p.test"}

	_, proxyClient, err := buildFetchClients(fetch)
	if err != nil {
		t.Fatalf("buildFetchClients: %v", err)
	}
	if proxyClient == http.DefaultClient {
		t.Fatal("proxyClient is http.DefaultClient -- a laddered proxy rung must get its own fingerprinted client, not the bare default")
	}
}

func TestBuildFetchClientsLeavesProxyAtDefaultWhenNotConfigured(t *testing.T) {
	fetch := jobspec.Fetch{Use: "local", Profile: "chrome"}

	_, proxyClient, err := buildFetchClients(fetch)
	if err != nil {
		t.Fatalf("buildFetchClients: %v", err)
	}
	if proxyClient != http.DefaultClient {
		t.Fatal("proxyClient should stay http.DefaultClient when no proxy is configured at all")
	}
}

func TestBuildFetchClientsAlwaysFingerprintsLocal(t *testing.T) {
	fetch := jobspec.Fetch{Use: "local", Profile: "chrome"}

	localClient, _, err := buildFetchClients(fetch)
	if err != nil {
		t.Fatalf("buildFetchClients: %v", err)
	}
	if localClient == http.DefaultClient {
		t.Fatal("localClient is http.DefaultClient, want a fingerprinted client")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./services/core/... -run TestBuildFetchClients -v`
Expected: FAIL to compile — `buildFetchClients` doesn't exist yet.

- [ ] **Step 3: Extract the helper and fix `createFetcher`**

In `services/core/build.go`, add the new function and change `createFetcher` to call it instead of its current `switch spec.Fetch.Use`:

```go
func buildFetchClients(fetch jobspec.Fetch) (localClient, proxyClient *http.Client, err error) {
	localClient, err = tiers.NewFingerprintedClient(fetch.Profile, "")
	if err != nil {
		return nil, nil, err
	}

	proxyClient = http.DefaultClient
	if fetch.Proxy != "" {
		proxyClient, err = tiers.NewFingerprintedClient(fetch.Profile, fetch.Proxy)
		if err != nil {
			return nil, nil, err
		}
	}
	return localClient, proxyClient, nil
}
```

Replace the body of `(e *liveExecutor) createFetcher` from:
```go
	localClient := http.DefaultClient
	proxyClient := http.DefaultClient

	switch spec.Fetch.Use {
	case "local":
		client, err := tiers.NewFingerprintedClient(spec.Fetch.Profile, "")
		if err != nil {
			return nil, err
		}
		localClient = client
	case "proxy":
		client, err := tiers.NewFingerprintedClient(spec.Fetch.Profile, spec.Fetch.Proxy)
		if err != nil {
			return nil, err
		}
		proxyClient = client
	}
```
to:
```go
	localClient, proxyClient, err := buildFetchClients(spec.Fetch)
	if err != nil {
		return nil, err
	}
```

`localClient`/`proxyClient` are then used exactly as before, further down in the same function, in the `tierMap` literal.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./services/core/... -run TestBuildFetchClients -v`
Expected: PASS

- [ ] **Step 5: Run the full core suite**

Run: `go test -race ./services/core/... -v 2>&1 | tail -100`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l . && go build ./... && go vet ./...
git add services/core/build.go services/core/build_internal_test.go
git commit -m "core: fix escalated-to proxy tier silently running unfingerprinted"
```

---

### Task 7: CLI `--ladder` flag, validation, wiring

**Files:**
- Modify: `services/cli/internal/command/flags.go` (`commonFlags`, `registerCommonFlags`, new `resolveFetchUse`)
- Modify: `services/cli/internal/command/scrape.go`, `services/cli/internal/command/crawl.go` (use the new helper instead of their own inline `switch`)
- Modify: `services/cli/internal/command/spec.go` (`verboseFetch`, `printJSON`)
- Modify: `services/cli/internal/jobgen/jobgen.go` (`Input`, `buildFetchDoc`, `minimalFetch`, `minimalFetchFrom`)
- Test: `cmd/core/testdata/scrape/` and `cmd/core/testdata/crawl/` (new clitest fixtures), `services/cli/internal/jobgen/jobgen_test.go` (existing file — add cases)

**Interfaces:**
- Consumes: nothing new from earlier tasks (this task only touches the CLI/jobgen layer that assembles a `jobspec.Spec` via YAML text, which `jobspec.Parse` — Tasks 1-3 — then validates).
- Produces: `resolveFetchUse(f *commonFlags) (string, *cliError)`; `jobgen.Input.Ladder bool`.

- [ ] **Step 1: Add `Ladder` to `jobgen.Input` and wire it into spec construction**

In `services/cli/internal/jobgen/jobgen.go`:

Add to `Input`:
```go
type Input struct {
	URLs        []string
	Crawl       *CrawlInput
	FetchUse    string
	Ladder      bool
	Proxy       string
	Provider    string
	Template    string
	Profile     string
	Options     map[string]any
	Timeout     string
	NoRedirects bool
	Stages      []jobspec.Stage
	Concurrency *int
	RPS         *int
	Retries     *int
	SinkUse     string
	SinkPath    string
}
```

In `buildFetchDoc`, add right after the `"use"` key is set:
```go
func buildFetchDoc(in Input) map[string]any {
	fetch := map[string]any{"use": in.FetchUse}
	if in.Ladder {
		fetch["ladder"] = in.Ladder
	}
	if in.Provider != "" {
		fetch["name"] = in.Provider
	}
	...
```
(leave the rest of the function exactly as it is — this is the only new line.)

For `--print-spec`'s minimal-diff output, add `Ladder` to `minimalFetch` and its `diffBool` line in `minimalFetchFrom`:
```go
type minimalFetch struct {
	Use         string            `json:"use"`
	Ladder      *bool             `json:"ladder,omitempty"`
	Name        *string           `json:"name,omitempty"`
	Template    *string           `json:"template,omitempty"`
	Proxy       *string           `json:"proxy,omitempty"`
	Profile     *string           `json:"profile,omitempty"`
	Options     map[string]any    `json:"options,omitempty"`
	Timeout     *string           `json:"timeout,omitempty"`
	NoRedirects *bool             `json:"noRedirects,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

func minimalFetchFrom(r, d jobspec.Fetch) minimalFetch {
	profile := diffString(r.Profile, d.Profile)
	if r.Use == "provider" {
		profile = nil
	}
	return minimalFetch{
		Use:         r.Use,
		Ladder:      diffBool(r.Ladder, d.Ladder),
		Name:        diffString(r.Name, d.Name),
		Template:    diffString(r.Template, d.Template),
		Proxy:       diffString(r.Proxy, d.Proxy),
		Profile:     profile,
		Options:     diffMap(r.Options, d.Options),
		Timeout:     diffString(r.Timeout, d.Timeout),
		NoRedirects: diffBool(r.NoRedirects, d.NoRedirects),
		Headers:     diffHeaders(r.Headers, d.Headers),
	}
}
```

- [ ] **Step 2: Write a failing jobgen test**

Read `services/cli/internal/jobgen/jobgen_test.go` first to match its existing style, then add:
```go
func TestBuildFetchDocIncludesLadderWhenTrue(t *testing.T) {
	spec, err := jobgen.Build(jobgen.Input{URLs: []string{"https://a.test"}, FetchUse: "local", Ladder: true, Provider: "acme", Template: "t"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !spec.Fetch.Ladder {
		t.Fatal("spec.Fetch.Ladder = false, want true")
	}
}
```
(Note: this test needs a manifest-free path or should use a `use`/`name` combo that parses cleanly under Task 2's rules — `local` + `ladder: true` + `name: acme` is valid per `ladder-local-to-provider-valid.yaml` from Task 2, so this should parse fine through `jobspec.Parse`.)

Run: `go test ./services/cli/internal/jobgen/... -run TestBuildFetchDocIncludesLadderWhenTrue -v`
Expected: FAIL — `Input` has no `Ladder` field yet if you run this before Step 1, or PASS immediately after Step 1 (Step 1's code already makes this pass — this is a confirmation step, not new red-green cycle, since Step 1's change is small and mechanical).

- [ ] **Step 3: Run it to confirm it passes after Step 1**

Run: `go test ./services/cli/internal/jobgen/... -v`
Expected: PASS across the whole package.

- [ ] **Step 4: Add `--ladder` and the shared `resolveFetchUse` helper**

In `services/cli/internal/command/flags.go`, add `ladder bool` to `commonFlags` and register the flag:
```go
type commonFlags struct {
	proxy            string
	provider         string
	template         string
	profile          string
	render           bool
	geo              string
	opts             []string
	timeout          string
	extract          string
	css              []string
	out              string
	bare             bool
	ladder           bool
	printSpec        bool
	printSpecVerbose bool
	failOnEmpty      bool
	concurrency      int
	retries          int
	rps              int
	wasSet           func(name string) bool
}
```
```go
	flags.BoolVar(&f.ladder, "ladder", false, "escalate to a higher configured tier (proxy, then provider) when blocked or rate-limited")
```
(add this line to `registerCommonFlags`, anywhere alongside the other `BoolVar` calls — e.g. right after the `--bare` registration.)

Add the new shared resolver, right after `registerCommonFlags`:
```go
func resolveFetchUse(f *commonFlags) (string, *cliError) {
	if f.proxy != "" && f.provider != "" && !f.ladder {
		return "", newCLIError(exitcode.UsageError, "both --proxy and --provider are set -- add --ladder to escalate between them, or remove one")
	}
	if f.ladder && f.proxy == "" && f.provider == "" {
		return "", newCLIError(exitcode.UsageError, "--ladder requires --proxy and/or --provider to be set")
	}

	fetchUse := "local"
	switch {
	case f.proxy != "":
		fetchUse = "proxy"
	case f.provider != "":
		fetchUse = "provider"
	}
	return fetchUse, nil
}
```

(`exitcode` is already imported in `flags.go`.)

- [ ] **Step 5: Wire `scrapeRun`/`crawlRun` to use it**

In `services/cli/internal/command/scrape.go`, replace:
```go
	fetchUse := "local"
	switch {
	case f.provider != "":
		fetchUse = "provider"
	case f.proxy != "":
		fetchUse = "proxy"
	}
```
with:
```go
	fetchUse, cerr := resolveFetchUse(f)
	if cerr != nil {
		return 0, cerr
	}
```
(`cerr` is already declared earlier in `scrapeRun` via `:=` at the top from `resolveScrapeURLs` — since it's redeclared here with `:=` again in the same scope alongside the new `fetchUse`, Go allows this as long as at least one variable on the left is new, which `fetchUse` is.)

Then add `Ladder: f.ladder` to the `jobgen.Input{...}` literal further down:
```go
	resolvedIn := jobgen.Input{
		URLs: urls, FetchUse: fetchUse, Ladder: f.ladder, Proxy: f.proxy, Provider: f.provider, Template: f.template,
		Profile: profile, Options: fetchOptions, Timeout: f.timeout, Stages: stages,
		Concurrency: &f.concurrency, RPS: &f.rps, Retries: &f.retries, SinkUse: sinkUse, SinkPath: sinkPath,
	}
```

Do the identical two edits in `services/cli/internal/command/crawl.go`'s `crawlRun` (same old `switch` block shape, same `jobgen.Input{...}` literal shape — just add `Ladder: f.ladder` to it too).

- [ ] **Step 6: Wire `--print-spec-verbose`'s output**

In `services/cli/internal/command/spec.go`, add `Ladder` to `verboseFetch` and `printJSON`:
```go
type verboseFetch struct {
	Use         string            `json:"use"`
	Ladder      bool              `json:"ladder"`
	Name        string            `json:"name,omitempty"`
	Template    string            `json:"template,omitempty"`
	Proxy       string            `json:"proxy,omitempty"`
	Profile     string            `json:"profile,omitempty"`
	Options     map[string]any    `json:"options,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	NoRedirects bool              `json:"noRedirects"`
	Headers     map[string]string `json:"headers,omitempty"`
}
```
```go
		Fetch: verboseFetch{
			Use: spec.Fetch.Use, Ladder: spec.Fetch.Ladder, Name: spec.Fetch.Name, Template: spec.Fetch.Template,
			Proxy: spec.Fetch.Proxy, Profile: spec.Fetch.Profile, Options: spec.Fetch.Options,
			Timeout: spec.Fetch.Timeout, NoRedirects: spec.Fetch.NoRedirects, Headers: spec.Fetch.Headers,
		},
```

- [ ] **Step 7: Write failing CLI-level fixtures**

Read `cmd/core/testdata/scrape/` and `cmd/core/testdata/crawl/` and the `clitest` harness (`internal/testutil/clitest/clitest.go`) first to match the existing fixture format exactly. Fixtures support an optional `providers:` map (filename → inline YAML content) that `clitest` writes to a temp `$POBEAR_PROVIDERS_DIR` before running that one case — see `cmd/core/testdata/scrape/provider-option-unknown.yaml` for the exact shape. `exitcode.UsageError` is `3` (confirmed by reading `pkg/exitcode/exitcode.go` — every fixture below uses `exit: 3` for a usage error, not `2`). Add:

`cmd/core/testdata/scrape/ladder-requires-proxy-or-provider.yaml`:
```yaml
args: scrape {{base}}/x --ladder
stdout: ""
stderr: |
  pobear: --ladder requires --proxy and/or --provider to be set
exit: 3
```

`cmd/core/testdata/scrape/proxy-and-provider-without-ladder-errors.yaml` (the fake `acme` provider name is never actually looked up — `resolveFetchUse`'s error fires before `scrapeRun` ever reaches provider-manifest resolution, so no `providers:` block is needed here):
```yaml
args: scrape {{base}}/x --proxy http://p.test --provider acme
stdout: ""
stderr: |
  pobear: both --proxy and --provider are set -- add --ladder to escalate between them, or remove one
exit: 3
```

`cmd/core/testdata/scrape/ladder-print-spec-verbose.yaml` (this one DOES reach provider resolution, since `--print-spec-verbose` only skips the credential-existence check, not manifest loading — so `acme` needs to be a real, loadable manifest via the `providers:` block, matching `provider-option-unknown.yaml`'s established pattern):
```yaml
args: scrape {{base}}/x --proxy http://p.test --provider acme --ladder --print-spec-verbose
stdout: |
    {
      "version": 1,
      "source": {
        "urls": [
          "{{base}}/x"
        ]
      },
      "fetch": {
        "use": "proxy",
        "ladder": true,
        "name": "acme",
        "proxy": "http://p.test",
        "profile": "chrome",
        "timeout": "30s",
        "noRedirects": false
      },
      "limits": {
        "concurrency": 8,
        "rps": 0,
        "retries": 3,
        "backoff": "exponential",
        "pluginConcurrency": 0,
        "pluginTimeout": "30s"
      },
      "stages": [],
      "sink": {
        "use": "stdout"
      }
    }
stderr: ""
exit: 0
providers:
    acme.yaml: |
        name: acme
        endpoint: https://acme.example.com/scrape
        auth: {type: bearer, credential: token}
        request: {body: {url: "{{ .URL }}"}}
        response: {body: "$.html"}
```

(No `options:` declared on `acme` deliberately, so `fetch.options` resolves to empty and is omitted from the JSON via its `omitempty` tag — matching the expected output above, which has no `"options"` key. Still cross-check the exact JSON indentation/whitespace against a real neighboring `--print-spec-verbose` fixture in the same directory once you run this — `clitest`'s exact formatting conventions are easiest to confirm by looking at one you can already see passing.)

- [ ] **Step 8: Run the CLI test suite to verify the new fixtures fail, then pass**

Run: `go test ./cmd/core/... -run TestCLI -v 2>&1 | grep -A5 "ladder\|proxy-and-provider"`
Expected: FAIL initially (before Steps 4-6's code exists — if you're doing this task in order, Steps 4-6 already landed, so this may already PASS; if so, that's fine, it confirms the implementation is correct — the point of writing the fixture is the regression coverage it leaves behind, not necessarily watching it fail first when the surrounding implementation steps already happened earlier in this same task).

- [ ] **Step 9: Run the full CLI suite and full repo test/lint gate**

Run:
```bash
cd /Users/adomas.skliutas/code/pobear
go test ./cmd/core/... ./services/cli/... -v 2>&1 | tail -150
gofmt -l .
go build ./...
go vet ./...
make lint
go test -race -count=1 ./...
```
Expected: everything green.

- [ ] **Step 10: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
git add services/cli/internal/command/flags.go services/cli/internal/command/scrape.go services/cli/internal/command/crawl.go services/cli/internal/command/spec.go services/cli/internal/jobgen/jobgen.go services/cli/internal/jobgen/jobgen_test.go cmd/core/testdata/scrape/ cmd/core/testdata/crawl/
git commit -m "cli: add --ladder flag, validation, and print-spec wiring"
```

---

### Task 8: Wizard — `fetchConfig` restructuring and the tier sub-screen

This is the first of three TUI tasks. This one introduces the new state shape and the reusable sub-screen; Task 9 wires it into the main Scrape/Crawl forms; Task 10 wires job-spec construction and result display.

**Files:**
- Modify: `services/cli/internal/wizard/form.go` (`fetchConfig`, `newFetchConfig`, delete `fetchFields`, add per-tier field-list functions)
- Create: `services/cli/internal/wizard/tier_screen.go`
- Test: `services/cli/internal/wizard/tier_screen_test.go` (new)

**Interfaces:**
- Produces:
  - `fetchConfig` gains `localConfigured bool`; loses `fetchVia string`.
  - `localTierFields(c fetchConfig) []field`, `proxyTierFields(c fetchConfig) []field`, `providerTierFields(c fetchConfig, manifests []*provider.Manifest) []field` — replace the old `fetchFields`'s switch branches.
  - `newTierScreen(tier string, fc *fetchConfig, manifests []*provider.Manifest, onDone func()) *tierScreen` — pushed via `pushScreen`, pops itself via `popScreen`; mutates `*fc` in place as the user edits (so the caller's own state is already updated by the time it pops — no message-passing hand-back needed), then calls `onDone()` synchronously before returning the pop command.
- Consumes: existing `field`/`textField`/`choiceField`/`providerAndOptionFields`/`syncProviderOrTemplateChanged` (`form.go`), `renderForm` (`layout.go`), `pushScreen`/`popScreen` (`app.go`).

- [ ] **Step 1: Update `fetchConfig` and extract the per-tier field lists**

In `services/cli/internal/wizard/form.go`, change:
```go
type fetchConfig struct {
	fetchVia        string
	profile         string
	proxyURL        string
	providerName    string
	templateName    string
	templateQuery   string
	providerOptions map[string]string
}

func newFetchConfig() fetchConfig {
	return fetchConfig{fetchVia: "local", profile: "chrome"}
}
```
to:
```go
type fetchConfig struct {
	localConfigured bool
	profile         string
	proxyURL        string
	providerName    string
	templateName    string
	templateQuery   string
	providerOptions map[string]string
}

func newFetchConfig() fetchConfig {
	return fetchConfig{profile: "chrome"}
}
```

Delete `fetchFields` entirely (its old switch-on-`fetchVia` body) and replace it with three standalone functions:
```go
func localTierFields(c fetchConfig) []field {
	return []field{choiceField("profile", "Profile", "(TLS fingerprint)", []string{"chrome", "firefox", "safari"}, c.profile)}
}

func proxyTierFields(c fetchConfig) []field {
	return []field{
		textField("proxyURL", "Proxy URL", "", c.proxyURL),
		choiceField("profile", "Profile", "(TLS fingerprint)", []string{"chrome", "firefox", "safari"}, c.profile),
	}
}

func providerTierFields(c fetchConfig, manifests []*provider.Manifest) []field {
	return providerAndOptionFields(c, manifests)
}
```

Leave `providerAndOptionFields`, `providerFields`, `optionFields`, `syncProviderOrTemplateChanged`, and everything else in `form.go` untouched — they're reused as-is by `providerTierFields`.

This is an intentionally breaking change to `form.go`'s public-to-the-package surface: `computeFields`/`computeCrawlFields` (in `scraper_form.go`/`crawl_form.go`) still call the now-deleted `fetchFields` — **this will not compile until Task 9**. That's expected; Task 9 is the very next task and fixes it. Do not attempt to make the package compile at the end of this task — run only the specific new test below, scoped to the one new file, which doesn't depend on the rest of the package compiling... except it does, since it's the same Go package. **Given that, do Task 8 and Task 9 as one combined commit** — write this task's code, then immediately continue into Task 9's changes before running `go build`/tests/committing. The two-task split above is for planning clarity (state design vs. wiring), not a real compile-clean boundary.

- [ ] **Step 2: Create the tier sub-screen**

`services/cli/internal/wizard/tier_screen.go`:
```go
package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/provider"
)

const (
	tierScreenBackHint = "esc: back"
)

func tierScreenTitle(tier string) string {
	switch tier {
	case "local":
		return "Local"
	case "proxy":
		return "Proxy"
	default:
		return "Provider"
	}
}

func tierScreenFields(tier string, fc fetchConfig, manifests []*provider.Manifest) []field {
	switch tier {
	case "local":
		return localTierFields(fc)
	case "proxy":
		return proxyTierFields(fc)
	default:
		return providerTierFields(fc, manifests)
	}
}

type tierScreen struct {
	tier      string
	fc        *fetchConfig
	manifests []*provider.Manifest
	onDone    func()

	fields        []field
	cursor        int
	width, height int
}

func newTierScreen(tier string, fc *fetchConfig, manifests []*provider.Manifest, onDone func()) *tierScreen {
	s := &tierScreen{tier: tier, fc: fc, manifests: manifests, onDone: onDone}
	s.rebuild()
	return s
}

func (s *tierScreen) rebuild() {
	focusedName := ""
	if len(s.fields) > 0 && s.cursor < len(s.fields) {
		focusedName = s.fields[s.cursor].name
	}

	s.fields = tierScreenFields(s.tier, *s.fc, s.manifests)

	newCursor := 0
	for i, fld := range s.fields {
		if fld.name == focusedName {
			newCursor = i
			break
		}
	}
	s.cursor = newCursor
	s.refocus()
}

func (s *tierScreen) refocus() {
	for i := range s.fields {
		if i == s.cursor {
			s.fields[i] = s.fields[i].focus()
		} else {
			s.fields[i] = s.fields[i].blur()
		}
	}
}

func (s *tierScreen) syncToConfig() {
	if s.tier == "local" {
		s.fc.localConfigured = true
	}
	skipOptions := s.tier == "provider" && syncProviderOrTemplateChanged(s.fields, s.fc)

	for _, fld := range s.fields {
		switch {
		case fld.name == "profile":
			s.fc.profile = fld.value()
		case fld.name == "proxyURL":
			s.fc.proxyURL = fld.value()
		case fld.name == "provider" || fld.name == "template":
		case strings.HasPrefix(fld.name, "opt."):
			if skipOptions {
				continue
			}
			if s.fc.providerOptions == nil {
				s.fc.providerOptions = map[string]string{}
			}
			s.fc.providerOptions[strings.TrimPrefix(fld.name, "opt.")] = fld.value()
		}
	}
}

func (s *tierScreen) Init() tea.Cmd { return nil }

func (s *tierScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.syncToConfig()
			s.onDone()
			return s, popScreen()
		case "up", "down":
			if s.cursor < len(s.fields) && s.fields[s.cursor].capturesVerticalNav(msg.String()) {
				current := s.fields[s.cursor]
				updated, changed := current.update(msg)
				s.fields[s.cursor] = updated
				if changed && current.isStructural() {
					s.syncToConfig()
					s.rebuild()
				}
				return s, nil
			}
			s.syncToConfig()
			if msg.String() == "up" {
				s.cursor = (s.cursor - 1 + len(s.fields)) % len(s.fields)
			} else {
				s.cursor = (s.cursor + 1) % len(s.fields)
			}
			s.refocus()
			return s, nil
		case "tab":
			s.syncToConfig()
			s.cursor = (s.cursor + 1) % len(s.fields)
			s.refocus()
			return s, nil
		}

		if s.cursor < len(s.fields) {
			current := s.fields[s.cursor]
			updated, changed := current.update(msg)
			s.fields[s.cursor] = updated
			if changed && current.isStructural() {
				s.syncToConfig()
				s.rebuild()
			}
		}
		return s, nil
	}
	return s, nil
}

func (s *tierScreen) View() string {
	return renderForm(s.width, s.height, tierScreenTitle(s.tier), s.fields, s.cursor, "")
}
```

Note `syncToConfig` unconditionally sets `localConfigured = true` for the local tier every time `esc` is pressed on it — there is intentionally no "clear this tier" action in this first version (matches the spec's explicitly-listed non-goal: "a way to configure a tier but exclude it from a particular run's ladder without un-configuring it" was out of scope; an "unconfigure" action for Proxy/Provider is a separate, reasonable follow-up but is not in this plan — Proxy/Provider are already "unconfigured" by construction whenever their URL/name field is blank, so those two need no explicit clear action at all; only Local's `localConfigured` bool has no natural "blank" state to fall back to, since Local's own field, Profile, always has a valid value).

- [ ] **Step 3: Write a test for the sub-screen (do this once Task 9 has made the package compile again — see the note in Step 1)**

`services/cli/internal/wizard/tier_screen_test.go`:
```go
package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTierScreenLocalSetsConfiguredOnEsc(t *testing.T) {
	fc := &fetchConfig{profile: "chrome"}
	done := false
	s := newTierScreen("local", fc, nil, func() { done = true })
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected esc to return a pop command, got nil")
	}
	if _, ok := updated.(*tierScreen); !ok {
		t.Fatalf("expected Update to return the same *tierScreen, got %T", updated)
	}
	if !fc.localConfigured {
		t.Fatal("fc.localConfigured = false after opening and leaving the Local screen, want true")
	}
	if !done {
		t.Fatal("onDone was not called")
	}
}

func TestTierScreenProxySyncsURLIntoSharedConfig(t *testing.T) {
	fc := &fetchConfig{profile: "chrome"}
	s := newTierScreen("proxy", fc, nil, func() {})
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	for _, r := range "http://p.test" {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	s.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if fc.proxyURL != "http://p.test" {
		t.Fatalf("fc.proxyURL = %q, want %q", fc.proxyURL, "http://p.test")
	}
	if fc.localConfigured {
		t.Fatal("fc.localConfigured = true after only ever opening the Proxy screen, want false")
	}
}

func TestTierScreenViewRendersItsTitle(t *testing.T) {
	fc := &fetchConfig{profile: "chrome"}
	s := newTierScreen("proxy", fc, nil, func() {})
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if !strings.Contains(s.View(), "Proxy") {
		t.Fatalf("View() = %q, want it to contain the tier title \"Proxy\"", s.View())
	}
}
```

- [ ] **Step 4: Run the new test (only meaningful once Task 9 makes the package compile again)**

Run: `go test ./services/cli/internal/wizard/... -run TestTierScreen -v`
Expected: after Task 9 is also done, PASS.

- [ ] **Step 5: Do not commit yet** — continue straight into Task 9; they land as one commit (see the compile-boundary note in Step 1).

---

### Task 9: Wizard — three tier rows replace `Fetch via` in the main forms

**Files:**
- Modify: `services/cli/internal/wizard/scraper_form.go` (`computeFields`, `scrapeForm.Update`'s `"enter"` handling)
- Modify: `services/cli/internal/wizard/crawl_form.go` (`computeCrawlFields`, `crawlForm.Update`'s `"enter"` handling)
- Test: `services/cli/internal/wizard/scraper_form_test.go`, `services/cli/internal/wizard/crawl_form_test.go` (existing files — add cases)

**Interfaces:**
- Consumes: `localTierFields`/`proxyTierFields`/`providerTierFields`, `newTierScreen` (Task 8).
- Produces: three new field names on the main form, `"localTier"`, `"proxyTier"`, `"providerTier"` — `fieldChoice`-kind rows whose `label` is a live summary (`✓ chrome` / `— not set`), pressing `enter` on one pushes a `tierScreen`.

- [ ] **Step 1: Add the summary-row builder to `form.go`**

Add to `services/cli/internal/wizard/form.go`, near the other `fetchConfig`-related helpers:
```go
func tierRowField(name, label string, configured bool, summary string) field {
	value := "— not set"
	if configured {
		value = "✓ " + summary
	}
	return field{name: name, label: label, hint: "(enter to configure)", kind: fieldChoice, options: []string{value}, choice: 0}
}

func fetchTierRows(c fetchConfig) []field {
	providerSummary := c.providerName
	if providerSummary != "" && c.templateName != "" {
		providerSummary += " / " + c.templateName
	}
	return []field{
		tierRowField("localTier", "Local", c.localConfigured, c.profile),
		tierRowField("proxyTier", "Proxy", c.proxyURL != "", c.proxyURL),
		tierRowField("providerTier", "Provider", c.providerName != "", providerSummary),
	}
}
```

`field{... kind: fieldChoice, options: []string{value} ...}` is a one-option choice field, the same pattern the existing `saveActionName`/`runActionName`/`addFieldName` action rows already use (`field{name: saveActionName, label: "[ Save as job.yaml ]", kind: fieldChoice, options: []string{""}}` in `scraper_form.go`) — it renders as a normal labeled row with no cycling behavior a user could accidentally trigger with left/right, since there's only one option.

- [ ] **Step 2: Wire it into `computeFields` (Scrape)**

In `services/cli/internal/wizard/scraper_form.go`, change:
```go
func computeFields(s formState, manifests []*provider.Manifest) []field {
	fields := []field{textAreaField("URL", "(one per line)", s.url, urlAreaHeight, urlAreaWidth)}
	fields = append(fields, fetchFields(s.fetchConfig, manifests)...)
	fields = append(fields, extractFields(s.extractConfig)...)
	...
```
to:
```go
func computeFields(s formState, manifests []*provider.Manifest) []field {
	fields := []field{textAreaField("URL", "(one per line)", s.url, urlAreaHeight, urlAreaWidth)}
	fields = append(fields, fetchTierRows(s.fetchConfig)...)
	fields = append(fields, extractFields(s.extractConfig)...)
	...
```
(everything after that — `concurrency`, `retries`, the `saveActionName`/`runActionName` rows — is unchanged.)

Do the identical one-line swap in `services/cli/internal/wizard/crawl_form.go`'s `computeCrawlFields` (`fetchFields(s.fetchConfig, manifests)` → `fetchTierRows(s.fetchConfig)`).

- [ ] **Step 3: Handle `enter` on a tier row**

`scrapeForm.syncStateFromFields` currently has a case for `fld.name == "fetchVia"` — delete it (the field no longer exists). Its `case fld.name == "provider" || fld.name == "template":` line also goes, since those field names now only ever appear *inside* a `tierScreen`, never in the main form's own `f.fields` — the main form has no need to read them anymore.

In `scrapeForm.Update`'s `tea.KeyMsg` handling, in the block that already special-cases `msg.String() == "enter"` for `addFieldName`/`saveActionName`/`runActionName`, add three more cases:
```go
		if f.cursor < len(f.fields) && msg.String() == "enter" {
			switch f.fields[f.cursor].name {
			case "localTier":
				f.syncStateFromFields()
				return f, pushScreen(newTierScreen("local", &f.state.fetchConfig, f.providerManifests, f.rebuild))
			case "proxyTier":
				f.syncStateFromFields()
				return f, pushScreen(newTierScreen("proxy", &f.state.fetchConfig, f.providerManifests, f.rebuild))
			case "providerTier":
				f.syncStateFromFields()
				return f, pushScreen(newTierScreen("provider", &f.state.fetchConfig, f.providerManifests, f.rebuild))
			case addFieldName:
				...
```
(keep the existing `addFieldName`/`saveActionName`/`runActionName` cases exactly as they are, just add the three new ones above them in the same `switch`.)

`f.rebuild` — the form's own existing method — is passed directly as the `onDone` callback: when the `tierScreen` pops (via `esc`), it has already mutated `f.state.fetchConfig` in place (same struct, addressed via `&f.state.fetchConfig`), and calling `f.rebuild()` recomputes `f.fields` from that now-current state so the row's summary text (`✓ chrome` etc.) is fresh the instant control returns — no bubbletea message-ordering dependency, since this all happens synchronously inside `tierScreen.Update`'s `"esc"` case before it returns.

Do the identical change in `crawlForm.Update` (same three new `case` branches, same `f.rebuild` callback, same deletions from `syncStateFromFields`).

- [ ] **Step 4: Now the whole package compiles — build it**

Run:
```bash
cd /Users/adomas.skliutas/code/pobear
go build ./...
```
Expected: clean build. This is the point where Task 8's `tier_screen.go` (which couldn't compile on its own, since `computeFields`/`computeCrawlFields` still called the now-deleted `fetchFields` until this step) becomes part of a compiling package.

- [ ] **Step 5: Run Task 8's test now that the package compiles**

Run: `go test ./services/cli/internal/wizard/... -run TestTierScreen -v`
Expected: PASS.

- [ ] **Step 6: Update the existing form tests that referenced `fetchVia`/`fetchFields`**

Read `services/cli/internal/wizard/scraper_form_test.go` and `services/cli/internal/wizard/crawl_form_test.go` in full — every test that sets `s.fetchVia = "proxy"` or asserts on a `"fetchVia"` field name needs rewriting to instead: construct a `formState`/`crawlFormState` with `fetchConfig{localConfigured: true}` / `fetchConfig{proxyURL: "..."}` / `fetchConfig{providerName: "..."}` directly (bypassing the old chooser-field simulation entirely, since there's no `"fetchVia"` field left to set), and any test asserting on the presence/values of `"profile"`/`"proxyURL"`/provider option fields *in the main form's field list* needs to instead assert on `"localTier"`/`"proxyTier"`/`"providerTier"`'s row `.value()` (the `✓ .../— not set` string) — those per-tier fields only exist inside a pushed `tierScreen` now, not in the main form.

Add new tests replacing anything that specifically exercised `fetchVia` switching behavior:
```go
func TestComputeFieldsShowsThreeTierRows(t *testing.T) {
	fields := computeFields(newFormState(), nil)
	names := fieldNames(fields)
	for _, want := range []string{"localTier", "proxyTier", "providerTier"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("computeFields() fields = %v, want %q among them", names, want)
		}
	}
}

func TestComputeFieldsTierRowsStartUnconfigured(t *testing.T) {
	fields := computeFields(newFormState(), nil)
	for _, name := range []string{"localTier", "proxyTier", "providerTier"} {
		fld, ok := findField(fields, name)
		if !ok {
			t.Fatalf("expected a %q field", name)
		}
		if fld.value() != "— not set" {
			t.Errorf("%s.value() = %q, want %q", name, fld.value(), "— not set")
		}
	}
}

func TestEnterOnLocalTierRowPushesTierScreen(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	updated, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f = updated.(*scrapeForm)
	if cmd == nil {
		t.Fatal("expected enter on the first field (localTier, cursor starts at 0) to return a push command")
	}
	msg := cmd()
	push, ok := msg.(pushScreenMsg)
	if !ok {
		t.Fatalf("expected a pushScreenMsg, got %T", msg)
	}
	if _, ok := push.screen.(*tierScreen); !ok {
		t.Fatalf("expected the pushed screen to be *tierScreen, got %T", push.screen)
	}
}
```

(`fieldNames`/`findField` are existing test helpers already used elsewhere in this package's tests — reuse them, don't redefine them. If either doesn't already exist under those exact names, search the test files for the equivalent helper that's actually there and use that instead.)

Add the mirrored three tests for `crawl_form_test.go` (`computeCrawlFields`, `newCrawlForm`).

- [ ] **Step 7: Run the full wizard test suite**

Run: `go test ./services/cli/internal/wizard/... -v 2>&1 | tail -150`
Expected: every test passes. Fix any remaining test that still references the deleted `fetchVia`/`fetchFields` — there is likely more than one, since `fetchVia` was a widely-referenced field name throughout this package's existing tests (form tests, actions tests). Task 10 covers `scraper_actions.go`/`crawl_actions.go`'s own `fetchVia` usage specifically — if a test failure traces back to those files rather than the form files this task touches, leave it for Task 10 and note it, don't fix it here.

- [ ] **Step 8: Regenerate affected screenshot golden files**

Run: `go test ./services/cli/internal/wizard/... -run "TestScrapeFormScreenshotScenarios|TestCrawlFormScreenshotScenarios" -update -v`
Then: `git diff services/cli/internal/wizard/testdata/` and review the diff by eye — it should show the old `Fetch via` / `Profile` / `Provider` / etc. rows replaced by exactly three new `Local` / `Proxy` / `Provider` rows, each showing `— not set` (or, for any scenario fixture that pre-seeded a provider/proxy, `✓ ...`), and nothing else in the rendered box should differ. If the diff shows anything beyond that swap, stop and investigate before proceeding — do not blindly accept an `-update` diff you haven't read.

- [ ] **Step 9: Commit Tasks 8 and 9 together**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l . && go build ./... && go vet ./...
git add services/cli/internal/wizard/form.go services/cli/internal/wizard/tier_screen.go services/cli/internal/wizard/tier_screen_test.go services/cli/internal/wizard/scraper_form.go services/cli/internal/wizard/crawl_form.go services/cli/internal/wizard/scraper_form_test.go services/cli/internal/wizard/crawl_form_test.go services/cli/internal/wizard/testdata/
git commit -m "wizard: replace Fetch via with three tier rows and sub-screens"
```

---

### Task 10: Wizard — job-spec construction (ladder/use derivation) and result display

**Files:**
- Modify: `services/cli/internal/wizard/scraper_actions.go` (`buildJobSpec`)
- Modify: `services/cli/internal/wizard/crawl_actions.go` (`buildCrawlJobSpec`)
- Modify: `services/cli/internal/wizard/form.go` (new `ladderTiers`/`resolveFetchUseAndLadder` helpers)
- Modify: `services/cli/internal/wizard/actions.go` (`buildResultRows` — show tier in the results-list detail)
- Test: `services/cli/internal/wizard/actions_test.go` (existing — update/add cases), `services/cli/internal/wizard/crawl_actions_test.go` (existing — update/add cases)

**Interfaces:**
- Consumes: `fetchConfig.localConfigured`/`.proxyURL`/`.providerName` (Task 8); `jobgen.Input.Ladder` (Task 7); `sinkItem`/`resultRow` (existing, `actions.go`).
- Produces: `ladderTiers(c fetchConfig) []string`, `resolveFetchUseAndLadder(c fetchConfig) (use string, ladder bool)`.

- [ ] **Step 1: Add the derivation helpers**

Add to `services/cli/internal/wizard/form.go`:
```go
func ladderTiers(c fetchConfig) []string {
	var tiers []string
	if c.localConfigured {
		tiers = append(tiers, "local")
	}
	if c.proxyURL != "" {
		tiers = append(tiers, "proxy")
	}
	if c.providerName != "" {
		tiers = append(tiers, "provider")
	}
	return tiers
}

func resolveFetchUseAndLadder(c fetchConfig) (use string, ladder bool) {
	tiers := ladderTiers(c)
	if len(tiers) == 0 {
		return "", false
	}
	return tiers[0], len(tiers) > 1
}
```

- [ ] **Step 2: Write failing tests for `buildJobSpec`**

Read `services/cli/internal/wizard/actions_test.go` in full first (you've read it earlier in this session already) — several existing tests there construct a `formState` and set `s.fetchVia = "provider"` etc.; those need the same rewrite as Task 9's form tests (construct via `s.providerName = "..."` directly, no `fetchVia` field exists anywhere in the package anymore after this task). Update every such existing test in this file and in `crawl_actions_test.go`, then add:

```go
func TestBuildJobSpecNoTierConfiguredErrors(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"

	_, err := buildJobSpec(s, "")
	if err == nil {
		t.Fatal("buildJobSpec with no tier configured = nil error, want one")
	}
}

func TestBuildJobSpecTwoTiersConfiguredSetsLadder(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.localConfigured = true
	s.providerName = "decodo"

	spec, err := buildJobSpec(s, writeManifest(t, "decodo", flatOptionsManifest))
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if spec.Fetch.Use != "local" {
		t.Errorf("Fetch.Use = %q, want %q (the lowest configured tier)", spec.Fetch.Use, "local")
	}
	if !spec.Fetch.Ladder {
		t.Error("Fetch.Ladder = false, want true -- two tiers were configured")
	}
}

func TestBuildJobSpecOneTierConfiguredLeavesLadderFalse(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.localConfigured = true

	spec, err := buildJobSpec(s, "")
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if spec.Fetch.Use != "local" {
		t.Errorf("Fetch.Use = %q, want %q", spec.Fetch.Use, "local")
	}
	if spec.Fetch.Ladder {
		t.Error("Fetch.Ladder = true, want false -- only one tier was configured")
	}
}
```

`writeManifest`/`flatOptionsManifest` are the existing test fixtures already defined in `actions_test.go` (you've read them earlier this session) — reuse them as-is.

- [ ] **Step 3: Run the new tests to verify they fail**

Run: `go test ./services/cli/internal/wizard/... -run "TestBuildJobSpec" -v`
Expected: FAIL to compile (`formState`/`fetchConfig` have no `fetchVia` for the untouched old tests you haven't rewritten yet, and `buildJobSpec` doesn't yet derive `use`/`ladder` the new way).

- [ ] **Step 4: Rewrite `buildJobSpec`**

In `services/cli/internal/wizard/scraper_actions.go`, replace:
```go
func buildJobSpec(state formState, providersDir string) (*jobspec.Spec, error) {
	concurrency, _ := strconv.Atoi(state.concurrency)
	retries, _ := strconv.Atoi(state.retries)

	profile, proxy, providerName, templateName := state.profile, "", "", ""
	var options map[string]any
	switch state.fetchVia {
	case "proxy":
		proxy = state.proxyURL
	case "provider":
		profile = ""
		providerName = state.providerName
		var err error
		templateName, options, err = resolveProviderOptions(state.fetchConfig, providersDir)
		if err != nil {
			return nil, err
		}
	}

	in := jobgen.Input{
		URLs:        urlList(state),
		FetchUse:    state.fetchVia,
		Proxy:       proxy,
		Provider:    providerName,
		Template:    templateName,
		Options:     options,
		Profile:     profile,
		Timeout:     "30s",
		Stages:      buildStages(state.extractConfig),
		Concurrency: &concurrency,
		Retries:     &retries,
		SinkUse:     "stdout",
	}
	return jobgen.Build(in)
}
```
with:
```go
func buildJobSpec(state formState, providersDir string) (*jobspec.Spec, error) {
	concurrency, _ := strconv.Atoi(state.concurrency)
	retries, _ := strconv.Atoi(state.retries)

	fetchUse, ladder := resolveFetchUseAndLadder(state.fetchConfig)
	if fetchUse == "" {
		return nil, fmt.Errorf("configure at least one of Local/Proxy/Provider")
	}

	profile, proxy, providerName, templateName := "", "", "", ""
	var options map[string]any
	if state.localConfigured || state.proxyURL != "" {
		profile = state.profile
	}
	if state.proxyURL != "" {
		proxy = state.proxyURL
	}
	if state.providerName != "" {
		providerName = state.providerName
		var err error
		templateName, options, err = resolveProviderOptions(state.fetchConfig, providersDir)
		if err != nil {
			return nil, err
		}
	}

	in := jobgen.Input{
		URLs:        urlList(state),
		FetchUse:    fetchUse,
		Ladder:      ladder,
		Proxy:       proxy,
		Provider:    providerName,
		Template:    templateName,
		Options:     options,
		Profile:     profile,
		Timeout:     "30s",
		Stages:      buildStages(state.extractConfig),
		Concurrency: &concurrency,
		Retries:     &retries,
		SinkUse:     "stdout",
	}
	return jobgen.Build(in)
}
```
(`fmt` is already imported in this file.)

Do the mirrored rewrite in `services/cli/internal/wizard/crawl_actions.go`'s `buildCrawlJobSpec` — same `fetchUse, ladder := resolveFetchUseAndLadder(...)`/`if fetchUse == "" { return nil, fmt.Errorf(...) }` guard and the same `profile`/`proxy`/`providerName`/`templateName` derivation, added into that function's existing `crawlIn`/`in := jobgen.Input{...}` construction (which additionally sets `Crawl: crawlIn`, `RPS: &rps` — keep those exactly as they are, just add `Ladder: ladder` and swap `FetchUse: state.fetchVia` for `FetchUse: fetchUse`).

- [ ] **Step 5: Wire the "configure at least one" error into the Run key handler**

In `scraper_form.go`'s `Update`, the existing `runActionName` case already checks `total := len(urlList(f.state)); if total == 0 { f.status = "✗ no URLs to scrape" ... }` before calling `startRun`. Add an analogous check right after it, using the same helper:
```go
			case runActionName:
				f.syncStateFromFields()
				total := len(urlList(f.state))
				if total == 0 {
					f.status = "✗ no URLs to scrape"
					return f, nil
				}
				if use, _ := resolveFetchUseAndLadder(f.state.fetchConfig); use == "" {
					f.status = "✗ configure at least one of Local/Proxy/Provider"
					return f, nil
				}
				if msg := checkProviderCredential(f.state.fetchConfig, f.providersDir, f.credentialsPath); msg != "" {
					f.status = msg
					return f, nil
				}
				...
```
(keep everything else in that case exactly as it is — this is one new `if` block inserted between the existing URL-count check and the existing credential check.)

Do the identical insertion in `crawlForm.go`'s `runActionName` case (its existing check is `if strings.TrimSpace(f.state.startingURL) == "" { ... }` — insert the new tier check right after that, same pattern).

- [ ] **Step 6: Show the tier in the results list**

In `services/cli/internal/wizard/actions.go`'s `buildResultRows`, in the `jobresult.Succeeded` branch, change:
```go
			row.status = fmt.Sprintf("✓ %d", si.Status)
			row.detail = summarizeData(si.Data)
			if row.detail == "" {
				row.detail = bodyPreview(si.Body)
			}
```
to:
```go
			row.status = fmt.Sprintf("✓ %d", si.Status)
			row.detail = summarizeData(si.Data)
			if row.detail == "" {
				row.detail = bodyPreview(si.Body)
			}
			if si.Tier != "" {
				row.detail = "(" + si.Tier + ") " + row.detail
			}
```
(everything else in that branch — setting `raw["finalUrl"]`, `raw["status"]`, etc. — is unchanged; just this one addition.)

`sinkItem` (also in `actions.go`) needs a `Tier` field so `si.Tier` above actually has something to read:
```go
type sinkItem struct {
	URL      string         `json:"url"`
	FinalURL string         `json:"finalUrl"`
	Status   int            `json:"status"`
	Body     string         `json:"body"`
	Depth    int            `json:"depth"`
	Data     map[string]any `json:"data"`
	Tier     string         `json:"tier"`
}
```

Also add `raw["tier"] = si.Tier` next to the other `raw[...]` assignments in that same branch, so item detail (which just renders `row.raw` as JSON) shows it too — no changes needed to `itemdetail.go` itself, since it already renders whatever's in `row.raw`.

- [ ] **Step 7: Run the wizard tests to verify everything passes**

Run: `go test ./services/cli/internal/wizard/... -v 2>&1 | tail -200`
Expected: every test in the package passes, including every rewritten `fetchVia`-referencing test from Tasks 9 and this task.

- [ ] **Step 8: Add a test for the tier-in-results-detail display**

Add to `services/cli/internal/wizard/actions_test.go`:
```go
func TestBuildResultRowsShowsTierInDetail(t *testing.T) {
	items := []jobresult.Item{{URL: "https://a.test", Outcome: jobresult.Succeeded}}
	sinkItems := map[string]sinkItem{
		"https://a.test": {URL: "https://a.test", Status: 200, Tier: "proxy", Data: map[string]any{"title": "hi"}},
	}

	rows := buildResultRows(items, sinkItems, false)
	if !strings.Contains(rows[0].detail, "(proxy)") {
		t.Fatalf("detail = %q, want it to mention the tier", rows[0].detail)
	}
	if rows[0].raw["tier"] != "proxy" {
		t.Fatalf(`raw["tier"] = %v, want "proxy"`, rows[0].raw["tier"])
	}
}
```

Run: `go test ./services/cli/internal/wizard/... -run TestBuildResultRowsShowsTierInDetail -v`
Expected: PASS.

- [ ] **Step 9: Run the full repo gate**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l .
go build ./...
go vet ./...
make lint
go test -race -count=1 ./...
```
Expected: everything green — this is the first point where the entire feature (jobspec through TUI) is complete and the whole repo's test suite passes end to end.

- [ ] **Step 10: Live pty verification**

Follow this session's established pattern: build a temp binary (`go build -o /tmp/pobear-ladder-check ./cmd/core`), run it via a Python `pty`+`pyte` script from a directory outside the repo with an isolated fake `$HOME`, navigate Menu → Scrape a page, configure Local, then also configure Provider (decodo, using the real `providers/` manifest via `POBEAR_PROVIDERS_DIR`) so two tiers end up configured, confirm both rows show `✓ ...`, run it against `https://example.com`, and confirm the result shows `(local)` in its detail line (since local alone will already succeed and never needs to escalate — this test verifies the *plumbing* end-to-end, not the escalation path itself, which Task 5's unit tests already cover directly). Delete the temp binary and script afterward.

- [ ] **Step 11: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
git add services/cli/internal/wizard/scraper_actions.go services/cli/internal/wizard/crawl_actions.go services/cli/internal/wizard/form.go services/cli/internal/wizard/actions.go services/cli/internal/wizard/scraper_form.go services/cli/internal/wizard/crawl_form.go services/cli/internal/wizard/actions_test.go services/cli/internal/wizard/crawl_actions_test.go
git commit -m "wizard: derive fetch.use/ladder from configured tiers, show tier in results"
```

---

### Task 11: Docs

**Files:**
- Modify: `docs/JOB_SPEC_REFERENCE.md`
- Modify: `docs/CLI_REFERENCE.md`

**Interfaces:**
- Consumes: nothing code-level — this task documents the already-implemented and already-tested behavior from Tasks 1-10.

- [ ] **Step 1: `docs/JOB_SPEC_REFERENCE.md`**

Read the existing `## 4. fetch` section (or wherever `fetch.use`/`fetch.proxy`/`fetch.name` are documented as a field table) and add a `ladder` row to that table:
```
| `ladder` | `bool` | `false` | escalate to a higher configured tier (proxy, then provider) when a fetch is `blocked` or `rateLimit`; requires a tier above `use` to be configured |
```
Add a short subsection right after that table (matching the existing doc's style of a prose paragraph following each field table — read a neighboring section like `robots`/`sitemap`'s explanation in the crawl-source section for the expected tone) explaining: the fixed `local < proxy < provider` hierarchy; that a tier is "configured" purely by whether its own fields (`proxy`, `name`) are present, no separate per-tier enable flag; that with `ladder: false` (the default), `proxy`/`name`/`template`/`options` must exactly match `use` (today's existing rule, already documented — just clarify it's the `ladder: false` case specifically now); that each rung gets its own full `limits.retries`; and reference the envelope's new `tier` field (see the `sink`/output-shape section) as how a completed job reports which tier actually served each URL.

Also update the `sink`/output-envelope's field table (`docs/JOB_SPEC_REFERENCE.md`'s §7, referenced earlier in this session's CLI work) to add:
```
| `tier` | `string` | which fetch tier actually served this URL (`local`, `proxy`, or `provider`) — always present, even when `ladder` was never used |
```

- [ ] **Step 2: `docs/CLI_REFERENCE.md`**

Add `--ladder` to the scrape flags table (alongside `--proxy`/`--provider`):
```
| `--ladder` | off | escalate to proxy/provider (whichever are configured) if blocked/rate-limited — requires --proxy and/or --provider |
```
(crawl's flags table inherits scrape's via the "Everything scrape has, plus:" framing already used in that doc — no separate row needed there, same as `--bare`/`--concurrency`/etc. aren't re-listed for crawl.)

Add a short example block (matching this doc's existing `$ pobear scrape ... --print-spec` style) showing `--ladder` in use and its two new validation errors as literal terminal transcripts, e.g.:
```
$ pobear scrape https://example.com --ladder
pobear: --ladder requires --proxy and/or --provider to be set
$ echo $?
2

$ pobear scrape https://example.com --proxy http://p.test --provider decodo
pobear: both --proxy and --provider are set -- add --ladder to escalate between them, or remove one
$ echo $?
2
```
(confirm the exit code shown matches `exitcode.UsageError`'s real value — read `pkg/exitcode` if unsure, don't guess.)

Update the wizard walkthrough section's Scrape/Crawl form mockups (the ones already showing `Local`/`Proxy`/`Provider` rows — if Task 9/10 already updated these as part of their own screenshot-regeneration step, this is just confirming the doc's hand-written ASCII mockups, which are separate from the test golden files, also reflect the three-row model; if this repo's CLI_REFERENCE.md still shows the old single `Fetch via` mockup from before this feature, replace it the same way earlier sessions in this project have kept that doc's mockups in sync with the real TUI — read the current mockup section first and match its established style exactly).

- [ ] **Step 3: Final full-repo verification**

```bash
cd /Users/adomas.skliutas/code/pobear
gofmt -l .
go build ./...
go vet ./...
make lint
go test -race -count=1 ./...
```
Expected: clean. (Docs changes don't affect this, but it's the last task in the plan — confirm the repo is still fully green before the final commit.)

- [ ] **Step 4: Commit**

```bash
cd /Users/adomas.skliutas/code/pobear
git add docs/JOB_SPEC_REFERENCE.md docs/CLI_REFERENCE.md
git commit -m "docs: document fetch.ladder, --ladder, and the tier output field"
```

---

## Self-review notes (for whoever executes this plan)

- Task 8 and Task 9 are written as separate tasks for planning clarity (state/sub-screen design vs. main-form wiring) but **do not form a working, compiling, testable boundary on their own** — Task 8 alone leaves the package non-compiling. Treat them as one execution unit: implement both, then run the build/test/commit steps once, at the end of Task 9.
- Every task from 8 onward touches code that today reads `state.fetchVia`/`c.fetchVia` in more places than this plan's snippets show individually (both `scraper_form.go`/`crawl_form.go`'s `syncStateFromFields`, and both `scraper_actions.go`/`crawl_actions.go`'s `buildJobSpec`/`buildCrawlJobSpec`). Before considering Task 9 or Task 10 done, `grep -rn "fetchVia" services/cli/internal/wizard/` should return zero matches anywhere in non-test code, and the full wizard test suite (not just the specific tests this plan calls out) must pass — read every compiler error `go build ./...` produces after deleting `fetchVia` and fix each one; this plan's snippets are the main, expected ones, not necessarily an exhaustive list.
