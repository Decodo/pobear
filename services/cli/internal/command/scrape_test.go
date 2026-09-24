package command_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestScrapePrintsPerFailureErrorDetail(t *testing.T) {
	args := cliArgs("scrape https://example.com")

	runner := &fakeRunner{result: command.RunResult{
		Failed: 1,
		Kinds:  []fault.Kind{fault.HTTP},
		Errors: []string{"https://example.com [http]: tiers: http status 403"},
	}}
	cmd := &command.Scrape{Runner: runner, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 5 {
		t.Fatalf("Run() = %d, want 5 (total failure)", got)
	}
	want := "https://example.com [http]: tiers: http status 403\nfetched 0, errors 1, skipped 0\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
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

func TestScrapeBareWithTwoFieldsErrors(t *testing.T) {
	args := cliArgs("scrape https://example.com --css title=h1 --css link=a@href --bare")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (--bare requires at most one field)", got)
	}
	if want := "pobear: --bare requires at most one extracted value, but 2 fields are set (title, link)\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestScrapeBearIsAnAliasForBare(t *testing.T) {
	args := cliArgs("scrape https://example.com --css title=h1 --css link=a@href --bear")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (--bear should behave exactly like --bare)", got)
	}
	if want := "pobear: --bare requires at most one extracted value, but 2 fields are set (title, link)\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestBearFlagIsHiddenFromHelp(t *testing.T) {
	args := cliArgs("scrape --help")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stdout bytes.Buffer
	cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &bytes.Buffer{})

	if strings.Contains(stdout.String(), "--bear") {
		t.Fatalf("--help output = %q, want no mention of the hidden --bear alias", stdout.String())
	}
}

func TestScrapeBareWithMultipleURLsErrorsBeforeFetching(t *testing.T) {
	urlsFile := filepath.Join(t.TempDir(), "urls.txt")
	if err := os.WriteFile(urlsFile, []byte("https://a.example\nhttps://b.example\n"), 0o600); err != nil {
		t.Fatalf("writing urls file: %v", err)
	}
	args := cliArgs("scrape " + urlsFile + " --bare")

	runner := &fakeRunner{}
	cmd := &command.Scrape{Runner: runner, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (--bare requires exactly one URL) -- same usage-error class as the two-fields case, not a runtime failure", got)
	}
	if want := "pobear: --bare requires exactly one URL to be targeted, got 2\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if runner.gotData != nil {
		t.Fatalf("runner was called (gotData = %q), want it never called -- both URLs would have been fetched for real before this usage error was ever reported", runner.gotData)
	}
}

func TestScrapeProviderTierOmitsProfile(t *testing.T) {
	args := cliArgs("scrape https://example.com --provider decodo")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "decodo.yaml"), []byte(`
name: decodo
endpoint: https://scraper-api.decodo.com/v2/scrape
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
	cmd := &command.Scrape{Runner: runner, Providers: providerator.New(dir), CredentialsPath: credPath}

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

func TestScrapeHeaderFlagSetsFetchHeaders(t *testing.T) {
	args := []string{"https://example.com", "--header", "X-Custom: 1", "--header", "Authorization: Bearer xyz"}

	runner := &fakeRunner{result: command.RunResult{Succeeded: 1}}
	cmd := &command.Scrape{Runner: runner, Providers: providerator.New(t.TempDir())}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	spec, err := jobspec.Parse(runner.gotData, jobspec.ModePlain)
	if err != nil {
		t.Fatalf("parsing the executed spec: %v", err)
	}
	if spec.Fetch.Headers["X-Custom"] != "1" {
		t.Fatalf("Fetch.Headers[X-Custom] = %q, want %q", spec.Fetch.Headers["X-Custom"], "1")
	}
	if spec.Fetch.Headers["Authorization"] != "Bearer xyz" {
		t.Fatalf("Fetch.Headers[Authorization] = %q, want %q", spec.Fetch.Headers["Authorization"], "Bearer xyz")
	}
}

func TestScrapeInvalidHeaderErrors(t *testing.T) {
	args := cliArgs("scrape https://example.com --header X-Custom")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	if want := "pobear: invalid --header \"X-Custom\", want \"name: value\"\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestScrapeDuplicateHeaderErrors(t *testing.T) {
	args := cliArgs("scrape https://example.com --header X-Custom:1 --header X-Custom:2")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	if want := "pobear: --header \"X-Custom\" is set more than once\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestScrapeMissingTargetPrintsHelp(t *testing.T) {
	args := cliArgs("scrape")

	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(t.TempDir())}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (missing required argument)", got)
	}
	if want := "pobear: scrape requires a URL or file argument\n"; !strings.HasPrefix(stderr.String(), want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr.String(), want)
	}
	if !strings.Contains(stderr.String(), "Usage:") || !strings.Contains(stderr.String(), "Flags:") {
		t.Fatalf("stderr = %q, want it to include the command's full usage/flags help", stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestScrapeProviderHelpShowsLiveSchema(t *testing.T) {
	args := cliArgs("scrape --provider decodo --help")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "decodo.yaml"), []byte(`
name: decodo
endpoint: https://scraper-api.decodo.com/v2/scrape
auth: {type: bearer, credential: token}
templates:
  - name: universal
    default: true
    options:
      - {name: render, type: bool, default: false}
    request: {body: {url: "{{ .URL }}"}}
  - name: amazon_product
    options:
      - {name: domain, type: enum, values: [com, co.uk, de], default: com}
    request: {body: {query: "{{ .URL }}"}}
response: {body: "$.html"}
`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	cmd := &command.Scrape{Runner: &fakeRunner{}, Providers: providerator.New(dir)}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if want := `Provider "decodo"`; !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), want)
	}
	if !strings.Contains(stdout.String(), "amazon_product") {
		t.Fatalf("stdout = %q, want it to list the amazon_product template", stdout.String())
	}
}
