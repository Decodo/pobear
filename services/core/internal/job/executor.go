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
	Errors    []string
}

type Executor struct {
	scrape ScrapeWorker
	crawl  CrawlWorker
	sink   SinkCloser
}

func NewExecutor(scrape ScrapeWorker, crawl CrawlWorker, sink SinkCloser) *Executor {
	return &Executor{scrape: scrape, crawl: crawl, sink: sink}
}

func (e *Executor) Run(ctx context.Context, spec *jobspec.Spec) (result Result, err error) {
	defer func() {
		if closeErr := e.sink.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if spec.Source.Crawl != nil {
		result, err = e.crawl.Run(ctx, spec)
		return
	}

	for _, url := range spec.Source.URLs {
		_, kind, scrapeErr := e.scrape.Run(ctx, spec, url, "", 0)
		if scrapeErr == nil && kind == "" {
			result.Succeeded++
			continue
		}
		result.Failed++
		if kind != "" {
			result.Kinds = append(result.Kinds, kind)
		}
		if scrapeErr != nil {
			result.Errors = append(result.Errors, fault.FormatFailure(url, kind, scrapeErr))
		}
	}
	return
}
