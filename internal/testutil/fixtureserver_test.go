package testutil_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Decodo/pobear/internal/testutil"
)

func TestNewFixtureServerServesAFileAtItsPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page1.html"), []byte("<h1>hi</h1>"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/page1.html")
	if err != nil {
		t.Fatalf("GET /page1.html: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(got) != "<h1>hi</h1>" {
		t.Fatalf("body = %q, want %q", got, "<h1>hi</h1>")
	}
}

func TestNewFixtureServerServesNestedPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "robots.txt"), []byte("User-agent: *\nDisallow: /private\n"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/robots.txt")
	if err != nil {
		t.Fatalf("GET /robots.txt: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestNewFixtureServer404sOnMissingPath(t *testing.T) {
	dir := t.TempDir()
	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nope.html")
	if err != nil {
		t.Fatalf("GET /nope.html: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestNewFixtureServerRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()

	rel, err := filepath.Rel(dir, filepath.Join(outside, "secret.txt"))
	if err != nil {
		t.Fatalf("computing relative path: %v", err)
	}
	resp, err := http.Get(srv.URL + "/" + filepath.ToSlash(rel))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (path traversal must not escape dir)", resp.StatusCode)
	}
}
