package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Decodo/pobear/pkg/provider"
)

const acmeManifest = `
name: acme
endpoint: https://acme.example.com/scrape
auth: {type: bearer, credential: token}
options:
  - {name: device, type: string}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`

func findProviderEntry(entries []providerListEntry, name string) (providerListEntry, bool) {
	for _, e := range entries {
		if e.manifest.Name == name {
			return e, true
		}
	}
	return providerListEntry{}, false
}

func TestLoadProviderEntriesIncludesEmbeddedAndDisk(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)

	entries, err := loadProviderEntries(dir, filepath.Join(t.TempDir(), "credentials"))
	if err != nil {
		t.Fatalf("loadProviderEntries: %v", err)
	}
	if _, ok := findProviderEntry(entries, "decodo"); !ok {
		t.Errorf("entries = %v, want the embedded decodo manifest present", entries)
	}
	acme, ok := findProviderEntry(entries, "acme")
	if !ok {
		t.Fatalf("entries = %v, want the on-disk acme manifest present", entries)
	}
	if acme.diskPath == "" {
		t.Error("acme.diskPath is empty, want it set (it came from disk)")
	}
	decodo, _ := findProviderEntry(entries, "decodo")
	if decodo.diskPath != "" {
		t.Errorf("decodo.diskPath = %q, want empty (it's only embedded, no on-disk file)", decodo.diskPath)
	}
}

func TestLoadProviderEntriesMarksCredentialStatus(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	credPath := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(credPath, []byte("POBEAR_PROVIDER_ACME_TOKEN=secret\n"), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	entries, err := loadProviderEntries(dir, credPath)
	if err != nil {
		t.Fatalf("loadProviderEntries: %v", err)
	}
	acme, ok := findProviderEntry(entries, "acme")
	if !ok {
		t.Fatal("entries has no acme")
	}
	if !acme.hasCred {
		t.Error("acme.hasCred = false, want true (credential was stored)")
	}
	decodo, ok := findProviderEntry(entries, "decodo")
	if !ok {
		t.Fatal("entries has no decodo")
	}
	if decodo.hasCred {
		t.Error("decodo.hasCred = true, want false (no credential stored for it)")
	}
}

func TestDiskManifestPathsSkipsNonManifestFiles(t *testing.T) {
	dir := t.TempDir()
	writeManifest := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeManifest("acme.yaml", acmeManifest)
	writeManifest("README.md", "not a manifest")

	paths, err := diskManifestPaths(dir)
	if err != nil {
		t.Fatalf("diskManifestPaths: %v", err)
	}
	if len(paths) != 1 || paths["acme"] == "" {
		t.Fatalf("paths = %v, want just {acme: <path>}", paths)
	}
}

func TestProvidersScreenEnterPushesDetailScreen(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	s := newProvidersScreen(dir, filepath.Join(t.TempDir(), "credentials"))
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to return a push command, got nil")
	}
	msg := cmd()
	push, ok := msg.(pushScreenMsg)
	if !ok {
		t.Fatalf("expected a pushScreenMsg, got %T", msg)
	}
	if _, ok := push.screen.(*providerDetailScreen); !ok {
		t.Fatalf("expected the pushed screen to be *providerDetailScreen, got %T", push.screen)
	}
}

func TestProvidersScreenDeleteRequiresConfirmation(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	s := newProvidersScreen(dir, filepath.Join(t.TempDir(), "credentials"))
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	for i, e := range s.entries {
		if e.manifest.Name == "acme" {
			s.cursor = i
			break
		}
	}

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	s = updated.(*providersScreen)
	if !s.confirming {
		t.Fatal("expected 'd' to enter confirm-delete mode")
	}
	if _, err := os.Stat(filepath.Join(dir, "acme.yaml")); err != nil {
		t.Fatalf("acme.yaml should still exist before confirming: %v", err)
	}

	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	s = updated.(*providersScreen)
	if _, err := os.Stat(filepath.Join(dir, "acme.yaml")); !os.IsNotExist(err) {
		t.Fatalf("acme.yaml should be deleted after confirming, stat err = %v", err)
	}
	if _, ok := findProviderEntry(s.entries, "acme"); ok {
		t.Error("acme should no longer be in entries after deletion")
	}
	if _, ok := findProviderEntry(s.entries, "decodo"); !ok {
		t.Error("decodo (embedded) should still be in entries after deleting acme")
	}
}

func TestProvidersScreenDeleteCancelledOnN(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	s := newProvidersScreen(dir, filepath.Join(t.TempDir(), "credentials"))
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	s = updated.(*providersScreen)
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	s = updated.(*providersScreen)

	if s.confirming {
		t.Error("expected 'n' to cancel confirm-delete mode")
	}
	if _, err := os.Stat(filepath.Join(dir, "acme.yaml")); err != nil {
		t.Fatalf("acme.yaml should still exist after cancelling: %v", err)
	}
}

func TestProvidersScreenDeleteIsNoOpForEmbeddedOnlyProvider(t *testing.T) {
	s := newProvidersScreen(t.TempDir(), filepath.Join(t.TempDir(), "credentials"))
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	for i, e := range s.entries {
		if e.manifest.Name == "decodo" {
			s.cursor = i
			break
		}
	}

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	s = updated.(*providersScreen)
	if s.confirming {
		t.Error("expected 'd' on an embedded-only provider to do nothing (nothing on disk to delete)")
	}
}

func TestProvidersScreenAuthStartsEntering(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	s := newProvidersScreen(dir, filepath.Join(t.TempDir(), "credentials"))
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	s = updated.(*providersScreen)
	if !s.entering {
		t.Fatal("expected 'a' to start the credential-entry mode")
	}
}

func TestProvidersScreenAuthSubmitSavesAndMarksStored(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	credPath := filepath.Join(t.TempDir(), "credentials")
	s := newProvidersScreen(dir, credPath)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	for i, e := range s.entries {
		if e.manifest.Name == "acme" {
			s.cursor = i
			break
		}
	}

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	s = updated.(*providersScreen)
	for _, r := range "s3cr3t" {
		updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		s = updated.(*providersScreen)
	}
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s = updated.(*providersScreen)

	if s.entering {
		t.Fatal("expected submitting to leave entering mode")
	}
	if !strings.HasPrefix(s.status, "✓") {
		t.Fatalf("status after submit = %q, want success", s.status)
	}
	if !s.entries[s.cursor].hasCred {
		t.Error("expected entries[cursor].hasCred to be true immediately after saving, without a reload")
	}

	data, err := os.ReadFile(credPath) //nolint:gosec // credPath is this test's own t.TempDir() output file
	if err != nil {
		t.Fatalf("reading %s: %v", credPath, err)
	}
	if !strings.Contains(string(data), "POBEAR_PROVIDER_ACME_TOKEN=s3cr3t") {
		t.Errorf("%s = %q, want the saved credential", credPath, data)
	}
}

func TestProvidersScreenAuthEscCancelsEnteringWithoutSaving(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	credPath := filepath.Join(t.TempDir(), "credentials")
	s := newProvidersScreen(dir, credPath)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	s = updated.(*providersScreen)
	for _, r := range "abc" {
		updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		s = updated.(*providersScreen)
	}
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	s = updated.(*providersScreen)

	if s.entering {
		t.Fatal("expected esc to leave entering mode")
	}
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Fatalf("credentials file should not exist after cancelling, stat err = %v", err)
	}
}

func TestProvidersScreenAuthEmptySecretIsRejected(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	credPath := filepath.Join(t.TempDir(), "credentials")
	s := newProvidersScreen(dir, credPath)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	updated, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	s = updated.(*providersScreen)
	updated, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s = updated.(*providersScreen)

	if !strings.HasPrefix(s.status, "✗") {
		t.Fatalf("status after empty submit = %q, want a failure", s.status)
	}
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Fatalf("credentials file should not exist after an empty submit, stat err = %v", err)
	}
}

func TestProvidersScreenNeverExceedsRequestedSize(t *testing.T) {
	entries := make([]providerListEntry, 60)
	for i := range entries {
		name := string(rune('a'+i%26)) + string(rune('0'+i/26))
		entries[i] = providerListEntry{manifest: &provider.Manifest{Name: name, Endpoint: "https://" + name + ".example.com/scrape"}}
	}
	s := &providersScreen{entries: entries}

	assertNeverExceedsSize(t, providersMinHeight, 40, 3, providersMinWidth, 15, func(w, h int) string {
		s.width, s.height = w, h
		s.clampScroll()
		return s.View()
	})
}

func TestProvidersScreenEscPopsScreen(t *testing.T) {
	s := newProvidersScreen(t.TempDir(), filepath.Join(t.TempDir(), "credentials"))
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to return a pop command, got nil")
	}
}

func TestFormatProviderRowShowsCredentialStatus(t *testing.T) {
	dir := writeManifest(t, "acme", acmeManifest)
	entries, err := loadProviderEntries(dir, filepath.Join(t.TempDir(), "credentials"))
	if err != nil {
		t.Fatalf("loadProviderEntries: %v", err)
	}
	acme, _ := findProviderEntry(entries, "acme")

	line := formatProviderRow(acme, false, 80)
	if !strings.Contains(line, "✗") {
		t.Errorf("formatProviderRow(no credential) = %q, want it to show ✗", line)
	}
}
