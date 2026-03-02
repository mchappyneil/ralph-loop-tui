package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// httpReporter implements Reporter by POSTing events to a ralph-hub server.
// It uses a single-slot atomic pointer for global replacement: only the most
// recent event is ever pending, and a dedicated sender goroutine retries with
// exponential backoff until delivery succeeds or Close drains.
type httpReporter struct {
	hubURL    string
	apiKey    string
	client    *http.Client
	pending   atomic.Pointer[Event]
	wake      chan struct{}
	done      chan struct{}
	closing   atomic.Bool
	closeOnce sync.Once
}

// newHTTPReporter creates an httpReporter that sends events to hubURL.
// It starts a background sender goroutine immediately.
func newHTTPReporter(hubURL, apiKey, _, _ string) *httpReporter {
	h := &httpReporter{
		hubURL: hubURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	go h.senderLoop()
	return h
}

// Send stores the event in the single slot and wakes the sender goroutine.
// It never blocks: if a previous event hasn't been sent yet, it is replaced.
func (h *httpReporter) Send(ev Event) {
	if h.closing.Load() {
		return
	}
	h.pending.Store(&ev)
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// Close signals the sender to drain any pending event and waits up to 15s
// for it to finish. Safe to call multiple times.
func (h *httpReporter) Close() error {
	var err error
	h.closeOnce.Do(func() {
		h.closing.Store(true)
		select {
		case h.wake <- struct{}{}:
		default:
		}
		select {
		case <-h.done:
		case <-time.After(15 * time.Second):
			err = fmt.Errorf("httpReporter: close timed out after 15s")
		}
	})
	return err
}

// senderLoop runs in a dedicated goroutine, waiting for wake signals and
// sending pending events with retry.
func (h *httpReporter) senderLoop() {
	defer close(h.done)
	for {
		<-h.wake

		ev := h.pending.Swap(nil)
		if ev == nil {
			if h.closing.Load() {
				return
			}
			continue
		}

		h.sendWithRetry(ev, 0)

		if h.closing.Load() {
			// Drain any final pending event before exiting.
			ev = h.pending.Swap(nil)
			if ev != nil {
				h.sendWithRetry(ev, 10)
			}
			return
		}
	}
}

// sendWithRetry attempts to deliver ev with exponential backoff.
// maxAttempts==0 means normal mode (unlimited retries until closing triggers drain).
// maxAttempts>0 means drain mode (bounded attempts, 2s backoff cap).
func (h *httpReporter) sendWithRetry(ev *Event, maxAttempts int) {
	backoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second
	if maxAttempts > 0 {
		maxBackoff = 2 * time.Second
	}

	for attempt := 1; ; attempt++ {
		if maxAttempts > 0 && attempt > maxAttempts {
			return
		}

		if h.doSend(ev) {
			return
		}

		// Global replacement: if a newer event arrived during the attempt,
		// switch to it and reset backoff.
		if newer := h.pending.Swap(nil); newer != nil {
			ev = newer
			backoff = 100 * time.Millisecond
		}

		// Transition to drain mode if Close was called during normal retry.
		if maxAttempts == 0 && h.closing.Load() {
			h.sendWithRetry(ev, 10)
			return
		}

		time.Sleep(backoff)
		backoff = min(backoff*2, maxBackoff)
	}
}

// doSend performs a single HTTP POST. Returns true if the event was accepted
// or the error is non-retryable (marshal failure, 4xx). Returns false for
// network errors and 5xx responses (retryable).
func (h *httpReporter) doSend(ev *Event) bool {
	body, err := json.Marshal(ev)
	if err != nil {
		return true // non-retryable
	}

	req, err := http.NewRequest("POST", h.hubURL+"/api/v1/events", bytes.NewReader(body))
	if err != nil {
		return true // non-retryable
	}

	req.Header.Set("Content-Type", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return false // network error, retryable
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return true // client error, non-retryable
	}
	return false // 5xx, retryable
}
