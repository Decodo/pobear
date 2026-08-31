package fetch

import "context"

type Throttle struct {
	rps int
}

func NewThrottle(rps int) *Throttle {
	return &Throttle{rps: rps}
}

func (t *Throttle) Wait(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}
