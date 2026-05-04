package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const wrapBreakpoints = " -/,=:"

// wrapLines wraps each line of s to width, applying a hanging indent to any
// wrapped continuation lines so they visually align past a prefix column
// (e.g., past "timestamp level" in a log pane). Lines already fitting within
// width are returned unchanged.
//
// ANSI escape sequences are preserved by ansi.Wrap. width <= 0 or
// hangingIndent >= width returns s unchanged.
func wrapLines(s string, width, hangingIndent int) string {
	if width <= 0 || hangingIndent < 0 || hangingIndent >= width {
		return s
	}

	pad := strings.Repeat(" ", hangingIndent)
	contWidth := width - hangingIndent

	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if ansi.StringWidth(line) <= width {
			out = append(out, line)
			continue
		}

		// First pass: wrap to full width so the first line uses all available
		// columns.
		wrapped := ansi.Wrap(line, width, wrapBreakpoints)
		parts := strings.Split(wrapped, "\n")
		out = append(out, parts[0])
		if len(parts) == 1 {
			continue
		}

		// Re-wrap the remainder at the narrower continuation width and
		// prepend the hanging indent.
		rest := strings.Join(parts[1:], " ")
		restWrapped := ansi.Wrap(rest, contWidth, wrapBreakpoints)
		for rl := range strings.SplitSeq(restWrapped, "\n") {
			out = append(out, pad+rl)
		}
	}
	return strings.Join(out, "\n")
}
