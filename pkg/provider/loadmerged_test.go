package provider_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/Decodo/pobear/pkg/provider"
)

func manifestYAML(name, endpoint string) string {
	return `
name: ` + name + `
endpoint: ` + endpoint + `
auth: {type: bearer, credential: token}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`
}

func TestLoadMergedIncludesEmbeddedOnlyManifest(t *testing.T) {
	embedded := fstest.MapFS{
		"decodo.yaml": {Data: []byte(manifestYAML("decodo", "https://data.decodo.com/v1/scrape"))},
	}

	got, err := provider.LoadMerged(embedded, t.TempDir())
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if len(got) != 1 || got[0].Name != "decodo" {
		t.Fatalf("LoadMerged() = %v, want the embedded-only decodo manifest", got)
	}
}

func TestLoadMergedIncludesDiskOnlyManifest(t *testing.T) {
	embedded := fstest.MapFS{
		"decodo.yaml": {Data: []byte(manifestYAML("decodo", "https://data.decodo.com/v1/scrape"))},
	}
	dir := t.TempDir()
	acmeYAML := manifestYAML("acme", "https://acme.example.com/scrape")
	if err := os.WriteFile(filepath.Join(dir, "acme.yaml"), []byte(acmeYAML), 0o600); err != nil {
		t.Fatalf("write disk-only manifest: %v", err)
	}

	got, err := provider.LoadMerged(embedded, dir)
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}

	names := map[string]bool{}
	for _, m := range got {
		names[m.Name] = true
	}
	if !names["decodo"] || !names["acme"] {
		t.Fatalf("LoadMerged() = %v, want both the embedded decodo and the disk-only acme manifest", got)
	}
}

func TestLoadMergedDiskOverridesEmbeddedByName(t *testing.T) {
	embedded := fstest.MapFS{
		"decodo.yaml": {Data: []byte(manifestYAML("decodo", "https://data.decodo.com/v1/scrape"))},
	}
	dir := t.TempDir()
	overrideYAML := manifestYAML("decodo", "https://custom.example.com/scrape")
	if err := os.WriteFile(filepath.Join(dir, "decodo.yaml"), []byte(overrideYAML), 0o600); err != nil {
		t.Fatalf("write override manifest: %v", err)
	}

	got, err := provider.LoadMerged(embedded, dir)
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LoadMerged() = %v, want exactly one decodo entry, the on-disk override", got)
	}
	if got[0].Endpoint != "https://custom.example.com/scrape" {
		t.Fatalf("LoadMerged()[0].Endpoint = %q, want the on-disk manifest's endpoint to win over the embedded one", got[0].Endpoint)
	}
}

func TestLoadMergedEmptyDiskDirStillReturnsEmbedded(t *testing.T) {
	embedded := fstest.MapFS{
		"decodo.yaml": {Data: []byte(manifestYAML("decodo", "https://data.decodo.com/v1/scrape"))},
	}

	got, err := provider.LoadMerged(embedded, filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if len(got) != 1 || got[0].Name != "decodo" {
		t.Fatalf("LoadMerged() = %v, want the embedded manifest even when the disk dir doesn't exist", got)
	}
}
