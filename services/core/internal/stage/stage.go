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

func (s *Stager) Run(ctx context.Context, body []byte, cfgs []jobspec.Stage) (data map[string]any, warnings []string, err error) {
	data = make(map[string]any)
	writtenBy := make(map[string]int)

	for i, cfg := range cfgs {
		ext, ok := s.extractors[cfg.Use]
		if !ok {
			return nil, nil, fmt.Errorf("%w: %q", ErrUnknownExtractor, cfg.Use)
		}

		results, err := ext.Run(ctx, body, cfg)
		if err != nil {
			return nil, nil, err
		}

		for _, r := range results {
			warnings = recordResult(data, writtenBy, warnings, i, cfg, cfgs, r)
		}
	}

	return data, warnings, nil
}

func recordResult(data map[string]any, writtenBy map[string]int, warnings []string, i int, cfg jobspec.Stage, cfgs []jobspec.Stage, r extractors.Result) []string {
	if prevIdx, collided := writtenBy[r.Key]; collided {
		warnings = append(warnings, fmt.Sprintf(
			"stages[%d] (%s) overwrites data[%q] previously written by stages[%d] (%s)",
			i, cfg.Use, r.Key, prevIdx, cfgs[prevIdx].Use))
	}
	data[r.Key] = r.Value
	writtenBy[r.Key] = i
	return warnings
}
