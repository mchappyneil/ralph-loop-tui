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

// receivedEvent is a test helper that captures a decoded Event plus its raw HTTP metadata.
type receivedEvent struct {
	Event
	AuthHeader  string
	ContentType string
}

// eventCollector is a thread-safe slice of received events for use in httptest handlers.
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

func TestHTTPReporter_SendsSessionStarted(t *testing.T) {
	col := &eventCollector{}
	srv := httptest.NewServer(http.HandlerFunc(col.handler))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "BD-1")

	if err := r.SessionStarted(SessionConfig{MaxIterations: 50, SleepSeconds: 2}); err != nil {
		t.Fatalf("SessionStarted returned error: %v", err)
	}

	// Wait for goroutine to deliver the event.
	time.Sleep(200 * time.Millisecond)

	events := col.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]

	if ev.AuthHeader != "Bearer test-key" {
		t.Errorf("auth header = %q, want %q", ev.AuthHeader, "Bearer test-key")
	}
	if ev.ContentType != "application/json" {
		t.Errorf("content-type = %q, want %q", ev.ContentType, "application/json")
	}
	if ev.Type != EventSessionStarted {
		t.Errorf("event type = %q, want %q", ev.Type, EventSessionStarted)
	}
	if ev.InstanceID != "test-repo/BD-1" {
		t.Errorf("instance_id = %q, want %q", ev.InstanceID, "test-repo/BD-1")
	}
	if ev.ID == "" {
		t.Error("event_id is empty")
	}
	if ev.Context.SessionID == "" {
		t.Error("context.session_id is empty")
	}
	if ev.Context.MaxIterations != 50 {
		t.Errorf("context.max_iterations = %d, want 50", ev.Context.MaxIterations)
	}
}

func TestHTTPReporter_DoesNotBlockOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "")

	start := time.Now()
	if err := r.IterationStarted(1, "planner"); err != nil {
		t.Fatalf("IterationStarted returned error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("IterationStarted took %v, want < 50ms (fire-and-forget)", elapsed)
	}
}

func TestHTTPReporter_AllMethodsSendEvents(t *testing.T) {
	col := &eventCollector{}
	srv := httptest.NewServer(http.HandlerFunc(col.handler))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "")

	// Call every Reporter method once.
	_ = r.SessionStarted(SessionConfig{MaxIterations: 10})
	_ = r.IterationStarted(1, "planner")
	_ = r.PhaseChanged("planner", "reviewer")
	_ = r.TaskClaimed("BD-42", "Fix the widget")
	_ = r.IterationCompleted(IterationResult{
		Iteration: 1,
		Duration:  5 * time.Second,
		TaskID:    "BD-42",
		Passed:    true,
		Notes:     "all green",
	})
	_ = r.TaskClosed("BD-42", "abc123")
	_ = r.SessionEnded("all_complete")

	time.Sleep(300 * time.Millisecond)

	events := col.snapshot()
	if len(events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(events))
	}

	// Goroutines deliver in non-deterministic order; check all types are present.
	expectedTypes := map[EventType]bool{
		EventSessionStarted:     false,
		EventIterationStarted:   false,
		EventPhaseChanged:       false,
		EventTaskClaimed:        false,
		EventIterationCompleted: false,
		EventTaskClosed:         false,
		EventSessionEnded:       false,
	}
	for _, ev := range events {
		if _, ok := expectedTypes[ev.Type]; ok {
			expectedTypes[ev.Type] = true
		} else {
			t.Errorf("unexpected event type: %q", ev.Type)
		}
	}
	for typ, seen := range expectedTypes {
		if !seen {
			t.Errorf("missing event type: %q", typ)
		}
	}
}

func TestHTTPReporter_ContextUpdatesAcrossCalls(t *testing.T) {
	col := &eventCollector{}
	srv := httptest.NewServer(http.HandlerFunc(col.handler))
	defer srv.Close()

	r := newHTTPReporter(srv.URL, "test-key", "test-repo", "")

	_ = r.SessionStarted(SessionConfig{MaxIterations: 20})
	_ = r.IterationStarted(3, "reviewer")

	time.Sleep(200 * time.Millisecond)

	events := col.snapshot()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Find the IterationStarted event (goroutine ordering is non-deterministic).
	var found bool
	for _, ev := range events {
		if ev.Type != EventIterationStarted {
			continue
		}
		found = true
		ctx := ev.Context
		if ctx.CurrentIteration != 3 {
			t.Errorf("context.current_iteration = %d, want 3", ctx.CurrentIteration)
		}
		if ctx.CurrentPhase != "reviewer" {
			t.Errorf("context.current_phase = %q, want %q", ctx.CurrentPhase, "reviewer")
		}
		if ctx.MaxIterations != 20 {
			t.Errorf("context.max_iterations = %d, want 20", ctx.MaxIterations)
		}
	}
	if !found {
		t.Fatal("IterationStarted event not found")
	}
}
