package sinks_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pobear/pobear/services/core/internal/sink/sinks"
)

func TestJSONBuffersUntilClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	j := &sinks.JSON{Path: path}

	if err := j.Write(context.Background(), sinks.Item{URL: "https://a.test"}); err != nil {
		t.Fatalf("Write 1: expected no error, got %v", err)
	}
	if err := j.Write(context.Background(), sinks.Item{URL: "https://b.test"}); err != nil {
		t.Fatalf("Write 2: expected no error, got %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no file on disk before Close — json buffers until completion, unlike ndjson")
	}

	if err := j.Close(); err != nil {
		t.Fatalf("Close: expected no error, got %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // path is a t.TempDir() path this test constructed itself
	if err != nil {
		t.Fatalf("expected the file to exist after Close: %v", err)
	}

	var items []sinks.Item
	if err := json.Unmarshal(got, &items); err != nil {
		t.Fatalf("expected a single JSON array, got invalid JSON: %v\n%s", err, got)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].URL != "https://a.test" || items[1].URL != "https://b.test" {
		t.Errorf("items out of order or wrong: %+v", items)
	}
}

func TestJSONWriteAfterCloseErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	j := &sinks.JSON{Path: path}

	_ = j.Close()

	if err := j.Write(context.Background(), sinks.Item{URL: "https://a.test"}); err == nil {
		t.Fatal("Write after Close: expected an error, got nil")
	}
}

func TestJSONCloseWithNoWritesProducesEmptyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	j := &sinks.JSON{Path: path}

	_ = j.Close()

	got, err := os.ReadFile(path) //nolint:gosec // path is a t.TempDir() path this test constructed itself
	if err != nil {
		t.Fatalf("expected the file to exist after Close with no Writes, got: %v", err)
	}

	var items []sinks.Item
	if err := json.Unmarshal(got, &items); err != nil {
		t.Fatalf("expected a valid JSON array, got invalid JSON: %v\n%s", err, got)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
	if strings.TrimSpace(string(got)) != "[]" {
		t.Fatalf("expected literal `[]`, got:\n%s", got)
	}
}
