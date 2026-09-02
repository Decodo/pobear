package goldencli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/internal/testutil"
)

type caseFile struct {
	Args      string            `yaml:"args"`
	Stdin     string            `yaml:"stdin,omitempty"`
	Stdout    string            `yaml:"stdout"`
	Stderr    string            `yaml:"stderr"`
	Exit      int               `yaml:"exit"`
	Providers map[string]string `yaml:"providers,omitempty"`
	Fixtures  map[string]string `yaml:"fixtures,omitempty"`
}

func Build(t testing.TB) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "pobear")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/pobear/pobear/cmd/core") //nolint:gosec // binPath is this test run's own t.TempDir() path, the build target is a fixed literal
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build cmd/core: %v\n%s", err, out)
	}
	return binPath
}

func Run(t *testing.T, binPath, path string) {
	t.Helper()

	raw, err := os.ReadFile(path) //nolint:gosec // path comes from a fixed, repo-local testdata walk
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var c caseFile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	fixturesDir := t.TempDir()
	for name, content := range c.Fixtures {
		writeNamed(t, fixturesDir, name, content)
	}
	srv := testutil.NewFixtureServer(fixturesDir)
	defer srv.Close()

	homeDir := t.TempDir()

	runDir := t.TempDir()
	if len(c.Providers) > 0 {
		providersDir := filepath.Join(runDir, "providers")
		if err := os.MkdirAll(providersDir, 0o750); err != nil {
			t.Fatalf("creating providers dir: %v", err)
		}
		for name, content := range c.Providers {
			writeNamed(t, providersDir, name, content)
		}
	}

	args := strings.Fields(substitute(c.Args, srv.URL, homeDir))
	wantStdout := substitute(c.Stdout, srv.URL, homeDir)
	wantStderr := substitute(c.Stderr, srv.URL, homeDir)

	cmd := exec.Command(binPath, args...) //nolint:gosec // binPath is this test run's own built binary, args come from repo-local testdata
	cmd.Dir = runDir
	cmd.Env = append(isolatedEnv(), "HOME="+homeDir)
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	gotExit := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			gotExit = exitErr.ExitCode()
		} else {
			t.Fatalf("running binary: %v", runErr)
		}
	}

	if gotExit != c.Exit {
		t.Errorf("exit code = %d, want %d", gotExit, c.Exit)
	}
	if stdout.String() != wantStdout {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout.String(), wantStdout)
	}
	if stderr.String() != wantStderr {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr.String(), wantStderr)
	}
}

func writeNamed(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name) //nolint:gosec // name is a fixed key from repo-local testdata, not user input
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("creating dir for %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", full, err)
	}
}

func substitute(s, base, home string) string {
	s = strings.ReplaceAll(s, "{{base}}", base)
	return strings.ReplaceAll(s, "{{home}}", home)
}

func isolatedEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "POBEAR_") {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}
