package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// httpReporter implements Reporter by POSTing events to a ralph-hub server.
// All sends happen in goroutines so they never block the Bubble Tea event loop.
// Call Close() before process exit to flush pending events.
type httpReporter struct {
	hubURL     string
	apiKey     string
	instanceID string
	repo       string
	epic       string
	client     *http.Client

	// Mutable context state, updated by each Reporter method call.
	sessionID        string
	sessionStart     time.Time
	maxIterations    int
	currentIteration int
	status           string
	currentPhase     string
	analytics        *analyticsData

	// wg tracks in-flight send goroutines so Close() can wait for them.
	wg sync.WaitGroup
}

// newHTTPReporter creates an httpReporter that sends events to hubURL.
// It derives the instance ID from repo/epic and generates a fresh session ID.
func newHTTPReporter(hubURL, apiKey, repo, epic string) *httpReporter {
	return &httpReporter{
		hubURL:       hubURL,
		apiKey:       apiKey,
		instanceID:   deriveInstanceID(repo, epic),
		repo:         repo,
		epic:         epic,
		client:       &http.Client{Timeout: 10 * time.Second},
		sessionID:    uuid.New().String(),
		sessionStart: time.Now(),
		analytics:    &analyticsData{},
	}
}

func (h *httpReporter) SessionStarted(config SessionConfig) error {
	h.maxIterations = config.MaxIterations
	h.status = "running"
	h.send(h.buildEvent(EventSessionStarted, map[string]any{
		"max_iterations":    config.MaxIterations,
		"sleep_seconds":     config.SleepSeconds,
		"epic":              config.Epic,
		"max_review_cycles": config.MaxReviewCycles,
	}))
	return nil
}

func (h *httpReporter) SessionEnded(reason string) error {
	h.status = "ended"
	h.send(h.buildEvent(EventSessionEnded, map[string]any{
		"reason": reason,
	}))
	return nil
}

// PrepareShutdown sets status="ended" and currentPhase so that all subsequent
// events carry the final context. Must be called before any completion-batch
// events (IterationCompleted, PhaseChanged, SessionEnded) are fired.
func (h *httpReporter) PrepareShutdown(phase string) {
	h.status = "ended"
	h.currentPhase = phase
}

func (h *httpReporter) IterationStarted(iteration int, phase string) error {
	h.currentIteration = iteration
	h.currentPhase = phase
	h.send(h.buildEvent(EventIterationStarted, map[string]any{
		"iteration": iteration,
		"phase":     phase,
	}))
	return nil
}

func (h *httpReporter) IterationCompleted(result IterationResult) error {
	h.send(h.buildEvent(EventIterationCompleted, map[string]any{
		"iteration":     result.Iteration,
		"duration_ms":   result.Duration.Milliseconds(),
		"task_id":       result.TaskID,
		"passed":        result.Passed,
		"notes":         result.Notes,
		"review_cycles": result.ReviewCycles,
		"final_verdict": result.FinalVerdict,
	}))
	return nil
}

func (h *httpReporter) PhaseChanged(from, to string) error {
	h.currentPhase = to
	h.send(h.buildEvent(EventPhaseChanged, map[string]any{
		"from": from,
		"to":   to,
	}))
	return nil
}

func (h *httpReporter) TaskClaimed(taskID, description string) error {
	h.send(h.buildEvent(EventTaskClaimed, map[string]any{
		"task_id":     taskID,
		"description": description,
	}))
	return nil
}

func (h *httpReporter) TaskClosed(taskID, commitHash string) error {
	h.send(h.buildEvent(EventTaskClosed, map[string]any{
		"task_id":     taskID,
		"commit_hash": commitHash,
	}))
	return nil
}

func (h *httpReporter) Send(ev Event) {
	h.send(ev)
}

// buildEvent constructs a full Event envelope with a context snapshot.
func (h *httpReporter) buildEvent(eventType EventType, data map[string]any) Event {
	return Event{
		ID:         uuid.New().String(),
		Type:       eventType,
		Timestamp:  time.Now(),
		InstanceID: h.instanceID,
		Repo:       h.repo,
		Epic:       h.epic,
		Data:       data,
		Context:    h.buildContext(),
	}
}

// buildContext creates an EventContext snapshot from the reporter's current state.
func (h *httpReporter) buildContext() EventContext {
	ctx := EventContext{
		SessionID:        h.sessionID,
		SessionStart:     h.sessionStart,
		MaxIterations:    h.maxIterations,
		CurrentIteration: h.currentIteration,
		Status:           h.status,
		CurrentPhase:     h.currentPhase,
	}
	if h.analytics != nil {
		ctx.Analytics = EventAnalytics{
			PassedCount:     h.analytics.passedCount,
			FailedCount:     h.analytics.failedCount,
			TasksClosed:     h.analytics.tasksClosed,
			InitialReady:    h.analytics.initialReady,
			CurrentReady:    h.analytics.currentReady,
			AvgDurationMs:   h.analytics.avgDuration().Milliseconds(),
			TotalDurationMs: h.analytics.totalDuration().Milliseconds(),
		}
	}
	return ctx
}

// send fires the event to the hub in a goroutine. Errors are logged to stderr
// but never returned — the caller has already moved on.
// The goroutine is tracked by h.wg so Close() can wait for delivery.
func (h *httpReporter) send(ev Event) {
	body, err := json.Marshal(ev)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reporter: marshal error: %v\n", err)
		return
	}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.doSend(body, ev.Type)
	}()
}

// doSend performs the actual HTTP POST. Shared by send (async) and sendSync.
func (h *httpReporter) doSend(body []byte, eventType EventType) {
	req, err := http.NewRequest(http.MethodPost, h.hubURL+"/api/v1/events", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "reporter: request error: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reporter: send error: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		fmt.Fprintf(os.Stderr, "reporter: hub returned %s for %s: %s\n", resp.Status, eventType, respBody)
	}
}

// Close waits for all pending send goroutines to complete, with a timeout.
// Must be called before the process exits to guarantee event delivery.
func (h *httpReporter) Close() error {
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(15 * time.Second):
		return fmt.Errorf("reporter: timed out waiting for %s pending events", h.hubURL)
	}
}
