package workers

import (
	"context"
	"errors"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/fetch/tiers"
	"github.com/pobear/pobear/services/core/internal/sink/sinks"
)

type Fetcher interface {
	Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error)
}

type Stager interface {
	Run(ctx context.Context, body []byte, cfgs []jobspec.Stage) (data map[string]any, warnings []string, err error)
}

type Sinker interface {
	Write(ctx context.Context, item sinks.Item) error
}

type LinkExtractor interface {
	Extract(body []byte) []string
}

var ErrNotImplemented = errors.New("workers: not implemented")

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
	_ = ctx
	_ = spec
	_ = url
	_ = parentURL
	_ = depth
	return nil, "", ErrNotImplemented
}
