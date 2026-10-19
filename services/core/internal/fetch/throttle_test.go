package fetch_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Decodo/pobear/services/core/internal/fetch"
)

func TestThrottleZeroRPSIsUnlimited(t *testing.T) {
	th := fetch.NewThrottle(0)

	start := time.Now()
	for i := 0; i < 50; i++ {
		if err := th.Wait(context.Background()); err != nil {
			t.Fatalf("call %d: expected no error, got %v", i, err)
		}
	}
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Fatalf("50 calls at rps=0 took %v, expected effectively instant (no throttling)", elapsed)
	}
}

func TestThrottleLimitsCallsPerSecond(t *testing.T) {
	const rps = 5
	th := fetch.NewThrottle(rps)

	start := time.Now()
	allowedWithinFirstWindow := 0
	for i := 0; i < rps; i++ {
		if err := th.Wait(context.Background()); err != nil {
			t.Fatalf("call %d: expected no error, got %v", i, err)
		}
		allowedWithinFirstWindow++
	}
	firstBatchElapsed := time.Since(start)
	if firstBatchElapsed > 500*time.Millisecond {
		t.Fatalf("first %d calls (within the rps budget) took %v, expected them to proceed promptly", rps, firstBatchElapsed)
	}

	callStart := time.Now()
	if err := th.Wait(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	waited := time.Since(callStart)
	if waited < 400*time.Millisecond {
		t.Fatalf("the %dth call within one second returned after only %v, expected it to wait for the next window", rps+1, waited)
	}
	if waited > 2*time.Second {
		t.Fatalf("the %dth call took %v to proceed, expected roughly one second, not a long hang", rps+1, waited)
	}

	secondBatchStart := time.Now()
	for i := 0; i < rps-1; i++ {
		if err := th.Wait(context.Background()); err != nil {
			t.Fatalf("call %d in second window: expected no error, got %v", i, err)
		}
	}
	secondBatchElapsed := time.Since(secondBatchStart)
	if secondBatchElapsed > 500*time.Millisecond {
		t.Fatalf("remaining calls within the refilled window took %v, expected them to proceed promptly", secondBatchElapsed)
	}

	thirdCallStart := time.Now()
	if err := th.Wait(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	waited = time.Since(thirdCallStart)
	if waited < 400*time.Millisecond {
		t.Fatalf("the call after the refilled window returned after only %v, expected it to wait for the next window", waited)
	}
	if waited > 2*time.Second {
		t.Fatalf("the call after the refilled window took %v to proceed, expected roughly one second, not a long hang", waited)
	}
}

func TestThrottleEnforcesLimitAcrossGoroutines(t *testing.T) {
	const rps = 5
	th := fetch.NewThrottle(rps)

	var completed atomic.Int32
	var errOnce sync.Once
	var firstErr error
	var wg sync.WaitGroup
	for i := 0; i < rps*3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := th.Wait(context.Background()); err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
			completed.Add(1)
		}()
	}

	time.Sleep(750 * time.Millisecond)
	got := completed.Load()
	if got > rps {
		t.Fatalf("expected at most %d calls to complete within the first window across goroutines, got %d", rps, got)
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("expected no error from any goroutine's Wait, got %v", firstErr)
	}
}

func TestThrottleWaitRespectsContextCancellation(t *testing.T) {
	th := fetch.NewThrottle(1)
	if err := th.Wait(context.Background()); err != nil {
		t.Fatalf("expected no error consuming the only rps=1 slot, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- th.Wait(ctx)
	}()
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait ignored context cancellation")
	}
}
