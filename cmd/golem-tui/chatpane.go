package main

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

const chatHangingIndent = 9

type chatPane struct {
	scrollableViewport
	ring    *ringBuffer
	hasData bool
}

func newChatPane() *chatPane {
	return &chatPane{
		scrollableViewport: newScrollableViewport(),
		ring:               newRingBuffer(2000),
	}
}

var _ Pane = (*chatPane)(nil)

func (p *chatPane) Init() tea.Cmd { return nil }

func (p *chatPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ChatMsg:
		p.ring.Push(formatChatLine(msg))
		p.dirty = true
		if !p.hasData {
			p.hasData = true
			return p, func() tea.Msg { return ChatPaneActivateMsg{} }
		}
		return p, nil

	case tea.KeyPressMsg:
		if p.handleScrollKey(msg) {
			return p, nil
		}
	}

	return p, p.delegateViewport(msg)
}

func (p *chatPane) View() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), chatHangingIndent)
	}
	return renderBorderedPane(p.viewport.View(), p.Title(), p.width, p.height, p.focused)
}

func (p *chatPane) Title() string     { return "Chat" }
func (p *chatPane) Focused() bool     { return p.focused }
func (p *chatPane) SetFocused(f bool) { p.focused = f }
func (p *chatPane) SetSize(w, h int)  { p.setInnerSize(w, h) }

func formatChatLine(msg ChatMsg) string {
	ts := dimStyle.Render(time.Now().Format("15:04:05"))
	text := fmt.Sprintf("<%s> %s", msg.Sender, msg.Message)
	if msg.IsBot {
		text = botChatStyle.Render(text)
	}
	return ts + " " + text
}
