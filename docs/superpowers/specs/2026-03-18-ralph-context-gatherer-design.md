# Ralph Context Gatherer Design

Date: 2026-03-18

## Overview

Replace the current Planner phase with a Context Gatherer phase. The Planner's original goal was to analyze a task and produce an implementation plan before the dev writes code. Since tickets already carry their own context and plan, the Planner is redundant for that purpose. The Context Gatherer takes its place with a different mandate: understand how this codebase does things, so the dev writes code that is idiomatic and fits the repo rather than just functional.

## Pipeline

```
Context Gatherer → Dev → Reviewer → Fixer
```

The Planner prompt (`buildPlannerPrompt`) and its invocation in the update loop are replaced by a Context Gatherer prompt (`buildContextGathererPrompt`). Dev, Reviewer, and Fixer prompt functions have their `plannerOutput` parameter renamed to `gathererOutput`, and any internal prompt text referencing "the original implementation plan" is updated to "the codebase context gathered for this task."

## Context Gatherer Phase

### Responsibilities

1. Find the next ready task via `bd ready` and read it with `bd show <T> --json`
2. Read the instance-scoped cache file (`.ralph-context-<instanceID>.md`) if it exists
3. If the cache covers the task's domain well enough: use cached context, skip research
4. If the cache does not cover the task's domain: research docs and code, write updated cache to disk, then output pattern summary
5. Output a concise pattern summary for the dev

### Research Sources (when cache miss)

Two sources, with recency as the tiebreaker:

- **Docs**: CLAUDE.md, AGENTS.md, README, anything under `docs/`
- **Existing code**: similar files/functions to what the task requires

When docs and code contradict each other, prefer whichever is more recent. If docs describe a new pattern the code hasn't caught up to yet, follow docs. If docs are stale and the codebase has clearly evolved a different approach, follow the code.

### Cache Write Mechanism

The Context Gatherer runs as a Claude invocation with `--dangerously-skip-permissions`, so it has shell tool access and writes the cache file directly to disk during its invocation (e.g., via `bash` tool calls writing to `.ralph-context-<instanceID>.md`). The Go host does not parse gatherer output to write the cache — the file is entirely managed by Claude's shell operations within the gatherer phase.

### Output Format

The gatherer outputs this block at the end of its response. The Go host does not parse `cache_hit` — it is informational only for operator visibility in the TUI.

```
[Context Gatherer output]
task: <T>
cache_hit: full|partial|none
patterns:
<concise summary of relevant patterns, conventions, and files to follow>
```

`cache_hit` values:
- `full` — all relevant domains were found in cache, no research performed
- `partial` — some domains were cached, fresh research done for uncovered areas
- `none` — no relevant cache found, full research performed

### Prompt Structure

`buildContextGathererPrompt(epic, instanceID string) string` instructs Claude to:

1. Run `bd ready` (with optional epic filter), pick the highest-priority task T, run `bd show <T> --json`
2. Read the cache file at `.ralph-context-<instanceID>.md` if it exists
3. Assess: does the cache cover the domains this task touches?
4. If cache is sufficient: skip to step 6 using cached patterns
5. If cache is insufficient: read relevant docs (CLAUDE.md, AGENTS.md, README, docs/) and find similar existing code. Then write updated cache back to `.ralph-context-<instanceID>.md` (append new sections or update stale ones with a "Last updated" note)
6. Output the `[Context Gatherer output]` block with task ID, cache_hit status, and pattern summary

The `instanceID` parameter is used to compute the absolute cache file path (via `ralphContextCachePath`) and inject it into the prompt so Claude uses the same path the Go host does.

## Cache File

### Naming

`.ralph-context-<instanceID>.md`

The instance ID is already derived from repo name + epic filter (see `instance_id.go`). This ensures multiple Ralph instances running on different epics never share or overwrite each other's cache.

### Format

Markdown with sections by codebase domain. Each section records the patterns found and when they were last updated.

```markdown
# Ralph Context Cache
Instance: <instanceID>

## <Domain: e.g., HTTP Handlers>
Last updated: <date>
<pattern summary>

## <Domain: e.g., Testing>
Last updated: <date>
<pattern summary>
```

### Location

Repo root. Add `/.ralph-context-*.md` to the repo's `.gitignore` as a one-time committed change in this PR (not runtime behavior — the Go host does not modify `.gitignore`).

### Lifecycle

- **On loop start**: delete any existing cache file for this instance ID. This happens synchronously in `main()` before `tea.NewProgram` is called, via `os.Remove(ralphContextCachePath(derivedID))` (error ignored if file does not exist). Stale context from a prior run is worse than no context.
- **On COMPLETE**: delete the cache file when the loop finishes. COMPLETE detection happens in `update.go` where `<promise>COMPLETE</promise>` is scanned. The cleanup call (`os.Remove(ralphContextCachePath(m.instanceID))`) is added at that detection site in `update.go`.
- **During loop**: Claude writes updated cache during the gatherer phase; subsequent iterations reuse accumulated knowledge

## Implementation Notes

### New / changed functions

**`main.go`**
- `buildContextGathererPrompt(epic, instanceID string) string` — replaces `buildPlannerPrompt`
- `buildDevPrompt(epic, gathererOutput string) string` — `plannerOutput` parameter renamed to `gathererOutput`; internal prompt text updated from "Here is your implementation plan:" to "Here is the codebase context gathered for this task:"
- `buildReviewerPrompt(gathererOutput, diff, specialist string) string` — `plannerOutput` parameter renamed to `gathererOutput`; internal prompt text updated accordingly
- `buildFixerPrompt(epic, gathererOutput, reviewerFeedback string) string` — `plannerOutput` parameter renamed to `gathererOutput`; internal prompt text "Here is the original implementation plan:" updated to "Here is the codebase context gathered for this task:"
- `ralphContextCachePath(instanceID string) string` — returns an absolute path by joining the current working directory (obtained via `os.Getwd()`) with `.ralph-context-<instanceID>.md`. Ralph is always invoked from the repo root; the Go host and the Claude subprocess both resolve the path the same way. The prompt passed to Claude includes the absolute path so there is no ambiguity.

**`update.go`**
- Wherever planner phase is invoked and `plannerOutput` is threaded through to dev/reviewer/fixer, rename to `gathererOutput` and update phase name string (displayed in TUI) from `"planner"` to `"context-gatherer"`
- At the `<promise>COMPLETE</promise>` detection site: add `os.Remove(ralphContextCachePath(m.instanceID))`

### `.gitignore`

Add `/.ralph-context-*.md` as a committed change to the repo's `.gitignore`.

### TUI phase label

The phase name shown in the TUI status bar changes from `"planner"` to `"context-gatherer"` (or `"gathering context"`).

## Out of Scope

- No changes to Reviewer or Fixer prompt logic (only parameter/variable renaming and wording updates to match new semantics)
- No changes to the reporter, analytics, or TUI screens
- No persistent cross-session cache (cache is deleted on loop start and on COMPLETE)
- Go host does not parse `cache_hit` field — informational only
- `buildPlannerPrompt` is deleted (not kept as dead code)
- The full raw gatherer output text is passed verbatim as `gathererOutput` to dev/reviewer/fixer prompts — no extraction of individual fields by the Go host
