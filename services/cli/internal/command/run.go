package command

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/Decodo/pobear/pkg/exitcode"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/cli/internal/jobgen"
)

type JobSpecParser interface {
	Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error)
}

type Run struct {
	Parser JobSpecParser
	Runner JobRunner
}

func (c *Run) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var printSpec, printSpecVerbose bool
	code := 0

	cmd := &cobra.Command{
		Use:   "run <job.yaml|->",
		Short: (&Run{}).Summary(),
		Long: "Run a job spec file, or read it from stdin with '-'.\n\n" +
			"Since '-' reads from stdin, the output of --print-spec from any other\n" +
			"command is itself a valid input to run.",
		Args: requireExactlyOne("run requires a job spec path"),
		RunE: func(_ *cobra.Command, cmdArgs []string) error {
			cc, cerr := c.run(ctx, cmdArgs[0], printSpec, printSpecVerbose, stdin, stdout, stderr)
			code = cc
			if cerr != nil {
				return cerr
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&printSpec, "print-spec", false, "print the parsed spec, don't run it")
	flags.BoolVar(&printSpecVerbose, "print-spec-verbose", false, "like --print-spec, but every field is shown fully resolved")

	return runCobraCommand(cmd, args, stdin, stdout, stderr, &code)
}

func (c *Run) run(ctx context.Context, target string, printSpec, printSpecVerbose bool, stdin io.Reader, stdout, stderr io.Writer) (int, *cliError) {
	var data []byte
	var err error
	if target == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(target) //nolint:gosec // target is the CLI's own positional argument, this command's documented file-path contract
	}
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}

	spec, err := c.Parser.Parse(data, jobspec.ModePlain)
	if err != nil {
		return 0, buildErrorToCLIError(err)
	}

	if printSpecVerbose {
		return printJSON(spec, stdout)
	}
	if printSpec {
		out, err := jobgen.MinimalJSON(spec, runDefaultsSpec(spec))
		if err != nil {
			return 0, newCLIError(exitcode.Fatal, "%v", err)
		}
		fmt.Fprintln(stdout, string(out))
		return exitcode.Success, nil
	}

	code, _, cerr := execAndSummarize(ctx, c.Runner, data, stderr)
	return code, cerr
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

func (*Run) Summary() string { return "Run a job spec file" }
