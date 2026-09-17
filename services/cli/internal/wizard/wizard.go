// Package wizard implements the `pobear wizard` full-screen reactive
// terminal UI. See docs/CLI_REFERENCE.md's wizard section.
package wizard

import (
	"context"
	"fmt"
	"io"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type Wizard struct {
	Providers       *providerator.Providerator
	CredentialsPath string
}

func (w *Wizard) Run(ctx context.Context, _ []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p := tea.NewProgram(newApp(newModel(w.providerNames(), ctx, w.providersDir(), w.CredentialsPath)),
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

func (*Wizard) Summary() string { return "Launch the interactive full-screen wizard" }

func (w *Wizard) providersDir() string {
	if w.Providers == nil {
		return ""
	}
	return w.Providers.Dir
}

func (w *Wizard) providerNames() []string {
	if w.Providers == nil {
		return nil
	}
	manifests, err := w.Providers.Load()
	if err != nil {
		return nil
	}
	names := make([]string, len(manifests))
	for i, m := range manifests {
		names[i] = m.Name
	}
	slices.Sort(names)
	return names
}
