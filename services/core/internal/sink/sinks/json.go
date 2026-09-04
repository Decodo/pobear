package sinks

import (
	"context"
	"encoding/json"
)

type JSON struct {
	Path string

	items  []Item
	closed bool
}

func (j *JSON) Write(_ context.Context, item Item) error {
	return bufferedWrite(&j.closed, &j.items, item)
}

func (j *JSON) Close() error {
	return bufferedClose(&j.closed, j.Path, j.items, json.Marshal)
}
