package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type Providers struct {
	Providers *providerator.Providerator
}

func (p *Providers) Run(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) int {
	target, rest, hasTarget := splitPositional(args)

	var jsonOut bool
	fs := flag.NewFlagSet("providers", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&jsonOut, "json", false, "")

	if cerr := checkKnownFlags(fs, rest); cerr != nil {
		cerr.print(stderr)
		return cerr.code
	}
	if err := fs.Parse(rest); err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.UsageError
	}

	manifests, err := p.Providers.Load()
	if err != nil {
		fmt.Fprintf(stderr, "pobear: %v\n", err)
		return exitcode.Fatal
	}

	if !hasTarget {
		if len(manifests) == 0 {
			fmt.Fprintln(stderr, "no providers installed")
			return exitcode.Success
		}
		printProvidersList(manifests, stdout)
		return exitcode.Success
	}

	m, ok := providerator.Lookup(manifests, target)
	if !ok {
		fmt.Fprintf(stderr, "pobear: unknown provider %q\n", target)
		return exitcode.UsageError
	}

	if jsonOut {
		data, err := json.Marshal(providerator.BuildSchema(m))
		if err != nil {
			fmt.Fprintf(stderr, "pobear: %v\n", err)
			return exitcode.Fatal
		}
		fmt.Fprintln(stdout, string(data))
		return exitcode.Success
	}

	printProviderDetail(m, stdout)
	return exitcode.Success
}

func printProvidersList(manifests []*provider.Manifest, stdout io.Writer) {
	for _, m := range manifests {
		line := fmt.Sprintf("%-13s%s", m.Name, m.Endpoint)
		if len(m.Templates) > 0 {
			names := make([]string, len(m.Templates))
			for i, t := range m.Templates {
				n := t.Name
				if t.Default {
					n += " (default)"
				}
				names[i] = n
			}
			line += " templates: " + strings.Join(names, ", ")
		}
		fmt.Fprintln(stdout, line)
	}
}

func printProviderDetail(m *provider.Manifest, stdout io.Writer) {
	fmt.Fprintf(stdout, "%s — %s\n", m.Name, m.Endpoint)
	if len(m.Templates) == 0 {
		fmt.Fprintln(stdout)
		for _, o := range m.Options {
			fmt.Fprintf(stdout, "    %s\n", formatOptionLine(o))
		}
		return
	}
	for _, t := range m.Templates {
		fmt.Fprintln(stdout)
		name := t.Name
		if t.Default {
			name += " (default)"
		}
		fmt.Fprintf(stdout, "  %s\n", name)
		for _, o := range t.Options {
			fmt.Fprintf(stdout, "    %s\n", formatOptionLine(o))
		}
	}
}

func formatOptionLine(o provider.Option) string {
	typeDesc := o.Type
	if o.Type == "enum" {
		typeDesc = "enum: " + strings.Join(o.Values, ", ")
	}
	switch {
	case o.Default != nil:
		return fmt.Sprintf("%-9s%-34sdefault: %v", o.Name, typeDesc, o.Default)
	case o.Pattern != "":
		return fmt.Sprintf("%-9s%s   pattern: %s", o.Name, typeDesc, o.Pattern)
	case o.Min != nil || o.Max != nil:
		return fmt.Sprintf("%-9s%s   %s", o.Name, typeDesc, formatMinMax(o.Min, o.Max))
	default:
		return fmt.Sprintf("%-9s%s", o.Name, typeDesc)
	}
}

func formatMinMax(min, max *int) string {
	switch {
	case min != nil && max != nil:
		return fmt.Sprintf("min: %d, max: %d", *min, *max)
	case min != nil:
		return fmt.Sprintf("min: %d", *min)
	default:
		return fmt.Sprintf("max: %d", *max)
	}
}
