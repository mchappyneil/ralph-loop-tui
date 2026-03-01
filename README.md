# Ralph Loop

A Go terminal UI that runs Claude in an autonomous loop, working through issues until there's nothing left to do.

Ralph is an AI agent that finds ready tasks from a [Beads](https://github.com/steveyegge/beads) issue tracker, implements them one at a time, runs tests, and commits passing work—all without human intervention. This wrapper provides real-time visibility into what Ralph is doing.

Optionally, Ralph can report its progress to a [Ralph Loop Hub](https://github.com/fireynis/ralph-loop-go-hub) server for centralized monitoring across multiple agents and repositories.

## Demo

```
┌─────────────────────────────────────────────────────────────┐
│ Ralph TUI  [1] Homebase  [2] Output  [3] Analytics  q:quit │
├─────────────────────────────────────────────────────────────┤
│ Iteration: 3/50 | Status: running | Current: 1m23s          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ [Ralph status]                                              │
│ ready_before: 5                                             │
│ ready_after: 4                                              │
│ task: PROJ-12                                               │
│ tests: PASSED                                               │
│ notes: Added input validation to user registration          │
│                                                             │
│ ─────────────────────────────────────────────────────────── │
│ Starting iteration 4...                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Requirements

- [Go 1.21+](https://golang.org/dl/) (for building from source)
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude` in PATH)
- [Beads](https://github.com/steveyegge/beads) initialized in your project (`bd init`)

## Installation

### Quick install (Linux/macOS)

```bash
curl -sSfL https://raw.githubusercontent.com/fireynis/ralph-loop-tui/main/install.sh | sh
```

This downloads the latest release, verifies the checksum, and installs the binary to `/usr/local/bin` (or `~/.local/bin`).

### Go install

```bash
go install github.com/fireynis/ralph-loop-go@latest
```

### Build from source

```bash
git clone https://github.com/fireynis/ralph-loop-go.git
cd ralph-loop-go
go build -o ralph-loop-go
```

## Usage

```bash
# Run with defaults (50 iterations max, 2s between iterations)
ralph-loop-go

# Custom iteration limits
ralph-loop-go -max-iterations 100 -sleep-seconds 5

# Filter to a specific epic (prevents collision with other agents)
ralph-loop-go -epic BD-42

# Use a specific Claude binary
ralph-loop-go -claude-bin /usr/local/bin/claude

# Connect to Ralph Loop Hub for centralized monitoring
ralph-loop-go -hub-url https://your-hub.example.com -hub-api-key your-secret-key
```

### Flags

| Flag | Default | Env Variable | Description |
|------|---------|-------------|-------------|
| `-max-iterations` | `50` | | Maximum loop iterations before stopping |
| `-sleep-seconds` | `2` | | Pause between iterations (seconds) |
| `-claude-bin` | `claude` | | Path to Claude CLI executable |
| `-epic` | (none) | | Filter to tasks within a specific epic (e.g., `BD-42`) |
| `-max-review-cycles` | `3` | | Maximum reviewer/fixer cycles per iteration |
| `-hub-url` | (none) | `RALPH_HUB_URL` | URL of a [Ralph Loop Hub](https://github.com/fireynis/ralph-loop-go-hub) server |
| `-hub-api-key` | (none) | `RALPH_HUB_API_KEY` | API key for Ralph Loop Hub authentication |
| `-instance-id` | (derived) | `RALPH_INSTANCE_ID` | Instance identifier (defaults to `repo/epic`) |

Flags take precedence over environment variables. For hub configuration, environment variables are often more convenient so you don't leak secrets in shell history.

## TUI Screens

Ralph provides three switchable screens. Press `Tab` to cycle through them, or press `1`, `2`, or `3` to jump directly.

### 1. Homebase

Iteration logs and Ralph activity summary. Shows preflight census data (ready/blocked/in-progress counts and dependency graph) and per-iteration status updates as Ralph works.

### 2. Output

Live Claude output stream. Supports two view modes:

- **Parsed** (default) — Tool calls, results, and text extracted from Claude's stream-json output
- **Raw** — The full JSON lines as emitted by Claude

Press `r` to toggle between parsed and raw. Press `f` to toggle follow mode (auto-scroll to bottom).

### 3. Analytics

A four-panel dashboard showing:

- **Progress** — Iteration count, pass/fail rates, success percentage
- **Timing** — Total runtime, current/average/fastest/slowest iteration durations, estimated remaining time
- **Task Tracking** — Initial and current ready counts, tasks closed, last task worked on, hub connection status
- **Recent Iterations** — Table of the last 10 iterations with duration, verdict, review cycles, and task ID

### Controls

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` / `Esc` | Graceful shutdown |
| `Tab` | Next screen |
| `1` / `2` / `3` | Jump to Homebase / Output / Analytics |
| `↑` / `↓` / `PgUp` / `PgDn` | Scroll |
| `f` | Toggle follow mode (Output screen) |
| `r` | Toggle raw/parsed view (Output screen) |

## How It Works

### Iteration Pipeline

Each iteration runs a multi-phase pipeline:

1. **Planner** — Finds the highest-priority ready task, analyzes it, and produces a structured implementation plan (approach, files to touch, edge cases, test strategy). No code is written in this phase.

2. **Developer** — Receives the plan and implements exactly one task: modifies code, runs tests, commits if passing, or notes failures.

3. **Reviewer** — Examines the git diff as a specialist reviewer (auto-detected from file types—Go engineer, TypeScript/React engineer, Python engineer, etc.). Returns `APPROVED` or `CHANGES_REQUESTED`.

4. **Fixer** (if needed) — Addresses reviewer feedback, re-runs tests, and re-commits. The reviewer/fixer cycle repeats up to `-max-review-cycles` times.

### End Condition

After each iteration, Ralph checks for remaining work via `bd ready`. If no ready tasks remain, it outputs `<promise>COMPLETE</promise>` and the loop ends. The TUI also verifies this independently to catch cases where closing a task unblocks new dependent work.

### Error Handling

If Claude encounters 3 consecutive errors, the loop stops to avoid burning through iterations on a persistent problem.

## Connecting to Ralph Loop Hub

[Ralph Loop Hub](https://github.com/fireynis/ralph-loop-go-hub) is a separate server that provides centralized monitoring for one or more Ralph instances. When connected, Ralph sends real-time events so you can track progress from a dashboard without needing terminal access to each running agent.

### Setup

1. **Deploy a Ralph Loop Hub instance** — See the [Ralph Loop Hub README](https://github.com/fireynis/ralph-loop-go-hub) for setup instructions.

2. **Configure Ralph to report to it** — Either via flags or environment variables:

   ```bash
   # Via environment variables (recommended — avoids secrets in shell history)
   export RALPH_HUB_URL=https://your-hub.example.com
   export RALPH_HUB_API_KEY=your-secret-key
   ralph-loop-go

   # Via flags
   ralph-loop-go -hub-url https://your-hub.example.com -hub-api-key your-secret-key
   ```

3. **Verify the connection** — When hub reporting is enabled, you'll see a startup message:
   ```
   reporter: hub enabled → https://your-hub.example.com
   ```
   The Analytics screen (press `3`) also shows hub connection status, URL, and instance ID.

### Instance Identity

Each Ralph instance identifies itself to the hub using an instance ID. By default this is derived automatically from:

- The **git remote** origin URL (e.g., `fireynis/ralph-loop-go`)
- The **epic filter**, if set (appended as `/BD-42`)

So a Ralph running on `fireynis/my-project` with `-epic BD-42` would have instance ID `fireynis/my-project/BD-42`.

You can override this with `-instance-id` or `RALPH_INSTANCE_ID` if you need a custom identifier.

### Events Reported

Ralph sends the following events to the hub, each including a full context snapshot (session ID, iteration count, analytics summary) so the dashboard can reconstruct state from any single event:

| Event | When |
|-------|------|
| `session.started` | Loop begins, includes configuration (max iterations, sleep, epic, review cycles) |
| `session.ended` | Loop finishes, includes reason (completed, interrupted, max iterations reached) |
| `iteration.started` | Each iteration begins, includes iteration number and phase |
| `iteration.completed` | Each iteration ends, includes duration, task ID, pass/fail, review cycles |
| `phase.changed` | Pipeline phase transitions (planner → dev → reviewer → fixer) |
| `task.claimed` | Ralph picks up a task, includes task ID and description |
| `task.closed` | Ralph closes a completed task, includes task ID and commit hash |

Events are sent as fire-and-forget HTTP POSTs to `{hub-url}/api/v1/events` with a Bearer token for authentication. Failed sends are logged to stderr but never block the TUI — Ralph continues working regardless of hub availability.

### Running Multiple Agents

To run multiple Ralph instances on the same repo without task collisions, use the `-epic` flag to scope each agent to a different epic:

```bash
# Terminal 1: Ralph works on the backend epic
ralph-loop-go -epic BD-10 -hub-url https://hub.example.com -hub-api-key key123

# Terminal 2: Ralph works on the frontend epic
ralph-loop-go -epic BD-20 -hub-url https://hub.example.com -hub-api-key key123
```

Each instance gets a unique instance ID (`repo/BD-10`, `repo/BD-20`) and reports independently to the hub.

## Example Workflow

```bash
# 1. Initialize Beads in your project
bd init

# 2. Create some tasks
bd create --title="Add input validation to login form" --priority=1
bd create --title="Write tests for user service" --priority=2
bd create --title="Update README with API examples" --priority=3

# 3. Let Ralph work through them
ralph-loop-go

# 4. Or with hub monitoring
export RALPH_HUB_URL=https://your-hub.example.com
export RALPH_HUB_API_KEY=your-secret-key
ralph-loop-go -max-iterations 100
```

## Architecture

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) using the Elm architecture:

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    Model     │────▶│    Update    │────▶│     View     │
│  (state)     │◀────│  (messages)  │◀────│   (render)   │
└──────────────┘     └──────────────┘     └──────────────┘
                            │
                            ▼
                  ┌───────────────────┐
                  │  Phase Pipeline   │
                  │ planner → dev →   │
                  │ reviewer → fixer  │
                  └───────────────────┘
                            │
                            ▼
                  ┌───────────────────┐
                  │    Reporter       │
                  │ (HTTP → Hub)      │
                  └───────────────────┘
```

**Message flow:**
```
preflight → startIterationMsg → planner → dev → reviewer → [fixer →] claudeDoneMsg → next iteration or finish
                                                                         │
                                                          events sent to hub (async)
```

## Configuration

Ralph expects Claude CLI with these capabilities:
- `--dangerously-skip-permissions` for autonomous operation
- `--output-format stream-json` for real-time output
- Access to Beads CLI (`bd`) commands

## Related Projects

- [Ralph Loop Hub](https://github.com/fireynis/ralph-loop-go-hub) — Centralized monitoring dashboard for Ralph instances
- [Beads](https://github.com/steveyegge/beads) — Git-backed issue tracker Ralph uses for task management
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) — The AI that does the actual work
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework

## License

MIT
