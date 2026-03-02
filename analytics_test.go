package main

import (
	"testing"
	"time"
)

func TestToEventAnalytics(t *testing.T) {
	a := newAnalyticsData()
	a.initialReady = 5
	a.currentReady = 3

	// One pass (10s), one fail (20s)
	a.addIteration(1, 10*time.Second, true, "BD-1", "done", "APPROVED", 0)
	a.addIteration(2, 20*time.Second, false, "BD-2", "broke", "ERROR", 1)

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

func TestParseReviewerStatus_Approved(t *testing.T) {
	output := `[Reviewer status]
verdict: APPROVED
specialist: senior Go engineer
issues:
- none
notes: Implementation looks clean and correct.

`
	status := parseReviewerStatus(output)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.Verdict != "APPROVED" {
		t.Errorf("got verdict %q, want APPROVED", status.Verdict)
	}
	if status.Specialist != "senior Go engineer" {
		t.Errorf("got specialist %q", status.Specialist)
	}
	if status.Notes != "Implementation looks clean and correct." {
		t.Errorf("got notes %q", status.Notes)
	}
	if len(status.Issues) != 1 || status.Issues[0] != "none" {
		t.Errorf("got issues %v, want [\"none\"]", status.Issues)
	}
}

func TestParseReviewerStatus_ChangesRequested(t *testing.T) {
	output := `[Reviewer status]
verdict: CHANGES_REQUESTED
specialist: senior Go engineer
issues:
- Missing error handling in handler
- No unit tests added
notes: Need to address error paths before approving.

`
	status := parseReviewerStatus(output)
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.Verdict != "CHANGES_REQUESTED" {
		t.Errorf("got verdict %q, want CHANGES_REQUESTED", status.Verdict)
	}
	if len(status.Issues) != 2 {
		t.Errorf("got %d issues, want 2", len(status.Issues))
	}
}

func TestParseReviewerStatus_MissingBlock(t *testing.T) {
	output := "No reviewer block here at all."
	status := parseReviewerStatus(output)
	if status == nil {
		t.Fatal("expected non-nil fallback status")
	}
	if status.Verdict != "APPROVED" {
		t.Errorf("missing block should default to APPROVED, got %q", status.Verdict)
	}
}
