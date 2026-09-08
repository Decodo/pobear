package command

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
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
	Runner          JobRunner
	Providers       *providerator.Providerator
	CredentialsPath string
}

func (s *Scrape) Run(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	target, rest, hasTarget := splitPositional(args)

	var f commonFlags
	fs := flag.NewFlagSet("scrape", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerCommonFlags(fs, &f, 0)

	if cerr := checkKnownFlags(fs, rest); cerr != nil {
		cerr.print(stderr)
		return cerr.code
	}
	if err := fs.Parse(rest); err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.UsageError
	}
	if !hasTarget {
		fmt.Fprintln(stderr, "pobear: scrape requires a URL or file argument")
		return exitcode.UsageError
	}

	urls, cerr := resolveScrapeURLs(target)
	if cerr != nil {
		cerr.print(stderr)
		return cerr.code
	}

	stages, cerr := buildStages(f.extract, f.css)
	if cerr != nil {
		cerr.print(stderr)
		return cerr.code
	}

	names := extractedFieldNames(f.extract, f.css)
	if cerr := validateOutputFormatRaw(f.outputFormat, names); cerr != nil {
		cerr.print(stderr)
		return cerr.code
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
		fetchOptions, perr = resolveProviderOptions(s.Providers, s.CredentialsPath, f.provider, f.template, opts)
		if perr != nil {
			perr.print(stderr)
			return perr.code
		}
	} else {
		fetchOptions = stringMapToAny(opts)
	}

	sinkUse, sinkPath := "stdout", ""
	if f.out != "" {
		sinkUse, sinkPath = "ndjson", f.out
	}

	resolvedIn := jobgen.Input{
		URLs: urls, FetchUse: fetchUse, Proxy: f.proxy, Provider: f.provider, Template: f.template,
		Profile: f.profile, Options: fetchOptions, Timeout: f.timeout, Stages: stages,
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
	return printSpecOrRun(ctx, s.Runner, resolvedIn, defaultsIn, f.printSpec, f.printSpecVerbose, f.outputFormat, rawField, stdout, stderr)
}
