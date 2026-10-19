package command_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

func TestCrawlBareWithTwoFieldsErrors(t *testing.T) {
	args := cliArgs("crawl https://example.com --depth 0 --css title=h1 --css link=a@href --bare")

	cmd := &command.Crawl{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (--bare requires at most one field)", got)
	}
	if want := "pobear: --bare requires at most one extracted value, but 2 fields are set (title, link)\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestCrawlBareWithNonZeroDepthErrorsBeforeFetching(t *testing.T) {
	args := cliArgs("crawl https://example.com --bare")

	runner := &fakeRunner{}
	cmd := &command.Crawl{Runner: runner, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (--bare requires --depth 0) -- same usage-error class as the two-fields case, not a runtime failure", got)
	}
	want := "pobear: --bare requires --depth 0 -- crawling deeper can discover more than one page, and --bare only ever prints a single value (use --out <dir> to write one file per page instead)\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if runner.gotData != nil {
		t.Fatalf("runner was called (gotData = %q), want it never called -- the whole crawl would have run for real before this usage error was ever reported", runner.gotData)
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

func TestCrawlProviderTierOmitsProfile(t *testing.T) {
	args := cliArgs("crawl https://example.com --depth 0 --provider decodo")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "decodo.yaml"), []byte(`
name: decodo
endpoint: https://data.decodo.com/v1/scrape
auth: {type: bearer, credential: token}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	credPath := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(credPath, []byte("POBEAR_PROVIDER_DECODO_TOKEN=secret\n"), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	runner := &fakeRunner{result: command.RunResult{Succeeded: 1}}
	cmd := &command.Crawl{Runner: runner, Providers: providerator.New(dir), CredentialsPath: credPath}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0; stderr=%s", got, stderr.String())
	}
	if runner.gotData == nil {
		t.Fatal("runner was never called")
	}
	if strings.Contains(string(runner.gotData), "profile") {
		t.Fatalf("runner.gotData = %s, want no profile field for the provider tier", runner.gotData)
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
