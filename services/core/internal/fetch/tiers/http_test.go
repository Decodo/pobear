package tiers

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
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
