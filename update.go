package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const maxConsecutiveErrors = 3

// Init starts first iteration and a periodic tick
func (m model) Init() tea.Cmd {
	m.sendEvent(EventSessionStarted, map[string]any{
		"max_iterations":    m.maxIter,
		"sleep_seconds":     int(m.sleep.Seconds()),
		"epic":              m.epic,
		"max_review_cycles": m.maxReviewCycles,
	})
	return tea.Batch(runPreflight(m.ctx, m.epic), tick())
}

func startNextIteration() tea.Cmd {
	return func() tea.Msg { return startIterationMsg{} }
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ringBell sends terminal bell character to notify user
func ringBell() tea.Cmd {
	return tea.Printf("\a")
}

// Update handles all messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate viewport height (total - tab bar - status bar - help bar)
		vpHeight := msg.Height - 4
		if vpHeight < 1 {
			vpHeight = 1
		}

		m.homebaseVP.Width = msg.Width
		m.homebaseVP.Height = vpHeight
		m.outputVP.Width = msg.Width
		m.outputVP.Height = vpHeight - 1 // Account for follow indicator line

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.endSession("interrupted")
			m.cancel()
			return m, tea.Quit

		// Screen switching
		case "1":
			m.activeScreen = screenHomebase
		case "2":
			m.activeScreen = screenOutput
		case "3":
			m.activeScreen = screenAnalytics
		case "tab":
			m.activeScreen = (m.activeScreen + 1) % 3

		// Output screen specific: toggle follow mode
		case "f":
			if m.activeScreen == screenOutput {
				m.followOutput = !m.followOutput
				if m.followOutput {
					m.outputVP.GotoBottom()
				}
			}

		// Output screen specific: toggle raw/parsed output
		case "r":
			if m.activeScreen == screenOutput {
				m.showRawOutput = !m.showRawOutput
				if m.showRawOutput {
					m.outputVP.SetContent(m.rawOutputLog)
				} else {
					m.outputVP.SetContent(m.outputContent)
				}
				if m.followOutput {
					m.outputVP.GotoBottom()
				}
			}
		}

	case preflightDoneMsg:
		if msg.err != nil {
			m.status = statusIdle
			m.statusText = "Preflight failed, starting loop"
			m.appendHomebase(fmt.Sprintf("Preflight error: %v", msg.err))
			m.appendHomebase("Starting loop anyway...")
			return m, startNextIteration()
		}

		m.analytics.initialReady = msg.readyCount
		m.analytics.currentReady = msg.readyCount

		epicLabel := "all work"
		if m.epic != "" {
			epicLabel = fmt.Sprintf("epic %s", m.epic)
		}
		m.appendHomebase(fmt.Sprintf("=== Work Summary (%s) ===", epicLabel))
		m.appendHomebase(fmt.Sprintf("Ready: %d | Blocked: %d | In Progress: %d | Total Open: %d",
			msg.readyCount, msg.blockedCount, msg.inProgressCount, msg.totalOpenCount))

		if msg.graphOutput != "" {
			m.appendHomebase("")
			m.appendHomebase("=== Dependency Graph ===")
			m.appendHomebase(msg.graphOutput)
			m.appendHomebase("")
		}

		if msg.readyCount == 0 {
			m.loopDone = true
			m.status = statusFinished
			m.statusText = "No ready work available"
			m.appendHomebase("No ready issues found. Nothing to do.")
			m.endSession("no_ready_work")
			return m, ringBell()
		}

		m.appendHomebase(fmt.Sprintf("Starting loop (max %d iterations)...\n", m.maxIter))
		return m, startNextIteration()

	case startIterationMsg:
		if m.loopDone || m.iteration >= m.maxIter {
			m.sendEvent(EventPhaseChanged, map[string]any{
				"from": m.currentPhase.String(),
				"to":   "complete",
			})
			m.endSession("finished")
			m.status = statusFinished
			m.statusText = "Finished (max iterations or COMPLETE)"
			return m, ringBell()
		}
		m.iteration++
		m.startTime = time.Now()
		m.endTime = time.Time{}
		m.status = statusRunning
		m.statusText = fmt.Sprintf("Iteration %d • context-gatherer", m.iteration)
		m.lastError = ""
		m.rawOutput = ""

		// Reset phase pipeline state for this iteration
		m.currentPhase = phaseContextGatherer
		m.reviewCycle = 0
		m.gathererOutput = ""
		m.reviewerFeedback = ""

		// Add iteration header to output screens
		m.appendOutput(fmt.Sprintf("--- Iteration %d Output ---", m.iteration))
		if m.rawOutputLog == "" {
			m.rawOutputLog = fmt.Sprintf("--- Iteration %d Raw Output ---", m.iteration)
		} else {
			m.rawOutputLog = m.rawOutputLog + fmt.Sprintf("\n\n--- Iteration %d Raw Output ---", m.iteration)
		}

		m.sendEvent(EventIterationStarted, map[string]any{
			"iteration": m.iteration,
			"phase":     m.currentPhase.String(),
		})

		m.appendHomebase(fmt.Sprintf("\n=== Iteration %d of %d ===", m.iteration, m.maxIter))
		m.appendHomebase(fmt.Sprintf("Start: %s", m.startTime.Format(time.RFC3339)))
		m.appendHomebase("Phase: context-gatherer")

		return m, runClaudeCmd(m.ctx, m.claudePath, buildContextGathererPrompt(m.epic, m.instanceID))

	case claudeOutputLineMsg:
		// Handle streaming output - parse and display each line as it arrives
		line := msg.line

		// Append to raw log
		m.rawOutputLog = m.rawOutputLog + "\n" + line

		// Parse single line and display if it's meaningful
		parsed := ParseStreamLine(line)
		if parsed != nil {
			// Add to parsed output content (but don't update viewport yet)
			if m.outputContent == "" {
				m.outputContent = parsed.Summary
			} else {
				m.outputContent = m.outputContent + "\n" + parsed.Summary
			}

			// Show key events on homebase (tool calls, results, text)
			switch parsed.Type {
			case "tool_call", "result":
				m.appendHomebase("  " + parsed.Summary)
			case "text":
				// Only show short text on homebase
				if len(parsed.Summary) < 100 {
					m.appendHomebase("  " + parsed.Summary)
				}
			}
		}

		// Update viewport based on current mode
		if m.showRawOutput {
			m.outputVP.SetContent(m.rawOutputLog)
		} else {
			m.outputVP.SetContent(m.outputContent)
		}
		if m.followOutput {
			m.outputVP.GotoBottom()
		}

	case claudeDoneMsg:
		m.rawOutput += msg.output

		// Output is already displayed via streaming claudeOutputLineMsg messages
		// Here we just handle completion: analytics, status parsing, next iteration

		if msg.err != nil {
			m.endTime = time.Now()
			m.lastError = msg.err.Error()
			m.consecutiveErrors++
			m.appendHomebase(fmt.Sprintf("Error: %v", msg.err))

			// Record failed iteration
			elapsed := m.endTime.Sub(m.startTime)
			m.analytics.addIteration(m.iteration, elapsed, false, "", msg.err.Error(), "ERROR", 0)
			m.sendEvent(EventIterationCompleted, map[string]any{
				"iteration":     m.iteration,
				"duration_ms":   elapsed.Milliseconds(),
				"passed":        false,
				"notes":         msg.err.Error(),
				"final_verdict": "ERROR",
			})

			// Don't retry if context is cancelled (user quit)
			if m.ctx.Err() != nil {
				m.status = statusError
				m.statusText = "Error running Claude"
				return m, nil
			}

			// Retry if under the consecutive error limit
			if m.consecutiveErrors < maxConsecutiveErrors {
				m.status = statusError
				m.statusText = fmt.Sprintf("Error running Claude (retry %d/%d)", m.consecutiveErrors, maxConsecutiveErrors)
				m.appendHomebase(fmt.Sprintf("  Transient error, retrying (%d/%d)...", m.consecutiveErrors, maxConsecutiveErrors))
				return m, tea.Tick(m.sleep, func(time.Time) tea.Msg {
					return startIterationMsg{}
				})
			}

			// Retries exhausted — stop the loop
			m.status = statusFinished
			m.statusText = "Stopped (repeated Claude errors)"
			m.appendHomebase(fmt.Sprintf("  %d consecutive errors — stopping loop", m.consecutiveErrors))
			m.sendEvent(EventPhaseChanged, map[string]any{
				"from": m.currentPhase.String(),
				"to":   "error",
			})
			m.endSession("error")
			return m, ringBell()
		}

		m.consecutiveErrors = 0

		switch m.currentPhase {
		case phaseContextGatherer:
			m.gathererOutput = ExtractFullText(msg.output)
			m.currentPhase = phaseDev
			m.sendEvent(EventPhaseChanged, map[string]any{
				"from": "context-gatherer",
				"to":   "dev",
			})
			m.statusText = fmt.Sprintf("Iteration %d • dev", m.iteration)
			m.appendHomebase("Phase: dev")
			return m, runClaudeCmd(m.ctx, m.claudePath, buildDevPrompt(m.epic, m.gathererOutput))

		case phaseDev:
			if strings.Contains(msg.output, "<promise>COMPLETE</promise>") {
				// Don't trust Ralph's COMPLETE blindly — closing a task may unblock
				// dependent tasks. Verify by running bd ready ourselves.
				m.statusText = fmt.Sprintf("Iteration %d • verifying COMPLETE", m.iteration)
				m.appendHomebase("Ralph reported COMPLETE — verifying with bd ready...")
				return m, checkBdReady(m.ctx, m.epic)
			}
			diff, _ := getGitDiff(m.ctx)
			m.currentPhase = phaseReviewer
			m.sendEvent(EventPhaseChanged, map[string]any{
				"from": "dev",
				"to":   "reviewer",
			})
			m.reviewCycle = 1
			specialist := detectSpecialist(diff)
			m.statusText = fmt.Sprintf("Iteration %d • reviewer (%d/%d)", m.iteration, m.reviewCycle, m.maxReviewCycles)
			m.appendHomebase(fmt.Sprintf("Phase: reviewer (cycle %d/%d)", m.reviewCycle, m.maxReviewCycles))
			return m, runClaudeCmd(m.ctx, m.claudePath, buildReviewerPrompt(m.gathererOutput, diff, specialist))

		case phaseReviewer:
			reviewerStatus := parseReviewerStatus(msg.output)
			approved := reviewerStatus.Verdict == "APPROVED"
			gaveUp := m.reviewCycle >= m.maxReviewCycles

			if approved || gaveUp {
				finalVerdict := "APPROVED"
				if !approved {
					finalVerdict = "GAVE_UP"
				}

				m.endTime = time.Now()
				m.status = statusCompleted
				m.statusText = fmt.Sprintf("Iteration %d complete (%s)", m.iteration, finalVerdict)

				// Parse Ralph status from accumulated output for analytics
				ralphStatus := parseRalphStatus(m.rawOutput)
				taskID := ""
				passed := approved
				notes := reviewerStatus.Notes

				if ralphStatus != nil {
					taskID = ralphStatus.Task
					if m.analytics.initialReady == 0 && ralphStatus.ReadyBefore > 0 {
						m.analytics.initialReady = ralphStatus.ReadyBefore
					}
					m.analytics.currentReady = ralphStatus.ReadyAfter
				}

				elapsed := m.endTime.Sub(m.startTime)
				m.analytics.addIteration(m.iteration, elapsed, passed, taskID, notes, finalVerdict, m.reviewCycle)
				m.sendEvent(EventIterationCompleted, map[string]any{
					"iteration":     m.iteration,
					"duration_ms":   elapsed.Milliseconds(),
					"task_id":       taskID,
					"passed":        passed,
					"notes":         notes,
					"final_verdict": finalVerdict,
					"review_cycles": m.reviewCycle,
				})

				m.appendHomebase(fmt.Sprintf("Iteration %d complete. Duration: %s | Verdict: %s | Cycles: %d",
					m.iteration, elapsed.Truncate(time.Second), finalVerdict, m.reviewCycle))

				if strings.Contains(m.rawOutput, "<promise>COMPLETE</promise>") {
					// Verify COMPLETE — closing may have unblocked new work
					m.statusText = fmt.Sprintf("Iteration %d • verifying COMPLETE", m.iteration)
					m.appendHomebase("Ralph reported COMPLETE — verifying with bd ready...")
					return m, checkBdReady(m.ctx, m.epic)
				}

				return m, tea.Tick(m.sleep, func(time.Time) tea.Msg {
					return startIterationMsg{}
				})
			}

			// Changes requested — move to fixer
			m.reviewerFeedback = ExtractFullText(msg.output)
			m.reviewCycle++
			m.currentPhase = phaseFixer
			m.sendEvent(EventPhaseChanged, map[string]any{
				"from": "reviewer",
				"to":   "fixer",
			})
			m.statusText = fmt.Sprintf("Iteration %d • fixer", m.iteration)
			m.appendHomebase(fmt.Sprintf("Phase: fixer (reviewer cycle %d/%d requested changes)", m.reviewCycle-1, m.maxReviewCycles))
			return m, runClaudeCmd(m.ctx, m.claudePath, buildFixerPrompt(m.epic, m.gathererOutput, m.reviewerFeedback))

		case phaseFixer:
			diff, _ := getGitDiff(m.ctx)
			m.currentPhase = phaseReviewer
			m.sendEvent(EventPhaseChanged, map[string]any{
				"from": "fixer",
				"to":   "reviewer",
			})
			specialist := detectSpecialist(diff)
			m.statusText = fmt.Sprintf("Iteration %d • reviewer (%d/%d)", m.iteration, m.reviewCycle, m.maxReviewCycles)
			m.appendHomebase(fmt.Sprintf("Phase: reviewer (cycle %d/%d)", m.reviewCycle, m.maxReviewCycles))
			return m, runClaudeCmd(m.ctx, m.claudePath, buildReviewerPrompt(m.gathererOutput, diff, specialist))

		default:
			m.status = statusFinished
			m.statusText = "Unknown phase"
			m.lastError = fmt.Sprintf("unexpected phase: %d", m.currentPhase)
			m.appendHomebase(fmt.Sprintf("Error: unexpected phase %d", m.currentPhase))
			m.endSession("error")
			return m, nil
		}

	case bdReadyCheckMsg:
		if msg.err != nil {
			m.appendHomebase(fmt.Sprintf("  bd ready check failed: %v — treating as COMPLETE", msg.err))
		}

		if msg.err == nil && msg.readyCount > 0 {
			// COMPLETE was premature — there's still ready work
			m.appendHomebase(fmt.Sprintf("  Found %d ready task(s) after close — COMPLETE was premature, continuing loop", msg.readyCount))
			m.status = statusCompleted
			m.statusText = fmt.Sprintf("Iteration %d complete (COMPLETE overridden)", m.iteration)

			// Record iteration analytics
			m.endTime = time.Now()
			elapsed := m.endTime.Sub(m.startTime)
			ralphStatus := parseRalphStatus(m.rawOutput)
			taskID := ""
			if ralphStatus != nil {
				taskID = ralphStatus.Task
			}
			m.analytics.addIteration(m.iteration, elapsed, true, taskID, "COMPLETE overridden — ready work remains", "OVERRIDE", 0)
			m.sendEvent(EventIterationCompleted, map[string]any{
				"iteration":     m.iteration,
				"duration_ms":   elapsed.Milliseconds(),
				"task_id":       taskID,
				"passed":        true,
				"notes":         "COMPLETE overridden — ready work remains",
				"final_verdict": "OVERRIDE",
			})

			return m, tea.Tick(m.sleep, func(time.Time) tea.Msg {
				return startIterationMsg{}
			})
		}

		// Verified: no ready work remains — clean up cache file
		_ = os.Remove(ralphContextCachePath(m.instanceID))
		m.loopDone = true
		m.status = statusFinished
		m.statusText = "Ralph reported COMPLETE (verified)"
		m.endTime = time.Now()
		elapsed := m.endTime.Sub(m.startTime)

		ralphStatus := parseRalphStatus(m.rawOutput)
		taskID := ""
		if ralphStatus != nil {
			taskID = ralphStatus.Task
		}
		m.analytics.addIteration(m.iteration, elapsed, true, taskID, "No ready work remaining (verified)", "COMPLETE", 0)
		m.sendEvent(EventIterationCompleted, map[string]any{
			"iteration":     m.iteration,
			"duration_ms":   elapsed.Milliseconds(),
			"task_id":       taskID,
			"passed":        true,
			"notes":         "No ready work remaining (verified)",
			"final_verdict": "COMPLETE",
		})
		m.sendEvent(EventPhaseChanged, map[string]any{
			"from": m.currentPhase.String(),
			"to":   "complete",
		})
		m.endSession("complete")

		m.appendHomebase("  Verified: no ready work remains. Loop finished.")
		return m, ringBell()

	case tickMsg:
		// Only continue ticking if the loop is still running
		if m.status != statusFinished {
			cmds = append(cmds, tick())
		}
	}

	// Update the active viewport
	switch m.activeScreen {
	case screenHomebase:
		var vpCmd tea.Cmd
		m.homebaseVP, vpCmd = m.homebaseVP.Update(msg)
		cmds = append(cmds, vpCmd)
	case screenOutput:
		var vpCmd tea.Cmd
		m.outputVP, vpCmd = m.outputVP.Update(msg)
		cmds = append(cmds, vpCmd)
	case screenAnalytics:
		// Analytics screen doesn't have a scrollable viewport currently
	}

	return m, tea.Batch(cmds...)
}
