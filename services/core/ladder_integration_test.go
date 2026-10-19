package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Decodo/pobear/pkg/credentials"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core"
	"github.com/Decodo/pobear/services/core/internal/sink/sinks"
)

// This exercises a real fetch.ladder: true spec through the production
// core.New(...) wiring (real manifest loading, real credential loading, a
// real *fetch.Fetcher with all three tiers) rather than the simplified
// tier map the fixture-driven integrationtest harness builds by hand.
func TestLadderEscalatesFromLocalToProviderThroughRealWiring(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/target":
			w.WriteHeader(http.StatusForbidden)
		case "/provider-endpoint":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"content":"rescued by provider"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	providersDir := t.TempDir()
	manifest := fmt.Sprintf(`
name: fixture-provider
endpoint: %s/provider-endpoint
auth: {type: header, name: X-Api-Key, credential: token}
request:
  body: {url: "{{ .URL }}"}
response:
  body: "$.content"
`, srv.URL)
	if err := os.WriteFile(filepath.Join(providersDir, "fixture-provider.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("writing manifest: %v", err)
	}

	credentialsPath := filepath.Join(t.TempDir(), "credentials.env")
	if err := credentials.Save(credentialsPath, "fixture-provider", "token", "secret-value"); err != nil {
		t.Fatalf("saving credential: %v", err)
	}

	ndjsonPath := filepath.Join(t.TempDir(), "out.ndjson")
	spec := fmt.Sprintf(`
version: 1
source: {urls: [%q]}
fetch: {use: local, ladder: true, name: fixture-provider}
sink: {use: ndjson, path: %q}
`, srv.URL+"/target", ndjsonPath)

	svc := core.New(providersDir, credentialsPath, io.Discard)
	result, kind, err := svc.Run(context.Background(), []byte(spec), jobspec.ModePlain)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if kind != "" {
		t.Fatalf("top-level fault kind = %q, want empty (the ladder should have recovered by escalating to the provider tier)", kind)
	}
	if result.Succeeded != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want 1 succeeded, 0 failed", result)
	}

	data, err := os.ReadFile(ndjsonPath) //nolint:gosec // ndjsonPath is this test's own t.TempDir() file
	if err != nil {
		t.Fatalf("reading ndjson output: %v", err)
	}
	var item sinks.Item
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("decoding item: %v (raw=%s)", err, data)
	}
	if item.Tier != "provider" {
		t.Errorf("item.Tier = %q, want %q -- the ladder should have escalated past the blocked local fetch to the provider tier", item.Tier, "provider")
	}
	if item.Body != "rescued by provider" {
		t.Errorf("item.Body = %q, want %q", item.Body, "rescued by provider")
	}
}
