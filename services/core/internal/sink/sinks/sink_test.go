package sinks_test

import (
	"encoding/json"
	"testing"

	"github.com/pobear/pobear/services/core/internal/sink/sinks"
)

func TestItemJSON(t *testing.T) {
	tests := []struct {
		name       string
		item       sinks.Item
		wantKeys   map[string]bool
		wantValues map[string]any
	}{
		{
			name: "fully populated",
			item: sinks.Item{
				URL:       "https://example.com",
				FinalURL:  "https://example.com/",
				ParentURL: "https://example.com/parent",
				Status:    200,
				Headers:   map[string][]string{"Content-Type": {"text/html"}},
				Body:      "<!doctype html>hello",
				Depth:     1,
				Data:      map[string]any{"title": "Example"},
			},
			wantKeys: map[string]bool{
				"url": true, "finalUrl": true, "parentUrl": true, "status": true,
				"headers": true, "body": true, "depth": true, "data": true,
			},
			wantValues: map[string]any{
				"url":       "https://example.com",
				"finalUrl":  "https://example.com/",
				"parentUrl": "https://example.com/parent",
				"status":    float64(200),
				"body":      "<!doctype html>hello",
				"depth":     float64(1),
			},
		},
		{
			name: "minimal (zero values)",
			item: sinks.Item{
				URL:    "https://example.com",
				Status: 200,
			},
			wantKeys: map[string]bool{
				"url": true, "finalUrl": true, "parentUrl": true, "status": true,
				"body": true, "depth": true,
				"headers": false, "data": false,
			},
			wantValues: map[string]any{
				"url":       "https://example.com",
				"parentUrl": "",
				"body":      "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.item)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("Unmarshal into map: %v", err)
			}

			for key, wantPresent := range tc.wantKeys {
				_, present := m[key]
				if present != wantPresent {
					t.Errorf("key %q: present = %v, want %v (json: %s)", key, present, wantPresent, raw)
				}
			}

			for key, want := range tc.wantValues {
				got, present := m[key]
				if !present {
					t.Errorf("key %q: expected to be present with value %v, but missing", key, want)
					continue
				}
				if got != want {
					t.Errorf("key %q = %v (%T), want %v (%T)", key, got, got, want, want)
				}
			}
		})
	}
}

func TestItemBodyIsLiteralStringNotBase64(t *testing.T) {
	item := sinks.Item{Body: "<!doctype html>plain text body"}

	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("Unmarshal into map: %v", err)
	}

	got, ok := m["body"].(string)
	if !ok {
		t.Fatalf("body is not a string in decoded JSON: %v", m["body"])
	}
	if got != item.Body {
		t.Errorf("body = %q, want literal %q (not base64-encoded)", got, item.Body)
	}
}
