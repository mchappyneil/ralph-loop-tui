package main

import "testing"

func TestParseContextGathererOutput(t *testing.T) {
	output := `[Context Gatherer output]
task: beads-abc123
task_title: Add input validation for signup
cache_hit: partial
patterns:
Use table-driven tests, validate at boundaries

`
	result := parseContextGathererOutput(output)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Task != "beads-abc123" {
		t.Errorf("Task = %q, want %q", result.Task, "beads-abc123")
	}
	if result.TaskTitle != "Add input validation for signup" {
		t.Errorf("TaskTitle = %q, want %q", result.TaskTitle, "Add input validation for signup")
	}
	if result.CacheHit != "partial" {
		t.Errorf("CacheHit = %q, want %q", result.CacheHit, "partial")
	}
}

func TestParseContextGathererOutput_Missing(t *testing.T) {
	result := parseContextGathererOutput("no block here")
	if result != nil {
		t.Errorf("expected nil for missing block, got %+v", result)
	}
}

func TestParseRalphStatus_TaskTitle(t *testing.T) {
	output := `[Ralph status]
ready_before: 3
ready_after: 2
task: beads-xyz789
task_title: Fix email regex
tests: PASSED
notes: Done

`
	status := parseRalphStatus(output)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.TaskTitle != "Fix email regex" {
		t.Errorf("TaskTitle = %q, want %q", status.TaskTitle, "Fix email regex")
	}
}
