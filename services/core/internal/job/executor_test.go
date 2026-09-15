package job_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/job"
	"github.com/pobear/pobear/services/core/internal/job/workers"
)

type fakeScrapeWorker struct {
	mu      sync.Mutex
	results map[string]fakeScrapeResult
	calls   int
	gotURLs []string
}

type fakeScrapeResult struct {
	kind fault.Kind
	err  error
}

func (f *fakeScrapeWorker) Run(_ context.Context, _ *jobspec.Spec, url, _ string, _ int, _ workers.EmitGate) ([]string, string, fault.Kind, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotURLs = append(f.gotURLs, url)
	f.calls++
	r := f.results[url]
	return nil, "", r.kind, r.err
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
	scrape := &fakeScrapeWorker{}
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
	gotURLs := slices.Clone(scrape.gotURLs)
	slices.Sort(gotURLs)
	wantURLs := slices.Clone(want)
	slices.Sort(wantURLs)
	if !slices.Equal(gotURLs, wantURLs) {
		t.Fatalf("dispatched URLs (any order) = %v, want %v", scrape.gotURLs, want)
	}
}

func TestExecutorURLListContinuesPastPerItemFailure(t *testing.T) {
	scrape := &fakeScrapeWorker{results: map[string]fakeScrapeResult{
		"https://b.test": {kind: fault.Network, err: errors.New("connection refused")},
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
	if len(got.Errors) != 1 || !strings.Contains(got.Errors[0], "https://b.test") || !strings.Contains(got.Errors[0], "connection refused") {
		t.Fatalf("Errors = %v, want one entry naming the failed URL and its error", got.Errors)
	}
	if scrape.calls != 3 {
		t.Fatalf("expected all 3 URLs attempted despite the middle failure, got %d calls", scrape.calls)
	}
}

type slowScrapeWorker struct {
	sleep   time.Duration
	current int32
	peak    int32
}

func (f *slowScrapeWorker) Run(_ context.Context, _ *jobspec.Spec, _, _ string, _ int, _ workers.EmitGate) ([]string, string, fault.Kind, error) {
	n := atomic.AddInt32(&f.current, 1)
	for {
		old := atomic.LoadInt32(&f.peak)
		if n <= old || atomic.CompareAndSwapInt32(&f.peak, old, n) {
			break
		}
	}
	time.Sleep(f.sleep)
	atomic.AddInt32(&f.current, -1)
	return nil, "", "", nil
}

func TestExecutorURLListRunsFetchesConcurrentlyUpToTheConfiguredLimit(t *testing.T) {
	const limit = 3
	scrape := &slowScrapeWorker{sleep: 20 * time.Millisecond}
	e := job.NewExecutor(scrape, &fakeCrawlWorker{}, &fakeSinkCloser{})

	urls := make([]string, 12)
	for i := range urls {
		urls[i] = "https://a.test/" + string(rune('a'+i))
	}
	spec := &jobspec.Spec{
		Source: jobspec.Source{URLs: urls},
		Limits: jobspec.Limits{Concurrency: limit},
	}

	start := time.Now()
	got, err := e.Run(context.Background(), spec)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Succeeded != len(urls) {
		t.Fatalf("got Succeeded=%d, want %d", got.Succeeded, len(urls))
	}
	if peak := atomic.LoadInt32(&scrape.peak); peak > limit {
		t.Fatalf("peak concurrent fetches = %d, want <= %d", peak, limit)
	}
	if peak := atomic.LoadInt32(&scrape.peak); peak < 2 {
		t.Fatalf("peak concurrent fetches = %d, want > 1 -- dispatch doesn't look concurrent at all", peak)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("Run took %v, expected roughly ~80-100ms if truly concurrent (bounded by %d in flight)", elapsed, limit)
	}
}

func TestExecutorURLListEmptyKindFailureIsTalliedButNotAddedToKinds(t *testing.T) {
	scrape := &fakeScrapeWorker{results: map[string]fakeScrapeResult{"https://a.test": {err: errors.New("stage failed")}}}
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
	scrape := &fakeScrapeWorker{}
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
	scrape := &fakeScrapeWorker{results: map[string]fakeScrapeResult{"https://a.test": {kind: fault.Timeout, err: errors.New("boom")}}}
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
