package provider_test

import (
	"testing"

	"github.com/Decodo/pobear/pkg/provider"
)

func TestRenderBodyAndQuery(t *testing.T) {
	req := provider.Request{
		Body:  map[string]any{"url": "{{ .URL }}", "render": "{{ .Options.render }}", "count": 3, "flag": true},
		Query: map[string]any{"device": "{{ .Options.device }}"},
	}
	options := map[string]any{"render": true, "device": "mobile"}

	body, query, err := provider.Render(req, "https://a.test/page", options)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if body["url"] != "https://a.test/page" {
		t.Errorf(`body["url"] = %v, want %q`, body["url"], "https://a.test/page")
	}
	if body["render"] != true {
		t.Errorf(`body["render"] = %v (%T), want the real bool true, not its stringified form`, body["render"], body["render"])
	}
	if body["count"] != 3 {
		t.Errorf(`body["count"] = %v, want 3 (non-string values pass through untouched)`, body["count"])
	}
	if body["flag"] != true {
		t.Errorf(`body["flag"] = %v, want true (non-string values pass through untouched)`, body["flag"])
	}
	if query["device"] != "mobile" {
		t.Errorf(`query["device"] = %q, want %q`, query["device"], "mobile")
	}
}

func TestRenderBareOptionsRefPreservesNativeType(t *testing.T) {
	req := provider.Request{
		Body: map[string]any{
			"page_count": "{{ .Options.page_count }}",
			"parse":      "{{ .Options.parse }}",
			"geo":        "{{ .Options.geo }}",
		},
	}
	options := map[string]any{"page_count": 3, "parse": false, "geo": "us"}

	body, _, err := provider.Render(req, "https://a.test", options)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if body["page_count"] != 3 {
		t.Errorf(`body["page_count"] = %v (%T), want the real int 3`, body["page_count"], body["page_count"])
	}
	if body["parse"] != false {
		t.Errorf(`body["parse"] = %v (%T), want the real bool false`, body["parse"], body["parse"])
	}
	if body["geo"] != "us" {
		t.Errorf(`body["geo"] = %v, want %q`, body["geo"], "us")
	}
}

func TestRenderComposedTemplateStillStringifies(t *testing.T) {
	req := provider.Request{Body: map[string]any{"ua": "device-{{ .Options.device }}"}}
	options := map[string]any{"device": "mobile"}

	body, _, err := provider.Render(req, "https://a.test", options)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if body["ua"] != "device-mobile" {
		t.Errorf(`body["ua"] = %v, want %q (a template mixing text with a reference has no single typed value, so it stays a string)`, body["ua"], "device-mobile")
	}
}

func TestRenderEmptyTemplateResultIsDropped(t *testing.T) {
	req := provider.Request{
		Body:  map[string]any{"geo": "{{ .Options.geo }}"},
		Query: map[string]any{"q": "{{ .Options.missing }}"},
	}

	body, query, err := provider.Render(req, "https://a.test", map[string]any{})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, present := body["geo"]; present {
		t.Errorf(`body["geo"] present = %v, want dropped (renders to "")`, body["geo"])
	}
	if _, present := query["q"]; present {
		t.Errorf(`query["q"] present = %v, want dropped (renders to "")`, query["q"])
	}
}

func TestRenderInvalidTemplateSyntaxErrors(t *testing.T) {
	req := provider.Request{Body: map[string]any{"bad": "{{ .URL"}}

	_, _, err := provider.Render(req, "https://a.test", nil)

	if err == nil {
		t.Fatal("expected an error for invalid template syntax, got nil")
	}
}

func TestExtractSuccessfulMatches(t *testing.T) {
	resp := provider.Response{Status: []string{"$.logicalStatus"}, Body: "$.content", FinalURL: "$.resolvedUrl"}
	httpBody := []byte(`{"logicalStatus":429,"content":"the page","resolvedUrl":"https://a.test/final"}`)

	status, body, finalURL, _, err := provider.Extract(resp, 200, httpBody, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != 429 {
		t.Errorf("status = %d, want 429", status)
	}
	if string(body) != "the page" {
		t.Errorf("body = %q, want %q", body, "the page")
	}
	if finalURL != "https://a.test/final" {
		t.Errorf("finalURL = %q, want %q", finalURL, "https://a.test/final")
	}
}

func TestExtractStructuredBodyIsJSON(t *testing.T) {
	resp := provider.Response{Body: "$.content"}
	cases := []struct {
		name     string
		httpBody string
		want     string
	}{
		{"object", `{"content":{"price":9.96,"tags":["a"]}}`, `{"price":9.96,"tags":["a"]}`},
		{"array", `{"content":[1,"x"]}`, `[1,"x"]`},
		{"string stays raw", `{"content":"<html>"}`, `<html>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, body, _, _, err := provider.Extract(resp, 200, []byte(c.httpBody), "https://a.test")
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != c.want {
				t.Errorf("body = %s, want %s", body, c.want)
			}
		})
	}
}

func TestExtractBodyMissProducesEmptyNotNilString(t *testing.T) {
	resp := provider.Response{Body: "$.content"}

	_, body, _, _, err := provider.Extract(resp, 500, []byte(`{"error":"boom"}`), "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(body) != "" {
		t.Fatalf(`body = %q, want empty -- a JSONPath miss must never surface as the literal string "<nil>"`, body)
	}
}

func TestExtractBodyMissOnEmptyHTTPBodyProducesEmptyBody(t *testing.T) {
	resp := provider.Response{Body: "$.content"}

	_, body, _, _, err := provider.Extract(resp, 500, nil, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(body) != "" {
		t.Fatalf("body = %q, want empty", body)
	}
}

func TestExtractStatusMissFallsBackToHTTPStatus(t *testing.T) {
	resp := provider.Response{Status: []string{"$.logicalStatus"}, Body: "$.content"}

	status, _, _, _, err := provider.Extract(resp, 500, []byte(`{"content":"x"}`), "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != 500 {
		t.Fatalf("status = %d, want 500 (falls back to httpStatus when the JSONPath doesn't match)", status)
	}
}

func TestExtractStatusTriesPathsInOrder(t *testing.T) {
	resp := provider.Response{Status: []string{"$.results[0].status_code", "$.status_code"}, Body: "$.results[0].content"}
	cases := []struct {
		name     string
		httpBody string
		want     int
	}{
		{"first path wins", `{"results":[{"status_code":200,"content":{"status_code":12000}}]}`, 200},
		{"falls back to second", `{"status_code":613}`, 613},
		{"falls back to http status", `{}`, 502},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, _, _, err := provider.Extract(resp, 502, []byte(c.httpBody), "https://a.test")
			if err != nil {
				t.Fatal(err)
			}
			if status != c.want {
				t.Errorf("status = %d, want %d", status, c.want)
			}
		})
	}
}

func TestExtractFinalURLMissFallsBackToRequestURL(t *testing.T) {
	resp := provider.Response{FinalURL: "$.redirectedTo", Body: "$.content"}

	_, _, finalURL, _, err := provider.Extract(resp, 200, []byte(`{"content":"x"}`), "https://a.test/page")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if finalURL != "https://a.test/page" {
		t.Fatalf("finalURL = %q, want the request URL (falls back when the JSONPath doesn't match)", finalURL)
	}
}

func TestExtractInvalidJSONPathErrors(t *testing.T) {
	resp := provider.Response{Body: "$["}

	_, _, _, _, err := provider.Extract(resp, 200, []byte(`{}`), "https://a.test")

	if err == nil {
		t.Fatal("expected an error for an invalid JSONPath, got nil")
	}
}

func TestApplyOptionDefaultsFillsUnsetOptions(t *testing.T) {
	declared := []provider.Option{
		{Name: "headless", Type: "enum", Default: "html"},
		{Name: "proxy_pool", Type: "enum", Default: "premium"},
		{Name: "geo", Type: "string"},
	}

	got := provider.ApplyOptionDefaults(declared, map[string]any{"geo": "us"})

	want := map[string]any{"headless": "html", "proxy_pool": "premium", "geo": "us"}
	if len(got) != len(want) {
		t.Fatalf("expected %d options, got %d: %v", len(want), len(got), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("option %q: expected %v, got %v", k, v, got[k])
		}
	}
}

func TestApplyOptionDefaultsDoesNotOverrideSuppliedValue(t *testing.T) {
	declared := []provider.Option{{Name: "headless", Type: "enum", Default: "html"}}

	got := provider.ApplyOptionDefaults(declared, map[string]any{"headless": "png"})

	if got["headless"] != "png" {
		t.Errorf("expected supplied value to win, got %v", got["headless"])
	}
}

func TestApplyOptionDefaultsLeavesSuppliedMapUnmodified(t *testing.T) {
	declared := []provider.Option{{Name: "headless", Type: "enum", Default: "html"}}
	supplied := map[string]any{"geo": "us"}

	provider.ApplyOptionDefaults(declared, supplied)

	if _, ok := supplied["headless"]; ok {
		t.Error("expected the supplied map to be left unmodified")
	}
}

func TestRenderDropsKeyWhenUnsetOptionIsInterpolated(t *testing.T) {
	req := provider.Request{
		Body: map[string]any{
			"url":    "{{ .URL }}",
			"intref": "count={{ .Options.n }}",
		},
	}

	body, _, err := provider.Render(req, "https://a.test/page", map[string]any{})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := body["intref"]; ok {
		t.Errorf("expected intref to be dropped, got %q", body["intref"])
	}
	if body["url"] != "https://a.test/page" {
		t.Errorf("expected url to survive, got %v", body["url"])
	}
}
