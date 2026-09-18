package wizard

import (
	"io"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func truncatePlain(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func padRightPlain(s string, width int) string {
	n := lipgloss.Width(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func wrapLine(l string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	wrapped := ansi.Hardwrap(ansi.Wordwrap(l, width, ""), width, true)
	return strings.Split(wrapped, "\n")
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	const maxLen = 80
	out := b.String()
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	if out == "" {
		out = "item"
	}
	return out
}

type countingWriter struct {
	out     io.Writer
	counter *atomic.Int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.out.Write(p)
	w.counter.Add(1)
	return n, err
}

const (
	visibleCountFloor   = 3
	scrollIndicatorRows = 2
)

func visibleCountRaw(height, overhead int, hasStatus bool, total int) int {
	if hasStatus {
		overhead += 2
	}
	v := max(height-overhead, visibleCountFloor)
	if v > total {
		v = total
	}
	return v
}

func visibleCount(height, overhead int, hasStatus bool, total int) int {
	visible := visibleCountRaw(height, overhead, hasStatus, total)
	if total <= visible {
		return visible
	}
	return visibleCountRaw(height, overhead+scrollIndicatorRows, hasStatus, total)
}

func maxScrollOffset(total, visible int) int {
	return max(0, total-visible)
}
