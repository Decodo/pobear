package command_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/cli/internal/command"
)

type fakeSpecParser struct {
	gotData []byte
	gotMode jobspec.Mode
	spec    *jobspec.Spec
	err     error
}

func (f *fakeSpecParser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error) {
	f.gotData = data
	f.gotMode = mode
	return f.spec, f.err
}

func TestRunReadsStdinWhenPathIsDash(t *testing.T) {
	args := cliArgs("run -")

	parser := &fakeSpecParser{spec: &jobspec.Spec{Version: 1, Fetch: jobspec.Fetch{Use: "local"}}}
	cmd := &command.Run{Parser: parser, Runner: &fakeRunner{result: command.RunResult{Succeeded: 1}}}

	got := cmd.Run(context.Background(), args, strings.NewReader("version: 1"), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if string(parser.gotData) != "version: 1" {
		t.Fatalf("parser.gotData = %q, want stdin's content", parser.gotData)
	}
}

func TestRunValidationErrorPrintsSpecPrefixedMessage(t *testing.T) {
	args := cliArgs("run -")

	parser := &fakeSpecParser{err: &jobspec.ValidationError{
		Field:   "source",
		Message: "`source` must set exactly one of `urls` or `crawl`, but neither is set",
	}}
	cmd := &command.Run{Parser: parser, Runner: &fakeRunner{}}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, strings.NewReader("fetch: {use: local}"), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	want := "pobear: spec: `source` must set exactly one of `urls` or `crawl`, but neither is set\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunNonValidationParseErrorIsFatal(t *testing.T) {
	args := cliArgs("run -")

	parser := &fakeSpecParser{err: errors.New("unexpected EOF")}
	cmd := &command.Run{Parser: parser, Runner: &fakeRunner{}}

	got := cmd.Run(context.Background(), args, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 5 {
		t.Fatalf("Run() = %d, want 5 (fatal, not a validation error)", got)
	}
}

func TestRunPrintSpecDoesNotRunTheJob(t *testing.T) {
	args := cliArgs("run - --print-spec")

	runner := &fakeRunner{}
	parser := &fakeSpecParser{spec: &jobspec.Spec{Version: 1, Source: jobspec.Source{URLs: []string{"https://a.test"}}, Fetch: jobspec.Fetch{Use: "local"}}}
	cmd := &command.Run{Parser: parser, Runner: runner}

	got := cmd.Run(context.Background(), args, strings.NewReader("version: 1"), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if runner.gotData != nil {
		t.Fatalf("runner was called, want it never called for --print-spec")
	}
}

func TestRunMissingTargetPrintsHelp(t *testing.T) {
	args := cliArgs("run")

	cmd := &command.Run{Parser: &fakeSpecParser{}, Runner: &fakeRunner{}}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (missing required argument)", got)
	}
	if want := "pobear: run requires a job spec path\n"; !strings.HasPrefix(stderr.String(), want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr.String(), want)
	}
	if !strings.Contains(stderr.String(), "Usage:") || !strings.Contains(stderr.String(), "Flags:") {
		t.Fatalf("stderr = %q, want it to include the command's full usage/flags help", stderr.String())
	}
}
