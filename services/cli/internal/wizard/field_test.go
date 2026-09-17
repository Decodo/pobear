package wizard

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func renderTextAreaField(f field) field {
	f.textarea.View()
	return f
}

func typeTextArea(f field, s string) field {
	for _, r := range s {
		f, _ = f.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		f = renderTextAreaField(f)
	}
	return f
}

func pressEnter(f field) field {
	f, _ = f.update(tea.KeyMsg{Type: tea.KeyEnter})
	return renderTextAreaField(f)
}

func TestTextAreaFieldStartsAtOneVisibleRow(t *testing.T) {
	f := textAreaField("URL", "", "", 4, 60)
	if got := f.textarea.Height(); got != 1 {
		t.Fatalf("initial textarea height = %d, want 1", got)
	}
}

func TestTextAreaFieldGrowsWithEachLineUpToMax(t *testing.T) {
	f := textAreaField("URL", "", "", 4, 60).focus()

	for i := 1; i <= 3; i++ {
		f = pressEnter(f)
		if got := f.textarea.Height(); got != i+1 {
			t.Fatalf("after %d enter(s), textarea height = %d, want %d", i, got, i+1)
		}
	}

	f = pressEnter(f)
	if got := f.textarea.Height(); got != 4 {
		t.Fatalf("textarea height after exceeding maxVisible = %d, want capped at 4", got)
	}
	if got := f.textarea.LineCount(); got != 5 {
		t.Fatalf("LineCount() = %d, want 5 (no cap on the number of URL lines)", got)
	}
}

func TestTextAreaFieldHasNoLimitOnLineCount(t *testing.T) {
	f := textAreaField("URL", "", "", 4, 60).focus()
	for i := 0; i < 20; i++ {
		f = typeTextArea(f, "https://a.test/"+strconv.Itoa(i))
		f = pressEnter(f)
	}
	if got := f.textarea.LineCount(); got != 21 {
		t.Fatalf("LineCount() = %d, want 21 (20 lines plus the trailing one after the last enter)", got)
	}
	if got := f.textarea.Height(); got != 4 {
		t.Fatalf("textarea height = %d, want capped at 4 regardless of line count", got)
	}
}

func TestTextAreaFieldRebuildPreservesGrownHeight(t *testing.T) {
	multiline := "https://a.test\nhttps://b.test\nhttps://c.test"
	f := textAreaField("URL", "", multiline, 4, 60)
	if got := f.textarea.Height(); got != 3 {
		t.Fatalf("textarea height for a fresh field built from a 3-line value = %d, want 3", got)
	}
}

func TestTextAreaFieldShowsFromTopWhileWithinMaxVisible(t *testing.T) {
	f := textAreaField("URL", "", "https://a.test", 4, 60).focus()
	f = pressEnter(f)
	f = typeTextArea(f, "https://b.test")

	view := f.render(true, 14, 60)
	if !strings.Contains(view, "https://a.test") {
		t.Fatalf("render() = %q, want the first line still visible (view must not scroll away from the top while content still fits)", view)
	}
	if !strings.Contains(view, "https://b.test") {
		t.Fatalf("render() = %q, want the second line visible too", view)
	}
}

func TestTextAreaFieldScrollsOnceContentExceedsMaxVisible(t *testing.T) {
	f := textAreaField("URL", "", "", 4, 60).focus()
	for i := 0; i < 6; i++ {
		f = typeTextArea(f, "https://a.test/"+strconv.Itoa(i))
		f = pressEnter(f)
	}
	view := f.render(true, 14, 60)
	if strings.Contains(view, "https://a.test/0") {
		t.Fatalf("render() = %q, want the first line scrolled out of view once content exceeds maxVisible", view)
	}
	if !strings.Contains(view, "https://a.test/5") {
		t.Fatalf("render() = %q, want the most recently typed line visible", view)
	}
}

func TestTextAreaFieldShowsHintOnlyWhenEmpty(t *testing.T) {
	f := textAreaField("URL", "(one per line)", "", 4, 60).focus()
	if view := f.render(true, 14, 60); !strings.Contains(view, "(one per line)") {
		t.Fatalf("render() on an empty field = %q, want the hint visible", view)
	}

	f = typeTextArea(f, "https://a.test")
	if view := f.render(true, 14, 60); strings.Contains(view, "(one per line)") {
		t.Fatalf("render() after typing = %q, want the hint gone", view)
	}
}

func TestFieldRenderKeepsLineWidthStableWhenFocused(t *testing.T) {
	withForcedColor(t, func() {
		f := textField("x", "Field name", "(ctrl+d remove)", "title")

		unfocused := f.blur().render(false, 14, 14)
		focused := f.focus().render(true, 14, 14)

		if lipgloss.Width(unfocused) != lipgloss.Width(focused) {
			t.Errorf("line width changed when focused: unfocused=%d (%q) focused=%d (%q)",
				lipgloss.Width(unfocused), unfocused, lipgloss.Width(focused), focused)
		}
	})
}
