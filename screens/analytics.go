package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles for analytics screen
var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("241")).
			Padding(0, 1).
			MarginRight(1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("62"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(18)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	passedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)

	continueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	failedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("62")).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("241"))

	tableRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

// AnalyticsData holds the data needed for rendering
type AnalyticsData struct {
	// Progress
	CurrentIteration int
	MaxIterations    int
	PassedCount      int
	FailedCount      int

	// Timing
	SessionStart       time.Time
	TotalDuration      time.Duration
	AvgDuration        time.Duration
	FastestDuration    time.Duration
	SlowestDuration    time.Duration
	EstimatedRemaining time.Duration
	CurrentIterStart   time.Time // For "Current" timing

	// Task tracking
	InitialReady     int
	CurrentReady     int
	TasksClosed      int
	LastTask         string
	TotalTasks       int
	BlockedCount     int
	CurrentTaskTitle string

	// Hub reporting
	HubURL        string
	HubInstanceID string

	// History
	IterationHistory []IterationRecord
}

// IterationRecord represents a single iteration for display
type IterationRecord struct {
	Iteration    int
	Duration     time.Duration
	Passed       bool
	TaskID       string
	TaskTitle    string
	Notes        string
	ReviewCycles int
	FinalVerdict string
}

// RenderAnalytics renders the analytics dashboard
func RenderAnalytics(data AnalyticsData, width, height int) string {
	// Calculate panel widths
	panelWidth := (width - 4) / 2
	if panelWidth < 30 {
		panelWidth = 30
	}

	// Build progress panel
	progressContent := renderProgressPanel(data)
	progressPanel := panelStyle.Width(panelWidth).Render(progressContent)

	// Build timing panel
	timingContent := renderTimingPanel(data)
	timingPanel := panelStyle.Width(panelWidth).Render(timingContent)

	// Build task tracking panel
	taskContent := renderTaskPanel(data)
	taskPanel := panelStyle.Width(panelWidth).Render(taskContent)

	// Build iteration history panel
	historyContent := renderHistoryPanel(data, panelWidth)
	historyPanel := panelStyle.Width(panelWidth).Render(historyContent)

	// Layout: two columns
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, progressPanel, timingPanel)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, taskPanel, historyPanel)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)
}

func renderProgressPanel(data AnalyticsData) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Progress") + "\n\n")

	// Iterations
	b.WriteString(labelStyle.Render("Iterations:"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d / %d", data.CurrentIteration, data.MaxIterations)))
	b.WriteString("\n")

	// Passed
	b.WriteString(labelStyle.Render("Passed:"))
	b.WriteString(passedStyle.Render(fmt.Sprintf("%d", data.PassedCount)))
	b.WriteString("\n")

	// Failed
	b.WriteString(labelStyle.Render("Failed:"))
	b.WriteString(failedStyle.Render(fmt.Sprintf("%d", data.FailedCount)))
	b.WriteString("\n")

	// Success rate
	total := data.PassedCount + data.FailedCount
	rate := 0.0
	if total > 0 {
		rate = float64(data.PassedCount) / float64(total) * 100
	}
	b.WriteString(labelStyle.Render("Success Rate:"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.1f%%", rate)))

	return b.String()
}

func renderTimingPanel(data AnalyticsData) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Timing") + "\n\n")

	// Total runtime
	runtime := time.Since(data.SessionStart).Truncate(time.Second)
	b.WriteString(labelStyle.Render("Total Runtime:"))
	b.WriteString(valueStyle.Render(runtime.String()))
	b.WriteString("\n")

	// Current iteration time
	currentTime := time.Duration(0)
	if !data.CurrentIterStart.IsZero() {
		currentTime = time.Since(data.CurrentIterStart).Truncate(time.Second)
	}
	b.WriteString(labelStyle.Render("Current:"))
	b.WriteString(valueStyle.Render(currentTime.String()))
	b.WriteString("\n")

	// Average duration
	b.WriteString(labelStyle.Render("Average:"))
	b.WriteString(valueStyle.Render(data.AvgDuration.Truncate(time.Second).String()))
	b.WriteString("\n")

	// Fastest
	b.WriteString(labelStyle.Render("Fastest:"))
	b.WriteString(valueStyle.Render(data.FastestDuration.Truncate(time.Second).String()))
	b.WriteString("\n")

	// Slowest
	b.WriteString(labelStyle.Render("Slowest:"))
	b.WriteString(valueStyle.Render(data.SlowestDuration.Truncate(time.Second).String()))
	b.WriteString("\n")

	// Estimated remaining
	b.WriteString(labelStyle.Render("Est. Remaining:"))
	b.WriteString(valueStyle.Render(data.EstimatedRemaining.Truncate(time.Second).String()))

	return b.String()
}

func renderProgressBar(completed, total, width int) string {
	if total == 0 {
		return ""
	}
	barWidth := width - 8 // room for "  XX%  "
	if barWidth < 10 {
		barWidth = 10
	}
	filled := barWidth * completed / total
	if filled > barWidth {
		filled = barWidth
	}
	pct := float64(completed) / float64(total) * 100

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	return fmt.Sprintf("  %s  %.0f%%", bar, pct)
}

func renderTaskPanel(data AnalyticsData) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Task Tracking") + "\n\n")

	// Completed / Total
	completed := data.TasksClosed
	total := data.TotalTasks
	if total == 0 {
		total = data.InitialReady // fallback if totalTasks not set
	}
	pct := 0.0
	if total > 0 {
		pct = float64(completed) / float64(total) * 100
	}
	b.WriteString(labelStyle.Render("Completed:"))
	b.WriteString(passedStyle.Render(fmt.Sprintf("%d / %d  (%.0f%%)", completed, total, pct)))
	b.WriteString("\n")

	// Progress bar
	b.WriteString(renderProgressBar(completed, total, 30))
	b.WriteString("\n\n")

	// Ready / Blocked
	b.WriteString(labelStyle.Render("Ready:"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", data.CurrentReady)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Blocked:"))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", data.BlockedCount)))
	b.WriteString("\n\n")

	// Current task
	taskDisplay := data.CurrentTaskTitle
	if taskDisplay == "" {
		taskDisplay = data.LastTask
	}
	if taskDisplay == "" {
		taskDisplay = "-"
	}
	b.WriteString(labelStyle.Render("Current Task:"))
	b.WriteString(valueStyle.Render(taskDisplay))
	b.WriteString("\n\n")

	// Hub section (unchanged)
	b.WriteString(titleStyle.Render("Hub") + "\n\n")
	if data.HubURL != "" {
		b.WriteString(labelStyle.Render("Status:"))
		b.WriteString(passedStyle.Render("enabled"))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("URL:"))
		b.WriteString(valueStyle.Render(data.HubURL))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Instance:"))
		b.WriteString(valueStyle.Render(data.HubInstanceID))
	} else {
		b.WriteString(labelStyle.Render("Status:"))
		b.WriteString(failedStyle.Render("disabled"))
	}

	return b.String()
}

func renderHistoryPanel(data AnalyticsData, width int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Recent Iterations") + "\n\n")

	if len(data.IterationHistory) == 0 {
		b.WriteString(valueStyle.Render("No iterations yet"))
		return b.String()
	}

	// Table header
	b.WriteString(tableHeaderStyle.Render(fmt.Sprintf("%-4s %-10s %-10s %-4s %-24s", "#", "Duration", "Verdict", "Cyc", "Task")))
	b.WriteString("\n")

	// Show last 10 iterations
	history := data.IterationHistory
	if len(history) > 10 {
		history = history[len(history)-10:]
	}

	for _, record := range history {
		verdict := record.FinalVerdict
		if verdict == "" {
			if record.Passed {
				verdict = "PASSED"
			} else {
				verdict = "FAILED"
			}
		}
		var verdictStr string
		switch verdict {
		case "APPROVED", "PASSED", "COMPLETE":
			verdictStr = passedStyle.Render(fmt.Sprintf("%-10s", verdict))
		case "CONTINUE":
			verdictStr = continueStyle.Render(fmt.Sprintf("%-10s", verdict))
		default:
			verdictStr = failedStyle.Render(fmt.Sprintf("%-10s", verdict))
		}

		taskDisplay := record.TaskTitle
		if taskDisplay == "" {
			taskDisplay = record.TaskID
		}
		if len(taskDisplay) > 24 {
			taskDisplay = taskDisplay[:24]
		}
		if taskDisplay == "" {
			taskDisplay = "-"
		}

		cycles := fmt.Sprintf("%-4d", record.ReviewCycles)

		row := fmt.Sprintf("%-4d %-10s %s %s %-24s",
			record.Iteration,
			record.Duration.Truncate(time.Second).String(),
			verdictStr,
			cycles,
			taskDisplay,
		)
		b.WriteString(tableRowStyle.Render(row))
		b.WriteString("\n")
	}

	return b.String()
}
