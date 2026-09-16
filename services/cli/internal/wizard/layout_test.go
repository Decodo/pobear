package wizard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name          string
		width, height int
		want          layoutKind
	}{
		{"just below width floor", 35, 24, layoutUnsupported},
		{"just below height floor", 80, 7, layoutUnsupported},
		{"at the width floor", 36, 8, layoutCompact},
		{"top of compact width range", 59, 24, layoutCompact},
		{"compact regardless of generous height", 40, 100, layoutCompact},
		{"just above compact, short height", 60, 8, layoutShort},
		{"top of short height range", 80, 11, layoutShort},
		{"standard", 80, 12, layoutStandard},
		{"top of standard height range", 80, 27, layoutStandard},
		{"standard tall (bear on top)", 80, 28, layoutStandardTall},
		{"just below wide width, tall enough for bear-on-top", 99, 40, layoutStandardTall},
		{"wide and just past the short cutoff shows the bear on the side", 100, 12, layoutWideRoomy},
		{"very wide and tall stays wide-roomy, not standard-tall", 140, 40, layoutWideRoomy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classify(c.width, c.height); got != c.want {
				t.Errorf("classify(%d, %d) = %v, want %v", c.width, c.height, got, c.want)
			}
		})
	}
}

func TestRenderMenuEveryNonUnsupportedSizeProducesExactCanvas(t *testing.T) {
	sizes := [][2]int{
		{36, 8}, {40, 15}, {59, 60}, {60, 8}, {80, 11}, {80, 24},
		{99, 40}, {100, 12}, {100, 16}, {200, 60},
	}
	for _, wh := range sizes {
		w, h := wh[0], wh[1]
		out := renderMenu(w, h, 0, false)
		lines := strings.Split(out, "\n")
		if len(lines) != h {
			t.Errorf("renderMenu(%d,%d): got %d lines, want %d", w, h, len(lines), h)
			continue
		}
		for i, l := range lines {
			if lipgloss.Width(l) != w {
				t.Errorf("renderMenu(%d,%d): line %d width = %d, want %d", w, h, i, lipgloss.Width(l), w)
			}
		}
	}
}
