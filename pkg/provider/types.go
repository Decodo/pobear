package provider

// Manifest is PROVIDER_REFERENCE.md §1. Parse returns one with every
// optional field resolved to its default.
type Manifest struct {
	Name      string         `yaml:"name"`
	Endpoint  string         `yaml:"endpoint"`
	Method    string         `yaml:"method,omitempty"`
	Auth      Auth           `yaml:"auth"`
	Options   []Option       `yaml:"options"`
	Request   Request        `yaml:"request"`
	Response  Response       `yaml:"response"`
	Errors    []ErrorMapping `yaml:"errors"`
	Templates []Template     `yaml:"templates,omitempty"`
}

// Auth is PROVIDER_REFERENCE.md §5.
type Auth struct {
	Type       string `yaml:"type"`
	Credential string `yaml:"credential"`
	Name       string `yaml:"name,omitempty"`
}

// Option is PROVIDER_REFERENCE.md §6, reused identically inside Template.
type Option struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Default any      `yaml:"default,omitempty"`
	Pattern string   `yaml:"pattern,omitempty"`
	Min     *int     `yaml:"min,omitempty"`
	Max     *int     `yaml:"max,omitempty"`
	Values  []string `yaml:"values,omitempty"`
}

// Request is PROVIDER_REFERENCE.md §7, reused identically inside Template.
type Request struct {
	Body  map[string]any `yaml:"body,omitempty"`
	Query map[string]any `yaml:"query,omitempty"`
}

// Response is PROVIDER_REFERENCE.md §9.
type Response struct {
	Status   string `yaml:"status,omitempty"`
	Body     string `yaml:"body"`
	FinalURL string `yaml:"finalUrl,omitempty"`
}

// ErrorMapping is one entry of PROVIDER_REFERENCE.md §10.
type ErrorMapping struct {
	Status int    `yaml:"status"`
	Kind   string `yaml:"kind"`
}

// Template is one entry of PROVIDER_REFERENCE.md §11.
type Template struct {
	Name     string   `yaml:"name"`
	Default  bool     `yaml:"default,omitempty"`
	Endpoint string   `yaml:"endpoint,omitempty"`
	Method   string   `yaml:"method,omitempty"`
	Options  []Option `yaml:"options,omitempty"`
	Request  Request  `yaml:"request"`
}
