package crawl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pobear/pobear/internal/testutil"
	"github.com/pobear/pobear/services/core/internal/job/workers/crawl"
)

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir for %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func rewriteBase(t *testing.T, dir, base string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path) //nolint:gosec // path is this test's own t.TempDir() fixture file
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		rewritten := strings.ReplaceAll(string(data), "{{base}}", base)
		if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil { //nolint:gosec // path is this test's own t.TempDir() fixture file
			t.Fatalf("rewriting %s: %v", path, err)
		}
	}
}

func TestHTTPSitemapFetcherReadsAFlatURLSet(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "sitemap.xml", `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>{{base}}/a</loc></url>
  <url><loc>{{base}}/b</loc></url>
</urlset>`)
	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()
	rewriteBase(t, dir, srv.URL)

	f := crawl.NewHTTPSitemapFetcher(srv.Client())
	got, err := f.URLs(context.Background(), srv.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	want := []string{srv.URL + "/a", srv.URL + "/b"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHTTPSitemapFetcherFollowsOneLevelOfSitemapIndex(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "sitemap.xml", `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>{{base}}/section-a.xml</loc></sitemap>
  <sitemap><loc>{{base}}/section-b.xml</loc></sitemap>
</sitemapindex>`)
	writeFixture(t, dir, "section-a.xml", `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>{{base}}/a1</loc></url>
</urlset>`)
	writeFixture(t, dir, "section-b.xml", `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>{{base}}/b1</loc></url>
</urlset>`)
	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()
	rewriteBase(t, dir, srv.URL)

	f := crawl.NewHTTPSitemapFetcher(srv.Client())
	got, err := f.URLs(context.Background(), srv.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	want := map[string]bool{srv.URL + "/a1": true, srv.URL + "/b1": true}
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 URLs (one from each child sitemap)", got)
	}
	for _, u := range got {
		if !want[u] {
			t.Fatalf("got unexpected URL %q, want one of %v", u, want)
		}
	}
}

func TestHTTPSitemapFetcherSkipsCrossOriginChildSitemap(t *testing.T) {
	var hitOtherOrigin atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitOtherOrigin.Store(true)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>http://example.invalid/exfil</loc></url>
</urlset>`))
	}))
	defer other.Close()

	dir := t.TempDir()
	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()
	writeFixture(t, dir, "sitemap.xml", `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>`+other.URL+`/external.xml</loc></sitemap>
</sitemapindex>`)

	f := crawl.NewHTTPSitemapFetcher(srv.Client())
	got, err := f.URLs(context.Background(), srv.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if hitOtherOrigin.Load() {
		t.Fatal("fetched a child sitemap from a different origin than the top-level sitemap.xml -- want it skipped, never requested")
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want no URLs (the only child sitemap was cross-origin and should be skipped)", got)
	}
}

func TestHTTPSitemapFetcherErrorsWhenSitemapMissing(t *testing.T) {
	dir := t.TempDir()
	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()

	f := crawl.NewHTTPSitemapFetcher(srv.Client())
	_, err := f.URLs(context.Background(), srv.URL)

	if err == nil {
		t.Fatal("expected an error for a missing sitemap.xml (404), got nil")
	}
}

func TestHTTPSitemapFetcherErrorsOnMalformedXML(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "sitemap.xml", "not valid xml at all")
	srv := testutil.NewFixtureServer(dir)
	defer srv.Close()

	f := crawl.NewHTTPSitemapFetcher(srv.Client())
	_, err := f.URLs(context.Background(), srv.URL)

	if err == nil {
		t.Fatal("expected an error for malformed XML, got nil")
	}
}
