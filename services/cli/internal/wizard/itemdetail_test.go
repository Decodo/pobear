package wizard

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestItemDetailScreenShowsPrettifiedJSON(t *testing.T) {
	row := resultRow{url: "https://a.test", raw: map[string]any{"url": "https://a.test", "status": 200}}
	s := newItemDetailScreen(row)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := s.View()
	if !strings.Contains(view, `"url": "https://a.test"`) || !strings.Contains(view, `"status": 200`) {
		t.Fatalf("View() = %q, want indented JSON with the item's fields", view)
	}
}

func TestItemDetailScreenEscPopsBackToResults(t *testing.T) {
	s := newItemDetailScreen(resultRow{url: "https://a.test", raw: map[string]any{"url": "https://a.test"}})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected esc to return a pop command, got nil")
	}
}

func TestItemDetailScreenWritesItemToFile(t *testing.T) {
	t.Chdir(t.TempDir())

	row := resultRow{url: "https://a.test/page", raw: map[string]any{"url": "https://a.test/page"}}
	s := newItemDetailScreen(row)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	s = updated.(*itemDetailScreen)
	if !strings.HasPrefix(s.status, "✓") {
		t.Fatalf("status after 'w' = %q, want success", s.status)
	}

	name := sanitizeFilename(row.url) + ".json"
	if _, err := os.Stat(name); err != nil {
		t.Fatalf("expected %s to be written: %v", name, err)
	}
}

func TestItemDetailScreenNeverExceedsRequestedSize(t *testing.T) {
	data := map[string]any{}
	for i := 0; i < 60; i++ {
		data[string(rune('a'+i%26))+string(rune('0'+i/26))] = strings.Repeat("x", 5)
	}
	row := resultRow{url: "https://a.test", raw: data}

	assertNeverExceedsSize(t, 10, 40, 3, unsupportedMinW, 15, func(w, h int) string {
		s := newItemDetailScreen(row)
		s.Update(tea.WindowSizeMsg{Width: w, Height: h})
		return s.View()
	})
}

func TestItemDetailScreenLongLinesAreTruncatedNotWrapped(t *testing.T) {
	row := resultRow{url: "https://a.test", raw: map[string]any{"body": strings.Repeat("x", 500)}}
	s := newItemDetailScreen(row)
	s.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	for _, l := range strings.Split(s.View(), "\n") {
		if got := lipgloss.Width(l); got > 60 {
			t.Fatalf("line %d cells wide exceeds requested width 60: %q", got, l)
		}
	}
}

func TestItemDetailScreenTKeyTogglesWrap(t *testing.T) {
	s := newItemDetailScreen(resultRow{url: "https://a.test", raw: map[string]any{"url": "https://a.test"}})
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if s.wrap {
		t.Fatal("expected wrap to start off")
	}
	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	s = updated.(*itemDetailScreen)
	if !s.wrap {
		t.Fatal("expected 't' to turn wrap on")
	}
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	s = updated.(*itemDetailScreen)
	if s.wrap {
		t.Fatal("expected a second 't' to turn wrap back off")
	}
}

func TestItemDetailScreenWrapShowsFullContentInsteadOfTruncating(t *testing.T) {
	longWord := strings.Repeat("x", 500)
	row := resultRow{url: "https://a.test", raw: map[string]any{"body": longWord}}
	s := newItemDetailScreen(row)
	s.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	s.wrap = true

	if strings.Contains(s.View(), "…") {
		t.Fatalf("View() with wrap on = %q, want no truncation marker", s.View())
	}

	joined := strings.Join(s.displayLines(), "")
	if !strings.Contains(joined, longWord) {
		t.Fatalf("displayLines() joined doesn't contain the full %d-char value (wrapping should split it across lines, never drop any of it)", len(longWord))
	}
}

func TestItemDetailScreenWrapNeverExceedsRequestedSize(t *testing.T) {
	row := resultRow{url: "https://a.test", raw: map[string]any{"body": strings.Repeat("hello world ", 40)}}
	s := newItemDetailScreen(row)
	s.wrap = true

	assertNeverExceedsSize(t, 10, 30, 5, unsupportedMinW, 15, func(w, h int) string {
		s.Update(tea.WindowSizeMsg{Width: w, Height: h})
		return s.View()
	})
}

func TestItemDetailScreenScrollsLongJSON(t *testing.T) {
	data := map[string]any{}
	for i := 0; i < 50; i++ {
		data[strings.Repeat("k", 1)+string(rune('a'+i))] = i
	}
	row := resultRow{url: "https://a.test", raw: data}
	s := newItemDetailScreen(row)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 12})

	if s.visibleLines() >= len(s.lines) {
		t.Fatalf("visibleLines() = %d, want fewer than the %d total lines given the small height", s.visibleLines(), len(s.lines))
	}

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s = updated.(*itemDetailScreen)
	if s.offset == 0 {
		t.Fatal("expected down to scroll past offset 0 for long JSON content")
	}
}
