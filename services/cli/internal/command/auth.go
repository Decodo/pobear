package command

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Decodo/pobear/pkg/exitcode"
	"github.com/Decodo/pobear/services/cli/internal/providerator"
)

type Auth struct {
	Providers *providerator.Providerator
	Prompt    func(label string) (string, error)
	Path      string
}

func (a *Auth) Run(_ context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	code := 0

	cmd := &cobra.Command{
		Use:   "auth <provider>",
		Short: (&Auth{}).Summary(),
		Long:  "Prompts for a secret and saves it for every subsequent run.",
		Args:  a.requireProviderName(),
		RunE: func(_ *cobra.Command, cmdArgs []string) error {
			if cerr := a.run(cmdArgs[0], stdout); cerr != nil {
				return cerr
			}
			code = exitcode.Success
			return nil
		},
	}

	return runCobraCommand(cmd, args, stdin, stdout, stderr, &code)
}

func (a *Auth) requireProviderName() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return nil
		}
		manifests, err := a.Providers.Load()
		if err != nil {
			return newCLIError(exitcode.Fatal, "%v", err)
		}
		if len(manifests) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "pobear: auth requires a provider name (no providers installed)")
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(), "pobear: you need to specify one of the available providers:")
			printProvidersList(manifests, cmd.ErrOrStderr())
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "\n%s", cmd.UsageString())
		return errArgsHandled
	}
}

func (a *Auth) run(name string, stdout io.Writer) *cliError {
	manifests, err := a.Providers.Load()
	if err != nil {
		return newCLIError(exitcode.Fatal, "%v", err)
	}

	m, ok := providerator.Lookup(manifests, name)
	if !ok {
		return newCLIError(exitcode.UsageError, "unknown provider %q", name)
	}

	if err := providerator.Auth(m, a.Path, a.Prompt, stdout); err != nil {
		return newCLIError(exitcode.UsageError, "%v", err)
	}
	return nil
}

func (*Auth) Summary() string { return "Save a credential for a provider" }
