package job_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/job"
)

type fakeSpecParser struct {
	spec    *jobspec.Spec
	err     error
	gotData []byte
	gotMode jobspec.Mode
	calls   int
}

func (f *fakeSpecParser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error) {
	f.gotData = data
	f.gotMode = mode
	f.calls++
	return f.spec, f.err
}

func TestParserSuccessPassesSpecThrough(t *testing.T) {
	want := &jobspec.Spec{Version: 1}
	fp := &fakeSpecParser{spec: want}
	p := job.NewParser(fp)

	got, kind, err := p.Parse([]byte("version: 1"), jobspec.ModePlain)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if kind != "" {
		t.Fatalf("expected empty fault.Kind on success, got %q", kind)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParserValidationErrorIsFaultSpec(t *testing.T) {
	fp := &fakeSpecParser{err: &jobspec.ValidationError{Field: "source", Message: "exactly one of urls or crawl required"}}
	p := job.NewParser(fp)

	_, kind, err := p.Parse([]byte("bogus"), jobspec.ModePlain)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if kind != fault.Spec {
		t.Fatalf("expected fault.Spec for a *jobspec.ValidationError, got %q", kind)
	}
	if !errors.Is(err, fp.err) {
		t.Fatalf("expected the underlying error to be preserved via errors.Is, got %v", err)
	}
}

func TestParserGenericErrorIsFaultParse(t *testing.T) {
	fp := &fakeSpecParser{err: errors.New("yaml: line 3: could not find expected ':'")}
	p := job.NewParser(fp)

	_, kind, err := p.Parse([]byte("not: valid: yaml: at: all"), jobspec.ModePlain)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if kind != fault.Parse {
		t.Fatalf("expected fault.Parse for a non-ValidationError, got %q", kind)
	}
	if !errors.Is(err, fp.err) {
		t.Fatalf("expected the underlying error to be preserved via errors.Is, got %v", err)
	}
}

func TestParserWrappedValidationErrorIsStillFaultSpec(t *testing.T) {
	original := &jobspec.ValidationError{Field: "fetch.use", Message: "required"}
	wrapped := fmt.Errorf("parse spec: %w", original)
	fp := &fakeSpecParser{err: wrapped}
	p := job.NewParser(fp)

	_, kind, err := p.Parse([]byte("bogus"), jobspec.ModePlain)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if kind != fault.Spec {
		t.Fatalf("expected fault.Spec for a wrapped *jobspec.ValidationError (errors.As, not a direct type assertion), got %q", kind)
	}
	if !errors.Is(err, original) {
		t.Fatalf("expected the underlying error to be preserved via errors.Is, got %v", err)
	}
}

func TestParserDelegatesDataAndModeVerbatim(t *testing.T) {
	fp := &fakeSpecParser{spec: &jobspec.Spec{}}
	p := job.NewParser(fp)

	wantData := []byte("version: 1\nsource:\n  urls: [https://a.test]\n")
	_, _, err := p.Parse(wantData, jobspec.ModeRPC)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fp.calls != 1 {
		t.Fatalf("expected exactly 1 call to the injected SpecParser, got %d", fp.calls)
	}
	if string(fp.gotData) != string(wantData) {
		t.Fatalf("data forwarded = %q, want %q", fp.gotData, wantData)
	}
	if fp.gotMode != jobspec.ModeRPC {
		t.Fatalf("mode forwarded = %v, want %v", fp.gotMode, jobspec.ModeRPC)
	}
}
