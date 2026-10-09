package provider

type Manifest struct {
	Name      string            `yaml:"name"`
	Endpoint  string            `yaml:"endpoint"`
	Method    string            `yaml:"method,omitempty"`
	Auth      Auth              `yaml:"auth"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	Options   []Option          `yaml:"options"`
	Request   Request           `yaml:"request"`
	Response  Response          `yaml:"response"`
	Errors    []ErrorMapping    `yaml:"errors"`
	Templates []Template        `yaml:"templates,omitempty"`
}

type Auth struct {
	Type       string `yaml:"type"`
	Credential string `yaml:"credential"`
	Name       string `yaml:"name,omitempty"`
}

type Option struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Default any      `yaml:"default,omitempty"`
	Pattern string   `yaml:"pattern,omitempty"`
	Min     *int     `yaml:"min,omitempty"`
	Max     *int     `yaml:"max,omitempty"`
	Values  []string `yaml:"values,omitempty"`
}

type Request struct {
	Body  map[string]any `yaml:"body,omitempty"`
	Query map[string]any `yaml:"query,omitempty"`
}

type Response struct {
	Status   string `yaml:"status,omitempty"`
	Body     string `yaml:"body"`
	FinalURL string `yaml:"finalUrl,omitempty"`
}

type ErrorMapping struct {
	Status int    `yaml:"status"`
	Kind   string `yaml:"kind"`
}

type Template struct {
	Name     string   `yaml:"name"`
	Default  bool     `yaml:"default,omitempty"`
	Endpoint string   `yaml:"endpoint,omitempty"`
	Method   string   `yaml:"method,omitempty"`
	Options  []Option `yaml:"options,omitempty"`
	Request  Request  `yaml:"request"`
}
