package fetch

import (
	"context"
	"errors"
	"slices"
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
	release, err := f.acquire(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()

	var result *tiers.Result
	var kind fault.Kind

	for _, tierName := range ladderFor(cfg) {
		tier, ok := f.tiers[tierName]
		if !ok {
			return nil, "", ErrUnknownTier
		}

		result, kind, err = f.fetchTierWithRetries(ctx, tier, cfg, url)
		if result != nil {
			result.Tier = tierName
		}
		if err == nil && kind == "" {
			return result, kind, nil
		}
		if !escalates(kind) {
			return result, kind, err
		}
	}
	return result, kind, err
}

func (f *Fetcher) fetchTierWithRetries(ctx context.Context, tier Tier, cfg jobspec.Fetch, url string) (*tiers.Result, fault.Kind, error) {
	retries := max(f.retries, 1)

	var result *tiers.Result
	var kind fault.Kind
	var err error

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

func ladderFor(cfg jobspec.Fetch) []string {
	if !cfg.Ladder {
		return []string{cfg.Use}
	}
	start := slices.Index(jobspec.FetchTierOrder, cfg.Use)
	if start < 0 {
		return []string{cfg.Use}
	}
	var ladder []string
	for _, t := range jobspec.FetchTierOrder[start:] {
		if configured(cfg, t) {
			ladder = append(ladder, t)
		}
	}
	return ladder
}

// configured reports whether tier has what it needs to be used as a rung on
// the ladder. It's only ever called for tiers at or above cfg.Use (see
// ladderFor), so tier == "local" always takes the tier == cfg.Use branch --
// local needs no fields of its own to be configured.
func configured(cfg jobspec.Fetch, tier string) bool {
	if tier == cfg.Use {
		return true
	}
	switch tier {
	case "proxy":
		return cfg.Proxy != ""
	case "provider":
		return cfg.Name != ""
	default:
		return false
	}
}

func escalates(k fault.Kind) bool {
	return k == fault.Blocked || k == fault.RateLimit
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
