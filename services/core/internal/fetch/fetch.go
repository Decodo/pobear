package fetch

import (
	"context"
	"errors"
	"time"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/fetch/tiers"
)

type Tier interface {
	Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error)
}

var ErrUnknownTier = errors.New("fetch: unknown tier")

var ErrNotImplemented = errors.New("fetch: not implemented")

type Fetcher struct {
	tiers       map[string]Tier
	concurrency int
	retries     int
	backoff     func(attempt int) time.Duration
}

func NewFetcher(ts map[string]Tier, concurrency, retries int, backoff func(attempt int) time.Duration) *Fetcher {
	return &Fetcher{tiers: ts, concurrency: concurrency, retries: retries, backoff: backoff}
}

func (f *Fetcher) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error) {
	_ = ctx
	_ = cfg
	_ = url
	return nil, "", ErrNotImplemented
}
