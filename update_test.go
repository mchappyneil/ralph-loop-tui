package main

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// spyReporter records all events dispatched via Send for test assertions.
type spyReporter struct {
	events []Event
}

func (s *spyReporter) Send(ev Event) { s.events = append(s.events, ev) }
func (s *spyReporter) Close() error  { return nil }

func (s *spyReporter) eventsOfType(t EventType) []Event {
	var out []Event
	for _, ev := range s.events {
		if ev.Type == t {
			out = append(out, ev)
		}
	}
	return out
}

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

func TestBdReadyCheckComplete_SendsSessionEnded(t *testing.T) {
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

	sessionEnds := spy.eventsOfType(EventSessionEnded)
	if len(sessionEnds) == 0 {
		t.Fatal("expected SessionEnded event, got none")
	}
	if sessionEnds[0].Data["reason"] != "complete" {
		t.Errorf("SessionEnded reason = %q, want %q", sessionEnds[0].Data["reason"], "complete")
	}
}

func TestPreflightZeroReady_SendsSessionEnded(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{readyCount: 0, blockedCount: 2, inProgressCount: 0, totalOpenCount: 2}
	m.Update(msg)

	sessionEnds := spy.eventsOfType(EventSessionEnded)
	if len(sessionEnds) == 0 {
		t.Fatal("expected SessionEnded event when preflight finds no ready work, got none")
	}
}

func TestMaxIterationsReached_SendsPhaseChangedToComplete(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 5
	m.maxIter = 5
	m.currentPhase = phaseReviewer

	msg := startIterationMsg{}
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
		t.Errorf("expected PhaseChanged to 'complete' on max iterations, got events: %v", spy.events)
	}
}

// --- Claude error recovery tests ---

func TestClaudeError_FirstError_ContinuesLoop(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseDev
	m.status = statusRunning

	msg := claudeDoneMsg{output: "", err: fmt.Errorf("claude error: exit status 1")}
	result, cmd := m.Update(msg)
	updated := result.(model)

	if cmd == nil {
		t.Fatal("cmd = nil, want non-nil command to continue loop after transient error")
	}
	if updated.status == statusFinished {
		t.Error("status should not be finished after first error — should retry")
	}
	if updated.consecutiveErrors != 1 {
		t.Errorf("consecutiveErrors = %d, want 1", updated.consecutiveErrors)
	}
	// Should NOT send SessionEnded since we're recovering
	sessionEnds := spy.eventsOfType(EventSessionEnded)
	if len(sessionEnds) != 0 {
		t.Errorf("should not send SessionEnded on recoverable error, got %d events", len(sessionEnds))
	}
}

func TestClaudeError_MaxConsecutiveErrors_SendsSessionEnded(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseDev
	m.status = statusRunning
	m.consecutiveErrors = 2 // Already had 2 errors

	msg := claudeDoneMsg{output: "", err: fmt.Errorf("claude error: exit status 1")}
	result, cmd := m.Update(msg)
	updated := result.(model)

	if updated.status != statusFinished {
		t.Errorf("status = %q, want %q after max consecutive errors", updated.status, statusFinished)
	}
	// Should send SessionEnded since retries exhausted
	sessionEnds := spy.eventsOfType(EventSessionEnded)
	if len(sessionEnds) == 0 {
		t.Fatal("expected SessionEnded event after max consecutive errors, got none")
	}
	if sessionEnds[0].Data["reason"] != "error" {
		t.Errorf("SessionEnded reason = %q, want %q", sessionEnds[0].Data["reason"], "error")
	}
	// Loop should stop — cmd may be ringBell() but status must be finished
	_ = cmd // ringBell is acceptable here
}

func TestClaudeError_ContextCancelled_DoesNotRetry(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseDev
	m.status = statusRunning

	// Cancel the context to simulate user quit
	m.cancel()

	msg := claudeDoneMsg{output: "", err: fmt.Errorf("claude error: signal: killed")}
	result, _ := m.Update(msg)
	updated := result.(model)

	// Should NOT schedule next iteration when context is cancelled
	if updated.status != statusError {
		t.Errorf("status = %q, want %q when context cancelled", updated.status, statusError)
	}
}

func TestClaudeSuccess_ResetsConsecutiveErrors(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseContextGatherer
	m.status = statusRunning
	m.consecutiveErrors = 2 // Had errors before

	// Successful context-gatherer output
	msg := claudeDoneMsg{output: `{"type":"text","text":"plan here"}`, err: nil}
	result, _ := m.Update(msg)
	updated := result.(model)

	if updated.consecutiveErrors != 0 {
		t.Errorf("consecutiveErrors = %d, want 0 after success", updated.consecutiveErrors)
	}
}

func TestSessionEnded_NotDuplicated_OnQuitAfterComplete(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseDev
	m.status = statusRunning

	// Simulate loop completion via bdReadyCheckMsg
	msg := bdReadyCheckMsg{readyCount: 0, err: nil}
	result, _ := m.Update(msg)
	updated := result.(model)

	// Now simulate user pressing q
	quitResult, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_ = quitResult

	// Should have exactly 1 SessionEnded event (complete), not 2
	sessionEnds := spy.eventsOfType(EventSessionEnded)
	if len(sessionEnds) != 1 {
		t.Errorf("expected exactly 1 SessionEnded event, got %d: %v", len(sessionEnds), sessionEnds)
	}
	if len(sessionEnds) > 0 && sessionEnds[0].Data["reason"] != "complete" {
		t.Errorf("SessionEnded reason = %q, want %q", sessionEnds[0].Data["reason"], "complete")
	}
}

func TestDefaultPhase_SendsSessionEnded(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = iterationPhase(99) // invalid phase
	m.status = statusRunning

	msg := claudeDoneMsg{output: "some output", err: nil}
	m.Update(msg)

	sessionEnds := spy.eventsOfType(EventSessionEnded)
	if len(sessionEnds) == 0 {
		t.Fatal("expected SessionEnded event for unknown phase error, got none")
	}
	if sessionEnds[0].Data["reason"] != "error" {
		t.Errorf("SessionEnded reason = %q, want %q", sessionEnds[0].Data["reason"], "error")
	}
}

func TestBuildEventContext_SnapshotsModelState(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.sessionID = "test-session-123"
	m.iteration = 3
	m.maxIter = 10
	m.currentPhase = phaseReviewer
	m.status = statusRunning

	ctx := m.buildEventContext()

	if ctx.SessionID != "test-session-123" {
		t.Errorf("SessionID = %q, want %q", ctx.SessionID, "test-session-123")
	}
	if ctx.CurrentIteration != 3 {
		t.Errorf("CurrentIteration = %d, want 3", ctx.CurrentIteration)
	}
	if ctx.MaxIterations != 10 {
		t.Errorf("MaxIterations = %d, want 10", ctx.MaxIterations)
	}
	if ctx.Status != "running" {
		t.Errorf("Status = %q, want %q", ctx.Status, "running")
	}
	if ctx.CurrentPhase != "reviewer" {
		t.Errorf("CurrentPhase = %q, want %q", ctx.CurrentPhase, "reviewer")
	}
}

func TestBuildEventContext_StatusEnded_WhenFinished(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.status = statusFinished

	ctx := m.buildEventContext()

	if ctx.Status != "ended" {
		t.Errorf("Status = %q, want %q for statusFinished", ctx.Status, "ended")
	}
}

func TestSendEvent_DispatchesThroughReporter(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.sessionID = "test-session"
	m.instanceID = "test-instance"
	m.repo = "test-repo"
	m.epic = "test-epic"
	m.iteration = 2
	m.currentPhase = phaseDev
	m.status = statusRunning

	m.sendEvent(EventPhaseChanged, map[string]any{"task_id": "BD-1"})

	if len(spy.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(spy.events))
	}
	ev := spy.events[0]
	if ev.Type != EventPhaseChanged {
		t.Errorf("event type = %q, want %q", ev.Type, EventPhaseChanged)
	}
	if ev.InstanceID != "test-instance" {
		t.Errorf("InstanceID = %q, want %q", ev.InstanceID, "test-instance")
	}
	if ev.Repo != "test-repo" {
		t.Errorf("Repo = %q, want %q", ev.Repo, "test-repo")
	}
	if ev.Epic != "test-epic" {
		t.Errorf("Epic = %q, want %q", ev.Epic, "test-epic")
	}
	if ev.Context.SessionID != "test-session" {
		t.Errorf("Context.SessionID = %q, want %q", ev.Context.SessionID, "test-session")
	}
}
