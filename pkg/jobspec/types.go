package jobspec

type Spec struct {
	Version int     `yaml:"version"`
	Source  Source  `yaml:"source"`
	Fetch   Fetch   `yaml:"fetch"`
	Limits  Limits  `yaml:"limits"`
	Stages  []Stage `yaml:"stages"`
	Sink    Sink    `yaml:"sink"`
}

type Source struct {
	URLs  []string     `yaml:"urls,omitempty"`
	Crawl *CrawlSource `yaml:"crawl,omitempty"`
}

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

type Limits struct {
	Concurrency       int    `yaml:"concurrency,omitempty"`
	RPS               int    `yaml:"rps"`
	Retries           int    `yaml:"retries,omitempty"`
	Backoff           string `yaml:"backoff,omitempty"`
	PluginConcurrency int    `yaml:"pluginConcurrency"`
	PluginTimeout     string `yaml:"pluginTimeout,omitempty"`
}

type Stage struct {
	Use    string            `yaml:"use"`
	Fields map[string]string `yaml:"fields,omitempty"`
	ID     string            `yaml:"id,omitempty"`
}

type Sink struct {
	Use  string `yaml:"use"`
	Path string `yaml:"path,omitempty"`
}
