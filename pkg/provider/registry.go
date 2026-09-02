package provider

func ValidateUnique(manifests []*Manifest) error {
	_ = manifests
	return ErrNotImplemented
}

func LoadDir(dir string) ([]*Manifest, error) {
	_ = dir
	return nil, ErrNotImplemented
}
