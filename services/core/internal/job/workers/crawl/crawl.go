package crawl

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

type ScrapeWorker interface {
	Run(ctx context.Context, spec *jobspec.Spec, url, parentURL string, depth int) (links []string, kind fault.Kind, err error)
}

type RobotsChecker interface {
	Allowed(ctx context.Context, url string) (bool, error)
}

type SitemapFetcher interface {
	URLs(ctx context.Context, origin string) ([]string, error)
}

type Result struct {
	Succeeded int
	Failed    int
	Skipped   int
	Kinds     []fault.Kind
	Errors    []string
}

type Crawl struct {
	scrape  ScrapeWorker
	robots  RobotsChecker
	sitemap SitemapFetcher
}

func NewCrawl(scrape ScrapeWorker, robots RobotsChecker, sitemap SitemapFetcher) *Crawl {
	return &Crawl{scrape: scrape, robots: robots, sitemap: sitemap}
}

type frontierEntry struct {
	url       string
	parentURL string
	depth     int
	origin    string
}

func (c *Crawl) Run(ctx context.Context, spec *jobspec.Spec) (Result, error) {
	cs := spec.Source.Crawl
	deadline := crawlDeadline(cs)
	frontier := c.seedFrontier(ctx, cs)

	var result Result
	for len(frontier) > 0 {
		if !deadline.IsZero() && time.Now().After(deadline) {
			break
		}

		entry := frontier[0]
		frontier = frontier[1:]

		if c.shouldSkip(ctx, entry, cs) {
			result.Skipped++
			continue
		}
		if result.Succeeded+result.Failed >= cs.MaxPages {
			result.Skipped++
			break
		}

		links, kind, err := c.scrape.Run(ctx, spec, entry.url, entry.parentURL, entry.depth)
		if err != nil || kind != "" {
			result.Failed++
			if kind != "" {
				result.Kinds = append(result.Kinds, kind)
			}
			if err != nil {
				result.Errors = append(result.Errors, fault.FormatFailure(entry.url, kind, err))
			}
			continue
		}
		result.Succeeded++

		frontier = enqueueLinks(frontier, entry, links, cs)
	}

	return result, nil
}

func crawlDeadline(cs *jobspec.CrawlSource) time.Time {
	if cs.MaxDuration == "" {
		return time.Time{}
	}
	d, err := time.ParseDuration(cs.MaxDuration)
	if err != nil {
		return time.Time{}
	}
	return time.Now().Add(d)
}

func (c *Crawl) seedFrontier(ctx context.Context, cs *jobspec.CrawlSource) []frontierEntry {
	var frontier []frontierEntry
	for _, seed := range cs.Seeds {
		origin := hostOf(seed)
		frontier = append(frontier, frontierEntry{url: seed, depth: 0, origin: origin})
		if cs.Sitemap && c.sitemap != nil {
			urls, err := c.sitemap.URLs(ctx, seed)
			if err == nil {
				for _, u := range urls {
					frontier = append(frontier, frontierEntry{url: u, depth: 0, origin: origin})
				}
			}
		}
	}
	return frontier
}

func (c *Crawl) shouldSkip(ctx context.Context, entry frontierEntry, cs *jobspec.CrawlSource) bool {
	if entry.parentURL == "" {
		return false
	}
	if !inScope(entry.url, entry.origin, cs.Scope) {
		return true
	}
	if !matchesIncludeExclude(entry.url, cs.Include, cs.Exclude) {
		return true
	}
	if cs.Robots == "respect" && c.robots != nil {
		if allowed, err := c.robots.Allowed(ctx, entry.url); err == nil && !allowed {
			return true
		}
	}
	return false
}

func enqueueLinks(frontier []frontierEntry, entry frontierEntry, links []string, cs *jobspec.CrawlSource) []frontierEntry {
	if entry.depth >= cs.Depth {
		return frontier
	}
	for _, link := range links {
		frontier = append(frontier, frontierEntry{url: link, parentURL: entry.url, depth: entry.depth + 1, origin: entry.origin})
	}
	return frontier
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func inScope(candidateURL, origin, scope string) bool {
	if scope == "any" {
		return true
	}
	host := hostOf(candidateURL)
	if scope == "same-domain" {
		return host == origin || strings.HasSuffix(host, "."+origin)
	}
	return host == origin
}

func matchesIncludeExclude(candidateURL string, include, exclude []string) bool {
	for _, pattern := range exclude {
		if ok, _ := doublestar.Match(pattern, candidateURL); ok {
			return false
		}
	}
	if len(include) == 0 {
		return true
	}
	for _, pattern := range include {
		if ok, _ := doublestar.Match(pattern, candidateURL); ok {
			return true
		}
	}
	return false
}
