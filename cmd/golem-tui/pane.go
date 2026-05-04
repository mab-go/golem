package main

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// Pane is a rectangular region of the TUI that can receive messages,
// render itself, and track focus state. Update returns (Pane, tea.Cmd)
// rather than (tea.Model, tea.Cmd) so the layout can store panes
// without type assertions.
type Pane interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Pane, tea.Cmd)
	View() string
	Title() string
	Focused() bool
	SetFocused(bool)
	SetSize(width, height int)
}

// scrollableViewport holds the shared plumbing for viewport-based panes:
// scroll key handling, viewport delegation, dirty-flag content rebuild,
// and size management. Panes embed this to avoid duplicating ~20 lines
// of identical scroll logic in each Update method.
type scrollableViewport struct {
	viewport viewport.Model
	focused  bool
	follow   bool
	dirty    bool
	width    int
	height   int
}

func newScrollableViewport() scrollableViewport {
	return scrollableViewport{viewport: viewport.New(), follow: true}
}

// handleScrollKey processes G/end (jump to bottom) and g/home (jump to
// top). Returns true when the key was consumed so the caller can
// short-circuit.
func (sv *scrollableViewport) handleScrollKey(msg tea.KeyPressMsg) bool {
	if !sv.focused {
		return false
	}
	switch msg.String() {
	case "G", "end":
		sv.follow = true
		sv.viewport.GotoBottom()
		return true
	case "g", "home":
		sv.follow = false
		sv.viewport.GotoTop()
		return true
	}
	return false
}

// delegateViewport forwards a message to the underlying viewport model
// and syncs the follow flag. Call only when focused; returns the
// resulting tea.Cmd.
func (sv *scrollableViewport) delegateViewport(msg tea.Msg) tea.Cmd {
	if !sv.focused {
		return nil
	}
	var cmd tea.Cmd
	sv.viewport, cmd = sv.viewport.Update(msg)
	sv.follow = sv.viewport.AtBottom()
	return cmd
}

// setInnerSize updates the outer dimensions and resizes the viewport's
// content area (inset by 2 for the border).
func (sv *scrollableViewport) setInnerSize(width, height int) {
	sv.width = width
	sv.height = height
	sv.viewport.SetWidth(max(0, width-2))
	sv.viewport.SetHeight(max(0, height-2))
	sv.dirty = true
}

func (sv *scrollableViewport) contentWidth() int {
	return max(0, sv.width-2)
}

// rebuildFromLines joins the lines, wraps them to the content width
// with hangingIndent, and sets the viewport content.
func (sv *scrollableViewport) rebuildFromLines(lines []string, hangingIndent int) {
	cw := sv.contentWidth()
	joined := strings.Join(lines, "\n")
	sv.viewport.SetContent(wrapLines(joined, cw, hangingIndent))
	if sv.follow {
		sv.viewport.GotoBottom()
	}
	sv.dirty = false
}
