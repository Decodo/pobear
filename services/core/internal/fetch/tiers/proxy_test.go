package tiers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/fetch/tiers"
)

func forwardingProxy(t *testing.T, hit *atomic.Bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		resp, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		buf := make([]byte, 4096)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
			}
			if rerr != nil {
				break
			}
		}
	}))
}

func newProxyTier(t *testing.T, proxyURL string) *tiers.Proxy {
	t.Helper()
	client, err := tiers.NewFingerprintedClient("chrome", proxyURL)
	if err != nil {
		t.Fatalf("NewFingerprintedClient: %v", err)
	}
	return &tiers.Proxy{Client: client}
}

func TestProxyFetchRoutesThroughConfiguredProxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("via-proxy"))
	}))
	defer target.Close()

	var hit atomic.Bool
	proxy := forwardingProxy(t, &hit)
	defer proxy.Close()

	p := newProxyTier(t, proxy.URL)
	result, gotFault, err := p.Fetch(context.Background(), jobspec.Fetch{}, target.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.Status != 200 || string(result.Body) != "via-proxy" {
		t.Errorf("result = %+v, want status 200 body \"via-proxy\"", result)
	}
	if gotFault != "" {
		t.Errorf("fault = %q, want empty", gotFault)
	}
	if !hit.Load() {
		t.Error("expected the configured proxy to receive the request, it never did")
	}
}

func TestProxyFetchClassifiesFaultsLikeLocal(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantFault fault.Kind
	}{
		{name: "404-is-http", status: 404, wantFault: fault.HTTP},
		{name: "403-is-blocked", status: 403, wantFault: fault.Blocked},
		{name: "429-is-rate-limit", status: 429, wantFault: fault.RateLimit},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
			}))
			defer target.Close()

			var hit atomic.Bool
			proxy := forwardingProxy(t, &hit)
			defer proxy.Close()

			p := newProxyTier(t, proxy.URL)
			_, gotFault, err := p.Fetch(context.Background(), jobspec.Fetch{}, target.URL)

			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if gotFault != c.wantFault {
				t.Errorf("fault = %q, want %q", gotFault, c.wantFault)
			}
			if !hit.Load() {
				t.Error("expected the configured proxy to receive the request, it never did — cfg.Proxy may not be honored")
			}
		})
	}
}

func TestProxyFetchProxyUnreachable(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("should not be reached"))
	}))
	defer target.Close()

	p := newProxyTier(t, "http://127.0.0.1:1") // nothing listens on port 1
	_, gotFault, err := p.Fetch(context.Background(), jobspec.Fetch{}, target.URL)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if gotFault != fault.Network {
		t.Errorf("fault = %q, want %q", gotFault, fault.Network)
	}
}

func TestProxyFetchTimeout(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer target.Close()

	var hit atomic.Bool
	proxy := forwardingProxy(t, &hit)
	defer proxy.Close()

	p := newProxyTier(t, proxy.URL)
	_, gotFault, err := p.Fetch(context.Background(), jobspec.Fetch{Timeout: "50ms"}, target.URL)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if gotFault != fault.Timeout {
		t.Errorf("fault = %q, want %q", gotFault, fault.Timeout)
	}
}

func TestProxyFetchSendsHeaders(t *testing.T) {
	var gotHeader atomic.Value
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader.Store(r.Header.Get("X-Test-Header"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	var hit atomic.Bool
	proxy := forwardingProxy(t, &hit)
	defer proxy.Close()

	p := newProxyTier(t, proxy.URL)
	_, _, err := p.Fetch(context.Background(), jobspec.Fetch{Headers: map[string]string{"X-Test-Header": "custom-value"}}, target.URL)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got, _ := gotHeader.Load().(string); got != "custom-value" {
		t.Errorf("X-Test-Header = %q, want %q", got, "custom-value")
	}
	if !hit.Load() {
		t.Error("expected the configured proxy to receive the request, it never did")
	}
}
