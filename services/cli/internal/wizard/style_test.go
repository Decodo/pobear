package wizard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func withForcedColor(t *testing.T, fn func()) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
	fn()
}

func TestRenderMenuEmitsColorCodesWhenForced(t *testing.T) {
	withForcedColor(t, func() {
		out := renderMenu(80, 24, 2, false)
		if ansi.Strip(out) == out {
			t.Fatalf("expected renderMenu to emit ANSI color codes when color is forced on")
		}
	})
}

func TestRenderMenuStaysWellFormedWhenColored(t *testing.T) {
	withForcedColor(t, func() {
		out := renderMenu(140, 40, 0, false)
		if ansi.Strip(out) == out {
			t.Fatalf("expected the wide+tall frame (which includes the bear) to contain ANSI color codes")
		}
		lines := strings.Split(out, "\n")
		if len(lines) != 40 {
			t.Fatalf("got %d lines, want 40", len(lines))
		}
		for i, l := range lines {
			if lipgloss.Width(l) != 140 {
				t.Errorf("line %d has width %d, want 140", i, lipgloss.Width(l))
			}
		}
	})
}
