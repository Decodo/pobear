package extractors_test

import (
	"context"
	"testing"

	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/stage/extractors"
)

func TestMarkdownRun(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "heading", body: "<h1>Hello</h1>", want: "# Hello"},
		{name: "paragraph", body: "<p>Hello world</p>", want: "Hello world"},
		{name: "bold", body: "<strong>bold</strong>", want: "**bold**"},
		{name: "italic", body: "<em>italic</em>", want: "*italic*"},
		{name: "link", body: `<a href="https://x.test">link</a>`, want: "[link](https://x.test)"},
		{name: "multi-block-joined-with-blank-line", body: "<h1>A</h1><p>B</p>", want: "# A\n\nB"},
		{name: "unordered-list", body: "<ul><li>one</li><li>two</li></ul>", want: "- one\n- two"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := extractors.Markdown{}
			results, err := m.Run(context.Background(), []byte(c.body), jobspec.Stage{Use: "markdown"})
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected exactly 1 result, got %d", len(results))
			}
			if results[0].Key != "markdown" {
				t.Errorf("key = %q, want \"markdown\"", results[0].Key)
			}
			if results[0].Value != c.want {
				t.Errorf("value = %v, want %q", results[0].Value, c.want)
			}
		})
	}
}
