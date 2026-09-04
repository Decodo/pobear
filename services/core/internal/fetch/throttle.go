package fetch

import (
	"context"
	"sync"
	"time"
)

type Throttle struct {
	rps int

	mu          sync.Mutex
	windowStart time.Time
	count       int
}

func NewThrottle(rps int) *Throttle {
	return &Throttle{rps: rps}
}

func (t *Throttle) Wait(ctx context.Context) error {
	if t.rps <= 0 {
		return nil
	}

	for {
		t.mu.Lock()
		now := time.Now()
		if t.windowStart.IsZero() || now.Sub(t.windowStart) >= time.Second {
			t.windowStart = now
			t.count = 0
		}
		if t.count < t.rps {
			t.count++
			t.mu.Unlock()
			return nil
		}
		wait := time.Second - now.Sub(t.windowStart)
		t.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
