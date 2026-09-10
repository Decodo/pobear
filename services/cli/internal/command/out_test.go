package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsDirOut(t *testing.T) {
	if isDirOut("") {
		t.Error(`isDirOut("") = true, want false`)
	}

	notYetCreated := filepath.Join(t.TempDir(), "does-not-exist-yet") + string(os.PathSeparator)
	if !isDirOut(notYetCreated) {
		t.Errorf("isDirOut(%q) = false, want true (trailing separator signals directory intent)", notYetCreated)
	}

	existingDir := t.TempDir()
	if !isDirOut(existingDir) {
		t.Errorf("isDirOut(%q) = false, want true (already exists as a directory)", existingDir)
	}

	existingFile := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(existingFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if isDirOut(existingFile) {
		t.Errorf("isDirOut(%q) = true, want false (already exists as a regular file)", existingFile)
	}

	notYetCreatedNoSlash := filepath.Join(t.TempDir(), "does-not-exist-yet-no-slash")
	if isDirOut(notYetCreatedNoSlash) {
		t.Errorf("isDirOut(%q) = true, want false (doesn't exist, no trailing separator)", notYetCreatedNoSlash)
	}
}

func TestExtractRawValue(t *testing.T) {
	if _, err := extractRawValue(map[string]any{}, "title"); err == nil {
		t.Fatal("expected an error when the field was never produced")
	}

	got, err := extractRawValue(map[string]any{"title": []any{"only one"}}, "title")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "only one" {
		t.Fatalf("got %q, want %q", got, "only one")
	}

	if _, err := extractRawValue(map[string]any{"title": []any{}}, "title"); err == nil {
		t.Fatal("expected an error for zero matches (ambiguous)")
	}
	if _, err := extractRawValue(map[string]any{"title": []any{"a", "b"}}, "title"); err == nil {
		t.Fatal("expected an error for 2+ matches (ambiguous)")
	}

	got, err = extractRawValue(map[string]any{"raw": "plain string"}, "raw")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "plain string" {
		t.Fatalf("got %q, want %q", got, "plain string")
	}
}

func TestURLFromFailureLine(t *testing.T) {
	cases := map[string]string{
		"https://a.test [http]: tiers: http status 404": "https://a.test",
		"https://a.test: connection refused":            "https://a.test",
		"https://a.test":                                "https://a.test",
	}
	for line, want := range cases {
		if got := urlFromFailureLine(line); got != want {
			t.Errorf("urlFromFailureLine(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestWriteDirOutputsJSONFormat(t *testing.T) {
	tmpPath := filepath.Join(t.TempDir(), "out.ndjson")
	content := `{"url":"https://a.test","status":200,"data":{"raw":"a"}}
{"url":"https://b.test","status":200,"data":{"raw":"b"}}
`
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	outDir := t.TempDir()

	entries, err := writeDirOutputs(tmpPath, outDir, "json", "", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].URL != "https://a.test" || entries[0].File != "0001.json" || !entries[0].Succeeded || entries[0].Status != 200 {
		t.Errorf("entries[0] = %+v, want url=https://a.test file=0001.json succeeded=true status=200", entries[0])
	}
	if entries[1].File != "0002.json" {
		t.Errorf("entries[1].File = %q, want 0002.json", entries[1].File)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "0001.json")) //nolint:gosec // outDir is t.TempDir(), not user-controlled input
	if err != nil {
		t.Fatalf("reading 0001.json: %v", err)
	}
	if string(got) != `{"url":"https://a.test","status":200,"data":{"raw":"a"}}` {
		t.Fatalf("0001.json content = %q, want the verbatim NDJSON line", got)
	}
}

func TestWriteDirOutputsRawFormatPerItemFailureDoesNotAbortOthers(t *testing.T) {
	tmpPath := filepath.Join(t.TempDir(), "out.ndjson")
	content := `{"url":"https://a.test","status":200,"data":{"title":["Hello"]}}
{"url":"https://b.test","status":200,"data":{}}
{"url":"https://c.test","status":200,"data":{"title":["World"]}}
`
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	outDir := t.TempDir()

	entries, err := writeDirOutputs(tmpPath, outDir, "raw", "title", nil)
	if err != nil {
		t.Fatalf("expected writeDirOutputs itself to succeed even though one item's extraction failed, got %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	if !entries[0].Succeeded || entries[0].File != "0001.html" {
		t.Errorf("entries[0] = %+v, want succeeded with file 0001.html", entries[0])
	}
	if entries[1].Succeeded || entries[1].Error == "" {
		t.Errorf("entries[1] = %+v, want a failed entry (title field never produced) with a non-empty error, not aborting the whole batch", entries[1])
	}
	if entries[1].File != "" {
		t.Errorf("entries[1].File = %q, want empty (extraction failed, no file written)", entries[1].File)
	}
	// The 3rd item still gets its own file, numbered by NDJSON line
	// position, not by how many files were actually written so far.
	if !entries[2].Succeeded || entries[2].File != "0003.html" {
		t.Errorf("entries[2] = %+v, want succeeded with file 0003.html", entries[2])
	}

	if _, err := os.Stat(filepath.Join(outDir, "0002.html")); err == nil {
		t.Fatal("expected no 0002.html to have been written for the failed extraction")
	}
	got, err := os.ReadFile(filepath.Join(outDir, "0001.html")) //nolint:gosec // outDir is t.TempDir(), not user-controlled input
	if err != nil {
		t.Fatalf("reading 0001.html: %v", err)
	}
	if string(got) != "Hello" {
		t.Fatalf("0001.txt content = %q, want %q", got, "Hello")
	}
}

func TestWriteDirOutputsIncludesFailureLines(t *testing.T) {
	tmpPath := filepath.Join(t.TempDir(), "out.ndjson")
	content := `{"url":"https://a.test","status":200,"data":{"raw":"a"}}
`
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	outDir := t.TempDir()

	failureLine := `https://b.test [http]: tiers: http status 404`
	entries, err := writeDirOutputs(tmpPath, outDir, "json", "", []string{failureLine})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (1 succeeded + 1 failed)", len(entries))
	}
	failed := entries[1]
	if failed.Succeeded || failed.URL != "https://b.test" || failed.Error != failureLine {
		t.Errorf("failed entry = %+v, want url=https://b.test error=%q", failed, failureLine)
	}
}

func TestWriteDirIndexDoesNotEscapeHTML(t *testing.T) {
	outDir := t.TempDir()
	entries := []dirIndexEntry{{URL: "https://a.test", Succeeded: false, Error: "status 403: <h1>blocked</h1>"}}

	if err := writeDirIndex(outDir, entries); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "index.json")) //nolint:gosec // outDir is t.TempDir(), not user-controlled input
	if err != nil {
		t.Fatalf("reading index.json: %v", err)
	}
	if !strings.Contains(string(got), "<h1>blocked</h1>") {
		t.Fatalf("index.json = %s, want the literal HTML, not escaped to \\u003c", got)
	}
}
