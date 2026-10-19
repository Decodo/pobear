package workers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/fetch/tiers"
	"github.com/Decodo/pobear/services/core/internal/job/workers"
	"github.com/Decodo/pobear/services/core/internal/sink/sinks"
)

type fakeFetcher struct {
	result *tiers.Result
	fault  fault.Kind
	err    error
	gotURL string
}

func (f *fakeFetcher) Fetch(_ context.Context, _ jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error) {
	f.gotURL = url
	return f.result, f.fault, f.err
}

type fakeStager struct {
	data    map[string]any
	err     error
	gotBody []byte
	calls   int
}

func (f *fakeStager) Run(_ context.Context, body []byte, _ []jobspec.Stage) (map[string]any, error) {
	f.gotBody = body
	f.calls++
	return f.data, f.err
}

type fakeSinker struct {
	err     error
	gotItem sinks.Item
	calls   int
}

func (f *fakeSinker) Write(_ context.Context, item sinks.Item) error {
	f.gotItem = item
	f.calls++
	return f.err
}

type fakeLinkExtractor struct {
	links      []string
	gotBody    []byte
	gotBaseURL string
	calls      int
}

func (f *fakeLinkExtractor) Extract(body []byte, baseURL string) []string {
	f.gotBody = body
	f.gotBaseURL = baseURL
	f.calls++
	return f.links
}

func TestScrapeSuccessWritesMergedItem(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Body: []byte("<h1>hi</h1>"), FinalURL: "https://a.test/"}}
	stager := &fakeStager{data: map[string]any{"raw": "<h1>hi</h1>"}}
	sinker := &fakeSinker{}
	links := &fakeLinkExtractor{}

	s := workers.NewScrape(fetcher, stager, sinker, links)
	spec := &jobspec.Spec{Fetch: jobspec.Fetch{Use: "local"}, Stages: []jobspec.Stage{{Use: "raw"}}}
	_, _, kind, err := s.Run(context.Background(), spec, "https://a.test", "", 0, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if kind != "" {
		t.Fatalf("expected empty fault.Kind on success, got %q", kind)
	}
	if sinker.calls != 1 {
		t.Fatalf("expected exactly 1 sink write, got %d", sinker.calls)
	}
	got := sinker.gotItem
	if got.URL != "https://a.test" || got.FinalURL != "https://a.test/" {
		t.Errorf("URL/FinalURL = %q/%q, want %q/%q", got.URL, got.FinalURL, "https://a.test", "https://a.test/")
	}
	if got.Status != 200 {
		t.Errorf("Status = %d, want 200", got.Status)
	}
	if got.Body != "<h1>hi</h1>" {
		t.Errorf("Body = %q, want literal string of the fetched bytes, not base64", got.Body)
	}
	if got.ParentURL != "" || got.Depth != 0 {
		t.Errorf("ParentURL/Depth = %q/%d, want empty/0 for a non-crawl-discovered item", got.ParentURL, got.Depth)
	}
	if got.Data["raw"] != "<h1>hi</h1>" {
		t.Errorf("Data = %+v, want the stager's merged data map", got.Data)
	}
}

func TestScrapeSuccessSetsParentURLAndDepthForCrawlDiscoveredItem(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Body: []byte("body")}}
	stager := &fakeStager{data: map[string]any{}}
	sinker := &fakeSinker{}
	links := &fakeLinkExtractor{}

	s := workers.NewScrape(fetcher, stager, sinker, links)
	spec := &jobspec.Spec{Fetch: jobspec.Fetch{Use: "local"}}
	_, _, _, err := s.Run(context.Background(), spec, "https://a.test/child", "https://a.test", 2, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sinker.gotItem.ParentURL != "https://a.test" {
		t.Errorf("ParentURL = %q, want %q", sinker.gotItem.ParentURL, "https://a.test")
	}
	if sinker.gotItem.Depth != 2 {
		t.Errorf("Depth = %d, want 2", sinker.gotItem.Depth)
	}
}

func TestScrapePassesFetchedBodyToStager(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Body: []byte("the body")}}
	stager := &fakeStager{data: map[string]any{}}
	sinker := &fakeSinker{}
	links := &fakeLinkExtractor{}

	s := workers.NewScrape(fetcher, stager, sinker, links)
	_, _, _, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test", "", 0, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(stager.gotBody) != "the body" {
		t.Fatalf("stager received body %q, want %q", stager.gotBody, "the body")
	}
}

func TestScrapeSuccessReturnsExtractedLinks(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Body: []byte("<a href=b>b</a>"), FinalURL: "https://a.test/final"}}
	stager := &fakeStager{data: map[string]any{}}
	sinker := &fakeSinker{}
	links := &fakeLinkExtractor{links: []string{"https://a.test/b"}}

	s := workers.NewScrape(fetcher, stager, sinker, links)
	got, _, _, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test", "", 0, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 1 || got[0] != "https://a.test/b" {
		t.Fatalf("got links %v, want [https://a.test/b]", got)
	}
	if string(links.gotBody) != "<a href=b>b</a>" {
		t.Fatalf("LinkExtractor received body %q, want the fetched body", links.gotBody)
	}
	if links.gotBaseURL != "https://a.test/final" {
		t.Fatalf("LinkExtractor received baseURL %q, want the fetch's FinalURL %q (relative hrefs must resolve against where the page actually ended up, not the requested URL)", links.gotBaseURL, "https://a.test/final")
	}
}

func TestScrapeFetchErrorNeverCallsStagerSinkerOrLinkExtractor(t *testing.T) {
	wantErr := errors.New("connection refused")
	fetcher := &fakeFetcher{err: wantErr, fault: fault.Network}
	stager := &fakeStager{err: errors.New("stager should never run")}
	sinker := &fakeSinker{err: errors.New("sinker should never run")}
	links := &fakeLinkExtractor{}

	s := workers.NewScrape(fetcher, stager, sinker, links)
	gotLinks, _, kind, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test", "", 0, nil)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the fetch error, got %v", err)
	}
	if kind != fault.Network {
		t.Fatalf("expected fault.Network, got %q", kind)
	}
	if gotLinks != nil {
		t.Fatalf("expected no links on a fetch error, got %v", gotLinks)
	}
	if stager.calls != 0 {
		t.Fatalf("expected the stager never called after a fetch error, got %d calls", stager.calls)
	}
	if sinker.calls != 0 {
		t.Fatalf("expected the sinker never called after a fetch error, got %d calls", sinker.calls)
	}
	if links.calls != 0 {
		t.Fatalf("expected the LinkExtractor never called after a fetch error, got %d calls", links.calls)
	}
}

func TestScrapeStageErrorNeverCallsSinker(t *testing.T) {
	wantErr := errors.New("invalid selector")
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Body: []byte("body")}}
	stager := &fakeStager{err: wantErr}
	sinker := &fakeSinker{err: errors.New("sinker should never run")}
	links := &fakeLinkExtractor{}

	s := workers.NewScrape(fetcher, stager, sinker, links)
	_, _, kind, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test", "", 0, nil)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the stage error, got %v", err)
	}
	if kind != "" {
		t.Fatalf("expected an empty fault.Kind for a stage error (unclassified, no matching fault.Kind exists for it), got %q", kind)
	}
	if sinker.calls != 0 {
		t.Fatalf("expected the sinker never called after a stage error, got %d calls", sinker.calls)
	}
}

func TestScrapeSinkErrorIsReturned(t *testing.T) {
	wantErr := errors.New("disk full")
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Body: []byte("body")}}
	stager := &fakeStager{data: map[string]any{}}
	sinker := &fakeSinker{err: wantErr}
	links := &fakeLinkExtractor{}

	s := workers.NewScrape(fetcher, stager, sinker, links)
	_, _, kind, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test", "", 0, nil)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the sink error, got %v", err)
	}
	if kind != "" {
		t.Fatalf("expected an empty fault.Kind for a sink error (unclassified), got %q", kind)
	}
}

func TestScrapeGateBlocksARedirectedResultBeforeItIsEmitted(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{
		Status: 200, Body: []byte("<html><h1>secret</h1></html>"),
		FinalURL: "https://a.test/private/hidden",
	}}
	stager := &fakeStager{}
	sinker := &fakeSinker{}
	links := &fakeLinkExtractor{links: []string{"https://a.test/more"}}
	s := workers.NewScrape(fetcher, stager, sinker, links)

	gate := func(finalURL string) (string, bool) {
		return "disallowed by robots.txt", finalURL == "https://a.test/private/hidden"
	}
	gotLinks, blockedReason, kind, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test/toprivate", "", 0, gate)

	if err != nil || kind != "" {
		t.Fatalf("expected no error, got kind=%q err=%v", kind, err)
	}
	if blockedReason != "disallowed by robots.txt" {
		t.Fatalf("blockedReason = %q, want the gate's reason", blockedReason)
	}
	if sinker.calls != 0 {
		t.Error("a blocked result was written to the sink")
	}
	if len(gotLinks) != 0 {
		t.Errorf("a blocked result returned %d links to follow, want 0", len(gotLinks))
	}
}

func TestScrapeGateIsNotConsultedWhenThereWasNoRedirect(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{
		Status: 200, Body: []byte("<html><h1>ok</h1></html>"),
		FinalURL: "https://a.test/page",
	}}
	sinker := &fakeSinker{}
	s := workers.NewScrape(fetcher, &fakeStager{}, sinker, &fakeLinkExtractor{})

	called := false
	gate := func(string) (string, bool) { called = true; return "", true }
	_, blockedReason, _, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test/page", "", 0, gate)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if called {
		t.Error("the gate was consulted for a URL that was never redirected")
	}
	if blockedReason != "" {
		t.Errorf("blockedReason = %q, want empty", blockedReason)
	}
	if sinker.calls != 1 {
		t.Errorf("sink writes = %d, want 1", sinker.calls)
	}
}

func TestScrapeRunCopiesResultTierIntoTheSinkItem(t *testing.T) {
	fetcher := &fakeFetcher{result: &tiers.Result{Status: 200, Tier: "proxy"}}
	sinker := &fakeSinker{}
	s := workers.NewScrape(fetcher, &fakeStager{data: map[string]any{}}, sinker, &fakeLinkExtractor{})

	_, _, kind, err := s.Run(context.Background(), &jobspec.Spec{}, "https://a.test", "", 0, nil)

	if err != nil || kind != "" {
		t.Fatalf("Run() kind = %q, err = %v, want a clean success", kind, err)
	}
	if sinker.gotItem.Tier != "proxy" {
		t.Fatalf("sink item Tier = %q, want %q", sinker.gotItem.Tier, "proxy")
	}
}
