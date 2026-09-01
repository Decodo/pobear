package jobspec

type Spec struct {
	Version int     `yaml:"version" json:"version"`
	Source  Source  `yaml:"source" json:"source"`
	Fetch   Fetch   `yaml:"fetch" json:"fetch"`
	Limits  Limits  `yaml:"limits" json:"limits"`
	Stages  []Stage `yaml:"stages" json:"stages"`
	Sink    Sink    `yaml:"sink" json:"sink"`
}

type Source struct {
	URLs  []string     `yaml:"urls,omitempty" json:"urls,omitempty"`
	Crawl *CrawlSource `yaml:"crawl,omitempty" json:"crawl,omitempty"`
}

type CrawlSource struct {
	Seeds       []string `yaml:"seeds" json:"seeds"`
	Depth       int      `yaml:"depth" json:"depth"`
	MaxPages    int      `yaml:"maxPages" json:"maxPages"`
	MaxDuration string   `yaml:"maxDuration" json:"maxDuration"`
	Scope       string   `yaml:"scope" json:"scope"`
	Include     []string `yaml:"include" json:"include"`
	Exclude     []string `yaml:"exclude" json:"exclude"`
	Sitemap     bool     `yaml:"sitemap" json:"sitemap"`
	Robots      string   `yaml:"robots" json:"robots"`
}

type Fetch struct {
	Use         string            `yaml:"use" json:"use"`
	Name        string            `yaml:"name,omitempty" json:"name,omitempty"`
	Template    string            `yaml:"template,omitempty" json:"template,omitempty"`
	Proxy       string            `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	Profile     string            `yaml:"profile,omitempty" json:"profile,omitempty"`
	Options     map[string]any    `yaml:"options,omitempty" json:"options,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	NoRedirects bool              `yaml:"noRedirects,omitempty" json:"noRedirects,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type Limits struct {
	Concurrency       int    `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
	RPS               int    `yaml:"rps" json:"rps"`
	Retries           int    `yaml:"retries,omitempty" json:"retries,omitempty"`
	Backoff           string `yaml:"backoff,omitempty" json:"backoff,omitempty"`
	PluginConcurrency int    `yaml:"pluginConcurrency" json:"pluginConcurrency"`
	PluginTimeout     string `yaml:"pluginTimeout,omitempty" json:"pluginTimeout,omitempty"`
}

type Stage struct {
	Use    string            `yaml:"use" json:"use"`
	Fields map[string]string `yaml:"fields,omitempty" json:"fields,omitempty"`
	ID     string            `yaml:"id,omitempty" json:"id,omitempty"`
}

type Sink struct {
	Use  string `yaml:"use" json:"use"`
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
}
