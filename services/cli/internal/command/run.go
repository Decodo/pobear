package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
)

type JobSpecParser interface {
	Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error)
}

type Run struct {
	Parser JobSpecParser
	Runner JobRunner
}

func (c *Run) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	target, rest, hasTarget := splitPositional(args)
	if !hasTarget {
		fmt.Fprintln(stderr, "pobear: run requires a job spec path")
		return exitcode.UsageError
	}

	var printSpec, printSpecVerbose bool
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&printSpec, "print-spec", false, "")
	fs.BoolVar(&printSpecVerbose, "print-spec-verbose", false, "")

	if cerr := checkKnownFlags(fs, rest); cerr != nil {
		cerr.print(stderr)
		return cerr.code
	}
	if err := fs.Parse(rest); err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.UsageError
	}

	var data []byte
	var err error
	if target == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(target) //nolint:gosec // target is the CLI's own positional argument, this command's documented file-path contract
	}
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.Fatal
	}

	spec, err := c.Parser.Parse(data, jobspec.ModePlain)
	if err != nil {
		return handleBuildError(err, stderr)
	}

	if printSpecVerbose {
		return printJSON(spec, stdout, stderr)
	}
	if printSpec {
		out, err := jobgen.MinimalJSON(spec, runDefaultsSpec(spec))
		if err != nil {
			fmt.Fprintf(stderr, "pobear: %v\n", err)
			return exitcode.Fatal
		}
		fmt.Fprintln(stdout, string(out))
		return exitcode.Success
	}

	return execAndSummarize(ctx, c.Runner, data, stderr)
}

func runDefaultsSpec(spec *jobspec.Spec) *jobspec.Spec {
	d := &jobspec.Spec{
		Version: spec.Version,
		Source:  jobspec.Source{URLs: spec.Source.URLs},
		Fetch:   jobspec.Fetch{Use: "local"},
		Limits:  jobspec.Limits{Concurrency: 8, Retries: 3, Backoff: "exponential", PluginTimeout: "30s"},
		Sink:    jobspec.Sink{Use: "stdout"},
	}
	if spec.Source.Crawl != nil {
		d.Source = jobspec.Source{Crawl: &jobspec.CrawlSource{
			Seeds: spec.Source.Crawl.Seeds, Scope: "same-host", Robots: "respect", MaxPages: 100,
		}}
	}
	return d
}
