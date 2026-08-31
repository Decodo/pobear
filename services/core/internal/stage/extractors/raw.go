package extractors

import (
	"context"

	"github.com/pobear/pobear/pkg/jobspec"
)

type Raw struct{}

func (Raw) Run(ctx context.Context, body []byte, cfg jobspec.Stage) ([]Result, error) {
	_ = ctx
	_ = body
	_ = cfg
	return nil, ErrNotImplemented
}
