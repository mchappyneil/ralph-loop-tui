package main

import (
	"context"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/google/uuid"
)

// demoScenario defines a synthetic state preset for UI testing
type demoScenario struct {
	name  string
	apply func(*model)
}

// demoScenarios provides 7 presets covering the full lifecycle
var demoScenarios = []demoScenario{
	{"Fresh Start", applyFreshStart},
	{"Early Session", applyEarlySession},
	{"Mid-Session", applyMidSession},
	{"Post-Failure", applyPostFailure},
	{"Near Completion", applyNearCompletion},
	{"Completed All", applyCompletedAll},
	{"Completed Max", applyCompletedMax},
}

// demoModel creates a model with synthetic data for UI testing
func demoModel() model {
	ctx, cancel := context.WithCancel(context.Background())

	homebaseVP := viewport.New(0, 0)
	outputVP := viewport.New(0, 0)

	m := model{
		iteration:       0,
		maxIter:         50,
		status:          statusIdle,
		statusText:      "Demo mode",
		activeScreen:    screenHomebase,
		homebaseVP:      homebaseVP,
		outputVP:        outputVP,
		homebaseContent: "",
		outputContent:   "",
		followOutput:    true,
		analytics:       newAnalyticsData(),
		sessionStart:    time.Now().Add(-15 * time.Minute),
		sessionID:       uuid.New().String(),
		claudePath:      "claude",
		sleep:           2 * time.Second,
		epic:            "DEMO-1",
		maxReviewCycles: 3,
		reporter:        &noopReporter{},
		ctx:             ctx,
		cancel:          cancel,
		demoMode:        true,
		demoScenarioIdx: 0,
		currentPhase:    phaseContextGatherer,
		repo:            "demo-repo",
		instanceID:      "demo-instance",
	}

	applyDemoScenario(&m, 0)
	return m
}

// applyDemoScenario resets and applies a scenario by index
func applyDemoScenario(m *model, idx int) {
	if idx < 0 || idx >= len(demoScenarios) {
		idx = 0
	}
	m.demoScenarioIdx = idx
	demoScenarios[idx].apply(m)
	m.homebaseVP.SetContent(m.homebaseContent)
	m.outputVP.SetContent(m.outputContent)
	m.homebaseVP.GotoBottom()
	m.outputVP.GotoBottom()
}

// addDemoHistory populates iteration history with synthetic records
func addDemoHistory(m *model, records []iterationRecord) {
	m.analytics.iterationHistory = records
	m.analytics.passedCount = 0
	m.analytics.failedCount = 0
	m.analytics.tasksClosed = 0
	for _, r := range records {
		if r.passed {
			m.analytics.passedCount++
			if r.finalVerdict != "CONTINUE" && r.finalVerdict != "OVERRIDE" {
				m.analytics.tasksClosed++
			}
		} else {
			m.analytics.failedCount++
		}
	}
}

// applyFreshStart: iteration=0, 8 ready, 3 blocked, empty history
func applyFreshStart(m *model) {
	m.iteration = 0
	m.maxIter = 50
	m.status = statusIdle
	m.statusText = "Waiting to start"
	m.currentPhase = phaseContextGatherer
	m.sessionStart = time.Now()
	m.startTime = time.Time{}
	m.endTime = time.Time{}
	m.loopDone = false
	m.epic = "DEMO-1"
	m.currentTaskID = ""
	m.currentTaskTitle = ""
	m.analytics.initialReady = 8
	m.analytics.currentReady = 8
	m.analytics.totalTasks = 11
	m.analytics.blockedCount = 3
	m.analytics.iterationHistory = []iterationRecord{}
	m.homebaseContent = `Ralph loop starting...

Pre-flight checks:
- Ready tasks: 8
- Blocked tasks: 3
- In progress: 0
- Total open: 11

Dependency graph:
  DEMO-1 [epic]
    ├─ DEMO-101 [ready]
    ├─ DEMO-102 [ready]
    ├─ DEMO-103 [blocked by DEMO-101]
    └─ ...

Press Enter to start loop, q to quit.`
	m.outputContent = "No iterations yet. Output will appear here when Ralph starts working."
}

// applyEarlySession: iteration=2/50, 1 APPROVED in history, running context-gatherer
func applyEarlySession(m *model) {
	m.iteration = 2
	m.maxIter = 50
	m.status = statusRunning
	m.statusText = "Running context-gatherer phase"
	m.currentPhase = phaseContextGatherer
	m.sessionStart = time.Now().Add(-8 * time.Minute)
	m.startTime = time.Now().Add(-30 * time.Second)
	m.endTime = time.Time{}
	m.loopDone = false
	m.epic = "DEMO-1"
	m.analytics.initialReady = 8
	m.analytics.currentReady = 7
	m.analytics.totalTasks = 11
	m.analytics.blockedCount = 3
	m.currentTaskID = "beads-bbb2"
	m.currentTaskTitle = "Setup DB migrations"
	addDemoHistory(m, []iterationRecord{
		{iteration: 1, duration: 4*time.Minute + 23*time.Second, passed: true, taskID: "DEMO-101", taskTitle: "Add user auth endpoint", notes: "Implemented user auth endpoint", reviewCycles: 1, finalVerdict: "APPROVED"},
	})
	m.homebaseContent = `[Iteration 1] DEMO-101: Implemented user auth endpoint
  Duration: 4m23s | Review cycles: 1 | Verdict: APPROVED

[Iteration 2] RUNNING (context-gatherer phase)
  Elapsed: 30s | Gathering context for next task...`
	m.outputContent = `[Context Gatherer output]
task: DEMO-102
cache_hit: partial
patterns:
- Use http.Handler pattern for all endpoints
- Auth middleware via context.Context
- Tests use httptest.NewRecorder
- Follow repo style: package-level logger, explicit error returns`
}

// applyMidSession: iteration=8/50, mix of APPROVED(5)/FAILED(1)/GAVE_UP(1)/CONTINUE(1), running dev
func applyMidSession(m *model) {
	m.iteration = 8
	m.maxIter = 50
	m.status = statusRunning
	m.statusText = "Running dev phase"
	m.currentPhase = phaseDev
	m.sessionStart = time.Now().Add(-32 * time.Minute)
	m.startTime = time.Now().Add(-90 * time.Second)
	m.endTime = time.Time{}
	m.loopDone = false
	m.epic = "DEMO-1"
	m.analytics.initialReady = 8
	m.analytics.currentReady = 3
	m.analytics.totalTasks = 12
	m.analytics.blockedCount = 1
	m.currentTaskID = "beads-ggg7"
	m.currentTaskTitle = "Add request logging"
	addDemoHistory(m, []iterationRecord{
		{iteration: 1, duration: 4*time.Minute + 23*time.Second, passed: true, taskID: "DEMO-101", taskTitle: "Add user auth endpoint", notes: "Implemented user auth endpoint", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 2, duration: 3*time.Minute + 45*time.Second, passed: true, taskID: "DEMO-102", taskTitle: "Add session management", notes: "Added session management", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 3, duration: 5*time.Minute + 12*time.Second, passed: true, taskID: "DEMO-103", taskTitle: "DB migration for users", notes: "Database migration for users table", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 4, duration: 2*time.Minute + 58*time.Second, passed: false, taskID: "DEMO-104", taskTitle: "Fix integration timeout", notes: "Tests failed: timeout in integration test", reviewCycles: 3, finalVerdict: "GAVE_UP"},
		{iteration: 5, duration: 4*time.Minute + 38*time.Second, passed: true, taskID: "DEMO-105", taskTitle: "Password reset flow", notes: "Password reset flow", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 6, duration: 0, passed: true, taskID: "", notes: "COMPLETE overridden", finalVerdict: "CONTINUE", reviewCycles: 0},
		{iteration: 7, duration: 6*time.Minute + 15*time.Second, passed: true, taskID: "DEMO-104", taskTitle: "Fix integration timeout", notes: "Fixed timeout issue, increased test deadline", reviewCycles: 2, finalVerdict: "APPROVED"},
	})
	m.homebaseContent = `[Iteration 7] DEMO-106: Email verification endpoint
  Duration: 3m29s | Review cycles: 1 | Verdict: APPROVED

[Iteration 8] RUNNING (dev phase)
  Elapsed: 1m30s | Implementing next task...

Progress: 7 completed, 3 ready, 1 blocked`
	m.outputContent = `[Ralph status]
ready_before: 4
ready_after: 3
task: DEMO-107
tests: RUNNING
notes: Implementing role-based access control, tests in progress`
}

// applyPostFailure: iteration=6/50, last was GAVE_UP with 3 review cycles
func applyPostFailure(m *model) {
	m.iteration = 6
	m.maxIter = 50
	m.status = statusCompleted
	m.statusText = "Iteration complete (GAVE_UP after 3 review cycles)"
	m.currentPhase = phaseFixer
	m.sessionStart = time.Now().Add(-25 * time.Minute)
	m.startTime = time.Now().Add(-7*time.Minute - 45*time.Second)
	m.endTime = time.Now()
	m.loopDone = false
	m.epic = "DEMO-1"
	m.analytics.initialReady = 8
	m.analytics.currentReady = 7
	m.analytics.totalTasks = 11
	m.analytics.blockedCount = 3
	m.currentTaskID = "beads-fff6"
	m.currentTaskTitle = "Fix race condition"
	addDemoHistory(m, []iterationRecord{
		{iteration: 1, duration: 4*time.Minute + 23*time.Second, passed: true, taskID: "DEMO-101", taskTitle: "Add user auth endpoint", notes: "Implemented user auth endpoint", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 2, duration: 3*time.Minute + 45*time.Second, passed: true, taskID: "DEMO-102", taskTitle: "Add session management", notes: "Added session management", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 3, duration: 5*time.Minute + 12*time.Second, passed: true, taskID: "DEMO-103", taskTitle: "DB migration for users", notes: "Database migration for users table", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 4, duration: 2*time.Minute + 58*time.Second, passed: false, taskID: "DEMO-104", taskTitle: "Fix integration timeout", notes: "Tests failed: timeout in integration test", reviewCycles: 3, finalVerdict: "GAVE_UP"},
		{iteration: 5, duration: 4*time.Minute + 38*time.Second, passed: true, taskID: "DEMO-105", taskTitle: "Password reset flow", notes: "Password reset flow", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 6, duration: 7*time.Minute + 45*time.Second, passed: false, taskID: "DEMO-106", taskTitle: "Fix race condition", notes: "Reviewer found race condition, fixer unable to resolve after 3 cycles", reviewCycles: 3, finalVerdict: "GAVE_UP"},
	})
	m.homebaseContent = `[Iteration 5] DEMO-105: Password reset flow
  Duration: 4m38s | Review cycles: 1 | Verdict: APPROVED

[Iteration 6] DEMO-106: Race condition found, GAVE_UP after 3 review cycles
  Duration: 7m45s | Review cycles: 3 | Verdict: GAVE_UP
  Notes: Reviewer found race condition, fixer unable to resolve

Waiting 2s before next iteration...`
	m.outputContent = `[Reviewer status]
verdict: CHANGES_REQUESTED
specialist: senior Go engineer
issues:
- Race condition in session cleanup goroutine
- Missing mutex protection on shared map access
notes: Critical concurrency bug that could cause crashes

[Fixer attempt 3]
I've tried multiple approaches but the race persists. Need human intervention.

[Ralph status]
ready_before: 8
ready_after: 7
task: DEMO-106
tests: FAILED
notes: GAVE_UP after 3 review cycles, needs manual fix`
}

// applyNearCompletion: iteration=15/50, 11 tasks completed, 1 remaining
func applyNearCompletion(m *model) {
	m.iteration = 15
	m.maxIter = 50
	m.status = statusCompleted
	m.statusText = "Iteration complete"
	m.currentPhase = phaseDev
	m.sessionStart = time.Now().Add(-58 * time.Minute)
	m.startTime = time.Now().Add(-3*time.Minute - 22*time.Second)
	m.endTime = time.Now()
	m.loopDone = false
	m.epic = "DEMO-1"
	m.analytics.initialReady = 8
	m.analytics.currentReady = 1
	m.analytics.totalTasks = 12
	m.analytics.blockedCount = 0
	m.currentTaskID = "beads-015"
	m.currentTaskTitle = "Final integration test"
	addDemoHistory(m, []iterationRecord{
		{iteration: 1, duration: 4*time.Minute + 23*time.Second, passed: true, taskID: "DEMO-101", taskTitle: "Add user auth endpoint", notes: "Implemented user auth endpoint", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 2, duration: 3*time.Minute + 45*time.Second, passed: true, taskID: "DEMO-102", taskTitle: "Add session management", notes: "Added session management", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 3, duration: 5*time.Minute + 12*time.Second, passed: true, taskID: "DEMO-103", taskTitle: "DB migration for users", notes: "Database migration for users table", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 4, duration: 2*time.Minute + 58*time.Second, passed: false, taskID: "DEMO-104", taskTitle: "Fix integration timeout", notes: "Tests failed", reviewCycles: 3, finalVerdict: "GAVE_UP"},
		{iteration: 5, duration: 4*time.Minute + 38*time.Second, passed: true, taskID: "DEMO-105", taskTitle: "Password reset flow", notes: "Password reset flow", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 6, duration: 6*time.Minute + 15*time.Second, passed: true, taskID: "DEMO-104", taskTitle: "Fix integration timeout", notes: "Fixed previous failure", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 7, duration: 3*time.Minute + 29*time.Second, passed: true, taskID: "DEMO-106", taskTitle: "Email verification", notes: "Email verification", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 8, duration: 4*time.Minute + 52*time.Second, passed: true, taskID: "DEMO-107", taskTitle: "RBAC implementation", notes: "RBAC implementation", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 9, duration: 3*time.Minute + 18*time.Second, passed: true, taskID: "DEMO-108", taskTitle: "Admin dashboard", notes: "Admin dashboard", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 10, duration: 5*time.Minute + 44*time.Second, passed: true, taskID: "DEMO-109", taskTitle: "Audit logging", notes: "Audit logging", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 11, duration: 2*time.Minute + 38*time.Second, passed: true, taskID: "DEMO-110", taskTitle: "Rate limiting", notes: "Rate limiting", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 12, duration: 4*time.Minute + 7*time.Second, passed: true, taskID: "DEMO-111", taskTitle: "API documentation", notes: "API documentation", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 13, duration: 3*time.Minute + 51*time.Second, passed: false, taskID: "DEMO-112", taskTitle: "Fix flaky test suite", notes: "Flaky integration test", reviewCycles: 3, finalVerdict: "GAVE_UP"},
		{iteration: 14, duration: 5*time.Minute + 29*time.Second, passed: true, taskID: "DEMO-112", taskTitle: "Fix flaky test suite", notes: "Fixed test flakiness with proper mocking", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 15, duration: 3*time.Minute + 22*time.Second, passed: true, taskID: "DEMO-113", taskTitle: "Optimize auth queries", notes: "Performance optimization for auth queries", reviewCycles: 1, finalVerdict: "APPROVED"},
	})
	m.homebaseContent = `[Iteration 14] DEMO-112: Fixed test flakiness
  Duration: 5m29s | Review cycles: 2 | Verdict: APPROVED

[Iteration 15] DEMO-113: Performance optimization
  Duration: 3m22s | Review cycles: 1 | Verdict: APPROVED

Progress: 11/12 tasks completed, 1 ready, 0 blocked
Session time: 58m | Avg: 4m12s/task`
	m.outputContent = `[Ralph status]
ready_before: 2
ready_after: 1
task: DEMO-113
tests: PASSED
notes: Added index on user_sessions.user_id, 80% query speedup

One task remaining in epic DEMO-1.`
}

// applyCompletedAll: iteration=12/50, loopDone=true, all tasks done
func applyCompletedAll(m *model) {
	m.iteration = 12
	m.maxIter = 50
	m.status = statusFinished
	m.statusText = "All tasks complete"
	m.currentPhase = phaseDev
	m.sessionStart = time.Now().Add(-48 * time.Minute)
	m.startTime = time.Now().Add(-3*time.Minute - 44*time.Second)
	m.endTime = time.Now()
	m.loopDone = true
	m.epic = "DEMO-1"
	m.analytics.initialReady = 8
	m.analytics.currentReady = 0
	m.analytics.totalTasks = 12
	m.analytics.blockedCount = 0
	m.currentTaskID = ""
	m.currentTaskTitle = ""
	addDemoHistory(m, []iterationRecord{
		{iteration: 1, duration: 4*time.Minute + 23*time.Second, passed: true, taskID: "DEMO-101", taskTitle: "Add user auth endpoint", notes: "User auth endpoint", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 2, duration: 3*time.Minute + 45*time.Second, passed: true, taskID: "DEMO-102", taskTitle: "Add session management", notes: "Session management", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 3, duration: 5*time.Minute + 12*time.Second, passed: true, taskID: "DEMO-103", taskTitle: "DB migration for users", notes: "Database migration", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 4, duration: 2*time.Minute + 58*time.Second, passed: false, taskID: "DEMO-104", taskTitle: "Fix integration timeout", notes: "Test timeout", reviewCycles: 3, finalVerdict: "GAVE_UP"},
		{iteration: 5, duration: 4*time.Minute + 38*time.Second, passed: true, taskID: "DEMO-105", taskTitle: "Password reset flow", notes: "Password reset", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 6, duration: 6*time.Minute + 15*time.Second, passed: true, taskID: "DEMO-104", taskTitle: "Fix integration timeout", notes: "Fixed timeout", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 7, duration: 3*time.Minute + 29*time.Second, passed: true, taskID: "DEMO-106", taskTitle: "Email verification", notes: "Email verification", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 8, duration: 4*time.Minute + 52*time.Second, passed: true, taskID: "DEMO-107", taskTitle: "RBAC implementation", notes: "RBAC", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 9, duration: 3*time.Minute + 18*time.Second, passed: true, taskID: "DEMO-108", taskTitle: "Admin dashboard", notes: "Admin dashboard", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 10, duration: 5*time.Minute + 44*time.Second, passed: true, taskID: "DEMO-109", taskTitle: "Audit logging", notes: "Audit logging", reviewCycles: 2, finalVerdict: "APPROVED"},
		{iteration: 11, duration: 2*time.Minute + 38*time.Second, passed: true, taskID: "DEMO-110", taskTitle: "Rate limiting", notes: "Rate limiting", reviewCycles: 1, finalVerdict: "APPROVED"},
		{iteration: 12, duration: 3*time.Minute + 44*time.Second, passed: true, taskID: "DEMO-111", taskTitle: "Final cleanup and docs", notes: "Final cleanup and docs", reviewCycles: 1, finalVerdict: "APPROVED"},
	})
	m.homebaseContent = `[Iteration 11] DEMO-110: Rate limiting
  Duration: 2m38s | Review cycles: 1 | Verdict: APPROVED

[Iteration 12] DEMO-111: Final cleanup and docs
  Duration: 3m44s | Review cycles: 1 | Verdict: APPROVED

<promise>COMPLETE</promise>

Session complete! All 8 tasks in epic DEMO-1 finished.
- Total time: 48m
- Tasks completed: 8
- Avg time per task: 4m
- Success rate: 92% (11 passed / 12 iterations)

Press q to exit.`
	m.outputContent = `[Ralph status]
ready_before: 1
ready_after: 0
task: DEMO-111
tests: PASSED
notes: Added comprehensive README and cleaned up unused imports

<promise>COMPLETE</promise>

All tasks in epic DEMO-1 are now closed. No remaining work.

Summary:
- 8 features implemented
- 11 commits
- 1 retry after initial failure
- All tests passing
- Documentation updated`
}

// applyCompletedMax: iteration=50/50, max iter reached, 3 remaining
func applyCompletedMax(m *model) {
	m.iteration = 50
	m.maxIter = 50
	m.status = statusFinished
	m.statusText = "Max iterations reached"
	m.currentPhase = phaseDev
	m.sessionStart = time.Now().Add(-3*time.Hour - 25*time.Minute)
	m.startTime = time.Now().Add(-5*time.Minute - 18*time.Second)
	m.endTime = time.Now()
	m.loopDone = true
	m.epic = "DEMO-1"
	m.analytics.initialReady = 8
	m.analytics.currentReady = 3
	m.analytics.totalTasks = 12
	m.analytics.blockedCount = 3
	m.currentTaskID = ""
	m.currentTaskTitle = ""

	// Build realistic history: 32 passed, 18 failed/gave up
	history := make([]iterationRecord, 50)
	taskNum := 101
	for i := 0; i < 50; i++ {
		passed := true
		verdict := "APPROVED"
		cycles := 1

		// Create realistic failure pattern: some failures, some retries
		if i == 3 || i == 8 || i == 12 || i == 19 || i == 24 || i == 29 || i == 35 || i == 38 || i == 43 {
			passed = false
			verdict = "GAVE_UP"
			cycles = 3
		} else if i == 16 || i == 22 || i == 27 || i == 31 || i == 41 || i == 47 {
			cycles = 2
		}

		duration := time.Duration(2+i%5)*time.Minute + time.Duration(15+i%45)*time.Second
		notes := "Task implementation"
		if !passed {
			notes = "Failed after multiple review cycles"
		}

		history[i] = iterationRecord{
			iteration:    i + 1,
			duration:     duration,
			passed:       passed,
			taskID:       "DEMO-" + strconv.Itoa(taskNum),
			notes:        notes,
			reviewCycles: cycles,
			finalVerdict: verdict,
		}

		if passed || i%2 == 0 {
			taskNum++
		}
	}
	addDemoHistory(m, history)

	m.homebaseContent = `[Iteration 49] DEMO-138: Task implementation
  Duration: 5m23s | Review cycles: 1 | Verdict: APPROVED

[Iteration 50] DEMO-139: Task implementation
  Duration: 5m18s | Review cycles: 1 | Verdict: APPROVED

Max iterations reached (50/50).

Session summary:
- Total time: 3h25m
- Tasks completed: 32
- Failed tasks: 18
- Success rate: 64%
- Remaining ready tasks: 3

Ralph stopped due to iteration limit. Consider:
1. Reviewing failed tasks for patterns
2. Increasing -max-iterations if needed
3. Manually addressing complex failures

Press q to exit.`
	m.outputContent = `[Ralph status]
ready_before: 4
ready_after: 3
task: DEMO-139
tests: PASSED
notes: Implemented task but iteration limit reached

Stopping due to max iterations (50).
Remaining work: 3 ready tasks in epic DEMO-1.

Consider reviewing the 18 failed iterations to identify systematic issues.`
}
