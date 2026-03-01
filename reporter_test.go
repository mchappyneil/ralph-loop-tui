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
