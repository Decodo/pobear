package command

import (
	"testing"

	"github.com/pobear/pobear/pkg/exitcode"
)

func TestResolveFetchUseLadderAlwaysStartsAtLocal(t *testing.T) {
	tests := []struct {
		name     string
		proxy    string
		provider string
	}{
		{"provider only", "", "acme"},
		{"proxy only", "http://p.test", ""},
		{"both proxy and provider", "http://p.test", "acme"},
		{"neither -- a harmless no-op ladder", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &commonFlags{proxy: tt.proxy, provider: tt.provider, ladder: true}
			got, err := resolveFetchUse(f)
			if err != nil {
				t.Fatalf("resolveFetchUse: %v", err)
			}
			if got != "local" {
				t.Errorf("fetchUse = %q, want %q", got, "local")
			}
		})
	}
}

func TestResolveFetchUseWithoutLadderIsUnchanged(t *testing.T) {
	tests := []struct {
		name     string
		proxy    string
		provider string
		want     string
	}{
		{"proxy only", "http://p.test", "", "proxy"},
		{"provider only", "", "acme", "provider"},
		{"neither", "", "", "local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &commonFlags{proxy: tt.proxy, provider: tt.provider}
			got, err := resolveFetchUse(f)
			if err != nil {
				t.Fatalf("resolveFetchUse: %v", err)
			}
			if got != tt.want {
				t.Errorf("fetchUse = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveFetchUseBothWithoutLadderErrors(t *testing.T) {
	f := &commonFlags{proxy: "http://p.test", provider: "acme"}
	_, err := resolveFetchUse(f)
	if err == nil {
		t.Fatal("expected an error when both --proxy and --provider are set without --ladder")
	}
	if err.code != exitcode.UsageError {
		t.Errorf("exit code = %d, want %d", err.code, exitcode.UsageError)
	}
}
