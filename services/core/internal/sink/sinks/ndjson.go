package sinks

import "context"

type NDJSON struct {
	Path string
}

func (n *NDJSON) Write(ctx context.Context, item Item) error {
	return ErrNotImplemented
}

func (n *NDJSON) Close() error {
	return ErrNotImplemented
}
