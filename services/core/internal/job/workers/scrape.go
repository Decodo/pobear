package workers

import (
	"context"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/fetch/tiers"
	"github.com/pobear/pobear/services/core/internal/sink/sinks"
)

type Fetcher interface {
	Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error)
}

type Stager interface {
	Run(ctx context.Context, body []byte, cfgs []jobspec.Stage) (data map[string]any, err error)
}

type Sinker interface {
	Write(ctx context.Context, item sinks.Item) error
}

type LinkExtractor interface {
	Extract(body []byte, baseURL string) []string
}

type Scrape struct {
	fetcher Fetcher
	stager  Stager
	sinker  Sinker
	links   LinkExtractor
}

func NewScrape(f Fetcher, st Stager, sk Sinker, le LinkExtractor) *Scrape {
	return &Scrape{fetcher: f, stager: st, sinker: sk, links: le}
}

func (s *Scrape) Run(ctx context.Context, spec *jobspec.Spec, url, parentURL string, depth int) (links []string, kind fault.Kind, err error) {
	result, fetchKind, fetchErr := s.fetcher.Fetch(ctx, spec.Fetch, url)
	if fetchErr != nil || fetchKind != "" {
		return nil, fetchKind, fetchErr
	}

	data, stageErr := s.stager.Run(ctx, result.Body, spec.Stages)
	if stageErr != nil {
		return nil, "", stageErr
	}

	item := sinks.Item{
		URL:       url,
		FinalURL:  result.FinalURL,
		ParentURL: parentURL,
		Status:    result.Status,
		Body:      string(result.Body),
		Depth:     depth,
		Data:      data,
	}
	if err := s.sinker.Write(ctx, item); err != nil {
		return nil, "", err
	}

	return s.links.Extract(result.Body, result.FinalURL), "", nil
}
