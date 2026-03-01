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
				PassedCount:     6,
				FailedCount:     1,
				TasksClosed:     6,
				InitialReady:    12,
				CurrentReady:    5,
				AvgDurationMs:   42000,
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
