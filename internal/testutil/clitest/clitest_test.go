package clitest_test

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/internal/testutil/clitest"
)

type testCase struct {
	Args   string `yaml:"args"`
	Stdout string `yaml:"stdout"`
	Stderr string `yaml:"stderr"`
	Exit   int    `yaml:"exit"`
}

func TestRunMatchingExpectationPasses(t *testing.T) {
	binPath := clitest.Build(t)
	path := writeCase(t, testCase{Args: "version", Stdout: "pobear dev\n", Exit: 0})

	clitest.Run(t, binPath, path)
}

func TestRunMismatchedExitCodeFails(t *testing.T) {
	binPath := clitest.Build(t)
	path := writeCase(t, testCase{Args: "version", Stdout: "pobear dev\n", Exit: 1})

	inner := &testing.T{}
	clitest.Run(inner, binPath, path)
	if !inner.Failed() {
		t.Fatalf("expected the inner test to fail on a mismatched exit code, it passed")
	}
}

func TestRunSubstitutesBaseInArgsAndStderr(t *testing.T) {
	binPath := clitest.Build(t)
	path := writeCase(t, testCase{Args: "scrape {{base}}/page.html", Stderr: "{{base}}/page.html [http]: tiers: http status 404: 404 page not found\nfetched 0, errors 1, skipped 0\n", Exit: 5})

	clitest.Run(t, binPath, path)
}

func writeCase(t *testing.T, c testCase) string {
	t.Helper()
	data, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal case: %v", err)
	}
	path := filepath.Join(t.TempDir(), "case.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
