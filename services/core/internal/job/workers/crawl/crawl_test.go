package crawl_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/job/workers/crawl"
)

type fakeScrapeFailure struct {
	kind fault.Kind
	err  error
}

type fakeScrapeWorker struct {
	children map[string][]string
	fails    map[string]fakeScrapeFailure
	delay    time.Duration
	visited  []string
}

func (f *fakeScrapeWorker) Run(_ context.Context, _ *jobspec.Spec, url, _ string, _ int) ([]string, fault.Kind, error) {
	f.visited = append(f.visited, url)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if failure, ok := f.fails[url]; ok {
		return nil, failure.kind, failure.err
	}
	return f.children[url], "", nil
}

func TestCrawlDepthZeroVisitsOnlySeeds(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/child"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 0, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 1 {
		t.Fatalf("got Succeeded=%d, want 1 (seed only, depth=0 means no link-following)", got.Succeeded)
	}
	if len(scrape.visited) != 1 || scrape.visited[0] != "https://a.test" {
		t.Fatalf("visited %v, want exactly [https://a.test]", scrape.visited)
	}
}

func TestCrawlDepthOneFollowsOneLevelOfLinks(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test":       {"https://a.test/child"},
		"https://a.test/child": {"https://a.test/grandchild"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + 1 level of links, grandchild never reached)", got.Succeeded)
	}
	for _, want := range []string{"https://a.test", "https://a.test/child"} {
		found := false
		for _, v := range scrape.visited {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q visited, got %v", want, scrape.visited)
		}
	}
	for _, v := range scrape.visited {
		if v == "https://a.test/grandchild" {
			t.Fatalf("grandchild visited at depth 2, exceeding Depth=1: %v", scrape.visited)
		}
	}
}

func TestCrawlMaxPagesStopsAtLimit(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/1", "https://a.test/2", "https://a.test/3"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 5, MaxPages: 2, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want exactly 2 (MaxPages=2 caps total fetches regardless of Depth)", got.Succeeded)
	}
	if len(scrape.visited) != 2 {
		t.Fatalf("visited %d URLs, want exactly 2", len(scrape.visited))
	}
	if got.Skipped != 1 {
		t.Fatalf("got Skipped=%d, want 1 (the 3rd discovered child never attempted once MaxPages=2 was hit)", got.Skipped)
	}
}

func buildURLChain(length int) map[string][]string {
	children := make(map[string][]string, length)
	for i := 0; i < length; i++ {
		url := fmt.Sprintf("https://a.test/%d", i)
		next := fmt.Sprintf("https://a.test/%d", i+1)
		children[url] = []string{next}
	}
	return children
}

func TestCrawlMaxDurationStopsTheWalk(t *testing.T) {
	children := buildURLChain(50)
	children["https://a.test"] = []string{"https://a.test/0"}
	scrape := &fakeScrapeWorker{children: children, delay: 20 * time.Millisecond}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1000, MaxPages: 10000, MaxDuration: "100ms", Scope: "same-host", Robots: "ignore",
	}}}
	start := time.Now()
	_, err := c.Run(context.Background(), spec)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error (MaxDuration is a stopping condition, not a failure), got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Run took %v, expected it to stop promptly once MaxDuration elapsed, not hang", elapsed)
	}
	if len(scrape.visited) >= 10 {
		t.Fatalf("visited %d URLs, want well under the 51-URL chain length, proving MaxDuration (not chain exhaustion) stopped the walk", len(scrape.visited))
	}
}

func TestCrawlStoppingConditionsAreIndependent(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/1", "https://a.test/2"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 0, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 1 {
		t.Fatalf("got Succeeded=%d, want 1 — Depth=0 alone must stop the walk even though MaxPages=100 would allow more", got.Succeeded)
	}
}

type fakeRobotsChecker struct {
	disallowed map[string]bool
}

func (f *fakeRobotsChecker) Allowed(_ context.Context, url string) (bool, error) {
	return !f.disallowed[url], nil
}

type fakeSitemapFetcher struct {
	urls map[string][]string
}

func (f *fakeSitemapFetcher) URLs(_ context.Context, origin string) ([]string, error) {
	return f.urls[origin], nil
}

func TestCrawlScopeSameHostExcludesOtherHosts(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/child", "https://other.test/page", "https://sub.a.test/page"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + same-host child)", got.Succeeded)
	}
	if got.Skipped != 2 {
		t.Fatalf("got Skipped=%d, want 2 (other.test and the sub.a.test subdomain both filtered by scope=same-host, never fetched)", got.Skipped)
	}
	for _, v := range scrape.visited {
		if v == "https://other.test/page" {
			t.Fatal("other.test was fetched despite scope=same-host")
		}
		if v == "https://sub.a.test/page" {
			t.Fatal("sub.a.test was fetched despite scope=same-host (same-host must exclude subdomains too)")
		}
	}
}

func TestCrawlScopeSameDomainAllowsSubdomains(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://sub.a.test/page", "https://other.test/page"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-domain", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + subdomain, scope=same-domain allows subdomains)", got.Succeeded)
	}
	if got.Skipped != 1 {
		t.Fatalf("got Skipped=%d, want 1 (other.test filtered by scope=same-domain, never fetched)", got.Skipped)
	}
	found := false
	for _, v := range scrape.visited {
		if v == "https://sub.a.test/page" {
			found = true
		}
		if v == "https://other.test/page" {
			t.Fatal("other.test was fetched despite scope=same-domain")
		}
	}
	if !found {
		t.Fatalf("expected https://sub.a.test/page visited, got %v", scrape.visited)
	}
}

func TestCrawlScopeAnyAllowsEveryHost(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://other.test/page"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "any", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 || got.Skipped != 0 {
		t.Fatalf("got %+v, want Succeeded=2 Skipped=0 (scope=any follows every host)", got)
	}
}

func TestCrawlExcludeWinsOverInclude(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/blog/post", "https://a.test/blog/draft"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "ignore",
		Include: []string{"**/blog/**"}, Exclude: []string{"**/draft"},
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + /blog/post, matches include and not exclude)", got.Succeeded)
	}
	if got.Skipped != 1 {
		t.Fatalf("got Skipped=%d, want 1 (/blog/draft matches include AND exclude — exclude wins per JOB_SPEC_REFERENCE.md §3.2)", got.Skipped)
	}
}

func TestCrawlSitemapSeedsAdditionalURLs(t *testing.T) {
	scrape := &fakeScrapeWorker{}
	sitemap := &fakeSitemapFetcher{urls: map[string][]string{
		"https://a.test": {"https://a.test/from-sitemap"},
	}}
	c := crawl.NewCrawl(scrape, nil, sitemap)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 0, MaxPages: 100, Scope: "same-host", Robots: "ignore", Sitemap: true,
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + the sitemap-seeded URL, even though Depth=0 would otherwise block link-following)", got.Succeeded)
	}
}

func TestCrawlSitemapFalseNeverConsultsSitemapFetcher(t *testing.T) {
	scrape := &fakeScrapeWorker{}
	sitemap := &fakeSitemapFetcher{urls: map[string][]string{"https://a.test": {"https://a.test/from-sitemap"}}}
	c := crawl.NewCrawl(scrape, nil, sitemap)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 0, MaxPages: 100, Scope: "same-host", Robots: "ignore", Sitemap: false,
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 1 {
		t.Fatalf("got Succeeded=%d, want 1 (Sitemap=false, the sitemap-seeded URL must never be added)", got.Succeeded)
	}
}

func TestCrawlRobotsRespectSkipsDisallowedURLs(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/private", "https://a.test/public"},
	}}
	robots := &fakeRobotsChecker{disallowed: map[string]bool{"https://a.test/private": true}}
	c := crawl.NewCrawl(scrape, robots, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "respect",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + /public)", got.Succeeded)
	}
	if got.Skipped != 1 {
		t.Fatalf("got Skipped=%d, want 1 (/private disallowed by robots.txt, never fetched)", got.Skipped)
	}
	for _, v := range scrape.visited {
		if v == "https://a.test/private" {
			t.Fatal("/private was fetched despite robots=respect disallowing it")
		}
	}
}

func TestCrawlRobotsIgnoreNeverConsultsRobotsChecker(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/private"},
	}}
	robots := &fakeRobotsChecker{disallowed: map[string]bool{"https://a.test/private": true}}
	c := crawl.NewCrawl(scrape, robots, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 || got.Skipped != 0 {
		t.Fatalf("got %+v, want Succeeded=2 Skipped=0 (robots=ignore fetches everything scope/include/exclude allow)", got)
	}
}

func TestCrawlTalliesFailuresAndTheirKinds(t *testing.T) {
	scrape := &fakeScrapeWorker{
		children: map[string][]string{
			"https://a.test":       {"https://a.test/child"},
			"https://a.test/child": {"https://a.test/grandchild"},
		},
		fails: map[string]fakeScrapeFailure{
			"https://a.test/child": {kind: fault.Timeout, err: errors.New("timed out")},
		},
	}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 5, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 1 {
		t.Fatalf("got Succeeded=%d, want 1 (only the seed)", got.Succeeded)
	}
	if got.Failed != 1 {
		t.Fatalf("got Failed=%d, want 1 (the child failed)", got.Failed)
	}
	if len(got.Kinds) != 1 || got.Kinds[0] != fault.Timeout {
		t.Fatalf("got Kinds=%v, want [fault.Timeout]", got.Kinds)
	}
	if len(got.Errors) != 1 || !strings.Contains(got.Errors[0], "https://a.test/child") || !strings.Contains(got.Errors[0], "timed out") {
		t.Fatalf("got Errors=%v, want one entry naming the failed URL and its error", got.Errors)
	}
	for _, v := range scrape.visited {
		if v == "https://a.test/grandchild" {
			t.Fatal("grandchild visited, but its parent failed and should never have contributed links to the frontier")
		}
	}
}

func TestCrawlDedupeSkipsALinkAlreadyDiscoveredFromAnotherParent(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test":       {"https://a.test/other", "https://a.test/shared"},
		"https://a.test/other": {"https://a.test/shared"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 2, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 3 {
		t.Fatalf("got Succeeded=%d, want 3 (seed + /other + /shared, /shared discovered twice but fetched once)", got.Succeeded)
	}
	sharedCount := 0
	for _, v := range scrape.visited {
		if v == "https://a.test/shared" {
			sharedCount++
		}
	}
	if sharedCount != 1 {
		t.Fatalf("https://a.test/shared visited %d times, want exactly 1", sharedCount)
	}
}

func TestCrawlDedupeDiscardsADuplicateSeed(t *testing.T) {
	scrape := &fakeScrapeWorker{}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test", "https://a.test"}, Depth: 0, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 1 {
		t.Fatalf("got Succeeded=%d, want 1 (the duplicate seed discarded outright, not fetched twice, not counted as skipped)", got.Succeeded)
	}
	if got.Skipped != 0 {
		t.Fatalf("got Skipped=%d, want 0 (a dedupe discard is not a skip)", got.Skipped)
	}
	if len(scrape.visited) != 1 {
		t.Fatalf("visited %v, want exactly one fetch", scrape.visited)
	}
}

func TestCrawlDedupeTreatsFragmentOnlyDifferenceAsSameURL(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/page#section-a", "https://a.test/page#section-b"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + one fetch of /page, the second differing only by fragment discarded)", got.Succeeded)
	}
	gotA, gotB := false, false
	for _, v := range scrape.visited {
		if v == "https://a.test/page#section-a" {
			gotA = true
		}
		if v == "https://a.test/page#section-b" {
			gotB = true
		}
	}
	if gotA && gotB {
		t.Fatalf("both fragment variants were fetched, want only one: %v", scrape.visited)
	}
	if !gotA && !gotB {
		t.Fatalf("neither fragment variant was fetched: %v", scrape.visited)
	}
}

func TestCrawlDedupeTreatsReorderedQueryParamsAsSameURL(t *testing.T) {
	scrape := &fakeScrapeWorker{children: map[string][]string{
		"https://a.test": {"https://a.test/page?a=1&b=2", "https://a.test/page?b=2&a=1"},
	}}
	c := crawl.NewCrawl(scrape, nil, nil)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{
		Seeds: []string{"https://a.test"}, Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "ignore",
	}}}
	got, err := c.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 2 {
		t.Fatalf("got Succeeded=%d, want 2 (seed + one fetch of /page, the reordered-query-param duplicate discarded)", got.Succeeded)
	}
}
