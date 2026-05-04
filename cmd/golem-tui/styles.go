package main

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

func renderBorderedPane(content, title string, width, height int, focused bool) string {
	style := unfocusedBorder
	if focused {
		style = focusedBorder
	}
	// lipgloss v2 treats Width/Height on a bordered style as the total frame
	// size (borders included). Pass width/height directly so the body matches
	// the custom title-border line below, which is also width chars wide.
	innerW := max(0, width-2)
	content = lipgloss.NewStyle().MaxWidth(innerW).Render(content)
	rendered := style.Width(width).Height(height).MaxHeight(height).Render(content)

	if title != "" {
		lines := strings.Split(rendered, "\n")
		if len(lines) > 0 {
			lines[0] = buildTitleBorderLine(title, width, focused)
			rendered = strings.Join(lines, "\n")
		}
	}

	return rendered
}

func buildTitleBorderLine(title string, width int, focused bool) string {
	color := unfocusedBorderColor
	if focused {
		color = focusedBorderColor
	}
	bStyle := lipgloss.NewStyle().Foreground(color)
	tStyle := lipgloss.NewStyle().Foreground(color).Bold(true)

	innerW := max(0, width-2)
	label := " " + title + " "
	labelW := lipgloss.Width(label)
	fill := max(0, innerW-1-labelW)

	return bStyle.Render("╭─") + tStyle.Render(label) + bStyle.Render(strings.Repeat("─", fill)+"╮")
}
