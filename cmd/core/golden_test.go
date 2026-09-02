package main_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pobear/pobear/internal/testutil/goldencli"
)

func TestGolden(t *testing.T) {
	binPath := goldencli.Build(t)

	root := "testdata/golden"
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) == 0 {
		t.Fatal("no golden fixture files found under " + root)
	}

	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relative path for %s: %v", path, err)
		}
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".yaml")
		t.Run(name, func(t *testing.T) {
			goldencli.Run(t, binPath, path)
		})
	}
}
