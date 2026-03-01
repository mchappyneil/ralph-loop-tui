package main

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
)

func TestPreflightDoneMsg_Fields(t *testing.T) {
	msg := preflightDoneMsg{
		readyCount:      3,
		blockedCount:    1,
		inProgressCount: 2,
		totalOpenCount:  6,
		graphOutput:     "dep-graph-output",
		err:             nil,
	}

	if msg.readyCount != 3 {
		t.Errorf("readyCount = %d, want 3", msg.readyCount)
	}
	if msg.blockedCount != 1 {
		t.Errorf("blockedCount = %d, want 1", msg.blockedCount)
	}
	if msg.inProgressCount != 2 {
		t.Errorf("inProgressCount = %d, want 2", msg.inProgressCount)
	}
	if msg.totalOpenCount != 6 {
		t.Errorf("totalOpenCount = %d, want 6", msg.totalOpenCount)
	}
	if msg.graphOutput != "dep-graph-output" {
		t.Errorf("graphOutput = %q, want %q", msg.graphOutput, "dep-graph-output")
	}
	if msg.err != nil {
		t.Errorf("err = %v, want nil", msg.err)
	}
}

func TestRunPreflight_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Ensure bd CLI is available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd CLI not found, skipping integration test")
	}

	ctx := context.Background()
	cmd := runPreflight(ctx, "")
	result := cmd()

	msg, ok := result.(preflightDoneMsg)
	if !ok {
		t.Fatalf("expected preflightDoneMsg, got %T", result)
	}

	if msg.err != nil {
		t.Fatalf("runPreflight returned error: %v", msg.err)
	}

	if msg.readyCount < 0 {
		t.Errorf("readyCount = %d, want >= 0", msg.readyCount)
	}
	if msg.blockedCount < 0 {
		t.Errorf("blockedCount = %d, want >= 0", msg.blockedCount)
	}
	if msg.inProgressCount < 0 {
		t.Errorf("inProgressCount = %d, want >= 0", msg.inProgressCount)
	}
	if msg.totalOpenCount < 0 {
		t.Errorf("totalOpenCount = %d, want >= 0", msg.totalOpenCount)
	}
}

func TestPreflightDoneMsg_ZeroReady_SetsFinished(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{readyCount: 0, blockedCount: 2, inProgressCount: 0, totalOpenCount: 2}
	result, _ := m.Update(msg)
	updated := result.(model)

	if updated.status != statusFinished {
		t.Errorf("status = %q, want %q", updated.status, statusFinished)
	}
	if !updated.loopDone {
		t.Error("loopDone = false, want true")
	}
}

func TestPreflightDoneMsg_WithReady_StartsLoop(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{readyCount: 3, blockedCount: 1, inProgressCount: 0, totalOpenCount: 4}
	result, cmd := m.Update(msg)
	updated := result.(model)

	if updated.status == statusFinished {
		t.Error("status should not be statusFinished when ready work exists")
	}
	if updated.analytics.initialReady != 3 {
		t.Errorf("analytics.initialReady = %d, want 3", updated.analytics.initialReady)
	}
	if cmd == nil {
		t.Error("cmd = nil, want non-nil command to start loop")
	}
}

func TestPreflightDoneMsg_SeedsInitialReady(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{readyCount: 7, blockedCount: 0, inProgressCount: 0, totalOpenCount: 7}
	result, _ := m.Update(msg)
	updated := result.(model)

	if updated.analytics.initialReady != 7 {
		t.Errorf("analytics.initialReady = %d, want 7", updated.analytics.initialReady)
	}
}

func TestPreflightDoneMsg_Error_SetsIdleStatus(t *testing.T) {
	m := initialModel(&noopReporter{})
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{err: fmt.Errorf("bd not found")}
	result, cmd := m.Update(msg)
	updated := result.(model)

	if updated.status != statusIdle {
		t.Errorf("status = %q, want %q", updated.status, statusIdle)
	}
	if updated.statusText != "Preflight failed, starting loop" {
		t.Errorf("statusText = %q, want %q", updated.statusText, "Preflight failed, starting loop")
	}
	if cmd == nil {
		t.Error("cmd = nil, want non-nil command to start iteration")
	}
}
