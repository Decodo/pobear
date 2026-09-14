package command

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
	"github.com/pobear/pobear/services/cli/internal/providerator"
	"github.com/pobear/pobear/services/core"
)

type RunResult struct {
	Succeeded   int
	Failed      int
	Skipped     int
	Kinds       []fault.Kind
	Errors      []string
	SkippedURLs []string
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
		Succeeded:   result.Succeeded,
		Failed:      result.Failed,
		Skipped:     result.Skipped,
		Kinds:       result.Kinds,
		Errors:      result.Errors,
		SkippedURLs: result.SkippedURLs,
	}, kind, err
}

type Scrape struct {
	Runner          JobRunner
	Providers       *providerator.Providerator
	CredentialsPath string
}

func (s *Scrape) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var f commonFlags
	code := 0

	cmd := &cobra.Command{
		Use:   "pobear scrape <url|file>",
		Short: "Scrape a single URL, or every URL listed in a file",
		Long: "Scrape a single URL, or every URL listed in a file.\n\n" +
			"The positional argument is either a single absolute http/https URL, or a\n" +
			"path to a file listing many: plain text, one URL per line, blank lines and\n" +
			"lines starting with # skipped.",
		Args: requireExactlyOne("scrape requires a URL or file argument"),
		RunE: func(_ *cobra.Command, cmdArgs []string) error {
			c, cerr := scrapeRun(ctx, s.Runner, s.Providers, s.CredentialsPath, cmdArgs[0], &f, stdout, stderr)
			code = c
			if cerr != nil {
				return cerr
			}
			return nil
		},
	}
	registerCommonFlags(cmd, &f, 0)
	wireProviderHelp(cmd, s.Providers, &f.provider)

	return runCobraCommand(cmd, args, stdin, stdout, stderr, &code)
}

func scrapeRun(ctx context.Context, runner JobRunner, providers *providerator.Providerator, credentialsPath, target string, f *commonFlags, stdout, stderr io.Writer) (int, *cliError) {
	urls, cerr := resolveScrapeURLs(target)
	if cerr != nil {
		return 0, cerr
	}

	stages, cerr := buildStages(f.extract, f.css)
	if cerr != nil {
		return 0, cerr
	}

	names := extractedFieldNames(f.extract, f.css)
	if cerr := validateBare(f.bare, names); cerr != nil {
		return 0, cerr
	}

	fetchUse := "local"
	switch {
	case f.provider != "":
		fetchUse = "provider"
	case f.proxy != "":
		fetchUse = "proxy"
	}

	opts := mergeOptions(f.render, f.geo, f.opts)
	var fetchOptions map[string]any
	if f.provider != "" {
		var perr *cliError
		fetchOptions, perr = resolveProviderOptions(providers, credentialsPath, f.provider, f.template, opts)
		if perr != nil {
			return 0, perr
		}
	} else {
		fetchOptions = stringMapToAny(opts)
	}

	sinkUse, sinkPath := "stdout", ""
	if f.out != "" && !isDirOut(f.out) {
		sinkUse, sinkPath = "ndjson", f.out
	}

	profile := f.profile
	if fetchUse == "provider" {
		profile = ""
	}

	resolvedIn := jobgen.Input{
		URLs: urls, FetchUse: fetchUse, Proxy: f.proxy, Provider: f.provider, Template: f.template,
		Profile: profile, Options: fetchOptions, Timeout: f.timeout, Stages: stages,
		Concurrency: f.concurrency, RPS: f.rps, Retries: f.retries, SinkUse: sinkUse, SinkPath: sinkPath,
	}
	defaultsIn := jobgen.Input{
		URLs: urls, FetchUse: "local", Profile: "chrome", Timeout: "30s",
		Concurrency: 8, Retries: 3, RPS: 0, SinkUse: "stdout",
	}

	rawField := ""
	if len(names) == 1 {
		rawField = names[0]
	}

	return printSpecOrRun(ctx, runner, resolvedIn, defaultsIn, f.printSpec, f.printSpecVerbose, f.bare, rawField, f.out, stdout, stderr)
}
