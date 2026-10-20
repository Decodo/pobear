package extractors

import (
	"context"

	"github.com/Decodo/pobear/pkg/jobspec"
)

type Raw struct{}

func (Raw) Run(ctx context.Context, body []byte, cfg jobspec.Stage) ([]Result, error) {
	return []Result{{Key: "raw", Value: string(body)}}, nil
}
