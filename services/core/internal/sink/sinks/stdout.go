package sinks

import (
	"context"
	"io"
)

type Stdout struct {
	Out    io.Writer
	closed bool
}

func (s *Stdout) Write(_ context.Context, item Item) error {
	if s.closed {
		return errClosed
	}
	return writeJSONLine(s.Out, item)
}

func (s *Stdout) Close() error {
	s.closed = true
	return nil
}
