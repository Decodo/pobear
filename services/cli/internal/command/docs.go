package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/pobear/pobear/pkg/exitcode"
)

type Docs struct {
	Version  string
	Commands map[string]string
}

func (*Docs) Summary() string { return "Print the contract for driving pobear from a script or agent" }

type docCommand struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type docExitCode struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

type docOutput struct {
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
	Item   []string `json:"itemFields"`
}

type docContract struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

type docPayload struct {
	Version   string        `json:"version"`
	Commands  []docCommand  `json:"commands"`
	ExitCodes []docExitCode `json:"exitCodes"`
	Output    docOutput     `json:"output"`
	Contracts []docContract `json:"contracts"`
}

func (d *Docs) Run(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(stderr, "pobear: unknown flag: %s\n", a)
			return exitcode.UsageError
		}
	}

	payload := d.payload()
	if asJSON {
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "pobear: %v\n", err)
			return exitcode.Fatal
		}
		fmt.Fprintln(stdout, string(encoded))
		return exitcode.Success
	}

	printDocs(stdout, payload)
	return exitcode.Success
}

func (d *Docs) payload() docPayload {
	names := make([]string, 0, len(d.Commands))
	for name := range d.Commands {
		names = append(names, name)
	}
	sort.Strings(names)

	commands := make([]docCommand, 0, len(names))
	for _, name := range names {
		commands = append(commands, docCommand{Name: name, Summary: d.Commands[name]})
	}

	version := d.Version
	if version == "" {
		version = "dev"
	}

	return docPayload{
		Version:  version,
		Commands: commands,
		ExitCodes: []docExitCode{
			{Code: exitcode.Success, Meaning: "everything succeeded"},
			{Code: exitcode.AuthFailure, Meaning: "auth failure: a credential is missing or was rejected"},
			{Code: exitcode.UsageError, Meaning: "usage error or invalid job spec, nothing was fetched"},
			{Code: exitcode.PartialFailure, Meaning: "some items failed; the ones that succeeded are on stdout"},
			{Code: exitcode.Fatal, Meaning: "every item failed, or the failure is unclassified"},
		},
		Output: docOutput{
			Stdout: "one NDJSON object per fetched item",
			Stderr: "the `fetched N, errors N, skipped N` summary, and one detail line per failed URL",
			Item:   []string{"url", "finalUrl", "parentUrl", "status", "body", "depth", "data"},
		},
		Contracts: []docContract{
			{
				ID:      "provider-schema",
				Summary: "`pobear providers <name> --json` is the stable way to discover a provider's fields; --help is free text and must not be parsed",
			},
			{
				ID:      "spec-round-trip",
				Summary: "`--print-spec` output is valid, equivalent input to `pobear run -`, and printing it needs no credential",
			},
			{
				ID:      "bare-output",
				Summary: "`--bare` prints only the extracted value with no envelope, and requires at most one extracted field",
			},
			{
				ID:      "empty-extraction",
				Summary: "`empty N` in the summary counts fetched items whose stages produced nothing; --fail-on-empty exits 5 when every item was empty",
			},
			{
				ID:      "soft-block",
				Summary: "a challenge page answers with HTTP 200 and real HTML, so it cannot be reported as [blocked]; `empty N` equal to the fetched count is the signal",
			},
		},
	}
}

func printDocs(stdout io.Writer, p docPayload) {
	fmt.Fprintf(stdout, "pobear %s — driving it from a script or agent\n\n", p.Version)

	fmt.Fprintln(stdout, "Commands:")
	width := 0
	for _, c := range p.Commands {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	for _, c := range p.Commands {
		fmt.Fprintf(stdout, "  %-*s  %s\n", width, c.Name, c.Summary)
	}

	fmt.Fprintln(stdout, "\nOutput:")
	fmt.Fprintf(stdout, "  stdout  %s\n", p.Output.Stdout)
	fmt.Fprintf(stdout, "  stderr  %s\n", p.Output.Stderr)
	fmt.Fprintf(stdout, "  fields  %v\n", p.Output.Item)

	fmt.Fprintln(stdout, "\nExit codes:")
	for _, e := range p.ExitCodes {
		fmt.Fprintf(stdout, "  %d  %s\n", e.Code, e.Meaning)
	}

	fmt.Fprintln(stdout, "\nContracts:")
	for _, c := range p.Contracts {
		fmt.Fprintf(stdout, "  %s\n    %s\n", c.ID, c.Summary)
	}

	fmt.Fprintln(stdout, "\nRun `pobear docs --json` for the same content as a parseable document.")
}

type Summarizer interface {
	Summary() string
}
