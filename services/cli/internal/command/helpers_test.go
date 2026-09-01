package command_test

import "strings"

func cliArgs(invocation string) []string {
	return strings.Fields(invocation)[1:]
}
