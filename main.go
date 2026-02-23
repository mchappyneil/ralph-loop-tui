package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	maxIterations   = flag.Int("max-iterations", 50, "Maximum iterations to run")
	sleepSeconds    = flag.Int("sleep-seconds", 2, "Seconds to sleep between iterations")
	claudeBin       = flag.String("claude-bin", "claude", "Path to claude CLI")
	epicFilter      = flag.String("epic", "", "Filter to tasks within a specific epic (e.g., BD-42)")
	maxReviewCycles = flag.Int("max-review-cycles", 3, "Maximum reviewer/fixer cycles per iteration (default 3)")
)

// Global program reference for sending messages from goroutines
var programRef *tea.Program

func main() {
	flag.Parse()
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	programRef = p
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

// runClaudeCmd runs Claude and streams output line by line.
// Each line is sent as a claudeOutputLineMsg via programRef.Send().
// When complete, returns claudeDoneMsg with all accumulated output.
func runClaudeCmd(ctx context.Context, claudePath, prompt string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.CommandContext(ctx, claudePath,
			"--dangerously-skip-permissions",
			"--output-format", "stream-json",
			"--verbose",
			"-p", prompt,
		)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return claudeDoneMsg{"", fmt.Errorf("stdout pipe: %w", err)}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return claudeDoneMsg{"", fmt.Errorf("stderr pipe: %w", err)}
		}

		if err := cmd.Start(); err != nil {
			return claudeDoneMsg{"", fmt.Errorf("start claude: %w", err)}
		}

		var buf bytes.Buffer

		// Read stdout and stream each line to the TUI
		done := make(chan struct{})
		go func() {
			defer close(done)
			scanner := bufio.NewScanner(stdout)
			// Increase buffer size for large JSON lines
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				buf.WriteString(line)
				buf.WriteString("\n")
				// Send each line immediately to the TUI for real-time display
				if programRef != nil {
					programRef.Send(claudeOutputLineMsg{line: line})
				}
			}
		}()

		// stderr: print to stderr for debugging
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Fprintf(os.Stderr, "[claude stderr] %s\n", line)
			}
		}()

		<-done
		if err := cmd.Wait(); err != nil {
			return claudeDoneMsg{buf.String(), fmt.Errorf("claude error: %w", err)}
		}

		return claudeDoneMsg{buf.String(), nil}
	}
}

// Prompt: Beads/AGENTS work is done entirely inside Claude.
func buildPrompt(epic string) string {
	// Build the bd ready command with optional epic filter
	bdReadyCmd := "bd ready --json"
	if epic != "" {
		bdReadyCmd = fmt.Sprintf("bd ready --parent %s --json", epic)
	}

	epicNote := ""
	if epic != "" {
		epicNote = fmt.Sprintf("\n\n**IMPORTANT**: You are scoped to epic %s. Only work on tasks within this epic.", epic)
	}

	return fmt.Sprintf(`You are Ralph, an autonomous coding agent working in a codebase that uses **Beads** (steveyegge/beads) as its issue tracker and memory system.

All Beads operations (bd ready, bd show, bd update, bd close, etc.) are your responsibility, inside this Claude invocation. The outer TUI only calls you in a loop.%s

Your job in each iteration:

1. Find the next **doable** item of work with Beads:
   - Run: %s
   - Choose the highest-priority READY task according to Beads (P0 > P1 > P2 > P3 > P4, then epic priority / created time tiebreakers).
   - Call the selected task T.

2. Implement exactly ONE task:
   - Use bd show <T> --json to get full context.
   - Optionally mark as in_progress with bd update <T> --status in_progress --json if that fits the workflow.
   - Modify the codebase to satisfy T, keeping changes tightly scoped and consistent with existing patterns.

3. Run tests/type checks:
   - Run the appropriate tests and checks for this repo.
   - You MUST actually run commands in the shell; never assume success.

4. If tests PASS:
   - Close the task in Beads: bd close <T> --reason "Implemented" --json (or equivalent).
   - Commit your changes with message: feat: [concise task description]
   - Update Beads notes for T and AGENTS.md (if present) with durable, reusable guidance.

5. If tests FAIL:
   - Do NOT close T.
   - Do NOT commit broken code.
   - Update Beads notes on T (or a related tracking issue) describing:
     - What you attempted
     - What failed (error messages, failing tests)
     - What you think needs to change on the next iteration.

6. End condition - CRITICAL:
   - After finishing work on T (either closing it or updating notes on failure):
     - Run %s again to get the current ready count.
     - If there are NO READY issues left:
       - Output exactly: <promise>COMPLETE</promise>
       - Include a brief summary of remaining non-ready work or blockers.
     - If READY work remains:
       - Output your [Ralph status] block.
       - STOP IMMEDIATELY. Do NOT pick up another task. Do NOT continue working.
       - The outer loop will invoke you again for the next task.

   **IMPORTANT**: You must complete exactly ONE task per invocation, then STOP. Even if more work is ready, you must exit so the outer loop can track progress and call you again. Working on multiple tasks in one invocation breaks the monitoring system.

For operator observability, in every iteration you MUST include at the end of your response a short status block like:

[Ralph status]
ready_before: <integer count from bd ready before you picked T>
ready_after: <integer count from bd ready after you finish>
task: <T>
tests: <PASSED or FAILED>
notes: <1-2 sentence summary>`, epicNote, bdReadyCmd, bdReadyCmd)
}
