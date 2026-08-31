package sinks

import "errors"

var ErrNotImplemented = errors.New("sinks: not implemented")

type Item struct {
	URL       string              `json:"url"`
	FinalURL  string              `json:"finalUrl"`
	ParentURL string              `json:"parentUrl"`
	Status    int                 `json:"status"`
	Headers   map[string][]string `json:"headers,omitempty"`
	Body      string              `json:"body"`
	Depth     int                 `json:"depth"`
	Data      map[string]any      `json:"data,omitempty"`
}
