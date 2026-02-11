---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T07:42:15-05:00"
id: status-bead-breakdown
source_spec: status-bead-breakdown
---

# Status Bead Breakdown Implementation Plan

**Goal:** Replace the single "X ready" bead count in `gromit status` with a full breakdown showing counts for every bd status (ready, in-progress, blocked, deferred, closed) plus a "this run" parenthetical when run info is available.

**Architecture:** Add generic count methods to the bead client, expand `PipelineStatus` with per-status count fields, thread the run start time from status.json into `ReadStatus`, and rewrite `formatPipeline` to render the comma-separated breakdown line.

**Tech Stack:** Go

**Spec:** `.gromit/specs/status-bead-breakdown.md`

---

## Architecture

**Overview:**
Four files change in a bottom-up dependency chain: bead client gets count methods, pipeline status struct expands and queries all statuses, format function renders the new line, and the runner threads the start time through.

**Key Components:**
1. **`internal/bead/bead.go`**: New `CountByStatus(status string)` and `CountClosedAfter(after time.Time)` methods — thin wrappers around `bd list` with status/time filters
2. **`internal/pipeline/status.go`**: Expanded `PipelineStatus` struct with per-status counts. `ReadStatus` gains a `startedAt *time.Time` parameter and queries all bead statuses when a client is available
3. **`internal/runner/format.go`**: `formatPipeline` builds a comma-separated Beads line from non-zero counts, with optional "(X this run)" on closed
4. **`internal/runner/runner.go`**: `Runner.Status()` extracts `StartedAt` from status.json and passes it into `pipeline.ReadStatus`

**Integration Points:**
- `pipeline.ReadStatus` signature changes to accept `startedAt *time.Time` — only one caller (`Runner.Status()`)
- `bead.Client` gets two new exported methods — used only by the pipeline package (not the `BeadClient` interface)
- `PipelineStatus` struct gains 6 new fields — consumed by `formatPipeline`

**Data Flow:**
```
Runner.Status()
  → ReadStatus(gromitDir) → gets StartedAt from status.json
  → pipeline.ReadStatus(gromitDir, specsDir, plansDir, &startedAt)
    → bead.Client.CountReady()             → ready count
    → bead.Client.CountByStatus("open")    → open count
    → bead.Client.CountByStatus("in_progress") → in-progress count
    → bead.Client.CountByStatus("deferred")    → deferred count
    → bead.Client.CountByStatus("closed")      → closed count
    → bead.Client.CountClosedAfter(startedAt)  → closed-this-run count
    → blocked = open - ready
  → formatPipeline(pipelineStatus)
    → "Beads:    14 ready, 5 blocked, 543 closed (23 this run)"
```

**Tradeoffs:**
- **Generic `CountByStatus` vs specific methods**: Generic — one method handles all status types, reducing boilerplate
- **`*time.Time` vs full Status struct**: Pointer to time — keeps pipeline decoupled from runner's Status type

## Test Strategy

**Test Levels:**
1. **Unit Tests**: All new logic testable without `bd` CLI
2. **No integration tests**: Bead client methods are thin wrappers; formatting tests cover behavioral surface

**Key Test Cases (in `format_test.go`):**
- All non-zero statuses: `14 ready, 5 blocked, 543 closed`
- With "this run": `543 closed (23 this run)` when HasRunInfo + ClosedThisRunCount > 0
- Without parenthetical: `543 closed` when no run info
- Zero statuses omitted: only populated statuses shown
- All zero: `Beads:    none`
- Single status: just `3 ready`
- Display order: ready, in-progress, blocked, deferred, closed
- Ready bead IDs preserved below Beads line
- HasRunInfo true but ClosedThisRunCount 0: no parenthetical

**Mocking Strategy:**
- Format tests populate `PipelineStatus` directly — no mocking
- Pipeline tests use `t.TempDir()` — bead counts default to zero (no bd in tests)

## Implementation Tasks

### Task 1: Add count methods to bead client

**Files:**
- Modify: `internal/bead/bead.go`

**What to Do:**
Add two new methods to `Client`:
- `CountByStatus(status string) (int, error)` — runs `bd list --json --status <status> --limit 0`, parses JSON array, returns length
- `CountClosedAfter(after time.Time) (int, error)` — runs `bd list --json --status closed --closed-after <RFC3339> --limit 0`, parses JSON array, returns length

Follow `CountReady()` pattern: nil check, run command, handle empty/`[]`, parse array, return length.

**Acceptance Criteria:**
- `CountByStatus` accepts any bd status string and returns matching bead count
- `CountClosedAfter` accepts a `time.Time` and returns count of beads closed after that time
- Both return `(0, nil)` for empty results and `(0, error)` for command failures

**Dependencies:** None

### Task 2: Expand PipelineStatus and update ReadStatus

**Files:**
- Modify: `internal/pipeline/status.go`
- Modify: `internal/pipeline/status_test.go`

**What to Do:**
Add fields to `PipelineStatus`: `InProgressCount`, `BlockedCount`, `DeferredCount`, `ClosedCount`, `ClosedThisRunCount int`, `HasRunInfo bool`.

Change `ReadStatus` to `ReadStatus(gromitDir, specsDir, plansDir string, startedAt *time.Time)`. When bead client is available, query all statuses. Compute blocked = open - ready. When `startedAt != nil`, query `CountClosedAfter` and set `HasRunInfo = true`.

Update existing tests to pass `nil` for the new parameter.

**Acceptance Criteria:**
- `ReadStatus` populates all new count fields when bead client is available
- When `startedAt` is nil, `HasRunInfo` is false and `ClosedThisRunCount` is 0
- All existing pipeline tests pass with updated signature

**Dependencies:** Task 1

### Task 3: Update formatPipeline and its tests

**Files:**
- Modify: `internal/runner/format.go`
- Modify: `internal/runner/format_test.go`

**What to Do:**
Rewrite the Beads line in `formatPipeline()`:
- Build comma-separated list of non-zero counts in order: ready, in-progress, blocked, deferred, closed
- If closed > 0 and `HasRunInfo` and `ClosedThisRunCount > 0`: append `(X this run)` to closed segment
- If all counts are zero: `Beads:    none`
- Preserve ready bead ID list below

Add comprehensive test cases covering all combinations.

**Acceptance Criteria:**
- Comma-separated non-zero counts in lifecycle order
- "(X this run)" only when HasRunInfo=true AND ClosedThisRunCount > 0
- All-zero = `Beads:    none`
- Ready bead IDs still appear below

**Dependencies:** Task 2

### Task 4: Thread run start time into ReadStatus

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**
In `Runner.Status()`, after reading status.json, pass `&status.StartedAt` to `pipeline.ReadStatus()` when status is non-nil, or `nil` when status is nil.

**Acceptance Criteria:**
- status.json exists → ReadStatus receives run start time
- status.json absent → ReadStatus receives nil
- `gromit status` compiles and displays expanded Beads line

**Dependencies:** Task 2

---

## Notes

- The `BeadClient` interface in `interfaces.go` does NOT need updating — the new bead methods are only used by the pipeline package, which creates its own `bead.Client` directly
- Blocked count uses `open - ready` (not an explicit bd query), matching what users intuit as "blocked"
- The `--closed-after` flag on bd takes RFC3339 format
- If bd CLI is unavailable (e.g., in tests), all bead counts remain zero — this is the existing graceful degradation pattern
