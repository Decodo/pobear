package provider

import "errors"

// ValidationError reports a manifest that failed a
// PROVIDER_REFERENCE.md §12 rule. Field is a dot path rooted at the
// manifest document ("auth.type", "options[0].pattern",
// "templates[1].options[0].values", "request.body.target"); a structural
// error not tied to one field uses "$document". Tests assert on Field and
// Message, never inferring rules from the doc's own wording.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements error.
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ErrNotImplemented is what Parse and ValidateUnique return until a later
// phase implements them — every fixture under testdata/ is expected to
// fail against this; that's intended, not a bug.
var ErrNotImplemented = errors.New("provider: not implemented")

// Parse decodes and validates data (YAML or JSON) as a single provider
// manifest document, per every single-manifest rule in
// PROVIDER_REFERENCE.md §12, resolving every optional field to its
// default. It does not check a manifest's name against any other
// manifest — that's ValidateUnique's job, once the caller has more than
// one parsed manifest to compare.
func Parse(data []byte) (*Manifest, error) {
	_ = data
	return nil, ErrNotImplemented
}
