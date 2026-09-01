package command

import (
	"context"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
)

type Version struct{}

func (v *Version) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = ctx
	_ = args
	_ = stdin
	_ = stdout
	_ = stderr
	return exitcode.Fatal
}
