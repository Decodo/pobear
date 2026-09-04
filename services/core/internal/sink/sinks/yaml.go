package sinks

import (
	"context"

	"gopkg.in/yaml.v3"
)

type YAML struct {
	Path string

	items  []Item
	closed bool
}

func (y *YAML) Write(_ context.Context, item Item) error {
	return bufferedWrite(&y.closed, &y.items, item)
}

func (y *YAML) Close() error {
	return bufferedClose(&y.closed, y.Path, y.items, yaml.Marshal)
}
