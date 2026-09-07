package command

import (
	"context"
	"fmt"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type Auth struct {
	Providers *providerator.Providerator
	Prompt    func(label string) (string, error)
	Path      string
}

func (a *Auth) Run(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "pobear: auth requires a provider name")
		return exitcode.UsageError
	}
	name := args[0]

	manifests, err := a.Providers.Load()
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.UsageError
	}
	m, ok := providerator.Lookup(manifests, name)
	if !ok {
		fmt.Fprintf(stderr, "pobear: unknown provider %q\n", name)
		return exitcode.UsageError
	}

	if err := providerator.Auth(m, a.Path, a.Prompt, stdout); err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.UsageError
	}
	return exitcode.Success
}
