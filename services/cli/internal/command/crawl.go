package command

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type Crawl struct {
	Runner          JobRunner
	Providers       *providerator.Providerator
	CredentialsPath string
}

func (c *Crawl) Run(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	target, rest, hasTarget := splitPositional(args)

	var f commonFlags
	var depth, maxPages int
	var maxDuration, scope, robots string
	var include, exclude stringSliceFlag
	var sitemap, ignoreRobots, noRedirects bool

	fs := flag.NewFlagSet("crawl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerCommonFlags(fs, &f, 2)
	fs.IntVar(&depth, "depth", 1, "")
	fs.IntVar(&maxPages, "max-pages", 100, "")
	fs.StringVar(&maxDuration, "max-duration", "", "")
	fs.StringVar(&scope, "scope", "same-host", "")
	fs.Var(&include, "include", "")
	fs.Var(&exclude, "exclude", "")
	fs.BoolVar(&sitemap, "sitemap", false, "")
	fs.BoolVar(&ignoreRobots, "ignore-robots", false, "")
	fs.BoolVar(&noRedirects, "no-redirects", false, "")

	if cerr := checkKnownFlags(fs, rest); cerr != nil {
		cerr.print(stderr)
		return cerr.code
	}
	if err := fs.Parse(rest); err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.UsageError
	}
	if !hasTarget {
		fmt.Fprintln(stderr, "pobear: crawl requires a URL argument")
		return exitcode.UsageError
	}

	robots = "respect"
	if ignoreRobots {
		robots = "ignore"
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
		fetchOptions, perr = resolveProviderOptions(c.Providers, c.CredentialsPath, f.provider, f.template, opts)
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

	crawlIn := &jobgen.CrawlInput{
		Depth: depth, MaxPages: maxPages, MaxDuration: maxDuration, Scope: scope,
		Include: include, Exclude: exclude, Sitemap: sitemap, Robots: robots,
	}
	resolvedIn := jobgen.Input{
		URLs: []string{target}, Crawl: crawlIn, FetchUse: fetchUse, Proxy: f.proxy, Provider: f.provider, Template: f.template,
		Profile: f.profile, Options: fetchOptions, Timeout: f.timeout, NoRedirects: noRedirects, Stages: stages,
		Concurrency: f.concurrency, RPS: f.rps, Retries: f.retries, SinkUse: sinkUse, SinkPath: sinkPath,
	}
	defaultsIn := jobgen.Input{
		URLs: []string{target}, FetchUse: "local", Profile: "chrome", Timeout: "30s",
		Crawl:       &jobgen.CrawlInput{Depth: 1, MaxPages: 100, Scope: "same-host", Robots: "respect"},
		Concurrency: 8, Retries: 3, RPS: 2, SinkUse: "stdout",
	}

	rawField := ""
	if len(names) == 1 {
		rawField = names[0]
	}
	return printSpecOrRun(ctx, c.Runner, resolvedIn, defaultsIn, f.printSpec, f.printSpecVerbose, f.outputFormat, rawField, stdout, stderr)
}
