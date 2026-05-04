package main

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

var _ tabbedChild = (*serverLogPane)(nil)

const serverLogHangingIndent = 9

type serverLogPane struct {
	scrollableViewport
	ring *ringBuffer
}

func newServerLogPane() *serverLogPane {
	return &serverLogPane{
		scrollableViewport: newScrollableViewport(),
		ring:               newRingBuffer(5000),
	}
}

func (p *serverLogPane) Init() tea.Cmd { return nil }

func (p *serverLogPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ServerLogMsg:
		p.ring.Push(formatServerLine(msg))
		p.dirty = true
		return p, nil

	case ServerCmdResultMsg:
		for _, line := range formatRconResult(msg) {
			p.ring.Push(line)
		}
		p.dirty = true
		return p, nil

	case tea.KeyPressMsg:
		if p.handleScrollKey(msg) {
			return p, nil
		}
	}

	return p, p.delegateViewport(msg)
}

func (p *serverLogPane) View() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), serverLogHangingIndent)
	}
	return renderBorderedPane(p.viewport.View(), p.Title(), p.width, p.height, p.focused)
}

func (p *serverLogPane) viewportView() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), serverLogHangingIndent)
	}
	return p.viewport.View()
}

func (p *serverLogPane) Title() string     { return "Server Log" }
func (p *serverLogPane) Focused() bool     { return p.focused }
func (p *serverLogPane) SetFocused(f bool) { p.focused = f }
func (p *serverLogPane) SetSize(w, h int)  { p.setInnerSize(w, h) }

func formatRconResult(msg ServerCmdResultMsg) []string {
	ts := dimStyle.Render(time.Now().Format("15:04:05"))
	lines := []string{ts + " " + rconCmdStyle.Render("[rcon] > "+msg.Command)}
	if msg.Err != nil {
		lines = append(lines, ts+" "+warnStyle.Render("[rcon] error: "+msg.Err.Error()))
	} else if msg.Output != "" {
		for _, line := range strings.Split(msg.Output, "\n") {
			lines = append(lines, ts+" "+rconResultStyle.Render("[rcon] "+line))
		}
	}
	return lines
}

func formatServerLine(msg ServerLogMsg) string {
	ts := dimStyle.Render(msg.Time.Format("15:04:05"))
	line := msg.Line
	if msg.Stream == "stderr" {
		line = warnStyle.Render(msg.Line)
	}
	return ts + " " + line
}
