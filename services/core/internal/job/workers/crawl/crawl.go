package crawl

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobresult"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/fetch/tiers"
	"github.com/pobear/pobear/services/core/internal/job/pool"
	"github.com/pobear/pobear/services/core/internal/job/workers"
)

type ScrapeWorker interface {
	Run(ctx context.Context, spec *jobspec.Spec, url, parentURL string, depth int, gate workers.EmitGate) (links []string, blockedReason string, kind fault.Kind, err error)
}

type RobotsChecker interface {
	Allowed(ctx context.Context, url string) (bool, error)
}

type SitemapFetcher interface {
	URLs(ctx context.Context, origin string) ([]string, error)
}

type Result = jobresult.Result

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
	isSeed    bool
}

func (c *Crawl) Run(ctx context.Context, spec *jobspec.Spec) (Result, error) {
	cs := spec.Source.Crawl
	deadline := crawlDeadline(cs)
	dd := newDedupe()

	p := pool.New(spec.Limits.Concurrency)
	var mu sync.Mutex
	var result Result
	dispatched := 0

	var visit func(entry frontierEntry)
	visit = func(entry frontierEntry) {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return
		}

		if reason, skip := c.shouldSkip(ctx, entry, cs); skip {
			mu.Lock()
			result.Skipped++
			result.SkippedURLs = append(result.SkippedURLs, fault.FormatFailure(entry.url, "", errors.New(reason)))
			result.Items = append(result.Items, jobresult.Item{URL: entry.url, Outcome: jobresult.Skipped, Err: reason})
			mu.Unlock()
			return
		}

		mu.Lock()
		overBudget := dispatched >= cs.MaxPages
		if overBudget {
			const budgetReason = "over --max-pages budget"
			result.Skipped++
			result.SkippedURLs = append(result.SkippedURLs, fault.FormatFailure(entry.url, "", errors.New(budgetReason)))
			result.Items = append(result.Items, jobresult.Item{URL: entry.url, Outcome: jobresult.Skipped, Err: budgetReason})
		} else {
			dispatched++
		}
		mu.Unlock()
		if overBudget {
			return
		}

		var gate workers.EmitGate
		if !entry.isSeed {
			gate = func(finalURL string) (string, bool) {
				return c.shouldSkip(ctx, frontierEntry{url: finalURL, origin: entry.origin, depth: entry.depth}, cs)
			}
		}

		fetchCtx := ctx
		if gate != nil {
			fetchCtx = tiers.WithRedirectPolicy(ctx, func(target string) bool {
				_, skip := c.shouldSkip(ctx, frontierEntry{url: target, origin: entry.origin, depth: entry.depth}, cs)
				return !skip
			})
		}

		links, blockedReason, kind, err := c.scrape.Run(fetchCtx, spec, entry.url, entry.parentURL, entry.depth, gate)
		if err != nil || kind != "" {
			mu.Lock()
			result.Failed++
			errMsg := ""
			if kind != "" {
				result.Kinds = append(result.Kinds, kind)
			}
			if err != nil {
				errMsg = err.Error()
				result.Errors = append(result.Errors, fault.FormatFailure(entry.url, kind, err))
			}
			result.Items = append(result.Items, jobresult.Item{URL: entry.url, Outcome: jobresult.Failed, Kind: kind, Err: errMsg})
			mu.Unlock()
			return
		}

		if blockedReason != "" {
			mu.Lock()
			result.Skipped++
			result.SkippedURLs = append(result.SkippedURLs, fault.FormatFailure(entry.url, "", errors.New(blockedReason)))
			mu.Unlock()
			return
		}

		mu.Lock()
		result.Succeeded++
		result.Items = append(result.Items, jobresult.Item{URL: entry.url, Outcome: jobresult.Succeeded})
		mu.Unlock()

		if entry.depth >= cs.Depth {
			return
		}
		for _, link := range links {
			if !dd.claim(link) {
				continue
			}
			child := frontierEntry{url: link, parentURL: entry.url, depth: entry.depth + 1, origin: entry.origin}
			p.Submit(func() { visit(child) })
		}
	}

	c.seedFrontier(ctx, cs, dd, func(entry frontierEntry) {
		p.Submit(func() { visit(entry) })
	})
	p.Wait()

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

func (c *Crawl) seedFrontier(ctx context.Context, cs *jobspec.CrawlSource, dd *dedupe, dispatch func(frontierEntry)) {
	for _, seed := range cs.Seeds {
		if !dd.claim(seed) {
			continue
		}
		origin := hostOf(seed)
		dispatch(frontierEntry{url: seed, depth: 0, origin: origin, isSeed: true})
		if cs.Sitemap && c.sitemap != nil {
			urls, err := c.sitemap.URLs(ctx, seed)
			if err == nil {
				for _, u := range urls {
					if !dd.claim(u) {
						continue
					}
					dispatch(frontierEntry{url: u, depth: 0, origin: origin})
				}
			}
		}
	}
}

func (c *Crawl) shouldSkip(ctx context.Context, entry frontierEntry, cs *jobspec.CrawlSource) (reason string, skip bool) {
	if entry.isSeed {
		return "", false
	}
	if !inScope(entry.url, entry.origin, cs.Scope) {
		return "out of scope", true
	}
	if !matchesIncludeExclude(entry.url, cs.Include, cs.Exclude) {
		return "excluded by --include/--exclude", true
	}
	if cs.Robots == "respect" && c.robots != nil {
		if allowed, err := c.robots.Allowed(ctx, entry.url); err == nil && !allowed {
			return "disallowed by robots.txt", true
		}
	}
	return "", false
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
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
