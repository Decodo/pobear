package wizard

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var updateScreenshots = flag.Bool("update", false, "regenerate testdata screenshots from the current render output")

func readScreenshotFile(t *testing.T, path string) (tokens map[string]string, body string) {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // path always comes from filepath.Glob("testdata/.../*.txt") above, never external input
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	header, b, ok := strings.Cut(string(data), "\n---\n")
	if !ok {
		t.Fatalf("%s: missing '---' separator between precondition and expected view", path)
	}
	body = strings.TrimSuffix(b, "\n")

	tokens = map[string]string{}
	for tok := range strings.FieldsSeq(header) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			t.Fatalf("%s: malformed precondition token %q (want key=value)", path, tok)
		}
		tokens[k] = v
	}
	return tokens, body
}

func writeScreenshotFile(t *testing.T, path, header, view string) {
	t.Helper()
	content := header + "\n---\n" + view + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	t.Logf("updated %s", path)
}

func mustAtoi(t *testing.T, path string, values map[string]string, key string) int {
	t.Helper()
	v, ok := values[key]
	if !ok {
		t.Fatalf("%s: precondition is missing %q", path, key)
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("%s: precondition %q is not a number", path, key)
	}
	return n
}

func TestMenuScreenshotScenarios(t *testing.T) {
	paths, err := filepath.Glob("testdata/menu/*.txt")
	if err != nil {
		t.Fatalf("globbing testdata/menu: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no screenshot files found under testdata/menu")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			width, height, cursor, want := readScreenshot(t, path)
			got := ansi.Strip(renderMenu(width, height, cursor, false))

			if *updateScreenshots {
				writeScreenshot(t, path, width, height, cursor, got)
				return
			}
			if got != want {
				t.Errorf("renderMenu(%d, %d, %d) doesn't match %s (run with -update to regenerate)\n--- want ---\n%s\n--- got ---\n%s",
					width, height, cursor, path, want, got)
			}
		})
	}
}

func readScreenshot(t *testing.T, path string) (width, height, cursor int, view string) {
	t.Helper()
	tokens, body := readScreenshotFile(t, path)
	width = mustAtoi(t, path, tokens, "width")
	height = mustAtoi(t, path, tokens, "height")
	cursor = mustAtoi(t, path, tokens, "cursor")
	return width, height, cursor, body
}

func writeScreenshot(t *testing.T, path string, width, height, cursor int, view string) {
	t.Helper()
	header := fmt.Sprintf("width=%d height=%d cursor=%d", width, height, cursor)
	writeScreenshotFile(t, path, header, view)
}

func TestFormScreenshotScenarios(t *testing.T) {
	paths, err := filepath.Glob("testdata/form/*.txt")
	if err != nil {
		t.Fatalf("globbing testdata/form: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no screenshot files found under testdata/form")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			width, height, cursor, state, want := readFormScreenshot(t, path)
			fields := computeFields(state, []string{"acme", "decodo"})
			got := ansi.Strip(renderForm(width, height, fields, cursor, ""))

			if *updateScreenshots {
				writeFormScreenshot(t, path, width, height, cursor, state, got)
				return
			}
			if got != want {
				t.Errorf("renderForm doesn't match %s (run with -update to regenerate)\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
			}
		})
	}
}

func readFormScreenshot(t *testing.T, path string) (width, height, cursor int, state formState, view string) {
	t.Helper()
	tokens, body := readScreenshotFile(t, path)

	state = newFormState()
	width = mustAtoi(t, path, tokens, "width")
	height = mustAtoi(t, path, tokens, "height")
	cursor = mustAtoi(t, path, tokens, "cursor")

	if v, ok := tokens["fetchVia"]; ok {
		state.fetchVia = v
	}
	if v, ok := tokens["extract"]; ok {
		state.extract = strings.ReplaceAll(v, "-", " ")
	}
	if v, ok := tokens["cssFields"]; ok && v != "" {
		state.cssFields = parseCSSFields(t, path, v)
	}
	return width, height, cursor, state, body
}

func parseCSSFields(t *testing.T, path, encoded string) []cssFieldPair {
	t.Helper()
	var pairs []cssFieldPair
	for entry := range strings.SplitSeq(encoded, "|") {
		name, selector, ok := strings.Cut(entry, ":")
		if !ok {
			t.Fatalf("%s: malformed cssFields entry %q (want name:selector)", path, entry)
		}
		pairs = append(pairs, cssFieldPair{name: name, selector: selector})
	}
	return pairs
}

func encodeCSSFields(pairs []cssFieldPair) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.name + ":" + p.selector
	}
	return strings.Join(parts, "|")
}

func writeFormScreenshot(t *testing.T, path string, width, height, cursor int, state formState, view string) {
	t.Helper()
	header := fmt.Sprintf("width=%d height=%d cursor=%d fetchVia=%s extract=%s cssFields=%s",
		width, height, cursor, state.fetchVia, strings.ReplaceAll(state.extract, " ", "-"), encodeCSSFields(state.cssFields))
	writeScreenshotFile(t, path, header, view)
}

func TestRunningScreenshotScenarios(t *testing.T) {
	paths, err := filepath.Glob("testdata/running/*.txt")
	if err != nil {
		t.Fatalf("globbing testdata/running: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no screenshot files found under testdata/running")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			width, height, completed, total, want := readRunningScreenshot(t, path)
			progress := newRunProgress(total)
			progress.completed.Store(int64(completed))
			s := &runningScreen{progress: progress, width: width, height: height}
			got := ansi.Strip(s.View())

			if *updateScreenshots {
				writeRunningScreenshot(t, path, width, height, completed, total, got)
				return
			}
			if got != want {
				t.Errorf("runningScreen.View() doesn't match %s (run with -update to regenerate)\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
			}
		})
	}
}

func readRunningScreenshot(t *testing.T, path string) (width, height, completed, total int, view string) {
	t.Helper()
	tokens, body := readScreenshotFile(t, path)
	width = mustAtoi(t, path, tokens, "width")
	height = mustAtoi(t, path, tokens, "height")
	completed = mustAtoi(t, path, tokens, "completed")
	total = mustAtoi(t, path, tokens, "total")
	return width, height, completed, total, body
}

func writeRunningScreenshot(t *testing.T, path string, width, height, completed, total int, view string) {
	t.Helper()
	header := fmt.Sprintf("width=%d height=%d completed=%d total=%d", width, height, completed, total)
	writeScreenshotFile(t, path, header, view)
}

func sampleResultRows(n int) []resultRow {
	rows := make([]resultRow, n)
	for i := range rows {
		rows[i] = resultRow{
			url:    fmt.Sprintf("https://a.test/page%d", i),
			status: "✓ 200",
			detail: "markdown: # hello",
		}
	}
	return rows
}

func TestResultsScreenshotScenarios(t *testing.T) {
	paths, err := filepath.Glob("testdata/results/*.txt")
	if err != nil {
		t.Fatalf("globbing testdata/results: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no screenshot files found under testdata/results")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			width, height, cursor, rows, withStatus, want := readResultsScreenshot(t, path)
			s := &resultsScreen{
				rows:    sampleResultRows(rows),
				summary: fmt.Sprintf("✓ fetched %d, errors 0, skipped 0", rows),
				cursor:  cursor,
			}
			if withStatus {
				s.status = "✓ wrote 1 item(s) to results/"
			}
			s.width, s.height = width, height
			s.clampScroll()
			got := ansi.Strip(s.View())

			if *updateScreenshots {
				writeResultsScreenshot(t, path, width, height, cursor, rows, withStatus, got)
				return
			}
			if got != want {
				t.Errorf("resultsScreen.View() doesn't match %s (run with -update to regenerate)\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
			}
		})
	}
}

func readResultsScreenshot(t *testing.T, path string) (width, height, cursor, rows int, withStatus bool, view string) {
	t.Helper()
	tokens, body := readScreenshotFile(t, path)
	width = mustAtoi(t, path, tokens, "width")
	height = mustAtoi(t, path, tokens, "height")
	cursor = mustAtoi(t, path, tokens, "cursor")
	rows = mustAtoi(t, path, tokens, "rows")
	withStatus = tokens["status"] == "1"
	return width, height, cursor, rows, withStatus, body
}

func writeResultsScreenshot(t *testing.T, path string, width, height, cursor, rows int, withStatus bool, view string) {
	t.Helper()
	statusTok := "0"
	if withStatus {
		statusTok = "1"
	}
	header := fmt.Sprintf("width=%d height=%d cursor=%d rows=%d status=%s", width, height, cursor, rows, statusTok)
	writeScreenshotFile(t, path, header, view)
}

func sampleItemRow(kind string) resultRow {
	switch kind {
	case "long":
		return resultRow{
			url: "https://a.test",
			raw: map[string]any{
				"url":    "https://a.test",
				"status": 200,
				"body":   strings.Repeat("lorem ipsum dolor sit amet ", 10),
			},
		}
	case "many":
		data := map[string]any{}
		for i := range 30 {
			data[fmt.Sprintf("field%02d", i)] = i
		}
		return resultRow{url: "https://a.test", raw: data}
	}
	return resultRow{
		url: "https://a.test",
		raw: map[string]any{
			"url":    "https://a.test",
			"status": 200,
		},
	}
}

func TestItemDetailScreenshotScenarios(t *testing.T) {
	paths, err := filepath.Glob("testdata/item/*.txt")
	if err != nil {
		t.Fatalf("globbing testdata/item: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no screenshot files found under testdata/item")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			width, height, wrap, kind, want := readItemDetailScreenshot(t, path)
			s := newItemDetailScreen(sampleItemRow(kind))
			s.wrap = wrap
			s.width, s.height = width, height
			got := ansi.Strip(s.View())

			if *updateScreenshots {
				writeItemDetailScreenshot(t, path, width, height, wrap, kind, got)
				return
			}
			if got != want {
				t.Errorf("itemDetailScreen.View() doesn't match %s (run with -update to regenerate)\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
			}
		})
	}
}

func readItemDetailScreenshot(t *testing.T, path string) (width, height int, wrap bool, kind, view string) {
	t.Helper()
	tokens, body := readScreenshotFile(t, path)
	width = mustAtoi(t, path, tokens, "width")
	height = mustAtoi(t, path, tokens, "height")
	wrap = tokens["wrap"] == "1"
	kind = tokens["kind"]
	return width, height, wrap, kind, body
}

func writeItemDetailScreenshot(t *testing.T, path string, width, height int, wrap bool, kind, view string) {
	t.Helper()
	wrapTok := "0"
	if wrap {
		wrapTok = "1"
	}
	header := fmt.Sprintf("width=%d height=%d wrap=%s kind=%s", width, height, wrapTok, kind)
	writeScreenshotFile(t, path, header, view)
}
