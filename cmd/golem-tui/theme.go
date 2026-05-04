package main

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

// Palette inspired by the Anthropic brand guidelines:
//   https://github.com/anthropics/skills/blob/main/skills/brand-guidelines/SKILL.md
//   https://geist.co/work/anthropic

//nolint:unused // Full palette kept for future use.
var (
	colorSlateDark   = lipgloss.Color("#191919")
	colorSlateMedium = lipgloss.Color("#262625")
	colorSlateLight  = lipgloss.Color("#40403E")

	colorCloudDark   = lipgloss.Color("#666663")
	colorCloudMedium = lipgloss.Color("#91918D")
	colorCloudLight  = lipgloss.Color("#BFBFBA")

	colorIvoryDark   = lipgloss.Color("#E5E4DF")
	colorIvoryMedium = lipgloss.Color("#F0F0EB")
	colorIvoryLight  = lipgloss.Color("#FAFAF7")

	colorOrange  = lipgloss.Color("#D97757")
	colorKraft   = lipgloss.Color("#DFA27F")
	colorManilla = lipgloss.Color("#EBDBBC")
	colorBlue    = lipgloss.Color("#7CAFD6")
	colorGreen   = lipgloss.Color("#8FAF74")
	colorRed     = lipgloss.Color("#CC5748")
)

var (
	focusedBorderColor   color.Color = colorOrange
	unfocusedBorderColor color.Color = colorCloudMedium
)

var (
	focusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(focusedBorderColor)

	unfocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(unfocusedBorderColor)

	cmdInputStyle = lipgloss.NewStyle().
			Background(colorSlateMedium).
			Foreground(colorIvoryDark).
			Padding(0, 1)
)

var (
	dimStyle        = lipgloss.NewStyle().Foreground(colorCloudDark)
	infoStyle       = lipgloss.NewStyle().Foreground(colorBlue)
	warnStyle       = lipgloss.NewStyle().Foreground(colorKraft)
	errorStyle      = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	eventStyle      = lipgloss.NewStyle().Foreground(colorGreen)
	errField        = lipgloss.NewStyle().Foreground(colorRed)
	rconCmdStyle    = lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	rconResultStyle = lipgloss.NewStyle().Foreground(colorKraft)
)

var (
	toolMarkerStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	toolErrorStyle  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	separatorStyle  = lipgloss.NewStyle().Foreground(colorCloudMedium)
	streamingDots   = lipgloss.NewStyle().Foreground(colorBlue)
	gkWakeStyle     = lipgloss.NewStyle().Foreground(colorKraft).Bold(true)
	gkIdleStyle     = lipgloss.NewStyle().Foreground(colorCloudDark)
)

var (
	statusTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSlateDark).
				Background(colorOrange).
				Padding(0, 1)

	statusHelpStyle = lipgloss.NewStyle().
			Foreground(colorCloudMedium).
			Padding(0, 1)

	statusOKStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Padding(0, 1)

	statusDegradedStyle = lipgloss.NewStyle().
				Foreground(colorKraft).
				Padding(0, 1)

	statusDownStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Padding(0, 1)
)

var (
	catMemoryStyle = lipgloss.NewStyle().Foreground(colorKraft)
	catGameStyle   = lipgloss.NewStyle().Foreground(colorGreen)
	catToolStyle   = lipgloss.NewStyle().Foreground(colorBlue)
	catWakeStyle   = lipgloss.NewStyle().Foreground(colorOrange)
	catErrorStyle  = lipgloss.NewStyle().Foreground(colorRed)
)

var overlayStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorOrange).
	Padding(1, 3)

var botChatStyle = lipgloss.NewStyle().Foreground(colorKraft)
