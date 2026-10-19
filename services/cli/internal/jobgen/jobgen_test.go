package jobgen_test

import (
	"errors"
	"testing"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
)

func TestBuildMinimalScrape(t *testing.T) {
	in := jobgen.Input{
		URLs:     []string{"https://example.com"},
		FetchUse: "local",
	}

	got, err := jobgen.Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Version != 1 {
		t.Fatalf("Version = %d, want 1", got.Version)
	}
	if len(got.Source.URLs) != 1 || got.Source.URLs[0] != "https://example.com" {
		t.Fatalf("Source.URLs = %v, want [https://example.com]", got.Source.URLs)
	}
	if got.Fetch.Use != "local" {
		t.Fatalf("Fetch.Use = %q, want local", got.Fetch.Use)
	}
	if got.Limits.Concurrency != 8 || got.Limits.Retries != 3 || got.Limits.Backoff != "exponential" {
		t.Fatalf("Limits = %+v, want CLI defaults resolved", got.Limits)
	}
	if got.Sink.Use != "stdout" {
		t.Fatalf("Sink.Use = %q, want stdout", got.Sink.Use)
	}
}

func TestBuildCrawlWithDepth(t *testing.T) {
	in := jobgen.Input{
		URLs:     []string{"https://example.com"},
		FetchUse: "local",
		Crawl:    &jobgen.CrawlInput{Depth: 2, MaxPages: new(100), Scope: "same-host", Robots: "respect"},
		RPS:      new(2),
	}

	got, err := jobgen.Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Source.Crawl == nil {
		t.Fatalf("Source.Crawl is nil, want set")
	}
	if got.Source.Crawl.Depth != 2 {
		t.Fatalf("Source.Crawl.Depth = %d, want 2", got.Source.Crawl.Depth)
	}
	if len(got.Source.Crawl.Seeds) != 1 || got.Source.Crawl.Seeds[0] != "https://example.com" {
		t.Fatalf("Source.Crawl.Seeds = %v, want [https://example.com]", got.Source.Crawl.Seeds)
	}
	if got.Limits.RPS != 2 {
		t.Fatalf("Limits.RPS = %d, want 2 (crawl's own default)", got.Limits.RPS)
	}
}

func TestBuildProviderWithOptions(t *testing.T) {
	in := jobgen.Input{
		URLs:     []string{"https://example.com"},
		FetchUse: "provider",
		Provider: "decodo",
		Options:  map[string]any{"geo": "us"},
	}

	got, err := jobgen.Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Fetch.Name != "decodo" {
		t.Fatalf("Fetch.Name = %q, want decodo", got.Fetch.Name)
	}
	if got.Fetch.Options["geo"] != "us" {
		t.Fatalf("Fetch.Options[geo] = %v, want us", got.Fetch.Options["geo"])
	}
}

func TestBuildWithHeaders(t *testing.T) {
	in := jobgen.Input{
		URLs:     []string{"https://example.com"},
		FetchUse: "local",
		Headers:  map[string]string{"X-Custom": "1"},
	}

	got, err := jobgen.Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Fetch.Headers["X-Custom"] != "1" {
		t.Fatalf("Fetch.Headers[X-Custom] = %v, want 1", got.Fetch.Headers["X-Custom"])
	}
}

func TestBuildFetchDocIncludesLadderWhenTrue(t *testing.T) {
	spec, err := jobgen.Build(jobgen.Input{URLs: []string{"https://a.test"}, FetchUse: "local", Ladder: true, Provider: "acme", Template: "t"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !spec.Fetch.Ladder {
		t.Fatal("spec.Fetch.Ladder = false, want true")
	}
}

func TestBuildInvalidSpecReturnsError(t *testing.T) {
	in := jobgen.Input{FetchUse: "local"}

	_, err := jobgen.Build(in)
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
	var verr *jobspec.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *jobspec.ValidationError, got %T: %v", err, err)
	}
}

func TestMinimalJSONScrapeNoFlags(t *testing.T) {
	resolved, err := jobgen.Build(jobgen.Input{
		URLs: []string{"https://ip.decodo.com"}, FetchUse: "local",
		Profile: "chrome", Timeout: "30s", Concurrency: new(8), Retries: new(3),
		SinkUse: "stdout",
	})
	if err != nil {
		t.Fatalf("Build(resolved): %v", err)
	}
	defaults, err := jobgen.Build(jobgen.Input{
		URLs: []string{"https://ip.decodo.com"}, FetchUse: "local",
		Profile: "chrome", Timeout: "30s", Concurrency: new(8), Retries: new(3),
		SinkUse: "stdout",
	})
	if err != nil {
		t.Fatalf("Build(defaults): %v", err)
	}

	got, err := jobgen.MinimalJSON(resolved, defaults)
	if err != nil {
		t.Fatalf("MinimalJSON: %v", err)
	}

	want := `{
  "version": 1,
  "source": {
    "urls": [
      "https://ip.decodo.com"
    ]
  },
  "fetch": {
    "use": "local"
  }
}`
	if string(got) != want {
		t.Fatalf("MinimalJSON =\n%s\nwant\n%s", got, want)
	}
}

func TestMinimalJSONCrawlWithDepth(t *testing.T) {
	resolved, err := jobgen.Build(jobgen.Input{
		URLs: []string{"https://example.com"}, FetchUse: "local",
		Crawl:   &jobgen.CrawlInput{Depth: 2, MaxPages: new(100), Scope: "same-host", Robots: "respect"},
		Profile: "chrome", Timeout: "30s", Concurrency: new(8), Retries: new(3), RPS: new(2),
		SinkUse: "stdout",
	})
	if err != nil {
		t.Fatalf("Build(resolved): %v", err)
	}
	defaults, err := jobgen.Build(jobgen.Input{
		URLs: []string{"https://example.com"}, FetchUse: "local",
		Crawl:   &jobgen.CrawlInput{Depth: 1, MaxPages: new(100), Scope: "same-host", Robots: "respect"},
		Profile: "chrome", Timeout: "30s", Concurrency: new(8), Retries: new(3), RPS: new(2),
		SinkUse: "stdout",
	})
	if err != nil {
		t.Fatalf("Build(defaults): %v", err)
	}

	got, err := jobgen.MinimalJSON(resolved, defaults)
	if err != nil {
		t.Fatalf("MinimalJSON: %v", err)
	}

	want := `{
  "version": 1,
  "source": {
    "crawl": {
      "seeds": [
        "https://example.com"
      ],
      "depth": 2
    }
  },
  "fetch": {
    "use": "local"
  }
}`
	if string(got) != want {
		t.Fatalf("MinimalJSON =\n%s\nwant\n%s", got, want)
	}
}
