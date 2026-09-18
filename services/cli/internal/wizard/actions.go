package wizard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/jobresult"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/jobgen"
	"github.com/pobear/pobear/services/cli/internal/providerator"
	"github.com/pobear/pobear/services/core"
)

const savedJobFilename = "job.yaml"

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
		templateName, options, err = resolveProviderOptions(state, providersDir)
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
		Stages:      buildStages(state),
		Concurrency: &concurrency,
		Retries:     &retries,
		SinkUse:     "stdout",
	}
	return jobgen.Build(in)
}

func resolveProviderOptions(state formState, providersDir string) (templateName string, options map[string]any, err error) {
	manifests, err := providerator.New(providersDir).Load()
	if err != nil {
		return "", nil, err
	}
	m, ok := providerator.Lookup(manifests, state.providerName)
	if !ok {
		return "", nil, fmt.Errorf("unknown provider %q", state.providerName)
	}

	templateName = effectiveTemplateName(m, state.templateName)
	declared, err := providerator.ResolveTemplate(m, templateName)
	if err != nil {
		return "", nil, err
	}
	options, err = providerator.ValidateOptions(declared, state.providerOptions)
	if err != nil {
		return "", nil, err
	}
	return templateName, options, nil
}

func checkProviderCredential(state formState, providersDir, credentialsPath string) string {
	if state.fetchVia != "provider" {
		return ""
	}
	manifests, err := providerator.New(providersDir).Load()
	if err != nil {
		return "✗ " + err.Error()
	}
	m, ok := providerator.Lookup(manifests, state.providerName)
	if !ok {
		return fmt.Sprintf("✗ unknown provider %q", state.providerName)
	}
	hasCred, err := providerator.CheckCredential(m, credentialsPath)
	if err != nil {
		return "✗ " + err.Error()
	}
	if !hasCred {
		return fmt.Sprintf("✗ provider %q has no credential — run \"pobear auth %s\", or use \"Manage credentials\" from the main menu", m.Name, m.Name)
	}
	return ""
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

func buildStages(state formState) []jobspec.Stage {
	switch state.extract {
	case "raw", "markdown":
		return []jobspec.Stage{{Use: state.extract}}
	case "css fields":
		fields := map[string]string{}
		for _, pair := range state.cssFields {
			if pair.name == "" || pair.selector == "" {
				continue
			}
			fields[pair.name] = pair.selector
		}
		return []jobspec.Stage{{Use: "css", Fields: fields}}
	default:
		return nil
	}
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

type resultRow struct {
	url    string
	status string
	detail string
	raw    map[string]any
}

type sinkItem struct {
	URL      string         `json:"url"`
	FinalURL string         `json:"finalUrl"`
	Status   int            `json:"status"`
	Body     string         `json:"body"`
	Depth    int            `json:"depth"`
	Data     map[string]any `json:"data"`
}

func parseSinkItems(buf []byte) map[string]sinkItem {
	items := map[string]sinkItem{}
	dec := json.NewDecoder(bytes.NewReader(buf))
	for {
		var item sinkItem
		if err := dec.Decode(&item); err != nil {
			break
		}
		items[item.URL] = item
	}
	return items
}

func summarizeData(data map[string]any) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := strings.ReplaceAll(fmt.Sprintf("%v", data[k]), "\n", " ")
		const maxValueLen = 40
		if len(v) > maxValueLen {
			v = v[:maxValueLen] + "…"
		}
		parts = append(parts, k+": "+v)
	}
	return strings.Join(parts, "  ")
}

func bodyPreview(body string) string {
	preview := strings.TrimSpace(strings.ReplaceAll(body, "\n", " "))
	const maxLen = 80
	if len(preview) > maxLen {
		preview = preview[:maxLen] + "…"
	}
	return preview
}

func buildResultRows(items []jobresult.Item, sinkItems map[string]sinkItem) []resultRow {
	rows := make([]resultRow, len(items))
	for i, item := range items {
		row := resultRow{url: item.URL}
		raw := map[string]any{"url": item.URL}
		switch item.Outcome {
		case jobresult.Succeeded:
			row.status = "✓"
			raw["outcome"] = "succeeded"
			if si, ok := sinkItems[item.URL]; ok {
				row.status = fmt.Sprintf("✓ %d", si.Status)
				row.detail = summarizeData(si.Data)
				if row.detail == "" {
					row.detail = bodyPreview(si.Body)
				}
				raw["finalUrl"] = si.FinalURL
				raw["status"] = si.Status
				raw["depth"] = si.Depth
				raw["body"] = si.Body
				if len(si.Data) > 0 {
					raw["data"] = si.Data
				}
			}
		case jobresult.Failed:
			row.status = "✗"
			row.detail = strings.ReplaceAll(item.Err, "\n", " ")
			raw["outcome"] = "failed"
			raw["error"] = item.Err
			if item.Kind != "" {
				raw["kind"] = string(item.Kind)
			}
		case jobresult.Skipped:
			row.status = "skip"
			row.detail = strings.ReplaceAll(item.Err, "\n", " ")
			raw["outcome"] = "skipped"
			raw["reason"] = item.Err
		}
		row.raw = raw
		rows[i] = row
	}
	return rows
}

type runProgress struct {
	completed atomic.Int64
	total     int
}

func newRunProgress(total int) *runProgress {
	return &runProgress{total: total}
}

func (p *runProgress) percent() float64 {
	if p == nil || p.total <= 0 {
		return 0
	}
	pct := float64(p.completed.Load()) / float64(p.total)
	if pct > 1 {
		pct = 1
	}
	return pct
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

const resultsDirName = "results"

func itemJSON(row resultRow) ([]byte, error) {
	return json.MarshalIndent(row.raw, "", "  ")
}

func doWriteItem(row resultRow) string {
	data, err := itemJSON(row)
	if err != nil {
		return "✗ " + err.Error()
	}
	name := sanitizeFilename(row.url) + ".json"
	if err := os.WriteFile(name, data, 0o600); err != nil {
		return "✗ " + err.Error()
	}
	return "✓ wrote " + name
}

func doWriteAllItems(rows []resultRow) string {
	if err := os.MkdirAll(resultsDirName, 0o750); err != nil {
		return "✗ " + err.Error()
	}
	for i, row := range rows {
		data, err := itemJSON(row)
		if err != nil {
			return "✗ " + err.Error()
		}
		name := fmt.Sprintf("%03d-%s.json", i+1, sanitizeFilename(row.url))
		if err := os.WriteFile(filepath.Join(resultsDirName, name), data, 0o600); err != nil {
			return "✗ " + err.Error()
		}
	}
	return fmt.Sprintf("✓ wrote %d item(s) to %s/", len(rows), resultsDirName)
}
