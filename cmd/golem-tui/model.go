package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/mab-go/golem/internal/publisher"
)

type model struct {
	statusBar     statusBar
	layout        layout
	cmdInput      cmdInput
	inputActive   bool
	serverEnabled bool
	execServerCmd func(string) (string, error)
	logFiles      *logFiles
	width         int
	height        int
	ready         bool
	showHelp      bool
}

func newModel(serverEnabled bool) model {
	mind := newMindPane()
	var tabbed *tabbedPane
	if serverEnabled {
		tabbed = newTabbedPane(newLogPane(), newSidecarLogPane(), newServerLogPane(), newEventsPane())
	} else {
		tabbed = newTabbedPane(newLogPane(), newSidecarLogPane(), newEventsPane())
	}
	chat := newChatPane()
	lay := newLayout(mind, tabbed, chat)

	return model{
		statusBar:     newStatusBar(serverEnabled),
		layout:        lay,
		cmdInput:      newCmdInput(),
		serverEnabled: serverEnabled,
	}
}

func (m model) Init() tea.Cmd {
	return m.layout.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg), nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case ServerReadyMsg:
		m.execServerCmd = msg.ExecCmd
		return m, nil

	case ServerCmdResultMsg:
		if m.logFiles != nil {
			m.logFiles.WriteServerCmd(time.Now(), msg.Command, msg.Output, msg.Err)
		}

	case ChatPaneActivateMsg:
		m.layout.ShowPane(slotChat)
		return m, nil

	case ComponentStatusMsg:
		m.statusBar.UpdateComponent(msg.Component, msg.Status, msg.Detail)
		return m, nil

	case AgentCycleMsg:
		m.statusBar.UpdateComponent("player", publisher.StatusOK, "Thinking...")

	case TurnCompleteMsg:
		m.statusBar.UpdateComponent("player", publisher.StatusOK, "Idle")
	}

	var cmd tea.Cmd
	m.layout, cmd = m.layout.Update(msg)
	return m, cmd
}

func (m model) handleResize(msg tea.WindowSizeMsg) model {
	m.width = msg.Width
	m.height = msg.Height
	m.statusBar.SetWidth(msg.Width)
	m.cmdInput.SetWidth(msg.Width)
	layoutH := msg.Height - 1
	if m.inputActive {
		layoutH = msg.Height - 2
	}
	m.layout.SetSize(msg.Width, layoutH)
	m.ready = true
	return m
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}
	if m.inputActive {
		return m.handleCmdInputKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "1":
		m.layout.TogglePane(slotMind)
		return m, nil
	case "2":
		m.layout.TogglePane(slotTabbed)
		return m, nil
	case "3":
		m.layout.TogglePane(slotChat)
		return m, nil
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case ":":
		return m.activateServerCmd()
	}

	var cmd tea.Cmd
	m.layout, cmd = m.layout.Update(msg)
	return m, cmd
}

func (m model) activateServerCmd() (tea.Model, tea.Cmd) {
	if m.execServerCmd == nil {
		return m, nil
	}
	m.inputActive = true
	m.statusBar.SetInputActive(true)
	m.layout.SetSize(m.width, m.height-2)
	return m, m.cmdInput.Activate()
}

func (m model) handleCmdInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		cmd, ok := m.cmdInput.Submit()
		m.inputActive = false
		m.statusBar.SetInputActive(false)
		m.layout.SetSize(m.width, m.height-1)
		if ok {
			return m, m.runServerCmd(cmd)
		}
		return m, nil
	case "escape":
		m.cmdInput.Deactivate()
		m.inputActive = false
		m.statusBar.SetInputActive(false)
		m.layout.SetSize(m.width, m.height-1)
		return m, nil
	default:
		cmd := m.cmdInput.Update(msg)
		return m, cmd
	}
}

func (m model) runServerCmd(command string) tea.Cmd {
	return func() tea.Msg {
		if m.execServerCmd == nil {
			return ServerCmdResultMsg{Command: command, Err: fmt.Errorf("server not available")}
		}
		out, err := m.execServerCmd(command)
		return ServerCmdResultMsg{Command: command, Output: strings.TrimSpace(out), Err: err}
	}
}

func (m model) View() tea.View {
	var s string
	switch {
	case !m.ready:
		s = "Initializing..."
	case m.showHelp:
		s = renderHelpOverlay(m.width, m.height)
	default:
		parts := []string{m.layout.View()}
		if m.inputActive {
			parts = append(parts, m.cmdInput.View())
		}
		parts = append(parts, m.statusBar.View())
		s = lipgloss.JoinVertical(lipgloss.Top, parts...)
		s = lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(s)
	}
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}
