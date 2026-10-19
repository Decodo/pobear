package tiers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
	"github.com/Decodo/pobear/services/core/internal/fetch/tiers"
)

func TestLocalFetch(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		handler    http.HandlerFunc
		cfg        jobspec.Fetch
		wantStatus int
		wantBody   string
		wantFault  fault.Kind
		wantErr    bool
	}{
		{
			name: "200-ok",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("hello"))
			},
			wantStatus: 200,
			wantBody:   "hello",
		},
		{
			name: "404-not-found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
			},
			wantStatus: 404,
			wantFault:  fault.HTTP,
			wantErr:    true,
		},
		{
			name: "403-forbidden-is-blocked",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(403)
			},
			wantStatus: 403,
			wantFault:  fault.Blocked,
			wantErr:    true,
		},
		{
			name: "429-is-rate-limit",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(429)
			},
			wantStatus: 429,
			wantFault:  fault.RateLimit,
			wantErr:    true,
		},
		{
			name: "redirect-followed-by-default",
			path: "/start",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/start" {
					http.Redirect(w, r, "/end", http.StatusFound)
					return
				}
				_, _ = w.Write([]byte("landed"))
			},
			wantStatus: 200,
			wantBody:   "landed",
		},
		{
			name: "redirect-not-followed-when-configured",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/end", http.StatusFound)
			},
			cfg:        jobspec.Fetch{NoRedirects: true},
			wantStatus: http.StatusFound,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(c.handler)
			defer srv.Close()

			l := &tiers.Local{Client: srv.Client()}
			result, gotFault, err := l.Fetch(context.Background(), c.cfg, srv.URL+c.path)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil (result=%+v, fault=%q)", result, gotFault)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
			if gotFault != c.wantFault {
				t.Errorf("fault = %q, want %q", gotFault, c.wantFault)
			}
			if c.wantStatus != 0 {
				if result == nil || result.Status != c.wantStatus {
					t.Errorf("status = %v, want %d", result, c.wantStatus)
				}
			}
			if c.wantBody != "" {
				if result == nil || string(result.Body) != c.wantBody {
					t.Errorf("body = %v, want %q", result, c.wantBody)
				}
			}
		})
	}
}

func TestLocalFetchErrorIncludesResponseBodySnippet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte("blocked: automated traffic detected"))
	}))
	defer srv.Close()

	l := &tiers.Local{Client: srv.Client()}
	_, _, err := l.Fetch(context.Background(), jobspec.Fetch{}, srv.URL)

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if want := "blocked: automated traffic detected"; !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want it to contain the response body %q", err, want)
	}
}

func TestLocalFetchTransportErrors(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		wantFault fault.Kind
		wantErr   bool
	}{
		{
			name:      "connection-refused-is-network",
			url:       "http://127.0.0.1:1", // nothing listens on port 1
			wantFault: fault.Network,
			wantErr:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := &tiers.Local{Client: http.DefaultClient}
			_, gotFault, err := l.Fetch(context.Background(), jobspec.Fetch{}, c.url)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
			if gotFault != c.wantFault {
				t.Errorf("fault = %q, want %q", gotFault, c.wantFault)
			}
		})
	}
}

func TestLocalFetchTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	l := &tiers.Local{Client: srv.Client()}
	cfg := jobspec.Fetch{Timeout: "50ms"}
	_, gotFault, err := l.Fetch(context.Background(), cfg, srv.URL)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if gotFault != fault.Timeout {
		t.Errorf("fault = %q, want %q", gotFault, fault.Timeout)
	}
}

func TestLocalFetchSendsHeaders(t *testing.T) {
	var gotHeader atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader.Store(r.Header.Get("X-Test-Header"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	l := &tiers.Local{Client: srv.Client()}
	cfg := jobspec.Fetch{Headers: map[string]string{"X-Test-Header": "custom-value"}}
	_, _, err := l.Fetch(context.Background(), cfg, srv.URL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got, _ := gotHeader.Load().(string); got != "custom-value" {
		t.Errorf("X-Test-Header = %q, want %q", got, "custom-value")
	}
}
