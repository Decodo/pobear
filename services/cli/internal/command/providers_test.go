package command_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

func TestProvidersWithNoDiskManifestsStillListsEmbedded(t *testing.T) {
	args := cliArgs("providers")

	cmd := &command.Providers{Providers: providerator.New(t.TempDir())}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "decodo") {
		t.Fatalf("stdout = %q, want it to mention decodo (embedded, available with no providers directory)", stdout.String())
	}
}

func TestProvidersUnknownNameErrors(t *testing.T) {
	args := cliArgs("providers bogus-provider")

	cmd := &command.Providers{Providers: providerator.New(t.TempDir())}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	if want := "pobear: unknown provider \"bogus-provider\"\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestProvidersJSONFlagPrintsSchema(t *testing.T) {
	args := cliArgs("providers decodo --json")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "decodo.yaml"), []byte(`
name: decodo
endpoint: https://data.decodo.com/v1/scrape
auth: {type: bearer, credential: token}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
options:
  - {name: render, type: bool, default: false}
`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	cmd := &command.Providers{Providers: providerator.New(dir)}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if want := `{"name":"decodo"`; !hasPrefix(stdout.String(), want) {
		t.Fatalf("stdout = %q, want prefix %q", stdout.String(), want)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
