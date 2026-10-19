package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Decodo/pobear/pkg/exitcode"

	"github.com/Decodo/pobear/services/cli/internal/command"
)

func runDocs(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	d := &command.Docs{Version: "1.2.3", Commands: map[string]string{
		"scrape": "Scrape a single URL, or every URL listed in a file",
		"run":    "Run a job spec file",
	}}
	code := d.Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestDocsJSONIsParseableAndCarriesTheContract(t *testing.T) {
	stdout, _, code := runDocs(t, "--json")

	if code != exitcode.Success {
		t.Fatalf("exit code = %d, want %d", code, exitcode.Success)
	}

	var payload struct {
		Version  string `json:"version"`
		Commands []struct {
			Name    string `json:"name"`
			Summary string `json:"summary"`
		} `json:"commands"`
		ExitCodes []struct {
			Code int `json:"code"`
		} `json:"exitCodes"`
		Contracts []struct {
			ID string `json:"id"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if payload.Version != "1.2.3" {
		t.Errorf("version = %q, want the injected build version", payload.Version)
	}
	if len(payload.Commands) != 2 || payload.Commands[0].Name != "run" {
		t.Errorf("commands = %+v, want both, sorted", payload.Commands)
	}

	gotCodes := make([]int, 0, len(payload.ExitCodes))
	for _, e := range payload.ExitCodes {
		gotCodes = append(gotCodes, e.Code)
	}
	wantCodes := []int{exitcode.Success, exitcode.AuthFailure, exitcode.UsageError, exitcode.PartialFailure, exitcode.Fatal}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Errorf("exit codes = %v, want %v", gotCodes, wantCodes)
	}

	wantContracts := []string{"provider-schema", "spec-round-trip", "bare-output", "empty-extraction", "soft-block"}
	gotContracts := make([]string, 0, len(payload.Contracts))
	for _, c := range payload.Contracts {
		gotContracts = append(gotContracts, c.ID)
	}
	if !reflect.DeepEqual(gotContracts, wantContracts) {
		t.Errorf("contract ids = %v, want %v", gotContracts, wantContracts)
	}
}

func TestDocsItemFieldsAreTheDocumentedSet(t *testing.T) {
	stdout, _, _ := runDocs(t, "--json")

	var payload struct {
		Output struct {
			Item []string `json:"itemFields"`
		} `json:"output"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	want := []string{"url", "finalUrl", "parentUrl", "status", "body", "depth", "data", "tier"}
	if !reflect.DeepEqual(payload.Output.Item, want) {
		t.Errorf("itemFields = %v, want %v", payload.Output.Item, want)
	}
}

func TestDocsHumanOutputMentionsEachSection(t *testing.T) {
	stdout, _, code := runDocs(t)

	if code != exitcode.Success {
		t.Fatalf("exit code = %d, want %d", code, exitcode.Success)
	}
	for _, want := range []string{"Commands:", "Output:", "Exit codes:", "Contracts:", "--json"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("human output is missing %q", want)
		}
	}
}

func TestDocsRejectsAnUnknownFlag(t *testing.T) {
	_, stderr, code := runDocs(t, "--bogus")

	if code != exitcode.UsageError {
		t.Errorf("exit code = %d, want %d", code, exitcode.UsageError)
	}
	if !strings.Contains(stderr, "unknown flag") {
		t.Errorf("stderr = %q, want an unknown-flag error", stderr)
	}
}

func TestEachCommandHelpNamesItself(t *testing.T) {
	commands := map[string]command.Command{
		"scrape":    &command.Scrape{},
		"crawl":     &command.Crawl{},
		"auth":      &command.Auth{},
		"providers": &command.Providers{},
		"run":       &command.Run{},
	}

	for name, cmd := range commands {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd.Run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)
			out := stdout.String() + stderr.String()

			if want := "help for " + name; !strings.Contains(out, want) {
				t.Errorf("help does not contain %q", want)
			}
			if want := "pobear " + name; !strings.Contains(out, want) {
				t.Errorf("usage line does not start the invocation with %q", want)
			}
		})
	}
}
