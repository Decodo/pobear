package command

import (
	"context"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type Crawl struct {
	Runner    JobRunner
	Providers *providerator.Providerator
}

func (c *Crawl) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = ctx
	_ = args
	_ = stdin
	_ = stdout
	_ = stderr
	_ = c.Runner
	_ = c.Providers
	return exitcode.Fatal
}
