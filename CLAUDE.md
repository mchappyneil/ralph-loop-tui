# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ralph-loop-wrapper is a Go TUI application that orchestrates an autonomous Claude agent ("Ralph") working through a Beads issue tracker. It provides real-time monitoring of Claude executing tasks in a loop until work is complete.

## Build & Run Commands

```bash
# Build
go build

# Run with defaults (50 iterations, 2s sleep)
./ralph-loop-go

# Run with custom settings
./ralph-loop-go -max-iterations 100 -sleep-seconds 5 -claude-bin /path/to/claude
```

**Flags:**
- `-max-iterations` (default: 50) - Maximum loop iterations
- `-sleep-seconds` (default: 2) - Sleep between iterations
- `-claude-bin` (default: "claude") - Path to Claude CLI

## Architecture

### Bubble Tea TUI Pattern

The app uses Charm's Bubble Tea framework with the Elm architecture:

1. **Model** (`model` struct) - Holds all application state including iteration count, status, viewport, and Claude process context
2. **Update** (`Update()`) - Message handler that processes keyboard input, iteration events, and Claude command results
3. **View** (`View()`) - Renders status bar and scrollable viewport

### Message Flow

```
startIterationMsg → runClaudeCmd() → claudeDoneMsg → [next iteration or finish]
                                                   ↓
                                    Check for <promise>COMPLETE</promise>
```

### Key Components

- **Status tracking**: `iterationStatus` enum (idle/running/completed/error/finished)
- **Viewport**: Scrollable content area showing iteration logs and Claude output
- **Context cancellation**: Graceful shutdown on q/ctrl+c/esc

### Ralph Agent Protocol

Each iteration runs Claude with a prompt that:
1. Uses Beads to find ready tasks (no blockers)
2. Implements ONE task
3. Runs tests/type checks
4. Closes task and commits if passing, or updates notes if failing
5. Outputs `<promise>COMPLETE</promise>` when no ready work remains

## Code Organization

Single `main.go` file (~330 lines) containing:
- Lines 1-50: Imports, types, constants
- Lines 51-150: Model initialization and Bubble Tea lifecycle (Init/Update/View)
- Lines 151-250: UI rendering (statusBar, appendLine)
- Lines 251-320: Claude execution (runClaudeCmd, buildPrompt)
- Lines 321-330: main() entry point

## Testing

No tests exist yet. When adding tests:
- Use `testing.Short()` for tests requiring external services (per user's CLAUDE.md)
- Test the message flow and state transitions
- Mock `exec.CommandContext` for Claude CLI interactions
