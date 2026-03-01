package main

import (
	"testing"
	"time"
)

// spyReporter records all Reporter method calls for test assertions.
type spyReporter struct {
	calls []spyCall
}

type spyCall struct {
	method string
	args   map[string]any
}

func (s *spyReporter) SessionStarted(config SessionConfig) error {
	s.calls = append(s.calls, spyCall{"SessionStarted", map[string]any{"config": config}})
	return nil
}

func (s *spyReporter) SessionEnded(reason string) error {
	s.calls = append(s.calls, spyCall{"SessionEnded", map[string]any{"reason": reason}})
	return nil
}

func (s *spyReporter) IterationStarted(iteration int, phase string) error {
	s.calls = append(s.calls, spyCall{"IterationStarted", map[string]any{"iteration": iteration, "phase": phase}})
	return nil
}

func (s *spyReporter) IterationCompleted(result IterationResult) error {
	s.calls = append(s.calls, spyCall{"IterationCompleted", map[string]any{"result": result}})
	return nil
}

func (s *spyReporter) PhaseChanged(from, to string) error {
	s.calls = append(s.calls, spyCall{"PhaseChanged", map[string]any{"from": from, "to": to}})
	return nil
}

func (s *spyReporter) TaskClaimed(taskID, description string) error {
	s.calls = append(s.calls, spyCall{"TaskClaimed", map[string]any{"taskID": taskID, "description": description}})
	return nil
}

func (s *spyReporter) TaskClosed(taskID, commitHash string) error {
	s.calls = append(s.calls, spyCall{"TaskClosed", map[string]any{"taskID": taskID, "commitHash": commitHash}})
	return nil
}

func (s *spyReporter) callsOf(method string) []spyCall {
	var out []spyCall
	for _, c := range s.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func TestBdReadyCheckComplete_SendsPhaseChangedToComplete(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseDev
	m.status = statusRunning

	msg := bdReadyCheckMsg{readyCount: 0, err: nil}
	m.Update(msg)

	phaseChanges := spy.callsOf("PhaseChanged")
	found := false
	for _, c := range phaseChanges {
		if c.args["to"] == "complete" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PhaseChanged to 'complete', got calls: %v", spy.calls)
	}
}

func TestBdReadyCheckComplete_SendsSessionEnded(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 1
	m.startTime = time.Now().Add(-10 * time.Second)
	m.currentPhase = phaseDev
	m.status = statusRunning

	msg := bdReadyCheckMsg{readyCount: 0, err: nil}
	m.Update(msg)

	sessionEnds := spy.callsOf("SessionEnded")
	if len(sessionEnds) == 0 {
		t.Fatal("expected SessionEnded call, got none")
	}
	if sessionEnds[0].args["reason"] != "complete" {
		t.Errorf("SessionEnded reason = %q, want %q", sessionEnds[0].args["reason"], "complete")
	}
}

func TestPreflightZeroReady_SendsSessionEnded(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24

	msg := preflightDoneMsg{readyCount: 0, blockedCount: 2, inProgressCount: 0, totalOpenCount: 2}
	m.Update(msg)

	sessionEnds := spy.callsOf("SessionEnded")
	if len(sessionEnds) == 0 {
		t.Fatal("expected SessionEnded call when preflight finds no ready work, got none")
	}
}

func TestMaxIterationsReached_SendsPhaseChangedToComplete(t *testing.T) {
	spy := &spyReporter{}
	m := initialModel(spy)
	m.width = 80
	m.height = 24
	m.iteration = 5
	m.maxIter = 5
	m.currentPhase = phaseReviewer

	msg := startIterationMsg{}
	m.Update(msg)

	phaseChanges := spy.callsOf("PhaseChanged")
	found := false
	for _, c := range phaseChanges {
		if c.args["to"] == "complete" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PhaseChanged to 'complete' on max iterations, got calls: %v", spy.calls)
	}
}
