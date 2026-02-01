package screens

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	followIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	lineCountStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

var (
	rawModeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	parsedModeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)
)

// RenderOutput renders the output screen
// Shows Claude output with optional follow mode and raw/parsed toggle
func RenderOutput(vp viewport.Model, followMode bool, rawMode bool, lineCount int) string {
	header := ""
	if followMode {
		header = followIndicatorStyle.Render("[FOLLOW]") + " "
	}

	if rawMode {
		header += rawModeStyle.Render("[RAW]") + " "
	} else {
		header += parsedModeStyle.Render("[PARSED]") + " "
	}

	header += lineCountStyle.Render(fmt.Sprintf("Lines: %d", lineCount))

	return header + "\n" + vp.View()
}

// CountLines returns the number of lines in content
func CountLines(content string) int {
	if content == "" {
		return 0
	}
	count := 1
	for _, c := range content {
		if c == '\n' {
			count++
		}
	}
	return count
}
