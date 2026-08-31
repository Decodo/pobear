package sinks

import "context"

type JSON struct {
	Path string
}

func (j *JSON) Write(ctx context.Context, item Item) error {
	return ErrNotImplemented
}

func (j *JSON) Close() error {
	return ErrNotImplemented
}
