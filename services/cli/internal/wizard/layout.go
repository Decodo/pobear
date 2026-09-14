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

func topBorderLine(innerWidth int) string {
	const prefix, suffix = "─ ", " "
	fill := max(0, innerWidth-lipgloss.Width(prefix+menuBoxTitle+suffix))
	return borderStyle.Render("┌"+prefix) +
		titleStyle.Render(menuBoxTitle) +
		suffix +
		borderStyle.Render(strings.Repeat("─", fill)+"┐")
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

	sides := lipgloss.NewStyle().
		Width(innerWidth).
		Border(lipgloss.NormalBorder(), false, true, true, true).
		BorderForeground(borderColor)
	if includePadding {
		sides = sides.PaddingTop(1).PaddingBottom(1)
	}

	return lipgloss.JoinVertical(lipgloss.Left, topBorderLine(innerWidth), sides.Render(body))
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
		`  (___/     \___)`,
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
	withFooter := lipgloss.JoinVertical(lipgloss.Left, box, footerStyle.Render(footer))

	var block string
	switch kind {
	case layoutWideRoomy:
		combined := lipgloss.JoinHorizontal(lipgloss.Center, box, "    ", bearArt(blinking))
		block = lipgloss.JoinVertical(lipgloss.Left, combined, footerStyle.Render(footer))
	case layoutStandardTall:
		block = lipgloss.JoinVertical(lipgloss.Center, bearArt(blinking), "", withFooter)
	default:
		block = withFooter
	}

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
