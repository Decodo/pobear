package provider

import "testing"

func TestCompiledTemplateReusesTheSameCompiledTemplate(t *testing.T) {
	const src = "{{ .URL }}-cache-test"

	first, err := compiledTemplate(src)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	second, err := compiledTemplate(src)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if first != second {
		t.Fatalf("compiledTemplate(%q) returned two different *template.Template values across calls, want the same cached one reused", src)
	}
}

func TestCompiledTemplateDoesNotCacheAParseFailure(t *testing.T) {
	const src = "{{ .URL-invalid-cache-test"

	if _, err := compiledTemplate(src); err == nil {
		t.Fatal("expected a parse error, got nil")
	}
	if _, ok := templateCache.Load(src); ok {
		t.Fatal("expected a failed parse never to be cached")
	}
}
