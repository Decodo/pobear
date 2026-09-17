package wizard

import tea "github.com/charmbracelet/bubbletea"

type pushScreenMsg struct{ screen tea.Model }

type popScreenMsg struct{}

type replaceTopMsg struct{ screen tea.Model }

func pushScreen(screen tea.Model) tea.Cmd {
	return func() tea.Msg { return pushScreenMsg{screen: screen} }
}

func popScreen() tea.Cmd {
	return func() tea.Msg { return popScreenMsg{} }
}

func replaceTop(screen tea.Model) tea.Cmd {
	return func() tea.Msg { return replaceTopMsg{screen: screen} }
}

type app struct {
	stack         []tea.Model
	width, height int
}

func newApp(root tea.Model) app {
	return app{stack: []tea.Model{root}}
}

func (a app) top() tea.Model {
	return a.stack[len(a.stack)-1]
}

func (a app) Init() tea.Cmd {
	return a.top().Init()
}

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		return a, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		cmds := make([]tea.Cmd, len(a.stack))
		for i, screen := range a.stack {
			updated, cmd := screen.Update(msg)
			a.stack[i] = updated
			cmds[i] = cmd
		}
		return a, tea.Batch(cmds...)

	case pushScreenMsg:
		initCmd := msg.screen.Init()
		updated, sizeCmd := msg.screen.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		a.stack = append(a.stack, updated)
		return a, tea.Batch(initCmd, sizeCmd)

	case popScreenMsg:
		if len(a.stack) > 1 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil

	case replaceTopMsg:
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		initCmd := msg.screen.Init()
		updated, sizeCmd := msg.screen.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		a.stack = append(a.stack, updated)
		return a, tea.Batch(initCmd, sizeCmd)
	}

	updated, cmd := a.top().Update(msg)
	a.stack[len(a.stack)-1] = updated
	return a, cmd
}

func (a app) View() string {
	return a.top().View()
}
