package command_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type fakeRunner struct {
	gotData []byte
	gotMode jobspec.Mode
	result  command.RunResult
	kind    fault.Kind
	err     error
}

func (f *fakeRunner) Run(_ context.Context, data []byte, mode jobspec.Mode) (command.RunResult, fault.Kind, error) {
	f.gotData = data
	f.gotMode = mode
	return f.result, f.kind, f.err
}

func TestScrapePrintSpecDoesNotRunTheJob(t *testing.T) {
	args := cliArgs("scrape https://example.com --print-spec")

	runner := &fakeRunner{}
	cmd := &command.Scrape{Runner: runner, Providers: providerator.New(t.TempDir())}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if runner.gotData != nil {
		t.Fatalf("runner was called (gotData = %q), want it never called for --print-spec", runner.gotData)
	}
}

func TestScrapeRunsJobAndMapsSuccessExitCode(t *testing.T) {
	args := cliArgs("scrape https://example.com")

	runner := &fakeRunner{result: command.RunResult{Succeeded: 1}}
	cmd := &command.Scrape{Runner: runner, Providers: providerator.New(t.TempDir())}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if runner.gotMode != jobspec.ModePlain {
		t.Fatalf("runner.gotMode = %v, want ModePlain", runner.gotMode)
	}
}

func TestScrapeRunnerFailureMapsToPartialFailureExitCode(t *testing.T) {
	args := cliArgs("scrape https://example.com")

	runner := &fakeRunner{result: command.RunResult{Succeeded: 1, Failed: 1, Kinds: []fault.Kind{fault.Network}}}
	cmd := &command.Scrape{Runner: runner, Providers: providerator.New(t.TempDir())}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 4 {
		t.Fatalf("Run() = %d, want 4 (partial failure)", got)
	}
}

func TestScrapeUnknownProviderOptionErrors(t *testing.T) {
	args := cliArgs("scrape https://example.com --provider bogus-provider")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	if want := "pobear: unknown provider \"bogus-provider\"\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestScrapeOutputFormatRawWithoutExtractionErrors(t *testing.T) {
	args := cliArgs("scrape https://example.com --output-format raw")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (no extraction stage set, ambiguous)", got)
	}
}
