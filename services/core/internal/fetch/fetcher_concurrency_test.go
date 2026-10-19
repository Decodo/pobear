package fetch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/fetch"
	"github.com/pobear/pobear/services/core/internal/fetch/tiers"
)

type barrierTier struct {
	started chan struct{}
	release chan struct{}
}

func (b *barrierTier) Fetch(ctx context.Context, _ jobspec.Fetch, _ string) (*tiers.Result, fault.Kind, error) {
	b.started <- struct{}{}
	select {
	case <-b.release:
		return &tiers.Result{Status: 200}, "", nil
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
}

func TestFetchConcurrencyLimit(t *testing.T) {
	const (
		limit    = 2
		launched = 5
	)
	b := &barrierTier{started: make(chan struct{}, launched), release: make(chan struct{})}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": b}, limit, 1, noBackoff, nil)

	done := make(chan struct{}, launched)
	for i := 0; i < launched; i++ {
		go func() {
			_, _, _ = f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")
			done <- struct{}{}
		}()
	}

	for i := 0; i < limit; i++ {
		select {
		case <-b.started:
		case <-time.After(time.Second):
			t.Fatalf("expected %d calls to start, only %d did", limit, i)
		}
	}

	select {
	case <-b.started:
		t.Fatal("a call started beyond the configured concurrency limit")
	case <-time.After(100 * time.Millisecond):
	}

	close(b.release)

	for i := 0; i < launched; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("fetch %d never completed after release", i)
		}
	}
}

func TestFetchConcurrencyUnlimited(t *testing.T) {
	const launched = 64
	b := &barrierTier{started: make(chan struct{}), release: make(chan struct{})}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": b}, 0, 1, noBackoff, nil)

	done := make(chan struct{}, launched)
	for i := 0; i < launched; i++ {
		go func() {
			_, _, _ = f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")
			done <- struct{}{}
		}()
	}

	for i := 0; i < launched; i++ {
		select {
		case <-b.started:
		case <-time.After(time.Second):
			t.Fatalf("expected all %d calls to start concurrently, only %d did", launched, i)
		}
	}

	close(b.release)
	for i := 0; i < launched; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("fetch %d never completed after release", i)
		}
	}
}

func TestFetchConcurrencySlotReleasedOnError(t *testing.T) {
	const limit = 1
	tier := &fakeTier{results: []fakeTierResult{
		{err: errors.New("boom")},
		{err: errors.New("boom")},
	}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, limit, 1, noBackoff, nil)

	for i := 0; i < 2; i++ {
		done := make(chan struct{})
		go func() {
			_, _, _ = f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("fetch %d never completed — concurrency slot likely leaked on tier error", i)
		}
	}
	if tier.calls != 2 {
		t.Fatalf("expected exactly 2 calls to reach the tier, got %d", tier.calls)
	}
}

func TestFetchConcurrencyHeldAcrossRetries(t *testing.T) {
	const limit = 1
	wantErr := errors.New("boom")
	tier := &fakeTier{results: []fakeTierResult{
		{err: wantErr},
		{err: wantErr},
		{result: &tiers.Result{Status: 200}, err: nil},
	}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, limit, 3, noBackoff, nil)

	done := make(chan struct{})
	go func() {
		_, _, _ = f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Fetch with retries under a concurrency limit of 1 never completed — possible self-deadlock")
	}
	if tier.calls != 3 {
		t.Fatalf("expected exactly 3 calls, got %d", tier.calls)
	}
}
