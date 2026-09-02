package command

import (
	"context"
	"io"
)

type Command interface {
	Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int
}
