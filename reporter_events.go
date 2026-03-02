package main

import (
	"time"

	"github.com/google/uuid"
)

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

// NewEvent constructs a fully populated Event with a UUID and current timestamp.
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
