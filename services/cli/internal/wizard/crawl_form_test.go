package wizard

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/provider"
)

func TestComputeCrawlFieldsDefaultState(t *testing.T) {
	fields := computeCrawlFields(newCrawlFormState(), nil)
	want := []string{
		"startingURL", "fetchVia", "profile",
		"depth", "maxPages", "scope", "sitemap", "robots",
		"extract", "rps", "concurrency", "retries", "saveAction", "runAction",
	}
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

func TestComputeCrawlFieldsDefaultValues(t *testing.T) {
	fields := computeCrawlFields(newCrawlFormState(), nil)

	cases := map[string]string{
		"depth": "1", "maxPages": "100", "scope": "same-host",
		"sitemap": "true", "robots": "respect", "rps": "16",
	}
	for name, want := range cases {
		fld, ok := findField(fields, name)
		if !ok {
			t.Fatalf("expected a %q field", name)
		}
		if got := fld.value(); got != want {
			t.Errorf("%s default value = %q, want %q", name, got, want)
		}
	}
}

func TestComputeCrawlFieldsProviderShowsProviderAndOptionFields(t *testing.T) {
	s := newCrawlFormState()
	s.fetchVia = "provider"
	s.providerName = "decodo"

	fields := computeCrawlFields(s, []*provider.Manifest{flatOptionsManifestFixture()})
	if containsName(fields, "profile") {
		t.Error("expected no \"profile\" field when fetchVia is provider")
	}
	if !containsName(fields, "opt.headless") || !containsName(fields, "opt.geo") {
		t.Errorf("expected the provider's option fields, got %v", fieldNames(fields))
	}
}

func TestCrawlFormEscPopsScreen(t *testing.T) {
	f := newCrawlForm(nil, context.Background(), "", "")
	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to return a pop command, got nil")
	}
}

func TestCrawlFormRunRequiresStartingURL(t *testing.T) {
	f := newCrawlForm(nil, context.Background(), "", "")
	for i, fld := range f.fields {
		if fld.name == runActionName {
			f.cursor = i
			break
		}
	}
	f.refocus()

	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f = updated.(*crawlForm)
	if !strings.HasPrefix(f.status, "✗") {
		t.Fatalf("status after Run with no starting URL = %q, want a failure", f.status)
	}
}

func TestCrawlFormRunChecksProviderCredential(t *testing.T) {
	dir := writeManifest(t, "decodo", flatOptionsManifest)
	f := newCrawlForm([]*provider.Manifest{flatOptionsManifestFixture()}, context.Background(), dir, t.TempDir()+"/credentials")
	f.state.startingURL = "https://example.com"
	f.state.fetchVia = "provider"
	f.state.providerName = "decodo"
	f.rebuild()

	for i, fld := range f.fields {
		if fld.name == runActionName {
			f.cursor = i
			break
		}
	}
	f.refocus()

	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyEnter})
	f = updated.(*crawlForm)
	if !strings.HasPrefix(f.status, "✗") || !strings.Contains(f.status, "decodo") {
		t.Fatalf("status after Run with no stored credential = %q, want a decodo credential failure", f.status)
	}
}

func TestCrawlFormCtrlDRemovesCSSFieldPair(t *testing.T) {
	f := newCrawlForm(nil, context.Background(), "", "")
	f.state.extract = "css fields"
	f.state.cssFields = []cssFieldPair{{name: "title", selector: "h1"}}
	f.rebuild()

	for i, fld := range f.fields {
		if fld.name == "css.name.0" {
			f.cursor = i
			break
		}
	}
	f.refocus()

	updated, _ := f.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	f = updated.(*crawlForm)
	if len(f.state.cssFields) != 0 {
		t.Fatalf("cssFields after ctrl+d = %v, want empty", f.state.cssFields)
	}
}
