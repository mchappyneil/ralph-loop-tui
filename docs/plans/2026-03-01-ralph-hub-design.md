# Ralph Hub — Centralized Reporting Dashboard

**Date**: 2026-03-01
**Status**: Design

## Overview

Ralph Hub is a centralized event hub and dashboard for monitoring multiple Ralph Loop instances across different repositories. It provides real-time visibility into what Ralph is doing, historical reporting, and configurable webhook integrations for notifications.

## Architecture

Two separate repositories:

1. **ralph-hub** (new) — Go API server + Next.js dashboard
2. **ralph-loop-go** (existing) — Gets a thin, opt-in reporting client

### Data Flow

```
Ralph Instance A ──POST /events──►  ┌─────────────┐  ◄──WS──► Next.js Dashboard
Ralph Instance B ──POST /events──►  │ Go API Hub   │  ◄──WS──► (browser clients)
Ralph Instance C ──POST /events──►  └──────┬──────┘
                                           │
                                    SQLite or Postgres
                                           │
                                    Webhook Dispatcher
                                      │    │    │
                                    Slack Discord Custom
```

## ralph-hub Repository

### Project Layout

```
ralph-hub/
├── cmd/hub/main.go          # Entry point, config loading
├── internal/
│   ├── server/              # HTTP server, routes, middleware, auth
│   ├── store/               # Store interface + SQLite/Postgres implementations
│   ├── ws/                  # WebSocket hub (fan-out to browser clients)
│   ├── webhook/             # Outbound webhook delivery + retry queue
│   └── events/              # Event types, validation
├── web/                     # Next.js dashboard app
│   ├── src/
│   │   ├── app/             # App router pages
│   │   ├── components/      # Dashboard components
│   │   └── hooks/           # useWebSocket, useInstances, etc.
│   └── package.json
├── migrations/              # SQL migrations (SQLite + Postgres variants)
└── config.yaml              # Hub configuration
```

### Event Schema

Every event carries a full context envelope so the dashboard can reconstruct state from any single event (handles mid-session connections).

```json
{
  "event_id": "evt_abc123",
  "type": "iteration.completed",
  "timestamp": "2026-03-01T14:23:00Z",
  "instance_id": "my-app/BD-42",
  "repo": "my-app",
  "epic": "BD-42",

  "data": {
    "iteration": 7,
    "duration_ms": 45000,
    "task_id": "BD-45",
    "passed": true,
    "notes": "Added auth middleware",
    "review_cycles": 1,
    "verdict": "APPROVED"
  },

  "context": {
    "session_id": "sess_xyz",
    "session_start": "2026-03-01T14:00:00Z",
    "max_iterations": 50,
    "current_iteration": 7,
    "status": "running",
    "current_phase": "dev",
    "analytics": {
      "passed_count": 6,
      "failed_count": 1,
      "tasks_closed": 6,
      "initial_ready": 12,
      "current_ready": 5,
      "avg_duration_ms": 42000,
      "total_duration_ms": 294000
    }
  }
}
```

### Event Types

| Event | When | Key Data |
|-------|------|----------|
| `session.started` | Ralph loop begins | repo, epic, max_iterations, config |
| `session.ended` | Loop finishes or killed | reason (complete/interrupted/error), summary stats |
| `iteration.started` | New iteration begins | iteration_number, phase |
| `iteration.completed` | Iteration done | duration, task_id, passed, notes, review_cycles, verdict |
| `phase.changed` | Pipeline phase transition | from_phase, to_phase |
| `task.claimed` | Ralph picks up a task | task_id, priority, description |
| `task.closed` | Task completed | task_id, commit_hash |

### API Endpoints

**Ingestion (Ralph → Hub)**:
- `POST /api/v1/events` — Ingest events (requires `Authorization: Bearer <api_key>`)

**Query (Dashboard → Hub)**:
- `GET /api/v1/instances` — List all instances with latest context snapshot
- `GET /api/v1/instances/:id/history` — Iteration history for an instance
- `GET /api/v1/sessions` — List sessions (filterable by repo, date range)
- `GET /api/v1/sessions/:id` — Session detail with all events
- `GET /api/v1/stats` — Aggregate stats across all projects

**Real-time**:
- `WS /api/v1/ws` — WebSocket for live dashboard updates

### Store Interface

```go
type Store interface {
    SaveEvent(ctx context.Context, event Event) error
    GetActiveInstances(ctx context.Context) ([]InstanceState, error)
    GetInstanceHistory(ctx context.Context, instanceID string, limit int) ([]IterationRecord, error)
    GetSessions(ctx context.Context, filter SessionFilter) ([]Session, error)
    GetSessionDetail(ctx context.Context, sessionID string) (*SessionDetail, error)
    GetAggregateStats(ctx context.Context) (*AggregateStats, error)
}
```

Two implementations: `sqliteStore` and `pgStore`, selected via configuration.

### Storage Configuration

```yaml
server:
  port: 8080

storage:
  driver: sqlite  # or postgres
  sqlite:
    path: ./ralph-hub.db
  postgres:
    dsn: postgres://user:pass@localhost/ralph_hub

auth:
  api_keys:
    - name: "my-laptop"
      key: "rhk_abc123..."

webhooks:
  - url: https://hooks.slack.com/services/xxx
    events: ["session.ended", "iteration.completed"]
    filter:
      passed_only: false
  - url: https://discord.com/api/webhooks/xxx
    events: ["session.ended"]
```

### WebSocket Hub

- Browser clients connect to `/api/v1/ws`
- When an event arrives via POST, the server stores it then broadcasts to all connected WS clients
- Clients can subscribe to specific instances or receive everything
- Handles reconnection gracefully on the client side

### Webhook Dispatcher

- Outbound webhooks configured in `config.yaml`
- Events filtered by type before dispatch
- Delivery with exponential backoff retries (max 3 attempts)
- Failed deliveries logged, never blocking event ingestion
- Webhook payloads match the event schema

## Dashboard UI (Next.js)

### Tech Stack

- Next.js with App Router
- Tailwind CSS
- Recharts or Tremor for charts
- Zustand for client state (WebSocket-driven updates)

### Pages

**Overview (`/`)** — Main dashboard
- Grid of cards, one per active Ralph instance
- Each card: repo name, epic, iteration progress, current phase, pass/fail rate, last task
- Real-time updates via WebSocket
- Inactive sessions collapsed below
- Color coding: green (healthy), yellow (recent failures), gray (ended)

**Instance Detail (`/instances/:id`)** — Drill into one Ralph
- Live phase indicator (planner → dev → reviewer → fixer)
- Iteration history table (iteration #, task, pass/fail, duration, review cycles, verdict)
- Charts: pass rate over time, duration trend, tasks remaining burndown

**Sessions History (`/sessions`)** — Historical view
- Table of all sessions across repos
- Columns: repo, epic, started, ended, iterations, tasks closed, pass rate
- Click through to session detail

**Session Detail (`/sessions/:id`)** — Past session deep dive
- Full timeline of events
- Summary stats and charts

**Settings (`/settings`)** — Webhook management
- View/add/edit webhook configurations
- Test webhook delivery
- Delivery logs

### Real-Time Flow

```
Browser ──WS connect──► Go Server
                            │
Ralph POST event ──────► Server stores event
                            │
                        Server broadcasts via WS
                            │
Browser receives ◄────── WS message
  └──► Zustand state update
       └──► React re-renders
```

On initial connect / reconnect, the dashboard fetches `GET /api/v1/instances` for full current state.

## ralph-loop-go Changes

### New Flags

```
-hub-url        URL of the ralph-hub server (default: "")
-hub-api-key    API key for authentication (default: "")
-instance-id    Instance identifier (default: derived from repo/epic)
```

Also settable via env vars: `RALPH_HUB_URL`, `RALPH_HUB_API_KEY`, `RALPH_INSTANCE_ID`.

### Instance ID Default

Derived from the git remote or directory name. If `-epic` is set, becomes `{repo-name}/{epic}` (e.g., `my-app/BD-42`). Overridable with `-instance-id`.

### Reporter Interface

New file `reporter.go`:

```go
type Reporter interface {
    SessionStarted(config SessionConfig) error
    SessionEnded(reason string) error
    IterationStarted(iteration int, phase string) error
    IterationCompleted(record IterationResult) error
    PhaseChanged(from, to string) error
    TaskClaimed(taskID, description string) error
    TaskClosed(taskID, commitHash string) error
}
```

Two implementations:
- `httpReporter` — sends events to the hub via HTTP POST with context envelope
- `noopReporter` — does nothing (used when no hub URL configured)

### Integration Points

- Reporter created in `main.go` based on flags, injected into the model
- Event calls added at transition points in `update.go`
- Events sent fire-and-forget in goroutines — never block the TUI
- Errors logged to stderr
- Reporter maintains a running context snapshot from the existing `analyticsData` struct

### What Doesn't Change

- All TUI behavior unchanged
- No new dependencies beyond stdlib `net/http`
- No changes to Bubble Tea message flow
- Reporter is purely observational — never drives state transitions

## Authentication

- Simple API key auth: `Authorization: Bearer <key>`
- Keys configured in hub's `config.yaml`
- One key per Ralph instance or shared across instances (operator's choice)
- No user accounts for the dashboard initially (it's a monitoring tool, not multi-tenant)

## Deployment

Designed to work locally or remotely:
- **Local**: `ralph-hub` on localhost, Ralph instances point to `http://localhost:8080`
- **Remote**: `ralph-hub` on a VPS/cloud, Ralph instances point to the public URL
- Single binary for the Go server (Next.js can be embedded or run separately)
