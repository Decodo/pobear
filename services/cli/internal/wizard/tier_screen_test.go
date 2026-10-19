package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decodo/pobear/pkg/provider"
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

func TestTierScreenProviderOpenedThenEscDoesNotConfigureAProvider(t *testing.T) {
	fc := &fetchConfig{}
	manifests := []*provider.Manifest{{Name: "acme"}, {Name: "decodo"}}
	s := newTierScreen("provider", fc, manifests, func() {})
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	s.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if fc.providerName != "" {
		t.Fatalf("fc.providerName = %q after opening and leaving the Provider screen untouched, want \"\"", fc.providerName)
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

func TestTierScreenViewShowsItsOwnBackHintNotTheMainFormsHint(t *testing.T) {
	fc := &fetchConfig{profile: "chrome"}
	s := newTierScreen("proxy", fc, nil, func() {})
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := s.View()
	if !strings.Contains(view, "esc: back") {
		t.Fatalf("View() = %q, want it to contain %q", view, "esc: back")
	}
	if strings.Contains(view, "esc: back to menu") {
		t.Fatalf("View() = %q, want it NOT to contain %q -- esc on a tier screen pops back to the parent form, not to the top-level menu", view, "esc: back to menu")
	}
}
