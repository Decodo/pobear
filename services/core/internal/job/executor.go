package job

import (
	"context"
	"sync"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/job/pool"
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
	Succeeded   int
	Failed      int
	Skipped     int
	Kinds       []fault.Kind
	Errors      []string
	SkippedURLs []string
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

	p := pool.New(spec.Limits.Concurrency)
	var mu sync.Mutex

	for _, url := range spec.Source.URLs {
		p.Submit(func() {
			_, kind, scrapeErr := e.scrape.Run(ctx, spec, url, "", 0)

			mu.Lock()
			defer mu.Unlock()
			if scrapeErr == nil && kind == "" {
				result.Succeeded++
				return
			}
			result.Failed++
			if kind != "" {
				result.Kinds = append(result.Kinds, kind)
			}
			if scrapeErr != nil {
				result.Errors = append(result.Errors, fault.FormatFailure(url, kind, scrapeErr))
			}
		})
	}
	p.Wait()

	return
}
