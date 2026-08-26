package jobspec

// Spec is JOB_SPEC_REFERENCE.md §1. Parse returns one with every optional
// field resolved to its default.
type Spec struct {
	Version int     `yaml:"version"`
	Source  Source  `yaml:"source"`
	Fetch   Fetch   `yaml:"fetch"`
	Limits  Limits  `yaml:"limits"`
	Stages  []Stage `yaml:"stages"`
	Sink    Sink    `yaml:"sink"`
}

// Source is JOB_SPEC_REFERENCE.md §3.
type Source struct {
	URLs  []string     `yaml:"urls,omitempty"`
	Crawl *CrawlSource `yaml:"crawl,omitempty"`
}

// CrawlSource is JOB_SPEC_REFERENCE.md §3.2.
type CrawlSource struct {
	Seeds       []string `yaml:"seeds"`
	Depth       int      `yaml:"depth"`
	MaxPages    int      `yaml:"maxPages"`
	MaxDuration string   `yaml:"maxDuration"`
	Scope       string   `yaml:"scope"`
	Include     []string `yaml:"include"`
	Exclude     []string `yaml:"exclude"`
	Sitemap     bool     `yaml:"sitemap"`
	Robots      string   `yaml:"robots"`
}

// Fetch is JOB_SPEC_REFERENCE.md §4.
type Fetch struct {
	Use         string            `yaml:"use"`
	Name        string            `yaml:"name,omitempty"`
	Template    string            `yaml:"template,omitempty"`
	Proxy       string            `yaml:"proxy,omitempty"`
	Profile     string            `yaml:"profile,omitempty"`
	Options     map[string]any    `yaml:"options,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty"`
	NoRedirects bool              `yaml:"noRedirects,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
}

// Limits is JOB_SPEC_REFERENCE.md §5.
type Limits struct {
	Concurrency       int    `yaml:"concurrency,omitempty"`
	RPS               int    `yaml:"rps"`
	Retries           int    `yaml:"retries,omitempty"`
	Backoff           string `yaml:"backoff,omitempty"`
	PluginConcurrency int    `yaml:"pluginConcurrency"`
	PluginTimeout     string `yaml:"pluginTimeout,omitempty"`
}

// Stage is one entry of JOB_SPEC_REFERENCE.md §6.
type Stage struct {
	Use    string            `yaml:"use"`
	Fields map[string]string `yaml:"fields,omitempty"`
	ID     string            `yaml:"id,omitempty"`
}

// Sink is JOB_SPEC_REFERENCE.md §7.
type Sink struct {
	Use  string `yaml:"use"`
	Path string `yaml:"path,omitempty"`
}
