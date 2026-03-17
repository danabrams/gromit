# Spec 0003d — Same-Task Repeated Failure Escalation

## spec_id
0003d-repeated-failure-escalation

## Depends on
None

## Vision
In an observed run, the same task — "add FailureHistory/ScenarioTestsWritten to RunState in types.go" — failed 7 times across 6 replan cycles. Each replan generated a new task ID with a slightly reworded objective, but it was the same edit failing over and over. The planner had no visibility into prior attempts' error output, and the model tier never changed. The system needs to track task lineage across replans, include prior error context for the planner, and escalate model tier when a task repeatedly fails at a lower tier.

## Summary
Add task lineage tracking to the replan cycle. When the planner generates fix tasks, it tags each with a `fixes` field referencing the failed task ID it addresses. The specloop tracks consecutive failure counts per lineage chain. After a configurable threshold (default 2) consecutive failures, the prior attempt's error output is included in the replan context for the planner. After threshold+1 failures, the model tier is escalated (Sonnet → Opus) for that specific task. Thresholds are configurable via execution policy.

## Goals
### Primary
- Track task lineage across replan cycles via `fixes: <task-id>` tags on fix tasks
- Include prior failed attempt's error output in replan context after N consecutive failures
- Escalate model tier (Sonnet → Opus) for tasks that fail N+1 consecutive times
- Make escalation thresholds configurable via execution policy

### Secondary
- Provide visibility into failure lineage in run.json/evidence for debugging

## Non-goals
- Heuristic-based task matching across replans (file overlap, objective similarity) — lineage is established via explicit `fixes` tags from the planner, plus automatic root creation for first-time failures
- Escalation beyond Opus (Opus is the ceiling)
- De-escalation after success (once escalated, stay escalated for that lineage)
- Changing the fix planner's task decomposition logic — only adding the `fixes` field requirement
- Deferred: infrastructure detection (0003a), replan context dedup (0003b), review degradation (0003c)
- Cross-spec interaction with 0003b: The `prior-attempt-error:` strings appended to `rs.ReplanContext` use a non-`contract:` prefix and will pass through 0003b's `DeduplicateFailures` unchanged (if both specs are implemented). No special handling needed.

## Architecture

### Task Lineage

The `Task` struct in `internal/next/runstore/types.go` gains a `Fixes` field:

```go
type Task struct {
    // ... existing fields ...
    Fixes    string `json:"fixes,omitempty"`     // Task ID this fix addresses (e.g., "t-001")
}
```

The `TaskDef` struct in `internal/next/planner/types.go` also gains a `Fixes` field:

```go
type TaskDef struct {
    // ... existing fields ...
    Fixes    string `json:"fixes,omitempty"`     // Task ID this fix addresses (e.g., "t-001")
}
```

During task conversion, `PlanStage.Run()` copies `td.Fixes` to `task.Fixes` (alongside the existing copies of `td.TaskID`, `td.Objective`, etc.).

The fix planner prompt is updated to instruct the LLM to include a `fixes` field in each generated fix task, referencing the failed task ID it's addressing. This is a prompt change, not a code logic change — the planner already generates structured task JSON. No changes to `FixPlanRequest` or `PlanStage` are needed for error context — the `prior-attempt-error` strings go into `rs.ReplanContext` which is already read by the plan stage.

The fix planner prompt is defined in the planner package (the template used by `FixPlanRequest`). The prompt addition instructs: "For each fix task, include a fixes field with the task ID of the failed task it addresses (e.g., fixes: t-001)."

Note: The existing `FailuresAddressed []string` field on `Task` serves a different purpose — it lists failure strings from the replan context that a fix task addresses (human-readable descriptions). The new `Fixes string` field is a task-ID reference for lineage tracking. They are complementary.

### Lineage Chain Tracking

A new `TaskLineage` map on RunState tracks consecutive failure counts:

```go
type RunState struct {
    // ... existing fields ...
    TaskLineage map[string]TaskLineageEntry `json:"task_lineage,omitempty"`
}

type TaskLineageEntry struct {
    OriginalTaskID  string   `json:"original_task_id"`   // Root of the chain
    ConsecutiveFails int     `json:"consecutive_fails"`   // How many times this lineage has failed
    LastError       string   `json:"last_error,omitempty"` // Error output from most recent failure (truncated to 2000 chars)
    ChainIDs        []string `json:"chain_ids"`           // All task IDs in this lineage: [t-001, t-015, t-028, ...]
}
```

The map is keyed by the original task ID (root of the chain). When a fix task with `fixes: t-015` fails, and `t-015` itself had `fixes: t-001`, the system traces back to the root `t-001` and increments that entry's counter.

### Lineage Resolution

After task execution, in the specloop's task result processing:

```go
func resolveLineageRoot(lineage map[string]TaskLineageEntry, fixesTaskID string) string {
    // Look up which lineage chain contains fixesTaskID:
    // (1) Search each TaskLineageEntry.ChainIDs for fixesTaskID
    // (2) If found, return that entry's key (the root task ID)
    // (3) If not found, fixesTaskID is itself a root — return it
    // Guard against malformed data (max depth, e.g. 50 iterations)
}
```

Note: `rs.Tasks` is reset each cycle by `ResetForNewCycle`, so prior-cycle task objects are not available for chain walking. Instead, `resolveLineageRoot` searches the `TaskLineage` map — each `TaskLineageEntry` stores the full `ChainIDs` list. The resolution uses the current task's `Fixes` field to look up which lineage root it belongs to, avoiding any need for prior-cycle task objects.

### Lineage Tracking Location

Lineage updates happen in the specloop (`internal/next/specloop/specloop.go`) after task execution completes and before replanning — in the same block where persistent-failure hints are appended to `rs.ReplanContext` (around line 131). Specifically:

1. After task results are collected, for each failed task:
   - If the task has a `Fixes` field, resolve to lineage root; otherwise treat the task as a new lineage root
   - Increment `ConsecutiveFails` for that lineage
   - Store the failed task's error output in `LastError` (truncated to 2000 characters to prevent context window blowup)

2. The `prior-attempt-error` strings are appended to `rs.ReplanContext` in the same location where persistent-failure hints are appended. The plan stage already reads `rs.ReplanContext` — no changes to `FixPlanRequest` or `PlanStage` are needed.

### Escalation Logic

In the specloop, after lineage tracking and before replanning:

1. For lineages with `ConsecutiveFails >= errorContextThreshold` (default 2): include `LastError` in the replan context as `"prior-attempt-error: <task-id>: <error output>"`

2. For lineages with `ConsecutiveFails >= modelEscalationThreshold` (default 3): mark the lineage for model tier escalation

3. Task-level model tier override:
   - The execute stage (or specloop before calling execute) sets `task.ModelTier` on tasks whose lineage is marked for escalation. The `TaskRunner` reads `task.ModelTier` to select the model — if escalated, it uses Opus instead of the policy-defined tier

The escalation check happens at the START of task execution in each cycle. Before running a task, the execute stage checks if the task has a `Fixes` field, resolves to the lineage root using `TaskLineage`, and if that lineage has `ConsecutiveFails >= modelEscalationThreshold`, sets `task.ModelTier` to Opus before passing to the TaskRunner.

### Execution Policy Configuration

Add an `EscalationConfig` struct to the `Policy` struct in `internal/next/execpolicy/policy.go`:

```go
type EscalationConfig struct {
    ErrorContextThreshold   int `json:"error_context_threshold"`
    ModelEscalationThreshold int `json:"model_escalation_threshold"`
}
```

Added to the `Policy` struct:

```go
type Policy struct {
    // ... existing fields ...
    Escalation EscalationConfig `json:"escalation"`
}
```

Defaults in `DefaultPolicy()`:

```go
Escalation: EscalationConfig{
    ErrorContextThreshold:    2,
    ModelEscalationThreshold: 3,
},
```

Policy JSON example:

```json
{
  "escalation": {
    "error_context_threshold": 2,
    "model_escalation_threshold": 3
  }
}
```

Both default to 2 and 3 respectively if not specified in the policy (via the unmarshal-into-defaults pattern).

### Lineage Reset on Success

When a task in a lineage chain succeeds:
- The lineage entry's `ConsecutiveFails` resets to 0
- `LastError` is cleared
- The lineage entry is kept (not deleted) so the chain history is preserved

### TaskLineage Persistence

`TaskLineage` is NOT reset in `ResetForNewCycle` — it persists across replan cycles (same as `FailureHistory`).

### Nil-Field Normalization

`RunState.NormalizeNilFields()` must initialize `TaskLineage` if nil (to `map[string]TaskLineageEntry{}`). `TaskLineageEntry.ChainIDs` also needs nil normalization (to `[]string{}`).

## Acceptance Criteria

1. The fix planner prompt instructs the LLM to include a `fixes` field referencing the failed task ID, and the system handles both tagged and untagged fix tasks gracefully
2. The specloop tracks task lineage chains by resolving `fixes` references to the root task ID
3. `TaskLineage` on RunState stores consecutive failure counts and last error output per lineage chain
4. When a lineage's consecutive failure count reaches the `error_context_threshold` (default 2), the prior attempt's error output is included in replan context with a `"prior-attempt-error: "` prefix
5. When a lineage's consecutive failure count reaches the `model_escalation_threshold` (default 3), the task is executed with Opus instead of the policy-defined tier
6. Both thresholds are configurable via the execution policy's `escalation` section
7. When a task in a lineage chain succeeds, the lineage's consecutive failure count resets to 0
8. `TaskLineage` persists across replan cycles (not reset by `ResetForNewCycle`)
9. Lineage resolution handles chains of arbitrary depth (t-028 → t-015 → t-001) with cycle protection
10. `LastError` is truncated to 2000 characters to prevent context window blowup
11. All existing pipeline tests continue to pass

## Scenarios

### Scenario: First failure of a task, no escalation
**Given:** A run on cycle 1 where task `t-001` ("Add FailureHistory field to types.go") fails with error output "undefined: FailureHistory" and has no `fixes` field (it's an original task)
**When:** The specloop processes task results and triggers a replan
**Then:** A new `TaskLineage` entry is created keyed by `t-001` with `ConsecutiveFails: 1`, `LastError: "undefined: FailureHistory"`, `ChainIDs: ["t-001"]`. The replan context does NOT include prior error output (threshold not met). The fix plan is generated at normal model tier.

### Scenario: Second failure triggers error context inclusion
**Given:** A run where `TaskLineage["t-001"]` has `ConsecutiveFails: 1`. The planner generated fix task `t-015` with `fixes: "t-001"`. Task `t-015` fails with error "cannot use FailureHistory (variable of type map[string]int) as field in struct literal". The execution policy has `error_context_threshold: 2`.
**When:** The specloop processes task results and triggers a replan
**Then:** `TaskLineage["t-001"]` is updated to `ConsecutiveFails: 2`, `LastError: "cannot use FailureHistory..."`, `ChainIDs: ["t-001", "t-015"]`. The replan context includes: `"prior-attempt-error: t-015: cannot use FailureHistory (variable of type map[string]int) as field in struct literal"`. The fix plan is still generated at normal model tier (model escalation threshold not reached).
**Notes:** The planner now sees what specifically went wrong last time, enabling a more targeted fix.

### Scenario: Third failure triggers model escalation
**Given:** A run where `TaskLineage["t-001"]` has `ConsecutiveFails: 2`, `ChainIDs: ["t-001", "t-015"]`. The planner generated fix task `t-028` with `fixes: "t-015"`. Task `t-028` fails. The execution policy has `model_escalation_threshold: 3`.
**When:** The specloop processes task results and triggers a replan
**Then:** `TaskLineage["t-001"]` is updated to `ConsecutiveFails: 3`, `ChainIDs: ["t-001", "t-015", "t-028"]`. The replan context includes prior error output. The next fix task for this lineage is executed with Opus tier instead of Sonnet.

### Scenario: Fix task succeeds, lineage resets
**Given:** A run where `TaskLineage["t-001"]` has `ConsecutiveFails: 3`, was escalated to Opus. The planner generated fix task `t-035` with `fixes: "t-028"`. Task `t-035` succeeds.
**When:** The specloop processes task results
**Then:** `TaskLineage["t-001"]` is updated to `ConsecutiveFails: 0`, `LastError: ""`. The lineage entry is kept (not deleted). `ChainIDs` is updated to include `t-035`.
**Notes:** Success at any point resets the counter. The chain history is preserved for debugging.

### Scenario: Fix task without fixes field (planner didn't tag it)
**Given:** A fix task `t-020` on cycle 2 with `kind: "fix"` but no `fixes` field (the planner didn't include lineage tagging)
**When:** The specloop processes task results
**Then:** The task is treated as a new lineage root — a new entry `TaskLineage["t-020"]` is created if it fails. No error context or escalation from prior cycles applies. This is graceful degradation — the feature still works for tagged tasks.
**Notes:** The planner is instructed to include `fixes` but it's LLM-generated, so we can't guarantee it. Untagged fix tasks just don't get the escalation benefit.

### Scenario: Lineage chain resolution with depth > 2
**Given:** Task lineage: `t-028` has `fixes: "t-015"`, `t-015` has `fixes: "t-001"`, `t-001` has no `fixes` field
**When:** `resolveLineageRoot` is called for `t-028`
**Then:** Returns `"t-001"` — the root of the chain. Chain traversal: t-028 → t-015 → t-001 (no fixes) → root found.

### Scenario: Cycle in fixes chain
**Given:** Task `t-002` has `fixes: "t-001"` and task `t-001` has `fixes: "t-002"` (a cycle)
**When:** `resolveLineageRoot` is called for `t-002`
**Then:** Returns one of them without infinite looping, using the max-depth guard. The lineage entry is created normally.
**Notes:** Cycles should not occur in practice (the planner always references earlier tasks), but the max-depth guard prevents hangs if they do.

### Scenario: Custom thresholds via execution policy
**Given:** An execution policy with `"escalation": {"error_context_threshold": 1, "model_escalation_threshold": 2}`
**When:** A task fails for the first time (ConsecutiveFails becomes 1)
**Then:** Prior error output is included in replan context (threshold 1 reached). After the second failure, model tier is escalated (threshold 2 reached).
**Notes:** Lower thresholds mean more aggressive escalation — useful for expensive specs where fast convergence matters.

### Scenario: Default thresholds when policy omits escalation section
**Given:** An execution policy JSON that has no `"escalation"` section
**When:** The specloop reads escalation thresholds
**Then:** Defaults are used: `error_context_threshold: 2`, `model_escalation_threshold: 3`

### Scenario: LastError is truncated for long output
**Given:** A task fails with 5000 characters of compiler output
**When:** Lineage tracking stores LastError
**Then:** LastError is truncated to 2000 characters. The truncated output still provides the key error message (typically at the beginning of compiler output).

## Validation
- `go test ./internal/next/specloop/ -count=1`
- `go test ./internal/next/runstore/ -count=1`
- `go test ./internal/next/planner/ -count=1`
- `go test ./cmd/gromit-next/ -count=1`
- `go vet ./...`
