# Reporter Redesign: Thin Pipe with Retry

**Date:** 2026-03-02
**Status:** Approved

## Problem

The current reporter has an 8-method interface, mutable internal state mirroring the model, a shared analytics pointer, a PrepareShutdown pattern for state consistency, and no retry logic. Events are fire-and-forget — network failures lose them permanently.

## Goals

1. **Simplicity**: Minimal interface, reporter is just a delivery pipe
2. **Retry with global replacement**: Exponential backoff, but new events supersede pending retries
3. **Always close**: session.ended is reliably delivered on normal exit, signal exit, or error

## Design

### Interface

```go
type Reporter interface {
    Send(event Event)  // non-blocking, replaces any pending retry
    Close() error      // blocks until final event delivered or timeout
}
```

Two implementations: `httpReporter` (delivery engine) and `noopReporter` (no-ops).

### Delivery Engine (httpReporter)

- Single-slot atomic pointer holds the "pending" event
- Dedicated sender goroutine loops: grab pending, try to send, retry with exponential backoff (100ms -> 200ms -> 400ms, capped at 5s)
- `Send()` atomically swaps the new event into the slot — sender picks up the new one on next iteration, dropping the old one
- No mutable session state in the reporter — no sessionID, status, currentPhase, analytics pointer

### Event Construction

Moves to the model layer. The model builds events from its own state:

```go
func (m *model) buildEventContext() EventContext { ... }

// Called at each event site:
m.reporter.Send(NewEvent(EventSessionStarted, m.instanceID, m.repo, m.epic,
    m.buildEventContext(), map[string]any{...}))
```

`NewEvent()` constructor in reporter_events.go generates UUID, timestamp, and assembles the Event struct.

### PrepareShutdown Elimination

No longer needed. The model sets its own state (e.g., `m.iterStatus = finished`) before building final events, so `buildEventContext()` naturally returns `status: "ended"`.

### Close() Behavior

1. Signal sender goroutine to enter drain mode (no new events accepted)
2. Retry pending event with tight backoff, up to 10 attempts
3. Block for at most 15 seconds
4. Return error only on timeout

### Signal Handling

```go
// In main.go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
go func() {
    <-sigCh
    reporter.Close()
    os.Exit(1)
}()
```

Ensures Close() runs even if Bubble Tea doesn't clean up.

### session.ended Guarantee

The model calls `endSession()` before `Close()`. Since session.ended is the last event Send()'d, it occupies the pending slot when Close() runs and gets retried aggressively.

Coverage:
- Normal exit: endSession() -> Close() -> retried delivery
- Signal exit: signal handler -> Close() -> retried delivery
- Panic: not recoverable

## Removals

- `PrepareShutdown()` method and concept
- `TaskClaimed()` / `TaskClosed()` interface methods (unused)
- Shared analytics pointer pattern
- All mutable state in httpReporter
- `SessionConfig` / `IterationResult` types (data goes into map[string]any)

## File Changes

| File | Change |
|------|--------|
| reporter.go | Interface: Send(Event) + Close(). Remove typed config structs. Keep noopReporter. |
| reporter_http.go | Rewrite: atomic single-slot, sender goroutine, retry backoff, Close() drain. |
| reporter_events.go | Keep event types and structs. Add NewEvent() constructor. |
| update.go | Replace 7 typed calls with Send(). Remove PrepareShutdown calls. |
| model.go | Add buildEventContext(). Add sessionID/repo/epic/instanceID fields. |
| main.go | Simplify reporter init. Add signal handler. |
| analytics.go | Add toEventAnalytics() helper. |
| reporter_test.go | Update to new interface. |
| reporter_http_test.go | Rewrite for retry/replacement behavior. |
| update_test.go | Update spyReporter and assertions. |
