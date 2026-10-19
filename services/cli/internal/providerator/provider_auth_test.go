package providerator_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

func TestDefaultCredentialPathEndsInPobearCredentials(t *testing.T) {
	got, err := providerator.DefaultCredentialPath()
	if err != nil {
		t.Fatalf("DefaultCredentialPath: %v", err)
	}
	if !hasSuffix(got, "/.pobear/credentials") {
		t.Fatalf("DefaultCredentialPath() = %q, want a path ending in /.pobear/credentials", got)
	}
}

func TestAuthPromptsAndSaves(t *testing.T) {
	m := &provider.Manifest{Name: "decodo", Auth: provider.Auth{Type: "bearer", Credential: "token"}}
	var promptedLabel string
	prompt := func(label string) (string, error) {
		promptedLabel = label
		return "secret123", nil
	}
	var stdout bytes.Buffer

	err := providerator.Auth(m, t.TempDir()+"/credentials", prompt, &stdout)
	if err != nil {
		t.Fatalf("Auth: %v", err)
	}
	if promptedLabel != "decodo token" {
		t.Fatalf("prompt label = %q, want %q", promptedLabel, "decodo token")
	}
	if want := "saved POBEAR_PROVIDER_DECODO_TOKEN to "; !hasPrefix(stdout.String(), want) {
		t.Fatalf("stdout = %q, want prefix %q", stdout.String(), want)
	}
}

func TestAuthPropagatesPromptError(t *testing.T) {
	m := &provider.Manifest{Name: "decodo", Auth: provider.Auth{Type: "bearer", Credential: "token"}}
	wantErr := errors.New("stdin closed")
	prompt := func(string) (string, error) { return "", wantErr }

	err := providerator.Auth(m, t.TempDir()+"/credentials", prompt, &bytes.Buffer{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Auth() error = %v, want %v", err, wantErr)
	}
}

func TestCheckCredentialReflectsLoad(t *testing.T) {
	m := &provider.Manifest{Name: "decodo", Auth: provider.Auth{Type: "bearer", Credential: "token"}}

	_, err := providerator.CheckCredential(m, t.TempDir()+"/credentials")
	if err != nil {
		t.Fatalf("CheckCredential: %v", err)
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
