package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
	fields := computeFields(newFormState(), nil)
	want := []string{"url", "fetchVia", "profile", "extract", "concurrency", "retries"}
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

func TestComputeFieldsFetchViaProxyShowsProxyURLAndProfile(t *testing.T) {
	s := newFormState()
	s.fetchVia = "proxy"
	fields := computeFields(s, nil)
	if !containsName(fields, "proxyURL") {
		t.Error("expected a \"proxyURL\" field when fetchVia is proxy")
	}
	if !containsName(fields, "profile") {
		t.Error("expected a \"profile\" field when fetchVia is proxy -- fetch.profile is valid for both local and proxy")
	}
}

func TestComputeFieldsFetchViaProviderShowsProviderNotProfile(t *testing.T) {
	s := newFormState()
	s.fetchVia = "provider"
	fields := computeFields(s, nil)
	if containsName(fields, "profile") {
		t.Error("expected no \"profile\" field when fetchVia is provider")
	}
	if !containsName(fields, "provider") {
		t.Error("expected a \"provider\" field when fetchVia is provider")
	}
}

func TestComputeFieldsExtractFieldsShowsCSSEditor(t *testing.T) {
	s := newFormState()
	s.extract = "css fields"
	s.cssFields = []cssFieldPair{{name: "title", selector: "h1"}}
	fields := computeFields(s, nil)
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
		fields := computeFields(s, nil)
		if containsName(fields, addFieldName) {
			t.Errorf("extract=%s: expected no css editor, got %v", mode, fieldNames(fields))
		}
	}
}

func TestComputeFieldsProfileIsAChoiceOfThree(t *testing.T) {
	s := newFormState()
	fields := computeFields(s, nil)
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

func TestComputeFieldsProviderIsAChoiceOfKnownProviders(t *testing.T) {
	s := newFormState()
	s.fetchVia = "provider"
	fields := computeFields(s, []string{"acme", "decodo"})
	provider, ok := findField(fields, "provider")
	if !ok {
		t.Fatal("expected a \"provider\" field")
	}
	if provider.kind != fieldChoice {
		t.Fatalf("expected provider to be a choice field, got kind %v", provider.kind)
	}
	want := []string{"acme", "decodo"}
	if len(provider.options) != len(want) {
		t.Fatalf("provider options = %v, want %v", provider.options, want)
	}
	for i := range want {
		if provider.options[i] != want[i] {
			t.Errorf("provider option %d = %q, want %q", i, provider.options[i], want[i])
		}
	}
}

func TestComputeFieldsProviderWithNoneConfiguredStillRenders(t *testing.T) {
	s := newFormState()
	s.fetchVia = "provider"
	fields := computeFields(s, nil)
	provider, ok := findField(fields, "provider")
	if !ok {
		t.Fatal("expected a \"provider\" field even with no providers configured")
	}
	if len(provider.options) == 0 {
		t.Fatal("expected a placeholder option when no providers are configured")
	}
}

func TestScrapeFormCtrlDRemovesCSSFieldPair(t *testing.T) {
	f := newScrapeForm(nil)
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
	f := newScrapeForm(nil)
	before := len(f.fields)

	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	f = updated.(*scrapeForm)

	if len(f.fields) != before {
		t.Fatalf("expected ctrl+d on a non-css field to do nothing, field count changed from %d to %d", before, len(f.fields))
	}
}

func TestScrapeFormCursorMovesAndWraps(t *testing.T) {
	f := newScrapeForm(nil)
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
	f := newScrapeForm(nil)
	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to return a popScreen command, got nil")
	}
	if _, ok := cmd().(popScreenMsg); !ok {
		t.Fatalf("expected a popScreenMsg, got %T", cmd())
	}
}

func TestScrapeFormChangingFetchViaKeepsItFocused(t *testing.T) {
	f := newScrapeForm(nil)
	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyDown})
	f = updated.(*scrapeForm)
	if f.fields[f.cursor].name != "fetchVia" {
		t.Fatalf("expected cursor on fetchVia, got %q", f.fields[f.cursor].name)
	}

	updated, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f = updated.(*scrapeForm)

	if f.state.fetchVia != "proxy" {
		t.Fatalf("expected fetchVia to advance to \"proxy\", got %q", f.state.fetchVia)
	}
	if f.fields[f.cursor].name != "fetchVia" {
		t.Fatalf("expected focus to stay on fetchVia after the field list changed, got %q", f.fields[f.cursor].name)
	}
}

func TestScrapeFormAddFieldAppendsCSSPair(t *testing.T) {
	f := newScrapeForm(nil)
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
	f := newScrapeForm(nil)
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
	f := newScrapeForm(nil)
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

func TestScrapeFormNavigatingAwayAndBackPreservesTypedText(t *testing.T) {
	f := newScrapeForm(nil)
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
	f := numberField("retries", "Retries", "3", 0)
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

func TestNumberFieldBackspaceRemovesFromTheEndAfterArrowAdjustment(t *testing.T) {
	f := numberField("concurrency", "Concurrency", "8", 1)
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
	f := numberField("concurrency", "Concurrency", "8", 1)
	f = f.focus()

	updated, _ := f.update(tea.KeyMsg{Type: tea.KeyBackspace})
	updated, _ = updated.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("16")})

	if got := updated.value(); got != "16" {
		t.Fatalf("value after backspace + typing = %q, want \"16\"", got)
	}
}
