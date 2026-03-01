# Ralph Loop Hub Integration Guide

This guide covers how to connect Ralph Loop to a [Ralph Loop Hub](https://github.com/fireynis/ralph-loop-go-hub) server for centralized monitoring.

## Overview

Ralph Loop Hub is a separate server that collects real-time events from one or more Ralph instances. It provides a centralized dashboard so you can monitor agent progress across multiple repositories and epics without needing terminal access to each running agent.

Without the hub, Ralph operates identically — the hub is purely opt-in observability.

## Prerequisites

- A running Ralph Loop Hub instance — see the [Ralph Loop Hub README](https://github.com/fireynis/ralph-loop-go-hub) for deployment instructions
- An API key configured on the hub server
- Network connectivity from the machine running Ralph to the hub server

## Configuration

Ralph accepts hub configuration through either CLI flags or environment variables. Flags take precedence over environment variables.

### Environment Variables (Recommended)

Using environment variables avoids leaking secrets in shell history:

```bash
export RALPH_HUB_URL=https://your-hub.example.com
export RALPH_HUB_API_KEY=your-secret-key

ralph-loop-go
```

You can add these to your `.bashrc`, `.zshrc`, or a `.env` file that you source before running Ralph.

### CLI Flags

```bash
ralph-loop-go \
  -hub-url https://your-hub.example.com \
  -hub-api-key your-secret-key
```

### Configuration Reference

| Setting | Flag | Env Variable | Default | Description |
|---------|------|-------------|---------|-------------|
| Hub URL | `-hub-url` | `RALPH_HUB_URL` | (none) | Base URL of the hub server. When empty, hub reporting is disabled and Ralph runs with a no-op reporter. |
| API Key | `-hub-api-key` | `RALPH_HUB_API_KEY` | (none) | Bearer token sent in the `Authorization` header with every event POST. |
| Instance ID | `-instance-id` | `RALPH_INSTANCE_ID` | (derived) | Unique identifier for this Ralph instance. See [Instance Identity](#instance-identity). |

## Verifying the Connection

When hub reporting is enabled, Ralph logs a startup message to stderr:

```
reporter: hub enabled → https://your-hub.example.com
```

When disabled (no URL configured):

```
reporter: hub disabled (no RALPH_HUB_URL)
```

You can also check the **Analytics** screen (press `3` in the TUI). The "Hub" section in the Task Tracking panel shows:

- **Status**: `enabled` (green) or `disabled` (red)
- **URL**: The configured hub URL
- **Instance**: The resolved instance ID

## Instance Identity

Each Ralph instance identifies itself to the hub with an instance ID. This ID is used by the hub to group events and distinguish between different agents.

### Automatic Derivation

By default, the instance ID is derived from:

1. The **git remote** `origin` URL — parsed into `owner/repo` format (e.g., `fireynis/ralph-loop-go`)
2. The **epic filter** — if `-epic` is set, it's appended with a `/` separator

Examples:
| Git Remote | Epic | Instance ID |
|-----------|------|-------------|
| `git@github.com:fireynis/my-project.git` | (none) | `fireynis/my-project` |
| `https://github.com/fireynis/my-project.git` | `BD-42` | `fireynis/my-project/BD-42` |
| (no remote) | (none) | `my-project` (directory name) |
| (no remote, no git) | (none) | `unknown` |

### Manual Override

Set a custom instance ID when the automatic derivation doesn't suit your needs:

```bash
# Via environment variable
export RALPH_INSTANCE_ID=production-backend-agent

# Via flag
ralph-loop-go -instance-id production-backend-agent
```

## Events

Ralph sends events as JSON HTTP POSTs to `{hub-url}/api/v1/events`. Each event includes an `Authorization: Bearer {api-key}` header.

### Event Types

| Event Type | Trigger | Key Data Fields |
|-----------|---------|-----------------|
| `session.started` | Loop begins | `max_iterations`, `sleep_seconds`, `epic`, `max_review_cycles` |
| `session.ended` | Loop finishes | `reason` (completed, interrupted, max iterations) |
| `iteration.started` | Iteration begins | `iteration`, `phase` |
| `iteration.completed` | Iteration ends | `iteration`, `duration_ms`, `task_id`, `passed`, `notes`, `review_cycles`, `final_verdict` |
| `phase.changed` | Pipeline phase transitions | `from`, `to` (planner, dev, reviewer, fixer) |
| `task.claimed` | Ralph picks up a task | `task_id`, `description` |
| `task.closed` | Ralph closes a completed task | `task_id`, `commit_hash` |

### Event Envelope

Every event is wrapped in a standard envelope:

```json
{
  "event_id": "uuid-v4",
  "type": "iteration.completed",
  "timestamp": "2026-03-01T12:00:00Z",
  "instance_id": "fireynis/my-project/BD-42",
  "repo": "fireynis/my-project",
  "epic": "BD-42",
  "data": {
    "iteration": 3,
    "duration_ms": 45000,
    "task_id": "beads-abc123",
    "passed": true,
    "notes": "Added input validation",
    "review_cycles": 1,
    "final_verdict": "APPROVED"
  },
  "context": {
    "session_id": "uuid-v4",
    "session_start": "2026-03-01T11:55:00Z",
    "max_iterations": 50,
    "current_iteration": 3,
    "status": "running",
    "current_phase": "dev",
    "analytics": {
      "passed_count": 3,
      "failed_count": 0,
      "tasks_closed": 3,
      "initial_ready": 8,
      "current_ready": 5,
      "avg_duration_ms": 42000,
      "total_duration_ms": 126000
    }
  }
}
```

The `context` field is a snapshot of the Ralph instance's current state, attached to every event. This allows the hub dashboard to reconstruct the full state from any single event, which is important for handling mid-session connections or missed events.

### Delivery Semantics

- **Fire-and-forget**: Events are sent in background goroutines and never block the TUI event loop.
- **No retries**: If an event fails to send (network error, hub down), it is logged to stderr and dropped. Ralph continues working regardless.
- **10-second timeout**: Each HTTP request has a 10-second timeout to prevent hanging connections.
- **Error logging**: Failed sends and non-2xx responses are logged to stderr with the event type and response body.

This means hub connectivity is best-effort. Ralph's core functionality (finding tasks, implementing, testing, committing) is completely independent of the hub.

## Running Multiple Agents

When running multiple Ralph instances on the same repository, use the `-epic` flag to scope each agent to different work:

```bash
# Terminal 1: Backend work
ralph-loop-go -epic BD-10 -hub-url https://hub.example.com -hub-api-key key123

# Terminal 2: Frontend work
ralph-loop-go -epic BD-20 -hub-url https://hub.example.com -hub-api-key key123

# Terminal 3: Documentation work
ralph-loop-go -epic BD-30 -hub-url https://hub.example.com -hub-api-key key123
```

Each instance automatically gets a unique instance ID (`repo/BD-10`, `repo/BD-20`, `repo/BD-30`) and reports independently to the hub. The hub dashboard can then show all three agents side by side.

Without epic scoping, multiple agents on the same repo may pick up the same task simultaneously, leading to conflicts.

## Troubleshooting

### "reporter: hub disabled (no RALPH_HUB_URL)"

The hub URL is not configured. Set `RALPH_HUB_URL` or pass `-hub-url`.

### "reporter: send error: ..."

Ralph can't reach the hub server. Check:
- Is the hub URL correct and reachable from this machine?
- Is there a firewall blocking the connection?
- Is the hub server running?

Ralph will continue working — this only affects reporting.

### "reporter: hub returned 401 ..."

The API key is invalid or missing. Check that `RALPH_HUB_API_KEY` or `-hub-api-key` matches the key configured on the hub server.

### "reporter: hub returned 4xx/5xx ..."

The hub rejected the event. The response body is logged to stderr (up to 1024 bytes). Check the hub server logs for details.

### Analytics screen shows "Hub: disabled" but I set the URL

Make sure the environment variable is exported (not just set) and that no typo exists. The flag takes precedence, so if you pass `-hub-url ""` it will override the env var with empty.
