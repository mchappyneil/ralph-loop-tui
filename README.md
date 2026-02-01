# Ralph Loop

A Go terminal UI that runs Claude in an autonomous loop, working through issues until there's nothing left to do.

Ralph is an AI agent that finds ready tasks from a [Beads](https://github.com/steveyegge/beads) issue tracker, implements them one at a time, runs tests, and commits passing work—all without human intervention. This wrapper provides real-time visibility into what Ralph is doing.

## Demo

```
┌─────────────────────────────────────────────────────────────┐
│ Ralph Loop | Iteration 3/50 | Status: running               │
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

- [Go 1.21+](https://golang.org/dl/)
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude` in PATH)
- [Beads](https://github.com/steveyegge/beads) initialized in your project (`bd init`)

## Installation

```bash
go install github.com/fireynis/ralph-loop-go@latest
```

Or build from source:

```bash
git clone https://github.com/fireynis/ralph-loop-go.git
cd ralph-loop-go
go build -o ralph-loop-go
```

## Usage

```bash
# Run with defaults (50 iterations max, 2s between iterations)
ralph-loop-go

# Custom settings
ralph-loop-go -max-iterations 100 -sleep-seconds 5

# Filter to a specific epic (prevents collision with other agents)
ralph-loop-go -epic BD-42

# Use a specific Claude binary
ralph-loop-go -claude-bin /usr/local/bin/claude
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-max-iterations` | 50 | Maximum loop iterations before stopping |
| `-sleep-seconds` | 2 | Pause between iterations |
| `-claude-bin` | `claude` | Path to Claude CLI executable |
| `-epic` | (none) | Filter to tasks within a specific epic (e.g., `BD-42`) |

### Controls

- `q` / `Ctrl+C` / `Esc` — Graceful shutdown
- `↑` / `↓` / `PgUp` / `PgDn` — Scroll output

## How It Works

Each iteration, Ralph:

1. **Finds work** — Runs `bd ready --json` to get unblocked tasks (filtered by epic if `-epic` is set), picks the highest priority one
2. **Implements it** — Makes focused code changes to satisfy the task
3. **Runs tests** — Executes the project's test suite
4. **Commits or notes** — If tests pass: closes the task and commits. If tests fail: updates the task with what went wrong
5. **Checks for more** — If no ready tasks remain, outputs `<promise>COMPLETE</promise>` and the loop ends

The wrapper streams Claude's output in real-time so you can watch progress.

## Example Workflow

```bash
# 1. Initialize Beads in your project
bd init

# 2. Create some tasks
bd create "Add input validation to login form" --priority P1
bd create "Write tests for user service" --priority P2
bd create "Update README with API examples" --priority P3

# 3. Let Ralph work through them
ralph-loop-go
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
                     ┌──────────────┐
                     │  runClaudeCmd │
                     │  (async exec) │
                     └──────────────┘
```

**Message flow:**
```
startIterationMsg → runClaudeCmd() → claudeOutputLineMsg (streaming) → claudeDoneMsg → next iteration or finish
```

## Configuration

Ralph expects Claude CLI with these capabilities:
- `--dangerously-skip-permissions` for autonomous operation
- `--output-format stream-json` for real-time output
- Access to Beads CLI (`bd`) commands

## Related Projects

- [Beads](https://github.com/steveyegge/beads) — Git-backed issue tracker Ralph uses for task management
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) — The AI that does the actual work
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework

## License

MIT
