package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	maxIterations   = flag.Int("max-iterations", 50, "Maximum iterations to run")
	sleepSeconds    = flag.Int("sleep-seconds", 2, "Seconds to sleep between iterations")
	claudeBin       = flag.String("claude-bin", "claude", "Path to claude CLI")
	epicFilter      = flag.String("epic", "", "Filter to tasks within a specific epic (e.g., BD-42)")
	maxReviewCycles = flag.Int("max-review-cycles", 3, "Maximum reviewer/fixer cycles per iteration")
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

// getGitDiff returns the diff of the most recent commit (HEAD~1..HEAD).
// Used by the reviewer phase to see what dev/fixer actually changed.
func getGitDiff(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD~1..HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// detectSpecialist maps file extensions in a git diff to a reviewer persona.
// Multiple file types produce a combined persona.
func detectSpecialist(diff string) string {
	var personas []string
	if regexp.MustCompile(`(?m)^diff --git a/.*\.go `).MatchString(diff) {
		personas = append(personas, "senior Go engineer, idiomatic Go, concurrency, this codebase")
	}
	if regexp.MustCompile(`(?m)^diff --git a/.*\.tsx? `).MatchString(diff) {
		personas = append(personas, "senior TypeScript/React engineer")
	}
	if regexp.MustCompile(`(?m)^diff --git a/.*\.py `).MatchString(diff) {
		personas = append(personas, "senior Python engineer")
	}
	if regexp.MustCompile(`(?m)^diff --git a/.*\.tf `).MatchString(diff) {
		personas = append(personas, "senior Terraform/infrastructure engineer")
	}
	if len(personas) == 0 {
		return "senior software engineer"
	}
	return strings.Join(personas, " and ")
}

// buildDevPrompt produces the prompt for the developer phase.
// The developer finds the next ready task, implements it, runs tests, commits if passing.
// plannerOutput is injected when a preceding planner phase has provided an implementation plan.
func buildDevPrompt(epic, plannerOutput string) string {
	// Build the bd ready command with optional epic filter
	bdReadyCmd := "bd ready --json"
	if epic != "" {
		bdReadyCmd = fmt.Sprintf("bd ready --parent %s --json", epic)
	}

	epicNote := ""
	if epic != "" {
		epicNote = fmt.Sprintf("\n\n**IMPORTANT**: You are scoped to epic %s. Only work on tasks within this epic.", epic)
	}

	base := fmt.Sprintf(`You are Ralph, an autonomous coding agent working in a codebase that uses **Beads** (steveyegge/beads) as its issue tracker and memory system.

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
	if plannerOutput != "" {
		base += "\n\nHere is your implementation plan:\n" + plannerOutput
	}
	return base
}

// buildPlannerPrompt produces the prompt for the planner phase.
// The planner finds the next ready task, analyzes it, and outputs a structured plan.
// It does NOT write any code — analysis only.
func buildPlannerPrompt(epic string) string {
	bdReadyCmd := "bd ready --json"
	if epic != "" {
		bdReadyCmd = fmt.Sprintf("bd ready --parent %s --json", epic)
	}

	epicNote := ""
	if epic != "" {
		epicNote = fmt.Sprintf("\n\n**IMPORTANT**: You are scoped to epic %s. Only work on tasks within this epic.", epic)
	}

	return fmt.Sprintf(`You are the Planner phase of an AI pipeline. Your job is ANALYSIS ONLY — do not write any code or make any commits.%s

Your task:

1. Run: %s
   Pick the highest-priority READY task (P0 > P1 > P2 > P3 > P4). Call it T.

2. Run: bd show <T> --json
   Read the full task description, acceptance criteria, and any notes.

3. Produce a structured implementation plan covering:
   - Approach: how you would implement this
   - Files to touch: which files need creating or modifying
   - Edge cases: what could go wrong
   - Test strategy: what tests to write or run

Output your response in this exact format:

[Planner output]
task: <T>
description: <one-line summary of the task>
plan:
<your full structured plan here>`, epicNote, bdReadyCmd)
}

// buildReviewerPrompt produces the prompt for the reviewer phase.
// The reviewer checks the diff as a specialist and returns APPROVED or CHANGES_REQUESTED.
func buildReviewerPrompt(plannerOutput, diff, specialist string) string {
	return fmt.Sprintf(`You are a code reviewer acting as: %s

You are reviewing work done by an AI coding agent. Here is the context for the task that was implemented:

%s

Here is the git diff of the changes made:

%s

Review the diff carefully against the task requirements. Check for:
- Correctness: does it solve the task as described?
- Code quality: idiomatic style, naming, structure
- Tests: are appropriate tests included or run?
- Edge cases: are obvious failure modes handled?

Output your review in this exact format:

[Reviewer status]
verdict: APPROVED|CHANGES_REQUESTED
specialist: %s
issues:
- <issue 1, or "none" if approved>
notes: <1-2 sentence summary>`, specialist, plannerOutput, diff, specialist)
}

// buildFixerPrompt produces the prompt for the fixer phase.
// The fixer receives the original plan and reviewer feedback, then fixes the issues and re-commits.
func buildFixerPrompt(epic, plannerOutput, reviewerFeedback string) string {
	bdReadyCmd := "bd ready --json"
	if epic != "" {
		bdReadyCmd = fmt.Sprintf("bd ready --parent %s --json", epic)
	}

	return fmt.Sprintf(`You are Ralph, an autonomous coding agent. You are in the FIXER phase.

A reviewer has found issues with your previous implementation. Your job is to fix them and re-commit.

Here is the original implementation plan:

%s

A reviewer found these issues, fix them before re-committing:

%s

Instructions:
1. Address every issue listed in the reviewer feedback.
2. Run tests/type checks after making changes.
3. If tests PASS: commit your fixes with message: fix: address reviewer feedback
4. If tests FAIL: do NOT commit. Note the failures clearly.

For operator observability, include at the end of your response:

[Ralph status]
ready_before: <run %s to get count>
ready_after: <run %s again after work>
task: <task ID from the plan above>
tests: <PASSED or FAILED>
notes: <1-2 sentence summary>`, plannerOutput, reviewerFeedback, bdReadyCmd, bdReadyCmd)
}
