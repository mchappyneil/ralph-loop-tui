package main

import (
	"testing"
)

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
	if status.Notes != "Implementation looks clean and correct." {
		t.Errorf("got notes %q", status.Notes)
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
