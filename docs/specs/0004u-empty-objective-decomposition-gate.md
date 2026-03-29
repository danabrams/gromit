# Spec 0004u — Empty-Objective Decomposition Gate

## spec_id
empty-objective-decomposition-gate

## Vision
When the decomposer splits a failing task into sub-tasks, it can return sub-tasks with empty
objectives. These empty-objective tasks pass through the task loop unchecked, consuming full retry
budgets and token allocations while producing no useful work. In run-45d354991085877b, two such
tasks consumed ~1.4M tokens across 4 attempts. The decomposer's output must be validated before
sub-tasks enter the queue.

## Summary
Add post-decomposition validation in `RunTaskLoop` that rejects sub-task lists containing any task
with an empty `Objective` field. When validation fails, the decomposition is treated as a
decomposer error (same as `dErr != nil`), the parent task falls through to the existing
needs_split/failure path, and a diagnostic event is emitted. This prevents empty-objective tasks
from ever entering the execution queue.

## Goals
### Primary
- Validate that every sub-task returned by `TaskDecomposer.Decompose()` has a non-empty
  `Objective` before enqueuing
- Treat empty-objective sub-tasks as a decomposition failure, not a silent pass-through
- Emit a diagnostic event identifying which sub-task(s) had empty objectives

## Non-goals
- Validating objective quality (non-empty is the gate; semantic quality is out of scope)
- Changing the `TaskDecomposer` interface signature
- Validating other sub-task fields (e.g. `ExpectedTouchedArea`) — those are separate concerns
- Modifying `planner.ValidatePlan` (that validates `planner.TaskDef`, not `runstore.Task`)

## Architecture
The validation goes in `internal/next/specloop/taskloop.go`, in `RunTaskLoop`, immediately after
the `cfg.Decomposer.Decompose(ctx, entry.task)` call succeeds (line ~287-288). Before the
`decompositionsUsed++` increment and sub-task enqueuing:

1. Add a `validateSubTasks(subTasks []runstore.Task) error` function that iterates sub-tasks and
   returns an error if any have `Objective == ""` (or whitespace-only).
2. In the decomposition success path, call `validateSubTasks`. If it returns an error, treat it
   identically to `dErr != nil` — skip the decomposition, let the parent task fall through to the
   existing failure handling.
3. Emit a `decomposition_rejected` event (new event type in `runstore/events.go`) with the parent
   task ID and reason string.

This is a ~20-line change: one validation function, one call site, one event type.

## Acceptance Criteria
1. `validateSubTasks` returns an error when any sub-task has an empty or whitespace-only
   `Objective`.
2. `validateSubTasks` returns nil when all sub-tasks have non-empty objectives.
3. When decomposition produces empty-objective sub-tasks, `RunTaskLoop` does not enqueue them —
   the parent task proceeds through the existing failure path instead.
4. A `decomposition_rejected` event is emitted with the parent task ID and a reason containing
   the offending sub-task ID(s).

## Scenarios

### Scenario 1: Decomposer returns sub-tasks with empty objectives
Given a task `t-004` that needs splitting
When `Decompose` returns two sub-tasks: `{TaskID: "t-005", Objective: ""}` and `{TaskID: "t-006", Objective: ""}`
Then `validateSubTasks` returns an error mentioning `t-005` and `t-006`
And the sub-tasks are not added to the execution queue
And a `decomposition_rejected` event is emitted with TaskID `t-004`
And the parent task falls through to the needs_split failure path

### Scenario 2: Decomposer returns mix of valid and empty objectives
Given a task `t-004` that needs splitting
When `Decompose` returns `{TaskID: "t-005", Objective: "refactor parser"}` and `{TaskID: "t-006", Objective: ""}`
Then `validateSubTasks` returns an error mentioning `t-006`
And neither sub-task is enqueued (all-or-nothing rejection)

### Scenario 3: Decomposer returns all valid sub-tasks
Given a task `t-004` that needs splitting
When `Decompose` returns `{TaskID: "t-005", Objective: "refactor parser"}` and `{TaskID: "t-006", Objective: "add tests"}`
Then `validateSubTasks` returns nil
And both sub-tasks are enqueued normally

## Validation
```bash
go test ./internal/next/specloop/ -run TestValidateSubTasks -v
go test ./internal/next/specloop/ -run TestTaskLoop.*Decompos -v
go test ./internal/next/runstore/ -run TestDecompositionRejected -v
```
