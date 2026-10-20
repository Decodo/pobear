package core_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core"
	"github.com/Decodo/pobear/services/core/internal/job"
)

type fakeSpecParser struct {
	spec    *jobspec.Spec
	kind    fault.Kind
	err     error
	gotData []byte
	gotMode jobspec.Mode
	calls   int
}

func (f *fakeSpecParser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, fault.Kind, error) {
	f.gotData = data
	f.gotMode = mode
	f.calls++
	return f.spec, f.kind, f.err
}

type fakeJobExecutor struct {
	result  job.Result
	err     error
	gotSpec *jobspec.Spec
	calls   int
}

func (f *fakeJobExecutor) Run(_ context.Context, spec *jobspec.Spec) (job.Result, error) {
	f.gotSpec = spec
	f.calls++
	return f.result, f.err
}

func TestServiceParseErrorShortCircuitsBeforeExecutor(t *testing.T) {
	wantErr := errors.New("bad yaml")
	parser := &fakeSpecParser{err: wantErr, kind: fault.Parse}
	executor := &fakeJobExecutor{}

	s := core.NewService(parser, executor)
	result, kind, err := s.Run(context.Background(), []byte("not: valid"), jobspec.ModePlain)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the parser's error, got %v", err)
	}
	if kind != fault.Parse {
		t.Fatalf("expected fault.Parse, got %q", kind)
	}
	if !reflect.DeepEqual(result, job.Result{}) {
		t.Fatalf("expected a zero-value Result on parse failure, got %+v", result)
	}
	if executor.calls != 0 {
		t.Fatalf("expected the executor never called after a parse error, got %d calls", executor.calls)
	}
}

func TestServiceValidationErrorShortCircuitsBeforeExecutor(t *testing.T) {
	wantErr := &jobspec.ValidationError{Field: "source", Message: "required"}
	parser := &fakeSpecParser{err: wantErr, kind: fault.Spec}
	executor := &fakeJobExecutor{}

	s := core.NewService(parser, executor)
	_, kind, err := s.Run(context.Background(), []byte("version: 1"), jobspec.ModePlain)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the parser's error, got %v", err)
	}
	if kind != fault.Spec {
		t.Fatalf("expected fault.Spec, got %q", kind)
	}
	if executor.calls != 0 {
		t.Fatalf("expected the executor never called after a validation error, got %d calls", executor.calls)
	}
}

func TestServiceSuccessfulParseRunsExecutor(t *testing.T) {
	spec := &jobspec.Spec{Version: 1, Source: jobspec.Source{URLs: []string{"https://a.test"}}}
	wantResult := job.Result{Succeeded: 3, Failed: 1, Kinds: []fault.Kind{fault.Timeout}}
	parser := &fakeSpecParser{spec: spec}
	executor := &fakeJobExecutor{result: wantResult}

	s := core.NewService(parser, executor)
	result, kind, err := s.Run(context.Background(), []byte("version: 1"), jobspec.ModePlain)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if kind != "" {
		t.Fatalf("expected an empty fault.Kind on success, got %q", kind)
	}
	if !reflect.DeepEqual(result, wantResult) {
		t.Fatalf("got %+v, want the executor's own Result %+v passed straight through", result, wantResult)
	}
	if executor.gotSpec != spec {
		t.Fatalf("expected the executor to receive the exact parsed spec, got %+v", executor.gotSpec)
	}
}

func TestServiceExecutorErrorIsReturnedWithEmptyKind(t *testing.T) {
	spec := &jobspec.Spec{Version: 1}
	wantErr := errors.New("executor exploded")
	wantResult := job.Result{Succeeded: 2, Failed: 1, Kinds: []fault.Kind{fault.Timeout}}
	parser := &fakeSpecParser{spec: spec}
	executor := &fakeJobExecutor{result: wantResult, err: wantErr}

	s := core.NewService(parser, executor)
	result, kind, err := s.Run(context.Background(), []byte("version: 1"), jobspec.ModePlain)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the executor's error, got %v", err)
	}
	if kind != "" {
		t.Fatalf("expected an empty fault.Kind for an executor-level error (not a validation fault), got %q", kind)
	}
	if !reflect.DeepEqual(result, wantResult) {
		t.Fatalf("got %+v, want the executor's own Result %+v passed through even alongside an error", result, wantResult)
	}
}

func TestServiceDelegatesDataAndModeVerbatim(t *testing.T) {
	parser := &fakeSpecParser{spec: &jobspec.Spec{}}
	executor := &fakeJobExecutor{}

	s := core.NewService(parser, executor)
	wantData := []byte("version: 1\nsource:\n  urls: [https://a.test]\n")
	_, _, err := s.Run(context.Background(), wantData, jobspec.ModeRPC)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parser.calls != 1 {
		t.Fatalf("expected exactly 1 call to the injected SpecParser, got %d", parser.calls)
	}
	if string(parser.gotData) != string(wantData) {
		t.Fatalf("data forwarded = %q, want %q", parser.gotData, wantData)
	}
	if parser.gotMode != jobspec.ModeRPC {
		t.Fatalf("mode forwarded = %v, want %v", parser.gotMode, jobspec.ModeRPC)
	}
}
