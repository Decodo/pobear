// Command pobear builds the pobear binary, wiring the cli, rpc, and core
// services together (including the hidden `__rpc` subcommand).
//
// See docs/PROJECT_STRUCTURE.md for how this fits together.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/Decodo/pobear/services/cli"
	"github.com/Decodo/pobear/services/core"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	providersDir := os.Getenv("POBEAR_PROVIDERS_DIR")
	if providersDir == "" {
		dir, err := cli.DefaultProvidersDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pobear: %v\n", err)
			return 5
		}
		providersDir = dir
	}

	credentialsPath, err := cli.DefaultCredentialsPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pobear: %v\n", err)
		return 5
	}

	coreService := core.New(providersDir, credentialsPath, os.Stdout)
	cliService := cli.New(providersDir, coreService, credentialsPath, version, termPrompt)

	return cliService.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

func termPrompt(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s: ", label)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		secret, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(secret), nil
	}

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
