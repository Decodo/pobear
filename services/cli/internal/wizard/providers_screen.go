package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pobear/pobear/pkg/credentials"
	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

const (
	providersTitle        = "Manage providers"                                     //nolint:gosec // UI label, not a secret
	providersBackHint     = "esc: back to menu"                                    //nolint:gosec // UI label, not a secret
	providersFooterHint   = "↑/↓ move   enter view   a auth   d delete   esc back" //nolint:gosec // UI label, not a secret
	providersEntryHint    = "enter: save   esc: cancel"
	providersNameWidth    = 13
	providersRowOverhead  = 8
	providersOverheadRows = 7
	providersMinVisible   = 3
	providersWidthMargin  = 4
	providersMinWidth     = providersNameWidth + 30 + providersWidthMargin
	providersMinHeight    = providersOverheadRows + providersMinVisible
)

type providerListEntry struct {
	manifest *provider.Manifest
	hasCred  bool
	diskPath string
}

func diskManifestPaths(providersDir string) (map[string]string, error) {
	entries, err := os.ReadDir(providersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	paths := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(providersDir, entry.Name())
		data, err := os.ReadFile(path) //nolint:gosec // path is built from a directory listing of the caller-supplied providers dir, not user-controlled input
		if err != nil {
			return nil, err
		}
		m, err := provider.Parse(data)
		if err != nil {
			continue
		}
		paths[m.Name] = path
	}
	return paths, nil
}

func loadProviderEntries(providersDir, credentialsPath string) ([]providerListEntry, error) {
	manifests, err := providerator.New(providersDir).Load()
	if err != nil {
		return nil, err
	}
	diskPaths, err := diskManifestPaths(providersDir)
	if err != nil {
		return nil, err
	}

	entries := make([]providerListEntry, len(manifests))
	for i, m := range manifests {
		hasCred, _ := providerator.CheckCredential(m, credentialsPath)
		entries[i] = providerListEntry{manifest: m, hasCred: hasCred, diskPath: diskPaths[m.Name]}
	}
	return entries, nil
}

type providersScreen struct {
	entries       []providerListEntry
	cursor        int
	offset        int
	status        string
	confirming    bool
	width, height int

	entering bool
	input    textinput.Model

	providersDir    string
	credentialsPath string
}

func newProvidersScreen(providersDir, credentialsPath string) *providersScreen {
	s := &providersScreen{providersDir: providersDir, credentialsPath: credentialsPath}
	s.reload()
	return s
}

func (s *providersScreen) reload() {
	entries, err := loadProviderEntries(s.providersDir, s.credentialsPath)
	if err != nil {
		s.status = "✗ " + err.Error()
		return
	}
	s.entries = entries
	s.cursor = max(0, min(s.cursor, len(entries)-1))
}

func (s *providersScreen) Init() tea.Cmd { return nil }

func (s *providersScreen) innerWidth() int {
	w := s.width - providersWidthMargin
	if w > formInnerWidth {
		w = formInnerWidth
	}
	return w
}

func (s *providersScreen) visibleRows() int {
	return visibleCount(s.height, providersOverheadRows, s.status != "", len(s.entries))
}

func (s *providersScreen) maxOffset() int {
	return maxScrollOffset(len(s.entries), s.visibleRows())
}

func (s *providersScreen) clampScroll() {
	if len(s.entries) == 0 {
		s.cursor, s.offset = 0, 0
		return
	}
	s.cursor = max(0, min(s.cursor, len(s.entries)-1))

	visible := s.visibleRows()
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+visible {
		s.offset = s.cursor - visible + 1
	}
	s.offset = max(0, min(s.offset, s.maxOffset()))
}

func (s *providersScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		s.clampScroll()
		return s, nil

	case tea.KeyMsg:
		if s.entering {
			switch msg.Type {
			case tea.KeyEsc:
				s.entering = false
				return s, nil
			case tea.KeyEnter:
				s.submitEntry()
				return s, nil
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}

		if s.confirming {
			switch msg.String() {
			case "y":
				s.deleteSelected()
				s.confirming = false
			case "n", "esc":
				s.status = ""
				s.confirming = false
			}
			return s, nil
		}

		switch msg.String() {
		case "esc":
			return s, popScreen()
		case "up":
			if s.cursor > 0 {
				s.cursor--
			}
			s.clampScroll()
			return s, nil
		case "down":
			if s.cursor < len(s.entries)-1 {
				s.cursor++
			}
			s.clampScroll()
			return s, nil
		case "enter":
			if s.cursor < len(s.entries) {
				return s, pushScreen(newProviderDetailScreen(s.entries[s.cursor].manifest))
			}
			return s, nil
		case "a":
			if s.cursor < len(s.entries) {
				s.startEntering()
			}
			return s, nil
		case "d":
			if s.cursor < len(s.entries) && s.entries[s.cursor].diskPath != "" {
				s.confirming = true
				s.status = fmt.Sprintf("delete %s from disk? (y/n)", s.entries[s.cursor].manifest.Name)
			}
			return s, nil
		}
	}
	return s, nil
}

func (s *providersScreen) startEntering() {
	ti := textinput.New()
	ti.Prompt = ""
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()
	s.input = ti
	s.entering = true
	s.status = ""
}

func (s *providersScreen) submitEntry() {
	secret := s.input.Value()
	entry := s.entries[s.cursor]
	if secret == "" {
		s.status = "✗ no secret entered"
		s.entering = false
		return
	}
	if err := credentials.Save(s.credentialsPath, entry.manifest.Name, entry.manifest.Auth.Credential, secret); err != nil {
		s.status = "✗ " + err.Error()
		s.entering = false
		return
	}
	s.entries[s.cursor].hasCred = true
	s.status = "✓ saved credential for " + entry.manifest.Name
	s.entering = false
}

func (s *providersScreen) deleteSelected() {
	if s.cursor >= len(s.entries) {
		return
	}
	entry := s.entries[s.cursor]
	if entry.diskPath == "" {
		return
	}
	if err := os.Remove(entry.diskPath); err != nil {
		s.status = "✗ " + err.Error()
		return
	}
	s.status = "✓ deleted " + entry.manifest.Name
	s.reload()
}

func formatProviderRow(e providerListEntry, selected bool, innerWidth int) string {
	marker := "  "
	if selected {
		marker = "› "
	}
	cred := "✗"
	if e.hasCred {
		cred = "✓"
	}
	name := padRightPlain(e.manifest.Name, providersNameWidth)
	avail := innerWidth - providersNameWidth - providersRowOverhead
	line := marker + cred + " " + name + " " + truncatePlain(e.manifest.Endpoint, avail)
	if selected {
		return selectedStyle.Render(line)
	}
	return line
}

func (s *providersScreen) View() string {
	if s.width < providersMinWidth || s.height < providersMinHeight {
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, fallbackMessage())
	}

	innerWidth := s.innerWidth()

	if s.entering {
		entry := s.entries[s.cursor]
		label := "credential for " + entry.manifest.Name + " (" + entry.manifest.Auth.Credential + ")"
		body := label + "\n\n" + s.input.View()
		box := frameBox(innerWidth, providersTitle, providersBackHint, body, true)
		return placeWithBear(box, providersEntryHint, s.width, s.height, false)
	}

	visible := s.visibleRows()
	end := s.offset + visible
	if end > len(s.entries) {
		end = len(s.entries)
	}

	var lines []string
	if len(s.entries) == 0 {
		lines = append(lines, "no providers available")
	}
	for i := s.offset; i < end; i++ {
		lines = append(lines, formatProviderRow(s.entries[i], i == s.cursor, innerWidth))
	}
	if len(s.entries) > visible {
		lines = append(lines, "", fmt.Sprintf("(%d-%d of %d)", s.offset+1, end, len(s.entries)))
	}
	if s.status != "" {
		lines = append(lines, "", s.status)
	}

	body := strings.Join(lines, "\n")
	box := frameBox(innerWidth, providersTitle, providersBackHint, body, false)
	return placeWithBear(box, providersFooterHint, s.width, s.height, false)
}
