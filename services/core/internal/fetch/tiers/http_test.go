package tiers

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"syscall"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
)

func TestClassifyDefaultStatus(t *testing.T) {
	cases := []struct {
		status int
		want   fault.Kind
	}{
		{status: 200, want: ""},
		{status: 201, want: ""},
		{status: 299, want: ""},
		{status: 300, want: fault.HTTP},
		{status: 403, want: fault.Blocked},
		{status: 404, want: fault.HTTP},
		{status: 429, want: fault.RateLimit},
		{status: 500, want: fault.HTTP},
	}

	for _, c := range cases {
		t.Run(strconv.Itoa(c.status), func(t *testing.T) {
			got := classifyDefaultStatus(c.status)
			if got != c.want {
				t.Errorf("classifyDefaultStatus(%d) = %q, want %q", c.status, got, c.want)
			}
		})
	}
}

func TestReadBodyAsUTF8DecodesDeclaredCharset(t *testing.T) {
	latin1 := []byte("Caf\xe9 na\xefve")

	cases := []struct {
		name        string
		contentType string
		body        []byte
		want        string
	}{
		{name: "charset in header", contentType: "text/html; charset=ISO-8859-1", body: latin1, want: "Café naïve"},
		{name: "utf-8 passes through", contentType: "text/html; charset=utf-8", body: []byte("Café naïve"), want: "Café naïve"},
		{name: "unknown charset label is sniffed", contentType: "text/html; charset=nonsense-9000", body: latin1, want: "Café naïve"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{"Content-Type": []string{c.contentType}},
				Body:   io.NopCloser(bytes.NewReader(c.body)),
			}
			got, err := readBodyAsUTF8(resp)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if string(got) != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestReadBodyAsUTF8LeavesBinaryBodiesAlone(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0xFF, 0xFE}
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"image/png"}},
		Body:   io.NopCloser(bytes.NewReader(png)),
	}

	got, err := readBodyAsUTF8(resp)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Equal(got, png) {
		t.Errorf("binary body was transcoded: got % x, want % x", got, png)
	}
}

func TestClassifyDefaultStatusMapsAuthStatuses(t *testing.T) {
	for _, status := range []int{401, 407} {
		if got := classifyDefaultStatus(status); got != fault.Auth {
			t.Errorf("classifyDefaultStatus(%d) = %q, want %q", status, got, fault.Auth)
		}
	}
}

func TestTransportFault(t *testing.T) {
	socketTimeout := &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ETIMEDOUT)}
	cases := []struct {
		name string
		err  error
		want fault.Kind
	}{
		{"context deadline", fmt.Errorf("get: %w", context.DeadlineExceeded), fault.Timeout},
		{"socket timeout", socketTimeout, fault.Timeout},
		{"connection refused", &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, fault.Network},
		{"other", errors.New("boom"), fault.Network},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := transportFault(c.err); got != c.want {
				t.Errorf("transportFault(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}

func TestDoHTTPFetchProfileHeaders(t *testing.T) {
	cases := []struct {
		name  string
		cfg   jobspec.Fetch
		check func(t *testing.T, h http.Header)
	}{
		{"chrome sends its defaults", jobspec.Fetch{Profile: "chrome"}, wantProfileDefaults("chrome")},
		{"firefox sends its defaults", jobspec.Fetch{Profile: "firefox"}, wantProfileDefaults("firefox")},
		{"safari sends its defaults and no fetch metadata", jobspec.Fetch{Profile: "safari"}, func(t *testing.T, h http.Header) {
			wantProfileDefaults("safari")(t, h)
			if v := h.Values("Sec-Fetch-Site"); len(v) > 0 {
				t.Errorf("Sec-Fetch-Site = %q, want it absent", v)
			}
		}},
		{"user header overrides a default in any case", jobspec.Fetch{Profile: "chrome", Headers: map[string]string{"user-agent": "x"}}, func(t *testing.T, h http.Header) {
			if v := h.Values("User-Agent"); len(v) != 1 || v[0] != "x" {
				t.Errorf("User-Agent = %q, want exactly [\"x\"]", v)
			}
			if got, want := h.Get("Sec-Ch-Ua"), profileHeaders["chrome"]["Sec-Ch-Ua"]; got != want {
				t.Errorf("Sec-Ch-Ua = %q, want the chrome default %q", got, want)
			}
		}},
		{"no profile sends no defaults", jobspec.Fetch{}, func(t *testing.T, h http.Header) {
			if got := h.Get("User-Agent"); got != "Go-http-client/1.1" {
				t.Errorf("User-Agent = %q, want Go's default", got)
			}
			if v := h.Values("Sec-Ch-Ua"); len(v) > 0 {
				t.Errorf("Sec-Ch-Ua = %q, want it absent", v)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, fetchCapturingHeaders(t, c.cfg))
		})
	}
}

func wantProfileDefaults(profile string) func(t *testing.T, h http.Header) {
	return func(t *testing.T, h http.Header) {
		t.Helper()
		defaults := profileHeaders[profile]
		if len(defaults) == 0 {
			t.Fatalf("no default headers for profile %q", profile)
		}
		for name, want := range defaults {
			if got := h.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	}
}

func fetchCapturingHeaders(t *testing.T, cfg jobspec.Fetch) http.Header {
	t.Helper()
	got := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	l := &Local{Client: srv.Client()}
	if _, _, err := l.Fetch(t.Context(), cfg, srv.URL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	return <-got
}

func TestDoHTTPFetchStillDecodesGzip(t *testing.T) {
	gotAcceptEncoding := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAcceptEncoding <- r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte("hello, decoded"))
		_ = gz.Close()
	}))
	defer srv.Close()

	l := &Local{Client: srv.Client()}
	res, _, err := l.Fetch(t.Context(), jobspec.Fetch{Profile: "chrome"}, srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := <-gotAcceptEncoding; got != "gzip" {
		t.Errorf("Accept-Encoding = %q, want Go's transport-added \"gzip\"", got)
	}
	if got := string(res.Body); got != "hello, decoded" {
		t.Errorf("Body = %q, want the decoded text", got)
	}
}
