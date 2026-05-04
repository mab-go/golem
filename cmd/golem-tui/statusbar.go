package main

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/mab-go/golem/internal/publisher"
)

type componentState struct {
	status publisher.Status
	detail string
}

type statusBar struct {
	width          int
	components     map[string]componentState
	componentOrder []string
	inputActive    bool
	serverEnabled  bool
}

func newStatusBar(serverEnabled bool) statusBar {
	order := []string{"controller", "player"}
	if serverEnabled {
		order = []string{"server", "controller", "player"}
	}
	return statusBar{
		components:     make(map[string]componentState),
		componentOrder: order,
		serverEnabled:  serverEnabled,
	}
}

func (s *statusBar) UpdateComponent(name string, status publisher.Status, detail string) {
	s.components[name] = componentState{status: status, detail: detail}
}

func (s statusBar) View() string {
	title := statusTitleStyle.Render("golem-tui")
	remaining := max(0, s.width-lipgloss.Width(title))

	var right string
	switch {
	case s.inputActive:
		right = s.renderInputHints(remaining)
	default:
		summary := s.componentSummary()
		if summary == "" {
			right = s.renderDefaultHints(remaining)
		} else {
			right = s.renderComponentView(remaining, summary)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, title, right)
}

func (s statusBar) renderInputHints(width int) string {
	return statusHelpStyle.Width(width).Render("enter send │ esc cancel │ ↑↓ history")
}

func (s statusBar) renderDefaultHints(width int) string {
	hints := "q quit │ tab focus │ 1-3 panes │ [ ] tabs │ ? help"
	if s.serverEnabled {
		hints += " │ : cmd"
	}
	return statusHelpStyle.Width(width).Render(hints)
}

func (s statusBar) renderComponentView(width int, summary string) string {
	hints := "? help"
	if s.serverEnabled {
		hints += " │ : cmd"
	}
	hintsRendered := dimStyle.Render(hints)
	summaryPadded := " " + summary
	gap := max(1, width-lipgloss.Width(summaryPadded)-lipgloss.Width(hintsRendered)-1)
	return summaryPadded + strings.Repeat(" ", gap) + hintsRendered + " "
}

func (s statusBar) componentSummary() string {
	if len(s.components) == 0 {
		return ""
	}

	var parts []string
	for _, name := range s.componentOrder {
		cs, ok := s.components[name]
		if !ok {
			continue
		}
		parts = append(parts, renderStatusIndicator(name, cs))
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "  "
		}
		result += p
	}
	return result
}

func (s *statusBar) SetWidth(w int) {
	s.width = w
}

func (s *statusBar) SetInputActive(active bool) {
	s.inputActive = active
}

func renderStatusIndicator(name string, cs componentState) string {
	var style lipgloss.Style
	var indicator string
	switch cs.status {
	case publisher.StatusOK:
		style = statusOKStyle
		indicator = "●"
	case publisher.StatusDegraded:
		style = statusDegradedStyle
		indicator = "●"
	case publisher.StatusDown:
		style = statusDownStyle
		indicator = "●"
	}
	label := strings.ToUpper(name[:1]) + name[1:]
	return style.Render(indicator + " " + label + ": " + cs.detail)
}
