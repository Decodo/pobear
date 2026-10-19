package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/fault"
)

func TestResultsScreenEscPopsBackToForm(t *testing.T) {
	s := newResultsScreen([]resultRow{{url: "https://a.test", status: "✓ 200"}}, "✓ fetched 1, errors 0, skipped 0")
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected esc to return a pop command, got nil")
	}
}

func TestResultsScreenShowsOneRowForOneURL(t *testing.T) {
	s := newResultsScreen([]resultRow{{url: "https://a.test", status: "✓ 200", detail: "raw: <h1>hi</h1>"}}, "✓ fetched 1, errors 0, skipped 0")
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := s.View()
	if !strings.Contains(view, "https://a.test") || !strings.Contains(view, "200") {
		t.Fatalf("View() = %q, want it to contain the URL and its status", view)
	}
}

func TestResultsScreenScrollsWithinBounds(t *testing.T) {
	rows := make([]resultRow, 20)
	for i := range rows {
		rows[i] = resultRow{url: "https://a.test/" + string(rune('a'+i)), status: "✓"}
	}
	s := newResultsScreen(rows, "✓ fetched 20, errors 0, skipped 0")
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 12})

	if s.visibleRows() >= len(rows) {
		t.Fatalf("visibleRows() = %d, want fewer than the %d total rows given the small height", s.visibleRows(), len(rows))
	}

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyUp})
	s = updated.(*resultsScreen)
	if s.offset != 0 {
		t.Fatalf("offset after up-at-top = %d, want 0", s.offset)
	}

	for i := 0; i < len(rows)+5; i++ {
		updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyDown})
		s = updated.(*resultsScreen)
	}
	if s.offset != s.maxOffset() {
		t.Fatalf("offset after scrolling past the end = %d, want maxOffset()=%d", s.offset, s.maxOffset())
	}

	view := s.View()
	if !strings.Contains(view, "of 20") {
		t.Fatalf("View() = %q, want a scroll indicator naming the total row count", view)
	}
}

func TestFormatResultRowTruncatesLongURLsAndDetails(t *testing.T) {
	row := resultRow{
		url:    "https://example.test/a/very/long/path/that/exceeds/the/column/width/for/sure",
		status: "✓ 200",
		detail: strings.Repeat("x", 200),
	}
	line := formatResultRow(row, false, formInnerWidth)
	if strings.Contains(line, strings.Repeat("x", 200)) {
		t.Fatalf("formatResultRow did not truncate the detail: %q", line)
	}
	if !strings.Contains(line, "…") {
		t.Fatalf("formatResultRow = %q, want a truncation marker somewhere", line)
	}
}

func TestTruncatePlainLeavesShortStringsUntouched(t *testing.T) {
	if got := truncatePlain("hi", 10); got != "hi" {
		t.Fatalf("truncatePlain(short) = %q, want unchanged %q", got, "hi")
	}
}

func TestResultsScreenEnterOpensItemDetailForTheSelectedRow(t *testing.T) {
	rows := []resultRow{
		{url: "https://a.test", raw: map[string]any{"url": "https://a.test"}},
		{url: "https://b.test", raw: map[string]any{"url": "https://b.test"}},
	}
	s := newResultsScreen(rows, "✓ fetched 2, errors 0, skipped 0")
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s = updated.(*resultsScreen)
	if s.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", s.cursor)
	}

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to return a push command, got nil")
	}
	msg := cmd()
	push, ok := msg.(pushScreenMsg)
	if !ok {
		t.Fatalf("expected a pushScreenMsg, got %T", msg)
	}
	detail, ok := push.screen.(*itemDetailScreen)
	if !ok {
		t.Fatalf("expected the pushed screen to be *itemDetailScreen, got %T", push.screen)
	}
	if detail.row.url != rows[1].url {
		t.Fatalf("pushed detail row url = %q, want %q (the selected row)", detail.row.url, rows[1].url)
	}
}

func TestResultsScreenShowsFirstRowAtTopForALargeList(t *testing.T) {
	rows := make([]resultRow, 36)
	for i := range rows {
		rows[i] = resultRow{url: "https://a.test/" + string(rune('a'+i)), status: "✓ 200"}
	}
	s := newResultsScreen(rows, "✓ fetched 36, errors 0, skipped 0")
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	view := s.View()
	if !strings.Contains(view, rows[0].url) {
		t.Fatalf("View() = %q, want the first row visible at the top of a fresh results screen", view)
	}
}

func TestResultsScreenNeverExceedsRequestedSize(t *testing.T) {
	rows := make([]resultRow, 36)
	for i := range rows {
		rows[i] = resultRow{url: "https://a.test/" + string(rune('a'+i%26)), status: "✓ 200", detail: "some extracted detail text"}
	}
	s := newResultsScreen(rows, "✓ fetched 36, errors 0, skipped 0")

	assertNeverExceedsSize(t, resultsMinHeight, 40, 3, resultsMinWidth, 10, func(w, h int) string {
		s.width, s.height = w, h
		s.clampScroll()
		return s.View()
	})
}

func TestResultsScreenNeverExceedsRequestedSizeWithTip(t *testing.T) {
	rows := make([]resultRow, 36)
	for i := range rows {
		rows[i] = resultRow{url: "https://a.test/" + string(rune('a'+i%26)), status: "✗", detail: "some error detail text", kind: fault.Blocked}
	}
	s := newResultsScreen(rows, "✗ fetched 0, errors 36, skipped 0")

	assertNeverExceedsSize(t, resultsMinHeight, 40, 3, resultsMinWidth, 10, func(w, h int) string {
		s.width, s.height = w, h
		s.clampScroll()
		return s.View()
	})
}

func TestHasEscalatableFailureDetectsBlockedAndRateLimit(t *testing.T) {
	tests := []struct {
		name string
		rows []resultRow
		want bool
	}{
		{"no rows", nil, false},
		{"all succeeded", []resultRow{{status: "✓ 200"}}, false},
		{"failed but not escalatable", []resultRow{{status: "✗", kind: fault.Network}}, false},
		{"blocked", []resultRow{{status: "✗", kind: fault.Blocked}}, true},
		{"rate limited", []resultRow{{status: "✗", kind: fault.RateLimit}}, true},
		{"mixed, one blocked", []resultRow{{status: "✓ 200"}, {status: "✗", kind: fault.Blocked}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasEscalatableFailure(tt.rows); got != tt.want {
				t.Errorf("hasEscalatableFailure(%v) = %v, want %v", tt.rows, got, tt.want)
			}
		})
	}
}

func TestResultsScreenShowsEscalationTipOnlyWhenBlockedOrRateLimited(t *testing.T) {
	blocked := []resultRow{{url: "https://a.test", status: "✗", detail: "blocked", kind: fault.Blocked}}
	s := newResultsScreen(blocked, "✗ fetched 0, errors 1, skipped 0")
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if view := s.View(); !strings.Contains(view, "Tip:") {
		t.Fatalf("View() with a blocked failure = %q, want it to contain an escalation tip", view)
	}

	clean := []resultRow{{url: "https://a.test", status: "✓ 200"}}
	s2 := newResultsScreen(clean, "✓ fetched 1, errors 0, skipped 0")
	s2.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if view := s2.View(); strings.Contains(view, "Tip:") {
		t.Fatalf("View() with no escalatable failure = %q, want no tip", view)
	}
}

func TestResultsScreenBoxWidthIsCappedOnWideTerminals(t *testing.T) {
	rows := []resultRow{{url: "https://a.test", status: "✓ 200"}}
	s := newResultsScreen(rows, "✓ fetched 1, errors 0, skipped 0")
	s.Update(tea.WindowSizeMsg{Width: 220, Height: 30})

	if got := s.innerWidth(); got != formInnerWidth {
		t.Fatalf("innerWidth() on a 220-wide terminal = %d, want capped at formInnerWidth=%d", got, formInnerWidth)
	}
}

func TestResultsScreenWKeyWritesAllItems(t *testing.T) {
	t.Chdir(t.TempDir())

	rows := []resultRow{{url: "https://a.test", raw: map[string]any{"url": "https://a.test"}}}
	s := newResultsScreen(rows, "✓ fetched 1, errors 0, skipped 0")
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	s = updated.(*resultsScreen)
	if !strings.HasPrefix(s.status, "✓") {
		t.Fatalf("status after 'w' = %q, want success", s.status)
	}
}
