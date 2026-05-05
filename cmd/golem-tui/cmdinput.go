package main

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

const maxHistory = 100

type cmdInput struct {
	input      textinput.Model
	history    []string
	historyIdx int // -1 = composing new, 0..len-1 indexes from newest
	draft      string
	width      int
}

func newCmdInput() cmdInput {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.Placeholder = "server command"
	promptStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(colorIvoryDark)
	placeholderStyle := lipgloss.NewStyle().Foreground(colorSlateLight)
	styles := ti.Styles()
	styles.Focused.Prompt = promptStyle
	styles.Focused.Text = textStyle
	styles.Focused.Placeholder = placeholderStyle
	styles.Blurred.Prompt = promptStyle
	styles.Blurred.Text = textStyle
	styles.Blurred.Placeholder = placeholderStyle
	ti.SetStyles(styles)
	ti.KeyMap.AcceptSuggestion = key.NewBinding()
	ti.KeyMap.NextSuggestion = key.NewBinding()
	ti.KeyMap.PrevSuggestion = key.NewBinding()
	return cmdInput{input: ti, historyIdx: -1}
}

func (c *cmdInput) Activate() tea.Cmd {
	c.historyIdx = -1
	c.draft = ""
	c.input.SetValue("")
	return c.input.Focus()
}

func (c *cmdInput) Deactivate() {
	c.input.Blur()
	c.input.SetValue("")
	c.historyIdx = -1
	c.draft = ""
}

func (c *cmdInput) Submit() (string, bool) {
	val := strings.TrimSpace(c.input.Value())
	c.input.SetValue("")
	c.input.Blur()
	c.historyIdx = -1
	c.draft = ""
	if val == "" {
		return "", false
	}
	if len(c.history) == 0 || c.history[len(c.history)-1] != val {
		c.history = append(c.history, val)
		if len(c.history) > maxHistory {
			c.history = c.history[len(c.history)-maxHistory:]
		}
	}
	return val, true
}

func (c *cmdInput) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "up":
			c.historyUp()
			return nil
		case "down":
			c.historyDown()
			return nil
		}
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

func (c *cmdInput) historyUp() {
	if len(c.history) == 0 {
		return
	}
	switch {
	case c.historyIdx == -1:
		c.draft = c.input.Value()
		c.historyIdx = len(c.history) - 1
	case c.historyIdx > 0:
		c.historyIdx--
	default:
		return
	}
	c.input.SetValue(c.history[c.historyIdx])
	c.input.CursorEnd()
}

func (c *cmdInput) historyDown() {
	switch {
	case c.historyIdx == -1:
		return
	case c.historyIdx < len(c.history)-1:
		c.historyIdx++
		c.input.SetValue(c.history[c.historyIdx])
	default:
		c.historyIdx = -1
		c.input.SetValue(c.draft)
	}
	c.input.CursorEnd()
}

func (c *cmdInput) SetPrompt(prompt, placeholder string) {
	c.input.Prompt = prompt
	c.input.Placeholder = placeholder
}

func (c *cmdInput) SetWidth(w int) {
	c.width = w
	c.input.SetWidth(max(0, w-4))
}

func (c cmdInput) View() string {
	return cmdInputStyle.Width(c.width).Render(c.input.View())
}
