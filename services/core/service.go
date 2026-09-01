package core

import (
	"context"
	"errors"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/job"
)

type SpecParser interface {
	Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, fault.Kind, error)
}

type JobExecutor interface {
	Run(ctx context.Context, spec *jobspec.Spec) (job.Result, error)
}

var ErrNotImplemented = errors.New("core: not implemented")

type Service struct {
	parser   SpecParser
	executor JobExecutor
}

func NewService(parser SpecParser, executor JobExecutor) *Service {
	return &Service{parser: parser, executor: executor}
}

func (s *Service) Run(ctx context.Context, data []byte, mode jobspec.Mode) (job.Result, fault.Kind, error) {
	_ = ctx
	_ = data
	_ = mode
	return job.Result{}, "", ErrNotImplemented
}
