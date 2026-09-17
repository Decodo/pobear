package wizard

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	blinkInterval = 10 * time.Second
	blinkDuration = 500 * time.Millisecond
)

type blinkTickMsg struct{}

func scheduleBlinkOn() tea.Cmd {
	return tea.Tick(blinkInterval, func(time.Time) tea.Msg { return blinkTickMsg{} })
}

func scheduleBlinkOff() tea.Cmd {
	return tea.Tick(blinkDuration, func(time.Time) tea.Msg { return blinkTickMsg{} })
}

type model struct {
	width, height int
	cursor        int
	blinking      bool
	providerNames []string
}

func newModel(providerNames []string) model {
	return model{providerNames: providerNames}
}

func (m model) Init() tea.Cmd {
	return scheduleBlinkOn()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case blinkTickMsg:
		m.blinking = !m.blinking
		if m.blinking {
			return m, scheduleBlinkOff()
		}
		return m, scheduleBlinkOn()

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up", "k":
			m.cursor = (m.cursor - 1 + len(menuItems)) % len(menuItems)
		case "down", "j":
			m.cursor = (m.cursor + 1) % len(menuItems)
		case "enter":
			if menuItems[m.cursor] == "Scrape a page" {
				return m, pushScreen(newScrapeForm(m.providerNames))
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	return renderMenu(m.width, m.height, m.cursor, m.blinking)
}
