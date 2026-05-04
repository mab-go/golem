package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mab-go/golem/internal/logging"
)

var skipFields = []string{"delta", "input"}

// logHangingIndent aligns wrapped continuation lines past the timestamp and
// level columns (8 + 1 + 5 = 14 cells), leaving non-event log lines from being
// over-indented while still keeping continuations visually distinct.
const logHangingIndent = 14

type logPane struct {
	scrollableViewport
	ring *ringBuffer
}

func newLogPane() *logPane {
	return &logPane{
		scrollableViewport: newScrollableViewport(),
		ring:               newRingBuffer(10000),
	}
}

var _ tabbedChild = (*logPane)(nil)

func (p *logPane) Init() tea.Cmd { return nil }

func (p *logPane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case LogMsg:
		p.ring.Push(formatLogEntry(msg))
		p.dirty = true
		return p, nil

	case tea.KeyPressMsg:
		if p.handleScrollKey(msg) {
			return p, nil
		}
	}

	return p, p.delegateViewport(msg)
}

func (p *logPane) View() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), logHangingIndent)
	}
	return renderBorderedPane(p.viewport.View(), p.Title(), p.width, p.height, p.focused)
}

func (p *logPane) viewportView() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), logHangingIndent)
	}
	return p.viewport.View()
}

func (p *logPane) Title() string     { return "Agent Log" }
func (p *logPane) Focused() bool     { return p.focused }
func (p *logPane) SetFocused(f bool) { p.focused = f }
func (p *logPane) SetSize(w, h int)  { p.setInnerSize(w, h) }

func formatLogEntry(msg LogMsg) string {
	ts := dimStyle.Render(msg.Time.Format("15:04:05"))
	lvl := renderLogLevel(msg.Level)

	var parts []string
	parts = append(parts, ts, lvl)

	if msg.Event != "" {
		parts = append(parts, eventStyle.Render(fmt.Sprintf("%-18s", msg.Event)))
	}

	if msg.Message != "" && msg.Message != msg.Event {
		parts = append(parts, msg.Message)
	}

	fields := formatFields(msg.Fields)
	if fields != "" {
		parts = append(parts, dimStyle.Render(fields))
	}

	return strings.Join(parts, " ")
}

func formatFields(fields logging.Fields) string {
	if len(fields) == 0 {
		return ""
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		if slices.Contains(skipFields, k) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := fields[k]
		if k == "error" || k == "err" {
			parts = append(parts, errField.Render(fmt.Sprintf("%s=%v", k, v)))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return strings.Join(parts, " ")
}

func renderLogLevel(level logging.Level) string {
	switch level {
	case logging.DebugLevel:
		return dimStyle.Render("DEBUG")
	case logging.InfoLevel:
		return infoStyle.Render("INFO ")
	case logging.WarnLevel:
		return warnStyle.Render("WARN ")
	case logging.ErrorLevel:
		return errorStyle.Render("ERROR")
	case logging.FatalLevel:
		return errorStyle.Render("FATAL")
	case logging.PanicLevel:
		return errorStyle.Render("PANIC")
	default:
		return dimStyle.Render("?????")
	}
}
