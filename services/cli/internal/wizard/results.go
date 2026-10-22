package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Decodo/pobear/pkg/fault"
)

const (
	resultsTitle        = "Results"
	resultsBackHint     = "esc: back to form"
	resultsFooterHint   = "↑/↓ move   enter open   w write all   esc back"
	resultsStatusWidth  = 8
	resultsURLWidth     = 36
	resultsRowOverhead  = 6
	resultsOverheadRows = 5
	resultsTipRows      = 1
	resultsMinVisible   = 3
	resultsWidthMargin  = 4
	resultsMinWidth     = resultsStatusWidth + resultsURLWidth + 3 + resultsWidthMargin
	resultsMinHeight    = resultsOverheadRows + resultsTipRows + resultsMinVisible + scrollIndicatorRows
)

type resultsScreen struct {
	rows          []resultRow
	summary       string
	cursor        int
	offset        int
	status        string
	width, height int
}

func newResultsScreen(rows []resultRow, summary string) *resultsScreen {
	return &resultsScreen{rows: rows, summary: summary}
}

func (s *resultsScreen) Init() tea.Cmd { return nil }

func (s *resultsScreen) innerWidth() int {
	w := s.width - resultsWidthMargin
	if w > formInnerWidth {
		w = formInnerWidth
	}
	return w
}

func (s *resultsScreen) overhead() int {
	overhead := resultsOverheadRows
	if hasEscalatableFailure(s.rows) {
		overhead += resultsTipRows
	}
	return overhead
}

func (s *resultsScreen) visibleRows() int {
	return visibleCount(s.height, s.overhead(), s.status != "", len(s.rows))
}

func (s *resultsScreen) maxOffset() int {
	return maxScrollOffset(len(s.rows), s.visibleRows())
}

func (s *resultsScreen) clampScroll() {
	if len(s.rows) == 0 {
		s.cursor, s.offset = 0, 0
		return
	}
	s.cursor = max(0, min(s.cursor, len(s.rows)-1))

	visible := s.visibleRows()
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+visible {
		s.offset = s.cursor - visible + 1
	}
	s.offset = max(0, min(s.offset, s.maxOffset()))
}

func (s *resultsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.clampScroll()
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return s, popScreen()
		case "up":
			if s.cursor > 0 {
				s.cursor--
			}
			s.clampScroll()
			return s, nil
		case "down":
			if s.cursor < len(s.rows)-1 {
				s.cursor++
			}
			s.clampScroll()
			return s, nil
		case "enter":
			if s.cursor < len(s.rows) {
				return s, pushScreen(newItemDetailScreen(s.rows[s.cursor]))
			}
			return s, nil
		case "w":
			if len(s.rows) > 0 {
				s.status = doWriteAllItems(s.rows)
			}
			return s, nil
		}
	}
	return s, nil
}

func hasEscalatableFailure(rows []resultRow) bool {
	for _, row := range rows {
		if row.kind == fault.Blocked || row.kind == fault.RateLimit || row.kind == fault.Timeout {
			return true
		}
	}
	return false
}

func formatResultRow(row resultRow, selected bool, innerWidth int) string {
	marker := "  "
	if selected {
		marker = "› "
	}
	status := padRightPlain(row.status, resultsStatusWidth)
	url := padRightPlain(truncatePlain(row.url, resultsURLWidth), resultsURLWidth)
	line := marker + status + " " + url
	if row.detail != "" {
		avail := innerWidth - resultsStatusWidth - resultsURLWidth - resultsRowOverhead
		line += " " + truncatePlain(row.detail, avail)
	}
	if selected {
		return selectedStyle.Render(line)
	}
	return line
}

func (s *resultsScreen) View() string {
	if s.width < resultsMinWidth || s.height < resultsMinHeight {
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, fallbackMessage())
	}

	innerWidth := s.innerWidth()
	visible := s.visibleRows()
	end := s.offset + visible
	if end > len(s.rows) {
		end = len(s.rows)
	}

	lines := []string{s.summary}
	if hasEscalatableFailure(s.rows) {
		lines = append(lines, truncatePlain("Tip: blocked or rate-limited? configure a Proxy or Provider tier to auto-escalate past it", innerWidth))
	}
	lines = append(lines, "")
	for i := s.offset; i < end; i++ {
		lines = append(lines, formatResultRow(s.rows[i], i == s.cursor, innerWidth))
	}
	if len(s.rows) > visible {
		lines = append(lines, "", fmt.Sprintf("(%d-%d of %d)", s.offset+1, end, len(s.rows)))
	}
	if s.status != "" {
		lines = append(lines, "", s.status)
	}

	body := strings.Join(lines, "\n")
	box := frameBox(innerWidth, resultsTitle, resultsBackHint, body, false)
	return placeWithBear(box, resultsFooterHint, s.width, s.height, false)
}
