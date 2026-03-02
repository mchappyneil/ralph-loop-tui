package main

import "testing"

func TestHTTPReporter_NewReturnsNonNil(t *testing.T) {
	r := newHTTPReporter("http://localhost:9999", "key", "repo", "epic")
	if r == nil {
		t.Fatal("newHTTPReporter returned nil")
	}
}

func TestHTTPReporter_ImplementsReporter(t *testing.T) {
	var r Reporter = newHTTPReporter("http://localhost:9999", "key", "repo", "")
	r.Send(Event{Type: EventSessionStarted})
	if err := r.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}
