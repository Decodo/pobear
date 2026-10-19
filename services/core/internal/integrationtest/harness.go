package integrationtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/internal/testutil"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core"
	"github.com/pobear/pobear/services/core/internal/fetch"
	"github.com/pobear/pobear/services/core/internal/fetch/tiers"
	"github.com/pobear/pobear/services/core/internal/job"
	"github.com/pobear/pobear/services/core/internal/job/workers"
	"github.com/pobear/pobear/services/core/internal/job/workers/crawl"
	"github.com/pobear/pobear/services/core/internal/sink"
	"github.com/pobear/pobear/services/core/internal/sink/sinks"
	"github.com/pobear/pobear/services/core/internal/stage"
	"github.com/pobear/pobear/services/core/internal/stage/extractors"
)

type wantItem struct {
	URL       *string        `yaml:"url"`
	FinalURL  *string        `yaml:"finalUrl"`
	ParentURL *string        `yaml:"parentUrl"`
	Status    *int           `yaml:"status"`
	Depth     *int           `yaml:"depth"`
	Data      map[string]any `yaml:"data"`
}

type wantResult struct {
	Succeeded *int `yaml:"succeeded"`
	Failed    *int `yaml:"failed"`
	Skipped   *int `yaml:"skipped"`
}

type want struct {
	Items  []wantItem  `yaml:"items"`
	Result *wantResult `yaml:"result"`
}

func Run(t *testing.T, dir string) {
	t.Helper()

	srv := testutil.NewFixtureServer(filepath.Join(dir, "fixtures"))
	defer srv.Close()

	specBytes, err := os.ReadFile(filepath.Join(dir, "spec.yaml")) //nolint:gosec // dir comes from a fixed, repo-local testdata walk
	if err != nil {
		t.Fatalf("reading spec.yaml: %v", err)
	}
	specBytes = []byte(strings.ReplaceAll(string(specBytes), "{{base}}", srv.URL))

	wantBytes, err := os.ReadFile(filepath.Join(dir, "want.yaml")) //nolint:gosec // dir comes from a fixed, repo-local testdata walk
	if err != nil {
		t.Fatalf("reading want.yaml: %v", err)
	}
	wantBytes = []byte(strings.ReplaceAll(string(wantBytes), "{{base}}", srv.URL))
	var w want
	dec := yaml.NewDecoder(bytes.NewReader(wantBytes))
	dec.KnownFields(true)
	if err := dec.Decode(&w); err != nil {
		t.Fatalf("parsing want.yaml: %v", err)
	}

	ndjsonPath := filepath.Join(t.TempDir(), "out.ndjson")
	svc := buildService(srv.Client(), ndjsonPath)

	result, _, err := svc.Run(context.Background(), specBytes, jobspec.ModePlain)
	if err != nil {
		t.Fatalf("Service.Run: %v", err)
	}

	got, err := readNDJSONItems(ndjsonPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	assertItems(t, got, w.Items)
	assertResult(t, result, w.Result)
}

func buildService(client *http.Client, ndjsonPath string) *core.Service {
	tierMap := map[string]fetch.Tier{"local": &tiers.Local{Client: client}}
	fetcher := fetch.NewFetcher(tierMap, 8, 3, fetch.Backoff, nil)

	extractorMap := map[string]stage.Extractor{
		"raw":      extractors.Raw{},
		"markdown": extractors.Markdown{},
		"css":      extractors.CSS{},
	}
	stager := stage.NewStager(extractorMap)

	sinkMap := map[string]sink.Sink{"ndjson": &sinks.NDJSON{Path: ndjsonPath}}
	sinker := sink.NewSinker(sinkMap, "ndjson")

	scrape := workers.NewScrape(fetcher, stager, sinker, workers.HTMLLinks{})
	robotsChecker := crawl.NewHTTPRobotsChecker(client)
	sitemapFetcher := crawl.NewHTTPSitemapFetcher(client)
	crawlWorker := crawlAdapter{crawl: crawl.NewCrawl(scrape, robotsChecker, sitemapFetcher)}
	executor := job.NewExecutor(scrape, crawlWorker, sinker)
	parser := job.NewParser(jobspecParser{})

	return core.NewService(parser, executor)
}

type jobspecParser struct{}

func (jobspecParser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error) {
	return jobspec.Parse(data, mode)
}

type crawlAdapter struct {
	crawl *crawl.Crawl
}

func (a crawlAdapter) Run(ctx context.Context, spec *jobspec.Spec) (job.Result, error) {
	result, err := a.crawl.Run(ctx, spec)
	return job.Result{
		Succeeded: result.Succeeded,
		Failed:    result.Failed,
		Skipped:   result.Skipped,
		Kinds:     result.Kinds,
	}, err
}

func readNDJSONItems(path string) ([]sinks.Item, error) {
	f, err := os.Open(path) //nolint:gosec // path is this test run's own t.TempDir() file
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var items []sinks.Item
	dec := json.NewDecoder(f)
	for {
		var item sinks.Item
		if err := dec.Decode(&item); err != nil {
			if errors.Is(err, io.EOF) {
				return items, nil
			}
			return nil, err
		}
		items = append(items, item)
	}
}

func assertItems(t *testing.T, got []sinks.Item, want []wantItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d (got: %+v)", len(got), len(want), got)
	}

	byURL := make(map[string]sinks.Item, len(got))
	for _, g := range got {
		byURL[g.URL] = g
	}

	for i, w := range want {
		if w.URL == nil {
			t.Fatalf("want.yaml item %d has no url -- required to match against got items, since dispatch order isn't guaranteed", i)
		}
		g, ok := byURL[*w.URL]
		if !ok {
			t.Errorf("no got item with url %q (got: %+v)", *w.URL, got)
			continue
		}
		if w.FinalURL != nil && g.FinalURL != *w.FinalURL {
			t.Errorf("item %q: FinalURL = %q, want %q", *w.URL, g.FinalURL, *w.FinalURL)
		}
		if w.ParentURL != nil && g.ParentURL != *w.ParentURL {
			t.Errorf("item %q: ParentURL = %q, want %q", *w.URL, g.ParentURL, *w.ParentURL)
		}
		if w.Status != nil && g.Status != *w.Status {
			t.Errorf("item %q: Status = %d, want %d", *w.URL, g.Status, *w.Status)
		}
		if w.Depth != nil && g.Depth != *w.Depth {
			t.Errorf("item %q: Depth = %d, want %d", *w.URL, g.Depth, *w.Depth)
		}
		for key, wantVal := range w.Data {
			gotVal, ok := g.Data[key]
			if !ok {
				t.Errorf("item %q: data[%q] missing, want %+v", *w.URL, key, wantVal)
				continue
			}
			if !reflect.DeepEqual(gotVal, wantVal) {
				t.Errorf("item %q: data[%q] = %+v, want %+v", *w.URL, key, gotVal, wantVal)
			}
		}
	}
}

func assertResult(t *testing.T, got job.Result, want *wantResult) {
	t.Helper()
	if want == nil {
		return
	}
	if want.Succeeded != nil && got.Succeeded != *want.Succeeded {
		t.Errorf("Result.Succeeded = %d, want %d", got.Succeeded, *want.Succeeded)
	}
	if want.Failed != nil && got.Failed != *want.Failed {
		t.Errorf("Result.Failed = %d, want %d", got.Failed, *want.Failed)
	}
	if want.Skipped != nil && got.Skipped != *want.Skipped {
		t.Errorf("Result.Skipped = %d, want %d", got.Skipped, *want.Skipped)
	}
}
