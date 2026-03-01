package main

import "testing"

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
