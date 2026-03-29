# Spec 0004r2 — Criterion-to-File Scope Mapping

## spec_id
0004r2-criterion-file-scope-mapping

## Depends on
0004r (delta-diff evaluation must exist; this spec enhances its fast-path logic)

## Vision

0004r's fast-path carry-forward must conservatively re-evaluate any criterion
when it cannot determine file scope overlap. This means every criterion gets
re-evaluated every cycle unless the evaluator can prove the delta is
irrelevant — and without explicit scope data, it cannot prove anything. With
explicit criterion-to-file scope mapping populated during the plan stage,
the fast path can precisely skip criteria whose scope does not overlap the
delta, turning a conservative O(N-criteria) re-evaluation into a targeted
O(changed-criteria) evaluation.

## Summary

During planning, record which files each acceptance criterion is expected
to touch. Store this mapping in `RunState` as `CriterionFileScopes`. The
accept stage uses this mapping to determine which criteria need re-evaluation
after each cycle — criteria whose file scope does not overlap the delta skip
evaluation entirely. Criteria with no explicit scope fall through to
re-evaluation (conservative default).

## Goals

### Primary
- The plan stage populates `CriterionFileScopes` from task
  `ExpectedTouchedArea` when creating the initial plan
- The accept stage consults `CriterionFileScopes` to skip re-evaluation of
  criteria whose scope does not intersect the current cycle's changed files
- Criteria with no scope entry are always re-evaluated (conservative fallback)

### Secondary
- `CriterionFileScopes` is persisted in `run.json` and survives resume
- Scope entries support both exact file paths and directory prefixes
  (e.g., `internal/next/acceptor/`)

## Non-goals
- Auto-inferring file scope from criterion text (scope comes from task
  `ExpectedTouchedArea` only)
- Changing how the plan stage works beyond recording scope
- Modifying the evaluation prompt sent to the LLM
- Building the delta-diff evaluator itself (that is 0004r)

## Architecture

### `RunState.CriterionFileScopes`

New field on `RunState` in `internal/next/runstore/types.go`:
```go
CriterionFileScopes map[string][]string `json:"criterion_file_scopes,omitempty"`
```
Keyed by criterion text (the same string used in `AcceptStageConfig.Criteria`
and `acceptor.EvaluateInput.Criteria`). Values are file paths or directory
prefixes from the tasks that address that criterion. Persisted in `run.json`.
`NormalizeNilFields` maps nil → `map[string][]string{}`.

### Scope population in the plan stage

In `internal/next/specloop/stages/plan.go`, after the planner returns a `Plan`
and tasks are appended to `rs.Tasks`:

1. Parse acceptance criteria from the spec content using
   `acceptor.ParseAcceptanceCriteria`.
2. For each criterion, collect the union of `ExpectedTouchedArea` from all
   tasks in the plan whose `Objective` references that criterion (substring
   match on the criterion's numbered prefix, e.g., "criterion 1", "AC 1", or
   the criterion text itself).
3. If the plan is a fix plan (`Kind == "fix"`), merge new scope entries with
   existing `rs.CriterionFileScopes` (union, not replace) — fix tasks may
   touch additional files.
4. Criteria with no matching tasks get no entry (triggering the conservative
   fallback in the accept stage).

Helper function:
```go
// buildCriterionFileScopes maps each criterion string to the union of
// ExpectedTouchedArea from tasks whose objective references that criterion.
func buildCriterionFileScopes(criteria []string, tasks []planner.TaskDef) map[string][]string
```
Located in `internal/next/specloop/stages/plan.go`.

### Scope consumption in the accept stage

In `internal/next/specloop/stages/accept.go`, during the `Run` method, before
calling the evaluator:

1. Compute the set of files changed in the current cycle from completed tasks:
   union of `task.FilesChanged` for all tasks with `task.Cycle == rs.Cycle`.
2. For each criterion in the evaluation list:
   - If `rs.CriterionFileScopes[criterion]` is non-empty AND the criterion
     passed in the prior cycle (`rs.AcceptanceResults` contains a pass for it),
     check whether any scope entry intersects the changed-files set.
   - Intersection uses exact string comparison for file paths and
     `strings.HasPrefix` for directory scopes (paths ending in `/`). A
     directory scope like `internal/next/acceptor/` matches any changed file
     with that prefix.
   - If no intersection: skip re-evaluation, carry forward the prior pass
     result.
   - If intersection or no scope entry: include in the evaluation list sent to
     the evaluator.
3. Merge carried-forward results with freshly-evaluated results into the final
   `AcceptanceResult`.

Helper function:
```go
// scopeIntersectsDelta returns true if any path in scope overlaps with any path
// in changedFiles. Supports exact match and directory-prefix matching (paths
// ending in "/").
func scopeIntersectsDelta(scope []string, changedFiles []string) bool
```
Located in `internal/next/specloop/stages/accept.go`.

### Prior-cycle result lookup

The accept stage needs to know which criteria passed in the prior cycle.
`rs.AcceptanceResults` already stores string-formatted results. Add a helper:
```go
// priorCriterionPassed returns true if the criterion has a "pass" result
// in the prior acceptance results.
func priorCriterionPassed(criterion string, acceptanceResults []string) bool
```
Located in `internal/next/specloop/stages/accept.go`.

### Files in scope
- `internal/next/runstore/types.go` — new field + nil normalization
- `internal/next/runstore/types_test.go` — normalization test
- `internal/next/specloop/stages/plan.go` — scope population
- `internal/next/specloop/stages/plan_test.go` — unit tests for `buildCriterionFileScopes`
- `internal/next/specloop/stages/accept.go` — scope consumption + skip logic
- `internal/next/specloop/stages/accept_test.go` — unit tests for skip/no-skip

## Acceptance Criteria

1. `RunState.CriterionFileScopes` exists as `map[string][]string` with JSON
   tag `"criterion_file_scopes,omitempty"`, and `NormalizeNilFields` maps nil
   to an empty map.

2. After the plan stage completes with an initial plan containing tasks that
   have `ExpectedTouchedArea` values, `rs.CriterionFileScopes` contains
   entries mapping criteria to the union of those tasks'
   `ExpectedTouchedArea` paths.

3. When a fix plan adds tasks with new `ExpectedTouchedArea` values, existing
   `CriterionFileScopes` entries are extended (union), not replaced.

4. When the accept stage runs and a criterion (a) passed in the prior cycle
   and (b) has a non-empty `CriterionFileScopes` entry that does not
   intersect the current cycle's changed files, that criterion is not sent
   to the evaluator — its prior pass result is carried forward.

5. When a criterion has no entry in `CriterionFileScopes`, it is always sent
   to the evaluator regardless of prior results (conservative fallback).

6. `scopeIntersectsDelta` matches directory-prefix scopes (paths ending in
   `/`, e.g., `internal/next/acceptor/` matches
   `internal/next/acceptor/types.go`) and exact file paths.

7. All existing tests in `internal/next/specloop/stages/...` and
   `internal/next/runstore/...` continue to pass.

## Scenarios

### Scenario: Criterion skipped — scope does not overlap delta
**Given:** A run in cycle 2 where criterion C1 passed in cycle 1, and
`rs.CriterionFileScopes["C1"]` is `["internal/next/acceptor/types.go"]`. The
only task completed in cycle 2 changed `internal/next/planner/types.go`.
**When:** The accept stage runs
**Then:** C1 is not sent to the evaluator. The final `AcceptanceResult`
contains a pass result for C1 carried forward from cycle 1. The evaluator
receives only the criteria whose scope intersects the delta (or that have no
scope entry).

### Scenario: Criterion re-evaluated — scope overlaps delta
**Given:** A run in cycle 2 where criterion C2 passed in cycle 1, and
`rs.CriterionFileScopes["C2"]` is `["internal/next/planner/"]`. A cycle 2
task changed `internal/next/planner/sanitize.go`.
**When:** The accept stage runs
**Then:** C2 is sent to the evaluator because its directory-prefix scope
overlaps the changed file. The evaluator's result for C2 replaces the prior
result.

### Scenario: Fix plan merges scope with existing entries
**Given:** A run where the initial plan for criterion C1 set
`rs.CriterionFileScopes["C1"]` to `["internal/next/acceptor/types.go"]`. A
fix plan (Kind == "fix") adds a task for C1 with `ExpectedTouchedArea`
`["internal/next/acceptor/evaluator.go"]`.
**When:** The plan stage completes the fix plan
**Then:** `rs.CriterionFileScopes["C1"]` contains both
`"internal/next/acceptor/types.go"` and `"internal/next/acceptor/evaluator.go"`
(union, not replacement).

### Scenario: No scope entry — conservative re-evaluation
**Given:** A run in cycle 2 where criterion C3 passed in cycle 1, but
`rs.CriterionFileScopes` has no entry for C3 (the plan stage could not
map any task to C3).
**When:** The accept stage runs
**Then:** C3 is sent to the evaluator. No carry-forward occurs for criteria
without explicit scope.

## Validation

```bash
go test ./internal/next/runstore/... -run TestNormalize
go test ./internal/next/specloop/stages/... -run TestPlan
go test ./internal/next/specloop/stages/... -run TestAccept
go test ./internal/next/specloop/stages/...
go test ./internal/next/runstore/...
go vet ./...
```
