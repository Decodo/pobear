package job

import (
	"context"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

type ScrapeWorker interface {
	Run(ctx context.Context, spec *jobspec.Spec, url, parentURL string, depth int) (links []string, kind fault.Kind, err error)
}

type CrawlWorker interface {
	Run(ctx context.Context, spec *jobspec.Spec) (Result, error)
}

type SinkCloser interface {
	Close() error
}

type Result struct {
	Succeeded int
	Failed    int
	Skipped   int
	Kinds     []fault.Kind
}

type Executor struct {
	scrape ScrapeWorker
	crawl  CrawlWorker
	sink   SinkCloser
}

func NewExecutor(scrape ScrapeWorker, crawl CrawlWorker, sink SinkCloser) *Executor {
	return &Executor{scrape: scrape, crawl: crawl, sink: sink}
}

func (e *Executor) Run(ctx context.Context, spec *jobspec.Spec) (Result, error) {
	_ = ctx
	_ = spec
	return Result{}, ErrNotImplemented
}
