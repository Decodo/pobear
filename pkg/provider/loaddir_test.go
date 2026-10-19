package provider_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Decodo/pobear/pkg/provider"
)

func TestLoadDirEmptyDirIsNotAnError(t *testing.T) {
	got, err := provider.LoadDir(t.TempDir())
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadDir() = %v, want empty", got)
	}
}

func TestLoadDirMissingDirIsNotAnError(t *testing.T) {
	got, err := provider.LoadDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadDir() = %v, want empty", got)
	}
}

func TestLoadDirOneManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, filepath.Join(dir, "decodo.yaml"))

	got, err := provider.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(got) != 1 || got[0].Name != "decodo" {
		t.Fatalf("LoadDir() = %v, want one manifest named decodo", got)
	}
}

func writeManifest(t *testing.T, path string) {
	t.Helper()
	content := `
name: decodo
endpoint: https://data.decodo.com/v1/scrape
auth: {type: bearer, credential: token}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
