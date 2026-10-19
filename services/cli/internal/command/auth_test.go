package command_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Decodo/pobear/services/cli/internal/command"
	"github.com/Decodo/pobear/services/cli/internal/providerator"
)

func TestAuthUnknownProviderErrors(t *testing.T) {
	args := cliArgs("auth bogus-provider")

	cmd := &command.Auth{
		Providers: providerator.New(t.TempDir()),
		Prompt:    func(string) (string, error) { return "secret", nil },
	}

	var stdout, stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	if want := "pobear: unknown provider \"bogus-provider\"\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestAuthNoNameArgWithNoProvidersInstalledOnDiskStillListsEmbedded(t *testing.T) {
	args := cliArgs("auth")

	cmd := &command.Auth{
		Providers: providerator.New(t.TempDir()),
		Prompt:    func(string) (string, error) { return "secret", nil },
	}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (missing provider name)", got)
	}
	want := "pobear: you need to specify one of the available providers:\n"
	if !hasPrefix(stderr.String(), want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr.String(), want)
	}
	if !strings.Contains(stderr.String(), "decodo") {
		t.Fatalf("stderr = %q, want it to mention decodo (embedded, available with no providers directory)", stderr.String())
	}
}

func TestAuthNoNameArgListsInstalledProviders(t *testing.T) {
	args := cliArgs("auth")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "decodo.yaml"), []byte(`
name: decodo
endpoint: https://data.decodo.com/v1/scrape
auth: {type: bearer, credential: token}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	cmd := &command.Auth{
		Providers: providerator.New(dir),
		Prompt:    func(string) (string, error) { return "secret", nil },
	}

	var stderr bytes.Buffer
	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (missing provider name)", got)
	}
	if want := "pobear: you need to specify one of the available providers:\n"; !hasPrefix(stderr.String(), want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr.String(), want)
	}
	if !strings.Contains(stderr.String(), "decodo") {
		t.Fatalf("stderr = %q, want it to mention decodo", stderr.String())
	}
	if want := "\nUsage:\n  pobear auth <provider> [flags]\n"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
	}
}
