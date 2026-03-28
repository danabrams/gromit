# Spec 0004q2 — Remediation Strategy Rotation + Adaptive Generation Cap

## spec_id
0004q2-remediation-strategy-rotation

## Depends On
0004q (best-state tracking and regression detection)

## Vision

After 0004q detects that a remediation cycle regressed the pass count and
reverts to the best-known commit, the pipeline currently has no mechanism to
avoid repeating the same failed approach. It re-runs the fix planner with the
same context, which tends to produce the same tasks, which regress in the same
way. Separately, the generation cap (`Budgets.MaxSpecCycles`) is static — it
cannot reward a run that is making steady progress or cut short a run that is
oscillating between states without converging.

This spec adds two mechanisms that sit downstream of 0004q's regression
detection: strategy rotation (vary the approach after regression) and adaptive
generation cap (extend or shorten the run based on pass-count trajectory).

## Summary

When regression is detected (via 0004q), the replan path narrows remediation
scope to only still-failing criteria, excludes file paths touched by the
regressive cycle's beads, and escalates the model tier after 2 consecutive
regressions. Additionally, the specloop tracks pass-count trajectory across
accept cycles: strictly increasing for the last 3 cycles grants +1 bonus
generation (up to a hard maximum of `MaxSpecCycles + 2`); an oscillating
pattern (up-down-up or down-up-down in the last 3 entries) halts the run
early and reports the best state.

## Goals

### Primary
- On regression replan, scope fix tasks to only still-failing criteria and
  exclude file paths from regressive beads
- Escalate model tier to "high" after 2 consecutive regressions
- Track pass-count trajectory and adjust generation cap dynamically

### Secondary
- Emit `strategy_rotated` and `generation_cap_adjusted` events for
  observability
- Halt early on oscillating pass-count trajectory rather than burning cycles

## Non-goals
- Changing criterion evaluation logic or the accept stage's pass/fail
  determination
- Delta-diff scoping of evaluation input (separate concern, see idea #3)
- Modifying the review stage or review thrash detection (0004o)
- Changing the initial plan stage or cycle-1 behavior

## Architecture

### New `RunState` fields (`internal/next/runstore/types.go`)

```go
ConsecutiveRegressions int      `json:"consecutive_regressions,omitempty"`
PassCountHistory       []int    `json:"pass_count_history,omitempty"`
ExcludedPaths          []string `json:"excluded_paths,omitempty"`
ForceHighTier          bool     `json:"force_high_tier,omitempty"`
```

`ConsecutiveRegressions`: incremented by 0004q's regression detection logic
each time a revert occurs; reset to 0 when a cycle improves the pass count
over the high-water mark.

`PassCountHistory`: appended after each accept cycle with the pass count for
that cycle. Used by the adaptive generation cap logic. Persists across cycles
via run.json.

`ExcludedPaths`: populated by the specloop's strategy rotation logic with the
union of `FilesChanged` from all tasks in the regressive cycle. Consumed by
the plan stage when building the fix plan prompt and by
`filterForbiddenFixTasks`. Reset each replan.

`ForceHighTier`: set to `true` when `ConsecutiveRegressions >= 2`. Consumed
by the execute stage to force all tasks in the next cycle to `ModelTier =
"high"`. Reset to `false` when `ConsecutiveRegressions` resets.

`NormalizeNilFields` in `RunState` maps `nil` → `[]int{}` for
`PassCountHistory` and `nil` → `[]string{}` for `ExcludedPaths`.

### Strategy rotation on regression replan (`internal/next/specloop/specloop.go`)

When 0004q's regression detection fires and triggers a replan, the specloop
populates `RunState` fields with narrowed scope before the plan stage reads
them:

1. **Still-failing criteria only**: The `FailureContext.Failures` (from the
   accept stage's `NextAction`) are filtered to only acceptance criteria that
   are still failing (from the reverted best-state evaluation), not criteria
   that were passing before the regressive cycle. These filtered failures flow
   into `rs.ReplanContext` (which is `[]string`) via the existing
   `DeduplicateFailures` path.

2. **Excluded file paths**: The specloop computes the union of
   `task.FilesChanged` from all tasks in the regressive cycle (tasks where
   `task.Cycle == regressiveCycle`) and stores it in `rs.ExcludedPaths`. The
   plan stage reads this field when building the fix plan request.

3. **Model tier escalation**: When `rs.ConsecutiveRegressions >= 2`, the
   specloop sets `rs.ForceHighTier = true`. The execute stage
   (`internal/next/specloop/stages/execute.go`) checks this flag in addition
   to the existing `ShouldEscalateModel` check and sets `task.ModelTier =
   "high"` for all tasks in the cycle when true.

### Fix planner integration (`internal/next/specloop/stages/plan.go`)

When building the `FixPlanRequest`, the plan stage reads `rs.ExcludedPaths`
and threads it through:

```go
fixReq := planner.FixPlanRequest{
    // ... existing fields ...
    ExcludedPaths: rs.ExcludedPaths, // new
}
```

New field on `FixPlanRequest` (`internal/next/planner/planner.go`):
```go
ExcludedPaths []string `json:"excluded_paths,omitempty"`
```

New `NormalizeNilFields` method on `FixPlanRequest` (in
`internal/next/planner/planner.go`) maps `nil` → `[]string{}` for
`ExcludedPaths` and for the existing `ArchitectureConstraints` field.

The fix plan prompt (`buildFixPlanPrompt`) includes a section when
`ExcludedPaths` is non-empty:

```
## Excluded Paths (Do Not Touch)
The following files were touched by a prior regressive cycle. Do NOT generate
tasks that modify these files — find alternative approaches:
- <path1>
- <path2>
```

The structural filter `filterForbiddenFixTasks` (in
`internal/next/specloop/stages/plan.go`) gains a third parameter
`excludedPaths []string` and rejects tasks whose `ExpectedTouchedArea`
intersects `excludedPaths`, in addition to the existing test-file constraint.

### Pass-count trajectory tracking (`internal/next/specloop/specloop.go`)

After each accept cycle completes (regardless of pass/fail), the specloop
appends the pass count to `rs.PassCountHistory`. The pass count is extracted
from the `AcceptanceResultEvent` emitted by the accept stage, or computed
from `rs.AcceptanceResults` (count of total criteria minus failures).

### Adaptive generation cap (`internal/next/specloop/budget.go`)

New method on `Budget`:
```go
func (b *Budget) GrantBonusCycle()
```
Increments an internal `bonusCycles` counter (max 2). `MaxCycles()` returns
`b.limits.MaxSpecCycles + b.bonusCycles` (currently it returns
`b.limits.MaxSpecCycles`). `CyclesExhausted()` is updated to compare against
`MaxCycles()` instead of `b.limits.MaxSpecCycles` directly.

New function in `specloop`:
```go
func evaluateTrajectory(history []int) TrajectorySignal
```

`TrajectorySignal` is one of: `TrajectoryImproving`, `TrajectoryOscillating`,
`TrajectoryInsufficient` (fewer than 3 data points).

- **Improving**: last 3 entries are strictly increasing (`h[n-2] < h[n-1] < h[n]`).
  The specloop calls `budget.GrantBonusCycle()` and emits
  `generation_cap_adjusted` with `adjustment: +1`.
- **Oscillating**: last 3 entries alternate direction (up-down or down-up).
  The specloop sets `rs.Status = StatusNeedsHuman` with
  `rs.TerminalReason = "oscillating_pass_count"` and emits
  `generation_cap_adjusted` with `adjustment: -remaining`. The best state
  from 0004q is preserved.
- **Insufficient**: no action; normal budget rules apply.

### New event types (`internal/next/runstore/events.go`)

```go
type StrategyRotatedEvent struct {
    BaseEvent
    ConsecutiveRegressions int      `json:"consecutive_regressions"`
    ExcludedPathCount      int      `json:"excluded_path_count"`
    StillFailingCount      int      `json:"still_failing_count"`
    ForceHighTier          bool     `json:"force_high_tier"`
}

type GenerationCapAdjustedEvent struct {
    BaseEvent
    Trajectory string `json:"trajectory"` // "improving" or "oscillating"
    Adjustment int    `json:"adjustment"` // +1 or negative
    NewCap     int    `json:"new_cap"`
}
```

Both must be added to `unmarshalEvent` in `events.go`.

### Integration point in specloop main loop

The trajectory evaluation and strategy rotation logic is called at the
boundary between cycles in `SpecLoop.Run`, after the replan context is
threaded into `rs.ReplanContext` and before `Budget.IncrementCycle()`:

```
[accept stage returns ReplanFrom]
  → 0004q: regression detection + revert (if regressed)
  → 0004q2: strategy rotation (populate rs.ExcludedPaths, rs.ForceHighTier)
  → 0004q2: trajectory evaluation (grant bonus or halt early)
  → Budget.IncrementCycle()
  → next cycle begins at plan stage
```

## Acceptance Criteria

1. When a regression replan is triggered (0004q reverted), the
   `rs.ReplanContext` contains only still-failing criteria — criteria that
   were passing before the regressive cycle are excluded.

2. When a regression replan is triggered, `rs.ExcludedPaths`
   contains the union of `FilesChanged` from all tasks in the regressive
   cycle. The fix planner prompt includes these paths in an "Excluded Paths"
   section.

3. `filterForbiddenFixTasks` rejects tasks whose `ExpectedTouchedArea`
   intersects `ExcludedPaths`, in addition to the existing test-file
   constraint.

4. When `rs.ConsecutiveRegressions >= 2`, all tasks in the next cycle have
   `ModelTier` set to `"high"` via `rs.ForceHighTier`.

5. After each accept cycle, the pass count is appended to
   `rs.PassCountHistory`. The history persists in run.json and survives
   resume.

6. When the last 3 entries of `PassCountHistory` are strictly increasing,
   `Budget.GrantBonusCycle()` is called and `MaxCycles()` returns
   `MaxSpecCycles + bonusCycles` (up to +2 bonus).

7. When the last 3 entries of `PassCountHistory` oscillate (up-down or
   down-up), the run halts early with `TerminalReason =
   "oscillating_pass_count"` and `Status = needs_human`.

8. `strategy_rotated` and `generation_cap_adjusted` events are emitted at
   the appropriate points, with the fields specified in the Architecture
   section.

## Scenarios

### Scenario: Regression narrows scope and excludes paths
**Given:** A run where cycle 2 passes criteria A, B (fails C); cycle 3's
remediation touches `internal/foo.go` and `internal/bar.go` but regresses
criterion B
**When:** 0004q reverts to the cycle-2 state and triggers a replan
**Then:**
- `rs.ReplanContext` contains only the failure string for criterion C (not B,
  which was passing at cycle 2)
- `rs.ExcludedPaths` contains `["internal/foo.go",
  "internal/bar.go"]`
- The fix planner prompt contains "Excluded Paths" listing both files
- A `strategy_rotated` event is emitted with `still_failing_count: 1` and
  `excluded_path_count: 2`

### Scenario: Two consecutive regressions escalate model tier
**Given:** A run where cycles 3 and 4 both regressed (reverted by 0004q),
so `rs.ConsecutiveRegressions == 2`
**When:** Cycle 5's tasks are created by the plan stage and executed
**Then:**
- `rs.ForceHighTier` is true
- All tasks in cycle 5 have `ModelTier == "high"`
- `strategy_rotated` event has `force_high_tier: true`

### Scenario: Monotonically improving pass count grants bonus generation
**Given:** `PassCountHistory` is `[8, 10, 12]` (strictly increasing over
the last 3 cycles) and `MaxSpecCycles` is 5 with 0 bonus cycles used
**When:** The trajectory evaluation runs after cycle 3's accept
**Then:**
- `Budget.GrantBonusCycle()` is called; `MaxCycles()` returns 6
- A `generation_cap_adjusted` event is emitted with
  `trajectory: "improving"`, `adjustment: 1`, `new_cap: 6`
- The run is allowed to continue for at least one more cycle

### Scenario: Oscillating pass count halts early
**Given:** `PassCountHistory` is `[10, 12, 10]` (up-down pattern)
**When:** The trajectory evaluation runs after cycle 3's accept
**Then:**
- `rs.Status` is set to `needs_human`
- `rs.TerminalReason` is `"oscillating_pass_count"`
- A `generation_cap_adjusted` event is emitted with
  `trajectory: "oscillating"`
- The best state from 0004q is preserved (not overwritten)

## Validation

```bash
go test ./internal/next/specloop/... -run TestStrategy
go test ./internal/next/specloop/... -run TestTrajectory
go test ./internal/next/specloop/... -run TestBudget
go test ./internal/next/specloop/stages/... -run TestPlan
go test ./internal/next/specloop/stages/... -run TestExecute
go test ./internal/next/runstore/... -run TestRunState
go test ./internal/next/runstore/... -run TestNormalize
go vet ./...
```
