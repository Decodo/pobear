package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pobear/pobear/services/core/internal/integrationtest"
)

func TestIntegration(t *testing.T) {
	root := "testdata/integration"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading %s: %v", root, err)
	}
	cases := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cases++
		t.Run(e.Name(), func(t *testing.T) {
			integrationtest.Run(t, filepath.Join(root, e.Name()))
		})
	}
	if cases == 0 {
		t.Fatalf("no fixture case directories found under %s", root)
	}
}
