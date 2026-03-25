package screens

import (
	"fmt"
	"strings"

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

var (
	highlightPassStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).Bold(true)

	highlightFailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).Bold(true)

	highlightCommitStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).Bold(true)

	highlightCloseStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).Bold(true)

	phaseHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("62")).Bold(true)

	dimPrefixStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
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

// FormatParsedEventStyled formats a parsed event with visual styling for the output screen.
func FormatParsedEventStyled(eventType, summary string, highlight bool, highlightKind string) string {
	if highlight {
		switch highlightKind {
		case "pass":
			return highlightPassStyle.Render("  PASS  " + summary)
		case "fail":
			return highlightFailStyle.Render("  FAIL  " + summary)
		case "commit":
			return highlightCommitStyle.Render("  COMMIT  " + summary)
		case "close":
			return highlightCloseStyle.Render("  CLOSE  " + summary)
		}
	}

	switch eventType {
	case "tool_call":
		return dimPrefixStyle.Render("  tool  ") + summary
	case "tool_result":
		return dimPrefixStyle.Render("  result  ") + summary
	case "text":
		return dimPrefixStyle.Render("  text  ") + summary
	case "result":
		return dimPrefixStyle.Render("  ") + summary
	default:
		return "  " + summary
	}
}

// FormatPhaseHeader renders a phase transition header for the output screen.
func FormatPhaseHeader(phase string, iteration int) string {
	header := fmt.Sprintf(" %s (iteration %d) ", phase, iteration)
	rule := strings.Repeat("━", 40)
	return phaseHeaderStyle.Render(rule[:20] + header + rule[:20])
}
