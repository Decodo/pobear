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
	got := (&command.Version{}).Run(context.Background(), args, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if stdout.String() != "pobear 0.0.1\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "pobear 0.0.1\n")
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
