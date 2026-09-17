package wizard

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type cssFieldPair struct {
	name, selector string
}

type formState struct {
	url          string
	fetchVia     string
	profile      string
	proxyURL     string
	providerName string
	extract      string
	cssFields    []cssFieldPair
	concurrency  string
	retries      string
}

func newFormState() formState {
	return formState{
		fetchVia:    "local",
		profile:     "chrome",
		extract:     "markdown",
		concurrency: "8",
		retries:     "3",
	}
}

const addFieldName = "addField"

func cssFieldName(i int, part string) string {
	return "css." + part + "." + strconv.Itoa(i)
}

func computeFields(s formState, providerNames []string) []field {
	fields := []field{
		textField("url", "URL", "", s.url),
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
		names := providerNames
		if len(names) == 0 {
			names = []string{"(none configured)"}
		}
		fields = append(fields, choiceField("provider", "Provider", "", names, s.providerName))
	}

	fields = append(fields, choiceField("extract", "Extract", "(raw / markdown / css fields)", []string{"raw", "markdown", "css fields"}, s.extract))

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
		numberField("concurrency", "Concurrency", s.concurrency, 1),
		numberField("retries", "Retries", s.retries, 0),
	)

	return fields
}

type scrapeForm struct {
	state         formState
	fields        []field
	cursor        int
	width, height int
	providerNames []string
}

func newScrapeForm(providerNames []string) *scrapeForm {
	f := &scrapeForm{state: newFormState(), providerNames: providerNames}
	f.rebuild()
	return f
}

func (f *scrapeForm) rebuild() {
	focusedName := ""
	if len(f.fields) > 0 && f.cursor < len(f.fields) {
		focusedName = f.fields[f.cursor].name
	}

	f.fields = computeFields(f.state, f.providerNames)

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

func (f *scrapeForm) syncStateFromFields() {
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
		case fld.name == "provider":
			f.state.providerName = fld.value()
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
		case "up":
			f.syncStateFromFields()
			f.cursor = (f.cursor - 1 + len(f.fields)) % len(f.fields)
			f.refocus()
			return f, nil
		case "down", "tab":
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

		if f.cursor < len(f.fields) && f.fields[f.cursor].name == addFieldName && msg.String() == "enter" {
			f.syncStateFromFields()
			f.state.cssFields = append(f.state.cssFields, cssFieldPair{})
			f.rebuild()
			return f, nil
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
	return renderForm(f.width, f.height, f.fields, f.cursor)
}
