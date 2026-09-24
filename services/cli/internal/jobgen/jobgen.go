package jobgen

import (
	"encoding/json"
	"reflect"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/jobspec"
)

type CrawlInput struct {
	Depth       int
	MaxPages    *int
	MaxDuration string
	Scope       string
	Include     []string
	Exclude     []string
	Sitemap     bool
	Robots      string
}

type Input struct {
	URLs        []string
	Crawl       *CrawlInput
	FetchUse    string
	Ladder      bool
	Proxy       string
	Provider    string
	Template    string
	Profile     string
	Options     map[string]any
	Timeout     string
	NoRedirects bool
	Headers     map[string]string
	Stages      []jobspec.Stage
	Concurrency *int
	RPS         *int
	Retries     *int
	SinkUse     string
	SinkPath    string
}

func Build(in Input) (*jobspec.Spec, error) {
	doc := map[string]any{
		"version": 1,
		"source":  buildSourceDoc(in),
		"fetch":   buildFetchDoc(in),
	}
	if limits := buildLimitsDoc(in); limits != nil {
		doc["limits"] = limits
	}
	if len(in.Stages) > 0 {
		doc["stages"] = buildStagesDoc(in.Stages)
	}
	if sink := buildSinkDoc(in); sink != nil {
		doc["sink"] = sink
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return jobspec.Parse(data, jobspec.ModePlain)
}

func buildSourceDoc(in Input) map[string]any {
	if in.Crawl == nil {
		return map[string]any{"urls": in.URLs}
	}

	c := in.Crawl
	crawl := map[string]any{"seeds": in.URLs, "depth": c.Depth}
	if c.MaxPages != nil {
		crawl["maxPages"] = *c.MaxPages
	}
	if c.MaxDuration != "" {
		crawl["maxDuration"] = c.MaxDuration
	}
	if c.Scope != "" {
		crawl["scope"] = c.Scope
	}
	if len(c.Include) > 0 {
		crawl["include"] = c.Include
	}
	if len(c.Exclude) > 0 {
		crawl["exclude"] = c.Exclude
	}
	if c.Sitemap {
		crawl["sitemap"] = c.Sitemap
	}
	if c.Robots != "" {
		crawl["robots"] = c.Robots
	}
	return map[string]any{"crawl": crawl}
}

func buildFetchDoc(in Input) map[string]any {
	fetch := map[string]any{"use": in.FetchUse}
	if in.Ladder {
		fetch["ladder"] = in.Ladder
	}
	if in.Provider != "" {
		fetch["name"] = in.Provider
	}
	if in.Template != "" {
		fetch["template"] = in.Template
	}
	if in.Proxy != "" {
		fetch["proxy"] = in.Proxy
	}
	if in.Profile != "" {
		fetch["profile"] = in.Profile
	}
	if len(in.Options) > 0 {
		fetch["options"] = in.Options
	}
	if in.Timeout != "" {
		fetch["timeout"] = in.Timeout
	}
	if in.NoRedirects {
		fetch["noRedirects"] = in.NoRedirects
	}
	if len(in.Headers) > 0 {
		fetch["headers"] = in.Headers
	}
	return fetch
}

func buildLimitsDoc(in Input) map[string]any {
	limits := map[string]any{}
	if in.Concurrency != nil {
		limits["concurrency"] = *in.Concurrency
	}
	if in.RPS != nil {
		limits["rps"] = *in.RPS
	}
	if in.Retries != nil {
		limits["retries"] = *in.Retries
	}
	if len(limits) == 0 {
		return nil
	}
	return limits
}

func buildStagesDoc(stages []jobspec.Stage) []map[string]any {
	docs := make([]map[string]any, len(stages))
	for i, s := range stages {
		doc := map[string]any{"use": s.Use}
		if len(s.Fields) > 0 {
			doc["fields"] = s.Fields
		}
		if s.ID != "" {
			doc["id"] = s.ID
		}
		docs[i] = doc
	}
	return docs
}

func buildSinkDoc(in Input) map[string]any {
	if in.SinkUse == "" {
		return nil
	}
	sink := map[string]any{"use": in.SinkUse}
	if in.SinkPath != "" {
		sink["path"] = in.SinkPath
	}
	return sink
}

type minimalSpec struct {
	Version int             `json:"version"`
	Source  minimalSource   `json:"source"`
	Fetch   minimalFetch    `json:"fetch"`
	Limits  *minimalLimits  `json:"limits,omitempty"`
	Stages  []jobspec.Stage `json:"stages,omitempty"`
	Sink    *minimalSink    `json:"sink,omitempty"`
}

type minimalSource struct {
	URLs  []string            `json:"urls,omitempty"`
	Crawl *minimalCrawlSource `json:"crawl,omitempty"`
}

type minimalCrawlSource struct {
	Seeds       []string `json:"seeds"`
	Depth       *int     `json:"depth,omitempty"`
	MaxPages    *int     `json:"maxPages,omitempty"`
	MaxDuration *string  `json:"maxDuration,omitempty"`
	Scope       *string  `json:"scope,omitempty"`
	Include     []string `json:"include,omitempty"`
	Exclude     []string `json:"exclude,omitempty"`
	Sitemap     *bool    `json:"sitemap,omitempty"`
	Robots      *string  `json:"robots,omitempty"`
}

type minimalFetch struct {
	Use         string            `json:"use"`
	Ladder      *bool             `json:"ladder,omitempty"`
	Name        *string           `json:"name,omitempty"`
	Template    *string           `json:"template,omitempty"`
	Proxy       *string           `json:"proxy,omitempty"`
	Profile     *string           `json:"profile,omitempty"`
	Options     map[string]any    `json:"options,omitempty"`
	Timeout     *string           `json:"timeout,omitempty"`
	NoRedirects *bool             `json:"noRedirects,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type minimalLimits struct {
	Concurrency       *int    `json:"concurrency,omitempty"`
	RPS               *int    `json:"rps,omitempty"`
	Retries           *int    `json:"retries,omitempty"`
	Backoff           *string `json:"backoff,omitempty"`
	PluginConcurrency *int    `json:"pluginConcurrency,omitempty"`
	PluginTimeout     *string `json:"pluginTimeout,omitempty"`
}

type minimalSink struct {
	Use  *string `json:"use,omitempty"`
	Path *string `json:"path,omitempty"`
}

func MinimalJSON(resolved, defaults *jobspec.Spec) ([]byte, error) {
	m := minimalSpec{
		Version: resolved.Version,
		Source:  minimalSourceFrom(resolved.Source, defaults.Source),
		Fetch:   minimalFetchFrom(resolved.Fetch, defaults.Fetch),
		Limits:  minimalLimitsFrom(resolved.Limits, defaults.Limits),
		Sink:    minimalSinkFrom(resolved.Sink, defaults.Sink),
	}
	if !reflect.DeepEqual(resolved.Stages, defaults.Stages) {
		m.Stages = resolved.Stages
	}
	return json.MarshalIndent(m, "", "  ")
}

func minimalSourceFrom(r, d jobspec.Source) minimalSource {
	if r.Crawl != nil {
		return minimalSource{Crawl: minimalCrawlFrom(r.Crawl, d.Crawl)}
	}
	return minimalSource{URLs: r.URLs}
}

func minimalCrawlFrom(r, d *jobspec.CrawlSource) *minimalCrawlSource {
	if d == nil {
		d = &jobspec.CrawlSource{}
	}
	return &minimalCrawlSource{
		Seeds:       r.Seeds,
		Depth:       diffInt(r.Depth, d.Depth),
		MaxPages:    diffInt(r.MaxPages, d.MaxPages),
		MaxDuration: diffString(r.MaxDuration, d.MaxDuration),
		Scope:       diffString(r.Scope, d.Scope),
		Include:     diffStringSlice(r.Include, d.Include),
		Exclude:     diffStringSlice(r.Exclude, d.Exclude),
		Sitemap:     diffBool(r.Sitemap, d.Sitemap),
		Robots:      diffString(r.Robots, d.Robots),
	}
}

func minimalFetchFrom(r, d jobspec.Fetch) minimalFetch {
	profile := diffString(r.Profile, d.Profile)
	if r.Use == "provider" {
		profile = nil
	}
	return minimalFetch{
		Use:         r.Use,
		Ladder:      diffBool(r.Ladder, d.Ladder),
		Name:        diffString(r.Name, d.Name),
		Template:    diffString(r.Template, d.Template),
		Proxy:       diffString(r.Proxy, d.Proxy),
		Profile:     profile,
		Options:     diffMap(r.Options, d.Options),
		Timeout:     diffString(r.Timeout, d.Timeout),
		NoRedirects: diffBool(r.NoRedirects, d.NoRedirects),
		Headers:     diffHeaders(r.Headers, d.Headers),
	}
}

func minimalLimitsFrom(r, d jobspec.Limits) *minimalLimits {
	lim := minimalLimits{
		Concurrency:       diffInt(r.Concurrency, d.Concurrency),
		RPS:               diffInt(r.RPS, d.RPS),
		Retries:           diffInt(r.Retries, d.Retries),
		Backoff:           diffString(r.Backoff, d.Backoff),
		PluginConcurrency: diffInt(r.PluginConcurrency, d.PluginConcurrency),
		PluginTimeout:     diffString(r.PluginTimeout, d.PluginTimeout),
	}
	if lim == (minimalLimits{}) {
		return nil
	}
	return &lim
}

func minimalSinkFrom(r, d jobspec.Sink) *minimalSink {
	sink := minimalSink{
		Use:  diffString(r.Use, d.Use),
		Path: diffString(r.Path, d.Path),
	}
	if sink == (minimalSink{}) {
		return nil
	}
	return &sink
}

func diffInt(r, d int) *int {
	if r == d {
		return nil
	}
	return &r
}

func diffString(r, d string) *string {
	if r == d {
		return nil
	}
	return &r
}

func diffBool(r, d bool) *bool {
	if r == d {
		return nil
	}
	return &r
}

func diffStringSlice(r, d []string) []string {
	if reflect.DeepEqual(r, d) {
		return nil
	}
	return r
}

func diffMap(r, d map[string]any) map[string]any {
	if reflect.DeepEqual(r, d) {
		return nil
	}
	return r
}

func diffHeaders(r, d map[string]string) map[string]string {
	if reflect.DeepEqual(r, d) {
		return nil
	}
	return r
}
