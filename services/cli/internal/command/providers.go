package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Decodo/pobear/pkg/exitcode"
	"github.com/Decodo/pobear/pkg/provider"
	"github.com/Decodo/pobear/services/cli/internal/providerator"
)

type Providers struct {
	Providers *providerator.Providerator
}

func (p *Providers) Run(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var jsonOut bool
	code := 0

	cmd := &cobra.Command{
		Use:   "providers [name]",
		Short: (&Providers{}).Summary(),
		Long: "With no name, lists every installed manifest, one line each.\n\n" +
			"With a name, describes that one manifest in full -- every template,\n" +
			"every option, its type, constraints, and default.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, cmdArgs []string) error {
			var name string
			if len(cmdArgs) == 1 {
				name = cmdArgs[0]
			}
			if cerr := providersRun(p.Providers, name, jsonOut, stdout, stderr); cerr != nil {
				return cerr
			}
			code = exitcode.Success
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable schema instead of the human-readable table")

	return runCobraCommand(cmd, args, stdin, stdout, stderr, &code)
}

func providersRun(providers *providerator.Providerator, name string, jsonOut bool, stdout, stderr io.Writer) *cliError {
	manifests, err := providers.Load()
	if err != nil {
		return newCLIError(exitcode.Fatal, "%v", err)
	}

	if name == "" {
		if len(manifests) == 0 {
			fmt.Fprintln(stderr, "no providers installed")
			return nil
		}
		printProvidersList(manifests, stdout)
		return nil
	}

	m, ok := providerator.Lookup(manifests, name)
	if !ok {
		return newCLIError(exitcode.UsageError, "unknown provider %q", name)
	}

	if jsonOut {
		data, err := json.Marshal(providerator.BuildSchema(m))
		if err != nil {
			return newCLIError(exitcode.Fatal, "%v", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	printProviderDetail(m, stdout)
	return nil
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
	name := padName(o.Name, 9)
	typeDesc := o.Type
	if o.Type == "enum" {
		typeDesc = "enum: " + strings.Join(o.Values, ", ")
	}
	switch {
	case o.Default != nil:
		return fmt.Sprintf("%s%-34sdefault: %v", name, typeDesc, o.Default)
	case o.Pattern != "":
		return fmt.Sprintf("%s%s   pattern: %s", name, typeDesc, o.Pattern)
	case o.Min != nil || o.Max != nil:
		return fmt.Sprintf("%s%s   %s", name, typeDesc, formatMinMax(o.Min, o.Max))
	default:
		return fmt.Sprintf("%s%s", name, typeDesc)
	}
}

func padName(name string, width int) string {
	if len(name) < width {
		return name + strings.Repeat(" ", width-len(name))
	}
	return name + " "
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

func (*Providers) Summary() string { return "List installed provider manifests, or describe one" }
