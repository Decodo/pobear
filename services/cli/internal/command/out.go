package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Decodo/pobear/pkg/exitcode"
	"github.com/Decodo/pobear/pkg/jobspec"
)

func isDirOut(outPath string) bool {
	if outPath == "" {
		return false
	}
	if strings.HasSuffix(outPath, "/") || strings.HasSuffix(outPath, string(os.PathSeparator)) {
		return true
	}
	info, err := os.Stat(outPath)
	return err == nil && info.IsDir()
}

type rawItem struct {
	URL    string         `json:"url"`
	Status int            `json:"status"`
	Body   string         `json:"body"`
	Data   map[string]any `json:"data"`
}

func runJobRaw(ctx context.Context, runner JobRunner, spec *jobspec.Spec, fieldName, outPath string, stdout, stderr io.Writer) (int, *cliError) {
	tmpDir, err := os.MkdirTemp("", "pobear-raw-*")
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, "out.ndjson")

	raw := *spec
	raw.Sink = jobspec.Sink{Use: "ndjson", Path: tmpPath}

	data, err := yaml.Marshal(&raw)
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}

	code, _, cerr := execAndSummarize(ctx, runner, data, stderr)
	if cerr != nil {
		return 0, cerr
	}
	if code != exitcode.Success {
		return code, nil
	}

	value, err := readRawValue(tmpPath, fieldName)
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(value), 0o600); err != nil {
			return 0, newCLIError(exitcode.Fatal, "%v", err)
		}
		return code, nil
	}
	fmt.Fprint(stdout, value)
	return code, nil
}

func readRawValue(path, fieldName string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is this command's own os.MkdirTemp file, not user-controlled input
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 || lines[0] == "" {
		return "", fmt.Errorf("--bare requires exactly one URL to be targeted, got %d items", len(lines))
	}
	var item rawItem
	if err := json.Unmarshal([]byte(lines[0]), &item); err != nil {
		return "", err
	}
	if fieldName == "" {
		return item.Body, nil
	}
	return extractRawValue(item.Data, fieldName)
}

func extractRawValue(data map[string]any, fieldName string) (string, error) {
	v, ok := data[fieldName]
	if !ok {
		return "", fmt.Errorf("--bare: field %q was not produced", fieldName)
	}
	switch val := v.(type) {
	case []any:
		if len(val) != 1 {
			return "", fmt.Errorf("--bare: field %q matched %d values, want exactly 1", fieldName, len(val))
		}
		return fmt.Sprint(val[0]), nil
	default:
		return fmt.Sprint(val), nil
	}
}

type dirIndexEntry struct {
	URL       string `json:"url"`
	File      string `json:"file,omitempty"`
	Succeeded bool   `json:"succeeded"`
	Skipped   bool   `json:"skipped,omitempty"`
	Status    int    `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runJobToDir(ctx context.Context, runner JobRunner, spec *jobspec.Spec, bare bool, fieldName, outDir string, stderr io.Writer) (int, *cliError) {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}

	tmpDir, err := os.MkdirTemp("", "pobear-outdir-*")
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, "out.ndjson")

	raw := *spec
	raw.Sink = jobspec.Sink{Use: "ndjson", Path: tmpPath}

	data, err := yaml.Marshal(&raw)
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}

	code, result, cerr := execAndSummarize(ctx, runner, data, stderr)
	if cerr != nil {
		return 0, cerr
	}

	entries, err := writeDirOutputs(tmpPath, outDir, bare, fieldName, result.Errors, result.SkippedURLs)
	if err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}
	if err := writeDirIndex(outDir, entries); err != nil {
		return 0, newCLIError(exitcode.Fatal, "%v", err)
	}

	return code, nil
}

func writeDirOutputs(ndjsonPath, outDir string, bare bool, fieldName string, failureLines, skippedLines []string) ([]dirIndexEntry, error) {
	data, err := os.ReadFile(ndjsonPath) //nolint:gosec // ndjsonPath is this command's own os.MkdirTemp file, not user-controlled input
	if err != nil {
		return nil, err
	}

	var entries []dirIndexEntry
	n := 0
	for line := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var item rawItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, err
		}
		n++

		if !bare {
			fileName := fmt.Sprintf("%04d.json", n)
			if err := os.WriteFile(filepath.Join(outDir, fileName), []byte(line), 0o600); err != nil {
				return nil, err
			}
			entries = append(entries, dirIndexEntry{URL: item.URL, File: fileName, Succeeded: true, Status: item.Status})
			continue
		}

		var value string
		if fieldName == "" {
			value = item.Body
		} else {
			value, err = extractRawValue(item.Data, fieldName)
			if err != nil {
				entries = append(entries, dirIndexEntry{URL: item.URL, Status: item.Status, Error: fmt.Sprintf("%s: %v", item.URL, err)})
				continue
			}
		}
		fileName := fmt.Sprintf("%04d.html", n)
		if err := os.WriteFile(filepath.Join(outDir, fileName), []byte(value), 0o600); err != nil {
			return nil, err
		}
		entries = append(entries, dirIndexEntry{URL: item.URL, File: fileName, Succeeded: true, Status: item.Status})
	}

	for _, line := range failureLines {
		entries = append(entries, dirIndexEntry{URL: urlFromFailureLine(line), Error: line})
	}
	for _, line := range skippedLines {
		entries = append(entries, dirIndexEntry{URL: urlFromFailureLine(line), Skipped: true, Error: line})
	}

	return entries, nil
}

func urlFromFailureLine(line string) string {
	before, _, found := strings.Cut(line, " ")
	if !found {
		return line
	}
	return strings.TrimSuffix(before, ":")
}

func writeDirIndex(outDir string, entries []dirIndexEntry) error {
	f, err := os.OpenFile(filepath.Join(outDir, "index.json"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // outDir is this command's own --out directory, index.json is a fixed literal name
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}
