package main

import (
	"slices"
	"testing"

	"github.com/Decodo/pobear/pkg/provider"
)

const testIR = `{
  "version": "9.9.9",
  "apis": {"webScrapingApi": {"targets": {
    "universal": {"parameter_schema": {"properties": {
      "target": {"const": "universal"},
      "url": {"type": "string"},
      "headless": {"type": "string", "enum": ["html", "png"]},
      "proxy_pool": {"type": "string", "enum": ["standard", "premium"]},
      "markdown": {"type": "boolean"},
      "cookies": {"type": "object"},
      "successful_status_codes": {"type": "array", "items": {"type": "number"}}
    }}},
    "google": {"parameter_schema": {"properties": {
      "target": {"const": "google"},
      "url": {"type": "string"},
      "page_count": {"type": "number", "minimum": 1, "maximum": 10},
      "parse": {"type": "boolean"}
    }}},
    "reddit_post": {"parameter_schema": {"properties": {
      "target": {"const": "reddit_post"},
      "url": {"type": "string"}
    }}},
    "google_search": {"parameter_schema": {"properties": {
      "target": {"const": "google_search"},
      "query": {"type": "string"}
    }}}
  }}}
}`

func TestGenerateProducesValidManifest(t *testing.T) {
	out, err := generate([]byte(testIR))
	if err != nil {
		t.Fatal(err)
	}
	m, err := provider.Parse(out)
	if err != nil {
		t.Fatalf("generated manifest invalid: %v\n%s", err, out)
	}

	if m.Endpoint != endpoint || m.Auth.Type != "bearer" {
		t.Errorf("endpoint/auth = %s/%s", m.Endpoint, m.Auth.Type)
	}

	var names []string
	for _, tmpl := range m.Templates {
		names = append(names, tmpl.Name)
	}
	if want := []string{"universal", "google"}; !slices.Equal(names, want) {
		t.Fatalf("templates = %v, want %v (skipped and query-only targets dropped, default first)", names, want)
	}
	if !m.Templates[0].Default {
		t.Error("universal is not the default template")
	}

	universal := optionNames(m.Templates[0].Options)
	if want := []string{"render", "headless", "proxy_pool"}; !slices.Equal(universal, want) {
		t.Errorf("universal options = %v, want %v", universal, want)
	}
	if d := m.Templates[0].Options[1].Default; d != nil {
		t.Errorf("headless default = %v, want none", d)
	}
	if d := m.Templates[0].Options[2].Default; d != "premium" {
		t.Errorf("proxy_pool default = %v, want premium", d)
	}

	pageCount := m.Templates[1].Options[0]
	if pageCount.Name != "page_count" || pageCount.Type != "int" || *pageCount.Min != 1 || *pageCount.Max != 10 {
		t.Errorf("page_count option = %+v", pageCount)
	}
}

func TestGenerateMapsRenderToHeadless(t *testing.T) {
	out, err := generate([]byte(testIR))
	if err != nil {
		t.Fatal(err)
	}
	m, err := provider.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	req := m.Templates[0].Request

	cases := []struct {
		name    string
		options map[string]any
		want    any
	}{
		{"neither set", map[string]any{}, nil},
		{"render", map[string]any{"render": true}, "html"},
		{"render false", map[string]any{"render": false}, nil},
		{"explicit headless wins", map[string]any{"render": true, "headless": "png"}, "png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, _, err := provider.Render(req, "https://a.test", c.options)
			if err != nil {
				t.Fatal(err)
			}
			if got := body["headless"]; got != c.want {
				t.Errorf("headless = %v, want %v", got, c.want)
			}
			if _, ok := body["render"]; ok {
				t.Error("render must not be sent to the API")
			}
		})
	}
}

func TestGenerateRequiresUniversal(t *testing.T) {
	if _, err := generate([]byte(`{"version":"1.0.0","apis":{"webScrapingApi":{"targets":{}}}}`)); err == nil {
		t.Fatal("expected error for IR without a universal target")
	}
}

func optionNames(opts []provider.Option) []string {
	var names []string
	for _, o := range opts {
		names = append(names, o.Name)
	}
	return names
}
