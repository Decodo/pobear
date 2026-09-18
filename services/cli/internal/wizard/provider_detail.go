package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pobear/pobear/pkg/provider"
)

const (
	providerDetailTitle        = "Provider"
	providerDetailBackHint     = "esc: back to providers"
	providerDetailFooterHint   = "↑/↓ scroll   esc back"
	providerDetailWidthMargin  = 4
	providerDetailOverheadRows = 4
)

func formatProviderOptionLine(o provider.Option) string {
	desc := o.Type
	if o.Type == "enum" {
		desc = "enum: " + strings.Join(o.Values, ", ")
	}
	line := o.Name + " (" + desc + ")"
	switch {
	case o.Default != nil:
		line += fmt.Sprintf(" default: %v", o.Default)
	case o.Pattern != "":
		line += " pattern: " + o.Pattern
	case o.Min != nil || o.Max != nil:
		line += " " + formatProviderMinMax(o.Min, o.Max)
	}
	return line
}

func formatProviderMinMax(minV, maxV *int) string {
	switch {
	case minV != nil && maxV != nil:
		return fmt.Sprintf("min: %d, max: %d", *minV, *maxV)
	case minV != nil:
		return fmt.Sprintf("min: %d", *minV)
	default:
		return fmt.Sprintf("max: %d", *maxV)
	}
}

func formatProviderDetail(m *provider.Manifest) []string {
	lines := []string{m.Name + " — " + m.Endpoint}
	if len(m.Templates) == 0 {
		lines = append(lines, "")
		for _, o := range m.Options {
			lines = append(lines, "  "+formatProviderOptionLine(o))
		}
		return lines
	}
	for _, t := range m.Templates {
		lines = append(lines, "")
		name := t.Name
		if t.Default {
			name += " (default)"
		}
		lines = append(lines, name+":")
		for _, o := range t.Options {
			lines = append(lines, "  "+formatProviderOptionLine(o))
		}
	}
	return lines
}

type providerDetailScreen struct {
	manifest      *provider.Manifest
	lines         []string
	offset        int
	width, height int
}

func newProviderDetailScreen(m *provider.Manifest) *providerDetailScreen {
	return &providerDetailScreen{manifest: m, lines: formatProviderDetail(m)}
}

func (s *providerDetailScreen) Init() tea.Cmd { return nil }

func (s *providerDetailScreen) innerWidth() int {
	return s.width - providerDetailWidthMargin
}

func (s *providerDetailScreen) visibleLines() int {
	return visibleCount(s.height, providerDetailOverheadRows, false, len(s.lines))
}

func (s *providerDetailScreen) maxOffset() int {
	return maxScrollOffset(len(s.lines), s.visibleLines())
}

func (s *providerDetailScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		s.offset = min(s.offset, s.maxOffset())
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return s, popScreen()
		case "up":
			if s.offset > 0 {
				s.offset--
			}
			return s, nil
		case "down":
			if s.offset < s.maxOffset() {
				s.offset++
			}
			return s, nil
		}
	}
	return s, nil
}

func (s *providerDetailScreen) View() string {
	if s.width < unsupportedMinW || s.height < unsupportedMinH {
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, fallbackMessage())
	}

	innerWidth := s.innerWidth()
	visible := s.visibleLines()
	end := s.offset + visible
	if end > len(s.lines) {
		end = len(s.lines)
	}

	lines := make([]string, 0, visible+2)
	for _, l := range s.lines[s.offset:end] {
		lines = append(lines, truncatePlain(l, innerWidth))
	}
	if len(s.lines) > visible {
		lines = append(lines, "", fmt.Sprintf("(line %d-%d of %d)", s.offset+1, end, len(s.lines)))
	}

	body := strings.Join(lines, "\n")
	box := frameBox(innerWidth, providerDetailTitle, providerDetailBackHint, body, false)
	footer := footerStyle.Render(truncatePlain(providerDetailFooterHint, s.width))
	block := lipgloss.JoinVertical(lipgloss.Left, box, footer)
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, block)
}
