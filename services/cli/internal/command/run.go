package command

import (
	"context"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/jobspec"
)

type JobSpecParser interface {
	Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error)
}

type Run struct {
	Parser JobSpecParser
	Runner JobRunner
}

func (c *Run) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = ctx
	_ = args
	_ = stdin
	_ = stdout
	_ = stderr
	_ = c.Parser
	_ = c.Runner
	return exitcode.Fatal
}
