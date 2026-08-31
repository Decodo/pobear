package crawl

import (
	"context"
	"errors"

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
}

var ErrNotImplemented = errors.New("crawl: not implemented")

type Crawl struct {
	scrape  ScrapeWorker
	robots  RobotsChecker
	sitemap SitemapFetcher
}

func NewCrawl(scrape ScrapeWorker, robots RobotsChecker, sitemap SitemapFetcher) *Crawl {
	return &Crawl{scrape: scrape, robots: robots, sitemap: sitemap}
}

func (c *Crawl) Run(ctx context.Context, spec *jobspec.Spec) (Result, error) {
	_ = ctx
	_ = spec
	return Result{}, ErrNotImplemented
}
