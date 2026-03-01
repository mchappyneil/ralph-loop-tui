# Reporter Client Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an opt-in HTTP reporter to ralph-loop-go that sends events to a ralph-hub server.

**Architecture:** A `Reporter` interface with two implementations (`httpReporter` and `noopReporter`). The reporter is injected into the Bubble Tea model and called at state transition points in `update.go`. Events are sent fire-and-forget in goroutines — reporting never blocks or crashes the TUI.

**Tech Stack:** Go stdlib `net/http`, `encoding/json`, `os/exec` (for git remote detection)

**Design doc:** `docs/plans/2026-03-01-ralph-hub-design.md`

---

### Task 1: Define Event Types

**Files:**
- Create: `reporter_events.go`
- Test: `reporter_events_test.go`

**Step 1: Write the failing test**

```go
package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventMarshalJSON(t *testing.T) {
	evt := Event{
		ID:         "evt_test123",
		Type:       EventIterationCompleted,
		Timestamp:  time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC),
		InstanceID: "my-app/BD-42",
		Repo:       "my-app",
		Epic:       "BD-42",
		Data: map[string]any{
			"iteration":   7,
			"duration_ms": 45000,
			"task_id":     "BD-45",
			"passed":      true,
		},
		Context: EventContext{
			SessionID:        "sess_xyz",
			SessionStart:     time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC),
			MaxIterations:    50,
			CurrentIteration: 7,
			Status:           "running",
			CurrentPhase:     "dev",
			Analytics: EventAnalytics{
				PassedCount:    6,
				FailedCount:    1,
				TasksClosed:    6,
				InitialReady:   12,
				CurrentReady:   5,
				AvgDurationMs:  42000,
				TotalDurationMs: 294000,
			},
		},
	}

	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded["type"] != "iteration.completed" {
		t.Errorf("type = %v, want iteration.completed", decoded["type"])
	}
	if decoded["instance_id"] != "my-app/BD-42" {
		t.Errorf("instance_id = %v, want my-app/BD-42", decoded["instance_id"])
	}
	ctx := decoded["context"].(map[string]any)
	analytics := ctx["analytics"].(map[string]any)
	if analytics["passed_count"] != float64(6) {
		t.Errorf("passed_count = %v, want 6", analytics["passed_count"])
	}
}

func TestAllEventTypesAreDefined(t *testing.T) {
	expected := []EventType{
		EventSessionStarted,
		EventSessionEnded,
		EventIterationStarted,
		EventIterationCompleted,
		EventPhaseChanged,
		EventTaskClaimed,
		EventTaskClosed,
	}
	for _, et := range expected {
		if et == "" {
			t.Error("found empty event type")
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestEvent -v`
Expected: FAIL — types not defined

**Step 3: Write minimal implementation**

Create `reporter_events.go`:

```go
package main

import "time"

// EventType identifies the kind of event being reported.
type EventType string

const (
	EventSessionStarted     EventType = "session.started"
	EventSessionEnded       EventType = "session.ended"
	EventIterationStarted   EventType = "iteration.started"
	EventIterationCompleted EventType = "iteration.completed"
	EventPhaseChanged       EventType = "phase.changed"
	EventTaskClaimed        EventType = "task.claimed"
	EventTaskClosed         EventType = "task.closed"
)

// Event is the envelope sent to ralph-hub for every reportable occurrence.
type Event struct {
	ID         string         `json:"event_id"`
	Type       EventType      `json:"type"`
	Timestamp  time.Time      `json:"timestamp"`
	InstanceID string         `json:"instance_id"`
	Repo       string         `json:"repo"`
	Epic       string         `json:"epic,omitempty"`
	Data       map[string]any `json:"data"`
	Context    EventContext   `json:"context"`
}

// EventContext is a snapshot of the Ralph instance's current state,
// attached to every event so the dashboard can reconstruct state from
// any single event (handles mid-session connections).
type EventContext struct {
	SessionID        string         `json:"session_id"`
	SessionStart     time.Time      `json:"session_start"`
	MaxIterations    int            `json:"max_iterations"`
	CurrentIteration int            `json:"current_iteration"`
	Status           string         `json:"status"`
	CurrentPhase     string         `json:"current_phase"`
	Analytics        EventAnalytics `json:"analytics"`
}

// EventAnalytics holds the cumulative analytics snapshot.
type EventAnalytics struct {
	PassedCount     int   `json:"passed_count"`
	FailedCount     int   `json:"failed_count"`
	TasksClosed     int   `json:"tasks_closed"`
	InitialReady    int   `json:"initial_ready"`
	CurrentReady    int   `json:"current_ready"`
	AvgDurationMs   int64 `json:"avg_duration_ms"`
	TotalDurationMs int64 `json:"total_duration_ms"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestEvent -v`
Expected: PASS

**Step 5: Lint**

Run: `golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add reporter_events.go reporter_events_test.go
git commit -m "feat: add event types for ralph-hub reporting"
```

---

### Task 2: Define Reporter Interface and noopReporter

**Files:**
- Create: `reporter.go`
- Test: `reporter_test.go`

**Step 1: Write the failing test**

```go
package main

import "testing"

func TestNoopReporterImplementsInterface(t *testing.T) {
	var r Reporter = &noopReporter{}
	if err := r.SessionStarted(SessionConfig{}); err != nil {
		t.Errorf("SessionStarted error: %v", err)
	}
	if err := r.SessionEnded("test"); err != nil {
		t.Errorf("SessionEnded error: %v", err)
	}
	if err := r.IterationStarted(1, "planner"); err != nil {
		t.Errorf("IterationStarted error: %v", err)
	}
	if err := r.IterationCompleted(IterationResult{}); err != nil {
		t.Errorf("IterationCompleted error: %v", err)
	}
	if err := r.PhaseChanged("planner", "dev"); err != nil {
		t.Errorf("PhaseChanged error: %v", err)
	}
	if err := r.TaskClaimed("BD-1", "do stuff"); err != nil {
		t.Errorf("TaskClaimed error: %v", err)
	}
	if err := r.TaskClosed("BD-1", "abc123"); err != nil {
		t.Errorf("TaskClosed error: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestNoopReporter -v`
Expected: FAIL — Reporter, noopReporter not defined

**Step 3: Write minimal implementation**

Create `reporter.go`:

```go
package main

import "time"

// SessionConfig holds the configuration sent with session.started events.
type SessionConfig struct {
	MaxIterations   int    `json:"max_iterations"`
	SleepSeconds    int    `json:"sleep_seconds"`
	Epic            string `json:"epic,omitempty"`
	MaxReviewCycles int    `json:"max_review_cycles"`
}

// IterationResult holds the outcome of a completed iteration.
type IterationResult struct {
	Iteration    int           `json:"iteration"`
	Duration     time.Duration `json:"duration"`
	TaskID       string        `json:"task_id"`
	Passed       bool          `json:"passed"`
	Notes        string        `json:"notes"`
	ReviewCycles int           `json:"review_cycles"`
	FinalVerdict string        `json:"final_verdict"`
}

// Reporter sends events to a ralph-hub server.
// Implementations must be safe to call from goroutines.
type Reporter interface {
	SessionStarted(config SessionConfig) error
	SessionEnded(reason string) error
	IterationStarted(iteration int, phase string) error
	IterationCompleted(result IterationResult) error
	PhaseChanged(from, to string) error
	TaskClaimed(taskID, description string) error
	TaskClosed(taskID, commitHash string) error
}

// noopReporter is used when no hub URL is configured. All methods are no-ops.
type noopReporter struct{}

func (n *noopReporter) SessionStarted(SessionConfig) error               { return nil }
func (n *noopReporter) SessionEnded(string) error                        { return nil }
func (n *noopReporter) IterationStarted(int, string) error               { return nil }
func (n *noopReporter) IterationCompleted(IterationResult) error         { return nil }
func (n *noopReporter) PhaseChanged(string, string) error                { return nil }
func (n *noopReporter) TaskClaimed(string, string) error                 { return nil }
func (n *noopReporter) TaskClosed(string, string) error                  { return nil }
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestNoopReporter -v`
Expected: PASS

**Step 5: Lint**

Run: `golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add reporter.go reporter_test.go
git commit -m "feat: add Reporter interface and noopReporter"
```

---

### Task 3: Instance ID Derivation

**Files:**
- Create: `instance_id.go`
- Test: `instance_id_test.go`

**Step 1: Write the failing test**

```go
package main

import "testing"

func TestDeriveInstanceID_ExplicitOverride(t *testing.T) {
	id := deriveInstanceID("my-custom-id", "")
	if id != "my-custom-id" {
		t.Errorf("got %q, want %q", id, "my-custom-id")
	}
}

func TestDeriveInstanceID_FallbackToDirectory(t *testing.T) {
	// When no explicit ID and repo detection fails, use current dir name
	id := deriveInstanceID("", "")
	if id == "" {
		t.Error("instance ID should not be empty")
	}
}

func TestDeriveInstanceID_WithEpic(t *testing.T) {
	id := deriveInstanceID("my-app", "BD-42")
	if id != "my-app/BD-42" {
		t.Errorf("got %q, want %q", id, "my-app/BD-42")
	}
}

func TestRepoNameFromGitRemote(t *testing.T) {
	tests := []struct {
		remote string
		want   string
	}{
		{"git@github.com:user/my-repo.git", "my-repo"},
		{"https://github.com/user/my-repo.git", "my-repo"},
		{"https://github.com/user/my-repo", "my-repo"},
		{"", ""},
	}
	for _, tt := range tests {
		got := repoNameFromRemoteURL(tt.remote)
		if got != tt.want {
			t.Errorf("repoNameFromRemoteURL(%q) = %q, want %q", tt.remote, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestDeriveInstanceID -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `instance_id.go`:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// deriveInstanceID returns the instance identifier.
// Priority: explicit override > git repo name > directory name.
// If epic is set and the base ID doesn't already contain it, appends /epic.
func deriveInstanceID(explicit, epic string) string {
	base := explicit
	if base == "" {
		base = detectRepoName()
	}
	if base == "" {
		// Last resort: current directory name
		if wd, err := os.Getwd(); err == nil {
			base = filepath.Base(wd)
		}
	}
	if base == "" {
		base = "unknown"
	}
	if epic != "" && !strings.HasSuffix(base, "/"+epic) {
		base = base + "/" + epic
	}
	return base
}

// detectRepoName tries to get the repo name from git remote origin.
func detectRepoName() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return repoNameFromRemoteURL(strings.TrimSpace(string(out)))
}

// repoNameFromRemoteURL extracts the repository name from a git remote URL.
func repoNameFromRemoteURL(remote string) string {
	if remote == "" {
		return ""
	}
	// Handle SSH: git@github.com:user/repo.git
	if idx := strings.LastIndex(remote, ":"); idx != -1 && !strings.Contains(remote, "://") {
		remote = remote[idx+1:]
	}
	// Handle HTTPS: https://github.com/user/repo.git
	if idx := strings.LastIndex(remote, "/"); idx != -1 {
		remote = remote[idx+1:]
	}
	// Strip .git suffix
	remote = strings.TrimSuffix(remote, ".git")
	return remote
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run "TestDeriveInstanceID|TestRepoName" -v`
Expected: PASS

**Step 5: Lint**

Run: `golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add instance_id.go instance_id_test.go
git commit -m "feat: add instance ID derivation from repo/epic"
```

---

### Task 4: HTTP Reporter

**Files:**
- Create: `reporter_http.go`
- Test: `reporter_http_test.go`

**Step 1: Write the failing test**

```go
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPReporter_SendsEvents(t *testing.T) {
	var mu sync.Mutex
	var received []Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or wrong auth header: %s", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var evt Event
		if err := json.Unmarshal(body, &evt); err != nil {
			t.Errorf("unmarshal error: %v", err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(202)
	}))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "BD-1")

	err := r.SessionStarted(SessionConfig{MaxIterations: 50})
	if err != nil {
		t.Fatalf("SessionStarted error: %v", err)
	}

	// Give the goroutine time to send
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("received %d events, want 1", len(received))
	}
	if received[0].Type != EventSessionStarted {
		t.Errorf("type = %s, want %s", received[0].Type, EventSessionStarted)
	}
	if received[0].InstanceID != "test-repo/BD-1" {
		t.Errorf("instance_id = %s, want test-repo/BD-1", received[0].InstanceID)
	}
}

func TestHTTPReporter_DoesNotBlockOnFailure(t *testing.T) {
	// Server that always errors
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "key", "repo", "")

	// Should not block or return error (fire-and-forget)
	start := time.Now()
	err := r.IterationStarted(1, "planner")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("should not return error: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("took %v, should return immediately", elapsed)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestHTTPReporter -v`
Expected: FAIL — newHTTPReporter not defined

**Step 3: Write minimal implementation**

Create `reporter_http.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// httpReporter sends events to a ralph-hub server via HTTP POST.
// All sends are fire-and-forget in goroutines — never blocks the TUI.
type httpReporter struct {
	hubURL     string
	apiKey     string
	instanceID string
	repo       string
	epic       string
	sessionID  string
	client     *http.Client

	// Mutable state for context building (updated by the caller).
	// Access is safe because the TUI is single-threaded (Bubble Tea model).
	sessionStart     time.Time
	maxIterations    int
	currentIteration int
	status           string
	currentPhase     string
	analytics        *analyticsData
}

func newHTTPReporter(hubURL, apiKey, repo, epic string) *httpReporter {
	instanceID := deriveInstanceID(repo, epic)
	return &httpReporter{
		hubURL:     hubURL,
		apiKey:     apiKey,
		instanceID: instanceID,
		repo:       repo,
		epic:       epic,
		sessionID:  "sess_" + uuid.New().String()[:8],
		client:     &http.Client{Timeout: 10 * time.Second},
		analytics:  &analyticsData{},
	}
}

func (h *httpReporter) send(evt Event) {
	go func() {
		body, err := json.Marshal(evt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[reporter] marshal error: %v\n", err)
			return
		}
		req, err := http.NewRequest("POST", h.hubURL+"/api/v1/events", bytes.NewReader(body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "[reporter] request error: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+h.apiKey)

		resp, err := h.client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[reporter] send error: %v\n", err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "[reporter] hub returned %d\n", resp.StatusCode)
		}
	}()
}

func (h *httpReporter) buildEvent(eventType EventType, data map[string]any) Event {
	return Event{
		ID:         "evt_" + uuid.New().String()[:12],
		Type:       eventType,
		Timestamp:  time.Now().UTC(),
		InstanceID: h.instanceID,
		Repo:       h.repo,
		Epic:       h.epic,
		Data:       data,
		Context:    h.buildContext(),
	}
}

func (h *httpReporter) buildContext() EventContext {
	ctx := EventContext{
		SessionID:        h.sessionID,
		SessionStart:     h.sessionStart,
		MaxIterations:    h.maxIterations,
		CurrentIteration: h.currentIteration,
		Status:           h.status,
		CurrentPhase:     h.currentPhase,
	}
	if h.analytics != nil {
		ctx.Analytics = EventAnalytics{
			PassedCount:     h.analytics.passedCount,
			FailedCount:     h.analytics.failedCount,
			TasksClosed:     h.analytics.tasksClosed,
			InitialReady:    h.analytics.initialReady,
			CurrentReady:    h.analytics.currentReady,
			AvgDurationMs:   h.analytics.avgDuration().Milliseconds(),
			TotalDurationMs: h.analytics.totalDuration().Milliseconds(),
		}
	}
	return ctx
}

func (h *httpReporter) SessionStarted(config SessionConfig) error {
	h.sessionStart = time.Now().UTC()
	h.maxIterations = config.MaxIterations
	h.status = "running"
	h.send(h.buildEvent(EventSessionStarted, map[string]any{
		"max_iterations":    config.MaxIterations,
		"sleep_seconds":     config.SleepSeconds,
		"epic":              config.Epic,
		"max_review_cycles": config.MaxReviewCycles,
	}))
	return nil
}

func (h *httpReporter) SessionEnded(reason string) error {
	h.status = "ended"
	h.send(h.buildEvent(EventSessionEnded, map[string]any{
		"reason": reason,
	}))
	return nil
}

func (h *httpReporter) IterationStarted(iteration int, phase string) error {
	h.currentIteration = iteration
	h.currentPhase = phase
	h.send(h.buildEvent(EventIterationStarted, map[string]any{
		"iteration": iteration,
		"phase":     phase,
	}))
	return nil
}

func (h *httpReporter) IterationCompleted(result IterationResult) error {
	h.send(h.buildEvent(EventIterationCompleted, map[string]any{
		"iteration":     result.Iteration,
		"duration_ms":   result.Duration.Milliseconds(),
		"task_id":       result.TaskID,
		"passed":        result.Passed,
		"notes":         result.Notes,
		"review_cycles": result.ReviewCycles,
		"final_verdict": result.FinalVerdict,
	}))
	return nil
}

func (h *httpReporter) PhaseChanged(from, to string) error {
	h.currentPhase = to
	h.send(h.buildEvent(EventPhaseChanged, map[string]any{
		"from_phase": from,
		"to_phase":   to,
	}))
	return nil
}

func (h *httpReporter) TaskClaimed(taskID, description string) error {
	h.send(h.buildEvent(EventTaskClaimed, map[string]any{
		"task_id":     taskID,
		"description": description,
	}))
	return nil
}

func (h *httpReporter) TaskClosed(taskID, commitHash string) error {
	h.send(h.buildEvent(EventTaskClosed, map[string]any{
		"task_id":     taskID,
		"commit_hash": commitHash,
	}))
	return nil
}
```

**Note:** This introduces a dependency on `github.com/google/uuid`. Run:

```bash
go get github.com/google/uuid
go mod vendor
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestHTTPReporter -v`
Expected: PASS

**Step 5: Lint**

Run: `golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add reporter_http.go reporter_http_test.go go.mod go.sum vendor/
git commit -m "feat: add httpReporter for fire-and-forget event posting"
```

---

### Task 5: CLI Flags and Reporter Wiring

**Files:**
- Modify: `main.go` (add flags, create reporter, pass to model)
- Modify: `model.go` (add reporter field to model)

**Step 1: Add flags to `main.go`**

Add these flag vars alongside the existing ones at the top of `main.go`:

```go
var (
	// ... existing flags ...
	hubURL     = flag.String("hub-url", "", "URL of the ralph-hub server (env: RALPH_HUB_URL)")
	hubAPIKey  = flag.String("hub-api-key", "", "API key for ralph-hub auth (env: RALPH_HUB_API_KEY)")
	instanceID = flag.String("instance-id", "", "Instance identifier (default: repo-name/epic)")
)
```

**Step 2: Add env var fallback and reporter creation in `main()`**

After `flag.Parse()`, add:

```go
// Env var fallbacks for hub config
hubURLVal := *hubURL
if hubURLVal == "" {
	hubURLVal = os.Getenv("RALPH_HUB_URL")
}
hubKeyVal := *hubAPIKey
if hubKeyVal == "" {
	hubKeyVal = os.Getenv("RALPH_HUB_API_KEY")
}
instanceIDVal := *instanceID
if instanceIDVal == "" {
	instanceIDVal = os.Getenv("RALPH_INSTANCE_ID")
}

var reporter Reporter
if hubURLVal != "" {
	repoName := instanceIDVal
	if repoName == "" {
		repoName = detectRepoName()
	}
	reporter = newHTTPReporter(hubURLVal, hubKeyVal, repoName, *epicFilter)
} else {
	reporter = &noopReporter{}
}
```

Pass `reporter` into `initialModel()` — change the signature to accept it.

**Step 3: Add reporter field to model in `model.go`**

Add to the `model` struct:

```go
// Reporting
reporter Reporter
```

Update `initialModel` to accept and store it:

```go
func initialModel(reporter Reporter) model {
	// ... existing code ...
	return model{
		// ... existing fields ...
		reporter: reporter,
	}
}
```

**Step 4: Build and lint**

Run: `go build -o ralph-loop-go && golangci-lint run`
Expected: Both pass

**Step 5: Commit**

```bash
git add main.go model.go
git commit -m "feat: add hub flags and wire reporter into model"
```

---

### Task 6: Wire Reporter Calls into update.go

**Files:**
- Modify: `update.go`

**Step 1: Add SessionStarted call in `Init()`**

In `model.Init()`, before the existing return, add the session started report:

```go
func (m model) Init() tea.Cmd {
	m.reporter.SessionStarted(SessionConfig{
		MaxIterations:   m.maxIter,
		SleepSeconds:    int(m.sleep.Seconds()),
		Epic:            m.epic,
		MaxReviewCycles: m.maxReviewCycles,
	})
	return tea.Batch(startNextIteration(), tick())
}
```

**Step 2: Add reporter calls at state transitions in `Update()`**

In the `startIterationMsg` handler (around line 91), after incrementing iteration:

```go
m.reporter.IterationStarted(m.iteration, m.currentPhase.String())
```

In each phase transition in the `claudeDoneMsg` handler:

**phasePlanner → phaseDev (around line 186):**
```go
m.reporter.PhaseChanged("planner", "dev")
```

**phaseDev → phaseReviewer (around line 203):**
```go
m.reporter.PhaseChanged("dev", "reviewer")
```

**phaseReviewer → approved/gave_up (around line 216), before the sleep/next iteration:**
```go
m.reporter.IterationCompleted(IterationResult{
	Iteration:    m.iteration,
	Duration:     elapsed,
	TaskID:       taskID,
	Passed:       passed,
	Notes:        notes,
	FinalVerdict: finalVerdict,
	ReviewCycles: m.reviewCycle,
})
```

**phaseReviewer → phaseFixer (around line 259):**
```go
m.reporter.PhaseChanged("reviewer", "fixer")
```

**phaseFixer → phaseReviewer (around line 267):**
```go
m.reporter.PhaseChanged("fixer", "reviewer")
```

**Step 3: Add SessionEnded on quit and finish**

In the `q/ctrl+c/esc` handler:
```go
m.reporter.SessionEnded("interrupted")
```

Where `statusFinished` is set (both the max-iterations case and the COMPLETE case):
```go
m.reporter.SessionEnded("complete")
```

On error:
```go
m.reporter.SessionEnded("error")
```

**Step 4: Update the httpReporter's analytics pointer**

In `main.go`, after creating the model, point the reporter at the model's analytics:

```go
if hr, ok := reporter.(*httpReporter); ok {
	hr.analytics = &m.analytics
}
```

**Step 5: Build and lint**

Run: `go build -o ralph-loop-go && golangci-lint run`
Expected: Both pass

**Step 6: Manual test**

Run with no hub URL (should behave exactly as before):
```bash
./ralph-loop-go -max-iterations 1
```

Run with a fake hub URL to verify it doesn't crash:
```bash
./ralph-loop-go -max-iterations 1 -hub-url http://localhost:9999
```

Expected: TUI works normally, stderr shows reporter send errors.

**Step 7: Commit**

```bash
git add update.go main.go
git commit -m "feat: wire reporter events into iteration and phase transitions"
```

---

### Task 7: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add hub flags to the documentation**

Update the Flags section in CLAUDE.md to include:

```
- `-hub-url` (default: "", env: RALPH_HUB_URL) - URL of ralph-hub server for centralized reporting
- `-hub-api-key` (default: "", env: RALPH_HUB_API_KEY) - API key for hub authentication
- `-instance-id` (default: repo/epic, env: RALPH_INSTANCE_ID) - Instance identifier
```

Update the Code Organization section to include new files:

```
├── reporter.go       # Reporter interface, noopReporter, SessionConfig, IterationResult
├── reporter_events.go # Event types and JSON schema for ralph-hub
├── reporter_http.go  # httpReporter implementation (fire-and-forget HTTP POST)
├── instance_id.go    # Instance ID derivation from git remote / directory / epic
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with hub reporting flags and new files"
```
