package core_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core"
)

// Regression test for a real bug: the CLI/wizard's provider *listing* went
// through providerator, which merges the embedded manifests (see
// providers/embedded.go) with whatever's on disk -- but the actual job
// *execution* path (this package's liveExecutor, used by every scrape/crawl
// run) loaded manifests straight from disk, bypassing the embedded set
// entirely. A provider that only exists embedded -- Decodo, out of the box --
// was listed by `pobear providers` but failed with "unknown provider" the
// moment a job actually tried to use it, since nothing was ever placed on
// disk. This proves the run path now finds it too, without needing a real
// network call: an unknown provider fails before any HTTP request with
// "unknown provider", while a known provider with no stored credential fails
// later, in a different way, with "missing credential" -- so which error we
// get here proves whether Decodo was found.
func TestNewFindsEmbeddedProviderOnTheRunPath(t *testing.T) {
	os.Unsetenv("POBEAR_PROVIDER_DECODO_TOKEN")

	spec := []byte(strings.Join([]string{
		"version: 1",
		"source: { urls: [https://example.com] }",
		"fetch: { use: provider, name: decodo }",
	}, "\n"))

	svc := core.New(t.TempDir(), t.TempDir()+"/credentials", io.Discard)
	result, _, err := svc.Run(context.Background(), spec, jobspec.ModePlain)
	if err != nil {
		t.Fatalf("Service.Run returned a top-level error, want the fetch failure surfaced per-item instead: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("Items = %+v, want exactly 1", result.Items)
	}

	got := result.Items[0].Err
	if strings.Contains(got, "unknown provider") {
		t.Fatalf("item error = %q -- decodo's embedded manifest wasn't found on the run path", got)
	}
	if !strings.Contains(got, "missing credential") {
		t.Fatalf("item error = %q, want a missing-credential failure (proving decodo WAS found, just has no stored token)", got)
	}
}
