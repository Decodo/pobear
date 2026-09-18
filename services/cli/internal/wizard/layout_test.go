package wizard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func assertNeverExceedsSize(t *testing.T, minH, maxH, stepH, minW, maxW, stepW int, view func(w, h int) string) {
	t.Helper()
	for h := minH; h <= maxH; h += stepH {
		for w := minW; w <= maxW; w += stepW {
			lines := strings.Split(view(w, h), "\n")
			if len(lines) != h {
				t.Fatalf("view(%d,%d): got %d lines, want exactly %d", w, h, len(lines), h)
			}
			for _, l := range lines {
				if got := lipgloss.Width(l); got > w {
					t.Fatalf("view(%d,%d): a line is %d cells wide, want <= %d:\n%q", w, h, got, w, l)
				}
			}
		}
	}
}

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

func TestRenderFormNeverExceedsRequestedSize(t *testing.T) {
	fields := computeFields(newFormState(), nil)

	renderedLines := 0
	for i, f := range fields {
		renderedLines += strings.Count(f.render(i == 0, formValueWidth), "\n") + 1
	}
	minHeight := renderedLines + 4 + footerRows
	for h := minHeight; h <= 40; h++ {
		for w := 60; w <= 160; w += 5 {
			out := renderForm(w, h, "Scrape", fields, 0, "")
			lines := strings.Split(out, "\n")
			if len(lines) != h {
				t.Fatalf("renderForm(%d,%d): got %d lines, want exactly %d", w, h, len(lines), h)
			}
		}
	}
}

func TestDecideBearPlacement(t *testing.T) {
	const boxW, boxH = 86, 10
	const bearW, bearH = 18, 6

	cases := []struct {
		name                  string
		termWidth, termHeight int
		want                  bearPlacement
	}{
		{"box barely fits, no room for anything else", boxW, boxH, bearNone},
		{"wide enough for side, but not tall enough for the box+footer", boxW + bearGap + bearW, boxH + footerRows - 1, bearNone},
		{"exactly enough width and height for side placement", boxW + bearGap + bearW, boxH + footerRows, bearSide},
		{"generously wide and tall favors side over top", boxW + bearGap + bearW + 20, boxH + 20, bearSide},
		{"not wide enough for side, but tall enough for top", boxW + 1, bearH + bearTopGap + boxH + footerRows, bearTop},
		{"not wide enough for side, not tall enough for top either", boxW + 1, bearH + bearTopGap + boxH + footerRows - 1, bearNone},
		{"narrower than the box itself never gets a bear", boxW - 5, bearH + bearTopGap + boxH + 20, bearNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decideBearPlacement(boxW, boxH, bearW, bearH, c.termWidth, c.termHeight); got != c.want {
				t.Errorf("decideBearPlacement(box=%dx%d, bear=%dx%d, term=%dx%d) = %v, want %v",
					boxW, boxH, bearW, bearH, c.termWidth, c.termHeight, got, c.want)
			}
		})
	}
}

func TestDecideTopOnlyBearPlacementNeverReturnsSide(t *testing.T) {
	const boxW, boxH = 86, 10
	const bearW, bearH = 18, 6

	cases := []struct {
		name                  string
		termWidth, termHeight int
		want                  bearPlacement
	}{
		{"box barely fits, no room for anything else", boxW, boxH, bearNone},
		{"generously wide and tall would favor side under decideBearPlacement, but this never returns it", boxW + bearGap + bearW + 20, boxH + 20, bearTop},
		{"not tall enough for top either", boxW + 1, bearH + bearTopGap + boxH + footerRows - 1, bearNone},
		{"narrower than the box itself never gets a bear", boxW - 5, bearH + bearTopGap + boxH + 20, bearNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decideTopOnlyBearPlacement(boxW, boxH, bearW, bearH, c.termWidth, c.termHeight); got != c.want {
				t.Errorf("decideTopOnlyBearPlacement(box=%dx%d, bear=%dx%d, term=%dx%d) = %v, want %v",
					boxW, boxH, bearW, bearH, c.termWidth, c.termHeight, got, c.want)
			}
			if got := decideTopOnlyBearPlacement(boxW, boxH, bearW, bearH, c.termWidth, c.termHeight); got == bearSide {
				t.Errorf("decideTopOnlyBearPlacement must never return bearSide, got it for term=%dx%d", c.termWidth, c.termHeight)
			}
		})
	}
}
