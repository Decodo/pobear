package command

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

func wireProviderHelp(cmd *cobra.Command, providers *providerator.Providerator, providerName *string) {
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, a []string) {
		defaultHelp(c, a)
		if *providerName == "" {
			return
		}
		printProviderHelp(c.OutOrStdout(), providers, *providerName)
	})
}

func printProviderHelp(w io.Writer, providers *providerator.Providerator, providerName string) {
	manifests, err := providers.Load()
	if err != nil {
		return
	}
	m, ok := providerator.Lookup(manifests, providerName)
	if !ok {
		return
	}

	fmt.Fprintln(w)
	if len(m.Templates) == 0 {
		fmt.Fprintf(w, "Provider %q\n\n", m.Name)
		for _, o := range m.Options {
			fmt.Fprintf(w, "  --opt %s\n", formatOptFlagLine(o))
		}
		return
	}

	defaultName, _ := resolvedTemplateName(m, "")
	fmt.Fprintf(w, "Provider %q — %d templates (default: %s). Pass --template to switch.\n\n", m.Name, len(m.Templates), defaultName)
	for i, t := range m.Templates {
		name := t.Name
		if t.Default {
			name += " (default)"
		}
		fmt.Fprintf(w, "  %s\n", name)
		for _, o := range t.Options {
			fmt.Fprintf(w, "    --opt %s\n", formatOptFlagLine(o))
		}
		if i < len(m.Templates)-1 {
			fmt.Fprintln(w)
		}
	}
}

func formatOptFlagLine(o provider.Option) string {
	typeDesc := o.Type
	if o.Type == "enum" {
		typeDesc = "enum: " + strings.Join(o.Values, "|")
	}
	head := fmt.Sprintf("%s=<%s>", o.Name, typeDesc)
	switch {
	case o.Default != nil:
		return fmt.Sprintf("%-30s default: %v", head, o.Default)
	case o.Pattern != "":
		return fmt.Sprintf("%-30s pattern: %s", head, o.Pattern)
	default:
		return head
	}
}

func resolvedTemplateName(m *provider.Manifest, templateName string) (name string, hasTemplates bool) {
	if len(m.Templates) == 0 {
		return "", false
	}
	if templateName != "" {
		return templateName, true
	}
	if len(m.Templates) == 1 {
		return m.Templates[0].Name, true
	}
	for _, t := range m.Templates {
		if t.Default {
			return t.Name, true
		}
	}
	return "", true
}

func suggestTemplateFor(m *provider.Manifest, optName, excludeName string) (string, bool) {
	for _, t := range m.Templates {
		if t.Name == excludeName {
			continue
		}
		for _, opt := range t.Options {
			if opt.Name == optName {
				return t.Name, true
			}
		}
	}
	return "", false
}

func findUnknownOption(declared []provider.Option, opts map[string]string) (string, bool) {
	byName := make(map[string]bool, len(declared))
	for _, o := range declared {
		byName[o.Name] = true
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	for _, k := range keys {
		if !byName[k] {
			return k, true
		}
	}
	return "", false
}

func resolveProviderOptions(providers *providerator.Providerator, credentialsPath, providerName, templateName string, opts map[string]string) (map[string]any, *cliError) {
	manifests, err := providers.Load()
	if err != nil {
		return nil, newCLIError(exitcode.Fatal, "%v", err)
	}
	m, ok := providerator.Lookup(manifests, providerName)
	if !ok {
		return nil, newCLIError(exitcode.UsageError, "unknown provider %q", providerName)
	}

	declared, rerr := providerator.ResolveTemplate(m, templateName)
	if rerr != nil {
		return nil, newCLIError(exitcode.UsageError, "%v", rerr)
	}

	if badName, isBad := findUnknownOption(declared, opts); isBad {
		resolvedName, hasTemplates := resolvedTemplateName(m, templateName)
		if hasTemplates {
			if suggestion, found := suggestTemplateFor(m, badName, resolvedName); found {
				return nil, newCLIError(exitcode.UsageError, "%s template %q has no option %q (did you mean --template %s?)", m.Name, resolvedName, badName, suggestion)
			}
		}
		return nil, newCLIError(exitcode.UsageError, "%s has no option %q", m.Name, badName)
	}

	validated, verr := providerator.ValidateOptions(declared, opts)
	if verr != nil {
		return nil, newCLIError(exitcode.UsageError, "%v", verr)
	}

	hasCred, cerr := providerator.CheckCredential(m, credentialsPath)
	if cerr != nil {
		return nil, newCLIError(exitcode.Fatal, "%v", cerr)
	}
	if !hasCred {
		return nil, newCLIError(exitcode.AuthFailure, "missing credential for provider %q (run: pobear auth %s)", m.Name, m.Name)
	}

	return validated, nil
}
