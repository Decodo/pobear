package sink_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Decodo/pobear/services/core/internal/sink"
	"github.com/Decodo/pobear/services/core/internal/sink/sinks"
)

type fakeSink struct {
	writeErr error
	closeErr error

	lastItem sinks.Item
	writes   int
	closed   bool
}

func (f *fakeSink) Write(_ context.Context, item sinks.Item) error {
	f.lastItem = item
	f.writes++
	return f.writeErr
}

func (f *fakeSink) Close() error {
	f.closed = true
	return f.closeErr
}

func TestSinkerWriteForwardsToSelectedSink(t *testing.T) {
	stdout := &fakeSink{}
	ndjson := &fakeSink{}

	s := sink.NewSinker(map[string]sink.Sink{"stdout": stdout, "ndjson": ndjson}, "stdout")

	first := sinks.Item{URL: "https://a.test"}
	if err := s.Write(context.Background(), first); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	second := sinks.Item{URL: "https://b.test"}
	if err := s.Write(context.Background(), second); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stdout.writes != 2 {
		t.Fatalf("expected 2 writes to the selected sink, got %d", stdout.writes)
	}
	if !reflect.DeepEqual(stdout.lastItem, second) {
		t.Fatalf("expected the most recent write's item, got %+v", stdout.lastItem)
	}
	if ndjson.writes != 0 {
		t.Fatalf("expected the non-selected sink never called, got %d writes", ndjson.writes)
	}
}

func TestSinkerCloseForwardsToSelectedSink(t *testing.T) {
	stdout := &fakeSink{}
	ndjson := &fakeSink{}

	s := sink.NewSinker(map[string]sink.Sink{"stdout": stdout, "ndjson": ndjson}, "ndjson")
	err := s.Close()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ndjson.closed {
		t.Fatal("expected the selected sink's Close to be called")
	}
	if stdout.closed {
		t.Fatal("expected the non-selected sink's Close never called")
	}
}

func TestSinkerWritePropagatesSinkError(t *testing.T) {
	wantErr := errors.New("disk full")
	stdout := &fakeSink{writeErr: wantErr}

	s := sink.NewSinker(map[string]sink.Sink{"stdout": stdout}, "stdout")
	err := s.Write(context.Background(), sinks.Item{})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSinkerClosePropagatesSinkError(t *testing.T) {
	wantErr := errors.New("flush failed")
	stdout := &fakeSink{closeErr: wantErr}

	s := sink.NewSinker(map[string]sink.Sink{"stdout": stdout}, "stdout")
	err := s.Close()

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSinkerUnknownSinkOnWrite(t *testing.T) {
	stdout := &fakeSink{}
	s := sink.NewSinker(map[string]sink.Sink{"stdout": stdout}, "missing")
	err := s.Write(context.Background(), sinks.Item{})

	if !errors.Is(err, sink.ErrUnknownSink) {
		t.Fatalf("expected ErrUnknownSink, got %v", err)
	}
	if stdout.writes != 0 {
		t.Fatalf("expected the registered-but-not-selected sink never called, got %d writes", stdout.writes)
	}
}

func TestSinkerUnknownSinkOnClose(t *testing.T) {
	s := sink.NewSinker(map[string]sink.Sink{}, "missing")
	err := s.Close()

	if !errors.Is(err, sink.ErrUnknownSink) {
		t.Fatalf("expected ErrUnknownSink, got %v", err)
	}
}
