package provider_test

import (
	"testing"

	"github.com/pobear/pobear/pkg/provider"
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
	resp := provider.Response{Status: "$.logicalStatus", Body: "$.content", FinalURL: "$.resolvedUrl"}
	httpBody := []byte(`{"logicalStatus":429,"content":"the page","resolvedUrl":"https://a.test/final"}`)

	status, body, finalURL, err := provider.Extract(resp, 200, httpBody, "https://a.test")

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

func TestExtractBodyMissProducesEmptyNotNilString(t *testing.T) {
	resp := provider.Response{Body: "$.content"}

	_, body, _, err := provider.Extract(resp, 500, []byte(`{"error":"boom"}`), "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(body) != "" {
		t.Fatalf(`body = %q, want empty -- a JSONPath miss must never surface as the literal string "<nil>"`, body)
	}
}

func TestExtractBodyMissOnEmptyHTTPBodyProducesEmptyBody(t *testing.T) {
	resp := provider.Response{Body: "$.content"}

	_, body, _, err := provider.Extract(resp, 500, nil, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(body) != "" {
		t.Fatalf("body = %q, want empty", body)
	}
}

func TestExtractStatusMissFallsBackToHTTPStatus(t *testing.T) {
	resp := provider.Response{Status: "$.logicalStatus", Body: "$.content"}

	status, _, _, err := provider.Extract(resp, 500, []byte(`{"content":"x"}`), "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != 500 {
		t.Fatalf("status = %d, want 500 (falls back to httpStatus when the JSONPath doesn't match)", status)
	}
}

func TestExtractFinalURLMissFallsBackToRequestURL(t *testing.T) {
	resp := provider.Response{FinalURL: "$.redirectedTo", Body: "$.content"}

	_, _, finalURL, err := provider.Extract(resp, 200, []byte(`{"content":"x"}`), "https://a.test/page")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if finalURL != "https://a.test/page" {
		t.Fatalf("finalURL = %q, want the request URL (falls back when the JSONPath doesn't match)", finalURL)
	}
}

func TestExtractInvalidJSONPathErrors(t *testing.T) {
	resp := provider.Response{Body: "$["}

	_, _, _, err := provider.Extract(resp, 200, []byte(`{}`), "https://a.test")

	if err == nil {
		t.Fatal("expected an error for an invalid JSONPath, got nil")
	}
}
