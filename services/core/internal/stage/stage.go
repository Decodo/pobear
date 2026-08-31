package stage

import (
	"context"
	"errors"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/stage/extractors"
)

type Extractor interface {
	Run(ctx context.Context, body []byte, cfg jobspec.Stage) ([]extractors.Result, error)
}

var ErrUnknownExtractor = errors.New("stage: unknown extractor")

var ErrNotImplemented = errors.New("stage: not implemented")

type Stager struct {
	extractors map[string]Extractor
}

func NewStager(exts map[string]Extractor) *Stager {
	return &Stager{extractors: exts}
}

func (s *Stager) Run(ctx context.Context, body []byte, cfgs []jobspec.Stage) (data map[string]any, warnings []string, err error) {
	_ = ctx
	_ = body
	_ = cfgs
	return nil, nil, ErrNotImplemented
}
