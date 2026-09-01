package testutil

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
)

func NewFixtureServer(dir string) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		full := filepath.Join(dir, filepath.Clean(r.URL.Path))
		data, err := os.ReadFile(full) //nolint:gosec // full is derived from dir (test-controlled) plus a cleaned request path; filepath.Clean on a rooted path can never produce a ".." that escapes dir
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data) //nolint:gosec // fixture files are authored by the test suite itself, not user input
	})
	return httptest.NewServer(handler)
}
