package jobspec

import "errors"

// Mode gates the handful of validation rules that depend on which front
// end is running: a plugin stage or stream sink is legal only under
// ModeRPC (JOB_SPEC_REFERENCE.md §6, §7).
type Mode int

// Mode values.
const (
	ModePlain Mode = iota
	ModeRPC
)

// ValidationError reports a spec document that failed a
// JOB_SPEC_REFERENCE.md §8 rule. Field is a dot path rooted at the spec
// document ("source.crawl.depth", "stages[0].use"); a structural error not
// tied to one field uses "$document". Tests assert on Field, never on
// Message's wording.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements error.
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ErrNotImplemented is what Parse returns until a later phase implements
// it — every fixture in testdata/validation is expected to fail against
// this; that's intended, not a bug.
var ErrNotImplemented = errors.New("jobspec: Parse not implemented")

// Parse decodes and validates data (YAML or JSON) per JOB_SPEC_REFERENCE.md
// §8, resolving every optional field to its default. It does not know
// about installed provider manifests — fetch.options/fetch.template
// validation against a manifest is the caller's job.
func Parse(data []byte, mode Mode) (*Spec, error) {
	_ = data
	_ = mode
	return nil, ErrNotImplemented
}
