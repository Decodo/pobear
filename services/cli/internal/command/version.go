package command

import (
	"context"
	"fmt"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
)

type Version struct {
	Version string
}

func (v *Version) Run(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) int {
	ver := v.Version
	if ver == "" {
		ver = "dev"
	}
	fmt.Fprintf(stdout, "pobear %s\n", ver)
	return exitcode.Success
}

func (*Version) Summary() string { return "Print the version pobear was built from" }
