package main

import "testing"

func TestHighlight_TestPass(t *testing.T) {
	event := &ParsedEvent{Type: "tool_result", Summary: "ok  github.com/foo/bar 0.5s"}
	applyHighlight(event)
	if !event.Highlight || event.HighlightKind != "pass" {
		t.Errorf("expected pass highlight, got Highlight=%v Kind=%q", event.Highlight, event.HighlightKind)
	}
}

func TestHighlight_TestFail(t *testing.T) {
	event := &ParsedEvent{Type: "tool_result", Summary: "FAIL github.com/foo/bar"}
	applyHighlight(event)
	if !event.Highlight || event.HighlightKind != "fail" {
		t.Errorf("expected fail highlight, got Highlight=%v Kind=%q", event.Highlight, event.HighlightKind)
	}
}

func TestHighlight_Commit(t *testing.T) {
	event := &ParsedEvent{Type: "text", Summary: "feat: add input validation"}
	applyHighlight(event)
	if !event.Highlight || event.HighlightKind != "commit" {
		t.Errorf("expected commit highlight, got Highlight=%v Kind=%q", event.Highlight, event.HighlightKind)
	}
}

func TestHighlight_BdClose(t *testing.T) {
	event := &ParsedEvent{Type: "text", Summary: "Running bd close beads-abc123"}
	applyHighlight(event)
	if !event.Highlight || event.HighlightKind != "close" {
		t.Errorf("expected close highlight, got Highlight=%v Kind=%q", event.Highlight, event.HighlightKind)
	}
}

func TestHighlight_NoMatch(t *testing.T) {
	event := &ParsedEvent{Type: "tool_call", Summary: "Read src/main.go"}
	applyHighlight(event)
	if event.Highlight {
		t.Errorf("expected no highlight for plain tool_call, got Kind=%q", event.HighlightKind)
	}
}
