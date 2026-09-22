package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/provider"
)

const (
	tierScreenBackHint = "esc: back"
)

func tierScreenTitle(tier string) string {
	switch tier {
	case "local":
		return "Local"
	case "proxy":
		return "Proxy"
	default:
		return "Provider"
	}
}

func tierScreenFields(tier string, fc fetchConfig, manifests []*provider.Manifest) []field {
	switch tier {
	case "local":
		return localTierFields(fc)
	case "proxy":
		return proxyTierFields(fc)
	default:
		return providerTierFields(fc, manifests)
	}
}

type tierScreen struct {
	tier      string
	fc        *fetchConfig
	manifests []*provider.Manifest
	onDone    func()

	fields        []field
	cursor        int
	width, height int
}

func newTierScreen(tier string, fc *fetchConfig, manifests []*provider.Manifest, onDone func()) *tierScreen {
	s := &tierScreen{tier: tier, fc: fc, manifests: manifests, onDone: onDone}
	s.rebuild()
	return s
}

func (s *tierScreen) rebuild() {
	focusedName := ""
	if len(s.fields) > 0 && s.cursor < len(s.fields) {
		focusedName = s.fields[s.cursor].name
	}

	s.fields = tierScreenFields(s.tier, *s.fc, s.manifests)

	newCursor := 0
	for i, fld := range s.fields {
		if fld.name == focusedName {
			newCursor = i
			break
		}
	}
	s.cursor = newCursor
	s.refocus()
}

func (s *tierScreen) refocus() {
	for i := range s.fields {
		if i == s.cursor {
			s.fields[i] = s.fields[i].focus()
		} else {
			s.fields[i] = s.fields[i].blur()
		}
	}
}

func (s *tierScreen) syncToConfig() {
	if s.tier == "local" {
		s.fc.localConfigured = true
	}
	skipOptions := s.tier == "provider" && syncProviderOrTemplateChanged(s.fields, s.fc)

	for _, fld := range s.fields {
		switch {
		case fld.name == "profile":
			s.fc.profile = fld.value()
		case fld.name == "proxyURL":
			s.fc.proxyURL = fld.value()
		case fld.name == "provider" || fld.name == "template":
		case strings.HasPrefix(fld.name, "opt."):
			if skipOptions {
				continue
			}
			if s.fc.providerOptions == nil {
				s.fc.providerOptions = map[string]string{}
			}
			s.fc.providerOptions[strings.TrimPrefix(fld.name, "opt.")] = fld.value()
		}
	}
}

func (s *tierScreen) Init() tea.Cmd { return nil }

func (s *tierScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.syncToConfig()
			s.onDone()
			return s, popScreen()
		case "up", "down":
			if s.cursor < len(s.fields) && s.fields[s.cursor].capturesVerticalNav(msg.String()) {
				current := s.fields[s.cursor]
				updated, changed := current.update(msg)
				s.fields[s.cursor] = updated
				if changed && current.isStructural() {
					s.syncToConfig()
					s.rebuild()
				}
				return s, nil
			}
			s.syncToConfig()
			if msg.String() == "up" {
				s.cursor = (s.cursor - 1 + len(s.fields)) % len(s.fields)
			} else {
				s.cursor = (s.cursor + 1) % len(s.fields)
			}
			s.refocus()
			return s, nil
		case "tab":
			s.syncToConfig()
			s.cursor = (s.cursor + 1) % len(s.fields)
			s.refocus()
			return s, nil
		}

		if s.cursor < len(s.fields) {
			current := s.fields[s.cursor]
			updated, changed := current.update(msg)
			s.fields[s.cursor] = updated
			if changed && current.isStructural() {
				s.syncToConfig()
				s.rebuild()
			}
		}
		return s, nil
	}
	return s, nil
}

func (s *tierScreen) View() string {
	return renderForm(s.width, s.height, tierScreenTitle(s.tier), s.fields, s.cursor, "", tierScreenBackHint)
}
