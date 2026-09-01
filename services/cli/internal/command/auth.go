package command

import (
	"context"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type Auth struct {
	Providers *providerator.Providerator
	Prompt    func(label string) (string, error)
	Path      string
}

func (a *Auth) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = ctx
	_ = args
	_ = stdin
	_ = stdout
	_ = stderr
	_ = a.Providers
	_ = a.Prompt
	_ = a.Path
	return exitcode.Fatal
}
