package tiers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/services/core/internal/fetch/tiers"
)

type fakeManifestLookup struct {
	byName map[string]*provider.Manifest
}

func (f fakeManifestLookup) Lookup(name string) (*provider.Manifest, bool) {
	m, ok := f.byName[name]
	return m, ok
}

type fakeCredentialLoader struct {
	secret string
	ok     bool
	err    error
}

func (f fakeCredentialLoader) Load(_, _, _ string) (string, bool, error) {
	return f.secret, f.ok, f.err
}

func testManifest(endpoint string) *provider.Manifest {
	return &provider.Manifest{
		Name:     "acme",
		Endpoint: endpoint,
		Auth:     provider.Auth{Type: "bearer", Credential: "token"},
		Request:  provider.Request{Body: map[string]any{"url": "{{ .URL }}"}},
		Response: provider.Response{Body: "$.content"},
		Errors: []provider.ErrorMapping{
			{Status: 401, Kind: "auth"},
			{Status: 429, Kind: "rateLimit"},
		},
	}
}

func TestProviderFetchSuccess(t *testing.T) {
	var gotAuth atomic.Value
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		gotBody.Store(string(body))
		_, _ = w.Write([]byte(`{"content":"the page"}`))
	}))
	defer srv.Close()

	p := &tiers.Provider{
		Client:      srv.Client(),
		Manifests:   fakeManifestLookup{byName: map[string]*provider.Manifest{"acme": testManifest(srv.URL)}},
		Credentials: fakeCredentialLoader{secret: "sekret", ok: true},
	}
	cfg := jobspec.Fetch{Use: "provider", Name: "acme"}
	result, gotFault, err := p.Fetch(context.Background(), cfg, "https://target.example/page")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || string(result.Body) != "the page" {
		t.Errorf("result = %+v, want body \"the page\"", result)
	}
	if gotFault != "" {
		t.Errorf("fault = %q, want empty", gotFault)
	}

	if auth, _ := gotAuth.Load().(string); auth != "Bearer sekret" {
		t.Errorf("Authorization header = %q, want %q", auth, "Bearer sekret")
	}

	rawBody, _ := gotBody.Load().(string)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(rawBody), &decoded); err != nil {
		t.Fatalf("decode request body: %v (raw=%q)", err, rawBody)
	}
	if decoded["url"] != "https://target.example/page" {
		t.Errorf("request body url = %v, want %q", decoded["url"], "https://target.example/page")
	}
}

func TestProviderFetchUnknownManifest(t *testing.T) {
	p := &tiers.Provider{
		Manifests:   fakeManifestLookup{byName: map[string]*provider.Manifest{}},
		Credentials: fakeCredentialLoader{},
	}
	cfg := jobspec.Fetch{Use: "provider", Name: "missing"}
	_, _, err := p.Fetch(context.Background(), cfg, "https://target.example/page")

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestProviderFetchMissingCredential(t *testing.T) {
	var reachedUpstream atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedUpstream.Store(true)
	}))
	defer srv.Close()

	p := &tiers.Provider{
		Client:      srv.Client(),
		Manifests:   fakeManifestLookup{byName: map[string]*provider.Manifest{"acme": testManifest(srv.URL)}},
		Credentials: fakeCredentialLoader{ok: false},
	}
	cfg := jobspec.Fetch{Use: "provider", Name: "acme"}
	_, gotFault, err := p.Fetch(context.Background(), cfg, "https://target.example/page")

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if gotFault != fault.Auth {
		t.Errorf("fault = %q, want %q", gotFault, fault.Auth)
	}
	if reachedUpstream.Load() {
		t.Error("request should never reach the upstream when the credential is missing")
	}
}

func TestProviderFetchErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	manifest := testManifest(srv.URL)
	manifest.Errors = []provider.ErrorMapping{
		{Status: 500, Kind: "blocked"},
	}

	p := &tiers.Provider{
		Client:      srv.Client(),
		Manifests:   fakeManifestLookup{byName: map[string]*provider.Manifest{"acme": manifest}},
		Credentials: fakeCredentialLoader{secret: "sekret", ok: true},
	}
	cfg := jobspec.Fetch{Use: "provider", Name: "acme"}
	_, gotFault, err := p.Fetch(context.Background(), cfg, "https://target.example/page")

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if gotFault != fault.Blocked {
		t.Errorf("fault = %q, want %q (per the manifest's errors[] mapping status 500 -> blocked)", gotFault, fault.Blocked)
	}
}

func TestProviderFetchFallsBackToDefaultClassificationWhenUnmapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	p := &tiers.Provider{
		Client:      srv.Client(),
		Credentials: fakeCredentialLoader{secret: "sekret", ok: true},
	}
	cfg := jobspec.Fetch{Use: "provider", Name: "acme"}
	_, gotFault, err := p.Fetch(context.Background(), cfg, "https://target.example/page")

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if gotFault != fault.Blocked {
		t.Errorf("fault = %q, want %q", gotFault, fault.Blocked)
	}
}

func TestProviderFetchUsesLogicalStatusFromResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":"the page","logicalStatus":429}`))
	}))
	defer srv.Close()

	manifest := testManifest(srv.URL)
	manifest.Response.Status = "$.logicalStatus"
	manifest.Errors = []provider.ErrorMapping{
		{Status: 429, Kind: "rateLimit"},
	}

	p := &tiers.Provider{
		Client:      srv.Client(),
		Manifests:   fakeManifestLookup{byName: map[string]*provider.Manifest{"acme": manifest}},
		Credentials: fakeCredentialLoader{secret: "sekret", ok: true},
	}
	cfg := jobspec.Fetch{Use: "provider", Name: "acme"}
	_, gotFault, err := p.Fetch(context.Background(), cfg, "https://target.example/page")

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if gotFault != fault.RateLimit {
		t.Errorf("fault = %q, want %q", gotFault, fault.RateLimit)
	}
}

func TestProviderFetchSendsHeaders(t *testing.T) {
	var gotHeader atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader.Store(r.Header.Get("X-Test-Header"))
		_, _ = w.Write([]byte(`{"content":"the page"}`))
	}))
	defer srv.Close()

	p := &tiers.Provider{
		Client:      srv.Client(),
		Manifests:   fakeManifestLookup{byName: map[string]*provider.Manifest{"acme": testManifest(srv.URL)}},
		Credentials: fakeCredentialLoader{secret: "sekret", ok: true},
	}
	cfg := jobspec.Fetch{Use: "provider", Name: "acme", Headers: map[string]string{"X-Test-Header": "custom-value"}}
	_, _, err := p.Fetch(context.Background(), cfg, "https://target.example/page")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got, _ := gotHeader.Load().(string); got != "custom-value" {
		t.Errorf("X-Test-Header = %q, want %q", got, "custom-value")
	}
}
