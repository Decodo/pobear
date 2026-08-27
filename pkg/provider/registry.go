package provider

// ValidateUnique checks that every manifest in manifests has a distinct
// Name (PROVIDER_REFERENCE.md §12: "No two loaded manifests share a
// name" — never "last one wins"). It takes already-parsed manifests, not
// raw files — loading manifests from a directory is the caller's job
// (services/cli/internal/providerator), not this package's.
func ValidateUnique(manifests []*Manifest) error {
	_ = manifests
	return ErrNotImplemented
}
