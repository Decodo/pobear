package wizard

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type cssFieldPair struct {
	name, selector string
}

type formState struct {
	url             string
	fetchVia        string
	profile         string
	proxyURL        string
	providerName    string
	templateName    string
	templateQuery   string
	providerOptions map[string]string
	extract         string
	cssFields       []cssFieldPair
	concurrency     string
	retries         string
}

func newFormState() formState {
	return formState{
		fetchVia:    "local",
		profile:     "chrome",
		extract:     "bare",
		concurrency: "8",
		retries:     "3",
	}
}

const (
	addFieldName   = "addField"
	saveActionName = "saveAction"
	runActionName  = "runAction"
)

func cssFieldName(i int, part string) string {
	return "css." + part + "." + strconv.Itoa(i)
}

func computeFields(s formState, manifests []*provider.Manifest) []field {
	fields := []field{
		textAreaField("URL", "(one per line)", s.url, urlAreaHeight, urlAreaWidth),
		choiceField("fetchVia", "Fetch via", "(local / proxy / provider)", []string{"local", "proxy", "provider"}, s.fetchVia),
	}

	switch s.fetchVia {
	case "local":
		fields = append(fields, choiceField("profile", "Profile", "(TLS fingerprint)", []string{"chrome", "firefox", "safari"}, s.profile))
	case "proxy":
		fields = append(fields,
			textField("proxyURL", "Proxy URL", "", s.proxyURL),
			choiceField("profile", "Profile", "(TLS fingerprint)", []string{"chrome", "firefox", "safari"}, s.profile),
		)
	case "provider":
		fields = append(fields, providerAndOptionFields(s, manifests)...)
	}

	extractField := choiceField("extract", "Extract", "(bare / raw / markdown / css fields)", []string{"bare", "raw", "markdown", "css fields"}, s.extract)
	if bareEasterEgg && s.extract == "bare" {
		extractField.displayOverride = "bear"
	}
	fields = append(fields, extractField)

	if s.extract == "css fields" {
		for i, pair := range s.cssFields {
			fields = append(fields,
				textField(cssFieldName(i, "name"), "  Field name", "(ctrl+d remove)", pair.name),
				textField(cssFieldName(i, "selector"), "    Selector", "", pair.selector),
			)
		}
		fields = append(fields, field{name: addFieldName, label: "  + add field", kind: fieldChoice, options: []string{""}})
	}

	fields = append(fields,
		numberField("concurrency", "Concurrency", "", s.concurrency, 1, nil),
		numberField("retries", "Retries", "", s.retries, 0, nil),
		field{name: saveActionName, label: "[ Save as " + savedJobFilename + " ]", kind: fieldChoice, options: []string{""}},
		field{name: runActionName, label: "[ Run ]", kind: fieldChoice, options: []string{""}},
	)

	return fields
}

func manifestNames(manifests []*provider.Manifest) []string {
	names := make([]string, len(manifests))
	for i, m := range manifests {
		names[i] = m.Name
	}
	return names
}

func effectiveChoice(current string, options []string) string {
	if current != "" || len(options) == 0 {
		return current
	}
	return options[0]
}

func providerAndOptionFields(s formState, manifests []*provider.Manifest) []field {
	names := manifestNames(manifests)
	displayNames := names
	if len(displayNames) == 0 {
		displayNames = []string{"(none configured)"}
	}
	fields := []field{choiceField("provider", "Provider", "(type to search)", displayNames, s.providerName)}

	m, ok := providerator.Lookup(manifests, effectiveChoice(s.providerName, names))
	if !ok {
		return fields
	}
	return append(fields, providerFields(s, m)...)
}

func defaultTemplateName(m *provider.Manifest) string {
	for _, t := range m.Templates {
		if t.Default {
			return t.Name
		}
	}
	if len(m.Templates) == 0 {
		return ""
	}
	return m.Templates[0].Name
}

func effectiveTemplateName(m *provider.Manifest, current string) string {
	if len(m.Templates) < 2 || current != "" {
		return current
	}
	return defaultTemplateName(m)
}

func providerFields(s formState, m *provider.Manifest) []field {
	switch len(m.Templates) {
	case 0:
		return optionFields(m.Options, m.Name, s)
	case 1:
		opts, _ := providerator.ResolveTemplate(m, "")
		return optionFields(opts, m.Name, s)
	}

	templateNames := make([]string, len(m.Templates))
	for i, t := range m.Templates {
		templateNames[i] = t.Name
	}
	fields := []field{templateSearchField(templateNames, defaultTemplateName(m), s.templateQuery)}
	selected := effectiveTemplateName(m, s.templateName)
	if opts, err := providerator.ResolveTemplate(m, selected); err == nil {
		fields = append(fields, optionFields(opts, selected, s)...)
	}
	return fields
}

func optionFields(opts []provider.Option, hintSource string, s formState) []field {
	hint := "(" + hintSource + " option)"
	fields := make([]field, len(opts))
	for i, opt := range opts {
		fields[i] = optionField(opt, hint, optionValue(s, opt))
	}
	return fields
}

func optionFieldName(name string) string {
	return "opt." + name
}

func optionLabel(name string) string {
	label := strings.ReplaceAll(name, "_", " ")
	if label == "" {
		return label
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func optionValue(s formState, opt provider.Option) string {
	if v, ok := s.providerOptions[opt.Name]; ok {
		return v
	}
	if opt.Default != nil {
		return fmt.Sprint(opt.Default)
	}
	return ""
}

func optionField(opt provider.Option, hint, current string) field {
	name, label := optionFieldName(opt.Name), optionLabel(opt.Name)
	switch opt.Type {
	case "enum":
		return choiceField(name, label, hint, opt.Values, current)
	case "bool":
		return choiceField(name, label, hint, []string{"false", "true"}, current)
	case "int":
		min := 0
		if opt.Min != nil {
			min = *opt.Min
		}
		return numberField(name, label, hint, current, min, opt.Max)
	default:
		return textField(name, label, hint, current)
	}
}

type scrapeForm struct {
	state             formState
	fields            []field
	cursor            int
	width, height     int
	providerManifests []*provider.Manifest
	status            string

	ctx             context.Context
	providersDir    string
	credentialsPath string
}

func newScrapeForm(providerManifests []*provider.Manifest, ctx context.Context, providersDir, credentialsPath string) *scrapeForm {
	f := &scrapeForm{
		state:             newFormState(),
		providerManifests: providerManifests,
		ctx:               ctx,
		providersDir:      providersDir,
		credentialsPath:   credentialsPath,
	}
	f.rebuild()
	return f
}

func (f *scrapeForm) rebuild() {
	focusedName := ""
	if len(f.fields) > 0 && f.cursor < len(f.fields) {
		focusedName = f.fields[f.cursor].name
	}

	f.fields = computeFields(f.state, f.providerManifests)

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

func (f *scrapeForm) refocus() {
	for i := range f.fields {
		if i == f.cursor {
			f.fields[i] = f.fields[i].focus()
		} else {
			f.fields[i] = f.fields[i].blur()
		}
	}
}

func (f *scrapeForm) providerOrTemplateChanged() bool {
	for _, fld := range f.fields {
		if fld.name != "provider" {
			continue
		}
		if v := fld.value(); v != f.state.providerName {
			f.state.providerName = v
			f.state.templateName = ""
			f.state.templateQuery = ""
			f.state.providerOptions = nil
			return true
		}
		break
	}

	for _, fld := range f.fields {
		if fld.name != "template" {
			continue
		}
		f.state.templateQuery = fld.rawText()
		if v := fld.value(); v != f.state.templateName {
			f.state.templateName = v
			f.state.providerOptions = nil
			return true
		}
		break
	}
	return false
}

func (f *scrapeForm) syncStateFromFields() {
	skipOptions := f.providerOrTemplateChanged()

	for _, fld := range f.fields {
		switch {
		case fld.name == "url":
			f.state.url = fld.value()
		case fld.name == "fetchVia":
			f.state.fetchVia = fld.value()
		case fld.name == "profile":
			f.state.profile = fld.value()
		case fld.name == "proxyURL":
			f.state.proxyURL = fld.value()
		case fld.name == "provider" || fld.name == "template":
		case strings.HasPrefix(fld.name, "opt."):
			if skipOptions {
				continue
			}
			if f.state.providerOptions == nil {
				f.state.providerOptions = map[string]string{}
			}
			f.state.providerOptions[strings.TrimPrefix(fld.name, "opt.")] = fld.value()
		case fld.name == "extract":
			f.state.extract = fld.value()
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

func cssFieldIndex(name, prefix string) (int, bool) {
	i, err := strconv.Atoi(strings.TrimPrefix(name, prefix))
	return i, err == nil
}

func cssPairIndexFromFieldName(name string) (int, bool) {
	if i, ok := cssFieldIndex(name, "css.name."); ok {
		return i, true
	}
	return cssFieldIndex(name, "css.selector.")
}

func (f *scrapeForm) Init() tea.Cmd {
	return nil
}

func (f *scrapeForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			case addFieldName:
				f.syncStateFromFields()
				f.state.cssFields = append(f.state.cssFields, cssFieldPair{})
				f.rebuild()
				return f, nil
			case saveActionName:
				f.syncStateFromFields()
				f.status = doSave(f.state, f.providersDir)
				return f, nil
			case runActionName:
				f.syncStateFromFields()
				total := len(urlList(f.state))
				if total == 0 {
					f.status = "✗ no URLs to scrape"
					return f, nil
				}
				if msg := checkProviderCredential(f.state, f.providersDir, f.credentialsPath); msg != "" {
					f.status = msg
					return f, nil
				}
				progress := newRunProgress(total)
				runCmd := startRun(f.ctx, f.state, f.providersDir, f.credentialsPath, progress)
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

func (f *scrapeForm) View() string {
	return renderForm(f.width, f.height, f.fields, f.cursor, f.status)
}
