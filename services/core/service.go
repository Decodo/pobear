package core

import (
	"context"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/job"
)

type SpecParser interface {
	Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, fault.Kind, error)
}

type JobExecutor interface {
	Run(ctx context.Context, spec *jobspec.Spec) (job.Result, error)
}

type Service struct {
	parser   SpecParser
	executor JobExecutor
}

func NewService(parser SpecParser, executor JobExecutor) *Service {
	return &Service{parser: parser, executor: executor}
}

func (s *Service) Run(ctx context.Context, data []byte, mode jobspec.Mode) (job.Result, fault.Kind, error) {
	spec, kind, err := s.parser.Parse(data, mode)
	if err != nil {
		return job.Result{}, kind, err
	}

	result, err := s.executor.Run(ctx, spec)
	return result, "", err
}
