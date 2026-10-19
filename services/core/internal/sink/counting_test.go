package sink_test

import (
	"context"
	"testing"

	"github.com/pobear/pobear/services/core/internal/sink"
	"github.com/pobear/pobear/services/core/internal/sink/sinks"
)

type recordingWriter struct {
	written int
	closed  bool
}

func (r *recordingWriter) Write(_ context.Context, _ sinks.Item) error {
	r.written++
	return nil
}

func (r *recordingWriter) Close() error {
	r.closed = true
	return nil
}

func TestEmptyCounterCountsItemsWithNoExtractedValue(t *testing.T) {
	cases := []struct {
		name string
		data map[string]any
		want int
	}{
		{name: "css matched nothing", data: map[string]any{"price": []string{}}, want: 1},
		{name: "css matched something", data: map[string]any{"price": []string{"9.99"}}, want: 0},
		{name: "empty markdown string", data: map[string]any{"markdown": ""}, want: 1},
		{name: "non-empty markdown string", data: map[string]any{"markdown": "# Title"}, want: 0},
		{name: "one of two fields matched", data: map[string]any{"a": []string{}, "b": []string{"x"}}, want: 0},
		{name: "no data at all", data: nil, want: 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inner := &recordingWriter{}
			counter := sink.NewEmptyCounter(inner, true)

			if err := counter.Write(context.Background(), sinks.Item{Data: c.data}); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got := counter.Empty(); got != c.want {
				t.Errorf("Empty() = %d, want %d", got, c.want)
			}
			if inner.written != 1 {
				t.Errorf("inner writes = %d, want 1 — counting must not swallow the item", inner.written)
			}
		})
	}
}

func TestEmptyCounterIgnoresItemsWhenJobHasNoStages(t *testing.T) {
	inner := &recordingWriter{}
	counter := sink.NewEmptyCounter(inner, false)

	if err := counter.Write(context.Background(), sinks.Item{Data: nil}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := counter.Empty(); got != 0 {
		t.Errorf("Empty() = %d, want 0 — with no stages there is nothing to extract", got)
	}
}

func TestEmptyCounterClosesTheWrappedSink(t *testing.T) {
	inner := &recordingWriter{}
	counter := sink.NewEmptyCounter(inner, true)

	if err := counter.Close(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !inner.closed {
		t.Error("Close did not reach the wrapped sink")
	}
}
