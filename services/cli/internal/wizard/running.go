package wizard

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	runningTitle      = "Running"
	runningFooterHint = "please wait…"
	runningTickEvery  = 100 * time.Millisecond
	progressBarWidth  = 40
)

type runTickMsg struct{}

type runFinishedMsg struct {
	status string
	rows   []resultRow
}

func runTick() tea.Cmd {
	return tea.Tick(runningTickEvery, func(time.Time) tea.Msg { return runTickMsg{} })
}

type statusSetter interface {
	setStatus(string)
}

type runningScreen struct {
	form          statusSetter
	progress      *runProgress
	width, height int
}

func newRunningScreen(form statusSetter, progress *runProgress) *runningScreen {
	return &runningScreen{form: form, progress: progress}
}

func (s *runningScreen) Init() tea.Cmd {
	return runTick()
}

func (s *runningScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		return s, nil

	case runTickMsg:
		return s, runTick()

	case runFinishedMsg:
		s.form.setStatus(msg.status)
		if len(msg.rows) > 0 {
			return s, replaceTop(newResultsScreen(msg.rows, msg.status))
		}
		return s, popScreen()
	}
	return s, nil
}

func renderProgressBar(percent float64) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(percent*progressBarWidth + 0.5)
	if filled > progressBarWidth {
		filled = progressBarWidth
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", progressBarWidth-filled) + "]"
}

func (s *runningScreen) View() string {
	percent := s.progress.percent()
	completed, total := s.progress.completed.Load(), s.progress.total

	body := fmt.Sprintf("%s  %3d%%  (%d/%d)", renderProgressBar(percent), int(percent*100), completed, total)
	box := frameBox(formInnerWidth, runningTitle, "", body, true)
	return placeWithBear(box, runningFooterHint, s.width, s.height, true)
}
