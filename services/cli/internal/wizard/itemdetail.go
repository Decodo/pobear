package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	itemDetailTitle        = "Item"
	itemDetailBackHint     = "esc: back to results"
	itemDetailFooterHint   = "↑/↓ scroll   w write to file   t wrap   esc back"
	itemDetailWidthMargin  = 4
	itemDetailOverheadRows = 3
)

type itemDetailScreen struct {
	row           resultRow
	lines         []string
	wrap          bool
	offset        int
	status        string
	width, height int
}

func newItemDetailScreen(row resultRow) *itemDetailScreen {
	text := row.url
	if data, err := itemJSON(row); err == nil {
		text = string(data)
	}
	return &itemDetailScreen{row: row, lines: strings.Split(text, "\n")}
}

func (s *itemDetailScreen) Init() tea.Cmd { return nil }

func (s *itemDetailScreen) innerWidth() int {
	return s.width - itemDetailWidthMargin
}

func (s *itemDetailScreen) displayLines() []string {
	if !s.wrap {
		return s.lines
	}
	innerWidth := s.innerWidth()
	out := make([]string, 0, len(s.lines))
	for _, l := range s.lines {
		out = append(out, wrapLine(l, innerWidth)...)
	}
	return out
}

func (s *itemDetailScreen) visibleLines() int {
	return visibleCount(s.height, itemDetailOverheadRows, s.status != "", len(s.displayLines()))
}

func (s *itemDetailScreen) maxOffset() int {
	return maxScrollOffset(len(s.displayLines()), s.visibleLines())
}

func (s *itemDetailScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
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
		case "w":
			s.status = doWriteItem(s.row)
			return s, nil
		case "t":
			s.wrap = !s.wrap
			s.offset = min(s.offset, s.maxOffset())
			return s, nil
		}
	}
	return s, nil
}

func (s *itemDetailScreen) View() string {
	if s.width < unsupportedMinW || s.height < unsupportedMinH {
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, fallbackMessage())
	}

	innerWidth := s.innerWidth()
	displayLines := s.displayLines()
	visible := s.visibleLines()
	end := s.offset + visible
	if end > len(displayLines) {
		end = len(displayLines)
	}

	lines := make([]string, 0, visible+4)
	for _, l := range displayLines[s.offset:end] {
		if !s.wrap {
			l = truncatePlain(l, innerWidth)
		}
		lines = append(lines, l)
	}
	if len(displayLines) > visible {
		lines = append(lines, "", fmt.Sprintf("(line %d-%d of %d)", s.offset+1, end, len(displayLines)))
	}
	if s.status != "" {
		lines = append(lines, "", s.status)
	}

	body := strings.Join(lines, "\n")
	box := frameBox(innerWidth, itemDetailTitle, itemDetailBackHint, body, false)
	footer := footerStyle.Render(truncatePlain(itemDetailFooterHint, s.width))
	block := lipgloss.JoinVertical(lipgloss.Left, box, footer)
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, block)
}
