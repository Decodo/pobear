package job_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/job"
)

type fakeScrapeWorker struct {
	results []fakeScrapeResult
	calls   int
	gotURLs []string
}

type fakeScrapeResult struct {
	kind fault.Kind
	err  error
}

func (f *fakeScrapeWorker) Run(_ context.Context, _ *jobspec.Spec, url, _ string, _ int) ([]string, fault.Kind, error) {
	f.gotURLs = append(f.gotURLs, url)
	if f.calls >= len(f.results) {
		panic(fmt.Sprintf("fakeScrapeWorker.Run called more times (%d) than results were configured (%d) — likely a dispatch bug", f.calls+1, len(f.results)))
	}
	r := f.results[f.calls]
	f.calls++
	return nil, r.kind, r.err
}

type fakeCrawlWorker struct {
	result job.Result
	err    error
	calls  int
}

func (f *fakeCrawlWorker) Run(_ context.Context, _ *jobspec.Spec) (job.Result, error) {
	f.calls++
	return f.result, f.err
}

type fakeSinkCloser struct {
	err   error
	calls int
}

func (f *fakeSinkCloser) Close() error {
	f.calls++
	return f.err
}

func TestExecutorURLListDispatchesEachURLAndTalliesSuccesses(t *testing.T) {
	scrape := &fakeScrapeWorker{results: []fakeScrapeResult{{}, {}, {}}}
	sink := &fakeSinkCloser{}
	e := job.NewExecutor(scrape, &fakeCrawlWorker{}, sink)

	spec := &jobspec.Spec{Source: jobspec.Source{URLs: []string{"https://a.test", "https://b.test", "https://c.test"}}}
	got, err := e.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != 3 || got.Failed != 0 || got.Skipped != 0 {
		t.Fatalf("got %+v, want Succeeded=3 Failed=0 Skipped=0", got)
	}
	want := []string{"https://a.test", "https://b.test", "https://c.test"}
	if len(scrape.gotURLs) != len(want) {
		t.Fatalf("dispatched %d URLs, want %d", len(scrape.gotURLs), len(want))
	}
	for i, u := range want {
		if scrape.gotURLs[i] != u {
			t.Errorf("URL %d = %q, want %q", i, scrape.gotURLs[i], u)
		}
	}
}

func TestExecutorURLListContinuesPastPerItemFailure(t *testing.T) {
	scrape := &fakeScrapeWorker{results: []fakeScrapeResult{
		{},
		{kind: fault.Network, err: errors.New("connection refused")},
		{},
	}}
	e := job.NewExecutor(scrape, &fakeCrawlWorker{}, &fakeSinkCloser{})

	spec := &jobspec.Spec{Source: jobspec.Source{URLs: []string{"https://a.test", "https://b.test", "https://c.test"}}}
	got, err := e.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected Run itself to succeed even though one item failed, got %v", err)
	}
	if got.Succeeded != 2 || got.Failed != 1 {
		t.Fatalf("got %+v, want Succeeded=2 Failed=1", got)
	}
	if len(got.Kinds) != 1 || got.Kinds[0] != fault.Network {
		t.Fatalf("Kinds = %v, want [fault.Network]", got.Kinds)
	}
	if scrape.calls != 3 {
		t.Fatalf("expected all 3 URLs attempted despite the middle failure, got %d calls", scrape.calls)
	}
}

func TestExecutorURLListEmptyKindFailureIsTalliedButNotAddedToKinds(t *testing.T) {
	scrape := &fakeScrapeWorker{results: []fakeScrapeResult{{err: errors.New("stage failed")}}}
	e := job.NewExecutor(scrape, &fakeCrawlWorker{}, &fakeSinkCloser{})

	spec := &jobspec.Spec{Source: jobspec.Source{URLs: []string{"https://a.test"}}}
	got, err := e.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected Run itself to succeed even though the item failed, got %v", err)
	}
	if got.Failed != 1 {
		t.Fatalf("got Failed=%d, want 1", got.Failed)
	}
	if len(got.Kinds) != 0 {
		t.Fatalf("got Kinds=%v, want empty (an empty fault.Kind must never be appended)", got.Kinds)
	}
}

func TestExecutorCrawlSourceDelegatesToCrawlWorkerOnce(t *testing.T) {
	crawlResult := job.Result{Succeeded: 5, Failed: 1, Skipped: 2, Kinds: []fault.Kind{fault.HTTP}}
	crawl := &fakeCrawlWorker{result: crawlResult}
	scrape := &fakeScrapeWorker{}
	e := job.NewExecutor(scrape, crawl, &fakeSinkCloser{})

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{Seeds: []string{"https://a.test"}}}}
	got, err := e.Run(context.Background(), spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != crawlResult.Succeeded || got.Failed != crawlResult.Failed || got.Skipped != crawlResult.Skipped || !slices.Equal(got.Kinds, crawlResult.Kinds) {
		t.Fatalf("got %+v, want the CrawlWorker's own Result %+v passed straight through", got, crawlResult)
	}
	if crawl.calls != 1 {
		t.Fatalf("expected CrawlWorker.Run called exactly once, got %d", crawl.calls)
	}
	if scrape.calls != 0 {
		t.Fatalf("expected ScrapeWorker never called directly for a crawl source, got %d calls", scrape.calls)
	}
}

func TestExecutorClosesSinkExactlyOnceForURLList(t *testing.T) {
	scrape := &fakeScrapeWorker{results: []fakeScrapeResult{{}, {}}}
	sink := &fakeSinkCloser{}
	e := job.NewExecutor(scrape, &fakeCrawlWorker{}, sink)

	spec := &jobspec.Spec{Source: jobspec.Source{URLs: []string{"https://a.test", "https://b.test"}}}
	if _, err := e.Run(context.Background(), spec); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink.calls != 1 {
		t.Fatalf("expected Close called exactly once, got %d", sink.calls)
	}
}

func TestExecutorClosesSinkExactlyOnceForCrawl(t *testing.T) {
	sink := &fakeSinkCloser{}
	e := job.NewExecutor(&fakeScrapeWorker{}, &fakeCrawlWorker{}, sink)

	spec := &jobspec.Spec{Source: jobspec.Source{Crawl: &jobspec.CrawlSource{Seeds: []string{"https://a.test"}}}}
	if _, err := e.Run(context.Background(), spec); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sink.calls != 1 {
		t.Fatalf("expected Close called exactly once, got %d", sink.calls)
	}
}

func TestExecutorClosesSinkEvenWhenItemsFail(t *testing.T) {
	scrape := &fakeScrapeWorker{results: []fakeScrapeResult{{kind: fault.Timeout, err: errors.New("boom")}}}
	sink := &fakeSinkCloser{}
	e := job.NewExecutor(scrape, &fakeCrawlWorker{}, sink)

	spec := &jobspec.Spec{Source: jobspec.Source{URLs: []string{"https://a.test"}}}
	if _, err := e.Run(context.Background(), spec); err != nil {
		t.Fatalf("expected Run itself to succeed, got %v", err)
	}
	if sink.calls != 1 {
		t.Fatalf("expected Close still called exactly once even though the only item failed, got %d", sink.calls)
	}
}
