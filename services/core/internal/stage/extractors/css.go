package extractors

import (
	"context"

	"github.com/pobear/pobear/pkg/jobspec"
)

type CSS struct{}

func (CSS) Run(ctx context.Context, body []byte, cfg jobspec.Stage) ([]Result, error) {
	_ = ctx
	_ = body
	_ = cfg
	return nil, ErrNotImplemented
}
