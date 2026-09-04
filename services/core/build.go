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
	fetcher := fetch.NewFetcher(tierMap, spec.Limits.Concurrency, spec.Limits.Retries, fetch.Backoff)

	extractorMap := map[string]stage.Extractor{
		"raw":      extractors.Raw{},
		"markdown": extractors.Markdown{},
		"css":      extractors.CSS{},
	}
	stager := stage.NewStager(extractorMap)

	sinkMap := map[string]sink.Sink{
		"stdout": &sinks.Stdout{Out: e.stdout},
		"ndjson": &sinks.NDJSON{Path: spec.Sink.Path},
		"json":   &sinks.JSON{Path: spec.Sink.Path},
		"yaml":   &sinks.YAML{Path: spec.Sink.Path},
	}
	sinker := sink.NewSinker(sinkMap, spec.Sink.Use)

	scrapeWorker := workers.NewScrape(fetcher, stager, sinker, noLinks{})
	crawlWorker := crawlAdapter{crawl: crawl.NewCrawl(scrapeWorker, nil, nil)}
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

type noLinks struct{}

func (noLinks) Extract(_ []byte) []string { return nil }

type crawlAdapter struct{ crawl *crawl.Crawl }

func (a crawlAdapter) Run(ctx context.Context, spec *jobspec.Spec) (job.Result, error) {
	result, err := a.crawl.Run(ctx, spec)
	return job.Result{
		Succeeded: result.Succeeded,
		Failed:    result.Failed,
		Skipped:   result.Skipped,
		Kinds:     result.Kinds,
	}, err
}
