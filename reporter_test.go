package main

import "testing"

func TestNoopReporterImplementsInterface(t *testing.T) {
	var r Reporter = &noopReporter{}
	r.Send(Event{})
	if err := r.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}
