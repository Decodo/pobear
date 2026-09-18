package wizard

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type fieldKind int

const (
	fieldText fieldKind = iota
	fieldChoice
	fieldNumber
	fieldTextArea
	fieldSearch
)

var bareEasterEgg bool

type field struct {
	name  string
	label string
	hint  string
	kind  fieldKind

	input     textinput.Model
	numberMin int
	numberMax *int

	textarea           textarea.Model
	textareaMaxVisible int

	options         []string
	choice          int
	filter          string
	searchDefault   string
	resolved        string
	displayOverride string
}

func textField(name, label, hint, value string) field {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(value)
	return field{name: name, label: label, hint: hint, kind: fieldText, input: ti}
}

func textAreaField(label, hint, value string, maxVisible, width int) field { //nolint:unparam // general constructor; only the URL field uses it today
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.Placeholder = hint
	ta.SetWidth(width)
	ta.SetValue(value)
	f := field{name: "url", label: label, kind: fieldTextArea, textarea: ta, textareaMaxVisible: maxVisible}
	f.resizeTextArea()
	return f
}

func (f *field) resizeTextArea() {
	h := f.textarea.LineCount()
	if h < 1 {
		h = 1
	}
	if h > f.textareaMaxVisible {
		h = f.textareaMaxVisible
	}
	f.textarea.SetHeight(h)
}

func numberField(name, label, hint, value string, min int, max *int) field {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(value)
	return field{name: name, label: label, hint: hint, kind: fieldNumber, input: ti, numberMin: min, numberMax: max}
}

func choiceField(name, label, hint string, options []string, current string) field {
	idx := 0
	for i, o := range options {
		if o == current {
			idx = i
			break
		}
	}
	return field{name: name, label: label, hint: hint, kind: fieldChoice, options: options, choice: idx}
}

func templateSearchField(options []string, def, query string) field {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(query)
	resolved := def
	if match, ok := matchSearch(options, query); ok {
		resolved = match
	}
	return field{name: "template", label: "Template", kind: fieldSearch, input: ti, options: options, searchDefault: def, resolved: resolved}
}

func matchSearch(options []string, query string) (string, bool) {
	if query == "" {
		return "", false
	}
	if idx, ok := firstMatchIndex(options, query); ok {
		return options[idx], true
	}
	return "", false
}

func (f field) value() string {
	switch f.kind {
	case fieldChoice:
		return f.options[f.choice]
	case fieldSearch:
		return f.resolved
	case fieldTextArea:
		return f.textarea.Value()
	default:
		return f.input.Value()
	}
}

func (f field) rawText() string {
	return f.input.Value()
}

func (f field) focus() field {
	switch f.kind {
	case fieldChoice:
		f.filter = ""
	case fieldTextArea:
		f.textarea.Focus()
	default:
		f.input.Focus()
	}
	return f
}

func (f field) blur() field {
	switch f.kind {
	case fieldChoice:
	case fieldTextArea:
		f.textarea.Blur()
	default:
		f.input.Blur()
	}
	return f
}

func (f field) isStructural() bool {
	return f.kind == fieldChoice || f.kind == fieldSearch
}

func (f field) capturesVerticalNav(dir string) bool {
	if f.kind != fieldTextArea {
		return false
	}
	if dir == "up" {
		return f.textarea.Line() > 0
	}
	return f.textarea.Line() < f.textarea.LineCount()-1
}

func (f field) update(msg tea.Msg) (field, bool) {
	key, isKey := msg.(tea.KeyMsg)

	switch f.kind {
	case fieldChoice:
		if isKey {
			switch key.Type {
			case tea.KeyEnter, tea.KeyRight:
				f.choice = (f.choice + 1) % len(f.options)
				f.filter = ""
				return f, true
			case tea.KeyLeft:
				f.choice = (f.choice - 1 + len(f.options)) % len(f.options)
				f.filter = ""
				return f, true
			case tea.KeyRunes:
				return f.applyFilter(key.Runes)
			}
		}
		return f, false

	case fieldNumber:
		if isKey {
			switch key.String() {
			case "right":
				f.bumpNumber(1)
				return f, true
			case "left":
				f.bumpNumber(-1)
				return f, true
			}
		}
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		_ = cmd
		return f, true

	case fieldSearch:
		before := f.resolved
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		_ = cmd
		query := f.input.Value()
		if query == "" {
			f.resolved = f.searchDefault
		} else if match, ok := matchSearch(f.options, query); ok {
			f.resolved = match
		}
		return f, f.resolved != before

	case fieldTextArea:
		f.textarea.SetHeight(f.textareaMaxVisible)
		var cmd tea.Cmd
		f.textarea, cmd = f.textarea.Update(msg)
		_ = cmd
		f.resizeTextArea()
		return f, true

	default:
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		_ = cmd
		return f, true
	}
}

func (f field) applyFilter(runes []rune) (field, bool) {
	before := f.choice
	for _, r := range runes {
		f.filter += string(r)
		if idx, ok := firstMatchIndex(f.options, f.filter); ok {
			f.choice = idx
			continue
		}
		f.filter = string(r)
		if idx, ok := firstMatchIndex(f.options, f.filter); ok {
			f.choice = idx
		}
	}
	return f, f.choice != before
}

func firstMatchIndex(options []string, filter string) (int, bool) {
	lower := strings.ToLower(filter)
	for i, o := range options {
		if strings.HasPrefix(strings.ToLower(o), lower) {
			return i, true
		}
	}
	return 0, false
}

func (f *field) bumpNumber(delta int) {
	n, err := strconv.Atoi(f.input.Value())
	if err != nil {
		n = f.numberMin
	}
	n += delta
	if n < f.numberMin {
		n = f.numberMin
	}
	if f.numberMax != nil && n > *f.numberMax {
		n = *f.numberMax
	}
	f.input.SetValue(strconv.Itoa(n))
	f.input.CursorEnd()
}

func (f field) render(cursor bool, valueWidth int) string {
	switch f.kind {
	case fieldTextArea:
		return f.renderTextArea(cursor, valueWidth)
	case fieldSearch:
		return f.renderSearch(cursor, valueWidth)
	}

	marker := " "
	if cursor {
		marker = "›"
	}
	value := f.value()
	if f.displayOverride != "" {
		value = f.displayOverride
	}
	if f.kind != fieldChoice && cursor {
		value = f.input.View()
	}
	value = padRightPlain(value, valueWidth)
	label := padRightPlain(f.label, formLabelWidth)

	line := marker + " " + label + " " + value
	if f.hint != "" {
		line += " " + f.hint
	}
	return line
}

func (f field) renderSearch(cursor bool, valueWidth int) string {
	marker := " "
	if cursor {
		marker = "›"
	}
	value := f.input.Value()
	if cursor {
		value = f.input.View()
	}
	value = padRightPlain(value, valueWidth)
	label := padRightPlain(f.label, formLabelWidth)

	return marker + " " + label + " " + value + " (" + f.resolved + ")"
}

func (f field) renderTextArea(cursor bool, valueWidth int) string {
	marker := " "
	if cursor {
		marker = "›"
	}
	label := padRightPlain(f.label, formLabelWidth)
	indent := strings.Repeat(" ", lipgloss.Width(marker+" "+label+" "))

	valueLines := strings.Split(f.textarea.View(), "\n")
	rows := make([]string, len(valueLines))
	for i, vl := range valueLines {
		vl = padRightPlain(vl, valueWidth)
		if i == 0 {
			rows[i] = marker + " " + label + " " + vl
		} else {
			rows[i] = indent + vl
		}
	}
	return strings.Join(rows, "\n")
}
