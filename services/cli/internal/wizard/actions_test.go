package wizard

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobresult"
)

func TestBuildJobSpecLocal(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.localConfigured = true

	spec, err := buildJobSpec(s, "")
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if got := spec.Source.URLs; len(got) != 1 || got[0] != s.url {
		t.Errorf("Source.URLs = %v, want [%q]", got, s.url)
	}
	if spec.Fetch.Use != "local" {
		t.Errorf("Fetch.Use = %q, want \"local\"", spec.Fetch.Use)
	}
	if spec.Fetch.Profile != "chrome" {
		t.Errorf("Fetch.Profile = %q, want \"chrome\"", spec.Fetch.Profile)
	}
	if len(spec.Stages) != 0 {
		t.Errorf("Stages = %v, want none for the default \"bare\" extract mode", spec.Stages)
	}
	if spec.Limits.Concurrency != 8 || spec.Limits.Retries != 3 {
		t.Errorf("Limits = %+v, want concurrency=8 retries=3", spec.Limits)
	}
}

func TestBuildJobSpecProxy(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.proxyURL = "http://proxy.example.com:8080"

	spec, err := buildJobSpec(s, "")
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if spec.Fetch.Use != "proxy" {
		t.Errorf("Fetch.Use = %q, want \"proxy\"", spec.Fetch.Use)
	}
	if spec.Fetch.Proxy != s.proxyURL {
		t.Errorf("Fetch.Proxy = %q, want %q", spec.Fetch.Proxy, s.proxyURL)
	}
	if spec.Fetch.Profile != "chrome" {
		t.Errorf("Fetch.Profile = %q, want \"chrome\" (profile applies to proxy too)", spec.Fetch.Profile)
	}
}

func writeManifest(t *testing.T, name, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dir
}

const flatOptionsManifest = `
name: decodo
endpoint: https://data.decodo.com/v1/scrape
auth: {type: bearer, credential: token}
options:
  - {name: headless, type: enum, values: [html, png], default: html}
  - {name: geo, type: string}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`

const templatedManifest = `
name: acme
endpoint: https://acme.example.com/scrape
auth: {type: bearer, credential: token}
templates:
  - name: product
    options:
      - {name: domain, type: string}
    request: {body: {url: "{{ .URL }}"}}
  - name: search
    options:
      - {name: query, type: string}
    request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`

func TestBuildJobSpecProviderClearsProfile(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.providerName = "decodo"

	spec, err := buildJobSpec(s, writeManifest(t, "decodo", flatOptionsManifest))
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if spec.Fetch.Use != "provider" {
		t.Errorf("Fetch.Use = %q, want \"provider\"", spec.Fetch.Use)
	}
	if spec.Fetch.Name != "decodo" {
		t.Errorf("Fetch.Name = %q, want \"decodo\"", spec.Fetch.Name)
	}
	if spec.Fetch.Profile != "" {
		t.Errorf("Fetch.Profile = %q, want empty for provider fetch", spec.Fetch.Profile)
	}
}

func TestBuildJobSpecProviderIncludesResolvedOptions(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.providerName = "decodo"
	s.providerOptions = map[string]string{"headless": "png", "geo": "us"}

	spec, err := buildJobSpec(s, writeManifest(t, "decodo", flatOptionsManifest))
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if got := spec.Fetch.Options["headless"]; got != "png" {
		t.Errorf(`Fetch.Options["headless"] = %v, want "png"`, got)
	}
	if got := spec.Fetch.Options["geo"]; got != "us" {
		t.Errorf(`Fetch.Options["geo"] = %v, want "us"`, got)
	}
}

func TestBuildJobSpecProviderTemplateDefaultsToFirstWhenAmbiguous(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.providerName = "acme"
	s.providerOptions = map[string]string{"domain": "example.com"}

	spec, err := buildJobSpec(s, writeManifest(t, "acme", templatedManifest))
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if spec.Fetch.Template != "product" {
		t.Errorf("Fetch.Template = %q, want \"product\" (the first declared template, matching the form's own default)", spec.Fetch.Template)
	}
	if got := spec.Fetch.Options["domain"]; got != "example.com" {
		t.Errorf(`Fetch.Options["domain"] = %v, want "example.com"`, got)
	}
}

const templatedManifestWithDefault = `
name: decodo
endpoint: https://data.decodo.com/v1/scrape
auth: {type: bearer, credential: token}
templates:
  - name: universal
    default: true
    options:
      - {name: geo, type: string}
    request: {body: {url: "{{ .URL }}"}}
  - name: google
    options:
      - {name: page_count, type: int, default: 1, min: 1, max: 10}
    request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`

func TestBuildJobSpecProviderDefaultsToTheMarkedDefaultTemplate(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.providerName = "decodo"
	s.providerOptions = map[string]string{"geo": "us"}

	spec, err := buildJobSpec(s, writeManifest(t, "decodo", templatedManifestWithDefault))
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if spec.Fetch.Template != "universal" {
		t.Errorf("Fetch.Template = %q, want \"universal\" (the manifest's declared default)", spec.Fetch.Template)
	}
	if got := spec.Fetch.Options["geo"]; got != "us" {
		t.Errorf(`Fetch.Options["geo"] = %v, want "us"`, got)
	}
}

func TestBuildJobSpecProviderHonorsExplicitNonDefaultTemplate(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.providerName = "decodo"
	s.templateName = "google"
	s.providerOptions = map[string]string{"page_count": "5"}

	spec, err := buildJobSpec(s, writeManifest(t, "decodo", templatedManifestWithDefault))
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if spec.Fetch.Template != "google" {
		t.Errorf("Fetch.Template = %q, want \"google\" (explicitly chosen, overriding the default)", spec.Fetch.Template)
	}
	if got := spec.Fetch.Options["page_count"]; got != 5 {
		t.Errorf(`Fetch.Options["page_count"] = %v, want 5`, got)
	}
}

func TestBuildJobSpecProviderUnknownProviderErrors(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.providerName = "nope"

	if _, err := buildJobSpec(s, t.TempDir()); err == nil {
		t.Fatal("buildJobSpec with an unknown provider = nil error, want one")
	}
}

func TestBuildJobSpecCSSFieldsFiltersBlankPairs(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.localConfigured = true
	s.extract = "css fields"
	s.cssFields = []cssFieldPair{
		{name: "title", selector: "h1"},
		{name: "", selector: ""},
		{name: "price", selector: ".price"},
	}

	spec, err := buildJobSpec(s, "")
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if len(spec.Stages) != 1 || spec.Stages[0].Use != "css" {
		t.Fatalf("Stages = %v, want one css stage", spec.Stages)
	}
	want := map[string]string{"title": "h1", "price": ".price"}
	if len(spec.Stages[0].Fields) != len(want) {
		t.Fatalf("Stages[0].Fields = %v, want %v", spec.Stages[0].Fields, want)
	}
	for k, v := range want {
		if spec.Stages[0].Fields[k] != v {
			t.Errorf("Stages[0].Fields[%q] = %q, want %q", k, spec.Stages[0].Fields[k], v)
		}
	}
}

func TestBuildJobSpecBareRunsNoStage(t *testing.T) {
	s := newFormState()
	s.url = "https://example.com"
	s.localConfigured = true
	s.extract = "bare"

	spec, err := buildJobSpec(s, "")
	if err != nil {
		t.Fatalf("buildJobSpec: %v", err)
	}
	if len(spec.Stages) != 0 {
		t.Fatalf("Stages = %v, want none for bare extract (falls back to the raw fetched body)", spec.Stages)
	}
}

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

func TestBuildResultRowsFallsBackToBodyPreviewWhenNoDataExtracted(t *testing.T) {
	items := []jobresult.Item{{URL: "https://a.test", Outcome: jobresult.Succeeded}}
	sinkItems := map[string]sinkItem{
		"https://a.test": {URL: "https://a.test", Status: 200, Body: "<html>\n<body>hello</body>\n</html>"},
	}

	rows := buildResultRows(items, sinkItems, false)
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want 1", rows)
	}
	if !strings.Contains(rows[0].detail, "hello") {
		t.Fatalf("detail = %q, want a body preview since Data was empty", rows[0].detail)
	}
	if _, hasData := rows[0].raw["data"]; hasData {
		t.Errorf(`raw = %v, want no "data" key when nothing was extracted -- just the body`, rows[0].raw)
	}
	if _, hasBody := rows[0].raw["body"]; !hasBody {
		t.Errorf(`raw = %v, want a "body" key`, rows[0].raw)
	}
}

func TestBuildResultRowsCarriesBareAndBodyForItemDetail(t *testing.T) {
	items := []jobresult.Item{{URL: "https://a.test", Outcome: jobresult.Succeeded}}
	sinkItems := map[string]sinkItem{
		"https://a.test": {URL: "https://a.test", Status: 200, Body: "<html>hi</html>"},
	}

	rows := buildResultRows(items, sinkItems, true)
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want 1", rows)
	}
	if !rows[0].bare {
		t.Fatalf("rows[0].bare = %v, want true", rows[0].bare)
	}
	if rows[0].body != "<html>hi</html>" {
		t.Fatalf("rows[0].body = %q, want the raw fetched body", rows[0].body)
	}
}

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

func TestBuildResultRowsCollapsesMultilineErrorsToOneLine(t *testing.T) {
	multiline := "connection failed:\nsome cause\nanother cause"
	items := []jobresult.Item{{URL: "https://a.test", Outcome: jobresult.Failed, Err: multiline}}

	rows := buildResultRows(items, nil, false)
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want 1", rows)
	}
	if strings.Contains(rows[0].detail, "\n") {
		t.Fatalf("detail = %q, want a single line with no embedded newlines", rows[0].detail)
	}
	if !strings.Contains(rows[0].detail, "connection failed:") || !strings.Contains(rows[0].detail, "some cause") {
		t.Fatalf("detail = %q, want it to still contain the error text, just on one line", rows[0].detail)
	}
	if rows[0].raw["error"] != multiline {
		t.Fatalf(`raw["error"] = %q, want the full multiline error preserved for the JSON detail view`, rows[0].raw["error"])
	}
}

func TestBuildResultRowsCarriesFaultKindForFailedItems(t *testing.T) {
	items := []jobresult.Item{
		{URL: "https://a.test", Outcome: jobresult.Failed, Err: "blocked", Kind: fault.Blocked},
		{URL: "https://b.test", Outcome: jobresult.Failed, Err: "rate limited", Kind: fault.RateLimit},
		{URL: "https://c.test", Outcome: jobresult.Succeeded},
	}

	rows := buildResultRows(items, nil, false)
	if rows[0].kind != fault.Blocked {
		t.Errorf("rows[0].kind = %q, want %q", rows[0].kind, fault.Blocked)
	}
	if rows[1].kind != fault.RateLimit {
		t.Errorf("rows[1].kind = %q, want %q", rows[1].kind, fault.RateLimit)
	}
	if rows[2].kind != "" {
		t.Errorf("rows[2].kind (succeeded item) = %q, want empty", rows[2].kind)
	}
}

func TestBuildResultRowsSortsSkippedItemsToTheBottom(t *testing.T) {
	items := []jobresult.Item{
		{URL: "https://a.test", Outcome: jobresult.Skipped, Err: "over budget"},
		{URL: "https://b.test", Outcome: jobresult.Succeeded},
		{URL: "https://c.test", Outcome: jobresult.Failed, Err: "boom"},
		{URL: "https://d.test", Outcome: jobresult.Skipped, Err: "over budget"},
		{URL: "https://e.test", Outcome: jobresult.Succeeded},
	}

	rows := buildResultRows(items, nil, false)
	want := []string{"https://b.test", "https://c.test", "https://e.test", "https://a.test", "https://d.test"}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v, want %d rows", rows, len(want))
	}
	for i, url := range want {
		if rows[i].url != url {
			t.Errorf("rows[%d].url = %q, want %q (non-skipped first, skipped stable-sorted to the bottom)", i, rows[i].url, url)
		}
	}
}

func TestDoSaveWritesJobFile(t *testing.T) {
	t.Chdir(t.TempDir())

	s := newFormState()
	s.url = "https://example.com"
	s.localConfigured = true

	status := doSave(s, "")
	if !strings.HasPrefix(status, "✓") {
		t.Fatalf("doSave status = %q, want success", status)
	}

	data, err := os.ReadFile(savedJobFilename)
	if err != nil {
		t.Fatalf("reading %s: %v", savedJobFilename, err)
	}
	if !strings.Contains(string(data), "https://example.com") {
		t.Errorf("%s doesn't contain the URL:\n%s", savedJobFilename, data)
	}
}

func TestDoSaveReportsBuildErrors(t *testing.T) {
	t.Chdir(t.TempDir())

	s := newFormState()
	s.url = "" // no URL at all should fail jobspec validation

	status := doSave(s, "")
	if !strings.HasPrefix(status, "✗") {
		t.Fatalf("doSave status = %q, want a failure", status)
	}
	if _, err := os.Stat(savedJobFilename); err == nil {
		t.Errorf("expected no %s to be written on a build error", savedJobFilename)
	}
}

func TestCheckProviderCredentialCatchesMissingCredential(t *testing.T) {
	s := newFormState()
	s.providerName = "decodo"

	dir := writeManifest(t, "decodo", flatOptionsManifest)
	credPath := filepath.Join(t.TempDir(), "credentials")

	got := checkProviderCredential(s.fetchConfig, dir, credPath)
	if !strings.HasPrefix(got, "✗") {
		t.Fatalf("checkProviderCredential = %q, want a failure message", got)
	}
	if !strings.Contains(got, "decodo") || !strings.Contains(got, "Manage credentials") {
		t.Fatalf("checkProviderCredential = %q, want it to name the provider and point at \"Manage credentials\"", got)
	}
}

func TestCheckProviderCredentialPassesWhenCredentialPresent(t *testing.T) {
	s := newFormState()
	s.providerName = "decodo"

	dir := writeManifest(t, "decodo", flatOptionsManifest)
	credPath := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(credPath, []byte("POBEAR_PROVIDER_DECODO_TOKEN=secret\n"), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	if got := checkProviderCredential(s.fetchConfig, dir, credPath); got != "" {
		t.Fatalf("checkProviderCredential = %q, want empty (credential present)", got)
	}
}

func TestCheckProviderCredentialSkippedForNonProviderFetch(t *testing.T) {
	s := newFormState()
	if got := checkProviderCredential(s.fetchConfig, t.TempDir(), ""); got != "" {
		t.Fatalf("checkProviderCredential for a non-provider fetch = %q, want empty", got)
	}
}

func TestDoRunAgainstLocalServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body><h1>hello</h1></body></html>")
	}))
	defer srv.Close()

	s := newFormState()
	s.url = srv.URL
	s.localConfigured = true
	s.extract = "raw"

	status, rows := doRun(context.Background(), s, t.TempDir(), filepath.Join(t.TempDir(), "credentials"), nil)
	if !strings.HasPrefix(status, "✓") {
		t.Fatalf("doRun status = %q, want success", status)
	}
	if !strings.Contains(status, "fetched 1, errors 0, skipped 0") {
		t.Errorf("doRun status = %q, want a 1/0/0 summary", status)
	}
	if len(rows) != 1 {
		t.Fatalf("doRun rows = %v, want exactly 1 row", rows)
	}
	if rows[0].url != s.url || !strings.HasPrefix(rows[0].status, "✓") {
		t.Errorf("doRun rows[0] = %+v, want url=%q and a success status", rows[0], s.url)
	}
}

func TestDoRunReportsFetchFailures(t *testing.T) {
	s := newFormState()
	s.url = "http://127.0.0.1:1" // nothing listens here -- should fail fast
	s.localConfigured = true
	s.extract = "raw"

	status, rows := doRun(context.Background(), s, t.TempDir(), filepath.Join(t.TempDir(), "credentials"), nil)
	if !strings.HasPrefix(status, "✗") {
		t.Fatalf("doRun status = %q, want a failure", status)
	}
	if len(rows) != 1 || strings.HasPrefix(rows[0].status, "✓") {
		t.Fatalf("doRun rows = %v, want exactly 1 non-success row", rows)
	}
}

func TestDoRunReportsBuildErrors(t *testing.T) {
	s := newFormState()
	s.url = ""

	status, rows := doRun(context.Background(), s, t.TempDir(), "", nil)
	if !strings.HasPrefix(status, "✗") {
		t.Fatalf("doRun status = %q, want a build-error failure", status)
	}
	if rows != nil {
		t.Fatalf("doRun rows = %v, want nil for a build error (job never ran)", rows)
	}
}

func TestDoWriteItemWritesJSONFile(t *testing.T) {
	t.Chdir(t.TempDir())

	row := resultRow{url: "https://example.com/page", status: "✓ 200", raw: map[string]any{"url": "https://example.com/page", "status": 200}}
	status := doWriteItem(row)
	if !strings.HasPrefix(status, "✓") {
		t.Fatalf("doWriteItem status = %q, want success", status)
	}

	name := sanitizeFilename(row.url) + ".json"
	data, err := os.ReadFile(name) //nolint:gosec // name is this test's own t.Chdir(t.TempDir()) output file
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	if !strings.Contains(string(data), "https://example.com/page") {
		t.Errorf("%s doesn't contain the URL:\n%s", name, data)
	}
}

func TestDoWriteAllItemsWritesOneFilePerRow(t *testing.T) {
	t.Chdir(t.TempDir())

	rows := []resultRow{
		{url: "https://a.test", raw: map[string]any{"url": "https://a.test"}},
		{url: "https://b.test", raw: map[string]any{"url": "https://b.test"}},
	}
	status := doWriteAllItems(rows)
	if !strings.HasPrefix(status, "✓") {
		t.Fatalf("doWriteAllItems status = %q, want success", status)
	}

	entries, err := os.ReadDir(resultsDirName)
	if err != nil {
		t.Fatalf("reading %s: %v", resultsDirName, err)
	}
	if len(entries) != len(rows) {
		t.Fatalf("wrote %d files, want %d", len(entries), len(rows))
	}
}

func TestSanitizeFilenameProducesASafeName(t *testing.T) {
	got := sanitizeFilename("https://a.test/path?q=1&x=2")
	if strings.ContainsAny(got, "/:?&") {
		t.Fatalf("sanitizeFilename(%q) = %q, still contains unsafe characters", "https://a.test/path?q=1&x=2", got)
	}
	if got == "" {
		t.Fatal("sanitizeFilename produced an empty name")
	}
}
