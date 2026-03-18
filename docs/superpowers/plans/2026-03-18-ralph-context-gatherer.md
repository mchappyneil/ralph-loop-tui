# Ralph Context Gatherer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Planner phase with a Context Gatherer that reads repo docs and code patterns before each task, caching findings in an instance-scoped file to avoid redundant research.

**Architecture:** Four-phase pipeline (Context Gatherer → Dev → Reviewer → Fixer) replaces (Planner → Dev → Reviewer → Fixer). The gatherer is a Claude invocation that writes a `.ralph-context-<instanceID>.md` cache file directly to disk via shell tools; the Go host reads it indirectly by injecting its path into the gatherer prompt. Cache is deleted on loop start and on COMPLETE.

**Tech Stack:** Go, Bubble Tea (bubbletea), existing `runClaudeCmd` infrastructure, `os.Getwd` / `os.Remove` from stdlib.

---

## File Map

| File | Change |
|------|--------|
| `main.go` | Add `ralphContextCachePath`, add `buildContextGathererPrompt`, delete `buildPlannerPrompt`, rename `plannerOutput` → `gathererOutput` in 3 prompt functions + wording update, add cache cleanup call before `tea.NewProgram` |
| `model.go` | Rename `phasePlanner` → `phaseContextGatherer`, update `String()`, rename field `plannerOutput` → `gathererOutput` |
| `update.go` | Replace `phasePlanner` references, `buildPlannerPrompt` call, `plannerOutput` field references, status text strings; add cache cleanup at COMPLETE site |
| `main_test.go` | Update/replace tests for renamed/new prompt functions; add `ralphContextCachePath` test |
| `.gitignore` | Add `/.ralph-context-*.md` |

---

## Task 1: `ralphContextCachePath` helper + gitignore

**Files:**
- Modify: `main.go`
- Modify: `.gitignore`
- Modify: `main_test.go`

- [ ] **Step 1: Write the failing test**

Add to `main_test.go`:

```go
func TestRalphContextCachePath(t *testing.T) {
	path := ralphContextCachePath("my-repo-BD-42")
	if !strings.HasSuffix(path, ".ralph-context-my-repo-BD-42.md") {
		t.Errorf("unexpected cache path: %q", path)
	}
	if !strings.HasPrefix(path, "/") {
		t.Errorf("cache path should be absolute, got: %q", path)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test -run TestRalphContextCachePath -v
```

Expected: FAIL — `ralphContextCachePath` undefined

- [ ] **Step 3: Implement `ralphContextCachePath` in `main.go`**

Add after the `buildFixerPrompt` function:

```go
// ralphContextCachePath returns the absolute path to the instance-scoped context cache file.
// The file lives in the working directory (repo root) and is named .ralph-context-<instanceID>.md.
func ralphContextCachePath(instanceID string) string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return filepath.Join(cwd, ".ralph-context-"+instanceID+".md")
}
```

Add `"path/filepath"` to imports.

- [ ] **Step 4: Run test to confirm it passes**

```bash
go test -run TestRalphContextCachePath -v
```

Expected: PASS

- [ ] **Step 5: Add `.gitignore` entry**

Open `.gitignore` and add at the bottom:

```
/.ralph-context-*.md
```

- [ ] **Step 6: Lint**

```bash
golangci-lint run
```

Fix any issues.

- [ ] **Step 7: Commit**

```bash
git add main.go main_test.go .gitignore
git commit -m "feat: add ralphContextCachePath helper and gitignore cache files"
```

---

## Task 2: Rename `phasePlanner` → `phaseContextGatherer` in model

**Files:**
- Modify: `model.go`

The `iterationPhase` const and its `String()` method drive phase names throughout the app. Rename cleanly here; update.go will follow in Task 4.

- [ ] **Step 1: Write the failing test in `main_test.go`**

```go
func TestPhaseContextGatherer_StringValue(t *testing.T) {
	if phaseContextGatherer.String() != "context-gatherer" {
		t.Errorf("got %q, want context-gatherer", phaseContextGatherer.String())
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test -run TestPhaseContextGatherer_StringValue -v
```

Expected: FAIL — `phaseContextGatherer` undefined

- [ ] **Step 3: Update `model.go`**

In `model.go`:

1. Rename the const:
```go
// Before
phasePlanner  iterationPhase = iota

// After
phaseContextGatherer  iterationPhase = iota
```

2. Update the `String()` switch case:
```go
// Before
case phasePlanner:
    return "planner"

// After
case phaseContextGatherer:
    return "context-gatherer"
```

3. Rename the field on the `model` struct:
```go
// Before
plannerOutput    string // stored between planner → dev/reviewer/fixer

// After
gathererOutput    string // stored between context-gatherer → dev/reviewer/fixer
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
go test -run TestPhaseContextGatherer_StringValue -v
```

Expected: PASS

- [ ] **Step 5: Run build to confirm compile errors in update.go (expected)**

```bash
go build ./...
```

Expected: compile errors referencing `phasePlanner` and `plannerOutput` in `update.go` and `main.go`. This confirms the rename is necessary. Note every error location.

- [ ] **Step 6: Skip lint until Task 4 resolves compile errors.**

---

## Task 3: Update prompt functions in `main.go`

**Files:**
- Modify: `main.go`
- Modify: `main_test.go`

### 3a — Update `buildDevPrompt`

- [ ] **Step 1: Delete `TestBuildDevPrompt_ContainsPlannerOutput` and replace with the new test**

In `main_test.go`, delete the entire `TestBuildDevPrompt_ContainsPlannerOutput` function (lines 47-56 in the original file), then add:

```go
func TestBuildDevPrompt_ContainsGathererOutput(t *testing.T) {
	gathererOutput := "[Context Gatherer output]\ntask: BD-1\npatterns:\nUse table-driven tests."
	prompt := buildDevPrompt("", gathererOutput)
	if !strings.Contains(prompt, "Here is the codebase context gathered for this task:") {
		t.Error("dev prompt missing gatherer context header")
	}
	if !strings.Contains(prompt, gathererOutput) {
		t.Error("dev prompt missing gatherer output content")
	}
}
```

- [ ] **Step 2: Update `buildDevPrompt` in `main.go`**

```go
// Before signature
func buildDevPrompt(epic, plannerOutput string) string {

// After
func buildDevPrompt(epic, gathererOutput string) string {
```

And near the bottom of the function:
```go
// Before
if plannerOutput != "" {
    base += "\n\nHere is your implementation plan:\n" + plannerOutput
}

// After
if gathererOutput != "" {
    base += "\n\nHere is the codebase context gathered for this task:\n" + gathererOutput
}
```

Also update the comment on the function:
```go
// Before
// buildDevPrompt produces the prompt for the developer phase.
// The developer finds the next ready task, implements it, runs tests, commits if passing.
// plannerOutput is injected when a preceding planner phase has provided an implementation plan.

// After
// buildDevPrompt produces the prompt for the developer phase.
// The developer finds the next ready task, implements it, runs tests, commits if passing.
// gathererOutput is injected when a preceding context-gatherer phase has provided codebase patterns.
```

### 3b — Update `buildReviewerPrompt`

- [ ] **Step 3: Update the test in `main_test.go`**

In `TestBuildReviewerPrompt_ContainsDiffAndSpecialist`, rename the local variable:
```go
// Before
plannerOutput := "task: BD-1 implement feature"
prompt := buildReviewerPrompt(plannerOutput, diff, specialist)

// After
gathererOutput := "task: BD-1 implement feature"
prompt := buildReviewerPrompt(gathererOutput, diff, specialist)
```

- [ ] **Step 4: Update `buildReviewerPrompt` in `main.go`**

Three changes:

1. Rename the parameter:
```go
// Before
func buildReviewerPrompt(plannerOutput, diff, specialist string) string {
// After
func buildReviewerPrompt(gathererOutput, diff, specialist string) string {
```

2. Update the `fmt.Sprintf` argument to match:
```go
// Before
}, specialist, plannerOutput, diff, specialist)
// After
}, specialist, gathererOutput, diff, specialist)
```

3. Update the prompt template wording:
```go
// Before (in the template string)
Here is the context for the task that was implemented:

%s
// After
Here is the codebase context gathered for this task:

%s
```

### 3c — Update `buildFixerPrompt`

- [ ] **Step 5: Update the test in `main_test.go`**

In `TestBuildFixerPrompt_ContainsFeedback`, rename the local variable:
```go
// Before
plannerOutput := "plan: step 1"
prompt := buildFixerPrompt("", plannerOutput, feedback)

// After
gathererOutput := "task: BD-1\npatterns:\nUse table-driven tests."
prompt := buildFixerPrompt("", gathererOutput, feedback)
```

- [ ] **Step 6: Update `buildFixerPrompt` in `main.go`**

```go
// Before
func buildFixerPrompt(epic, plannerOutput, reviewerFeedback string) string {
    ...
    return fmt.Sprintf(`...
Here is the original implementation plan:

%s
...`, plannerOutput, reviewerFeedback, bdReadyCmd, bdReadyCmd)

// After
func buildFixerPrompt(epic, gathererOutput, reviewerFeedback string) string {
    ...
    return fmt.Sprintf(`...
Here is the codebase context gathered for this task:

%s
...`, gathererOutput, reviewerFeedback, bdReadyCmd, bdReadyCmd)
```

### 3d — Delete `buildPlannerPrompt` and its test; add `buildContextGathererPrompt`

- [ ] **Step 7: Replace `TestBuildPlannerPrompt_ContainsAnalysisOnly` in `main_test.go`**

```go
func TestBuildContextGathererPrompt_ContainsCacheAndFormat(t *testing.T) {
	instanceID := "my-repo-BD-42"
	cachePath := ralphContextCachePath(instanceID)
	prompt := buildContextGathererPrompt("", instanceID)
	if !strings.Contains(prompt, cachePath) {
		t.Errorf("gatherer prompt missing cache file path %q", cachePath)
	}
	if !strings.Contains(prompt, "[Context Gatherer output]") {
		t.Error("gatherer prompt missing output format spec")
	}
	if !strings.Contains(prompt, "cache_hit") {
		t.Error("gatherer prompt missing cache_hit field")
	}
}

func TestBuildContextGathererPrompt_EpicFilter(t *testing.T) {
	prompt := buildContextGathererPrompt("BD-42", "repo-BD-42")
	if !strings.Contains(prompt, "BD-42") {
		t.Error("gatherer prompt missing epic filter")
	}
}
```

- [ ] **Step 8: Run tests to confirm they fail**

```bash
go test -run TestBuildContextGathererPrompt -v
```

Expected: FAIL — `buildContextGathererPrompt` undefined

- [ ] **Step 9: Delete `buildPlannerPrompt` and add `buildContextGathererPrompt` in `main.go`**

Delete the `buildPlannerPrompt` function entirely (lines ~353–390 in the original file).

Add `buildContextGathererPrompt`:

```go
// buildContextGathererPrompt produces the prompt for the context-gatherer phase.
// The gatherer finds the next ready task, checks the instance-scoped cache for known patterns,
// does fresh docs+code research on a cache miss, writes findings to the cache file, and
// outputs a pattern summary for the dev phase.
func buildContextGathererPrompt(epic, instanceID string) string {
	bdReadyCmd := "bd ready --json"
	if epic != "" {
		bdReadyCmd = fmt.Sprintf("bd ready --parent %s --json", epic)
	}

	epicNote := ""
	if epic != "" {
		epicNote = fmt.Sprintf("\n\n**IMPORTANT**: You are scoped to epic %s. Only work on tasks within this epic.", epic)
	}

	cachePath := ralphContextCachePath(instanceID)

	return fmt.Sprintf(`You are the Context Gatherer phase of an AI pipeline. Your job is RESEARCH ONLY — do not write any implementation code or make any commits.%s

Your task:

1. Run: %s
   Pick the highest-priority READY task (P0 > P1 > P2 > P3 > P4). Call it T.

2. Run: bd show <T> --json
   Read the full task description, acceptance criteria, and any notes.

3. Read the cache file at %s if it exists.
   Assess: does the cache cover the domains this task touches well enough to guide implementation?

4. If cache is sufficient (full or partial hit):
   - Use the cached patterns for covered domains.
   - For any uncovered domains, proceed to step 5 for those areas only.
   - Skip to step 6 if fully covered.

5. If cache is insufficient (partial or full miss):
   - Read relevant docs: CLAUDE.md, AGENTS.md, README.md, and anything under docs/
   - Find and read 2-3 existing files most similar to what this task requires (same package, same pattern area, etc.)
   - When docs and code contradict each other, prefer whichever is more recent.
   - Write updated findings to %s using this format:

     # Ralph Context Cache
     Instance: %s

     ## <Domain name>
     Last updated: <today's date>
     <pattern summary>

   Append new sections or update stale ones. Do not remove existing sections.

6. Output your response in this exact format:

[Context Gatherer output]
task: <T>
cache_hit: full|partial|none
patterns:
<concise summary of the relevant patterns and conventions the dev should follow>`, epicNote, bdReadyCmd, cachePath, cachePath, instanceID)
}
```

- [ ] **Step 10: Run tests**

```bash
go test -run "TestBuildContextGathererPrompt|TestBuildDevPrompt|TestBuildReviewerPrompt|TestBuildFixerPrompt" -v
```

Expected: all PASS

- [ ] **Step 11: Lint**

```bash
golangci-lint run
```

Fix any issues (likely unused import `strings` in main.go if applicable, or missing imports).

---

## Task 4: Update `update.go` references + cache lifecycle

**Files:**
- Modify: `update.go`
- Modify: `main.go` (cache cleanup at startup)

- [ ] **Step 1: Fix compile errors in `update.go`**

Find all references to `phasePlanner`, `buildPlannerPrompt`, `m.plannerOutput`, and `"planner"` strings. Make these replacements:

| Old | New |
|-----|-----|
| `phasePlanner` | `phaseContextGatherer` |
| `buildPlannerPrompt(m.epic)` | `buildContextGathererPrompt(m.epic, m.instanceID)` |
| `m.plannerOutput` | `m.gathererOutput` |
| `"planner"` (in status strings and phase strings) | `"context-gatherer"` |
| `"Phase: planner"` | `"Phase: context-gatherer"` |
| `fmt.Sprintf("Iteration %d • planner", ...)` | `fmt.Sprintf("Iteration %d • context-gatherer", ...)` |
| `"from": "planner"` | `"from": "context-gatherer"` |

Specific lines to update (reference line numbers from the grep output):

- `update.go:154` — status text string
- `update.go:159` — `m.currentPhase = phasePlanner`
- `update.go:161` — `m.plannerOutput = ""`
- `update.go:179` — `m.appendHomebase("Phase: planner")`
- `update.go:181` — `buildPlannerPrompt(m.epic)` call
- `update.go:277` — `case phasePlanner:`
- `update.go:278` — `m.plannerOutput = ExtractFullText(...)`
- `update.go:281` — `"from": "planner"` in event
- `update.go:286` — `buildDevPrompt(m.epic, m.plannerOutput)`
- `update.go:306` — `buildReviewerPrompt(m.plannerOutput, diff, specialist)`
- `update.go:374` — `buildFixerPrompt(m.epic, m.plannerOutput, m.reviewerFeedback)`
- `update.go:386` — `buildReviewerPrompt(m.plannerOutput, diff, specialist)`

- [ ] **Step 2: Add cache cleanup at COMPLETE site in `update.go`**

In the `bdReadyCheckMsg` handler, at the "Verified: no ready work remains" branch (around line 432), add cache cleanup before `m.loopDone = true`:

```go
// Verified: no ready work remains — clean up cache file
_ = os.Remove(ralphContextCachePath(m.instanceID))
m.loopDone = true
```

Add `"os"` to `update.go` imports if not already present.

- [ ] **Step 3: Add cache cleanup at loop start in `main.go`**

In `main()`, immediately after `derivedID` is finalized (after line `derivedID = instanceIDVal` around line 68) and before the signal handler goroutine setup, add:

```go
// Clean up any stale context cache from a prior run for this instance
_ = os.Remove(ralphContextCachePath(derivedID))
```

- [ ] **Step 4: Build to verify no compile errors**

```bash
go build ./...
```

Expected: clean build.

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: all PASS

- [ ] **Step 6: Lint**

```bash
golangci-lint run
```

Fix any issues.

- [ ] **Step 7: Commit**

```bash
git add main.go model.go update.go main_test.go .gitignore
git commit -m "feat: replace planner with context-gatherer phase using instance-scoped cache"
```

---

## Task 5: Update `update_test.go` references

**Files:**
- Modify: `update_test.go`

- [ ] **Step 1: Check for planner references in update_test.go**

```bash
grep -n "planner\|plannerOutput" update_test.go
```

- [ ] **Step 2: Update any references found**

For any reference to `"planner"` phase strings, `phasePlanner`, or `plannerOutput`, apply the same renames as Task 4. Typical patterns:

```go
// Before
m.currentPhase = phasePlanner
// or
if strings.Contains(output, "planner") { ... }
// or
m.plannerOutput = "some output"

// After
m.currentPhase = phaseContextGatherer
// or
if strings.Contains(output, "context-gatherer") { ... }
// or
m.gathererOutput = "some output"
```

- [ ] **Step 3: Run tests**

```bash
go test ./... -v
```

Expected: all PASS

- [ ] **Step 4: Lint**

```bash
golangci-lint run
```

- [ ] **Step 5: Commit if there were changes**

```bash
git add update_test.go
git commit -m "test: update phase references from planner to context-gatherer"
```

---

## Verification

- [ ] **Full test suite passes**

```bash
go test ./... -v
```

- [ ] **Binary builds cleanly**

```bash
go build -o ralph-loop-go
```

- [ ] **No planner references remain (outside vendor/)**

```bash
grep -rn "planner\|plannerOutput\|buildPlannerPrompt\|phasePlanner" --include="*.go" . | grep -v vendor/
```

Expected: zero matches

- [ ] **Cache path function works correctly**

The test `TestRalphContextCachePath` covers this.

- [ ] **Lint clean**

```bash
golangci-lint run
```
