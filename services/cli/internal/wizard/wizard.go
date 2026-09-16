// Package wizard implements the `pobear wizard` full-screen reactive
// terminal UI. See docs/CLI_REFERENCE.md's wizard section.
package wizard

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

type Wizard struct{}

func (w *Wizard) Run(ctx context.Context, _ []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p := tea.NewProgram(newModel(),
		tea.WithContext(ctx),
		tea.WithInput(stdin),
		tea.WithOutput(stdout),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return 1
	}
	return 0
}
