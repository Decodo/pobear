package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateTracksWindowSize(t *testing.T) {
	m := newModel(nil)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := updated.(model)
	if got.width != 80 || got.height != 24 {
		t.Fatalf("got width=%d height=%d, want 80x24", got.width, got.height)
	}
	if cmd != nil {
		t.Fatalf("expected no command from a resize, got %v", cmd)
	}
}

func TestUpdateCursorMovesAndWraps(t *testing.T) {
	m := newModel(nil)

	down := func(mm model) model {
		updated, _ := mm.Update(tea.KeyMsg{Type: tea.KeyDown})
		return updated.(model)
	}
	up := func(mm model) model {
		updated, _ := mm.Update(tea.KeyMsg{Type: tea.KeyUp})
		return updated.(model)
	}

	if m.cursor != 0 {
		t.Fatalf("expected cursor to start at 0, got %d", m.cursor)
	}

	m = up(m)
	if want := len(menuItems) - 1; m.cursor != want {
		t.Fatalf("cursor after wrapping up from 0 = %d, want %d", m.cursor, want)
	}

	m = down(m)
	if m.cursor != 0 {
		t.Fatalf("cursor after down from the last item = %d, want 0", m.cursor)
	}

	for range len(menuItems) {
		m = down(m)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor after a full lap of down presses = %d, want back at 0", m.cursor)
	}
}

func TestUpdateQReturnsQuitCommand(t *testing.T) {
	m := newModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected 'q' to return a quit command, got nil")
	}
}

func TestUpdateEnterOnScrapePushesForm(t *testing.T) {
	m := newModel(nil)
	m.cursor = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on \"Scrape a page\" to return a pushScreen command, got nil")
	}
	msg := cmd()
	if _, ok := msg.(pushScreenMsg); !ok {
		t.Fatalf("expected a pushScreenMsg, got %T", msg)
	}
}

func TestUpdateEnterOnOtherItemsDoesNothingYet(t *testing.T) {
	m := newModel(nil)
	m.cursor = 1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if got.cursor != 1 {
		t.Fatalf("expected enter to leave the cursor untouched, got %d", got.cursor)
	}
	if cmd != nil {
		t.Fatalf("expected enter to do nothing (no crawl form exists yet), got a command %v", cmd)
	}
}

func TestViewRendersWithoutPanicBeforeAnyWindowSize(t *testing.T) {
	m := newModel(nil)
	_ = m.View()
}
