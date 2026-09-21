package sinks

type Item struct {
	URL       string              `json:"url" yaml:"url"`
	FinalURL  string              `json:"finalUrl" yaml:"finalUrl"`
	ParentURL string              `json:"parentUrl" yaml:"parentUrl"`
	Status    int                 `json:"status" yaml:"status"`
	Headers   map[string][]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body      string              `json:"body" yaml:"body"`
	Depth     int                 `json:"depth" yaml:"depth"`
	Data      map[string]any      `json:"data,omitempty" yaml:"data,omitempty"`
	Tier      string              `json:"tier" yaml:"tier"`
}
