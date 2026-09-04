package sinks

import (
	"context"
	"os"
)

type NDJSON struct {
	Path string

	file   *os.File
	closed bool
}

func (n *NDJSON) Write(_ context.Context, item Item) error {
	if n.closed {
		return errClosed
	}
	if n.file == nil {
		f, err := os.Create(n.Path) //nolint:gosec // Path is the caller-supplied sink target, not user-controlled input
		if err != nil {
			return err
		}
		n.file = f
	}
	return writeJSONLine(n.file, item)
}

func (n *NDJSON) Close() error {
	if n.closed {
		return nil
	}
	n.closed = true
	if n.file == nil {
		f, err := os.Create(n.Path) //nolint:gosec // Path is the caller-supplied sink target, not user-controlled input
		if err != nil {
			return err
		}
		return f.Close()
	}
	return n.file.Close()
}
