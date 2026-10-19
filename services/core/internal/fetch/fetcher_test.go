package fetch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/fetch"
	"github.com/Decodo/pobear/services/core/internal/fetch/tiers"
)

type fakeTier struct {
	results []fakeTierResult
	calls   int
}

type fakeTierResult struct {
	result *tiers.Result
	fault  fault.Kind
	err    error
}

func (f *fakeTier) Fetch(_ context.Context, _ jobspec.Fetch, _ string) (*tiers.Result, fault.Kind, error) {
	i := f.calls
	f.calls++
	if i >= len(f.results) {
		r := f.results[len(f.results)-1]
		return r.result, r.fault, r.err
	}
	r := f.results[i]
	return r.result, r.fault, r.err
}

func noBackoff(_ int) time.Duration { return 0 }

func TestFetchDispatchesToConfiguredTier(t *testing.T) {
	want := &tiers.Result{Status: 200, Body: []byte("ok")}
	local := &fakeTier{results: []fakeTierResult{{result: want, fault: fault.HTTP, err: nil}}}
	proxy := &fakeTier{results: []fakeTierResult{{err: errors.New("proxy should not be called")}}}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 1, noBackoff, nil)
	got, gotFault, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if gotFault != fault.HTTP {
		t.Fatalf("fault = %q, want %q", gotFault, fault.HTTP)
	}
	if proxy.calls != 0 {
		t.Fatalf("expected proxy tier never called, got %d calls", proxy.calls)
	}
}

func TestFetchUnknownTier(t *testing.T) {
	f := fetch.NewFetcher(map[string]fetch.Tier{}, 0, 1, noBackoff, nil)
	_, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "missing"}, "https://a.test")

	if !errors.Is(err, fetch.ErrUnknownTier) {
		t.Fatalf("expected ErrUnknownTier, got %v", err)
	}
}

func TestFetchRetriesSameTierOnFailure(t *testing.T) {
	wantErr := errors.New("boom")
	tier := &fakeTier{results: []fakeTierResult{
		{err: wantErr},
		{err: wantErr},
		{result: &tiers.Result{Status: 200}, err: nil},
	}}
	proxy := &fakeTier{results: []fakeTierResult{{err: errors.New("proxy should never be called")}}}
	provider := &fakeTier{results: []fakeTierResult{{err: errors.New("provider should never be called")}}}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier, "proxy": proxy, "provider": provider}, 0, 3, noBackoff, nil)
	got, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error after the 3rd attempt succeeds, got %v", err)
	}
	if got == nil || got.Status != 200 {
		t.Fatalf("expected the successful 3rd attempt's result, got %+v", got)
	}
	if tier.calls != 3 {
		t.Fatalf("expected exactly 3 calls, got %d", tier.calls)
	}
	if proxy.calls != 0 || provider.calls != 0 {
		t.Fatalf("expected only the local tier ever called, got proxy=%d provider=%d", proxy.calls, provider.calls)
	}
}

func TestFetchGivesUpAfterRetriesExhausted(t *testing.T) {
	err1 := errors.New("boom-1")
	err2 := errors.New("boom-2")
	err3 := errors.New("boom-3")
	tier := &fakeTier{results: []fakeTierResult{{err: err1}, {err: err2}, {err: err3}}}
	proxy := &fakeTier{results: []fakeTierResult{{err: errors.New("proxy should never be called")}}}
	provider := &fakeTier{results: []fakeTierResult{{err: errors.New("provider should never be called")}}}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier, "proxy": proxy, "provider": provider}, 0, 3, noBackoff, nil)
	_, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if !errors.Is(err, err3) {
		t.Fatalf("expected the last attempt's error (boom-3), got %v", err)
	}
	if tier.calls != 3 {
		t.Fatalf("expected exactly 3 calls (retries exhausted, no more, no fewer), got %d", tier.calls)
	}
	if proxy.calls != 0 || provider.calls != 0 {
		t.Fatalf("expected only the local tier ever called, got proxy=%d provider=%d", proxy.calls, provider.calls)
	}
}

func TestFetchSucceedsImmediatelyStopsRetrying(t *testing.T) {
	tier := &fakeTier{results: []fakeTierResult{
		{result: &tiers.Result{Status: 200}, err: nil},
		{err: errors.New("should never be reached")},
	}}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 3, noBackoff, nil)
	_, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tier.calls != 1 {
		t.Fatalf("expected exactly 1 call (succeeded immediately), got %d", tier.calls)
	}
}

func TestFetchWaitsBackoffBetweenAttempts(t *testing.T) {
	wantErr := errors.New("boom")
	tier := &fakeTier{results: []fakeTierResult{
		{err: wantErr},
		{err: wantErr},
		{result: &tiers.Result{Status: 200}, err: nil},
	}}

	var gotAttempts []int
	durations := map[int]time.Duration{1: 20 * time.Millisecond, 2: 60 * time.Millisecond}
	backoff := func(attempt int) time.Duration {
		gotAttempts = append(gotAttempts, attempt)
		return durations[attempt]
	}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 3, backoff, nil)
	start := time.Now()
	_, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	wantAttempts := []int{1, 2}
	if len(gotAttempts) != len(wantAttempts) {
		t.Fatalf("backoff called %d times with %v, want %d times with %v", len(gotAttempts), gotAttempts, len(wantAttempts), wantAttempts)
	}
	for i, a := range wantAttempts {
		if gotAttempts[i] != a {
			t.Fatalf("backoff call %d used attempt=%d, want %d", i, gotAttempts[i], a)
		}
	}
	if elapsed < 80*time.Millisecond {
		t.Fatalf("Fetch took %v, expected at least ~80ms (20ms + 60ms, the two returned durations actually slept)", elapsed)
	}
	if elapsed > 400*time.Millisecond {
		t.Fatalf("Fetch took %v, expected roughly 80ms — a hardcoded sleep ignoring backoff's return value, or some other timing bug, looks likely", elapsed)
	}
}

func TestFetchRetriesOnFaultWithNilError(t *testing.T) {
	tier := &fakeTier{results: []fakeTierResult{
		{result: &tiers.Result{Status: 429}, fault: fault.RateLimit, err: nil},
		{result: &tiers.Result{Status: 429}, fault: fault.RateLimit, err: nil},
		{result: &tiers.Result{Status: 200}, err: nil},
	}}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 3, noBackoff, nil)
	got, gotFault, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error after the 3rd attempt succeeds, got %v", err)
	}
	if got == nil || got.Status != 200 {
		t.Fatalf("expected the successful 3rd attempt's result, got %+v", got)
	}
	if gotFault != "" {
		t.Fatalf("expected no fault on the final successful result, got %q", gotFault)
	}
	if tier.calls != 3 {
		t.Fatalf("expected exactly 3 calls (retried on the fault-with-nil-error outcome twice), got %d", tier.calls)
	}
}

func TestFetchGivesUpAfterRetriesExhaustedOnFault(t *testing.T) {
	tier := &fakeTier{results: []fakeTierResult{
		{result: &tiers.Result{Status: 429}, fault: fault.RateLimit, err: nil},
	}}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 3, noBackoff, nil)
	got, gotFault, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil {
		t.Fatalf("expected no Go error for a fault-only outcome, got %v", err)
	}
	if got == nil || got.Status != 429 {
		t.Fatalf("expected the last attempt's Result to be propagated, got %+v", got)
	}
	if gotFault != fault.RateLimit {
		t.Fatalf("fault = %q, want %q", gotFault, fault.RateLimit)
	}
	if tier.calls != 3 {
		t.Fatalf("expected exactly 3 calls (retries exhausted), got %d", tier.calls)
	}
}

func TestFetchRespectsContextCancellation(t *testing.T) {
	wantErr := errors.New("boom")
	tier := &fakeTier{results: []fakeTierResult{{err: wantErr}}}
	slowBackoff := func(_ int) time.Duration { return time.Hour }

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 5, slowBackoff, nil)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, _, err := f.Fetch(ctx, jobspec.Fetch{Use: "local"}, "https://a.test")
		done <- err
	}()
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Fetch ignored context cancellation, likely blocked in backoff wait")
	}
}

type fakeThrottle struct {
	calls int
	err   error
}

func (f *fakeThrottle) Wait(_ context.Context) error {
	f.calls++
	return f.err
}

func TestFetchWaitsOnThrottleBeforeEveryAttempt(t *testing.T) {
	wantErr := errors.New("boom")
	tier := &fakeTier{results: []fakeTierResult{
		{err: wantErr},
		{err: wantErr},
		{result: &tiers.Result{Status: 200}, err: nil},
	}}
	throttle := &fakeThrottle{}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 3, noBackoff, throttle)
	_, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if throttle.calls != 3 {
		t.Fatalf("throttle.Wait called %d times, want 3 (once per attempt, retries included)", throttle.calls)
	}
}

func TestFetchPropagatesThrottleErrorWithoutCallingTier(t *testing.T) {
	wantErr := errors.New("throttle context canceled")
	tier := &fakeTier{results: []fakeTierResult{{err: errors.New("tier should never be called")}}}
	throttle := &fakeThrottle{err: wantErr}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 3, noBackoff, throttle)
	_, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the throttle's error, got %v", err)
	}
	if tier.calls != 0 {
		t.Fatalf("expected the tier never called once the throttle errors, got %d calls", tier.calls)
	}
}

func TestFetchWithNilThrottleNeverWaits(t *testing.T) {
	tier := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}, err: nil}}}

	f := fetch.NewFetcher(map[string]fetch.Tier{"local": tier}, 0, 1, noBackoff, nil)
	_, _, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil {
		t.Fatalf("expected no error with a nil throttle, got %v", err)
	}
}

func TestFetchSetsResultTierToTheConfiguredUseTier(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local}, 0, 1, noBackoff, nil)

	result, kind, err := f.Fetch(context.Background(), jobspec.Fetch{Use: "local"}, "https://a.test")

	if err != nil || kind != "" {
		t.Fatalf("Fetch() = %v, %q, %v, want a clean success", result, kind, err)
	}
	if result.Tier != "local" {
		t.Fatalf("result.Tier = %q, want %q", result.Tier, "local")
	}
}

func TestFetchClimbsTheLadderOnBlocked(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{fault: fault.Blocked, err: errors.New("blocked")}}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 1, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Ladder: true, Proxy: "http://p.test"}
	result, kind, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if err != nil || kind != "" {
		t.Fatalf("Fetch() = %v, %q, %v, want success after climbing to proxy", result, kind, err)
	}
	if result.Tier != "proxy" {
		t.Fatalf("result.Tier = %q, want %q", result.Tier, "proxy")
	}
	if local.calls != 1 || proxy.calls != 1 {
		t.Fatalf("local calls = %d, proxy calls = %d, want 1 and 1", local.calls, proxy.calls)
	}
}

func TestFetchStopsOnNonEscalatingFaultKind(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{fault: fault.Auth, err: errors.New("bad credential")}}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 1, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Ladder: true, Proxy: "http://p.test"}
	_, kind, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if kind != fault.Auth || err == nil {
		t.Fatalf("Fetch() kind = %q, err = %v, want fault.Auth and a non-nil error, no escalation", kind, err)
	}
	if proxy.calls != 0 {
		t.Fatalf("proxy.calls = %d, want 0 -- fault.Auth must never escalate", proxy.calls)
	}
}

func TestFetchEachRungGetsItsOwnFullRetryBudget(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{
		{fault: fault.Blocked, err: errors.New("1")},
		{fault: fault.Blocked, err: errors.New("2")},
		{fault: fault.Blocked, err: errors.New("3")},
	}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 3, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Ladder: true, Proxy: "http://p.test"}
	_, _, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if err != nil {
		t.Fatalf("Fetch(): %v", err)
	}
	if local.calls != 3 {
		t.Fatalf("local.calls = %d, want 3 -- retries=3 should give local its own full budget before escalating", local.calls)
	}
	if proxy.calls != 1 {
		t.Fatalf("proxy.calls = %d, want 1", proxy.calls)
	}
}

func TestFetchLadderNeverDropsTheStartingTier(t *testing.T) {
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	provider := &fakeTier{results: []fakeTierResult{{err: errors.New("provider should never be called")}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"proxy": proxy, "provider": provider}, 0, 1, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "proxy", Ladder: true, Name: "someProvider"}
	result, kind, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if err != nil || kind != "" {
		t.Fatalf("Fetch() = %v, %q, %v, want success from the starting tier", result, kind, err)
	}
	if proxy.calls != 1 {
		t.Fatalf("proxy.calls = %d, want 1 -- the starting tier must never be dropped by the presence check", proxy.calls)
	}
	if provider.calls != 0 {
		t.Fatalf("provider.calls = %d, want 0", provider.calls)
	}
}

func TestFetchWithoutLadderRunsExactlyTheOldSingleTierPath(t *testing.T) {
	local := &fakeTier{results: []fakeTierResult{{fault: fault.Blocked, err: errors.New("blocked")}}}
	proxy := &fakeTier{results: []fakeTierResult{{result: &tiers.Result{Status: 200}}}}
	f := fetch.NewFetcher(map[string]fetch.Tier{"local": local, "proxy": proxy}, 0, 1, noBackoff, nil)

	cfg := jobspec.Fetch{Use: "local", Proxy: "http://p.test"}
	_, kind, err := f.Fetch(context.Background(), cfg, "https://a.test")

	if kind != fault.Blocked || err == nil {
		t.Fatalf("Fetch() kind = %q, err = %v, want fault.Blocked returned directly -- Ladder is false, must never escalate", kind, err)
	}
	if proxy.calls != 0 {
		t.Fatalf("proxy.calls = %d, want 0 -- Ladder is false", proxy.calls)
	}
}
