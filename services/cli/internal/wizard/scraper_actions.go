package wizard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
	"github.com/pobear/pobear/services/core"
)

func buildJobSpec(state formState, providersDir string) (*jobspec.Spec, error) {
	concurrency, _ := strconv.Atoi(state.concurrency)
	retries, _ := strconv.Atoi(state.retries)

	profile, proxy, providerName, templateName := state.profile, "", "", ""
	var options map[string]any
	switch state.fetchVia {
	case "proxy":
		proxy = state.proxyURL
	case "provider":
		profile = ""
		providerName = state.providerName
		var err error
		templateName, options, err = resolveProviderOptions(state.fetchConfig, providersDir)
		if err != nil {
			return nil, err
		}
	}

	in := jobgen.Input{
		URLs:        urlList(state),
		FetchUse:    state.fetchVia,
		Proxy:       proxy,
		Provider:    providerName,
		Template:    templateName,
		Options:     options,
		Profile:     profile,
		Timeout:     "30s",
		Stages:      buildStages(state.extractConfig),
		Concurrency: &concurrency,
		Retries:     &retries,
		SinkUse:     "stdout",
	}
	return jobgen.Build(in)
}

func urlList(state formState) []string {
	lines := strings.Split(state.url, "\n")
	urls := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			urls = append(urls, line)
		}
	}
	return urls
}

func doSave(state formState, providersDir string) string {
	spec, err := buildJobSpec(state, providersDir)
	if err != nil {
		return "✗ " + err.Error()
	}
	data, err := yaml.Marshal(spec)
	if err != nil {
		return "✗ " + err.Error()
	}
	if err := os.WriteFile(savedJobFilename, data, 0o600); err != nil {
		return "✗ " + err.Error()
	}
	return "✓ saved to " + savedJobFilename
}

func doRun(ctx context.Context, state formState, providersDir, credentialsPath string, progress *runProgress) (string, []resultRow) {
	spec, err := buildJobSpec(state, providersDir)
	if err != nil {
		return "✗ " + err.Error(), nil
	}
	data, err := yaml.Marshal(spec)
	if err != nil {
		return "✗ " + err.Error(), nil
	}

	var captured bytes.Buffer
	var out io.Writer = &captured
	if progress != nil {
		out = &countingWriter{out: &captured, counter: &progress.completed}
	}

	runner := command.NewJobRunner(core.New(providersDir, credentialsPath, out))
	result, _, err := runner.Run(ctx, data, jobspec.ModePlain)
	if err != nil {
		return "✗ " + err.Error(), nil
	}

	rows := buildResultRows(result.Items, parseSinkItems(captured.Bytes()))

	msg := fmt.Sprintf("fetched %d, errors %d, skipped %d", result.Succeeded, result.Failed, result.Skipped)
	if result.Failed > 0 {
		return "✗ " + msg, rows
	}
	return "✓ " + msg, rows
}

func startRun(ctx context.Context, state formState, providersDir, credentialsPath string, progress *runProgress) tea.Cmd {
	return func() tea.Msg {
		status, rows := doRun(ctx, state, providersDir, credentialsPath, progress)
		return runFinishedMsg{status: status, rows: rows}
	}
}
