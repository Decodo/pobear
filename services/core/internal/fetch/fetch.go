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

type Throttler interface {
	Wait(ctx context.Context) error
}

var ErrUnknownTier = errors.New("fetch: unknown tier")

type Fetcher struct {
	tiers    map[string]Tier
	retries  int
	backoff  func(attempt int) time.Duration
	throttle Throttler
	sem      chan struct{}
}

func NewFetcher(ts map[string]Tier, concurrency, retries int, backoff func(attempt int) time.Duration, throttle Throttler) *Fetcher {
	f := &Fetcher{tiers: ts, retries: retries, backoff: backoff, throttle: throttle}
	if concurrency > 0 {
		f.sem = make(chan struct{}, concurrency)
	}
	return f
}

func (f *Fetcher) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error) {
	tier, ok := f.tiers[cfg.Use]
	if !ok {
		return nil, "", ErrUnknownTier
	}

	release, err := f.acquire(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()

	retries := max(f.retries, 1)

	var result *tiers.Result
	var kind fault.Kind

	for attempt := 1; attempt <= retries; attempt++ {
		if f.throttle != nil {
			if err := f.throttle.Wait(ctx); err != nil {
				return nil, "", err
			}
		}

		result, kind, err = tier.Fetch(ctx, cfg, url)

		if err == nil && kind == "" {
			return result, kind, nil
		}
		if attempt == retries {
			return result, kind, err
		}
		if err := f.waitBackoff(ctx, attempt); err != nil {
			return nil, "", err
		}
	}

	return result, kind, err
}

func (f *Fetcher) acquire(ctx context.Context) (release func(), err error) {
	if f.sem == nil {
		return func() {}, nil
	}
	select {
	case f.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return func() { <-f.sem }, nil
}

func (f *Fetcher) waitBackoff(ctx context.Context, attempt int) error {
	timer := time.NewTimer(f.backoff(attempt))
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
