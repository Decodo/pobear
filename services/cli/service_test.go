package cli_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Decodo/pobear/services/cli"
	"github.com/Decodo/pobear/services/cli/internal/command"
)

type fakeCommand struct {
	gotArgs  []string
	gotStdin io.Reader
	exitCode int
}

func (f *fakeCommand) Run(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f.gotArgs = args
	f.gotStdin = stdin
	stdout.Write([]byte("ran\n")) //nolint:gosec,errcheck // test fake writing to a caller-supplied io.Writer, always a *bytes.Buffer in this file's tests; Write never errors here
	return f.exitCode
}

func TestServiceDispatchesToRegisteredCommand(t *testing.T) {
	argv := strings.Fields("scrape https://a.test --depth 2")

	fake := &fakeCommand{exitCode: 0}
	svc := cli.NewService(map[string]command.Command{"scrape": fake})

	var stdout, stderr bytes.Buffer
	got := svc.Run(context.Background(), argv, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if want := []string{"https://a.test", "--depth", "2"}; !equalStrings(fake.gotArgs, want) {
		t.Fatalf("command received args %v, want %v", fake.gotArgs, want)
	}
	if stdout.String() != "ran\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "ran\n")
	}
}

func TestServicePassesThroughCommandExitCode(t *testing.T) {
	argv := strings.Fields("run job.yaml")

	fake := &fakeCommand{exitCode: 4}
	svc := cli.NewService(map[string]command.Command{"run": fake})

	got := svc.Run(context.Background(), argv, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})

	if got != 4 {
		t.Fatalf("Run() = %d, want 4", got)
	}
}

func TestServiceUnknownCommand(t *testing.T) {
	argv := strings.Fields("bogus")

	svc := cli.NewService(map[string]command.Command{"scrape": &fakeCommand{}})

	var stdout, stderr bytes.Buffer
	got := svc.Run(context.Background(), argv, bytes.NewReader(nil), &stdout, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3 (usage error)", got)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if want := "pobear: unknown command \"bogus\"\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestServiceNoArgsDispatchesToWizard(t *testing.T) {
	fake := &fakeCommand{exitCode: 0}
	svc := cli.NewService(map[string]command.Command{"wizard": fake})

	var stdout bytes.Buffer
	got := svc.Run(context.Background(), nil, bytes.NewReader(nil), &stdout, &bytes.Buffer{})

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if len(fake.gotArgs) != 0 {
		t.Fatalf("wizard command got args %v, want none", fake.gotArgs)
	}
	if stdout.String() != "ran\n" {
		t.Fatalf("stdout = %q, want the wizard command to have run", stdout.String())
	}
}

func TestServiceNoArgsWithoutWizardRegisteredErrors(t *testing.T) {
	svc := cli.NewService(map[string]command.Command{"scrape": &fakeCommand{}})

	var stderr bytes.Buffer
	got := svc.Run(context.Background(), nil, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)

	if got != 3 {
		t.Fatalf("Run() = %d, want 3", got)
	}
	if want := "pobear: unknown command \"wizard\"\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestServiceHelpFlagListsRegisteredCommands(t *testing.T) {
	argv := strings.Fields("--help")

	svc := cli.NewService(map[string]command.Command{"scrape": &fakeCommand{}, "auth": &fakeCommand{}})

	var stdout, stderr bytes.Buffer
	got := svc.Run(context.Background(), argv, bytes.NewReader(nil), &stdout, &stderr)

	if got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "  auth\n") || !strings.Contains(stdout.String(), "  scrape\n") {
		t.Fatalf("stdout = %q, want it to list both registered commands", stdout.String())
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
