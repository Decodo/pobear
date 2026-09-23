package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/pkg/utils"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
)

func mergeOptions(render bool, geo string, opts []string) map[string]string {
	m := map[string]string{}
	if render {
		m["render"] = "true"
	}
	if geo != "" {
		m["geo"] = geo
	}
	for _, kv := range opts {
		name, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		m[name] = val
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func stringMapToAny(m map[string]string) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func buildStages(extract string, css []string) ([]jobspec.Stage, *cliError) {
	var stages []jobspec.Stage
	if extract != "" {
		stages = append(stages, jobspec.Stage{Use: extract})
	}
	if len(css) > 0 {
		fields := make(map[string]string, len(css))
		for _, kv := range css {
			name, sel, ok := strings.Cut(kv, "=")
			if !ok {
				return nil, newCLIError(exitcode.UsageError, "invalid --css %q, want name=selector", kv)
			}
			if name == "" {
				return nil, newCLIError(exitcode.UsageError, "invalid --css %q, field name is empty", kv)
			}
			if _, dup := fields[name]; dup {
				return nil, newCLIError(exitcode.UsageError, "--css field %q is set more than once", name)
			}
			fields[name] = sel
		}
		stages = append(stages, jobspec.Stage{Use: "css", Fields: fields})
	}
	return stages, nil
}

func extractedFieldNames(extract string, css []string) []string {
	var names []string
	if extract != "" {
		names = append(names, extract)
	}
	for _, kv := range css {
		name, _, _ := strings.Cut(kv, "=")
		names = append(names, name)
	}
	return names
}

func validateBare(bare bool, names []string) *cliError {
	if !bare || len(names) <= 1 {
		return nil
	}
	return newCLIError(exitcode.UsageError, "--bare requires at most one extracted value, but %d fields are set (%s)", len(names), strings.Join(names, ", "))
}

func validateBareSingleTarget(bare, dirOut bool, urlCount int) *cliError {
	if !bare || dirOut || urlCount <= 1 {
		return nil
	}
	return newCLIError(exitcode.UsageError, "--bare requires exactly one URL to be targeted, got %d", urlCount)
}

func validateBareCrawlDepth(bare, dirOut bool, depth int) *cliError {
	if !bare || dirOut || depth == 0 {
		return nil
	}
	return newCLIError(exitcode.UsageError, "--bare requires --depth 0 -- crawling deeper can discover more than one page, and --bare only ever prints a single value (use --out <dir> to write one file per page instead)")
}

func resolveScrapeURLs(arg string) ([]string, *cliError) {
	if utils.IsAbsoluteHTTPURL(arg) {
		return []string{arg}, nil
	}
	data, err := os.ReadFile(arg) //nolint:gosec // arg is the CLI's own positional argument, this command's documented file-path contract
	if err != nil {
		return nil, newCLIError(exitcode.UsageError, "%v", err)
	}
	var urls []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !utils.IsAbsoluteHTTPURL(line) {
			return nil, newCLIError(exitcode.UsageError, "%s: not an absolute http/https URL: %q", arg, line)
		}
		urls = append(urls, line)
	}
	if len(urls) == 0 {
		return nil, newCLIError(exitcode.UsageError, "%s: no URLs found", arg)
	}
	return urls, nil
}

func buildErrorToCLIError(err error) *cliError {
	if verr, ok := errors.AsType[*jobspec.ValidationError](err); ok {
		return newCLIError(exitcode.UsageError, "spec: %s", verr.Message)
	}
	return newCLIError(exitcode.Fatal, "%v", err)
}

func printSpecOrRun(ctx context.Context, runner JobRunner, resolvedIn, defaultsIn jobgen.Input, printSpec, printSpecVerbose, bare, failOnEmpty bool, rawField, outPath string, stdout, stderr io.Writer) (int, *cliError) {
	resolved, err := jobgen.Build(resolvedIn)
	if err != nil {
		return 0, buildErrorToCLIError(err)
	}

	if printSpecVerbose {
		return printJSON(resolved, stdout)
	}
	if printSpec {
		defaults, err := jobgen.Build(defaultsIn)
		if err != nil {
			return 0, buildErrorToCLIError(err)
		}
		data, err := jobgen.MinimalJSON(resolved, defaults)
		if err != nil {
			return 0, newCLIError(exitcode.Fatal, "%v", err)
		}
		fmt.Fprintln(stdout, string(data))
		return exitcode.Success, nil
	}

	if isDirOut(outPath) {
		return runJobToDir(ctx, runner, resolved, bare, rawField, outPath, stderr)
	}
	if bare {
		return runJobRaw(ctx, runner, resolved, rawField, outPath, stdout, stderr)
	}
	return runJob(ctx, runner, resolved, failOnEmpty, stderr)
}

type verboseFetch struct {
	Use         string            `json:"use"`
	Ladder      bool              `json:"ladder"`
	Name        string            `json:"name,omitempty"`
	Template    string            `json:"template,omitempty"`
	Proxy       string            `json:"proxy,omitempty"`
	Profile     string            `json:"profile,omitempty"`
	Options     map[string]any    `json:"options,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	NoRedirects bool              `json:"noRedirects"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type verboseSpec struct {
	Version int             `json:"version"`
	Source  jobspec.Source  `json:"source"`
	Fetch   verboseFetch    `json:"fetch"`
	Limits  jobspec.Limits  `json:"limits"`
	Stages  []jobspec.Stage `json:"stages"`
	Sink    jobspec.Sink    `json:"sink"`
}

func printJSON(spec *jobspec.Spec, stdout io.Writer) (int, *cliError) {
	v := verboseSpec{
		Version: spec.Version,
		Source:  spec.Source,
		Fetch: verboseFetch{
			Use: spec.Fetch.Use, Ladder: spec.Fetch.Ladder, Name: spec.Fetch.Name, Template: spec.Fetch.Template,
			Proxy: spec.Fetch.Proxy, Profile: spec.Fetch.Profile, Options: spec.Fetch.Options,
			Timeout: spec.Fetch.Timeout, NoRedirects: spec.Fetch.NoRedirects, Headers: spec.Fetch.Headers,
		},
		Limits: spec.Limits,
		Stages: spec.Stages,
		Sink:   spec.Sink,
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	fmt.Fprintln(stdout, string(data))
	return exitcode.Success, nil
}

func runJob(ctx context.Context, runner JobRunner, spec *jobspec.Spec, failOnEmpty bool, stderr io.Writer) (int, *cliError) {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	code, result, cerr := execAndSummarize(ctx, runner, data, stderr)
	if cerr != nil {
		return code, cerr
	}
	if failOnEmpty && result.Succeeded > 0 && result.Empty == result.Succeeded {
		return 0, newCLIError(exitcode.Fatal, "every fetched item extracted nothing (%d items)", result.Empty)
	}
	return code, cerr
}

func execAndSummarize(ctx context.Context, runner JobRunner, data []byte, stderr io.Writer) (int, RunResult, *cliError) {
	result, kind, err := runner.Run(ctx, data, jobspec.ModePlain)
	if err != nil {
		if kind != "" {
			return 0, RunResult{}, newCLIError(exitcode.FromValidationFault(kind), "%v", err)
		}
		return 0, RunResult{}, newCLIError(exitcode.Fatal, "%v", err)
	}
	for _, e := range result.Errors {
		fmt.Fprintln(stderr, e)
	}
	summary := fmt.Sprintf("fetched %d, errors %d, skipped %d", result.Succeeded, result.Failed, result.Skipped)
	if result.Empty > 0 {
		summary += fmt.Sprintf(", empty %d", result.Empty)
	}
	fmt.Fprintln(stderr, summary)
	return exitcode.FromRun(result.Succeeded, result.Failed, result.Kinds), result, nil
}
