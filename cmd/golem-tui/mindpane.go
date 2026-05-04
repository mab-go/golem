package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

var _ Pane = (*mindPane)(nil)

// mindEntryKind distinguishes the structured entries the mind pane keeps so
// width-dependent renderings (separators) can be regenerated when the
// terminal is resized.
type mindEntryKind int

const (
	entrySeparator mindEntryKind = iota
	entryToolExec
	entryToolResult
	entryThought
	entryGatekeeper
)

type mindEntry struct {
	kind    mindEntryKind
	cycle   uint64
	reason  string
	name    string
	elapsed time.Duration
	isError bool
	wake    bool
	text    string
}

type mindPane struct {
	scrollableViewport

	current   strings.Builder
	streaming bool

	entries    []mindEntry
	maxEntries int
}

func newMindPane() *mindPane {
	return &mindPane{
		scrollableViewport: newScrollableViewport(),
		maxEntries:         200,
	}
}

func (p *mindPane) Init() tea.Cmd { return nil }

func (p *mindPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case AgentCycleMsg:
		p.finalizeCurrentResponse()
		p.pushEntry(mindEntry{kind: entrySeparator, cycle: msg.Cycle, reason: msg.Reason})
		p.streaming = false
		p.dirty = true
		return p, nil

	case TextDeltaMsg:
		p.current.WriteString(msg.Delta)
		p.streaming = true
		p.dirty = true
		return p, nil

	case ThinkingTextMsg:
		p.finalizeCurrentResponse()
		p.streaming = false
		p.dirty = true
		return p, nil

	case GatekeeperDecisionMsg:
		p.finalizeCurrentResponse()
		p.pushEntry(mindEntry{kind: entryGatekeeper, wake: msg.Wake, reason: msg.Reason})
		p.dirty = true
		return p, nil

	case ToolExecMsg:
		p.pushEntry(mindEntry{kind: entryToolExec, name: msg.Name})
		p.dirty = true
		return p, nil

	case ToolResultMsg:
		p.pushEntry(mindEntry{kind: entryToolResult, name: msg.Name, isError: msg.IsError, elapsed: msg.Elapsed})
		p.dirty = true
		return p, nil

	case tea.KeyPressMsg:
		if p.handleScrollKey(msg) {
			return p, nil
		}
	}

	return p, p.delegateViewport(msg)
}

func (p *mindPane) View() string {
	if p.dirty {
		p.rebuildContent()
	}
	return renderBorderedPane(p.viewport.View(), p.Title(), p.width, p.height, p.focused)
}

func (p *mindPane) Title() string     { return "Claude's Mind" }
func (p *mindPane) Focused() bool     { return p.focused }
func (p *mindPane) SetFocused(f bool) { p.focused = f }
func (p *mindPane) SetSize(w, h int)  { p.setInnerSize(w, h) }

func (p *mindPane) pushEntry(e mindEntry) {
	p.entries = append(p.entries, e)
	if len(p.entries) > p.maxEntries {
		p.entries = p.entries[len(p.entries)-p.maxEntries:]
	}
}

func (p *mindPane) finalizeCurrentResponse() {
	text := p.current.String()
	if text == "" {
		return
	}
	p.pushEntry(mindEntry{kind: entryThought, text: text})
	p.current.Reset()
}

func (p *mindPane) rebuildContent() {
	cw := p.contentWidth()
	var b strings.Builder

	for i, e := range p.entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.renderEntry(e, cw))
	}

	if p.current.Len() > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		text := p.current.String()
		if cw > 0 {
			text = wrapLines(text, cw, 0)
		}
		b.WriteString(text)
		if p.streaming {
			b.WriteString(streamingDots.Render("..."))
		}
	}

	p.viewport.SetContent(b.String())
	if p.follow {
		p.viewport.GotoBottom()
	}
	p.dirty = false
}

func (p *mindPane) renderEntry(e mindEntry, width int) string {
	switch e.kind {
	case entrySeparator:
		return renderCycleSeparator(e.cycle, e.reason, width)
	case entryToolExec:
		return renderToolMarker(e.name)
	case entryToolResult:
		return renderToolResult(e.name, e.isError, e.elapsed)
	case entryThought:
		if width <= 0 {
			return e.text
		}
		return wrapLines(e.text, width, 0)
	case entryGatekeeper:
		return renderGatekeeperDecision(e.wake, e.reason)
	}
	return ""
}

func renderCycleSeparator(cycle uint64, reason string, width int) string {
	label := fmt.Sprintf(" Cycle %d (%s) ", cycle, reason)
	labelW := lipgloss.Width(label)
	if width <= 0 || labelW >= width {
		return separatorStyle.Render(strings.TrimSpace(label))
	}
	remaining := width - labelW
	left := remaining / 2
	right := remaining - left
	sep := strings.Repeat("─", left) + label + strings.Repeat("─", right)
	return separatorStyle.Render(sep)
}

func renderToolMarker(name string) string {
	return toolMarkerStyle.Render("  [tool] " + name)
}

func renderToolResult(name string, isError bool, elapsed time.Duration) string {
	dur := fmt.Sprintf("%.1fs", elapsed.Seconds())
	if isError {
		return toolErrorStyle.Render(fmt.Sprintf("  [error] %s (%s)", name, dur))
	}
	return toolMarkerStyle.Render(fmt.Sprintf("  [result] %s: ok (%s)", name, dur))
}

func renderGatekeeperDecision(wake bool, reason string) string {
	if wake {
		return gkWakeStyle.Render("  [wake] " + reason)
	}
	return gkIdleStyle.Render("  [idle] " + reason)
}
