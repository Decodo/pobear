// Package jobresult holds job outcome types shared across the job executor,
// crawl walker, and CLI command layer.
package jobresult

import "github.com/Decodo/pobear/pkg/fault"

type Outcome int

const (
	Succeeded Outcome = iota
	Failed
	Skipped
)

type Item struct {
	URL     string
	Outcome Outcome
	Kind    fault.Kind
	Err     string
}

type Result struct {
	Succeeded   int
	Failed      int
	Skipped     int
	Empty       int
	Kinds       []fault.Kind
	Errors      []string
	SkippedURLs []string
	Items       []Item
}
