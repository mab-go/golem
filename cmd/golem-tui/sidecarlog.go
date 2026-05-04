package main

import tea "charm.land/bubbletea/v2"

var _ tabbedChild = (*sidecarLogPane)(nil)

const sidecarHangingIndent = 9

type sidecarLogPane struct {
	scrollableViewport
	ring *ringBuffer
}

func newSidecarLogPane() *sidecarLogPane {
	return &sidecarLogPane{
		scrollableViewport: newScrollableViewport(),
		ring:               newRingBuffer(5000),
	}
}

func (p *sidecarLogPane) Init() tea.Cmd { return nil }

func (p *sidecarLogPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case SidecarLogMsg:
		p.ring.Push(formatSidecarLine(msg))
		p.dirty = true
		return p, nil

	case tea.KeyPressMsg:
		if p.handleScrollKey(msg) {
			return p, nil
		}
	}

	return p, p.delegateViewport(msg)
}

func (p *sidecarLogPane) View() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), sidecarHangingIndent)
	}
	return renderBorderedPane(p.viewport.View(), p.Title(), p.width, p.height, p.focused)
}

func (p *sidecarLogPane) viewportView() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), sidecarHangingIndent)
	}
	return p.viewport.View()
}

func (p *sidecarLogPane) Title() string     { return "Sidecar Log" }
func (p *sidecarLogPane) Focused() bool     { return p.focused }
func (p *sidecarLogPane) SetFocused(f bool) { p.focused = f }
func (p *sidecarLogPane) SetSize(w, h int)  { p.setInnerSize(w, h) }

func formatSidecarLine(msg SidecarLogMsg) string {
	ts := dimStyle.Render(msg.Time.Format("15:04:05"))
	line := msg.Line
	if msg.Stream == "stderr" {
		line = warnStyle.Render(msg.Line)
	}
	return ts + " " + line
}
