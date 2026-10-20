package core

import (
	"context"
	"io"
	"net/http"

	"github.com/Decodo/pobear/pkg/credentials"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/pkg/provider"
	"github.com/Decodo/pobear/providers"
	"github.com/Decodo/pobear/services/core/internal/fetch"
	"github.com/Decodo/pobear/services/core/internal/fetch/tiers"
	"github.com/Decodo/pobear/services/core/internal/job"
	"github.com/Decodo/pobear/services/core/internal/job/workers"
	"github.com/Decodo/pobear/services/core/internal/job/workers/crawl"
	"github.com/Decodo/pobear/services/core/internal/sink"
	"github.com/Decodo/pobear/services/core/internal/sink/sinks"
	"github.com/Decodo/pobear/services/core/internal/stage"
	"github.com/Decodo/pobear/services/core/internal/stage/extractors"
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
	manifests, err := provider.LoadMerged(providers.FS, e.providersDir)
	if err != nil {
		return job.Result{}, err
	}

	fetcher, err := e.createFetcher(manifests, spec)
	if err != nil {
		return job.Result{}, err
	}
	stager := e.createStager()
	sinker := sink.NewEmptyCounter(e.createSinker(spec), len(spec.Stages) > 0)

	scrapeWorker := workers.NewScrape(fetcher, stager, sinker, workers.HTMLLinks{})
	robotsChecker := crawl.NewHTTPRobotsChecker(http.DefaultClient)
	sitemapFetcher := crawl.NewHTTPSitemapFetcher(http.DefaultClient)
	crawlWorker := crawlAdapter{crawl: crawl.NewCrawl(scrapeWorker, robotsChecker, sitemapFetcher)}
	executor := job.NewExecutor(scrapeWorker, crawlWorker, sinker)

	result, runErr := executor.Run(ctx, spec)
	result.Empty = sinker.Empty()
	return result, runErr
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
	return a.crawl.Run(ctx, spec)
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

func buildFetchClients(fetch jobspec.Fetch) (localClient, proxyClient *http.Client, err error) {
	localClient = http.DefaultClient
	if fetch.Profile != "" {
		localClient, err = tiers.NewFingerprintedClient(fetch.Profile, "")
		if err != nil {
			return nil, nil, err
		}
	}

	proxyClient = http.DefaultClient
	if (fetch.Proxy != "" || fetch.Use == "proxy") && fetch.Profile != "" {
		proxyClient, err = tiers.NewFingerprintedClient(fetch.Profile, fetch.Proxy)
		if err != nil {
			return nil, nil, err
		}
	}
	return localClient, proxyClient, nil
}

func (e *liveExecutor) createFetcher(manifests []*provider.Manifest, spec *jobspec.Spec) (*fetch.Fetcher, error) {
	localClient, proxyClient, err := buildFetchClients(spec.Fetch)
	if err != nil {
		return nil, err
	}

	tierMap := map[string]fetch.Tier{
		"local": &tiers.Local{Client: localClient},
		"proxy": &tiers.Proxy{Client: proxyClient},
		"provider": &tiers.Provider{
			Client:          http.DefaultClient,
			Manifests:       manifestLookup(manifests),
			Credentials:     credentialsLoader{},
			CredentialsPath: e.credentialsPath,
		},
	}
	throttle := fetch.NewThrottle(spec.Limits.RPS)
	fetcher := fetch.NewFetcher(tierMap, spec.Limits.Concurrency, spec.Limits.Retries, fetch.Backoff, throttle)
	return fetcher, nil
}
