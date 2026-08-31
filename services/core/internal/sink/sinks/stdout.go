package sinks

import (
	"context"
	"io"
)

type Stdout struct {
	Out io.Writer
}

func (s *Stdout) Write(ctx context.Context, item Item) error {
	return ErrNotImplemented
}

func (s *Stdout) Close() error {
	return ErrNotImplemented
}
