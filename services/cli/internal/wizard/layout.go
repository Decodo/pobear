package wizard

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var menuItems = []string{
	"Scrape a page",
	"Crawl a site",
	"Manage providers",
	"Manage credentials",
	"Quit",
}

const (
	menuBoxTitle      = "pobear"
	menuInnerStandard = 50
	menuInnerCompact  = 34
	fullFooterHint    = "↑/↓ move   enter select   q quit"
	shortFooterHint   = "↑↓ enter q"
	unsupportedMinW   = 36
	unsupportedMinH   = 8
	compactMaxW       = 59
	wideMinW          = 100
	shortMaxH         = 11
	standardMaxH      = 27

	bearGap    = 4
	bearTopGap = 1
	footerRows = 1
)

type layoutKind int

const (
	layoutUnsupported layoutKind = iota
	layoutCompact
	layoutShort
	layoutStandard
	layoutStandardTall
	layoutWideRoomy
)

func classify(width, height int) layoutKind {
	if width < unsupportedMinW || height < unsupportedMinH {
		return layoutUnsupported
	}
	if width <= compactMaxW {
		return layoutCompact
	}
	if height <= shortMaxH {
		return layoutShort
	}
	if width >= wideMinW {
		return layoutWideRoomy
	}
	if height > standardMaxH {
		return layoutStandardTall
	}
	return layoutStandard
}

func topBorderLine(innerWidth int, title, hint string) string {
	const prefix, midSuffix = "─ ", " "
	right := ""
	if hint != "" {
		right = " " + hint + " ─"
	}
	fill := max(0, innerWidth-lipgloss.Width(prefix+title+midSuffix)-lipgloss.Width(right))
	return borderStyle.Render("┌"+prefix) +
		titleStyle.Render(title) +
		midSuffix +
		borderStyle.Render(strings.Repeat("─", fill)) +
		right +
		borderStyle.Render("┐")
}

func frameBox(innerWidth int, title, hint, body string, includePadding bool) string {
	sides := lipgloss.NewStyle().
		Width(innerWidth).
		Border(lipgloss.NormalBorder(), false, true, true, true).
		BorderForeground(borderColor)
	if includePadding {
		sides = sides.PaddingTop(1).PaddingBottom(1)
	}
	return lipgloss.JoinVertical(lipgloss.Left, topBorderLine(innerWidth, title, hint), sides.Render(body))
}

func menuBox(innerWidth, cursor int, includePadding bool) string {
	items := make([]string, len(menuItems))
	for i, item := range menuItems {
		if i == cursor {
			items[i] = selectedStyle.Render("   › " + item)
		} else {
			items[i] = "     " + item
		}
	}
	body := strings.Join(items, "\n")
	return frameBox(innerWidth, menuBoxTitle, "", body, includePadding)
}

func bearArt(blinking bool) string {
	eye := "•"
	if blinking {
		eye = "⌄"
	}
	lines := []string{
		`   __         __`,
		`  (  \_______/  )`,
		`  /   ` + eye + `     ` + eye + `   \`,
		` |      (ᴥ)      |`,
		`  \   <°)))><   /`,
		`   \____/_\____/`,
	}
	width := 0
	for _, l := range lines {
		width = max(width, lipgloss.Width(l))
	}
	for i, l := range lines {
		l = strings.ReplaceAll(l, eye, bearAccent.Render(eye))
		l = strings.ReplaceAll(l, "ᴥ", bearAccent.Render("ᴥ"))
		lines[i] = l
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func blankRows(n int) string {
	return strings.Repeat("\n", n-1)
}

type bearPlacement int

const (
	bearNone bearPlacement = iota
	bearTop
	bearSide
)

func composeWithBear(box, footer, bear string, placement bearPlacement) string {
	switch placement {
	case bearSide:
		combined := lipgloss.JoinHorizontal(lipgloss.Center, box, strings.Repeat(" ", bearGap), bear)
		return lipgloss.JoinVertical(lipgloss.Left, combined, footer)
	case bearTop:
		withFooter := lipgloss.JoinVertical(lipgloss.Left, box, footer)
		return lipgloss.JoinVertical(lipgloss.Center, bear, blankRows(bearTopGap), withFooter)
	default:
		return lipgloss.JoinVertical(lipgloss.Left, box, footer)
	}
}

func fallbackMessage() string {
	return strings.Join([]string{
		"pobear needs a bigger",
		"terminal (min " + strconv.Itoa(unsupportedMinW) + "x" + strconv.Itoa(unsupportedMinH) + ").",
		"",
		"Resize, or run",
		"pobear scrape/crawl",
		"directly instead. (q)",
	}, "\n")
}

func renderMenu(width, height, cursor int, blinking bool) string {
	kind := classify(width, height)

	if kind == layoutUnsupported {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, fallbackMessage())
	}

	innerWidth := menuInnerStandard
	includePadding := true
	footer := fullFooterHint
	if kind == layoutCompact {
		innerWidth = compactInnerWidth(width)
		includePadding = height >= unsupportedMinH+2
		footer = shortFooterHint
	}
	if kind == layoutShort {
		includePadding = false
	}

	box := menuBox(innerWidth, cursor, includePadding)

	placement := bearNone
	switch kind {
	case layoutWideRoomy:
		placement = bearSide
	case layoutStandardTall:
		placement = bearTop
	}
	block := composeWithBear(box, footerStyle.Render(footer), bearArt(blinking), placement)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}

func compactInnerWidth(termWidth int) int {
	w := termWidth - 4
	const floor = 28
	const ceiling = menuInnerCompact
	if w > ceiling {
		w = ceiling
	}
	if w < floor {
		w = floor
	}
	return w
}

const (
	formInnerWidth = 84
	formLabelWidth = 14
	formValueWidth = 14
	formBackHint   = "esc: back to menu"
	formFooterHint = "↑/↓ move   enter edit   esc back"

	urlAreaHeight = 4
	urlAreaWidth  = formInnerWidth - formLabelWidth - 4
)

func decideBearPlacement(boxWidth, boxHeight, bearWidth, bearHeight, termWidth, termHeight int) bearPlacement {
	if termHeight >= boxHeight+footerRows && termWidth >= boxWidth+bearGap+bearWidth {
		return bearSide
	}
	if termWidth >= max(boxWidth, bearWidth) && termHeight >= bearHeight+bearTopGap+boxHeight+footerRows {
		return bearTop
	}
	return bearNone
}

func decideTopOnlyBearPlacement(boxWidth, boxHeight, bearWidth, bearHeight, termWidth, termHeight int) bearPlacement {
	if termWidth >= max(boxWidth, bearWidth) && termHeight >= bearHeight+bearTopGap+boxHeight+footerRows {
		return bearTop
	}
	return bearNone
}

func placeWithBear(box, footerHint string, width, height int, topOnly bool) string {
	boxWidth, boxHeight := lipgloss.Width(box), lipgloss.Height(box)

	bear := bearArt(false)
	bearWidth, bearHeight := lipgloss.Width(bear), lipgloss.Height(bear)

	decide := decideBearPlacement
	if topOnly {
		decide = decideTopOnlyBearPlacement
	}
	placement := decide(boxWidth, boxHeight, bearWidth, bearHeight, width, height)

	footer := footerStyle.Render(truncatePlain(footerHint, width))
	block := composeWithBear(box, footer, bear, placement)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}

func renderForm(width, height int, title string, fields []field, cursor int, status string) string {
	lines := make([]string, len(fields))
	for i, f := range fields {
		lines[i] = f.render(i == cursor, formValueWidth)
	}
	if status != "" {
		lines = append(lines, "", status)
	}
	body := strings.Join(lines, "\n")
	box := frameBox(formInnerWidth, title, formBackHint, body, true)
	return placeWithBear(box, formFooterHint, width, height, false)
}
