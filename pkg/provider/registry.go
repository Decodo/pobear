package provider

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pobear/pobear/pkg/utils"
)

func ValidateUnique(manifests []*Manifest) error {
	seen := make(map[string]bool, len(manifests))
	for _, m := range manifests {
		if seen[m.Name] {
			return utils.Errorf("name", "manifest name %q is already declared", m.Name)
		}
		seen[m.Name] = true
	}
	return nil
}

func LoadDir(dir string) ([]*Manifest, error) {
	return LoadFS(os.DirFS(dir))
}

func LoadFS(fsys fs.FS) ([]*Manifest, error) {
	entries, err := fs.ReadDir(fsys, ".")
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

		data, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, err
		}

		manifest, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}
