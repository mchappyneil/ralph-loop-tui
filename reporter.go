package main

// Reporter sends events to a ralph-hub server.
// Implementations must be safe to call from goroutines.
// Close must be called before the process exits to flush pending events.
type Reporter interface {
	Send(event Event)
	Close() error
}

// noopReporter is used when no hub URL is configured.
type noopReporter struct{}

func (n *noopReporter) Send(Event) {}
func (n *noopReporter) Close() error { return nil }
