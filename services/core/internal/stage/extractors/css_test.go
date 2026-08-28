package extractors_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/stage/extractors"
)

func TestCSSRun(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		fields  map[string]string
		want    []extractors.Result
		wantErr bool
	}{
		{
			name:   "text-selector",
			body:   "<h1>Hello</h1>",
			fields: map[string]string{"title": "h1"},
			want:   []extractors.Result{{Key: "title", Value: []string{"Hello"}}},
		},
		{
			name:   "attr-selector",
			body:   `<a href="https://x.test">text</a>`,
			fields: map[string]string{"link": "a@href"},
			want:   []extractors.Result{{Key: "link", Value: []string{"https://x.test"}}},
		},
		{
			name:   "multiple-fields",
			body:   `<h1>Hello</h1><a href="https://x.test">text</a>`,
			fields: map[string]string{"title": "h1", "link": "a@href"},
			want: []extractors.Result{
				{Key: "title", Value: []string{"Hello"}},
				{Key: "link", Value: []string{"https://x.test"}},
			},
		},
		{
			name:   "no-match-is-empty-list",
			body:   "<p>nothing here</p>",
			fields: map[string]string{"title": "h1"},
			want:   []extractors.Result{{Key: "title", Value: []string{}}},
		},
		{
			name:   "multiple-matches-returns-all-in-document-order",
			body:   "<li>one</li><li>two</li>",
			fields: map[string]string{"items": "li"},
			want:   []extractors.Result{{Key: "items", Value: []string{"one", "two"}}},
		},
		{
			name:   "nested-markup-is-text-only",
			body:   "<h1>Hello <em>world</em></h1>",
			fields: map[string]string{"title": "h1"},
			want:   []extractors.Result{{Key: "title", Value: []string{"Hello world"}}},
		},
		{
			name:   "attr-present-element-missing-attr",
			body:   "<a>text</a>",
			fields: map[string]string{"link": "a@href"},
			want:   []extractors.Result{{Key: "link", Value: []string{}}},
		},
		{
			name:    "invalid-selector-errors",
			body:    "<a>text</a>",
			fields:  map[string]string{"bad": "[["},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			css := extractors.CSS{}
			results, err := css.Run(context.Background(), []byte(c.body), jobspec.Stage{Use: "css", Fields: c.fields})
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if len(results) != len(c.want) {
				t.Fatalf("got %d results, want %d: %+v", len(results), len(c.want), results)
			}
			byKey := make(map[string]any, len(results))
			for _, r := range results {
				byKey[r.Key] = r.Value
			}
			for _, w := range c.want {
				got, ok := byKey[w.Key]
				if !ok {
					t.Errorf("missing result for key %q", w.Key)
					continue
				}
				if !reflect.DeepEqual(got, w.Value) {
					t.Errorf("key %q: value = %v, want %v", w.Key, got, w.Value)
				}
			}
		})
	}
}
