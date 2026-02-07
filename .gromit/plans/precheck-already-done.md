---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T11:47:12-05:00"
id: precheck-already-done
priority: high
source_spec: precheck-already-done
---

# Pre-Check: Skip Already-Completed Beads — Implementation Plan

**Goal:** Before each bead is processed, run a lightweight haiku pre-check that auto-closes beads whose acceptance criteria are already satisfied, saving ~50% of wasted iterations.

**Architecture:** New `runPrecheck` method in runner.go calls `claude.Run` (non-streaming) with haiku using a `PROMPT_precheck.md` template. On `PRECHECK_PASSED`, the bead is closed and skipped without counting as an iteration. On `PRECHECK_NOT_MET` or error, normal processing continues.

**Tech Stack:** Go, existing prompt/claude/runner/logger packages

**Spec:** `.gromit/specs/precheck-already-done.md`

---

## Architecture

### Overview

A lightweight haiku pre-check runs between the stuck-bead check and `processBead` in the runner loop. It uses the existing `claude.Client.Run` (non-streaming) with a new `PROMPT_precheck.md` template.

### Key Components

1. **`PrecheckContext` (prompt package)** — New context struct holding bead title, description, acceptance criteria, and parent context. Follows `ScopeContext`/`DecomposeContext` pattern.

2. **`RenderPrecheck` (prompt package)** — New render method on `Renderer`, same pattern as `RenderScope`.

3. **`PROMPT_precheck.md` (template)** — Instructs Claude to read-only inspect the codebase and output `PRECHECK_PASSED` or `PRECHECK_NOT_MET`.

4. **`runPrecheck` (runner package)** — New method on `Runner` modeled after `checkScope`. Builds context, renders prompt, calls `claude.Run` with haiku, parses signal.

5. **`IterationLog.Outcome` (logger package)** — New optional field to distinguish `precheck_skipped` from normal iterations.

### Integration Point

`runner.go:Run()` — after stuck-bead check (line ~306), before `iteration++` (line ~313):
- Call `runPrecheck(ctx, bead)`
- If passed: log message, close bead, sync, write JSONL with `precheck_skipped`, `continue` without incrementing iteration
- If not-met or error: proceed normally

### Signal Detection

Use `strings.Contains(output, "PRECHECK_PASSED")` — same pattern as `IsValidationPassed`. Conservative: if signal is absent, treat as NOT_MET.

### Data Flow

1. Runner fetches bead → stuck-bead check passes → `runPrecheck(ctx, bead)`
2. `runPrecheck` fetches parent, builds `PrecheckContext`, renders template, calls haiku
3. Returns bool (passed/not-met)
4. If passed: close bead, log, continue
5. If not-met: `processBead` as normal

## Test Strategy

### Unit Tests (runner package)
- `runPrecheck` with mock Claude: PASSED, NOT_MET, and error cases
- Pre-check in `Run` loop: passed beads skipped/closed/logged, iteration counter unchanged
- Pre-check logging: `precheck_skipped` outcome written to JSONL

### Unit Tests (prompt package)
- `RenderPrecheck` renders template with bead data
- `PrecheckContext` fields accessible in template

### Unit Tests (logger package)
- `IterationLog` with `Outcome` field round-trips correctly
- Backward compatibility: logs without `Outcome` parse as empty string

### Key Test Cases
- PASSED → bead closed, synced, logged with `precheck_skipped`, iteration not incremented
- NOT_MET → normal `processBead` flow, iteration incremented
- Claude error → warning logged, falls through to `processBead` (non-blocking)
- Ambiguous output → treated as NOT_MET (conservative)
- Multiple beads, some pass → only non-passed count against `maxIterations`
- Outcome field backward compatible with old logs

### Mocking Strategy
- Mock `ClaudeClient.Run` for controlled outputs
- Mock `BeadClient` to verify Close/Sync calls
- Mock `IterationLogger` to verify Outcome field
- Real `Renderer` with temp template files for rendering tests

## Implementation Tasks

### Task 1: Add Outcome field to IterationLog

**Priority:** P0 (high) — foundational type change needed by other tasks

**Files:**
- Modify: `internal/logger/logger.go`
- Test: `internal/logger/logger_test.go`

**What to Do:**
Add an `Outcome` field to `IterationLog` struct with `json:"outcome,omitempty"`. This field distinguishes `precheck_skipped` from normal iterations. The `omitempty` tag ensures backward compatibility — existing logs without this field parse correctly as empty string.

**Acceptance Criteria:**
- `IterationLog` has `Outcome string` field with `json:"outcome,omitempty"` tag
- Round-trip serialization test passes (encode with outcome, decode, verify)
- Old logs without `outcome` field still parse correctly (empty string)

**Dependencies:** None

### Task 2: Add PrecheckContext and RenderPrecheck to prompt package

**Priority:** P0 (high) — needed by runner integration

**Files:**
- Modify: `internal/prompt/prompt.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Add `PrecheckContext` struct with `Bead *bead.Bead` and `ParentBead *bead.Bead` fields (same as `ScopeContext`). Add `RenderPrecheck(ctx *PrecheckContext) (string, error)` method to `Renderer` that renders `PROMPT_precheck.md`. Follow exact pattern of `RenderScope`.

**Acceptance Criteria:**
- `PrecheckContext` struct exists with Bead and ParentBead fields
- `RenderPrecheck` method renders the `PROMPT_precheck.md` template
- Template rendering test verifies bead fields appear in output

**Dependencies:** None

### Task 3: Add PROMPT_precheck.md template and register in gromit init

**Priority:** P0 (high) — template needed by renderer and runner

**Files:**
- Modify: `cmd/gromit/init.go`

**What to Do:**
Add `defaultPrecheckTemplate` constant with the pre-check prompt content. Register it in `runInit` following the existing pattern (write to `.gromit/templates/PROMPT_precheck.md`). Update the `Long` description of `initCmd` to include the new template. The prompt should instruct Claude to: read relevant files, check each acceptance criterion, output `PRECHECK_PASSED` if ALL met, output `PRECHECK_NOT_MET` if ANY not met, err on the side of NOT_MET when uncertain, and make no code changes.

**Acceptance Criteria:**
- `defaultPrecheckTemplate` constant contains the pre-check prompt
- `runInit` writes `PROMPT_precheck.md` to templates directory
- Template uses `{{.Bead.Title}}`, `{{.Bead.Description}}`, `{{.ParentBead}}` fields
- Prompt explicitly instructs read-only inspection and conservative NOT_MET default

**Dependencies:** Task 2 (template must match PrecheckContext fields)

### Task 4: Add RenderPrecheck to PromptRenderer interface and mocks

**Priority:** P0 (high) — interface change needed by runner

**Files:**
- Modify: `internal/runner/interfaces.go`
- Modify: `internal/runner/interfaces_test.go`

**What to Do:**
Add `RenderPrecheck(ctx *prompt.PrecheckContext) (string, error)` to the `PromptRenderer` interface. Add `RenderPrecheckFn` field and implementation to `mockPromptRenderer`. Add default implementation to `mockRenderer`. This follows the exact pattern of every other Render method in the interface.

**Acceptance Criteria:**
- `PromptRenderer` interface includes `RenderPrecheck` method
- `mockPromptRenderer` has `RenderPrecheckFn` field and working implementation
- `mockRenderer` has default `RenderPrecheck` implementation
- Compile-time interface check still passes

**Dependencies:** Task 2 (needs `PrecheckContext` type)

### Task 5: Implement runPrecheck method and integrate into Run loop

**Priority:** P0 (high) — core feature implementation

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/interfaces_test.go`

**What to Do:**
Add `runPrecheck(ctx context.Context, b *bead.Bead) bool` method to `Runner`. It should: get parent bead, build `PrecheckContext`, render prompt via `RenderPrecheck`, call `claude.Run(ctx, prompt, "haiku")` with a 30-second timeout, check output for `PRECHECK_PASSED` via `strings.Contains`. On any error, log warning and return false (non-blocking, like `checkScope`). Log the result regardless (passed or not-met).

Integrate into `Run()` loop: after the stuck-bead check and `continue` (line ~306), before the iteration separator print (line ~309). On true (passed): log `"Pre-check: acceptance criteria already met, auto-closing bead <id>"`, close bead, sync, write iteration log with `Outcome: "precheck_skipped"`, and `continue`. On false: proceed normally.

**Acceptance Criteria:**
- `runPrecheck` method exists and calls Claude haiku with rendered precheck prompt
- Pre-check runs between stuck-bead check and processBead in the Run loop
- Passed beads are auto-closed, synced, and logged with `precheck_skipped` outcome
- Passed beads do not increment the iteration counter
- Failed pre-checks (NOT_MET) proceed to normal processBead
- Pre-check errors are logged as warnings and fall through to normal processing
- Console log message: `"Pre-check: acceptance criteria already met, auto-closing bead <id>"`

**Dependencies:** Tasks 1, 2, 3, 4

### Task 6: Add unit tests for precheck behavior in runner

**Priority:** P0 (high) — validates core feature

**Files:**
- Test: `internal/runner/interfaces_test.go`

**What to Do:**
Add test functions following the existing `TestRunWithMocks_*` pattern:
- `TestRunWithMocks_PrecheckPassed`: Mock Claude.Run to return PRECHECK_PASSED. Verify bead is closed, synced, logged with `precheck_skipped`, and iteration counter is NOT incremented.
- `TestRunWithMocks_PrecheckNotMet`: Mock Claude.Run to return PRECHECK_NOT_MET. Verify normal processBead flow runs.
- `TestRunWithMocks_PrecheckError`: Mock Claude.Run to return error. Verify warning logged and normal processBead flow runs.
- `TestRunWithMocks_PrecheckDoesNotCountAsIteration`: Two beads where first passes precheck. With maxIterations=1, both should complete (precheck skip + 1 real iteration).

**Acceptance Criteria:**
- All four test cases pass
- Tests use existing mock patterns from interfaces_test.go
- Tests verify Close, Sync, log Outcome, and iteration count behavior

**Dependencies:** Task 5

---

## Notes

- The pre-check uses `claude.Run` (not `StreamRun`) because it's a quick, lightweight call that doesn't need heartbeat or stall detection. This matches the scope check pattern.
- The 30-second timeout for the pre-check is hardcoded (like the success learning timeout) since it's always haiku and should be fast.
- The `omitempty` on `Outcome` ensures zero disruption to existing log parsing — old consumers see the same JSON they always have.
- All tasks are marked P0/high priority for the decomposer since this feature saves ~50% of wasted iterations.
