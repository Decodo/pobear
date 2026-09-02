package jobgen

import (
	"errors"

	"github.com/pobear/pobear/pkg/jobspec"
)

type CrawlInput struct {
	Depth       int
	MaxPages    int
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
	Proxy       string
	Provider    string
	Template    string
	Profile     string
	Options     map[string]any
	Timeout     string
	NoRedirects bool
	Stages      []jobspec.Stage
	Concurrency int
	RPS         int
	Retries     int
	SinkUse     string
	SinkPath    string
}

var ErrNotImplemented = errors.New("jobgen: not implemented")

func Build(in Input) (*jobspec.Spec, error) {
	_ = in
	return nil, ErrNotImplemented
}

func MinimalJSON(resolved, defaults *jobspec.Spec) ([]byte, error) {
	_ = resolved
	_ = defaults
	return nil, ErrNotImplemented
}
