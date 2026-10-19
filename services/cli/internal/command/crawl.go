package command

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/Decodo/pobear/services/cli/internal/jobgen"
	"github.com/Decodo/pobear/services/cli/internal/providerator"
)

type Crawl struct {
	Runner          JobRunner
	Providers       *providerator.Providerator
	CredentialsPath string
}

func (c *Crawl) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var f commonFlags
	var depth, maxPages int
	var maxDuration, scope string
	var include, exclude []string
	var sitemap, ignoreRobots, noRedirects bool
	code := 0

	cmd := &cobra.Command{
		Use:   "crawl <url>",
		Short: (&Crawl{}).Summary(),
		Long: "Crawl a site starting from one URL, following links up to --depth.\n\n" +
			"Takes exactly one starting URL -- there's no flag to add extra seeds; a\n" +
			"crawl with multiple seeds is only reachable by writing the job spec\n" +
			"directly and running it with 'pobear run'.",
		Args: requireExactlyOne("crawl requires a URL argument"),
		RunE: func(_ *cobra.Command, cmdArgs []string) error {
			robots := "respect"
			if ignoreRobots {
				robots = "ignore"
			}
			crawlIn := &jobgen.CrawlInput{
				Depth: depth, MaxPages: &maxPages, MaxDuration: maxDuration, Scope: scope,
				Include: include, Exclude: exclude, Sitemap: sitemap, Robots: robots,
			}
			cc, cerr := crawlRun(ctx, c.Runner, c.Providers, c.CredentialsPath, cmdArgs[0], &f, crawlIn, noRedirects, stdout, stderr)
			code = cc
			if cerr != nil {
				return cerr
			}
			return nil
		},
	}
	registerCommonFlags(cmd, &f, 16)
	flags := cmd.Flags()
	flags.IntVarP(&depth, "depth", "d", 1, "link-following depth; 0 = seeds only")
	flags.IntVar(&maxPages, "max-pages", 100, "stop after fetching this many pages")
	flags.StringVar(&maxDuration, "max-duration", "", "stop the whole crawl after this long, even if max-pages isn't reached")
	flags.StringVarP(&scope, "scope", "s", "same-host", "same-host, same-domain, or any")
	flags.StringArrayVarP(&include, "include", "i", nil, "only follow links matching this glob (repeatable)")
	flags.StringArrayVar(&exclude, "exclude", nil, "never follow links matching this glob (repeatable); wins over --include")
	flags.BoolVar(&sitemap, "sitemap", true, "also seed from /sitemap.xml")
	flags.BoolVar(&ignoreRobots, "ignore-robots", false, "robots.txt is respected by default")
	flags.BoolVar(&noRedirects, "no-redirects", false, "treat a 3xx response as a dead end instead of following it")
	wireProviderHelp(cmd, c.Providers, &f.provider)

	return runCobraCommand(cmd, args, stdin, stdout, stderr, &code)
}

func crawlRun(ctx context.Context, runner JobRunner, providers *providerator.Providerator, credentialsPath, target string, f *commonFlags, crawlIn *jobgen.CrawlInput, noRedirects bool, stdout, stderr io.Writer) (int, *cliError) {
	stages, cerr := buildStages(f.extract, f.css)
	if cerr != nil {
		return 0, cerr
	}

	names := extractedFieldNames(f.extract, f.css)
	if cerr := validateBare(f.bare, names); cerr != nil {
		return 0, cerr
	}
	if cerr := validateBareCrawlDepth(f.bare, isDirOut(f.out), crawlIn.Depth); cerr != nil {
		return 0, cerr
	}

	fetchUse, cerr := resolveFetchUse(f)
	if cerr != nil {
		return 0, cerr
	}

	opts := mergeOptions(f.render, f.geo, f.opts)
	var fetchOptions map[string]any
	if f.provider != "" {
		var perr *cliError
		fetchOptions, perr = resolveProviderOptions(providers, credentialsPath, f.provider, f.template, opts, !f.printSpec && !f.printSpecVerbose)
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

	profile, cerr := resolveProfile(f, fetchUse)
	if cerr != nil {
		return 0, cerr
	}

	headers, cerr := buildHeaders(f.headers)
	if cerr != nil {
		return 0, cerr
	}

	resolvedIn := jobgen.Input{
		URLs: []string{target}, Crawl: crawlIn, FetchUse: fetchUse, Ladder: f.ladder, Proxy: f.proxy, Provider: f.provider, Template: f.template,
		Profile: profile, Options: fetchOptions, Timeout: f.timeout, NoRedirects: noRedirects, Headers: headers, Stages: stages,
		Concurrency: &f.concurrency, RPS: &f.rps, Retries: &f.retries, SinkUse: sinkUse, SinkPath: sinkPath,
	}
	defaultsIn := jobgen.Input{
		URLs: []string{target}, FetchUse: "local", Crawl: &jobgen.CrawlInput{},
	}

	rawField := ""
	if len(names) == 1 {
		rawField = names[0]
	}

	return printSpecOrRun(ctx, runner, resolvedIn, defaultsIn, f.printSpec, f.printSpecVerbose, f.bare, f.failOnEmpty, rawField, f.out, stdout, stderr)
}

func (*Crawl) Summary() string { return "Crawl a site starting from one URL" }
