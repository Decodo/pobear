package extractors_test

import (
	"context"
	"testing"

	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/stage/extractors"
)

func TestRawRun(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "plain-text", body: "hello world"},
		{name: "html", body: "<h1>Hello</h1><p>world</p>"},
		{name: "empty-body", body: ""},
		{name: "whitespace-and-entities-preserved", body: "  <p>a &amp; b</p>\n\n  "},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := extractors.Raw{}
			results, err := r.Run(context.Background(), []byte(c.body), jobspec.Stage{Use: "raw"})
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected exactly 1 result, got %d", len(results))
			}
			if results[0].Key != "raw" {
				t.Errorf("key = %q, want \"raw\"", results[0].Key)
			}
			if results[0].Value != c.body {
				t.Errorf("value = %v, want %q", results[0].Value, c.body)
			}
		})
	}
}
