package wizard

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type fieldKind int

const (
	fieldText fieldKind = iota
	fieldChoice
	fieldNumber
)

type field struct {
	name  string
	label string
	hint  string
	kind  fieldKind

	input     textinput.Model
	numberMin int

	options []string
	choice  int
}

func textField(name, label, hint, value string) field {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(value)
	return field{name: name, label: label, hint: hint, kind: fieldText, input: ti}
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
	if f.kind == fieldChoice {
		return f.options[f.choice]
	}
	return f.input.Value()
}

func (f field) focus() field {
	if f.kind != fieldChoice {
		f.input.Focus()
	}
	return f
}

func (f field) blur() field {
	if f.kind != fieldChoice {
		f.input.Blur()
	}
	return f
}

func (f field) isStructural() bool {
	return f.kind == fieldChoice
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

func padRightPlain(s string, width int) string {
	n := lipgloss.Width(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}
