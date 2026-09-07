package command_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

func TestCrawlPrintSpecDoesNotRunTheJob(t *testing.T) {
	args := cliArgs("crawl https://example.com --depth 2 --print-spec")

	runner := &fakeRunner{}
	cmd := &command.Crawl{Runner: runner, Providers: providerator.New(t.TempDir())}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if runner.gotData != nil {
		t.Fatalf("runner was called, want it never called for --print-spec")
	}
}

func TestCrawlRunsJobAndMapsSuccessExitCode(t *testing.T) {
	args := cliArgs("crawl https://example.com --depth 1")

	runner := &fakeRunner{result: command.RunResult{Succeeded: 2}}
	cmd := &command.Crawl{Runner: runner, Providers: providerator.New(t.TempDir())}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
}

func TestCrawlWithNoFlagsSucceeds(t *testing.T) {
	args := cliArgs("crawl https://example.com")

	runner := &fakeRunner{result: command.RunResult{Succeeded: 1}}
	cmd := &command.Crawl{Runner: runner, Providers: providerator.New(t.TempDir())}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
}

func TestCrawlUnknownProviderOptionErrors(t *testing.T) {
	args := cliArgs("crawl https://example.com --provider bogus-provider")

	cmd := &command.Crawl{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	if want := "pobear: unknown provider \"bogus-provider\"\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestCrawlMissingTargetPrintsHelp(t *testing.T) {
	args := cliArgs("crawl")

	cmd := &command.Crawl{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (missing required argument)", got)
	}
	if want := "pobear: crawl requires a URL argument\n"; !strings.HasPrefix(stderr.String(), want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr.String(), want)
	}
	if !strings.Contains(stderr.String(), "Usage:") || !strings.Contains(stderr.String(), "Flags:") {
		t.Fatalf("stderr = %q, want it to include the command's full usage/flags help", stderr.String())
	}
}
