package wizard

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCrawlJobSpecLocal(t *testing.T) {
	s := newCrawlFormState()
	s.startingURL = "https://example.com"

	spec, err := buildCrawlJobSpec(s, "")
	if err != nil {
		t.Fatalf("buildCrawlJobSpec: %v", err)
	}
	if spec.Source.Crawl == nil {
		t.Fatal("Source.Crawl = nil, want a crawl source")
	}
	if got := spec.Source.Crawl.Seeds; len(got) != 1 || got[0] != s.startingURL {
		t.Errorf("Source.Crawl.Seeds = %v, want [%q]", got, s.startingURL)
	}
	if spec.Source.Crawl.Depth != 1 {
		t.Errorf("Source.Crawl.Depth = %d, want 1 (the default)", spec.Source.Crawl.Depth)
	}
	if spec.Source.Crawl.MaxPages != 100 {
		t.Errorf("Source.Crawl.MaxPages = %d, want 100 (the default)", spec.Source.Crawl.MaxPages)
	}
	if spec.Source.Crawl.Scope != "same-host" {
		t.Errorf("Source.Crawl.Scope = %q, want \"same-host\" (the default)", spec.Source.Crawl.Scope)
	}
	if spec.Source.Crawl.Sitemap {
		t.Error("Source.Crawl.Sitemap = true, want false (the default)")
	}
	if spec.Source.Crawl.Robots != "respect" {
		t.Errorf("Source.Crawl.Robots = %q, want \"respect\" (the default)", spec.Source.Crawl.Robots)
	}
	if spec.Fetch.Use != "local" {
		t.Errorf("Fetch.Use = %q, want \"local\"", spec.Fetch.Use)
	}
	if spec.Limits.RPS != 2 {
		t.Errorf("Limits.RPS = %d, want 2 (crawl's default differs from scrape's)", spec.Limits.RPS)
	}
	if len(spec.Stages) != 0 {
		t.Errorf("Stages = %v, want none for the default \"bare\" extract mode", spec.Stages)
	}
}

func TestBuildCrawlJobSpecSitemapAndRobots(t *testing.T) {
	s := newCrawlFormState()
	s.startingURL = "https://example.com"
	s.sitemap = "true"
	s.robots = "ignore"

	spec, err := buildCrawlJobSpec(s, "")
	if err != nil {
		t.Fatalf("buildCrawlJobSpec: %v", err)
	}
	if !spec.Source.Crawl.Sitemap {
		t.Error("Source.Crawl.Sitemap = false, want true")
	}
	if spec.Source.Crawl.Robots != "ignore" {
		t.Errorf("Source.Crawl.Robots = %q, want \"ignore\"", spec.Source.Crawl.Robots)
	}
}

func TestBuildCrawlJobSpecProviderIncludesResolvedOptions(t *testing.T) {
	s := newCrawlFormState()
	s.startingURL = "https://example.com"
	s.fetchVia = "provider"
	s.providerName = "decodo"
	s.providerOptions = map[string]string{"headless": "png", "geo": "us"}

	spec, err := buildCrawlJobSpec(s, writeManifest(t, "decodo", flatOptionsManifest))
	if err != nil {
		t.Fatalf("buildCrawlJobSpec: %v", err)
	}
	if spec.Fetch.Use != "provider" || spec.Fetch.Name != "decodo" {
		t.Fatalf("Fetch = %+v, want use=provider name=decodo", spec.Fetch)
	}
	if got := spec.Fetch.Options["headless"]; got != "png" {
		t.Errorf(`Fetch.Options["headless"] = %v, want "png"`, got)
	}
	if got := spec.Fetch.Options["geo"]; got != "us" {
		t.Errorf(`Fetch.Options["geo"] = %v, want "us"`, got)
	}
}

func TestBuildCrawlJobSpecUnknownProviderErrors(t *testing.T) {
	s := newCrawlFormState()
	s.startingURL = "https://example.com"
	s.fetchVia = "provider"
	s.providerName = "nope"

	if _, err := buildCrawlJobSpec(s, t.TempDir()); err == nil {
		t.Fatal("buildCrawlJobSpec with an unknown provider = nil error, want one")
	}
}

func TestDoCrawlSaveWritesJobFile(t *testing.T) {
	t.Chdir(t.TempDir())

	s := newCrawlFormState()
	s.startingURL = "https://example.com"

	status := doCrawlSave(s, "")
	if !strings.HasPrefix(status, "✓") {
		t.Fatalf("doCrawlSave status = %q, want success", status)
	}

	data, err := os.ReadFile(savedJobFilename)
	if err != nil {
		t.Fatalf("reading %s: %v", savedJobFilename, err)
	}
	if !strings.Contains(string(data), "https://example.com") {
		t.Errorf("%s doesn't contain the starting URL:\n%s", savedJobFilename, data)
	}
}

func TestDoCrawlSaveReportsBuildErrors(t *testing.T) {
	t.Chdir(t.TempDir())

	s := newCrawlFormState()
	s.startingURL = ""

	status := doCrawlSave(s, "")
	if !strings.HasPrefix(status, "✗") {
		t.Fatalf("doCrawlSave status = %q, want a failure", status)
	}
}

func TestDoCrawlRunAgainstLocalServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body><h1>hello</h1></body></html>")
	}))
	defer srv.Close()

	s := newCrawlFormState()
	s.startingURL = srv.URL
	s.depth = "0"
	s.extract = "raw"

	status, rows := doCrawlRun(context.Background(), s, t.TempDir(), filepath.Join(t.TempDir(), "credentials"), nil)
	if !strings.HasPrefix(status, "✓") {
		t.Fatalf("doCrawlRun status = %q, want success", status)
	}
	if len(rows) != 1 {
		t.Fatalf("doCrawlRun rows = %v, want exactly 1 row", rows)
	}
}
