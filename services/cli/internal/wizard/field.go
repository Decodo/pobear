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
)

type field struct {
	name  string
	label string
	hint  string
	kind  fieldKind

	input     textinput.Model
	numberMin int

	textarea           textarea.Model
	textareaMaxVisible int

	options []string
	choice  int
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

func numberField(name, label, value string, min int) field {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(value)
	return field{name: name, label: label, kind: fieldNumber, input: ti, numberMin: min}
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

func (f field) value() string {
	switch f.kind {
	case fieldChoice:
		return f.options[f.choice]
	case fieldTextArea:
		return f.textarea.Value()
	default:
		return f.input.Value()
	}
}

func (f field) focus() field {
	switch f.kind {
	case fieldChoice:
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
	return f.kind == fieldChoice
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
			switch key.String() {
			case "enter", "right", "l":
				f.choice = (f.choice + 1) % len(f.options)
				return f, true
			case "left", "h":
				f.choice = (f.choice - 1 + len(f.options)) % len(f.options)
				return f, true
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

func (f *field) bumpNumber(delta int) {
	n, err := strconv.Atoi(f.input.Value())
	if err != nil {
		n = f.numberMin
	}
	n += delta
	if n < f.numberMin {
		n = f.numberMin
	}
	f.input.SetValue(strconv.Itoa(n))
	f.input.CursorEnd()
}

func (f field) render(cursor bool, labelWidth, valueWidth int) string {
	if f.kind == fieldTextArea {
		return f.renderTextArea(cursor, labelWidth, valueWidth)
	}

	marker := " "
	if cursor {
		marker = "›"
	}
	value := f.value()
	if f.kind != fieldChoice && cursor {
		value = f.input.View()
	}
	value = padRightPlain(value, valueWidth)
	label := padRightPlain(f.label, labelWidth)

	line := marker + " " + label + " " + value
	if f.hint != "" {
		line += " " + f.hint
	}
	return line
}

func (f field) renderTextArea(cursor bool, labelWidth, valueWidth int) string {
	marker := " "
	if cursor {
		marker = "›"
	}
	label := padRightPlain(f.label, labelWidth)
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
