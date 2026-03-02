# Reporter Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the 8-method Reporter interface with a thin `Send(Event)` + `Close()` pipe that retries with backoff and uses global replacement (new events supersede pending retries).

**Architecture:** The Reporter becomes a dumb delivery pipe — no session state, no analytics pointer. Event construction moves to the model via `sendEvent()` helper. The httpReporter uses a single-slot atomic pointer with a dedicated sender goroutine for retry with exponential backoff. `Close()` aggressively retries the final pending event.

**Tech Stack:** Go stdlib (`sync/atomic`, `net/http`, `os/signal`), `github.com/google/uuid`

---

### Task 1: Add NewEvent Constructor and toEventAnalytics Helper

These are additive — no existing code breaks.

**Files:**
- Modify: `reporter_events.go`
- Modify: `analytics.go`
- Test: `analytics_test.go`

**Step 1: Write failing test for toEventAnalytics**

In `analytics_test.go`, add:

```go
func TestToEventAnalytics(t *testing.T) {
	a := newAnalyticsData()
	a.initialReady = 5
	a.currentReady = 3
	a.addIteration(1, 10*time.Second, true, "BD-1", "ok", "APPROVED", 1)
	a.addIteration(2, 20*time.Second, false, "BD-2", "fail", "ERROR", 0)

	ea := a.toEventAnalytics()

	if ea.PassedCount != 1 {
		t.Errorf("PassedCount = %d, want 1", ea.PassedCount)
	}
	if ea.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", ea.FailedCount)
	}
	if ea.TasksClosed != 1 {
		t.Errorf("TasksClosed = %d, want 1", ea.TasksClosed)
	}
	if ea.InitialReady != 5 {
		t.Errorf("InitialReady = %d, want 5", ea.InitialReady)
	}
	if ea.CurrentReady != 3 {
		t.Errorf("CurrentReady = %d, want 3", ea.CurrentReady)
	}
	if ea.AvgDurationMs != 15000 {
		t.Errorf("AvgDurationMs = %d, want 15000", ea.AvgDurationMs)
	}
	if ea.TotalDurationMs != 30000 {
		t.Errorf("TotalDurationMs = %d, want 30000", ea.TotalDurationMs)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestToEventAnalytics -v ./...`
Expected: FAIL — `a.toEventAnalytics undefined`

**Step 3: Implement toEventAnalytics in analytics.go**

Add to `analytics.go`:

```go
// toEventAnalytics creates an EventAnalytics snapshot from the current state.
func (a *analyticsData) toEventAnalytics() EventAnalytics {
	return EventAnalytics{
		PassedCount:     a.passedCount,
		FailedCount:     a.failedCount,
		TasksClosed:     a.tasksClosed,
		InitialReady:    a.initialReady,
		CurrentReady:    a.currentReady,
		AvgDurationMs:   a.avgDuration().Milliseconds(),
		TotalDurationMs: a.totalDuration().Milliseconds(),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestToEventAnalytics -v ./...`
Expected: PASS

**Step 5: Add NewEvent constructor to reporter_events.go**

Add to `reporter_events.go`:

```go
import (
	"time"

	"github.com/google/uuid"
)
```

Then add the function:

```go
// NewEvent constructs a complete Event envelope with a generated ID and timestamp.
func NewEvent(eventType EventType, instanceID, repo, epic string, ctx EventContext, data map[string]any) Event {
	return Event{
		ID:         uuid.New().String(),
		Type:       eventType,
		Timestamp:  time.Now(),
		InstanceID: instanceID,
		Repo:       repo,
		Epic:       epic,
		Data:       data,
		Context:    ctx,
	}
}
```

**Step 6: Run all tests**

Run: `go test ./...`
Expected: PASS (all existing tests still pass, new test passes)

**Step 7: Lint and commit**

```bash
golangci-lint run
git add analytics.go analytics_test.go reporter_events.go
git commit -m "feat: add NewEvent constructor and toEventAnalytics helper"
```

---

### Task 2: Add Model Event Helpers

Add `buildEventContext()` and `sendEvent()` to the model. These are additive — nothing calls them yet.

**Files:**
- Modify: `model.go`

**Step 1: Add new fields and helper methods to model.go**

Add fields to the `model` struct (in the "Reporting" section, around line 146):

```go
	// Reporting
	reporter      Reporter
	hubURL        string
	hubInstanceID string
	sessionEnded  bool // prevents duplicate SessionEnded calls
	sessionID     string // unique ID for this session
	repo          string // repository name for event reporting
	instanceID    string // instance ID for event reporting
```

Note: `epic` already exists in the model struct.

Add method after `endSession`:

```go
// buildEventContext creates an EventContext snapshot from the model's current state.
func (m *model) buildEventContext() EventContext {
	status := "running"
	if m.status == statusFinished {
		status = "ended"
	}
	return EventContext{
		SessionID:        m.sessionID,
		SessionStart:     m.sessionStart,
		MaxIterations:    m.maxIter,
		CurrentIteration: m.iteration,
		Status:           status,
		CurrentPhase:     m.currentPhase.String(),
		Analytics:        m.analytics.toEventAnalytics(),
	}
}

// sendEvent builds and sends an event through the reporter.
func (m *model) sendEvent(eventType EventType, data map[string]any) {
	m.reporter.Send(NewEvent(eventType, m.instanceID, m.repo, m.epic,
		m.buildEventContext(), data))
}
```

**Step 2: Run all tests**

Run: `go test ./...`
Expected: PASS (new methods exist but nothing calls them yet)

**Step 3: Lint and commit**

```bash
golangci-lint run
git add model.go
git commit -m "feat: add buildEventContext and sendEvent helpers to model"
```

---

### Task 3: Swap Reporter Interface and Migrate All Call Sites

This is the big atomic change. The Reporter interface shrinks to 2 methods, and all call sites migrate simultaneously. The old httpReporter is temporarily stubbed.

**Files:**
- Modify: `reporter.go`
- Modify: `reporter_http.go` (temporary stub)
- Modify: `model.go` (endSession)
- Modify: `update.go` (all call sites)
- Modify: `update_test.go` (new spyReporter)
- Modify: `reporter_test.go`

**Step 1: Rewrite reporter.go**

Replace the entire file:

```go
package main

// Reporter sends events to a ralph-hub server.
// Send is non-blocking and replaces any pending retry.
// Close must be called before the process exits to flush pending events.
type Reporter interface {
	Send(event Event)
	Close() error
}

// noopReporter is used when no hub URL is configured.
type noopReporter struct{}

func (n *noopReporter) Send(Event) {}
func (n *noopReporter) Close() error { return nil }
```

**Step 2: Stub httpReporter for compilation**

Replace the entire `reporter_http.go` with a minimal stub that compiles:

```go
package main

import (
	"net/http"
	"time"
)

// httpReporter implements Reporter by POSTing events to a ralph-hub server.
// TODO: Task 4 will implement retry with backoff and global replacement.
type httpReporter struct {
	hubURL     string
	apiKey     string
	instanceID string
	repo       string
	epic       string
	client     *http.Client
}

func newHTTPReporter(hubURL, apiKey, repo, epic string) *httpReporter {
	return &httpReporter{
		hubURL:     hubURL,
		apiKey:     apiKey,
		instanceID: deriveInstanceID(repo, epic),
		repo:       repo,
		epic:       epic,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *httpReporter) Send(ev Event) {}

func (h *httpReporter) Close() error { return nil }
```

**Step 3: Rewrite spyReporter in update_test.go**

Replace the `spyReporter` and its methods (lines 12-67) with:

```go
// spyReporter records all events sent for test assertions.
type spyReporter struct {
	events []Event
}

func (s *spyReporter) Send(ev Event) {
	s.events = append(s.events, ev)
}

func (s *spyReporter) Close() error { return nil }

// eventsOfType returns all events matching the given type.
func (s *spyReporter) eventsOfType(t EventType) []Event {
	var out []Event
	for _, ev := range s.events {
		if ev.Type == t {
			out = append(out, ev)
		}
	}
	return out
}
```

**Step 4: Update model.endSession()**

In `model.go`, replace the `endSession` method:

```go
// endSession sends a SessionEnded event exactly once, preventing duplicates.
func (m *model) endSession(reason string) {
	if m.sessionEnded {
		return
	}
	m.sessionEnded = true
	m.sendEvent(EventSessionEnded, map[string]any{"reason": reason})
}
```

**Step 5: Update update.go Init()**

Replace lines 14-21 of `update.go`:

```go
func (m model) Init() tea.Cmd {
	m.sendEvent(EventSessionStarted, map[string]any{
		"max_iterations":    m.maxIter,
		"sleep_seconds":     int(m.sleep.Seconds()),
		"epic":              m.epic,
		"max_review_cycles": m.maxReviewCycles,
	})
	return tea.Batch(runPreflight(m.ctx, m.epic), tick())
}
```

**Step 6: Update startIterationMsg handler (max iterations path)**

Replace lines 141-142:

```go
		// Before: m.reporter.PrepareShutdown("complete")
		// Before: _ = m.reporter.PhaseChanged(...)
		m.sendEvent(EventPhaseChanged, map[string]any{
			"from": m.currentPhase.String(), "to": "complete",
		})
```

**Step 7: Update startIterationMsg handler (iteration start)**

Replace line 170:

```go
		m.sendEvent(EventIterationStarted, map[string]any{
			"iteration": m.iteration,
			"phase":     m.currentPhase.String(),
		})
```

**Step 8: Update claudeDoneMsg error path (IterationCompleted)**

Replace lines 232-238:

```go
			m.sendEvent(EventIterationCompleted, map[string]any{
				"iteration":     m.iteration,
				"duration_ms":   elapsed.Milliseconds(),
				"passed":        false,
				"notes":         msg.err.Error(),
				"final_verdict": "ERROR",
			})
```

**Step 9: Update claudeDoneMsg error exhaustion path**

Replace lines 261-262 (remove PrepareShutdown, update PhaseChanged):

```go
			m.sendEvent(EventPhaseChanged, map[string]any{
				"from": m.currentPhase.String(), "to": "error",
			})
```

**Step 10: Update phasePlanner → dev PhaseChanged**

Replace line 273:

```go
			m.sendEvent(EventPhaseChanged, map[string]any{"from": "planner", "to": "dev"})
```

**Step 11: Update phaseDev → reviewer PhaseChanged**

Replace lines 287-288:

```go
			m.sendEvent(EventPhaseChanged, map[string]any{"from": "dev", "to": "reviewer"})
```

**Step 12: Update phaseReviewer IterationCompleted**

Replace lines 326-334:

```go
				m.sendEvent(EventIterationCompleted, map[string]any{
					"iteration":     m.iteration,
					"duration_ms":   elapsed.Milliseconds(),
					"task_id":       taskID,
					"passed":        passed,
					"notes":         notes,
					"final_verdict": finalVerdict,
					"review_cycles": m.reviewCycle,
				})
```

**Step 13: Update reviewer → fixer PhaseChanged**

Replace line 355:

```go
			m.sendEvent(EventPhaseChanged, map[string]any{"from": "reviewer", "to": "fixer"})
```

**Step 14: Update fixer → reviewer PhaseChanged**

Replace line 363:

```go
			m.sendEvent(EventPhaseChanged, map[string]any{"from": "fixer", "to": "reviewer"})
```

**Step 15: Update bdReadyCheckMsg override path (IterationCompleted)**

Replace lines 398-405:

```go
			m.sendEvent(EventIterationCompleted, map[string]any{
				"iteration":     m.iteration,
				"duration_ms":   elapsed.Milliseconds(),
				"task_id":       taskID,
				"passed":        true,
				"notes":         "COMPLETE overridden — ready work remains",
				"final_verdict": "OVERRIDE",
			})
```

**Step 16: Update bdReadyCheckMsg complete path**

Replace lines 425-434 (remove PrepareShutdown):

```go
		m.sendEvent(EventIterationCompleted, map[string]any{
			"iteration":     m.iteration,
			"duration_ms":   elapsed.Milliseconds(),
			"task_id":       taskID,
			"passed":        true,
			"notes":         "No ready work remaining (verified)",
			"final_verdict": "COMPLETE",
		})
		m.sendEvent(EventPhaseChanged, map[string]any{
			"from": m.currentPhase.String(), "to": "complete",
		})
```

**Step 17: Update update_test.go assertions**

Rewrite each test to use the new spy. The tests check `spy.eventsOfType()` instead of `spy.callsOf()`. Example for `TestBdReadyCheckComplete_SendsPhaseChangedToComplete`:

```go
func TestBdReadyCheckComplete_SendsPhaseChangedToComplete(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseDev
	m.status = statusRunning

	msg := bdReadyCheckMsg{readyCount: 0, err: nil}
	m.Update(msg)

	phaseChanges := spy.eventsOfType(EventPhaseChanged)
	found := false
	for _, ev := range phaseChanges {
		if ev.Data["to"] == "complete" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PhaseChanged to 'complete', got events: %v", spy.events)
	}
}
```

Apply the same pattern to all other tests:
- `TestBdReadyCheckComplete_SendsSessionEnded`: check `spy.eventsOfType(EventSessionEnded)`
- `TestPreflightZeroReady_SendsSessionEnded`: same pattern
- `TestMaxIterationsReached_SendsPhaseChangedToComplete`: check PhaseChanged events
- `TestClaudeError_FirstError_ContinuesLoop`: check no `EventSessionEnded` events
- `TestClaudeError_MaxConsecutiveErrors_SendsSessionEnded`: check `EventSessionEnded` with reason
- `TestClaudeError_ContextCancelled_DoesNotRetry`: no change needed (checks model state)
- `TestClaudeSuccess_ResetsConsecutiveErrors`: no change needed
- `TestSessionEnded_NotDuplicated_OnQuitAfterComplete`: check exactly 1 `EventSessionEnded`
- `TestDefaultPhase_SendsSessionEnded`: check `EventSessionEnded` with reason "error"

For tests that check reason, access it via `ev.Data["reason"]`.

**Step 18: Update reporter_test.go**

Replace the entire file:

```go
package main

import "testing"

func TestNoopReporterImplementsInterface(t *testing.T) {
	var r Reporter = &noopReporter{}
	r.Send(Event{})
	if err := r.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}
```

**Step 19: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 20: Lint and commit**

```bash
golangci-lint run
git add reporter.go reporter_http.go model.go update.go update_test.go reporter_test.go
git commit -m "refactor: swap Reporter to thin Send/Close interface, migrate all call sites"
```

---

### Task 4: Implement httpReporter with Retry and Global Replacement (TDD)

**Files:**
- Rewrite: `reporter_http.go`
- Rewrite: `reporter_http_test.go`

**Step 1: Write test for basic Send + Close delivery**

Replace `reporter_http_test.go`:

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

type receivedEvent struct {
	Event
	AuthHeader  string
	ContentType string
}

type eventCollector struct {
	mu     sync.Mutex
	events []receivedEvent
}

func (c *eventCollector) handler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var ev Event
	if err := json.Unmarshal(body, &ev); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	c.events = append(c.events, receivedEvent{
		Event:       ev,
		AuthHeader:  r.Header.Get("Authorization"),
		ContentType: r.Header.Get("Content-Type"),
	})
	c.mu.Unlock()
	w.WriteHeader(http.StatusAccepted)
}

func (c *eventCollector) snapshot() []receivedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]receivedEvent, len(c.events))
	copy(out, c.events)
	return out
}

func TestHTTPReporter_SendAndClose_DeliversEvent(t *testing.T) {
	col := &eventCollector{}
	srv := httptest.NewServer(http.HandlerFunc(col.handler))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "BD-1")
	ev := NewEvent(EventSessionStarted, "test-repo/BD-1", "test-repo", "BD-1",
		EventContext{MaxIterations: 50}, map[string]any{"max_iterations": 50})

	r.Send(ev)
	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	events := col.snapshot()
	if len(events) == 0 {
		t.Fatal("expected at least 1 event, got 0")
	}

	got := events[len(events)-1]
	if got.AuthHeader != "Bearer test-key" {
		t.Errorf("auth = %q, want %q", got.AuthHeader, "Bearer test-key")
	}
	if got.Type != EventSessionStarted {
		t.Errorf("type = %q, want %q", got.Type, EventSessionStarted)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestHTTPReporter_SendAndClose -v ./...`
Expected: FAIL (stub Send does nothing)

**Step 3: Implement core httpReporter**

Replace `reporter_http.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// httpReporter implements Reporter by POSTing events to a ralph-hub server.
// It uses a single-slot pending event with a dedicated sender goroutine.
// New events replace any pending retry (global replacement).
type httpReporter struct {
	hubURL     string
	apiKey     string
	instanceID string
	repo       string
	epic       string
	client     *http.Client

	pending  atomic.Pointer[Event] // single-slot: latest event waiting to be sent
	wake     chan struct{}          // signals sender goroutine that new work is available
	done     chan struct{}          // closed when sender goroutine exits
	closeOnce sync.Once
	closing  atomic.Bool           // true after Close() is called
}

func newHTTPReporter(hubURL, apiKey, repo, epic string) *httpReporter {
	h := &httpReporter{
		hubURL:     hubURL,
		apiKey:     apiKey,
		instanceID: deriveInstanceID(repo, epic),
		repo:       repo,
		epic:       epic,
		client:     &http.Client{Timeout: 10 * time.Second},
		wake:       make(chan struct{}, 1),
		done:       make(chan struct{}),
	}
	go h.senderLoop()
	return h
}

// Send stores the event in the single slot and wakes the sender.
// If the sender is retrying an older event, it will be replaced.
func (h *httpReporter) Send(ev Event) {
	if h.closing.Load() {
		return
	}
	h.pending.Store(&ev)
	select {
	case h.wake <- struct{}{}:
	default: // sender already notified
	}
}

// Close signals the sender to drain and waits for completion.
// The final pending event is retried aggressively (up to 10 attempts).
func (h *httpReporter) Close() error {
	var err error
	h.closeOnce.Do(func() {
		h.closing.Store(true)
		// Wake sender so it sees the closing flag
		select {
		case h.wake <- struct{}{}:
		default:
		}

		select {
		case <-h.done:
		case <-time.After(15 * time.Second):
			err = fmt.Errorf("reporter: timed out waiting for %s pending events", h.hubURL)
		}
	})
	return err
}

// senderLoop runs in a goroutine, sending pending events with retry.
func (h *httpReporter) senderLoop() {
	defer close(h.done)

	for {
		// Wait for work or close signal
		<-h.wake

		ev := h.pending.Swap(nil)
		if ev == nil {
			if h.closing.Load() {
				return
			}
			continue
		}

		if h.closing.Load() {
			// Drain mode: retry aggressively
			h.sendWithRetry(ev, 10, 100*time.Millisecond, 2*time.Second)
			return
		}

		// Normal mode: retry with backoff, but check for replacement
		h.sendWithRetry(ev, 0, 100*time.Millisecond, 5*time.Second)
	}
}

// sendWithRetry attempts to send an event. maxAttempts=0 means unlimited
// (will stop when a new event replaces the current one or close is called).
func (h *httpReporter) sendWithRetry(ev *Event, maxAttempts int, initialBackoff, maxBackoff time.Duration) {
	backoff := initialBackoff
	attempt := 0

	for {
		if h.doSend(ev) {
			return // success
		}

		attempt++
		if maxAttempts > 0 && attempt >= maxAttempts {
			fmt.Fprintf(os.Stderr, "reporter: giving up on %s after %d attempts\n", ev.Type, attempt)
			return
		}

		// Check if a newer event has replaced this one
		if newer := h.pending.Swap(nil); newer != nil {
			// Replace current event with the newer one
			ev = newer
			backoff = initialBackoff
			attempt = 0
			continue
		}

		// Check if we should stop
		if h.closing.Load() && maxAttempts == 0 {
			// Switch to drain mode for current event
			h.sendWithRetry(ev, 10, 100*time.Millisecond, 2*time.Second)
			return
		}

		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// doSend performs a single HTTP POST attempt. Returns true on success.
func (h *httpReporter) doSend(ev *Event) bool {
	body, err := json.Marshal(ev)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reporter: marshal error: %v\n", err)
		return true // don't retry marshal errors
	}

	req, err := http.NewRequest(http.MethodPost, h.hubURL+"/api/v1/events", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "reporter: request error: %v\n", err)
		return true // don't retry request construction errors
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reporter: send error for %s: %v\n", ev.Type, err)
		return false // retry on network errors
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 500 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		fmt.Fprintf(os.Stderr, "reporter: hub returned %s for %s: %s\n", resp.Status, ev.Type, respBody)
		return false // retry on server errors
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		fmt.Fprintf(os.Stderr, "reporter: hub returned %s for %s: %s\n", resp.Status, ev.Type, respBody)
		return true // don't retry client errors (4xx)
	}

	return true
}
```

**Step 4: Run basic delivery test**

Run: `go test -run TestHTTPReporter_SendAndClose -v ./...`
Expected: PASS

**Step 5: Write test for Send not blocking**

Add to `reporter_http_test.go`:

```go
func TestHTTPReporter_SendDoesNotBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "")
	ev := NewEvent(EventIterationStarted, "test-repo", "test-repo", "",
		EventContext{}, map[string]any{"iteration": 1})

	start := time.Now()
	r.Send(ev)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("Send took %v, want < 50ms (non-blocking)", elapsed)
	}

	_ = r.Close()
}
```

**Step 6: Run non-blocking test**

Run: `go test -run TestHTTPReporter_SendDoesNotBlock -v ./...`
Expected: PASS

**Step 7: Write test for retry on server error**

Add to `reporter_http_test.go`:

```go
func TestHTTPReporter_RetriesOnServerError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "")
	ev := NewEvent(EventSessionStarted, "test-repo", "test-repo", "",
		EventContext{}, map[string]any{})
	r.Send(ev)

	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	mu.Lock()
	finalAttempts := attempts
	mu.Unlock()
	if finalAttempts < 3 {
		t.Errorf("attempts = %d, want >= 3 (should retry on 500)", finalAttempts)
	}
}
```

**Step 8: Run retry test**

Run: `go test -run TestHTTPReporter_RetriesOnServerError -v ./...`
Expected: PASS

**Step 9: Write test for global replacement**

Add to `reporter_http_test.go`:

```go
func TestHTTPReporter_NewEventReplacesOld(t *testing.T) {
	// Server blocks first request long enough for replacement to happen
	var mu sync.Mutex
	received := []EventType{}
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n == 1 {
			// First attempt: fail to trigger retry
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var ev Event
		_ = json.Unmarshal(body, &ev)
		mu.Lock()
		received = append(received, ev.Type)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "")

	// Send first event (will fail and enter retry)
	ev1 := NewEvent(EventIterationStarted, "test-repo", "test-repo", "",
		EventContext{}, map[string]any{"iteration": 1})
	r.Send(ev1)

	// Brief pause to let first attempt fail
	time.Sleep(50 * time.Millisecond)

	// Send second event — should replace the first during retry
	ev2 := NewEvent(EventSessionEnded, "test-repo", "test-repo", "",
		EventContext{}, map[string]any{"reason": "complete"})
	r.Send(ev2)

	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// The last successfully delivered event should be session.ended
	if len(received) == 0 {
		t.Fatal("expected at least 1 delivered event")
	}
	last := received[len(received)-1]
	if last != EventSessionEnded {
		t.Errorf("last delivered = %q, want %q (newer event should replace older)", last, EventSessionEnded)
	}
}
```

**Step 10: Run replacement test**

Run: `go test -run TestHTTPReporter_NewEventReplacesOld -v ./...`
Expected: PASS

**Step 11: Write test for Close guarantees delivery**

Add to `reporter_http_test.go`:

```go
func TestHTTPReporter_CloseGuaranteesDelivery(t *testing.T) {
	col := &eventCollector{}
	srv := httptest.NewServer(http.HandlerFunc(col.handler))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "")

	ev1 := NewEvent(EventSessionStarted, "test-repo", "test-repo", "",
		EventContext{}, map[string]any{})
	r.Send(ev1)

	// Small pause so first event is likely sent
	time.Sleep(50 * time.Millisecond)

	ev2 := NewEvent(EventSessionEnded, "test-repo", "test-repo", "",
		EventContext{}, map[string]any{"reason": "complete"})
	r.Send(ev2)

	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	events := col.snapshot()
	var foundEnded bool
	for _, ev := range events {
		if ev.Type == EventSessionEnded {
			foundEnded = true
		}
	}
	if !foundEnded {
		t.Error("session.ended not delivered after Close")
	}
}
```

**Step 12: Run all reporter tests**

Run: `go test -run TestHTTPReporter -v ./...`
Expected: PASS

**Step 13: Run full test suite**

Run: `go test ./...`
Expected: PASS

**Step 14: Lint and commit**

```bash
golangci-lint run
git add reporter_http.go reporter_http_test.go
git commit -m "feat: implement httpReporter with retry, backoff, and global replacement"
```

---

### Task 5: Simplify main.go and Add Signal Handler

**Files:**
- Modify: `main.go`

**Step 1: Remove analytics pointer sharing and simplify reporter init**

In `main.go`, replace lines 67-72:

```go
	m := initialModel(reporter)
	m.hubURL = hubURLVal
	if hr, ok := reporter.(*httpReporter); ok {
		m.hubInstanceID = hr.instanceID
		m.instanceID = hr.instanceID
		m.repo = repoName
	}
```

Remove the old analytics pointer sharing (`hr.analytics = m.analytics` is gone).

Also set `m.repo` and `m.instanceID` for the noop case too. Replace with:

```go
	m := initialModel(reporter)
	m.hubURL = hubURLVal
	m.repo = repoName
	m.instanceID = deriveInstanceID(repoName, *epicFilter)
	if hr, ok := reporter.(*httpReporter); ok {
		m.hubInstanceID = hr.instanceID
	}
```

**Step 2: Add signal handler**

Add import for `os/signal` and `syscall`. Before the `p := tea.NewProgram(m)` line, add:

```go
	// Ensure reporter.Close() runs on signal exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		_ = reporter.Close()
		os.Exit(1)
	}()
```

**Step 3: Add sessionID to model initialization**

In `model.go`, in the `initialModel` function, add to the return struct:

```go
		sessionID: uuid.New().String(),
```

Add `"github.com/google/uuid"` to model.go imports.

**Step 4: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 5: Build and verify**

```bash
go build -o ralph-loop-go
golangci-lint run
```

Expected: Clean build, no lint errors.

**Step 6: Commit**

```bash
git add main.go model.go
git commit -m "refactor: simplify reporter init, add signal handler for guaranteed close"
```

---

### Task 6: Remove Dead Code and Final Cleanup

**Files:**
- Modify: `reporter_events.go` (remove unused event type constants)

**Step 1: Remove unused EventTaskClaimed and EventTaskClosed constants**

In `reporter_events.go`, remove:

```go
	EventTaskClaimed        EventType = "task.claimed"
	EventTaskClosed         EventType = "task.closed"
```

**Step 2: Run all tests and lint**

```bash
go test ./...
golangci-lint run
```

Expected: PASS, no lint errors.

**Step 3: Commit**

```bash
git add reporter_events.go
git commit -m "chore: remove unused TaskClaimed/TaskClosed event types"
```

---

### Summary of Changes

| Before | After |
|--------|-------|
| 8-method Reporter interface | 2-method: `Send(Event)` + `Close() error` |
| `SessionConfig`, `IterationResult` types | Inline `map[string]any` data |
| `PrepareShutdown()` for state consistency | Model sets own state before `buildEventContext()` |
| Shared `*analyticsData` pointer | `toEventAnalytics()` snapshot at event-build time |
| Fire-and-forget goroutines, no retry | Single-slot atomic + sender goroutine with exponential backoff |
| Global replacement: new events supersede retries | Yes |
| Signal handler for guaranteed Close() | Yes (SIGTERM, SIGINT) |
| `TaskClaimed` / `TaskClosed` (unused) | Removed |
