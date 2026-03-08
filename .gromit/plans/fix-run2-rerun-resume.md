---
id: fix-run2-rerun-resume
spec: debug-investigation
created: 2026-03-07
decomposed: false
---

# Fix: run2 rerun resumes from existing plan and beads

## Research & Context

Investigation report: `.gromit/reports/debug-20260307-120000.md`

When `gromit run2` fails and is rerun with the same spec, it replans and redecomposes from scratch, losing all prior bead progress. The fix adds resume detection so the spec loop reuses existing plans and beads when they are already present.

## Architecture

The change is localized to the spec loop orchestration layer. No stage internals need modification — the spec loop simply skips stages whose outputs already exist.

Resume detection points:
- **Plan**: Check for existing plan file in the worktree before invoking the plan stage
- **Decompose**: Query the task tracker for existing beads with the spec label before invoking the decompose stage

## Tasks

### Task 1: Add plan resume detection to SpecLoop
**Size**: S (10-20 lines)
**Files**: `internal/v2/loop/spec_loop.go`

In `SpecLoop.Run()`, before calling `runPlanStage()`:
1. Check if the plan file exists in the worktree (path: `<worktree>/.gromit/v2/plan.md`)
2. If it exists and is non-empty, read its contents and skip the plan stage
3. If it doesn't exist, run the plan stage as normal
4. Extract the plan file path logic to a helper (reuses `gromitDir` + `v2DirName` constants already in the file)

### Task 2: Add bead resume detection to SpecLoop
**Size**: S (10-20 lines)
**Files**: `internal/v2/loop/spec_loop.go`

In `SpecLoop.Run()`, before calling `runDecompose()`:
1. Query the task tracker for beads with label `spec:<specID>` using the existing `ListWithLabel` or equivalent API
2. If beads exist (count > 0), skip decomposition and use the existing beads
3. If no beads exist, run the decompose stage as normal
4. Emit a log event when resuming (so the user knows beads were reused)

### Task 3: Add resume event types
**Size**: XS (5-10 lines)
**Files**: `internal/events/events.go` (or wherever spec events are defined)

Add event types for resume detection so the CLI subscriber can inform the user:
- `PlanResumedEvent` — emitted when an existing plan is reused
- `DecomposeResumedEvent` — emitted when existing beads are reused

### Task 4: Write tests for resume detection
**Size**: M (30-50 lines)
**Files**: `internal/v2/loop/spec_loop_test.go`

Add test cases:
1. When plan file exists in worktree, plan stage is not invoked and plan content is reused
2. When no plan file exists, plan stage runs normally
3. When beads exist for spec label, decompose stage is not invoked and existing beads are used
4. When no beads exist, decompose stage runs normally
5. Resume events are emitted when skipping stages

## Dependencies

- Task 3 (event types) should be done before Task 1 and Task 2 (they emit the events)
- Task 4 (tests) can be written first (TDD red phase) or alongside Tasks 1-2

## Testing Strategy

1. Unit tests in `spec_loop_test.go` using the existing mock adapters and stage stubs
2. Verify that plan stage mock is NOT called when plan file pre-exists
3. Verify that decompose stage mock is NOT called when beads pre-exist
4. Verify that the bead loop receives the correct beads in both resume and fresh paths
5. Manual test: run `gromit run2` on a spec, let it fail, rerun and confirm it skips plan/decompose
