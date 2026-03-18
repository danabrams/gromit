# Spec 0003i — Contract Deferral for Pending Tasks

## spec_id
0003i-contract-deferral-for-pending-tasks

## Depends on
0003g-contract-loop-detection, 0003h-contract-file-self-correction

## Vision
Contracts are written before implementation, predicting which files will
contain which patterns. When the validate stage evaluates contracts
mid-pipeline, some tasks may still be pending — their expected file changes
haven't happened yet. A `file_contains` assertion that targets a file a
pending task is about to create or modify will fail prematurely, triggering
an unnecessary replan. This wastes cycles and can lead to a blocked run when
the replan generates the same pending task again. The system should recognize
that a pending task is expected to address the failure, defer judgment, and
let the task run before evaluating the contract.

## Summary
When a `file_contains` or `file_exists` contract assertion fails, the
validate stage checks whether the target file appears in the
`expected_touched_area` of any pending task. If so, the failure is removed
from the failure list and a `contract_deferred` event is emitted. No replan
is triggered for deferred failures. The deferred contract is re-evaluated
naturally in the next cycle's validate stage after the pending task has had a
chance to execute.

## Goals
### Primary
- Defer `file_contains` and `file_exists` contract failures when a pending
  task's `expected_touched_area` covers the target file
- Emit a `contract_deferred` event per deferred assertion recording the
  scenario name, file path, pattern, and the task ID that covers it
- Prevent unnecessary replans and blocked runs caused by contracts that
  evaluate before their corresponding tasks execute

## Non-goals
- Deferring assertion types other than `file_contains` and `file_exists`
  (shell checks, `file_not_contains`)
- Re-evaluating deferred contracts within the same cycle
- Limiting how many times a contract can be deferred — 0003g's loop
  detection handles stuck situations
- Modifying the task execution order or stage pipeline

## Architecture

### Validate Stage

### ContractFailure: add Assertion field

The current `ContractFailure` struct (`contract/types.go`) has only
`ScenarioName`, `AssertionType`, and `Details` — no reference to the
original assertion. Add an `Assertion ContractAssertion` field so downstream
passes (deferral, self-correction) can inspect the structured assertion
without parsing the formatted `Details` string:

```go
type ContractFailure struct {
    ScenarioName  string
    AssertionType string
    Details       string
    Assertion     ContractAssertion // original assertion that failed
}
```

Update the evaluator to populate `Assertion` when constructing failures.
This also benefits 0003h, which currently must parse `Details` via
`fmt.Sscanf` — with the `Assertion` field available, 0003h can access
`f.Assertion.FileContains.Path` and `.Pattern` directly.

### Validate Stage restructuring

The deferral pass operates on raw `ContractFailure` objects returned by the
evaluator, **not** on the already-formatted failure strings. The current
`validate.go` code formats failures into strings immediately after
evaluation (lines ~109-111); the implementation must restructure this so
that the deferral pass (0003i) and self-correction pass (0003h) run on the
raw `[]ContractFailure` slice. String formatting happens **after** 0003i
and 0003h but **before** 0003g's loop detection, since 0003g compares
formatted `[]string` values against `rs.LastContractFailures`. The full
sequence is:

```
raw []ContractFailure → 0003i (defer) → 0003h (self-correct) →
  format to []string → 0003g (loop detect vs LastContractFailures)
```

In `validate.go`, after the initial `s.contractEvaluator.Evaluate()` call
and **before** 0003h's self-correction pass, run a deferral pass over any
`file_contains` and `file_exists` failures.

For each `ContractFailure` where `f.AssertionType` is `"file_contains"` or
`"file_exists"`:

1. Extract the target file path from `f.Assertion`. For `file_contains`,
   use `f.Assertion.FileContains.Path`. For `file_exists`, use
   `f.Assertion.FileExists`.
2. Extract the pattern: for `file_contains`, use
   `f.Assertion.FileContains.Pattern`. For `file_exists`, the pattern field
   in the event is empty (there is no pattern — only a file path).
3. Collect all pending tasks from `rs.Tasks` (status `"pending"`). Build a
   map from `ExpectedTouchedArea` path → first task ID that covers it.
   Matching is exact string equality — no prefix or directory matching.
   This means a task with `ExpectedTouchedArea: ["cmd/gromit-next/"]`
   (directory) will NOT match `cmd/gromit-next/spec.go`. The plan stage
   should produce file-level paths for deferral to work correctly.
   When multiple pending tasks cover the same file, the first task in
   `rs.Tasks` slice order wins.
4. If the extracted file path is in the map, remove the failure from the
   raw slice and emit a `contract_deferred` event with the mapped task ID.

The deferral pass runs first in the chain:
```
contract failures → 0003i (defer pending) → 0003h (self-correct paths) → 0003g (loop detect)
```

This ordering matters: a deferred failure should never reach the
self-correction or loop detection logic, since the pending task may produce
the file in the correct location. Note: this means that if a contract points
to the wrong file AND a pending task covers that wrong file, deferral wins
and 0003h's correction is delayed by one cycle. This is acceptable — the
pending task gets first shot, and if it doesn't fix the contract, 0003h
corrects it on the next cycle.

### Interaction with 0003g's LastContractFailures

Deferred failures are **excluded** from the failure list that 0003g stores
in `rs.LastContractFailures`. Specifically, `rs.LastContractFailures` is
populated from the formatted failure strings **after** all three passes
(deferral, self-correction, loop detection) have run — so it only contains
the final remaining failures. This ensures that if a deferred failure
reappears on the next cycle (because the pending task ran but didn't fix
it), 0003g sees it as a new failure rather than a repeated one, giving 0003h
a chance to correct it before loop detection escalates.

**Note on 0003h compatibility:** This spec introduces the `Assertion` field
on `ContractFailure` and restructures the validate stage to operate on raw
`[]ContractFailure` throughout the pass chain. 0003h's architecture section
currently describes parsing `f.Details` via `fmt.Sscanf`; once 0003i is
implemented, 0003h should use `f.Assertion.FileContains.Path` and `.Pattern`
directly instead. The `fmt.Sscanf` approach in 0003h is superseded.

### New event type

Add to `internal/next/runstore/events.go` alongside the other event types,
and register `"contract_deferred"` in the `unmarshalEvent()` switch:

```go
type ContractDeferredEvent struct {
    BaseEvent
    ScenarioName string `json:"scenario_name"`
    FilePath     string `json:"file_path"`
    Pattern      string `json:"pattern"`
    TaskID       string `json:"task_id"`
}
```

### ContractEvaluator

The evaluator is modified to populate the new `Assertion` field on
`ContractFailure` when constructing failure objects. The `fail` helper
closure in `check()` currently takes `(assertionType, detail string)` — it
must be updated to also capture the `ContractAssertion` value `a` and set
`Assertion: a` on the failure. No other evaluator logic changes.

### RunState

No new fields. The deferral pass reads `rs.Tasks` which already exists.
Unlike 0003g (which needs `LastContractFailures` to persist across cycles),
deferral is stateless — it just checks the current task list each cycle.

## Acceptance Criteria

1. When a `file_contains` or `file_exists` assertion fails and the target
   file is in the `ExpectedTouchedArea` of a pending task, the failure is
   removed from the failure list and does not trigger a replan
2. When a `file_contains` assertion fails and no pending task covers the
   target file, the failure propagates normally (to 0003h, then 0003g, then
   replan)
3. One `contract_deferred` event is emitted per deferred assertion, recording
   `scenario_name`, `file_path`, `pattern` (empty for `file_exists`), and
   `task_id`
4. When multiple assertions are deferrable, all are removed in a single pass
   before 0003h's correction pass runs
5. When all contract failures are deferred (and no shell check failures
   exist), validation passes for that cycle
6. When some failures are deferred and others are not, only the non-deferred
   failures propagate
7. The deferral pass operates on raw `ContractFailure` objects, extracting
   file paths from the assertion struct (not by parsing formatted strings)
8. Deferred failures are excluded from `rs.LastContractFailures` so 0003g's
   loop detection does not see them
9. Assertion types other than `file_contains` and `file_exists` are never
   deferred
10. All existing validate stage tests continue to pass

## Scenarios

### Scenario: pending task covers missing file — failure deferred
**Given:** A contract asserts `pattern: "no specs available to run"` in
`cmd/gromit-next/exec_scenario_spec_picker_no_eligible_test.go`. The file
does not exist in the worktree. Task t-018 has status `pending` with
`expected_touched_area: ["cmd/gromit-next/exec_scenario_spec_picker_no_eligible_test.go"]`.
**When:** The validate stage runs and initial evaluation returns a
`file_contains` failure with Details
`cannot read "cmd/gromit-next/exec_scenario_spec_picker_no_eligible_test.go": ...`
**Then:** The failure is removed from the failure list. A
`contract_deferred` event is emitted with `scenario_name`, `file_path:
"cmd/gromit-next/exec_scenario_spec_picker_no_eligible_test.go"`, `pattern:
"no specs available to run"`, `task_id: "t-018"`. No replan triggered.

### Scenario: pending task covers file but pattern not found
**Given:** A contract asserts `pattern: "func pickSpec("` in
`cmd/gromit-next/spec.go`. The file exists but the pattern is not found.
Task t-005 has status `pending` with
`expected_touched_area: ["cmd/gromit-next/spec.go"]`.
**When:** The validate stage runs and initial evaluation returns a
`file_contains` failure with Details
`pattern "func pickSpec(" not found in "cmd/gromit-next/spec.go"`
**Then:** The failure is deferred. A `contract_deferred` event is emitted
with `task_id: "t-005"`. No replan triggered.

### Scenario: file_exists assertion deferred for pending task
**Given:** A contract asserts `file_exists: "cmd/gromit-next/picker.go"`.
The file does not exist in the worktree. Task t-007 has status `pending`
with `expected_touched_area: ["cmd/gromit-next/picker.go"]`.
**When:** The validate stage runs and initial evaluation returns a
`file_exists` failure
**Then:** The failure is deferred. A `contract_deferred` event is emitted
with `file_path: "cmd/gromit-next/picker.go"`, `pattern: ""` (empty),
`task_id: "t-007"`. No replan triggered.

### Scenario: no pending task covers the file — failure propagates
**Given:** A contract asserts `pattern: "feature/foo"` in
`cmd/gromit-next/exec_test.go`. No pending task has
`cmd/gromit-next/exec_test.go` in its `expected_touched_area`.
**When:** The validate stage runs and initial evaluation returns a
`file_contains` failure
**Then:** The failure is not deferred. It propagates to 0003h's correction
pass, then 0003g's loop detection, then replan as normal. No
`contract_deferred` event emitted.

### Scenario: mix of deferrable and non-deferrable failures
**Given:** A contract has 3 failing `file_contains` assertions. Two target
files covered by pending tasks, one targets a file with no pending task
coverage.
**When:** The validate stage runs
**Then:** Two failures are deferred (2 `contract_deferred` events emitted).
The third propagates normally. Only the non-deferred failure reaches
0003h/0003g.

### Scenario: multiple pending tasks — first match wins
**Given:** A contract asserts a pattern in `cmd/gromit-next/spec.go`. Two
pending tasks (t-005 and t-009) both list `cmd/gromit-next/spec.go` in their
`expected_touched_area`.
**When:** The validate stage runs and the assertion fails
**Then:** The failure is deferred. The `contract_deferred` event records the
task ID of whichever pending task is encountered first. Only one event is
emitted for the assertion.

## Validation
- `go test ./internal/next/contract/... -count=1 -timeout 60s`
- `go test ./internal/next/specloop/stages/... -count=1 -timeout 60s`
- `go test ./internal/next/runstore/... -count=1 -timeout 60s`
- `go vet ./...`
