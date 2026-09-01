package cli

import (
	"context"
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
	_ = ctx
	_ = argv
	_ = stdin
	_ = stdout
	_ = stderr
	_ = s.commands
	return exitcode.Fatal
}
