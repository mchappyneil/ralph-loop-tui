package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// eventCollector captures events received by a test HTTP server.
type eventCollector struct {
	mu     sync.Mutex
	events []Event
}

func (c *eventCollector) collect(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var ev Event
	if err := json.Unmarshal(body, &ev); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	c.events = append(c.events, ev)
	c.mu.Unlock()
	w.WriteHeader(http.StatusAccepted)
}

func (c *eventCollector) getEvents() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]Event, len(c.events))
	copy(cp, c.events)
	return cp
}

func TestHTTPReporter_SendAndClose_DeliversEvent(t *testing.T) {
	collector := &eventCollector{}
	var lastAuthHeader string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		lastAuthHeader = r.Header.Get("Authorization")
		mu.Unlock()
		collector.collect(w, r)
	}))
	defer srv.Close()

	rpt := newHTTPReporter(srv.URL, "test-key", "repo", "epic")
	ev := Event{
		ID:   "evt-1",
		Type: EventSessionStarted,
	}
	rpt.Send(ev)

	if err := rpt.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	events := collector.getEvents()
	if len(events) == 0 {
		t.Fatal("expected at least 1 event, got 0")
	}
	if events[len(events)-1].ID != "evt-1" {
		t.Errorf("expected event ID evt-1, got %s", events[len(events)-1].ID)
	}

	mu.Lock()
	defer mu.Unlock()
	if lastAuthHeader != "Bearer test-key" {
		t.Errorf("expected Authorization header 'Bearer test-key', got %q", lastAuthHeader)
	}
}

func TestHTTPReporter_SendDoesNotBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	rpt := newHTTPReporter(srv.URL, "key", "repo", "epic")
	defer rpt.Close() //nolint:errcheck

	start := time.Now()
	rpt.Send(Event{ID: "evt-1", Type: EventSessionStarted})
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("Send blocked for %v, expected < 50ms", elapsed)
	}
}

func TestHTTPReporter_RetriesOnServerError(t *testing.T) {
	var attempts atomic.Int32
	collector := &eventCollector{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		collector.collect(w, r)
	}))
	defer srv.Close()

	rpt := newHTTPReporter(srv.URL, "key", "repo", "epic")
	rpt.Send(Event{ID: "evt-retry", Type: EventSessionStarted})

	if err := rpt.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if attempts.Load() < 3 {
		t.Errorf("expected >= 3 attempts, got %d", attempts.Load())
	}

	events := collector.getEvents()
	if len(events) == 0 {
		t.Fatal("expected event to be delivered after retries")
	}
	if events[0].ID != "evt-retry" {
		t.Errorf("expected event ID evt-retry, got %s", events[0].ID)
	}
}

func TestHTTPReporter_NewEventReplacesOld(t *testing.T) {
	var attempts atomic.Int32
	collector := &eventCollector{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		collector.collect(w, r)
	}))
	defer srv.Close()

	rpt := newHTTPReporter(srv.URL, "key", "repo", "epic")
	rpt.Send(Event{ID: "old-evt", Type: EventSessionStarted})

	// Give time for first attempt to start
	time.Sleep(20 * time.Millisecond)

	// Send newer event that should replace the old one during retry
	rpt.Send(Event{ID: "new-evt", Type: EventIterationStarted})

	if err := rpt.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	events := collector.getEvents()
	if len(events) == 0 {
		t.Fatal("expected at least 1 delivered event")
	}

	lastEvent := events[len(events)-1]
	if lastEvent.ID != "new-evt" {
		t.Errorf("expected last delivered event to be new-evt, got %s", lastEvent.ID)
	}
}

func TestHTTPReporter_CloseGuaranteesDelivery(t *testing.T) {
	collector := &eventCollector{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collector.collect(w, r)
	}))
	defer srv.Close()

	rpt := newHTTPReporter(srv.URL, "key", "repo", "epic")
	rpt.Send(Event{ID: "evt-a", Type: EventSessionStarted})
	rpt.Send(Event{ID: "evt-b", Type: EventSessionEnded})

	if err := rpt.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	events := collector.getEvents()
	if len(events) == 0 {
		t.Fatal("expected at least 1 event delivered after Close")
	}

	// With single-slot design, the last sent event must be delivered
	lastEvent := events[len(events)-1]
	if lastEvent.ID != "evt-b" {
		t.Errorf("expected final event to be evt-b, got %s", lastEvent.ID)
	}
}
