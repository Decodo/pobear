package pool_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/pobear/pobear/services/core/internal/job/pool"
)

func TestPoolRunsEverySubmittedUnit(t *testing.T) {
	p := pool.New(4)
	var ran int32
	for i := 0; i < 20; i++ {
		p.Submit(func() { atomic.AddInt32(&ran, 1) })
	}
	p.Wait()

	if got := atomic.LoadInt32(&ran); got != 20 {
		t.Fatalf("ran = %d, want 20", got)
	}
}

func TestPoolBoundsConcurrency(t *testing.T) {
	const limit = 3
	p := pool.New(limit)

	var current, peak int32
	for i := 0; i < 20; i++ {
		p.Submit(func() {
			n := atomic.AddInt32(&current, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if n <= old || atomic.CompareAndSwapInt32(&peak, old, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&current, -1)
		})
	}
	p.Wait()

	if got := atomic.LoadInt32(&peak); got > limit {
		t.Fatalf("peak concurrent units = %d, want <= %d", got, limit)
	}
}

func TestPoolWaitBlocksForSelfSubmittedWork(t *testing.T) {
	p := pool.New(2)
	var ran int32

	var submitChain func(depth int)
	submitChain = func(depth int) {
		p.Submit(func() {
			atomic.AddInt32(&ran, 1)
			if depth > 0 {
				submitChain(depth - 1)
			}
		})
	}
	submitChain(4)

	p.Wait()

	if got := atomic.LoadInt32(&ran); got != 5 {
		t.Fatalf("ran = %d, want 5 (the original unit plus its 4 chained descendants, all awaited)", got)
	}
}

func TestPoolZeroOrNegativeConcurrencyRunsSequentiallyInsteadOfDeadlocking(t *testing.T) {
	for _, n := range []int{0, -1} {
		p := pool.New(n)
		var ran int32
		p.Submit(func() { atomic.AddInt32(&ran, 1) })

		done := make(chan struct{})
		go func() {
			p.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("New(%d) deadlocked instead of treating it as concurrency 1", n)
		}
		if got := atomic.LoadInt32(&ran); got != 1 {
			t.Fatalf("New(%d): ran = %d, want 1", n, got)
		}
	}
}

func TestPoolWaitReturnsImmediatelyWithNoWork(t *testing.T) {
	p := pool.New(2)

	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait blocked with nothing ever submitted")
	}
}
