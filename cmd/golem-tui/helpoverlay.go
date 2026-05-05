package main

import lipgloss "charm.land/lipgloss/v2"

func renderHelpOverlay(width, height int) string {
	help := "" +
		"Keyboard Shortcuts\n" +
		"\n" +
		"  Tab        Cycle focus between panes\n" +
		"  1-3        Toggle pane visibility (Mind, Logs, Chat)\n" +
		"  [ or ]    Switch tab in focused Logs pane\n" +
		"  j/k        Scroll focused pane\n" +
		"  g          Jump to top\n" +
		"  G          Jump to bottom (follow mode)\n" +
		"  :          Server command (requires --server)\n" +
		"  /          Remote command (requires sidecar)\n" +
		"  ?          Toggle this help\n" +
		"  q          Quit\n" +
		"  Ctrl+C     Force quit\n" +
		"\n" +
		"  Press any key to dismiss"

	box := overlayStyle.Render(help)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
