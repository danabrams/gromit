# Spec 0004q — Remediation Regression Guard

## spec_id
0004q-remediation-regression-guard

## Vision

Remediation cycles are non-monotonic. Observed runs reached 12/13 passing
criteria on cycle 3, then regressed to 10/13 on cycle 4 — spending the final
generation on a strictly worse state. The pipeline has no mechanism to detect
or prevent regression. When a remediation cycle makes things worse, the
regressive changes are committed and the next cycle replans from the degraded
state, compounding the loss.

## Summary

Track the best-achieved acceptance pass count and corresponding git commit SHA
across accept cycles. After each acceptance evaluation, compare the current
pass count against the high-water mark. When a cycle produces a regression
(pass count strictly lower than the best), revert to the best-state commit
instead of committing the worse state, and replan from the best-state
checkpoint with regression context injected into the failure context.

## Goals

### Primary
- Best-state (pass count + commit SHA) is tracked in `RunState` and updated
  after each acceptance evaluation where the current pass count meets or
  exceeds the prior best
- When a regression is detected (current pass count < best), the worktree is
  reset to `BestPassCommitSHA` and the cycle returns `ReplanFrom` with
  regression context instead of committing the worse state
- A `regression_detected` event is emitted with current count, best count,
  and the criteria that regressed

### Secondary
- Best-state fields are persisted in `run.json` and survive resume
- Regression context includes the names of the regressed criteria so the
  planner can target them specifically

## Non-goals
- Strategy rotation on regression (deferred to future spec)
- Adaptive generation cap — granting extra cycles for monotonic progress or
  halting early for oscillation (deferred to future spec)
- Changing how acceptance criteria are evaluated (that is `0004p`'s scope)
- Delta-diff evaluation (separate concern)
- Reverting to best state when a cycle fails validation (only acceptance
  regression triggers revert; validation failures follow existing replan flow)

## Architecture

### New `RunState` fields

Two new fields on `RunState` in `internal/next/runstore/types.go`:

```go
BestPassCount     int    `json:"best_pass_count"`
BestPassCommitSHA string `json:"best_pass_commit_sha,omitempty"`
```

`BestPassCount` starts at 0. `BestPassCommitSHA` starts empty (no best state
recorded yet). Both persist in `run.json` automatically via existing JSON
serialization. Neither field requires `NormalizeNilFields` treatment (int
zero-value and empty string are valid initial states).

`ResetForNewCycle` in `internal/next/runstore/store.go` must NOT reset these
fields — they persist across the full run lifetime, like `FailureHistory` and
`TaskLineage`.

### New `GitOps` methods

The `GitOps` interface in `internal/next/specloop/stages/init.go` gains two
methods. The first:

```go
ResetHard(workDir, commitSHA string) error
```

Implementation: runs `git reset --hard <commitSHA>` in the worktree directory.
This is safe because worktree changes are committed per-cycle by the validate
stage before accept runs, so uncommitted work is not at risk.

### New `AcceptStage` dependency

`AcceptStage` in `internal/next/specloop/stages/accept.go` gains a `gitOps`
field (same `GitOps` interface already used by `InitStage` and
`ValidateStage`). Injected via `AcceptStageConfig`:

```go
type AcceptStageConfig struct {
    // ... existing fields ...
    GitOps   GitOps
    WorkDir  string // worktree path for git operations
}
```

### Accept stage regression logic

After the evaluator returns `AcceptanceResult` and the pass/fail/unclear
counts are computed (around the existing event emission in `accept.go`), insert
the regression guard:

1. Compute `passCount` from `result.Results` (count of `StatusPass`).
2. **If `passCount >= rs.BestPassCount`**: update `rs.BestPassCount = passCount`.
   Resolve `workDir` from `cfg.WorkDir` with fallback to `rs.WorktreePath`
   (same pattern as `ValidateStage`). If `workDir` is set and `gitOps` can
   resolve HEAD, update `rs.BestPassCommitSHA` to the current HEAD SHA.
   Continue with existing accept logic (return `Continue` or `ReplanFrom` as
   before).
3. **If `passCount < rs.BestPassCount` and `rs.BestPassCommitSHA != ""`**:
   - Emit `RegressionDetectedEvent`.
   - Identify regressed criteria: criteria that now show `fail` or `unclear`
     but were not present in the prior cycle's failure list. (Compare
     `result.Results` against the current `rs.AcceptanceResults` which
     holds the prior cycle's failure strings — any criterion absent from
     the prior failures but now failing represents a regression.)
   - Call `gitOps.ResetHard(workDir, rs.BestPassCommitSHA)` to revert the
     worktree (where `workDir` is resolved as described above).
   - Return `ReplanFrom` with a `FailureContext` that includes both the
     still-failing criteria from the best state AND a prefix annotation
     indicating regression was detected and reverted.
   - Do NOT update `rs.BestPassCount` or `rs.BestPassCommitSHA`.

### `HeadSHA` helper on `GitOps`

The second new `GitOps` method resolves the current HEAD commit SHA:

```go
HeadSHA(workDir string) (string, error)
```

Implementation: runs `git -C <workDir> rev-parse HEAD`. Used by the accept
stage to capture `BestPassCommitSHA` after a new best is established.

### New event type

In `internal/next/runstore/events.go`:

```go
type RegressionDetectedEvent struct {
    BaseEvent
    CurrentPassCount int      `json:"current_pass_count"`
    BestPassCount    int      `json:"best_pass_count"`
    RegressedCriteria []string `json:"regressed_criteria"`
    RevertedToSHA    string   `json:"reverted_to_sha"`
}
```

`NormalizeNilFields` is not needed (event types are write-once, not
round-tripped through nil-sensitive paths).

### Interaction with existing commit flow

The validate stage (`internal/next/specloop/stages/validate.go`) commits
worktree changes via `gitOps.CommitAll` when validation passes (line ~261).
This commit happens BEFORE the accept stage runs. The regression guard in the
accept stage therefore operates on already-committed state:

- **No regression**: the committed state stands; accept returns `Continue` or
  normal `ReplanFrom`.
- **Regression detected**: the committed state is reverted via `ResetHard` to
  the best-state SHA. The regressive commit is effectively discarded.

### Interaction with specloop replan flow

When the accept stage returns `ReplanFrom` due to regression, the specloop
(`internal/next/specloop/specloop.go`) processes it identically to a normal
acceptance failure replan — the `replanContext` flows through
`UpdateTaskLineage`, `UpdateFailureHistory`, and `AnnotateWithPersistentHints`
as usual. The only difference is that the failure strings carry a
`[REGRESSION REVERTED]` prefix so the planner knows this is a regression
scenario, not a first-time failure.

### Files in scope
- `internal/next/runstore/types.go` (new fields on `RunState`)
- `internal/next/runstore/store.go` (verify `ResetForNewCycle` does not reset new fields)
- `internal/next/runstore/events.go` (new `RegressionDetectedEvent`)
- `internal/next/specloop/stages/accept.go` (regression guard logic)
- `internal/next/specloop/stages/init.go` (`GitOps` interface: new methods)
- `internal/next/specloop/stages/accept_test.go` (unit tests)
- `internal/next/specloop/stages/accept_integration_test.go` (integration tests)

## Acceptance Criteria

1. `RunState` has fields `BestPassCount int` and `BestPassCommitSHA string`,
   both serialized to JSON with tags `best_pass_count` and
   `best_pass_commit_sha`. A round-trip through `json.Marshal` /
   `json.Unmarshal` preserves non-zero values.

2. After the accept stage evaluates criteria and the current pass count is
   greater than or equal to `rs.BestPassCount`, `rs.BestPassCount` is updated
   to the current pass count and `rs.BestPassCommitSHA` is updated to the
   current HEAD SHA of the worktree.

3. After the accept stage evaluates criteria and the current pass count is
   strictly less than `rs.BestPassCount` (and `rs.BestPassCommitSHA` is
   non-empty), the accept stage calls `gitOps.ResetHard` with the
   `BestPassCommitSHA` and returns `ReplanFrom`. It does NOT update
   `rs.BestPassCount` or `rs.BestPassCommitSHA`.

4. When regression is detected, a `RegressionDetectedEvent` is emitted with
   `CurrentPassCount`, `BestPassCount`, `RegressedCriteria`, and
   `RevertedToSHA` fields populated correctly.

5. The `FailureContext` returned on regression contains at least one failure
   string with the prefix `[REGRESSION REVERTED]` so the planner can
   distinguish regression replans from normal acceptance failure replans.

6. `ResetForNewCycle` does not reset `BestPassCount` or `BestPassCommitSHA`.
   A `RunState` with `BestPassCount=5` retains that value after
   `ResetForNewCycle` is called.

7. When `rs.BestPassCommitSHA` is empty (first cycle, no prior best), a pass
   count lower than `BestPassCount` (which is 0) is impossible, so the guard
   does not fire. The first acceptance evaluation always sets the initial best
   state.

8. All existing accept stage tests continue to pass.

## Scenarios

### Scenario: First cycle establishes baseline best state
**Given:** A fresh `RunState` with `BestPassCount=0` and
`BestPassCommitSHA=""`; the accept stage evaluator returns 10/13 passing
**When:** The accept stage runs
**Then:**
- `rs.BestPassCount` == 10
- `rs.BestPassCommitSHA` is set to the current HEAD SHA
- No `RegressionDetectedEvent` is emitted
- Accept returns `ReplanFrom` (since 3 criteria still fail)

### Scenario: Improvement updates best state
**Given:** `rs.BestPassCount=10`, `rs.BestPassCommitSHA="abc123"`; the accept
stage evaluator returns 12/13 passing
**When:** The accept stage runs
**Then:**
- `rs.BestPassCount` == 12
- `rs.BestPassCommitSHA` is updated to the new HEAD SHA (not "abc123")
- No `RegressionDetectedEvent` is emitted

### Scenario: Regression triggers revert and replan
**Given:** `rs.BestPassCount=12`, `rs.BestPassCommitSHA="abc123"`; the accept
stage evaluator returns 10/13 passing (2 criteria regressed)
**When:** The accept stage runs
**Then:**
- `gitOps.ResetHard` is called with `"abc123"`
- `rs.BestPassCount` remains 12
- `rs.BestPassCommitSHA` remains `"abc123"`
- A `RegressionDetectedEvent` is emitted with `CurrentPassCount=10`,
  `BestPassCount=12`, and `RegressedCriteria` listing the 2 regressed criteria
- Accept returns `ReplanFrom` with failure strings prefixed
  `[REGRESSION REVERTED]`

### Scenario: Best state survives resume
**Given:** A `RunState` serialized to `run.json` with `BestPassCount=12` and
`BestPassCommitSHA="abc123"`
**When:** The run is resumed (deserialized from `run.json`)
**Then:** `rs.BestPassCount` == 12 and `rs.BestPassCommitSHA` == `"abc123"`

## Validation

```bash
go test ./internal/next/runstore/... -run TestRunState
go test ./internal/next/runstore/... -run TestResetForNewCycle
go test ./internal/next/specloop/stages/... -run TestAccept
go test ./internal/next/specloop/...
go vet ./...
```
