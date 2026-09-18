package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/provider"
)

func TestFormatProviderDetailFlatOptions(t *testing.T) {
	m := &provider.Manifest{
		Name:     "acme",
		Endpoint: "https://acme.example.com/scrape",
		Options: []provider.Option{
			{Name: "device", Type: "enum", Values: []string{"desktop", "mobile"}, Default: "desktop"},
		},
	}

	lines := formatProviderDetail(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "acme") || !strings.Contains(joined, "https://acme.example.com/scrape") {
		t.Fatalf("formatProviderDetail = %v, want the name and endpoint", lines)
	}
	if !strings.Contains(joined, "device") || !strings.Contains(joined, "desktop, mobile") {
		t.Fatalf("formatProviderDetail = %v, want the device option and its enum values", lines)
	}
}

func TestFormatProviderDetailTemplates(t *testing.T) {
	m := &provider.Manifest{
		Name: "decodo",
		Templates: []provider.Template{
			{Name: "universal", Default: true, Options: []provider.Option{{Name: "geo", Type: "string"}}},
			{Name: "google", Options: []provider.Option{{Name: "page_count", Type: "int"}}},
		},
	}

	lines := formatProviderDetail(m)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "universal (default)") {
		t.Fatalf("formatProviderDetail = %v, want \"universal (default)\"", lines)
	}
	if !strings.Contains(joined, "google") || strings.Contains(joined, "google (default)") {
		t.Fatalf("formatProviderDetail = %v, want google listed without \"(default)\"", lines)
	}
}

func TestProviderDetailScreenScrolls(t *testing.T) {
	opts := make([]provider.Option, 40)
	for i := range opts {
		opts[i] = provider.Option{Name: strings.Repeat("x", 1) + string(rune('a'+i)), Type: "string"}
	}
	m := &provider.Manifest{Name: "acme", Endpoint: "https://acme.example.com", Options: opts}
	s := newProviderDetailScreen(m)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 12})

	if s.visibleLines() >= len(s.lines) {
		t.Fatalf("visibleLines() = %d, want fewer than the %d total lines given the small height", s.visibleLines(), len(s.lines))
	}

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s = updated.(*providerDetailScreen)
	if s.offset == 0 {
		t.Fatal("expected down to scroll past offset 0 for a long option list")
	}
}

func TestProviderDetailScreenEscPopsScreen(t *testing.T) {
	s := newProviderDetailScreen(&provider.Manifest{Name: "acme"})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to return a pop command, got nil")
	}
}
