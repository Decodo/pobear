package wizard

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

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
