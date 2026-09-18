package wizard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
	"github.com/pobear/pobear/services/core"
)

func buildCrawlJobSpec(state crawlFormState, providersDir string) (*jobspec.Spec, error) {
	concurrency, _ := strconv.Atoi(state.concurrency)
	retries, _ := strconv.Atoi(state.retries)
	rps, _ := strconv.Atoi(state.rps)
	depth, _ := strconv.Atoi(state.depth)
	maxPages, _ := strconv.Atoi(state.maxPages)

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

	crawlIn := &jobgen.CrawlInput{
		Depth:    depth,
		MaxPages: &maxPages,
		Scope:    state.scope,
		Sitemap:  state.sitemap == "true",
		Robots:   state.robots,
	}

	in := jobgen.Input{
		URLs:        []string{state.startingURL},
		Crawl:       crawlIn,
		FetchUse:    state.fetchVia,
		Proxy:       proxy,
		Provider:    providerName,
		Template:    templateName,
		Options:     options,
		Profile:     profile,
		Timeout:     "30s",
		Stages:      buildStages(state.extractConfig),
		Concurrency: &concurrency,
		RPS:         &rps,
		Retries:     &retries,
		SinkUse:     "stdout",
	}
	return jobgen.Build(in)
}

func doCrawlSave(state crawlFormState, providersDir string) string {
	spec, err := buildCrawlJobSpec(state, providersDir)
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

func doCrawlRun(ctx context.Context, state crawlFormState, providersDir, credentialsPath string, progress *runProgress) (string, []resultRow) {
	spec, err := buildCrawlJobSpec(state, providersDir)
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

	rows := buildResultRows(result.Items, parseSinkItems(captured.Bytes()), state.extract == "bare")

	msg := fmt.Sprintf("fetched %d, errors %d, skipped %d", result.Succeeded, result.Failed, result.Skipped)
	if result.Failed > 0 {
		return "✗ " + msg, rows
	}
	return "✓ " + msg, rows
}

func startCrawlRun(ctx context.Context, state crawlFormState, providersDir, credentialsPath string, progress *runProgress) tea.Cmd {
	return func() tea.Msg {
		status, rows := doCrawlRun(ctx, state, providersDir, credentialsPath, progress)
		return runFinishedMsg{status: status, rows: rows}
	}
}
