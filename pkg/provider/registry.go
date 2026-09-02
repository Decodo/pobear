package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ValidateUnique(manifests []*Manifest) error {
	seen := make(map[string]bool, len(manifests))
	for _, m := range manifests {
		if seen[m.Name] {
			return vErr("name", "manifest name %q is already declared", m.Name)
		}
		seen[m.Name] = true
	}
	return nil
}

func LoadDir(dir string) ([]*Manifest, error) {
	entries, err := os.ReadDir(dir) //nolint:gosec // dir is the caller-supplied providers directory, not user-controlled input
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path) //nolint:gosec // path is built from a caller-supplied directory listing, not user-controlled input
		if err != nil {
			return nil, err
		}

		manifest, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}
