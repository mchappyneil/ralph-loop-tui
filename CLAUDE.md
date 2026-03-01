# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ralph-loop-go (`github.com/fireynis/ralph-loop-go`) is a Go TUI application that orchestrates an autonomous Claude agent ("Ralph") working through a Beads issue tracker. It provides real-time monitoring of Claude executing tasks in a loop until work is complete. Go version: 1.25.5.

## Build & Run Commands

```bash
# Build
go build -o ralph-loop-go

# Lint (run after any Go code changes)
golangci-lint run

# Run with defaults (50 iterations, 2s sleep)
./ralph-loop-go

# Run with custom settings
./ralph-loop-go -max-iterations 100 -sleep-seconds 5 -claude-bin /path/to/claude

# Filter to specific epic (prevents collision with other agents)
./ralph-loop-go -epic BD-42
```

**Flags:**
- `-max-iterations` (default: 50) - Maximum loop iterations
- `-sleep-seconds` (default: 2) - Sleep between iterations
- `-claude-bin` (default: "claude") - Path to Claude CLI
- `-epic` (default: "") - Filter to tasks within a specific epic
- `-max-review-cycles` (default: 3) - Maximum reviewer/fixer cycles per iteration
- `-hub-url` (default: "", env: RALPH_HUB_URL) - URL of ralph-hub server for centralized reporting
- `-hub-api-key` (default: "", env: RALPH_HUB_API_KEY) - API key for ralph-hub authentication
- `-instance-id` (default: derived from repo/epic, env: RALPH_INSTANCE_ID) - Instance identifier

**Dependencies:** Vendored in `vendor/`. After modifying `go.mod`, run `go mod vendor` to update.

## Architecture

### Bubble Tea TUI Pattern

The app uses Charm's Bubble Tea framework with the Elm architecture:

1. **Model** (`model.go`) - All application state including iteration tracking, screen state, viewports, analytics, and Claude execution context
2. **Update** (`update.go`) - Message handler for keyboard input, iteration events, and Claude command results
3. **View** (`view.go`) - Renders tab bar, status bar, screen content, and help bar

### Message Flow

```
startIterationMsg → runClaudeCmd() → claudeOutputLineMsg (streaming) → claudeDoneMsg → [next iteration or finish]
                                                                        ↓
                                                         Check for <promise>COMPLETE</promise>
```

### Screen Architecture

Three switchable screens (Tab/1/2/3 to navigate):

1. **Homebase** (`screens/homebase.go`) - Iteration logs and Ralph activity summary
2. **Output** (`screens/output.go`) - Claude output with raw/parsed toggle (r key) and follow mode (f key)
3. **Analytics** (`screens/analytics.go`) - Dashboard with progress, timing, task tracking, and iteration history

### Key Components

- **Status tracking**: `iterationStatus` enum (idle/running/completed/error/finished)
- **JSON parsing** (`jsonparser.go`): Parses Claude's stream-json output, extracts tool calls/results/text
- **Analytics** (`analytics.go`): Tracks session statistics, iteration history, duration metrics
- **Styles** (`styles.go`): Centralized lipgloss styling definitions
- **Context cancellation**: Graceful shutdown on q/ctrl+c/esc

### Ralph Agent Protocol

Each iteration runs Claude with a prompt that:
1. Uses Beads to find ready tasks (no blockers), optionally filtered by epic
2. Implements ONE task
3. Runs tests/type checks
4. Closes task and commits if passing, or updates notes if failing
5. Outputs `<promise>COMPLETE</promise>` when no ready work remains

Ralph must output a status block at the end of each response:
```
[Ralph status]
ready_before: <count>
ready_after: <count>
task: <task-id>
tests: PASSED|FAILED
notes: <summary>
```

## Code Organization

```
├── main.go           # Entry point, CLI flags, runClaudeCmd, buildPrompt
├── model.go          # Model struct, message types, initialModel
├── update.go         # Init(), Update() message handler
├── view.go           # View() rendering, tab/status/help bars
├── styles.go         # lipgloss style definitions
├── analytics.go      # analyticsData, iterationRecord, RalphStatus parsing
├── jsonparser.go     # Claude stream-json parsing (ParseStreamLine, ExtractFullText)
├── reporter.go       # Reporter interface, noopReporter, SessionConfig, IterationResult
├── reporter_events.go # Event types and JSON envelope for ralph-hub
├── reporter_http.go  # httpReporter: fire-and-forget HTTP POST to ralph-hub
├── instance_id.go    # Instance ID derivation from git remote / directory / epic
└── screens/
    ├── homebase.go   # Homebase screen renderer
    ├── output.go     # Output screen renderer with follow/raw toggles
    └── analytics.go  # Analytics dashboard with 4-panel layout
```

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run short tests only (skips tests requiring external services)
go test -short ./...
```

Test files exist for analytics, reporter, reporter events, reporter HTTP, instance ID, and main. When adding new tests:
- Use `testing.Short()` for tests requiring external services
- Test the message flow and state transitions
- Mock `exec.CommandContext` for Claude CLI interactions
- Test JSON parsing with sample stream-json output
