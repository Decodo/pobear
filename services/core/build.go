package core

import (
	"context"
	"io"
	"net/http"

	"github.com/pobear/pobear/pkg/credentials"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/pkg/provider"
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

func New(providersDir, credentialsPath string, stdout io.Writer) *Service {
	parser := job.NewParser(jobspecParser{})
	executor := &liveExecutor{providersDir: providersDir, credentialsPath: credentialsPath, stdout: stdout}
	return NewService(parser, executor)
}

type jobspecParser struct{}

func (jobspecParser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error) {
	return jobspec.Parse(data, mode)
}

type liveExecutor struct {
	providersDir    string
	credentialsPath string
	stdout          io.Writer
}

func (e *liveExecutor) Run(ctx context.Context, spec *jobspec.Spec) (job.Result, error) {
	manifests, err := provider.LoadDir(e.providersDir)
	if err != nil {
		return job.Result{}, err
	}

	fetcher := e.createFetcher(manifests, spec)
	stager := e.createStager()
	sinker := e.createSinker(spec)

	scrapeWorker := workers.NewScrape(fetcher, stager, sinker, workers.HTMLLinks{})
	robotsChecker := crawl.NewHTTPRobotsChecker(http.DefaultClient)
	sitemapFetcher := crawl.NewHTTPSitemapFetcher(http.DefaultClient)
	crawlWorker := crawlAdapter{crawl: crawl.NewCrawl(scrapeWorker, robotsChecker, sitemapFetcher)}
	executor := job.NewExecutor(scrapeWorker, crawlWorker, sinker)

	return executor.Run(ctx, spec)
}

type manifestLookup []*provider.Manifest

func (m manifestLookup) Lookup(name string) (*provider.Manifest, bool) {
	for _, manifest := range m {
		if manifest.Name == name {
			return manifest, true
		}
	}
	return nil, false
}

type credentialsLoader struct{}

func (credentialsLoader) Load(path, providerName, credentialName string) (string, bool, error) {
	return credentials.Load(path, providerName, credentialName)
}

type crawlAdapter struct{ crawl *crawl.Crawl }

func (a crawlAdapter) Run(ctx context.Context, spec *jobspec.Spec) (job.Result, error) {
	result, err := a.crawl.Run(ctx, spec)
	return job.Result{
		Succeeded:   result.Succeeded,
		Failed:      result.Failed,
		Skipped:     result.Skipped,
		Kinds:       result.Kinds,
		Errors:      result.Errors,
		SkippedURLs: result.SkippedURLs,
	}, err
}

func (e *liveExecutor) createSinker(spec *jobspec.Spec) *sink.Sinker {
	sinkMap := map[string]sink.Sink{
		"stdout": &sinks.Stdout{Out: e.stdout},
		"ndjson": &sinks.NDJSON{Path: spec.Sink.Path},
		"json":   &sinks.JSON{Path: spec.Sink.Path},
		"yaml":   &sinks.YAML{Path: spec.Sink.Path},
	}
	sinker := sink.NewSinker(sinkMap, spec.Sink.Use)
	return sinker
}

func (*liveExecutor) createStager() *stage.Stager {
	extractorMap := map[string]stage.Extractor{
		"raw":      extractors.Raw{},
		"markdown": extractors.Markdown{},
		"css":      extractors.CSS{},
	}
	stager := stage.NewStager(extractorMap)
	return stager
}

func (e *liveExecutor) createFetcher(manifests []*provider.Manifest, spec *jobspec.Spec) *fetch.Fetcher {
	tierMap := map[string]fetch.Tier{
		"local": &tiers.Local{Client: http.DefaultClient},
		"proxy": &tiers.Proxy{Client: http.DefaultClient},
		"provider": &tiers.Provider{
			Client:          http.DefaultClient,
			Manifests:       manifestLookup(manifests),
			Credentials:     credentialsLoader{},
			CredentialsPath: e.credentialsPath,
		},
	}
	throttle := fetch.NewThrottle(spec.Limits.RPS)
	fetcher := fetch.NewFetcher(tierMap, spec.Limits.Concurrency, spec.Limits.Retries, fetch.Backoff, throttle)
	return fetcher
}
