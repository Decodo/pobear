package command_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/providerator"
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

func TestAuthNoNameArgIsUsageError(t *testing.T) {
	args := cliArgs("auth")

	cmd := &command.Auth{
		Providers: providerator.New(t.TempDir()),
		Prompt:    func(string) (string, error) { return "secret", nil },
	}

	got := cmd.Run(context.Background(), args, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (missing provider name)", got)
	}
}
