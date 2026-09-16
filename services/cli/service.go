package cli

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/services/cli/internal/command"
)

type Service struct {
	commands map[string]command.Command
}

func NewService(commands map[string]command.Command) *Service {
	return &Service{commands: commands}
}

func (s *Service) Run(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		argv = []string{"wizard"}
	}

	name, args := argv[0], argv[1:]
	if name == "--help" || name == "-h" || name == "help" {
		printHelp(stdout, s.commands)
		return exitcode.Success
	}

	cmd, ok := s.commands[name]
	if !ok {
		fmt.Fprintf(stderr, "pobear: unknown command %q\n", name)
		return exitcode.UsageError
	}
	return cmd.Run(ctx, args, stdin, stdout, stderr)
}

func printHelp(stdout io.Writer, commands map[string]command.Command) {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintln(stdout, "pobear - short for polar bear")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  pobear <command> [flags]")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Commands:")
	for _, name := range names {
		fmt.Fprintf(stdout, "  %s\n", name)
	}
}
