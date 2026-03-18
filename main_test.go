package main

import (
	"strings"
	"testing"
)

func TestDetectSpecialist_Go(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go`
	got := detectSpecialist(diff)
	if got != "senior Go engineer, idiomatic Go, concurrency, this codebase" {
		t.Errorf("got %q, want Go specialist", got)
	}
}

func TestDetectSpecialist_TypeScript(t *testing.T) {
	diff := `diff --git a/app.tsx b/app.tsx
--- a/app.tsx
+++ b/app.tsx`
	got := detectSpecialist(diff)
	if got != "senior TypeScript/React engineer" {
		t.Errorf("got %q, want TS specialist", got)
	}
}

func TestDetectSpecialist_Mixed(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
+++ b/main.go
diff --git a/handler.py b/handler.py
+++ b/handler.py`
	got := detectSpecialist(diff)
	if got == "" {
		t.Error("expected non-empty specialist for mixed diff")
	}
}

func TestDetectSpecialist_Unknown(t *testing.T) {
	diff := `diff --git a/README.md b/README.md`
	got := detectSpecialist(diff)
	if got != "senior software engineer" {
		t.Errorf("got %q, want default specialist", got)
	}
}

func TestBuildDevPrompt_ContainsPlannerOutput(t *testing.T) {
	plannerOutput := "PLAN: step 1, step 2"
	prompt := buildDevPrompt("", plannerOutput)
	if !strings.Contains(prompt, "Here is your implementation plan:") {
		t.Error("dev prompt missing plan header")
	}
	if !strings.Contains(prompt, plannerOutput) {
		t.Error("dev prompt missing planner output content")
	}
}

func TestBuildPlannerPrompt_ContainsAnalysisOnly(t *testing.T) {
	prompt := buildPlannerPrompt("")
	if !strings.Contains(prompt, "implementation plan") {
		t.Error("planner prompt should mention implementation plan")
	}
	if !strings.Contains(prompt, "[Planner output]") {
		t.Error("planner prompt should specify output format")
	}
}

func TestBuildReviewerPrompt_ContainsDiffAndSpecialist(t *testing.T) {
	diff := "diff --git a/main.go"
	specialist := "senior Go engineer"
	plannerOutput := "task: BD-1 implement feature"
	prompt := buildReviewerPrompt(plannerOutput, diff, specialist)
	if !strings.Contains(prompt, diff) {
		t.Error("reviewer prompt missing diff")
	}
	if !strings.Contains(prompt, specialist) {
		t.Error("reviewer prompt missing specialist persona")
	}
	if !strings.Contains(prompt, "[Reviewer status]") {
		t.Error("reviewer prompt missing output format spec")
	}
}

func TestBuildFixerPrompt_ContainsFeedback(t *testing.T) {
	plannerOutput := "plan: step 1"
	feedback := "verdict: CHANGES_REQUESTED\nissues:\n- missing tests"
	prompt := buildFixerPrompt("", plannerOutput, feedback)
	if !strings.Contains(prompt, "A reviewer found these issues") {
		t.Error("fixer prompt missing reviewer context header")
	}
	if !strings.Contains(prompt, feedback) {
		t.Error("fixer prompt missing reviewer feedback")
	}
}

func TestRalphContextCachePath(t *testing.T) {
	path := ralphContextCachePath("my-repo-BD-42")
	if !strings.HasSuffix(path, ".ralph-context-my-repo-BD-42.md") {
		t.Errorf("unexpected cache path: %q", path)
	}
	if !strings.HasPrefix(path, "/") {
		t.Errorf("cache path should be absolute, got: %q", path)
	}
}
