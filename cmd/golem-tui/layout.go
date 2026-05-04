package main

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// Layout slot indices. The TUI has three visible regions:
//
//	0 mind    - left column, Claude's Mind
//	1 tabbed  - right column top: Agent Log / Sidecar Log / Events (tabbed)
//	2 chat    - right column bottom
const (
	slotMind   = 0
	slotTabbed = 1
	slotChat   = 2
	slotCount  = 3
)

type paneSlot struct {
	pane    Pane
	visible bool
}

type layout struct {
	slots      [slotCount]paneSlot
	focusIndex int
	width      int
	height     int
}

func newLayout(mind, tabbed, chat Pane) layout {
	slots := [slotCount]paneSlot{
		{pane: mind, visible: true},
		{pane: tabbed, visible: true},
		{pane: chat, visible: true},
	}
	slots[slotMind].pane.SetFocused(true)
	return layout{slots: slots}
}

func (l *layout) SetSize(w, h int) {
	l.width = w
	l.height = h
	l.distributeSpace()
}

// distributeSpace is the single place that decides pixel budgets for each
// visible pane. Mind takes 45% of the width when present; the right column
// splits vertically 65/35 between the tabbed log group and Chat.
func (l *layout) distributeSpace() {
	visible := l.visibleIndices()
	if len(visible) == 0 {
		return
	}

	mindVisible := l.slots[slotMind].visible
	right := l.visibleRight()

	if mindVisible && len(right) == 0 {
		l.slots[slotMind].pane.SetSize(l.width, l.height)
		return
	}

	if !mindVisible {
		l.distributeRight(l.width, l.height, right)
		return
	}

	leftW := l.width * 45 / 100
	rightW := l.width - leftW
	l.slots[slotMind].pane.SetSize(leftW, l.height)
	l.distributeRight(rightW, l.height, right)
}

// distributeRight sizes whichever of tabbed/chat are visible inside a (w,h)
// region. With both visible, tabbed gets 65% of the height; with only one
// visible, it fills the region.
func (l *layout) distributeRight(w, h int, visible []int) {
	switch len(visible) {
	case 0:
		return
	case 1:
		l.slots[visible[0]].pane.SetSize(w, h)
	default:
		tabH := h * 65 / 100
		chatH := h - tabH
		l.slots[slotTabbed].pane.SetSize(w, tabH)
		l.slots[slotChat].pane.SetSize(w, chatH)
	}
}

func (l *layout) visibleRight() []int {
	var out []int
	for i := slotTabbed; i < slotCount; i++ {
		if l.slots[i].visible {
			out = append(out, i)
		}
	}
	return out
}

func (l *layout) visibleIndices() []int {
	var out []int
	for i := range slotCount {
		if l.slots[i].visible {
			out = append(out, i)
		}
	}
	return out
}

func (l *layout) visibleCount() int {
	n := 0
	for _, s := range l.slots {
		if s.visible {
			n++
		}
	}
	return n
}

func (l *layout) TogglePane(index int) {
	if index < 0 || index >= slotCount {
		return
	}
	if l.slots[index].visible && l.visibleCount() <= 1 {
		return
	}
	l.slots[index].visible = !l.slots[index].visible
	if l.focusIndex == index && !l.slots[index].visible {
		l.slots[index].pane.SetFocused(false)
		l.FocusNext()
	}
	l.distributeSpace()
}

func (l *layout) ShowPane(index int) {
	if index < 0 || index >= slotCount {
		return
	}
	if l.slots[index].visible {
		return
	}
	l.slots[index].visible = true
	l.distributeSpace()
}

func (l *layout) FocusNext() {
	visible := l.visibleIndices()
	if len(visible) == 0 {
		return
	}
	l.slots[l.focusIndex].pane.SetFocused(false)
	cur := 0
	for i, idx := range visible {
		if idx == l.focusIndex {
			cur = i
			break
		}
	}
	next := visible[(cur+1)%len(visible)]
	l.focusIndex = next
	l.slots[l.focusIndex].pane.SetFocused(true)
}

func (l layout) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, s := range l.slots {
		if cmd := s.pane.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (l layout) Update(msg tea.Msg) (layout, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "tab" {
		l.FocusNext()
		return l, nil
	}

	var cmds []tea.Cmd
	for i := range l.slots {
		updated, cmd := l.slots[i].pane.Update(msg)
		l.slots[i].pane = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return l, tea.Batch(cmds...)
}

func (l layout) View() string {
	mindVisible := l.slots[slotMind].visible
	right := l.visibleRight()

	if !mindVisible && len(right) == 0 {
		visible := l.visibleIndices()
		if len(visible) == 0 {
			return ""
		}
		return l.slots[visible[0]].pane.View()
	}

	rightCol := l.rightColumnView(right)

	if mindVisible && len(right) == 0 {
		return l.slots[slotMind].pane.View()
	}

	if !mindVisible {
		return rightCol
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, l.slots[slotMind].pane.View(), rightCol)
}

func (l layout) rightColumnView(visible []int) string {
	switch len(visible) {
	case 0:
		return ""
	case 1:
		return l.slots[visible[0]].pane.View()
	default:
		var views []string
		for _, idx := range visible {
			views = append(views, l.slots[idx].pane.View())
		}
		return lipgloss.JoinVertical(lipgloss.Left, views...)
	}
}
