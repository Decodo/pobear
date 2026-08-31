package sinks_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pobear/pobear/services/core/internal/sink/sinks"
)

func TestStdoutWriteStreamsOneLinePerItem(t *testing.T) {
	var buf bytes.Buffer
	s := &sinks.Stdout{Out: &buf}

	items := []sinks.Item{
		{URL: "https://a.test", Status: 200, Body: "a"},
		{URL: "https://b.test", Status: 200, Body: "b"},
	}

	for _, item := range items {
		if err := s.Write(context.Background(), item); err != nil {
			t.Fatalf("Write(%q): expected no error, got %v", item.URL, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: expected no error, got %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), buf.String())
	}

	scanner := bufio.NewScanner(strings.NewReader(buf.String()))
	i := 0
	for scanner.Scan() {
		var got sinks.Item
		if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
		if got.URL != items[i].URL {
			t.Errorf("line %d: url = %q, want %q", i, got.URL, items[i].URL)
		}
		if got.Status != items[i].Status {
			t.Errorf("line %d: status = %d, want %d", i, got.Status, items[i].Status)
		}
		if got.Body != items[i].Body {
			t.Errorf("line %d: body = %q, want %q", i, got.Body, items[i].Body)
		}
		i++
	}
}

func TestStdoutWriteAfterCloseErrors(t *testing.T) {
	var buf bytes.Buffer
	s := &sinks.Stdout{Out: &buf}

	_ = s.Close()

	if err := s.Write(context.Background(), sinks.Item{URL: "https://a.test"}); err == nil {
		t.Fatal("Write after Close: expected an error, got nil")
	}
}
