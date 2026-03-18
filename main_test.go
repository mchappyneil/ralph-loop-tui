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

func TestBuildDevPrompt_ContainsGathererOutput(t *testing.T) {
	gathererOutput := "[Context Gatherer output]\ntask: BD-1\npatterns:\nUse table-driven tests."
	prompt := buildDevPrompt("", gathererOutput)
	if !strings.Contains(prompt, "Here is the codebase context gathered for this task:") {
		t.Error("dev prompt missing gatherer context header")
	}
	if !strings.Contains(prompt, gathererOutput) {
		t.Error("dev prompt missing gatherer output content")
	}
}

func TestBuildContextGathererPrompt_ContainsCacheAndFormat(t *testing.T) {
	instanceID := "my-repo-BD-42"
	cachePath := ralphContextCachePath(instanceID)
	prompt := buildContextGathererPrompt("", instanceID)
	if !strings.Contains(prompt, cachePath) {
		t.Errorf("gatherer prompt missing cache file path %q", cachePath)
	}
	if !strings.Contains(prompt, "[Context Gatherer output]") {
		t.Error("gatherer prompt missing output format spec")
	}
	if !strings.Contains(prompt, "cache_hit") {
		t.Error("gatherer prompt missing cache_hit field")
	}
}

func TestBuildContextGathererPrompt_EpicFilter(t *testing.T) {
	prompt := buildContextGathererPrompt("BD-42", "repo-BD-42")
	if !strings.Contains(prompt, "BD-42") {
		t.Error("gatherer prompt missing epic filter")
	}
}

func TestBuildReviewerPrompt_ContainsDiffAndSpecialist(t *testing.T) {
	diff := "diff --git a/main.go"
	specialist := "senior Go engineer"
	gathererOutput := "task: BD-1 implement feature"
	prompt := buildReviewerPrompt(gathererOutput, diff, specialist)
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
	gathererOutput := "task: BD-1\npatterns:\nUse table-driven tests."
	feedback := "verdict: CHANGES_REQUESTED\nissues:\n- missing tests"
	prompt := buildFixerPrompt("", gathererOutput, feedback)
	if !strings.Contains(prompt, "A reviewer found these issues") {
		t.Error("fixer prompt missing reviewer context header")
	}
	if !strings.Contains(prompt, feedback) {
		t.Error("fixer prompt missing reviewer feedback")
	}
}

func TestPhaseContextGatherer_StringValue(t *testing.T) {
	if phaseContextGatherer.String() != "context-gatherer" {
		t.Errorf("got %q, want context-gatherer", phaseContextGatherer.String())
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
