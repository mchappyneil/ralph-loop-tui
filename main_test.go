package main

import (
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

