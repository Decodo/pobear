package command

import (
	"context"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/providerator"
	"github.com/pobear/pobear/services/core"
)

type RunResult struct {
	Succeeded int
	Failed    int
	Skipped   int
	Kinds     []fault.Kind
}

type JobRunner interface {
	Run(ctx context.Context, data []byte, mode jobspec.Mode) (RunResult, fault.Kind, error)
}

type coreRunner struct{ svc *core.Service }

func NewJobRunner(svc *core.Service) JobRunner {
	return coreRunner{svc: svc}
}

func (r coreRunner) Run(ctx context.Context, data []byte, mode jobspec.Mode) (RunResult, fault.Kind, error) {
	result, kind, err := r.svc.Run(ctx, data, mode)
	return RunResult{
		Succeeded: result.Succeeded,
		Failed:    result.Failed,
		Skipped:   result.Skipped,
		Kinds:     result.Kinds,
	}, kind, err
}

type Scrape struct {
	Runner    JobRunner
	Providers *providerator.Providerator
}

func (s *Scrape) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = ctx
	_ = args
	_ = stdin
	_ = stdout
	_ = stderr
	_ = s.Runner
	_ = s.Providers
	return exitcode.Fatal
}
