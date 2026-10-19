package main_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Decodo/pobear/internal/testutil/clitest"
)

func TestCLI(t *testing.T) {
	binPath := clitest.Build(t)

	root := "testdata"
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
		t.Fatal("no fixture files found under " + root)
	}

	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relative path for %s: %v", path, err)
		}
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".yaml")
		t.Run(name, func(t *testing.T) {
			clitest.Run(t, binPath, path)
		})
	}
}
