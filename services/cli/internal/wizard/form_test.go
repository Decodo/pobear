package wizard

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/provider"
)

func manifestsWithNames(names ...string) []*provider.Manifest {
	manifests := make([]*provider.Manifest, len(names))
	for i, n := range names {
		manifests[i] = &provider.Manifest{Name: n}
	}
	return manifests
}

func fieldNames(fields []field) []string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.name
	}
	return names
}

func containsName(fields []field, name string) bool {
	for _, f := range fields {
		if f.name == name {
			return true
		}
	}
	return false
}

func findField(fields []field, name string) (field, bool) {
	for _, f := range fields {
		if f.name == name {
			return f, true
		}
	}
	return field{}, false
}

func TestComputeFieldsDefaultState(t *testing.T) {
	fields := computeFields(newFormState())
	want := []string{"url", "localTier", "proxyTier", "providerTier", "extract", "concurrency", "retries", "saveAction", "runAction"}
	got := fieldNames(fields)
	if len(got) != len(want) {
		t.Fatalf("got fields %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestComputeFieldsShowsThreeTierRows(t *testing.T) {
	fields := computeFields(newFormState())
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
	fields := computeFields(newFormState())
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
	for i, fld := range f.fields {
		if fld.name == "localTier" {
			f.cursor = i
			break
		}
	}
	f.refocus()

	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on the localTier row to return a push command")
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

func TestProxyTierFieldsHasURLAndProfile(t *testing.T) {
	fields := proxyTierFields(fetchConfig{})
	if !containsName(fields, "proxyURL") {
		t.Error("expected a \"proxyURL\" field on the proxy tier")
	}
	if !containsName(fields, "profile") {
		t.Error("expected a \"profile\" field on the proxy tier -- fetch.profile is valid for both local and proxy")
	}
}

func TestProviderTierFieldsHasProviderNotProfile(t *testing.T) {
	fields := providerTierFields(fetchConfig{}, nil)
	if containsName(fields, "profile") {
		t.Error("expected no \"profile\" field on the provider tier")
	}
	if !containsName(fields, "provider") {
		t.Error("expected a \"provider\" field on the provider tier")
	}
}

func TestComputeFieldsExtractFieldsShowsCSSEditor(t *testing.T) {
	s := newFormState()
	s.extract = "css fields"
	s.cssFields = []cssFieldPair{{name: "title", selector: "h1"}}
	fields := computeFields(s)
	if !containsName(fields, "css.name.0") || !containsName(fields, "css.selector.0") {
		t.Errorf("expected css.name.0/css.selector.0 fields, got %v", fieldNames(fields))
	}
	if !containsName(fields, addFieldName) {
		t.Error("expected the \"+ add field\" row when extract is fields")
	}

	name, _ := findField(fields, "css.name.0")
	if name.hint == "" {
		t.Error("expected a visible hint on css fields for how to remove them (ctrl+d isn't otherwise discoverable)")
	}
}

func TestComputeFieldsRawOrMarkdownHasNoCSSEditor(t *testing.T) {
	for _, mode := range []string{"raw", "markdown"} {
		s := newFormState()
		s.extract = mode
		fields := computeFields(s)
		if containsName(fields, addFieldName) {
			t.Errorf("extract=%s: expected no css editor, got %v", mode, fieldNames(fields))
		}
	}
}

func TestComputeFieldsProfileIsAChoiceOfThree(t *testing.T) {
	fields := localTierFields(fetchConfig{})
	profile, ok := findField(fields, "profile")
	if !ok {
		t.Fatal("expected a \"profile\" field")
	}
	if profile.kind != fieldChoice {
		t.Fatalf("expected profile to be a choice field, got kind %v", profile.kind)
	}
	want := []string{"chrome", "firefox", "safari"}
	if len(profile.options) != len(want) {
		t.Fatalf("profile options = %v, want %v", profile.options, want)
	}
	for i := range want {
		if profile.options[i] != want[i] {
			t.Errorf("profile option %d = %q, want %q", i, profile.options[i], want[i])
		}
	}
}

func TestComputeFieldsExtractIsAChoiceOfFourIncludingBare(t *testing.T) {
	s := newFormState()
	fields := computeFields(s)
	extract, ok := findField(fields, "extract")
	if !ok {
		t.Fatal("expected an \"extract\" field")
	}
	want := []string{"bare", "raw", "markdown", "css fields"}
	if len(extract.options) != len(want) {
		t.Fatalf("extract options = %v, want %v", extract.options, want)
	}
	for i := range want {
		if extract.options[i] != want[i] {
			t.Errorf("extract option %d = %q, want %q", i, extract.options[i], want[i])
		}
	}
}

func TestNewFormStateDefaultsExtractToBare(t *testing.T) {
	if got := newFormState().extract; got != "bare" {
		t.Fatalf("newFormState().extract = %q, want \"bare\"", got)
	}
}

func TestBareEasterEggOnlyChangesDisplayNeverTheRealValue(t *testing.T) {
	old := bareEasterEgg
	defer func() { bareEasterEgg = old }()

	bareEasterEgg = true
	s := newFormState()
	s.extract = "bare"
	fields := computeFields(s)
	extract, _ := findField(fields, "extract")
	if extract.value() != "bare" {
		t.Fatalf("extract.value() with the easter egg active = %q, want the real value \"bare\" unchanged", extract.value())
	}
	if extract.options[0] != "bare" {
		t.Fatalf("extract.options[0] = %q, want the real option \"bare\" unchanged", extract.options[0])
	}
	if extract.displayOverride != "bear" {
		t.Fatalf("displayOverride with the easter egg active and extract=bare = %q, want \"bear\"", extract.displayOverride)
	}
	if !strings.Contains(extract.render(true, 20), "bear") {
		t.Fatalf("render() with the easter egg active = %q, want it to show \"bear\" instead of \"bare\"", extract.render(true, 20))
	}

	bareEasterEgg = false
	fields = computeFields(s)
	extract, _ = findField(fields, "extract")
	if extract.displayOverride != "" {
		t.Fatalf("displayOverride with the easter egg inactive = %q, want empty", extract.displayOverride)
	}

	bareEasterEgg = true
	s.extract = "raw"
	fields = computeFields(s)
	extract, _ = findField(fields, "extract")
	if extract.displayOverride != "" {
		t.Fatalf("displayOverride for extract=%q with the easter egg active = %q, want empty (only \"bare\" gets the substitution)", s.extract, extract.displayOverride)
	}
}

func TestComputeFieldsProviderIsAChoiceOfKnownProviders(t *testing.T) {
	fields := providerTierFields(fetchConfig{}, manifestsWithNames("acme", "decodo"))
	provider, ok := findField(fields, "provider")
	if !ok {
		t.Fatal("expected a \"provider\" field")
	}
	if provider.kind != fieldChoice {
		t.Fatalf("expected provider to be a choice field, got kind %v", provider.kind)
	}
	want := []string{"", "acme", "decodo"}
	if len(provider.options) != len(want) {
		t.Fatalf("provider options = %v, want %v", provider.options, want)
	}
	for i := range want {
		if provider.options[i] != want[i] {
			t.Errorf("provider option %d = %q, want %q", i, provider.options[i], want[i])
		}
	}
}

func TestComputeFieldsProviderStartsBlankNotFirstManifest(t *testing.T) {
	fields := providerTierFields(fetchConfig{}, manifestsWithNames("acme", "decodo"))
	provider, ok := findField(fields, "provider")
	if !ok {
		t.Fatal("expected a \"provider\" field")
	}
	if provider.value() != "" {
		t.Errorf("provider field value = %q, want \"\" (opening the screen must not silently select a provider)", provider.value())
	}
}

func TestComputeFieldsProviderWithNoneConfiguredStillRenders(t *testing.T) {
	fields := providerTierFields(fetchConfig{}, nil)
	provider, ok := findField(fields, "provider")
	if !ok {
		t.Fatal("expected a \"provider\" field even with no providers configured")
	}
	if len(provider.options) == 0 {
		t.Fatal("expected a placeholder option when no providers are configured")
	}
}

func flatOptionsManifestFixture() *provider.Manifest {
	return &provider.Manifest{
		Name: "decodo",
		Options: []provider.Option{
			{Name: "headless", Type: "enum", Values: []string{"html", "png"}, Default: "html"},
			{Name: "geo", Type: "string"},
		},
	}
}

func templatedManifestFixture() *provider.Manifest {
	return &provider.Manifest{
		Name: "acme",
		Templates: []provider.Template{
			{Name: "product", Options: []provider.Option{{Name: "domain", Type: "string"}}},
			{Name: "search", Options: []provider.Option{{Name: "query", Type: "string"}}},
		},
	}
}

func TestComputeFieldsProviderShowsManifestOptionFields(t *testing.T) {
	c := fetchConfig{providerName: "decodo"}

	fields := providerTierFields(c, []*provider.Manifest{flatOptionsManifestFixture()})
	if !containsName(fields, "opt.headless") || !containsName(fields, "opt.geo") {
		t.Fatalf("expected opt.headless/opt.geo fields, got %v", fieldNames(fields))
	}
	if containsName(fields, "template") {
		t.Error("expected no \"template\" field for a manifest with no templates")
	}

	headless, _ := findField(fields, "opt.headless")
	if headless.value() != "html" {
		t.Errorf("opt.headless default value = %q, want \"html\" (the manifest's declared default)", headless.value())
	}
}

func TestComputeFieldsProviderIntOptionFieldShowsHint(t *testing.T) {
	c := fetchConfig{providerName: "decodo"}
	m := &provider.Manifest{
		Name:    "decodo",
		Options: []provider.Option{{Name: "page_count", Type: "int", Default: 1, Min: intPtr(1), Max: intPtr(10)}},
	}

	fields := providerTierFields(c, []*provider.Manifest{m})
	pageCount, ok := findField(fields, "opt.page_count")
	if !ok {
		t.Fatal("expected an \"opt.page_count\" field")
	}
	if pageCount.hint == "" {
		t.Error("expected a non-empty hint on an int-typed option field, same as enum/bool/string options get")
	}
}

func intPtr(n int) *int { return &n }

func TestComputeFieldsProviderWithAmbiguousTemplatesShowsTemplateField(t *testing.T) {
	c := fetchConfig{providerName: "acme"}

	fields := providerTierFields(c, []*provider.Manifest{templatedManifestFixture()})
	tmpl, ok := findField(fields, "template")
	if !ok {
		t.Fatal("expected a \"template\" field when the manifest has 2+ templates and none is default")
	}
	want := []string{"product", "search"}
	if len(tmpl.options) != len(want) || tmpl.options[0] != want[0] || tmpl.options[1] != want[1] {
		t.Errorf("template options = %v, want %v", tmpl.options, want)
	}
	if !containsName(fields, "opt.domain") {
		t.Errorf("expected the first template's (product) options to show by default, got %v", fieldNames(fields))
	}
	if containsName(fields, "opt.query") {
		t.Errorf("expected only the selected template's options, not search's, got %v", fieldNames(fields))
	}
}

func TestComputeFieldsProviderWithDefaultTemplateStillShowsTemplateField(t *testing.T) {
	c := fetchConfig{providerName: "decodo"}
	m := &provider.Manifest{
		Name: "decodo",
		Templates: []provider.Template{
			{Name: "universal", Default: true, Options: []provider.Option{{Name: "geo", Type: "string"}}},
			{Name: "google", Options: []provider.Option{{Name: "page_count", Type: "int"}}},
		},
	}

	fields := providerTierFields(c, []*provider.Manifest{m})
	tmpl, ok := findField(fields, "template")
	if !ok {
		t.Fatal("expected a \"template\" field even though one template is marked default -- the user should still be able to reach the non-default ones")
	}
	if tmpl.value() != "universal" {
		t.Errorf("template default value = %q, want \"universal\" (the manifest's declared default)", tmpl.value())
	}
	if !containsName(fields, "opt.geo") || containsName(fields, "opt.page_count") {
		t.Errorf("expected only the default template's (universal) options until the user switches, got %v", fieldNames(fields))
	}
}

func TestComputeFieldsProviderWithSingleTemplateHidesTemplateField(t *testing.T) {
	c := fetchConfig{providerName: "decodo"}
	m := &provider.Manifest{
		Name: "decodo",
		Templates: []provider.Template{
			{Name: "universal", Options: []provider.Option{{Name: "geo", Type: "string"}}},
		},
	}

	fields := providerTierFields(c, []*provider.Manifest{m})
	if containsName(fields, "template") {
		t.Error("expected no \"template\" field when there's only one template -- there's nothing to choose")
	}
	if !containsName(fields, "opt.geo") {
		t.Errorf("expected the single template's options to show directly, got %v", fieldNames(fields))
	}
}

func TestTierScreenTypingIntoTemplateSwitchesToNonDefaultOptions(t *testing.T) {
	m := &provider.Manifest{
		Name: "decodo",
		Templates: []provider.Template{
			{Name: "universal", Default: true, Options: []provider.Option{{Name: "geo", Type: "string"}}},
			{Name: "google", Options: []provider.Option{{Name: "page_count", Type: "int"}}},
		},
	}
	fc := &fetchConfig{providerName: "decodo"}
	s := newTierScreen("provider", fc, []*provider.Manifest{m}, func() {})

	for i, fld := range s.fields {
		if fld.name == "template" {
			s.cursor = i
			break
		}
	}
	s.refocus()
	for _, r := range "goo" {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if fc.templateName != "google" {
		t.Fatalf("templateName after typing \"goo\" = %q, want \"google\"", fc.templateName)
	}
	if !containsName(s.fields, "opt.page_count") || containsName(s.fields, "opt.geo") {
		t.Errorf("expected google's options after switching away from the default, got %v", fieldNames(s.fields))
	}
}

func TestTierScreenChangingProviderResetsTemplateAndOptions(t *testing.T) {
	fc := &fetchConfig{
		providerName:    "acme",
		templateName:    "search",
		templateQuery:   "sea",
		providerOptions: map[string]string{"query": "shoes"},
	}
	s := newTierScreen("provider", fc, []*provider.Manifest{templatedManifestFixture(), flatOptionsManifestFixture()}, func() {})

	for i, fld := range s.fields {
		if fld.name == "provider" {
			s.cursor = i
			break
		}
	}
	s.refocus()
	s.Update(tea.KeyMsg{Type: tea.KeyRight})

	if fc.providerName != "decodo" {
		t.Fatalf("providerName after cycling = %q, want \"decodo\"", fc.providerName)
	}
	if fc.templateName != "" {
		t.Errorf("templateName after switching provider = %q, want reset to \"\"", fc.templateName)
	}
	if fc.templateQuery != "" {
		t.Errorf("templateQuery after switching provider = %q, want reset to \"\"", fc.templateQuery)
	}
	if len(fc.providerOptions) != 0 {
		t.Errorf("providerOptions after switching provider = %v, want reset to empty", fc.providerOptions)
	}
}

func TestScrapeFormCtrlDRemovesCSSFieldPair(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	f.state.extract = "css fields"
	f.state.cssFields = []cssFieldPair{
		{name: "title", selector: "h1"},
		{name: "price", selector: ".price"},
	}
	f.rebuild()

	for i, fld := range f.fields {
		if fld.name == "css.name.0" {
			f.cursor = i
			break
		}
	}
	f.refocus()

	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	f = updated.(*scrapeForm)

	if len(f.state.cssFields) != 1 {
		t.Fatalf("expected 1 css field remaining, got %d: %v", len(f.state.cssFields), f.state.cssFields)
	}
	if f.state.cssFields[0].name != "price" {
		t.Fatalf("expected the remaining field to be \"price\", got %q", f.state.cssFields[0].name)
	}
}

func TestScrapeFormCtrlDOnNonCSSFieldIsANoOp(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	before := len(f.fields)

	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	f = updated.(*scrapeForm)

	if len(f.fields) != before {
		t.Fatalf("expected ctrl+d on a non-css field to do nothing, field count changed from %d to %d", before, len(f.fields))
	}
}

func TestScrapeFormCursorMovesAndWraps(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	n := len(f.fields)

	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyUp})
	f = updated.(*scrapeForm)
	if f.cursor != n-1 {
		t.Fatalf("cursor after wrapping up from 0 = %d, want %d", f.cursor, n-1)
	}

	updated, _ = f.Update(tea.KeyMsg{Type: tea.KeyDown})
	f = updated.(*scrapeForm)
	if f.cursor != 0 {
		t.Fatalf("cursor after down from the last field = %d, want 0", f.cursor)
	}
}

func TestScrapeFormEscPopsScreen(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to return a popScreen command, got nil")
	}
	if _, ok := cmd().(popScreenMsg); !ok {
		t.Fatalf("expected a popScreenMsg, got %T", cmd())
	}
}

func TestEnterOnProxyTierRowPushesTierScreen(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyDown})
	f = updated.(*scrapeForm)
	updated, _ = f.Update(tea.KeyMsg{Type: tea.KeyDown})
	f = updated.(*scrapeForm)
	if f.fields[f.cursor].name != "proxyTier" {
		t.Fatalf("expected cursor on proxyTier, got %q", f.fields[f.cursor].name)
	}

	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on proxyTier to return a push command")
	}
	push, ok := cmd().(pushScreenMsg)
	if !ok {
		t.Fatalf("expected a pushScreenMsg, got %T", cmd())
	}
	ts, ok := push.screen.(*tierScreen)
	if !ok {
		t.Fatalf("expected the pushed screen to be *tierScreen, got %T", push.screen)
	}
	if ts.tier != "proxy" {
		t.Fatalf("pushed tierScreen.tier = %q, want %q", ts.tier, "proxy")
	}
}

func TestScrapeFormAddFieldAppendsCSSPair(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	f.state.extract = "css fields"
	f.rebuild()

	for i, fld := range f.fields {
		if fld.name == addFieldName {
			f.cursor = i
			break
		}
	}
	f.rebuild()

	before := len(f.state.cssFields)
	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f = updated.(*scrapeForm)

	if len(f.state.cssFields) != before+1 {
		t.Fatalf("expected cssFields to grow from %d to %d, got %d", before, before+1, len(f.state.cssFields))
	}
	if !containsName(f.fields, "css.name.0") {
		t.Errorf("expected a css.name.0 field after adding, got %v", fieldNames(f.fields))
	}
}

func TestScrapeFormTypingIntoURLUpdatesTheLiveField(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	for _, r := range "https://example.com" {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = updated.(*scrapeForm)
	}
	if got := f.fields[f.cursor].value(); got != "https://example.com" {
		t.Fatalf("focused field value = %q, want %q", got, "https://example.com")
	}
	f.syncStateFromFields()
	if f.state.url != "https://example.com" {
		t.Fatalf("state.url after an explicit sync = %q, want %q", f.state.url, "https://example.com")
	}
}

func TestScrapeFormLeftRightMovesCursorWithinURLWithoutResettingIt(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	for _, r := range "helloworld" {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = updated.(*scrapeForm)
	}
	for range 5 {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyLeft})
		f = updated.(*scrapeForm)
	}
	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	f = updated.(*scrapeForm)

	if got, want := f.fields[f.cursor].value(), "helloXworld"; got != want {
		t.Fatalf("URL value = %q, want %q (left arrow should move the cursor, not reset it)", got, want)
	}
}

func TestScrapeFormEnterOnURLAddsANewLineInsteadOfLeavingTheField(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	for _, r := range "https://a.test" {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = updated.(*scrapeForm)
	}
	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f = updated.(*scrapeForm)
	for _, r := range "https://b.test" {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = updated.(*scrapeForm)
	}

	if f.fields[f.cursor].name != "url" {
		t.Fatalf("expected enter to stay on the url field, cursor is on %q", f.fields[f.cursor].name)
	}
	want := "https://a.test\nhttps://b.test"
	if got := f.fields[f.cursor].value(); got != want {
		t.Fatalf("url field value = %q, want %q", got, want)
	}
}

func TestScrapeFormUpDownWithinURLMovesLinesNotFields(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	for _, r := range "https://a.test" {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = updated.(*scrapeForm)
	}
	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f = updated.(*scrapeForm)
	for _, r := range "https://b.test" {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = updated.(*scrapeForm)
	}

	updated, _ = f.Update(tea.KeyMsg{Type: tea.KeyUp})
	f = updated.(*scrapeForm)
	if f.fields[f.cursor].name != "url" {
		t.Fatalf("expected up to stay on the url field while it has more lines, cursor is on %q", f.fields[f.cursor].name)
	}

	updated, _ = f.Update(tea.KeyMsg{Type: tea.KeyUp})
	f = updated.(*scrapeForm)
	if f.fields[f.cursor].name == "url" {
		t.Fatal("expected up from the url field's top line to move to the previous form field")
	}
}

func TestURLListSplitsAndTrimsLinesDroppingBlanks(t *testing.T) {
	s := newFormState()
	s.url = "  https://a.test  \n\nhttps://b.test\n   \n"
	got := urlList(s)
	want := []string{"https://a.test", "https://b.test"}
	if len(got) != len(want) {
		t.Fatalf("urlList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("urlList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestScrapeFormNavigatingAwayAndBackPreservesTypedText(t *testing.T) {
	f := newScrapeForm(nil, context.Background(), "", "")
	for _, r := range "https://example.com" {
		updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = updated.(*scrapeForm)
	}
	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyDown})
	f = updated.(*scrapeForm)
	updated, _ = f.Update(tea.KeyMsg{Type: tea.KeyUp})
	f = updated.(*scrapeForm)

	if got := f.fields[f.cursor].value(); got != "https://example.com" {
		t.Fatalf("URL value after navigating away and back = %q, want %q", got, "https://example.com")
	}
}

func TestNumberFieldArrowsIncrementAndDecrementWithFloor(t *testing.T) {
	f := numberField("retries", "Retries", "", "3", 0, nil)
	f = f.focus()

	updated, changed := f.update(tea.KeyMsg{Type: tea.KeyRight})
	if !changed || updated.value() != "4" {
		t.Fatalf("right arrow: value = %q, changed = %v, want \"4\", true", updated.value(), changed)
	}

	updated, _ = updated.update(tea.KeyMsg{Type: tea.KeyLeft})
	updated, changed = updated.update(tea.KeyMsg{Type: tea.KeyLeft})
	if updated.value() != "2" {
		t.Fatalf("value after two left arrows = %q, want \"2\"", updated.value())
	}

	for range 5 {
		updated, changed = updated.update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	if !changed || updated.value() != "0" {
		t.Fatalf("value floored at min = %q, want \"0\"", updated.value())
	}
}

func TestChoiceFieldTypingJumpsToMatchingOption(t *testing.T) {
	f := choiceField("template", "Template", "", []string{"universal", "google", "bing", "amazon"}, "universal")
	f = f.focus()

	updated, changed := f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if !changed || updated.value() != "google" {
		t.Fatalf(`typing "g": value = %q, changed = %v, want "google", true`, updated.value(), changed)
	}

	updated, _ = updated.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if updated.value() != "google" {
		t.Fatalf(`typing "go": value = %q, want it to stay on "google"`, updated.value())
	}
}

func TestChoiceFieldTypingRestartsFilterWhenNothingMatches(t *testing.T) {
	f := choiceField("template", "Template", "", []string{"universal", "google", "bing", "amazon"}, "universal")
	f = f.focus()

	updated, _ := f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	updated, _ = updated.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	if updated.value() != "google" {
		t.Fatalf(`typing "gz" (no match): value = %q, want unchanged "google"`, updated.value())
	}

	updated, changed := updated.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if !changed || updated.value() != "bing" {
		t.Fatalf(`typing "b" after a failed match: value = %q, changed = %v, want "bing", true (filter restarts from just the new character)`, updated.value(), changed)
	}
}

func TestChoiceFieldFilterResetsOnFocus(t *testing.T) {
	f := choiceField("template", "Template", "", []string{"universal", "google", "bing", "amazon"}, "universal")
	f = f.focus()
	f, _ = f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	f = f.blur().focus()

	updated, changed := f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !changed || updated.value() != "amazon" {
		t.Fatalf(`typing "a" after refocusing: value = %q, changed = %v, want "amazon", true (stale "g" filter must not carry over)`, updated.value(), changed)
	}
}

func TestChoiceFieldArrowsStillCycleAndResetFilter(t *testing.T) {
	f := choiceField("template", "Template", "", []string{"universal", "google", "bing", "amazon"}, "universal")
	f = f.focus()
	f, _ = f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})

	updated, changed := f.update(tea.KeyMsg{Type: tea.KeyRight})
	if !changed || updated.value() != "bing" {
		t.Fatalf("right arrow after typing: value = %q, changed = %v, want \"bing\", true", updated.value(), changed)
	}
}

func TestSearchFieldStartsResolvedToTheDefaultWhenEmpty(t *testing.T) {
	f := templateSearchField([]string{"universal", "google", "bing"}, "universal", "")
	if got := f.value(); got != "universal" {
		t.Fatalf("value() on an empty search field = %q, want the default %q", got, "universal")
	}
	if got := f.render(true, 20); !strings.Contains(got, "(universal)") {
		t.Fatalf("render() = %q, want it to show the resolved default in the hint even with nothing typed", got)
	}
}

func TestSearchFieldTypingResolvesToTheMatchingOption(t *testing.T) {
	f := templateSearchField([]string{"universal", "google", "bing"}, "universal", "")
	f = f.focus()

	for _, r := range "goo" {
		f, _ = f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if f.value() != "google" {
		t.Fatalf("value() after typing \"goo\" = %q, want \"google\"", f.value())
	}
	if got := f.render(true, 20); !strings.Contains(got, "goo") || !strings.Contains(got, "(google)") {
		t.Fatalf("render() = %q, want the typed text \"goo\" and the resolved \"(google)\" hint", got)
	}
}

func TestSearchFieldNonMatchingTextKeepsThePreviousMatch(t *testing.T) {
	f := templateSearchField([]string{"universal", "google", "bing"}, "universal", "")
	f = f.focus()
	for _, r := range "goo" {
		f, _ = f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	updated, changed := f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	if changed || updated.value() != "google" {
		t.Fatalf(`typing "z" after "goo" (no match for "gooz"): value = %q, changed = %v, want unchanged "google", false`, updated.value(), changed)
	}
}

func TestSearchFieldClearingTheTextRevertsToTheDefault(t *testing.T) {
	f := templateSearchField([]string{"universal", "google", "bing"}, "universal", "")
	f = f.focus()
	for _, r := range "goo" {
		f, _ = f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	for range 3 {
		f, _ = f.update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	if got := f.value(); got != "universal" {
		t.Fatalf("value() after clearing the typed text = %q, want reverted to the default %q", got, "universal")
	}
}

func TestSearchFieldQueryTextSurvivesARebuild(t *testing.T) {
	m := &provider.Manifest{
		Name: "decodo",
		Templates: []provider.Template{
			{Name: "universal", Default: true, Options: []provider.Option{{Name: "geo", Type: "string"}}},
			{Name: "google", Options: []provider.Option{{Name: "page_count", Type: "int"}}},
		},
	}
	fc := &fetchConfig{providerName: "decodo"}
	s := newTierScreen("provider", fc, []*provider.Manifest{m}, func() {})

	for i, fld := range s.fields {
		if fld.name == "template" {
			s.cursor = i
			break
		}
	}
	s.refocus()
	for _, r := range "goo" {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	tmpl, ok := findField(s.fields, "template")
	if !ok {
		t.Fatal("expected a \"template\" field after typing")
	}
	if got := tmpl.rawText(); got != "goo" {
		t.Fatalf("template field's typed text after the rebuild triggered by resolving to google = %q, want \"goo\" preserved", got)
	}
}

func TestNumberFieldArrowsClampAtDeclaredMax(t *testing.T) {
	max := 10
	f := numberField("opt.page_count", "Page count", "", "9", 1, &max)
	f = f.focus()

	for range 3 {
		f, _ = f.update(tea.KeyMsg{Type: tea.KeyRight})
	}
	if got := f.value(); got != "10" {
		t.Fatalf("value after 3 right arrows past the max = %q, want clamped to \"10\"", got)
	}
}

func TestNumberFieldBackspaceRemovesFromTheEndAfterArrowAdjustment(t *testing.T) {
	f := numberField("concurrency", "Concurrency", "", "8", 1, nil)
	f = f.focus()

	for range 3 {
		f, _ = f.update(tea.KeyMsg{Type: tea.KeyRight})
	}
	if f.value() != "11" {
		t.Fatalf("value after 3 right arrows = %q, want \"11\"", f.value())
	}

	f, _ = f.update(tea.KeyMsg{Type: tea.KeyLeft})
	f, _ = f.update(tea.KeyMsg{Type: tea.KeyBackspace})
	f, _ = f.update(tea.KeyMsg{Type: tea.KeyBackspace})
	if f.value() != "" {
		t.Fatalf("value after two backspaces on \"10\" = %q, want \"\"", f.value())
	}

	f, _ = f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("42")})
	if f.value() != "42" {
		t.Fatalf("value after typing \"42\" = %q, want \"42\"", f.value())
	}
}

func TestNumberFieldIsHandEditable(t *testing.T) {
	f := numberField("concurrency", "Concurrency", "", "8", 1, nil)
	f = f.focus()

	updated, _ := f.update(tea.KeyMsg{Type: tea.KeyBackspace})
	updated, _ = updated.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("16")})

	if got := updated.value(); got != "16" {
		t.Fatalf("value after backspace + typing = %q, want \"16\"", got)
	}
}
