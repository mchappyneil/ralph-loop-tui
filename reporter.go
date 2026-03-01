package main

import "time"

// SessionConfig holds the configuration sent with session.started events.
type SessionConfig struct {
	MaxIterations   int    `json:"max_iterations"`
	SleepSeconds    int    `json:"sleep_seconds"`
	Epic            string `json:"epic,omitempty"`
	MaxReviewCycles int    `json:"max_review_cycles"`
}

// IterationResult holds the outcome of a completed iteration.
type IterationResult struct {
	Iteration    int           `json:"iteration"`
	Duration     time.Duration `json:"duration"`
	TaskID       string        `json:"task_id"`
	Passed       bool          `json:"passed"`
	Notes        string        `json:"notes"`
	ReviewCycles int           `json:"review_cycles"`
	FinalVerdict string        `json:"final_verdict"`
}

// Reporter sends events to a ralph-hub server.
// Implementations must be safe to call from goroutines.
// Close must be called before the process exits to flush pending events.
type Reporter interface {
	SessionStarted(config SessionConfig) error
	SessionEnded(reason string) error
	IterationStarted(iteration int, phase string) error
	IterationCompleted(result IterationResult) error
	PhaseChanged(from, to string) error
	TaskClaimed(taskID, description string) error
	TaskClosed(taskID, commitHash string) error
	Close() error
}

// noopReporter is used when no hub URL is configured. All methods are no-ops.
type noopReporter struct{}

func (n *noopReporter) SessionStarted(SessionConfig) error       { return nil }
func (n *noopReporter) SessionEnded(string) error                { return nil }
func (n *noopReporter) IterationStarted(int, string) error       { return nil }
func (n *noopReporter) IterationCompleted(IterationResult) error { return nil }
func (n *noopReporter) PhaseChanged(string, string) error        { return nil }
func (n *noopReporter) TaskClaimed(string, string) error         { return nil }
func (n *noopReporter) TaskClosed(string, string) error          { return nil }
func (n *noopReporter) Close() error                             { return nil }
