package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.Execute(); err != nil {
		if errors.Is(err, errArgsHandled) {
			return exitcode.UsageError
		}
		var cerr *cliError
		if errors.As(err, &cerr) {
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
	outputFormat     string
	printSpec        bool
	printSpecVerbose bool
	concurrency      int
	retries          int
	rps              int
}

func registerCommonFlags(cmd *cobra.Command, f *commonFlags, defaultRPS int) {
	flags := cmd.Flags()
	flags.StringVar(&f.proxy, "proxy", "", "fetch through this proxy URL (tier 2)")
	flags.StringVar(&f.provider, "provider", "", "fetch through this provider's manifest (tier 2)")
	flags.StringVar(&f.template, "template", "", "the provider template to use (default: the provider's own default template, if any)")
	flags.StringVar(&f.profile, "profile", "chrome", "local/proxy TLS fingerprint: chrome, firefox, or safari")
	flags.BoolVar(&f.render, "render", false, "sugar for --opt render=true")
	flags.StringVar(&f.geo, "geo", "", "sugar for --opt geo=<code>")
	flags.StringArrayVar(&f.opts, "opt", nil, "set a provider option as name=value (repeatable)")
	flags.StringVar(&f.timeout, "timeout", "30s", "per-request timeout")
	flags.StringVar(&f.extract, "extract", "", "add an extraction stage: raw or markdown")
	flags.StringArrayVar(&f.css, "css", nil, "add a css extraction field as name=selector[@attr] (repeatable)")
	flags.StringVar(&f.out, "out", "", "write NDJSON to this file instead of stdout")
	flags.StringVar(&f.outputFormat, "output-format", "json", "raw prints just the extracted value, no envelope")
	flags.BoolVar(&f.printSpec, "print-spec", false, "print the job spec, don't run it")
	flags.BoolVar(&f.printSpecVerbose, "print-spec-verbose", false, "like --print-spec, but every field is shown fully resolved")
	flags.IntVar(&f.concurrency, "concurrency", 8, "maximum concurrent fetches")
	flags.IntVar(&f.retries, "retries", 3, "retries per fetch before giving up")
	flags.IntVar(&f.rps, "rps", defaultRPS, "requests/sec across every host combined (0 = unlimited)")
}

func wireProviderHelp(cmd *cobra.Command, providers *providerator.Providerator, providerName *string) {
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, a []string) {
		defaultHelp(c, a)
		if *providerName == "" {
			return
		}
		printProviderHelp(c.OutOrStdout(), providers, *providerName)
	})
}

func printProviderHelp(w io.Writer, providers *providerator.Providerator, providerName string) {
	manifests, err := providers.Load()
	if err != nil {
		return
	}
	m, ok := providerator.Lookup(manifests, providerName)
	if !ok {
		return
	}

	fmt.Fprintln(w)
	if len(m.Templates) == 0 {
		fmt.Fprintf(w, "Provider %q\n\n", m.Name)
		for _, o := range m.Options {
			fmt.Fprintf(w, "  --opt %s\n", formatOptFlagLine(o))
		}
		return
	}

	defaultName, _ := resolvedTemplateName(m, "")
	fmt.Fprintf(w, "Provider %q — %d templates (default: %s). Pass --template to switch.\n\n", m.Name, len(m.Templates), defaultName)
	for i, t := range m.Templates {
		name := t.Name
		if t.Default {
			name += " (default)"
		}
		fmt.Fprintf(w, "  %s\n", name)
		for _, o := range t.Options {
			fmt.Fprintf(w, "    --opt %s\n", formatOptFlagLine(o))
		}
		if i < len(m.Templates)-1 {
			fmt.Fprintln(w)
		}
	}
}

func formatOptFlagLine(o provider.Option) string {
	typeDesc := o.Type
	if o.Type == "enum" {
		typeDesc = "enum: " + strings.Join(o.Values, "|")
	}
	head := fmt.Sprintf("%s=<%s>", o.Name, typeDesc)
	switch {
	case o.Default != nil:
		return fmt.Sprintf("%-30s default: %v", head, o.Default)
	case o.Pattern != "":
		return fmt.Sprintf("%-30s pattern: %s", head, o.Pattern)
	default:
		return head
	}
}

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

func buildErrorToCLIError(err error) *cliError {
	var verr *jobspec.ValidationError
	if errors.As(err, &verr) {
		return newCLIError(exitcode.UsageError, "spec: %s", verr.Message)
	}
	return newCLIError(exitcode.Fatal, "%v", err)
}

func printSpecOrRun(ctx context.Context, runner JobRunner, resolvedIn, defaultsIn jobgen.Input, printSpec, printSpecVerbose bool, outputFormat string, rawField string, stdout, stderr io.Writer) (int, *cliError) {
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

	if outputFormat == "raw" {
		return runJobRaw(ctx, runner, resolved, rawField, stdout, stderr)
	}
	return runJob(ctx, runner, resolved, stderr)
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

func printJSON(spec *jobspec.Spec, stdout io.Writer) (int, *cliError) {
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
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	fmt.Fprintln(stdout, string(data))
	return exitcode.Success, nil
}

func runJob(ctx context.Context, runner JobRunner, spec *jobspec.Spec, stderr io.Writer) (int, *cliError) {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	return execAndSummarize(ctx, runner, data, stderr)
}

func execAndSummarize(ctx context.Context, runner JobRunner, data []byte, stderr io.Writer) (int, *cliError) {
	result, kind, err := runner.Run(ctx, data, jobspec.ModePlain)
	if err != nil {
		if kind != "" {
			return 0, newCLIError(exitcode.FromValidationFault(kind), "%v", err)
		}
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	fmt.Fprintf(stderr, "fetched %d, errors %d, skipped %d\n", result.Succeeded, result.Failed, result.Skipped)
	return exitcode.FromRun(result.Succeeded, result.Failed, result.Kinds), nil
}

type rawItem struct {
	Data map[string]any `json:"data"`
}

func runJobRaw(ctx context.Context, runner JobRunner, spec *jobspec.Spec, fieldName string, stdout, stderr io.Writer) (int, *cliError) {
	tmpDir, err := os.MkdirTemp("", "pobear-raw-*")
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, "out.ndjson")

	raw := *spec
	raw.Sink = jobspec.Sink{Use: "ndjson", Path: tmpPath}

	data, err := yaml.Marshal(&raw)
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}

	code, cerr := execAndSummarize(ctx, runner, data, stderr)
	if cerr != nil {
		return 0, cerr
	}
	if code != exitcode.Success {
		return code, nil
	}

	if err := writeRawValue(tmpPath, fieldName, stdout); err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	return code, nil
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
