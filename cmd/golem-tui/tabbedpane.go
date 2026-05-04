package main

import (
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// tabbedChild is a Pane that can be rendered inside a tabbedPane. The extra
// viewportView method exposes just the scrollable content (without the
// child's own border) so the tabbedPane can place it beneath a shared tab
// strip inside a single outer frame.
type tabbedChild interface {
	Pane
	viewportView() string
}

// tabBorderWithBottom is the lipgloss trick for drawing browser-style tabs:
// each tab is a rounded box whose bottom corners use junction characters so
// the tabs merge cleanly with the content pane below. The active tab uses an
// open bottom (" ") so it appears to flow into the content area; inactive
// tabs use a horizontal segment on the bottom so they sit on the pane's top line.
func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	b := lipgloss.RoundedBorder()
	b.BottomLeft = left
	b.Bottom = middle
	b.BottomRight = right
	return b
}

var (
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
)

// tabStripHeight is the total rows a tab occupies: top border, label,
// bottom-border/junction row.
const tabStripHeight = 3

var _ Pane = (*tabbedPane)(nil)

type tabbedPane struct {
	children []tabbedChild
	active   int
	focused  bool
	width    int
	height   int
}

func newTabbedPane(children ...tabbedChild) *tabbedPane {
	return &tabbedPane{children: children}
}

func (t *tabbedPane) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, c := range t.children {
		if cmd := c.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (t *tabbedPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && t.focused {
		switch key.String() {
		case "[", "shift+tab":
			t.setActive((t.active - 1 + len(t.children)) % len(t.children))
			return t, nil
		case "]":
			t.setActive((t.active + 1) % len(t.children))
			return t, nil
		}
	}

	if _, ok := msg.(ServerCmdResultMsg); ok {
		for i, c := range t.children {
			if c.Title() == "Server Log" {
				t.setActive(i)
				break
			}
		}
	}

	var cmds []tea.Cmd
	for i, c := range t.children {
		updated, cmd := c.Update(msg)
		t.children[i] = updated.(tabbedChild)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return t, tea.Batch(cmds...)
}

func (t *tabbedPane) View() string {
	if len(t.children) == 0 {
		return ""
	}

	borderCol := t.borderColor()

	var tabs []string
	for i, c := range t.children {
		tabs = append(tabs, t.renderTab(c.Title(), i == t.active, i == 0, borderCol))
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Extend the tab strip's bottom line out to the right edge and cap it
	// with a top-right curve so the content pane's right border connects cleanly.
	gapWidth := max(0, t.width-lipgloss.Width(tabRow))
	if gapWidth > 0 {
		tabRow = lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow, t.renderTabGap(gapWidth, borderCol))
	}

	contentH := max(0, t.height-tabStripHeight)
	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(borderCol).
		Width(t.width).
		Height(contentH).
		MaxHeight(contentH).
		Render(t.children[t.active].viewportView())

	return tabRow + "\n" + content
}

func (t *tabbedPane) borderColor() color.Color {
	if t.focused {
		return focusedBorderColor
	}
	return unfocusedBorderColor
}

func (t *tabbedPane) renderTab(label string, active, isFirst bool, borderCol color.Color) string {
	border := inactiveTabBorder
	if active {
		border = activeTabBorder
	}
	// For the leftmost tab, the tab's bottom-left corner shares a column with
	// the content pane's left wall. Replace inactive bottom-left corners (no
	// downward stroke) with a vertical or tee junction so the pane's left wall
	// flows continuously through the tab strip.
	if isFirst {
		if active {
			border.BottomLeft = "│"
		} else {
			border.BottomLeft = "├"
		}
	}
	style := lipgloss.NewStyle().
		Border(border, true).
		BorderForeground(borderCol).
		Padding(0, 1).
		Foreground(unfocusedBorderColor)

	if active {
		style = style.Foreground(borderCol)
		if t.focused {
			style = style.Bold(true)
		}
	}
	return style.Render(label)
}

// renderTabGap draws the blank space to the right of the last tab: two blank
// rows for the tab-row height, a horizontal line that continues the tab
// bottoms, and a top-right curve so the line connects to the content pane's right
// border below.
func (t *tabbedPane) renderTabGap(width int, borderCol color.Color) string {
	if width <= 0 {
		return ""
	}
	line := lipgloss.NewStyle().Foreground(borderCol).Render(strings.Repeat("─", width-1) + "╮")
	blank := strings.Repeat(" ", width)
	return blank + "\n" + blank + "\n" + line
}

func (t *tabbedPane) Title() string {
	if len(t.children) == 0 || t.active < 0 || t.active >= len(t.children) {
		return "Logs"
	}
	return t.children[t.active].Title()
}

func (t *tabbedPane) Focused() bool { return t.focused }

func (t *tabbedPane) SetFocused(f bool) {
	t.focused = f
	for i, c := range t.children {
		c.SetFocused(f && i == t.active)
	}
}

// SetSize sizes the children so their viewports exactly fill the content
// pane. The content pane has no top border (formed by tab bottoms) and a
// bottom border, so its content area is (width-2) x (height-tabStripHeight-1).
// A child's SetSize(w, h) produces a viewport of (w-2, h-2); passing
// (width, height-2) gives viewport = (width-2, height-4), which matches.
func (t *tabbedPane) SetSize(width, height int) {
	t.width = width
	t.height = height
	childH := max(0, height-2)
	for _, c := range t.children {
		c.SetSize(width, childH)
	}
}

func (t *tabbedPane) setActive(idx int) {
	if idx == t.active || idx < 0 || idx >= len(t.children) {
		return
	}
	if t.focused {
		t.children[t.active].SetFocused(false)
		t.children[idx].SetFocused(true)
	}
	t.active = idx
}
