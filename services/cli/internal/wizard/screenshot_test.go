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

// Each screenshot file is self-contained: a "key=value" precondition line, a
// "---" separator, then the exact plain-text (ANSI-stripped) terminal frame
// renderMenu(width, height, cursor) should produce -- so opening the file
// shows both what was asked for and what should appear, with no need to
// cross-reference Go code.
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
