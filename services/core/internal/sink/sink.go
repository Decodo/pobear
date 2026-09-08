package sink

import (
	"context"
	"errors"

	"github.com/pobear/pobear/services/core/internal/sink/sinks"
)

type Sink interface {
	Write(ctx context.Context, item sinks.Item) error
	Close() error
}

var ErrUnknownSink = errors.New("sink: unknown sink")

type Sinker struct {
	sinks map[string]Sink
	use   string
}

func NewSinker(sinks map[string]Sink, use string) *Sinker {
	return &Sinker{sinks: sinks, use: use}
}

func (s *Sinker) Write(ctx context.Context, item sinks.Item) error {
	sk, ok := s.sinks[s.use]
	if !ok {
		return ErrUnknownSink
	}
	return sk.Write(ctx, item)
}

func (s *Sinker) Close() error {
	sk, ok := s.sinks[s.use]
	if !ok {
		return ErrUnknownSink
	}
	return sk.Close()
}
