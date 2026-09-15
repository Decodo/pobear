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

var updateScreenshots = flag.Bool("update", false, "regenerate testdata/menu screenshots from the current renderMenu output")

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
	data, err := os.ReadFile(path) //nolint:gosec // path always comes from filepath.Glob("testdata/menu/*.txt") above, never external input
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	header, body, ok := strings.Cut(string(data), "\n---\n")
	if !ok {
		t.Fatalf("%s: missing '---' separator between precondition and expected view", path)
	}
	body = strings.TrimSuffix(body, "\n")

	values := map[string]int{}
	for tok := range strings.FieldsSeq(header) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			t.Fatalf("%s: malformed precondition token %q (want key=value)", path, tok)
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("%s: precondition %q is not a number", path, tok)
		}
		values[k] = n
	}
	for _, key := range []string{"width", "height", "cursor"} {
		if _, ok := values[key]; !ok {
			t.Fatalf("%s: precondition is missing %q", path, key)
		}
	}
	return values["width"], values["height"], values["cursor"], body
}

func writeScreenshot(t *testing.T, path string, width, height, cursor int, view string) {
	t.Helper()
	content := fmt.Sprintf("width=%d height=%d cursor=%d\n---\n%s\n", width, height, cursor, view)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	t.Logf("updated %s", path)
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
			got := ansi.Strip(renderForm(width, height, fields, cursor))

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
	data, err := os.ReadFile(path) //nolint:gosec // path always comes from filepath.Glob("testdata/form/*.txt") above, never external input
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	header, body, ok := strings.Cut(string(data), "\n---\n")
	if !ok {
		t.Fatalf("%s: missing '---' separator between precondition and expected view", path)
	}
	body = strings.TrimSuffix(body, "\n")

	state = newFormState()
	values := map[string]string{}
	for tok := range strings.FieldsSeq(header) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			t.Fatalf("%s: malformed precondition token %q (want key=value)", path, tok)
		}
		values[k] = v
	}

	width = mustAtoi(t, path, values, "width")
	height = mustAtoi(t, path, values, "height")
	cursor = mustAtoi(t, path, values, "cursor")

	if v, ok := values["fetchVia"]; ok {
		state.fetchVia = v
	}
	if v, ok := values["extract"]; ok {
		state.extract = strings.ReplaceAll(v, "-", " ")
	}
	if v, ok := values["cssFields"]; ok && v != "" {
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

func writeFormScreenshot(t *testing.T, path string, width, height, cursor int, state formState, view string) {
	t.Helper()
	content := fmt.Sprintf("width=%d height=%d cursor=%d fetchVia=%s extract=%s cssFields=%s\n---\n%s\n",
		width, height, cursor, state.fetchVia, strings.ReplaceAll(state.extract, " ", "-"), encodeCSSFields(state.cssFields), view)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	t.Logf("updated %s", path)
}
