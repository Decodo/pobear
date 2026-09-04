// Package utils holds the generic map[string]any validation helpers shared
// by pkg/jobspec and pkg/provider: both parse a YAML/JSON document into a
// generic tree and walk it by hand (rather than decoding straight into a
// typed struct) so they can report a strict, dotted field path and an exact,
// fixture-locked message for the first rule violated. Nothing here knows
// about job specs or provider manifests specifically.
package utils
