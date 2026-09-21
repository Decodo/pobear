package command

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pobear/pobear/pkg/exitcode"
)

type cliError struct {
	code    int
	message string
}

func newCLIError(code int, format string, args ...any) *cliError {
	return &cliError{code: code, message: fmt.Sprintf(format, args...)}
}

func (e *cliError) Error() string { return e.message }

func (e *cliError) print(stderr io.Writer) {
	fmt.Fprintf(stderr, "pobear: %s\n", e.message)
}

var errArgsHandled = errors.New("command: argument error already reported")

func requireExactlyOne(usageNoun string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return nil
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "pobear: %s\n\n%s", usageNoun, cmd.UsageString())
		return errArgsHandled
	}
}

func runCobraCommand(cmd *cobra.Command, args []string, stdin io.Reader, stdout, stderr io.Writer, code *int) int {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetUsageTemplate(strings.Replace(cmd.UsageTemplate(), "{{.UseLine}}", "pobear {{.UseLine}}", 1))
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.Execute(); err != nil {
		if errors.Is(err, errArgsHandled) {
			return exitcode.UsageError
		}
		if cerr, ok := errors.AsType[*cliError](err); ok {
			cerr.print(stderr)
			return cerr.code
		}
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.UsageError
	}
	return *code
}

type commonFlags struct {
	proxy            string
	provider         string
	template         string
	profile          string
	render           bool
	geo              string
	opts             []string
	timeout          string
	extract          string
	css              []string
	out              string
	bare             bool
	ladder           bool
	printSpec        bool
	printSpecVerbose bool
	failOnEmpty      bool
	concurrency      int
	retries          int
	rps              int
	wasSet           func(name string) bool
}

func registerCommonFlags(cmd *cobra.Command, f *commonFlags, defaultRPS int) {
	flags := cmd.Flags()
	f.wasSet = flags.Changed
	flags.StringVar(&f.proxy, "proxy", "", "fetch through this proxy URL (tier 2)")
	flags.StringVar(&f.provider, "provider", "", "fetch through this provider's manifest (tier 3)")
	flags.StringVar(&f.template, "template", "", "the provider template to use (default: the provider's own default template, if any)")
	flags.StringVar(&f.profile, "profile", "chrome", "local/proxy TLS fingerprint: chrome, firefox, or safari")
	flags.BoolVar(&f.render, "render", false, "sugar for --opt render=true")
	flags.StringVar(&f.geo, "geo", "", "sugar for --opt geo=<code>")
	flags.StringArrayVar(&f.opts, "opt", nil, "set a provider option as name=value (repeatable)")
	flags.StringVarP(&f.timeout, "timeout", "t", "30s", "per-request timeout")
	flags.StringVarP(&f.extract, "extract", "e", "", "add an extraction stage: raw or markdown")
	flags.StringArrayVar(&f.css, "css", nil, "add a css extraction field as name=selector[@attr] (repeatable)")
	flags.StringVarP(&f.out, "out", "o", "", "write NDJSON to this file instead of stdout, or one file per target plus an index.json if this names a directory")
	flags.BoolVarP(&f.bare, "bare", "b", false, "print just the extracted value, no envelope -- requires at most one field from --extract/--css (falls back to the raw fetched body if neither is set)")
	flags.BoolVar(&f.ladder, "ladder", false, "escalate through configured tiers (local, then proxy, then provider) when blocked or rate-limited")
	flags.BoolVar(&f.printSpec, "print-spec", false, "print the job spec, don't run it")
	flags.BoolVar(&f.printSpecVerbose, "print-spec-verbose", false, "like --print-spec, but every field is shown fully resolved")
	flags.BoolVar(&f.failOnEmpty, "fail-on-empty", false, "exit non-zero when every fetched item extracted nothing")
	flags.IntVarP(&f.concurrency, "concurrency", "c", 8, "maximum concurrent fetches")
	flags.IntVarP(&f.retries, "retries", "r", 3, "retries per fetch before giving up")
	flags.IntVar(&f.rps, "rps", defaultRPS, "requests/sec across every host combined (0 = unlimited)")
}

func resolveFetchUse(f *commonFlags) (string, *cliError) {
	if f.proxy != "" && f.provider != "" && !f.ladder {
		return "", newCLIError(exitcode.UsageError, "both --proxy and --provider are set -- add --ladder to escalate between them, or remove one")
	}
	if f.ladder {
		return "local", nil
	}

	fetchUse := "local"
	switch {
	case f.proxy != "":
		fetchUse = "proxy"
	case f.provider != "":
		fetchUse = "provider"
	}
	return fetchUse, nil
}

func resolveProfile(f *commonFlags, fetchUse string) (string, *cliError) {
	if fetchUse != "provider" {
		return f.profile, nil
	}
	if f.wasSet != nil && f.wasSet("profile") {
		return "", newCLIError(exitcode.UsageError, "--profile requires --proxy or the local tier, not --provider")
	}
	return "", nil
}
