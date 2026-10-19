package sinks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Decodo/pobear/services/core/internal/sink/sinks"
)

func TestYAMLBuffersUntilClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.yaml")
	y := &sinks.YAML{Path: path}

	if err := y.Write(context.Background(), sinks.Item{URL: "https://a.test"}); err != nil {
		t.Fatalf("Write 1: expected no error, got %v", err)
	}
	if err := y.Write(context.Background(), sinks.Item{URL: "https://b.test"}); err != nil {
		t.Fatalf("Write 2: expected no error, got %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no file on disk before Close — yaml buffers until completion, like json")
	}

	if err := y.Close(); err != nil {
		t.Fatalf("Close: expected no error, got %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // path is a t.TempDir() path this test constructed itself
	if err != nil {
		t.Fatalf("expected the file to exist after Close: %v", err)
	}

	var items []sinks.Item
	if err := yaml.Unmarshal(got, &items); err != nil {
		t.Fatalf("expected a single YAML document, got invalid YAML: %v\n%s", err, got)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].URL != "https://a.test" || items[1].URL != "https://b.test" {
		t.Errorf("items out of order or wrong: %+v", items)
	}
}

func TestYAMLWriteAfterCloseErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.yaml")
	y := &sinks.YAML{Path: path}

	_ = y.Close()

	if err := y.Write(context.Background(), sinks.Item{URL: "https://a.test"}); err == nil {
		t.Fatal("Write after Close: expected an error, got nil")
	}
}

func TestYAMLCloseWithNoWritesProducesEmptyList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.yaml")
	y := &sinks.YAML{Path: path}

	_ = y.Close()

	got, err := os.ReadFile(path) //nolint:gosec // path is a t.TempDir() path this test constructed itself
	if err != nil {
		t.Fatalf("expected the file to exist after Close with no Writes, got: %v", err)
	}

	var items []sinks.Item
	if err := yaml.Unmarshal(got, &items); err != nil {
		t.Fatalf("expected a valid YAML document, got invalid YAML: %v\n%s", err, got)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
	if strings.TrimSpace(string(got)) != "[]" {
		t.Fatalf("expected literal `[]`, got:\n%s", got)
	}
}

func TestYAMLUsesSameFieldNamesAsJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.yaml")
	y := &sinks.YAML{Path: path}

	item := sinks.Item{URL: "https://a.test", FinalURL: "https://a.test/", ParentURL: "https://a.test/parent"}
	if err := y.Write(context.Background(), item); err != nil {
		t.Fatalf("Write: expected no error, got %v", err)
	}
	if err := y.Close(); err != nil {
		t.Fatalf("Close: expected no error, got %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // path is a t.TempDir() path this test constructed itself
	if err != nil {
		t.Fatalf("expected the file to exist after Close: %v", err)
	}

	for _, key := range []string{"url:", "finalUrl:", "parentUrl:"} {
		if !strings.Contains(string(got), key) {
			t.Errorf("expected key %q in output (matching JSON's camelCase field names), got:\n%s", key, got)
		}
	}
}
