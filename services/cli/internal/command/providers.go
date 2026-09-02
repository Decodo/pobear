package command

import (
	"context"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type Providers struct {
	Providers *providerator.Providerator
}

func (p *Providers) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = ctx
	_ = args
	_ = stdin
	_ = stdout
	_ = stderr
	_ = p.Providers
	return exitcode.Fatal
}
