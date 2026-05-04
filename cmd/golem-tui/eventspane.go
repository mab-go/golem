package main

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// eventsHangingIndent covers the timestamp + longest tag label "[memory]" so
// continuation lines align past the tag column.
const eventsHangingIndent = 18

type eventsPane struct {
	scrollableViewport
	ring *ringBuffer
}

func newEventsPane() *eventsPane {
	return &eventsPane{
		scrollableViewport: newScrollableViewport(),
		ring:               newRingBuffer(5000),
	}
}

var _ tabbedChild = (*eventsPane)(nil)

func (p *eventsPane) Init() tea.Cmd { return nil }

func (p *eventsPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case MemoryUpdateMsg:
		p.pushLine(formatEvent(catMemoryStyle, "memory", fmt.Sprintf("%s: %s", msg.File, msg.Preview)))
		return p, nil

	case GameEventMsg:
		if msg.Priority == 0 {
			return p, nil
		}
		p.pushLine(formatEvent(catGameStyle, "game", msg.Description))
		return p, nil

	case ToolResultMsg:
		if msg.IsError {
			p.pushLine(formatEvent(catErrorStyle, "error", fmt.Sprintf("%s failed", msg.Name)))
		} else {
			p.pushLine(formatEvent(catToolStyle, "tool", fmt.Sprintf("%s ok (%s)", msg.Name, msg.Elapsed.Round(time.Millisecond))))
		}
		return p, nil

	case AgentCycleMsg:
		p.pushLine(formatEvent(catWakeStyle, "wake", fmt.Sprintf("Cycle %d (%s)", msg.Cycle, msg.Reason)))
		return p, nil

	case tea.KeyPressMsg:
		if p.handleScrollKey(msg) {
			return p, nil
		}
	}

	return p, p.delegateViewport(msg)
}

func (p *eventsPane) pushLine(line string) {
	p.ring.Push(line)
	p.dirty = true
}

func (p *eventsPane) View() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), eventsHangingIndent)
	}
	return renderBorderedPane(p.viewport.View(), p.Title(), p.width, p.height, p.focused)
}

func (p *eventsPane) viewportView() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), eventsHangingIndent)
	}
	return p.viewport.View()
}

func (p *eventsPane) Title() string     { return "Events" }
func (p *eventsPane) Focused() bool     { return p.focused }
func (p *eventsPane) SetFocused(f bool) { p.focused = f }
func (p *eventsPane) SetSize(w, h int)  { p.setInnerSize(w, h) }

func formatEvent(style lipgloss.Style, tag, text string) string {
	ts := dimStyle.Render(time.Now().Format("15:04:05"))
	label := style.Render(fmt.Sprintf("[%s]", tag))
	return fmt.Sprintf("%s %s %s", ts, label, text)
}
