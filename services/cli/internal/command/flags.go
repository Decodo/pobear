package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/pkg/utils"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type cliError struct {
	code    int
	message string
}

func newCLIError(code int, format string, args ...any) *cliError {
	return &cliError{code: code, message: fmt.Sprintf(format, args...)}
}

func (e *cliError) print(stderr io.Writer) {
	fmt.Fprintf(stderr, "pobear: %s\n", e.message)
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }

func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type commonFlags struct {
	proxy            string
	provider         string
	template         string
	profile          string
	render           bool
	geo              string
	opts             stringSliceFlag
	timeout          string
	extract          string
	css              stringSliceFlag
	out              string
	outputFormat     string
	printSpec        bool
	printSpecVerbose bool
	concurrency      int
	retries          int
	rps              int
}

func registerCommonFlags(fs *flag.FlagSet, f *commonFlags, defaultRPS int) {
	fs.StringVar(&f.proxy, "proxy", "", "")
	fs.StringVar(&f.provider, "provider", "", "")
	fs.StringVar(&f.template, "template", "", "")
	fs.StringVar(&f.profile, "profile", "chrome", "")
	fs.BoolVar(&f.render, "render", false, "")
	fs.StringVar(&f.geo, "geo", "", "")
	fs.Var(&f.opts, "opt", "")
	fs.StringVar(&f.timeout, "timeout", "30s", "")
	fs.StringVar(&f.extract, "extract", "", "")
	fs.Var(&f.css, "css", "")
	fs.StringVar(&f.out, "out", "", "")
	fs.StringVar(&f.outputFormat, "output-format", "json", "")
	fs.BoolVar(&f.printSpec, "print-spec", false, "")
	fs.BoolVar(&f.printSpecVerbose, "print-spec-verbose", false, "")
	fs.IntVar(&f.concurrency, "concurrency", 8, "")
	fs.IntVar(&f.retries, "retries", 3, "")
	fs.IntVar(&f.rps, "rps", defaultRPS, "")
}

func splitPositional(args []string) (positional string, rest []string, ok bool) {
	if len(args) > 0 && (args[0] == "-" || !strings.HasPrefix(args[0], "-")) {
		return args[0], args[1:], true
	}
	return "", args, false
}

func checkKnownFlags(fs *flag.FlagSet, args []string) *cliError {
	known := map[string]bool{}
	fs.VisitAll(func(fl *flag.Flag) { known[fl.Name] = true })
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if !known[name] {
			return newCLIError(exitcode.UsageError, "unknown flag: %s", a)
		}
	}
	return nil
}

func mergeOptions(render bool, geo string, opts stringSliceFlag) map[string]string {
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

func buildStages(extract string, css stringSliceFlag) ([]jobspec.Stage, *cliError) {
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
			fields[name] = sel
		}
		stages = append(stages, jobspec.Stage{Use: "css", Fields: fields})
	}
	return stages, nil
}

func extractedFieldNames(extract string, css stringSliceFlag) []string {
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

func validateOutputFormatRaw(outputFormat string, names []string) *cliError {
	if outputFormat != "raw" {
		return nil
	}
	if len(names) == 1 {
		return nil
	}
	if len(names) == 0 {
		return newCLIError(exitcode.UsageError, "--output-format raw requires exactly one extracted value, but 0 fields are set")
	}
	return newCLIError(exitcode.UsageError, "--output-format raw requires exactly one extracted value, but %d fields are set (%s)", len(names), strings.Join(names, ", "))
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

func resolvedTemplateName(m *provider.Manifest, templateName string) (name string, hasTemplates bool) {
	if len(m.Templates) == 0 {
		return "", false
	}
	if templateName != "" {
		return templateName, true
	}
	if len(m.Templates) == 1 {
		return m.Templates[0].Name, true
	}
	for _, t := range m.Templates {
		if t.Default {
			return t.Name, true
		}
	}
	return "", true
}

func suggestTemplateFor(m *provider.Manifest, optName, excludeName string) (string, bool) {
	for _, t := range m.Templates {
		if t.Name == excludeName {
			continue
		}
		for _, opt := range t.Options {
			if opt.Name == optName {
				return t.Name, true
			}
		}
	}
	return "", false
}

func findUnknownOption(declared []provider.Option, opts map[string]string) (string, bool) {
	byName := make(map[string]bool, len(declared))
	for _, o := range declared {
		byName[o.Name] = true
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	for _, k := range keys {
		if !byName[k] {
			return k, true
		}
	}
	return "", false
}

func resolveProviderOptions(providers *providerator.Providerator, credentialsPath, providerName, templateName string, opts map[string]string) (map[string]any, *cliError) {
	manifests, err := providers.Load()
	if err != nil {
		return nil, newCLIError(exitcode.Fatal, "%v", err)
	}
	m, ok := providerator.Lookup(manifests, providerName)
	if !ok {
		return nil, newCLIError(exitcode.UsageError, "unknown provider %q", providerName)
	}

	declared, rerr := providerator.ResolveTemplate(m, templateName)
	if rerr != nil {
		return nil, newCLIError(exitcode.UsageError, "%v", rerr)
	}

	if badName, isBad := findUnknownOption(declared, opts); isBad {
		resolvedName, hasTemplates := resolvedTemplateName(m, templateName)
		if hasTemplates {
			if suggestion, found := suggestTemplateFor(m, badName, resolvedName); found {
				return nil, newCLIError(exitcode.UsageError, "%s template %q has no option %q (did you mean --template %s?)", m.Name, resolvedName, badName, suggestion)
			}
		}
		return nil, newCLIError(exitcode.UsageError, "%s has no option %q", m.Name, badName)
	}

	validated, verr := providerator.ValidateOptions(declared, opts)
	if verr != nil {
		return nil, newCLIError(exitcode.UsageError, "%v", verr)
	}

	hasCred, cerr := providerator.CheckCredential(m, credentialsPath)
	if cerr != nil {
		return nil, newCLIError(exitcode.Fatal, "%v", cerr)
	}
	if !hasCred {
		return nil, newCLIError(exitcode.AuthFailure, "missing credential for provider %q (run: pobear auth %s)", m.Name, m.Name)
	}

	return validated, nil
}

func handleBuildError(err error, stderr io.Writer) int {
	var verr *jobspec.ValidationError
	if errors.As(err, &verr) {
		fmt.Fprintf(stderr, "pobear: spec: %s\n", verr.Message)
		return exitcode.UsageError
	}
	fmt.Fprintf(stderr, "pobear: %v\n", err)
	return exitcode.Fatal
}

func printSpecOrRun(ctx context.Context, runner JobRunner, resolvedIn, defaultsIn jobgen.Input, printSpec, printSpecVerbose bool, outputFormat string, rawField string, stdout, stderr io.Writer) int {
	resolved, err := jobgen.Build(resolvedIn)
	if err != nil {
		return handleBuildError(err, stderr)
	}

	if printSpecVerbose {
		return printJSON(resolved, stdout, stderr)
	}
	if printSpec {
		defaults, err := jobgen.Build(defaultsIn)
		if err != nil {
			return handleBuildError(err, stderr)
		}
		data, err := jobgen.MinimalJSON(resolved, defaults)
		if err != nil {
			fmt.Fprintf(stderr, "pobear: %v\n", err)
			return exitcode.Fatal
		}
		fmt.Fprintln(stdout, string(data))
		return exitcode.Success
	}

	if outputFormat == "raw" {
		return runJobRaw(ctx, runner, resolved, rawField, stdout, stderr)
	}
	return runJob(ctx, runner, resolved, stdout, stderr)
}

type verboseFetch struct {
	Use         string            `json:"use"`
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

func printJSON(spec *jobspec.Spec, stdout, stderr io.Writer) int {
	v := verboseSpec{
		Version: spec.Version,
		Source:  spec.Source,
		Fetch: verboseFetch{
			Use: spec.Fetch.Use, Name: spec.Fetch.Name, Template: spec.Fetch.Template,
			Proxy: spec.Fetch.Proxy, Profile: spec.Fetch.Profile, Options: spec.Fetch.Options,
			Timeout: spec.Fetch.Timeout, NoRedirects: spec.Fetch.NoRedirects, Headers: spec.Fetch.Headers,
		},
		Limits: spec.Limits,
		Stages: spec.Stages,
		Sink:   spec.Sink,
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.Fatal
	}
	fmt.Fprintln(stdout, string(data))
	return exitcode.Success
}

func runJob(ctx context.Context, runner JobRunner, spec *jobspec.Spec, stdout, stderr io.Writer) int {
	_ = stdout
	data, err := yaml.Marshal(spec)
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.Fatal
	}
	return execAndSummarize(ctx, runner, data, stderr)
}

func execAndSummarize(ctx context.Context, runner JobRunner, data []byte, stderr io.Writer) int {
	result, kind, err := runner.Run(ctx, data, jobspec.ModePlain)
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		if kind != "" {
			return exitcode.FromValidationFault(kind)
		}
		return exitcode.Fatal
	}
	fmt.Fprintf(stderr, "fetched %d, errors %d, skipped %d\n", result.Succeeded, result.Failed, result.Skipped)
	return exitcode.FromRun(result.Succeeded, result.Failed, result.Kinds)
}

type rawItem struct {
	Data map[string]any `json:"data"`
}

func runJobRaw(ctx context.Context, runner JobRunner, spec *jobspec.Spec, fieldName string, stdout, stderr io.Writer) int {
	tmpDir, err := os.MkdirTemp("", "pobear-raw-*")
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.Fatal
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, "out.ndjson")

	raw := *spec
	raw.Sink = jobspec.Sink{Use: "ndjson", Path: tmpPath}

	data, err := yaml.Marshal(&raw)
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.Fatal
	}

	code := execAndSummarize(ctx, runner, data, stderr)
	if code != exitcode.Success {
		return code
	}

	if err := writeRawValue(tmpPath, fieldName, stdout); err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.Fatal
	}
	return code
}

func writeRawValue(path, fieldName string, stdout io.Writer) error {
	data, err := os.ReadFile(path) //nolint:gosec // path is this command's own os.MkdirTemp file, not user-controlled input
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 || lines[0] == "" {
		return fmt.Errorf("--output-format raw requires exactly one URL to be targeted, got %d items", len(lines))
	}
	var item rawItem
	if err := json.Unmarshal([]byte(lines[0]), &item); err != nil {
		return err
	}
	v, ok := item.Data[fieldName]
	if !ok {
		return fmt.Errorf("--output-format raw: field %q was not produced", fieldName)
	}
	switch val := v.(type) {
	case []any:
		if len(val) != 1 {
			return fmt.Errorf("--output-format raw: field %q matched %d values, want exactly 1", fieldName, len(val))
		}
		fmt.Fprint(stdout, fmt.Sprint(val[0]))
	default:
		fmt.Fprint(stdout, fmt.Sprint(val))
	}
	return nil
}
