package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	sectionHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true)

	hbLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	hbValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true)

	hbPassedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).Bold(true)

	hbFailedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).Bold(true)

	hbContinueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).Bold(true)

	hbDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
)

// HomebaseData holds all data needed to render the homebase screen.
type HomebaseData struct {
	// Current task
	CurrentTaskID    string
	CurrentTaskTitle string
	CurrentPhase     string
	Iteration        int
	MaxIterations    int
	IterationElapsed time.Duration
	Status           string // "idle", "running", "finished", etc.
	LoopDone         bool

	// Session summary
	TasksCompleted int
	TotalTasks     int
	PassedCount    int
	FailedCount    int

	// Live activity
	ActivityLines []string

	// Dependency graph
	GraphOutput string

	// Iteration history (uses IterationRecord from analytics.go — same package)
	Iterations []IterationRecord
}

// RenderHomebase renders the homebase screen with 5 structured sections.
func RenderHomebase(data HomebaseData, vp viewport.Model) string {
	var b strings.Builder

	// Section 1: Current Task
	b.WriteString(renderHBSection("Current", renderCurrentTask(data)))
	b.WriteString("\n")

	// Section 2: Session Summary
	b.WriteString(renderHBSection("Session", renderSessionSummary(data)))
	b.WriteString("\n")

	// Section 3: Live Activity
	if len(data.ActivityLines) > 0 {
		b.WriteString(renderHBSection("Activity", strings.Join(data.ActivityLines, "\n")))
		b.WriteString("\n")
	}

	// Section 4: Dependency Graph
	if data.GraphOutput != "" {
		b.WriteString(renderHBSection("Dependencies", data.GraphOutput))
		b.WriteString("\n")
	}

	// Section 5: Iteration Log
	b.WriteString(renderHBSection("Iterations", renderIterationLog(data)))

	vp.SetContent(b.String())
	return vp.View()
}

func renderHBSection(title, content string) string {
	header := sectionHeaderStyle.Render(fmt.Sprintf("--- %s ", title)) +
		hbDimStyle.Render(strings.Repeat("-", 40))
	return header + "\n" + content
}

func renderCurrentTask(data HomebaseData) string {
	if data.Status == "finished" {
		if data.LoopDone {
			return "  Loop complete -- all work finished"
		}
		return "  Loop complete -- max iterations reached"
	}
	if data.Status == "idle" || data.CurrentTaskID == "" {
		return "  Waiting for next iteration..."
	}

	taskDisplay := data.CurrentTaskID
	if len(taskDisplay) > 10 {
		taskDisplay = taskDisplay[:10]
	}
	title := data.CurrentTaskTitle
	if title != "" {
		if len(title) > 40 {
			title = title[:40]
		}
		taskDisplay += fmt.Sprintf(" %q", title)
	}

	elapsed := data.IterationElapsed.Truncate(time.Second)
	return fmt.Sprintf("  %s %s\n  %s %s (iteration %d/%d, %s elapsed)",
		hbLabelStyle.Render("Task:"), hbValueStyle.Render(taskDisplay),
		hbLabelStyle.Render("Phase:"), hbValueStyle.Render(data.CurrentPhase),
		data.Iteration, data.MaxIterations, elapsed)
}

func renderSessionSummary(data HomebaseData) string {
	total := data.TotalTasks
	if total == 0 {
		return "  No task data yet"
	}
	pct := 0.0
	if total > 0 {
		pct = float64(data.TasksCompleted) / float64(total) * 100
	}
	successRate := 0.0
	totalIter := data.PassedCount + data.FailedCount
	if totalIter > 0 {
		successRate = float64(data.PassedCount) / float64(totalIter) * 100
	}

	return fmt.Sprintf("  Progress: %d/%d tasks completed (%.0f%%)  |  Iterations: %d/%d  |  Success: %.0f%%",
		data.TasksCompleted, total, pct,
		data.Iteration, data.MaxIterations,
		successRate)
}

func renderIterationLog(data HomebaseData) string {
	if len(data.Iterations) == 0 {
		return "  No iterations yet"
	}

	var b strings.Builder
	// Show most recent first
	for i := len(data.Iterations) - 1; i >= 0; i-- {
		r := data.Iterations[i]
		verdict := r.FinalVerdict
		if verdict == "" {
			if r.Passed {
				verdict = "PASSED"
			} else {
				verdict = "FAILED"
			}
		}

		var verdictStyled string
		switch verdict {
		case "APPROVED", "PASSED", "COMPLETE":
			verdictStyled = hbPassedStyle.Render(verdict)
		case "CONTINUE":
			verdictStyled = hbContinueStyle.Render(verdict)
			b.WriteString(fmt.Sprintf("  #%-3d   %s  (developer claimed complete, more work found)\n",
				r.Iteration, verdictStyled))
			continue
		default:
			verdictStyled = hbFailedStyle.Render(verdict)
		}

		taskDisplay := r.TaskTitle
		if taskDisplay == "" {
			taskDisplay = r.TaskID
		}
		if len(taskDisplay) > 30 {
			taskDisplay = taskDisplay[:30]
		}
		if taskDisplay == "" {
			taskDisplay = "-"
		}

		duration := r.Duration.Truncate(time.Second)
		cycles := ""
		if r.ReviewCycles > 0 {
			cycles = fmt.Sprintf("(%d review cycle", r.ReviewCycles)
			if r.ReviewCycles > 1 {
				cycles += "s"
			}
			cycles += ")"
		}

		b.WriteString(fmt.Sprintf("  #%-3d %s  %s  %s  %s\n",
			r.Iteration, taskDisplay, verdictStyled, duration, cycles))
	}

	return b.String()
}
