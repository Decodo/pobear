package cli

import (
	"context"
	"fmt"
	"io"

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
	name := "wizard"
	var args []string
	if len(argv) > 0 {
		name = argv[0]
		args = argv[1:]
	}

	cmd, ok := s.commands[name]
	if !ok {
		fmt.Fprintf(stderr, "pobear: unknown command %q\n", name)
		return exitcode.UsageError
	}
	return cmd.Run(ctx, args, stdin, stdout, stderr)
}
