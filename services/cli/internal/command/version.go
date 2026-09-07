package command

import (
	"context"
	"fmt"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
)

type Version struct{}

func (v *Version) Run(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) int {
	fmt.Fprintln(stdout, "pobear 0.0.1")
	return exitcode.Success
}
