# Pre-loop Issue Census Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Before the iteration loop starts, gather issue counts (ready, blocked, in-progress, total open) and a dependency graph from `bd`, display them on homebase, and block startup if zero issues are ready.

**Architecture:** Add a new `statusPreflight` phase and `preflightDoneMsg` message. `Init()` fires a preflight command instead of `startIterationMsg`. The preflight command runs five `bd` subcommands sequentially (all scoped by `--parent <epic>` when set), collects results into a struct, and returns them as a single message. The `Update` handler displays the summary, seeds `analytics.initialReady`, and either starts the loop or finishes immediately.

**Tech Stack:** Go, Bubble Tea message passing, `os/exec` for `bd` CLI calls.

---

### Task 1: Add preflight types to model.go

**Files:**
- Modify: `model.go:36-42` (add `statusPreflight` to iterationStatus constants)
- Modify: `model.go:69-85` (add `preflightDoneMsg` message type)

**Step 1: Write the failing test**

Create `preflight_test.go` with a test that the new message type carries the expected fields:

```go
// preflight_test.go
package main

import "testing"

func TestPreflightDoneMsg_Fields(t *testing.T) {
	msg := preflightDoneMsg{
		readyCount:      5,
		blockedCount:    3,
		inProgressCount: 1,
		totalOpenCount:  9,
		graphOutput:     "layer0: BD-1\nlayer1: BD-2",
		err:             nil,
	}
	if msg.readyCount != 5 {
		t.Errorf("readyCount = %d, want 5", msg.readyCount)
	}
	if msg.blockedCount != 3 {
		t.Errorf("blockedCount = %d, want 3", msg.blockedCount)
	}
	if msg.inProgressCount != 1 {
		t.Errorf("inProgressCount = %d, want 1", msg.inProgressCount)
	}
	if msg.totalOpenCount != 9 {
		t.Errorf("totalOpenCount = %d, want 9", msg.totalOpenCount)
	}
	if msg.graphOutput != "layer0: BD-1\nlayer1: BD-2" {
		t.Errorf("graphOutput mismatch")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestPreflightDoneMsg_Fields -v ./...`
Expected: FAIL — `preflightDoneMsg` is not defined.

**Step 3: Add the types to model.go**

In `model.go`, add `statusPreflight` to the iterationStatus constants block (after `statusIdle`):

```go
const (
	statusIdle      iterationStatus = "idle"
	statusPreflight iterationStatus = "preflight"
	statusRunning   iterationStatus = "running"
	// ... rest unchanged
)
```

Add the new message type after the existing message types (~line 85):

```go
// preflightDoneMsg carries issue census results gathered before the loop starts.
type preflightDoneMsg struct {
	readyCount      int
	blockedCount    int
	inProgressCount int
	totalOpenCount  int
	graphOutput     string
	err             error
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestPreflightDoneMsg_Fields -v ./...`
Expected: PASS

**Step 5: Run linter**

Run: `golangci-lint run`
Expected: Clean

**Step 6: Commit**

```bash
git add model.go preflight_test.go
git commit -m "feat: add preflight message type and status constant"
```

---

### Task 2: Add runPreflight command to main.go

**Files:**
- Modify: `main.go` (add `runPreflight` function after `checkBdReady`)
- Test: `preflight_test.go` (add integration test)

**Step 1: Write the failing test**

Add to `preflight_test.go`:

```go
func TestRunPreflight_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test requiring bd CLI")
	}

	ctx := context.Background()
	cmd := runPreflight(ctx, "")
	msg := cmd()

	pfMsg, ok := msg.(preflightDoneMsg)
	if !ok {
		t.Fatalf("expected preflightDoneMsg, got %T", msg)
	}
	// Should not error (bd CLI available in dev env)
	if pfMsg.err != nil {
		t.Errorf("unexpected error: %v", pfMsg.err)
	}
	// Counts should be non-negative
	if pfMsg.readyCount < 0 || pfMsg.blockedCount < 0 || pfMsg.inProgressCount < 0 || pfMsg.totalOpenCount < 0 {
		t.Error("negative counts returned")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestRunPreflight_Integration -v ./...`
Expected: FAIL — `runPreflight` undefined.

**Step 3: Implement runPreflight in main.go**

Add after the `checkBdReady` function (~line 158):

```go
// runPreflight gathers issue census data before the loop starts.
// Runs bd ready, bd blocked, bd list (in_progress), bd list (open), and bd graph.
// All commands are scoped to the given epic if non-empty.
func runPreflight(ctx context.Context, epic string) tea.Cmd {
	return func() tea.Msg {
		parentFlag := []string{}
		if epic != "" {
			parentFlag = []string{"--parent", epic}
		}

		// Helper: run bd with args, return stdout bytes
		runBd := func(args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, "bd", args...)
			return cmd.Output()
		}

		// 1. Ready count
		readyArgs := append([]string{"ready", "--json"}, parentFlag...)
		readyOut, err := runBd(readyArgs...)
		if err != nil {
			return preflightDoneMsg{err: fmt.Errorf("bd ready: %w", err)}
		}
		var readyIssues []json.RawMessage
		readyCount := 0
		if err := json.Unmarshal(readyOut, &readyIssues); err == nil {
			readyCount = len(readyIssues)
		}

		// 2. Blocked count
		blockedArgs := append([]string{"blocked", "--json"}, parentFlag...)
		blockedOut, err := runBd(blockedArgs...)
		blockedCount := 0
		if err == nil {
			var blockedIssues []json.RawMessage
			if err := json.Unmarshal(blockedOut, &blockedIssues); err == nil {
				blockedCount = len(blockedIssues)
			}
		}

		// 3. In-progress count
		ipArgs := append([]string{"list", "--status=in_progress", "--json", "--limit", "0"}, parentFlag...)
		ipOut, err := runBd(ipArgs...)
		inProgressCount := 0
		if err == nil {
			var ipIssues []json.RawMessage
			if err := json.Unmarshal(ipOut, &ipIssues); err == nil {
				inProgressCount = len(ipIssues)
			}
		}

		// 4. Total open count
		openArgs := append([]string{"list", "--status=open", "--json", "--limit", "0"}, parentFlag...)
		openOut, err := runBd(openArgs...)
		totalOpenCount := 0
		if err == nil {
			var openIssues []json.RawMessage
			if err := json.Unmarshal(openOut, &openIssues); err == nil {
				totalOpenCount = len(openIssues)
			}
		}

		// 5. Dependency graph
		var graphArgs []string
		if epic != "" {
			graphArgs = []string{"graph", "--compact", epic}
		} else {
			graphArgs = []string{"graph", "--compact", "--all"}
		}
		graphOut, _ := runBd(graphArgs...)
		graphOutput := string(graphOut)

		return preflightDoneMsg{
			readyCount:      readyCount,
			blockedCount:    blockedCount,
			inProgressCount: inProgressCount,
			totalOpenCount:  totalOpenCount,
			graphOutput:     graphOutput,
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestRunPreflight_Integration -v ./...`
Expected: PASS (or SKIP if `-short`)

**Step 5: Run linter**

Run: `golangci-lint run`
Expected: Clean

**Step 6: Commit**

```bash
git add main.go preflight_test.go
git commit -m "feat: add runPreflight command for pre-loop issue census"
```

---

### Task 3: Wire preflight into Init and Update

**Files:**
- Modify: `update.go:12-20` (change `Init()` to fire preflight instead of startIteration)
- Modify: `update.go:38-391` (add `preflightDoneMsg` case to `Update`)
- Modify: `model.go:143-174` (set initial status to `statusPreflight`)

**Step 1: Write the failing test**

Add to `preflight_test.go`:

```go
func TestPreflightDoneMsg_ZeroReady_SetsFinished(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{
		readyCount:      0,
		blockedCount:    2,
		inProgressCount: 0,
		totalOpenCount:  2,
		graphOutput:     "test graph",
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.status != statusFinished {
		t.Errorf("status = %q, want %q", um.status, statusFinished)
	}
	if um.loopDone != true {
		t.Error("loopDone should be true when zero ready")
	}
}

func TestPreflightDoneMsg_WithReady_StartsLoop(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{
		readyCount:      3,
		blockedCount:    1,
		inProgressCount: 1,
		totalOpenCount:  5,
		graphOutput:     "test graph",
	}

	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.status == statusFinished {
		t.Error("status should not be finished when ready > 0")
	}
	if um.analytics.initialReady != 3 {
		t.Errorf("initialReady = %d, want 3", um.analytics.initialReady)
	}
	if cmd == nil {
		t.Error("expected a command to start the loop")
	}
}

func TestPreflightDoneMsg_SeedsInitialReady(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{
		readyCount:      7,
		blockedCount:    0,
		inProgressCount: 0,
		totalOpenCount:  7,
		graphOutput:     "",
	}

	updated, _ := m.Update(msg)
	um := updated.(model)

	if um.analytics.initialReady != 7 {
		t.Errorf("initialReady = %d, want 7", um.analytics.initialReady)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -run TestPreflightDoneMsg_ -v ./...`
Expected: FAIL — `Update` doesn't handle `preflightDoneMsg`.

**Step 3: Update model.go — set initial status to preflight**

In `initialModel()` at `model.go:153-154`, change:

```go
// Before:
status:     statusIdle,
statusText: "Idle",

// After:
status:     statusPreflight,
statusText: "Gathering issue census...",
```

**Step 4: Update Init() in update.go**

Change `Init()` to fire preflight instead of immediately starting the loop:

```go
func (m model) Init() tea.Cmd {
	_ = m.reporter.SessionStarted(SessionConfig{
		MaxIterations:   m.maxIter,
		SleepSeconds:    int(m.sleep.Seconds()),
		Epic:            m.epic,
		MaxReviewCycles: m.maxReviewCycles,
	})
	return tea.Batch(runPreflight(m.ctx, m.epic), tick())
}
```

**Step 5: Add preflightDoneMsg handler to Update()**

In `update.go`, add a new case in the `switch msg := msg.(type)` block, before the `startIterationMsg` case (~line 98):

```go
	case preflightDoneMsg:
		if msg.err != nil {
			m.appendHomebase(fmt.Sprintf("Preflight error: %v", msg.err))
			m.appendHomebase("Starting loop anyway...")
			m.status = statusIdle
			m.statusText = "Preflight failed, starting loop"
			return m, startNextIteration()
		}

		// Seed analytics
		m.analytics.initialReady = msg.readyCount
		m.analytics.currentReady = msg.readyCount

		// Display work summary
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

		// Block start if nothing is ready
		if msg.readyCount == 0 {
			m.loopDone = true
			m.status = statusFinished
			m.statusText = "No ready work available"
			m.appendHomebase("No ready issues found. Nothing to do.")
			return m, ringBell()
		}

		m.appendHomebase(fmt.Sprintf("Starting loop (max %d iterations)...\n", m.maxIter))
		return m, startNextIteration()
```

**Step 6: Run tests to verify they pass**

Run: `go test -run TestPreflightDoneMsg_ -v ./...`
Expected: PASS

**Step 7: Run full test suite**

Run: `go test -short ./...`
Expected: All pass

**Step 8: Run linter**

Run: `golangci-lint run`
Expected: Clean

**Step 9: Commit**

```bash
git add model.go update.go preflight_test.go
git commit -m "feat: wire preflight census into Init and Update loop"
```

---

### Task 4: Add statusPreflight to view rendering

**Files:**
- Modify: `view.go:79-126` (add `statusPreflight` styling to `renderStatusBar`)

**Step 1: Check if statusPreflight needs a style**

Read `styles.go` to find the existing status styles.

**Step 2: Add preflight status rendering**

In `view.go:renderStatusBar()`, the `switch m.status` block (~line 95-104) needs a case for `statusPreflight`:

```go
	case statusPreflight:
		statusDisplay = statusRunningStyle.Render(m.statusText)
```

This reuses the running style (yellow/animated) since preflight is an active operation. No new style constant needed.

**Step 3: Run full test suite**

Run: `go test -short ./...`
Expected: All pass

**Step 4: Run linter**

Run: `golangci-lint run`
Expected: Clean

**Step 5: Commit**

```bash
git add view.go
git commit -m "feat: render preflight status in status bar"
```

---

### Task 5: Final integration test and cleanup

**Files:**
- Test: `preflight_test.go` (add error-path test)

**Step 1: Add error-path test**

```go
func TestPreflightDoneMsg_Error_StartsAnyway(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{
		err: fmt.Errorf("bd not found"),
	}

	updated, cmd := m.Update(msg)
	um := updated.(model)

	if um.status == statusFinished {
		t.Error("should not finish on preflight error, should start loop")
	}
	if cmd == nil {
		t.Error("expected a command to start the loop despite error")
	}
}
```

**Step 2: Run full test suite**

Run: `go test -short -v ./...`
Expected: All pass

**Step 3: Run linter**

Run: `golangci-lint run`
Expected: Clean

**Step 4: Build and smoke test**

Run: `go build -o ralph-loop-go`
Expected: Builds successfully

**Step 5: Commit**

```bash
git add preflight_test.go
git commit -m "test: add preflight error-path test"
```
