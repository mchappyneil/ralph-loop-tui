package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fireynis/ralph-loop-go/screens"
)

// View renders the entire UI
func (m model) View() string {
	var b strings.Builder

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// Screen content
	b.WriteString(m.renderScreen())

	// Help bar
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return b.String()
}

func (m model) renderTabBar() string {
	tabs := []struct {
		key    string
		name   string
		screen screenType
	}{
		{"1", "Homebase", screenHomebase},
		{"2", "Output", screenOutput},
		{"3", "Analytics", screenAnalytics},
	}

	var parts []string
	parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).Render("Ralph TUI"))
	parts = append(parts, "  ")

	for _, tab := range tabs {
		label := fmt.Sprintf("[%s] %s", tab.key, tab.name)
		if m.activeScreen == tab.screen {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(label))
		}
		parts = append(parts, " ")
	}

	// Right-align quit hint
	tabContent := strings.Join(parts, "")
	quitHint := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("q:quit ?:help")

	// Calculate padding
	contentWidth := lipgloss.Width(tabContent) + lipgloss.Width(quitHint)
	padding := m.width - contentWidth
	if padding < 1 {
		padding = 1
	}

	return tabBarStyle.Width(m.width).Render(tabContent + strings.Repeat(" ", padding) + quitHint)
}

func (m model) renderStatusBar() string {
	// Current iteration elapsed time
	currentElapsed := ""
	if !m.startTime.IsZero() {
		end := m.endTime
		if end.IsZero() {
			end = time.Now()
		}
		currentElapsed = end.Sub(m.startTime).Truncate(time.Second).String()
	}

	// Total session time
	totalElapsed := time.Since(m.sessionStart).Truncate(time.Second).String()

	// Status with appropriate styling
	statusDisplay := m.statusText
	switch m.status {
	case statusRunning:
		statusDisplay = statusRunningStyle.Render(m.statusText)
	case statusCompleted:
		statusDisplay = statusCompletedStyle.Render(m.statusText)
	case statusError:
		statusDisplay = statusErrorStyle.Render(m.statusText)
	case statusFinished:
		statusDisplay = statusFinishedStyle.Render(m.statusText)
	}

	content := fmt.Sprintf("%s %s | %s %s | %s %s | %s %s",
		statusLabelStyle.Render("Iteration:"),
		statusValueStyle.Render(fmt.Sprintf("%d/%d", m.iteration, m.maxIter)),
		statusLabelStyle.Render("Status:"),
		statusDisplay,
		statusLabelStyle.Render("Current:"),
		statusValueStyle.Render(currentElapsed),
		statusLabelStyle.Render("Total:"),
		statusValueStyle.Render(totalElapsed),
	)

	return statusBarStyle.Width(m.width).Render(content)
}

func (m model) renderScreen() string {
	switch m.activeScreen {
	case screenHomebase:
		return screens.RenderHomebase(m.homebaseVP)
	case screenOutput:
		content := m.outputContent
		if m.showRawOutput {
			content = m.rawOutputLog
		}
		lineCount := screens.CountLines(content)
		return screens.RenderOutput(m.outputVP, m.followOutput, m.showRawOutput, lineCount)
	case screenAnalytics:
		data := m.buildAnalyticsData()
		// Calculate available height for analytics
		availHeight := m.height - 4 // tab bar + status bar + help bar + spacing
		if availHeight < 10 {
			availHeight = 10
		}
		return screens.RenderAnalytics(data, m.width, availHeight)
	default:
		return "Unknown screen"
	}
}

func (m model) buildAnalyticsData() screens.AnalyticsData {
	// Convert iteration history
	history := make([]screens.IterationRecord, len(m.analytics.iterationHistory))
	for i, r := range m.analytics.iterationHistory {
		history[i] = screens.IterationRecord{
			Iteration: r.iteration,
			Duration:  r.duration,
			Passed:    r.passed,
			TaskID:    r.taskID,
			Notes:     r.notes,
		}
	}

	return screens.AnalyticsData{
		CurrentIteration:   m.iteration,
		MaxIterations:      m.maxIter,
		PassedCount:        m.analytics.passedCount,
		FailedCount:        m.analytics.failedCount,
		SessionStart:       m.sessionStart,
		TotalDuration:      m.analytics.totalDuration(),
		AvgDuration:        m.analytics.avgDuration(),
		FastestDuration:    m.analytics.fastestDuration(),
		SlowestDuration:    m.analytics.slowestDuration(),
		EstimatedRemaining: m.analytics.estimatedRemaining(m.iteration, m.maxIter),
		CurrentIterStart:   m.startTime,
		InitialReady:       m.analytics.initialReady,
		CurrentReady:       m.analytics.currentReady,
		TasksClosed:        m.analytics.tasksClosed,
		LastTask:           m.analytics.lastTask(),
		IterationHistory:   history,
	}
}

func (m model) renderHelpBar() string {
	var keys []string

	// Common keys
	keys = append(keys, helpKeyStyle.Render("↑/↓")+helpDescStyle.Render(":scroll"))
	keys = append(keys, helpKeyStyle.Render("Tab")+helpDescStyle.Render(":next screen"))
	keys = append(keys, helpKeyStyle.Render("1/2/3")+helpDescStyle.Render(":switch"))

	// Screen-specific keys
	if m.activeScreen == screenOutput {
		followStatus := "on"
		if !m.followOutput {
			followStatus = "off"
		}
		keys = append(keys, helpKeyStyle.Render("f")+helpDescStyle.Render(fmt.Sprintf(":follow(%s)", followStatus)))

		rawStatus := "parsed"
		if m.showRawOutput {
			rawStatus = "raw"
		}
		keys = append(keys, helpKeyStyle.Render("r")+helpDescStyle.Render(fmt.Sprintf(":view(%s)", rawStatus)))
	}

	return helpBarStyle.Width(m.width).Render(strings.Join(keys, "  "))
}
