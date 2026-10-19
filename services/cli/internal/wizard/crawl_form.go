package wizard

import (
	"context"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/provider"
)

type crawlFormState struct {
	startingURL string
	depth       string
	maxPages    string
	scope       string
	sitemap     string
	robots      string
	fetchConfig
	extractConfig
	rps         string
	concurrency string
	retries     string
}

func newCrawlFormState() crawlFormState {
	return crawlFormState{
		depth:         "1",
		maxPages:      "100",
		scope:         "same-host",
		sitemap:       "true",
		robots:        "respect",
		fetchConfig:   newFetchConfig(),
		extractConfig: newExtractConfig(),
		rps:           "16",
		concurrency:   "8",
		retries:       "3",
	}
}

func computeCrawlFields(s crawlFormState) []field {
	fields := []field{textField("startingURL", "Starting URL", "", s.startingURL)}
	fields = append(fields, fetchTierRows(s.fetchConfig)...)
	fields = append(fields,
		numberField("depth", "Depth", "(0 = seeds only)", s.depth, 0, nil),
		numberField("maxPages", "Max pages", "(stop after this many pages)", s.maxPages, 1, nil),
		choiceField("scope", "Scope", "(same-host / same-domain / any)", []string{"same-host", "same-domain", "any"}, s.scope),
		choiceField("sitemap", "Sitemap", "(also seed from /sitemap.xml)", []string{"false", "true"}, s.sitemap),
		choiceField("robots", "Robots", "(respect / ignore robots.txt)", []string{"respect", "ignore"}, s.robots),
	)
	fields = append(fields, extractFields(s.extractConfig)...)
	fields = append(fields,
		numberField("rps", "Per-host rps", "(0 = unlimited)", s.rps, 0, nil),
		numberField("concurrency", "Concurrency", "", s.concurrency, 1, nil),
		numberField("retries", "Retries", "", s.retries, 0, nil),
		field{name: saveActionName, label: "[ Save as " + savedJobFilename + " ]", kind: fieldChoice, options: []string{""}},
		field{name: runActionName, label: "[ Run ]", kind: fieldChoice, options: []string{""}},
	)
	return fields
}

type crawlForm struct {
	state             crawlFormState
	fields            []field
	cursor            int
	width, height     int
	providerManifests []*provider.Manifest
	status            string

	ctx             context.Context
	providersDir    string
	credentialsPath string
}

func newCrawlForm(providerManifests []*provider.Manifest, ctx context.Context, providersDir, credentialsPath string) *crawlForm {
	f := &crawlForm{
		state:             newCrawlFormState(),
		providerManifests: providerManifests,
		ctx:               ctx,
		providersDir:      providersDir,
		credentialsPath:   credentialsPath,
	}
	f.rebuild()
	return f
}

func (f *crawlForm) setStatus(s string) {
	f.status = s
}

func (f *crawlForm) rebuild() {
	focusedName := ""
	if len(f.fields) > 0 && f.cursor < len(f.fields) {
		focusedName = f.fields[f.cursor].name
	}

	f.fields = computeCrawlFields(f.state)

	newCursor := 0
	for i, fld := range f.fields {
		if fld.name == focusedName {
			newCursor = i
			break
		}
	}
	f.cursor = newCursor
	f.refocus()
}

func (f *crawlForm) refocus() {
	for i := range f.fields {
		if i == f.cursor {
			f.fields[i] = f.fields[i].focus()
		} else {
			f.fields[i] = f.fields[i].blur()
		}
	}
}

func (f *crawlForm) syncStateFromFields() {
	skipOptions := syncProviderOrTemplateChanged(f.fields, &f.state.fetchConfig)

	for _, fld := range f.fields {
		switch {
		case fld.name == "startingURL":
			f.state.startingURL = fld.value()
		case fld.name == "profile":
			f.state.profile = fld.value()
		case fld.name == "proxyURL":
			f.state.proxyURL = fld.value()
		case strings.HasPrefix(fld.name, "opt."):
			if skipOptions {
				continue
			}
			if f.state.providerOptions == nil {
				f.state.providerOptions = map[string]string{}
			}
			f.state.providerOptions[strings.TrimPrefix(fld.name, "opt.")] = fld.value()
		case fld.name == "depth":
			f.state.depth = fld.value()
		case fld.name == "maxPages":
			f.state.maxPages = fld.value()
		case fld.name == "scope":
			f.state.scope = fld.value()
		case fld.name == "sitemap":
			f.state.sitemap = fld.value()
		case fld.name == "robots":
			f.state.robots = fld.value()
		case fld.name == "extract":
			f.state.extract = fld.value()
		case fld.name == "rps":
			f.state.rps = fld.value()
		case fld.name == "concurrency":
			f.state.concurrency = fld.value()
		case fld.name == "retries":
			f.state.retries = fld.value()
		case strings.HasPrefix(fld.name, "css.name."):
			if i, ok := cssFieldIndex(fld.name, "css.name."); ok && i < len(f.state.cssFields) {
				f.state.cssFields[i].name = fld.value()
			}
		case strings.HasPrefix(fld.name, "css.selector."):
			if i, ok := cssFieldIndex(fld.name, "css.selector."); ok && i < len(f.state.cssFields) {
				f.state.cssFields[i].selector = fld.value()
			}
		}
	}
}

func (f *crawlForm) Init() tea.Cmd {
	return nil
}

func (f *crawlForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height
		return f, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return f, popScreen()
		case "up", "down":
			if f.cursor < len(f.fields) && f.fields[f.cursor].capturesVerticalNav(msg.String()) {
				current := f.fields[f.cursor]
				updated, changed := current.update(msg)
				f.fields[f.cursor] = updated
				if changed && current.isStructural() {
					f.syncStateFromFields()
					f.rebuild()
				}
				return f, nil
			}
			f.syncStateFromFields()
			if msg.String() == "up" {
				f.cursor = (f.cursor - 1 + len(f.fields)) % len(f.fields)
			} else {
				f.cursor = (f.cursor + 1) % len(f.fields)
			}
			f.refocus()
			return f, nil
		case "tab":
			f.syncStateFromFields()
			f.cursor = (f.cursor + 1) % len(f.fields)
			f.refocus()
			return f, nil
		case "ctrl+d":
			if f.cursor < len(f.fields) {
				if i, ok := cssPairIndexFromFieldName(f.fields[f.cursor].name); ok {
					f.syncStateFromFields()
					f.state.cssFields = append(f.state.cssFields[:i], f.state.cssFields[i+1:]...)
					f.rebuild()
				}
			}
			return f, nil
		}

		if f.cursor < len(f.fields) && msg.String() == "enter" {
			switch f.fields[f.cursor].name {
			case "localTier":
				f.syncStateFromFields()
				return f, pushScreen(newTierScreen("local", &f.state.fetchConfig, f.providerManifests, f.rebuild))
			case "proxyTier":
				f.syncStateFromFields()
				return f, pushScreen(newTierScreen("proxy", &f.state.fetchConfig, f.providerManifests, f.rebuild))
			case "providerTier":
				f.syncStateFromFields()
				return f, pushScreen(newTierScreen("provider", &f.state.fetchConfig, f.providerManifests, f.rebuild))
			case addFieldName:
				f.syncStateFromFields()
				f.state.cssFields = append(f.state.cssFields, cssFieldPair{})
				f.rebuild()
				return f, nil
			case saveActionName:
				f.syncStateFromFields()
				f.status = doCrawlSave(f.state, f.providersDir)
				return f, nil
			case runActionName:
				f.syncStateFromFields()
				if strings.TrimSpace(f.state.startingURL) == "" {
					f.status = "✗ no starting URL to crawl"
					return f, nil
				}
				if use, _ := resolveFetchUseAndLadder(f.state.fetchConfig); use == "" {
					f.status = "✗ configure at least one of Local/Proxy/Provider"
					return f, nil
				}
				if msg := checkProviderCredential(f.state.fetchConfig, f.providersDir, f.credentialsPath); msg != "" {
					f.status = msg
					return f, nil
				}
				maxPages, _ := strconv.Atoi(f.state.maxPages)
				if maxPages < 1 {
					maxPages = 1
				}
				progress := newRunProgress(maxPages)
				runCmd := startCrawlRun(f.ctx, f.state, f.providersDir, f.credentialsPath, progress)
				return f, tea.Batch(pushScreen(newRunningScreen(f, progress)), runCmd)
			}
		}

		if f.cursor < len(f.fields) {
			current := f.fields[f.cursor]
			updated, changed := current.update(msg)
			f.fields[f.cursor] = updated
			if changed && current.isStructural() {
				f.syncStateFromFields()
				f.rebuild()
			}
		}
		return f, nil
	}
	return f, nil
}

func (f *crawlForm) View() string {
	return renderForm(f.width, f.height, "Crawl", f.fields, f.cursor, f.status, formBackHint)
}
