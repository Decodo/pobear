package command_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/pobear/pobear/services/cli/internal/command"
)

func TestVersionPrintsAndSucceeds(t *testing.T) {
	args := cliArgs("version")

	var stdout, stderr bytes.Buffer
	got := (&command.Version{Version: "v1.2.3"}).Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if stdout.String() != "pobear v1.2.3\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "pobear v1.2.3\n")
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionDefaultsToDevWhenUnset(t *testing.T) {
	args := cliArgs("version")

	var stdout, stderr bytes.Buffer
	got := (&command.Version{}).Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if stdout.String() != "pobear dev\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "pobear dev\n")
	}
}
