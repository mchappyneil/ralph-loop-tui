package main

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	primaryColor   = lipgloss.Color("62")  // Purple
	secondaryColor = lipgloss.Color("241") // Gray
	accentColor    = lipgloss.Color("86")  // Cyan
	successColor   = lipgloss.Color("82")  // Green
	errorColor     = lipgloss.Color("196") // Red
)

// Tab bar styles
var (
	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(primaryColor).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Padding(0, 1)

	tabBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Width(100) // Will be overridden by actual width
)

// Status bar styles
var (
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	statusLabelStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)

	statusValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true)

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	statusCompletedStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	statusFinishedStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)
)

// Help bar styles
var (
	helpBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(secondaryColor).
			Padding(0, 1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)
)

// Completion banner style
var (
	completionBannerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(successColor).
				Padding(0, 2).
				Align(lipgloss.Center).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(successColor)
)

