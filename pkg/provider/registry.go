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

// LoadMerged loads manifests embedded in embedded plus whatever's in dir on
// disk, and merges them by name -- a manifest on disk overrides an embedded
// one with the same name, everything else keeps its declared order with
// embedded manifests first.
func LoadMerged(embedded fs.FS, dir string) ([]*Manifest, error) {
	embeddedManifests, err := LoadFS(embedded)
	if err != nil {
		return nil, err
	}
	if err := ValidateUnique(embeddedManifests); err != nil {
		return nil, err
	}

	diskManifests, err := LoadDir(dir)
	if err != nil {
		return nil, err
	}
	if err := ValidateUnique(diskManifests); err != nil {
		return nil, err
	}

	return mergeManifests(embeddedManifests, diskManifests), nil
}

func mergeManifests(embedded, disk []*Manifest) []*Manifest {
	byName := make(map[string]*Manifest, len(embedded)+len(disk))
	order := make([]string, 0, len(embedded)+len(disk))
	for _, m := range embedded {
		byName[m.Name] = m
		order = append(order, m.Name)
	}
	for _, m := range disk {
		if _, exists := byName[m.Name]; !exists {
			order = append(order, m.Name)
		}
		byName[m.Name] = m
	}
	merged := make([]*Manifest, len(order))
	for i, name := range order {
		merged[i] = byName[name]
	}
	return merged
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
