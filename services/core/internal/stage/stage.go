package stage

import (
	"context"
	"errors"
	"fmt"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/stage/extractors"
)

type Extractor interface {
	Run(ctx context.Context, body []byte, cfg jobspec.Stage) ([]extractors.Result, error)
}

var ErrUnknownExtractor = errors.New("stage: unknown extractor")

type Stager struct {
	extractors map[string]Extractor
}

func NewStager(exts map[string]Extractor) *Stager {
	return &Stager{extractors: exts}
}

func (s *Stager) Run(ctx context.Context, body []byte, cfgs []jobspec.Stage) (data map[string]any, err error) {
	data = make(map[string]any)

	for _, cfg := range cfgs {
		ext, ok := s.extractors[cfg.Use]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownExtractor, cfg.Use)
		}

		results, err := ext.Run(ctx, body, cfg)
		if err != nil {
			return nil, err
		}

		for _, r := range results {
			data[r.Key] = r.Value
		}
	}

	return data, nil
}
