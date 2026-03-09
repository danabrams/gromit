---
id: fix-v2-cost-and-duration-capture
spec: debug-investigation
created: 2026-03-08
decomposed: false
---

# Fix v2 Loop Cost and Duration Capture

## Research & Context

Investigation report: `.gromit/reports/debug-20260308-204536.md`
Related prior plan: `.gromit/plans/fix-v2-invocation-metadata-events.md` (covers event emission gaps; this plan covers data correctness bugs found in this investigation)

The v2 bead loop has four bugs causing incorrect or missing cost/duration data in bead completion events. v1 handles all of these correctly via `stampBuildAttribution()` and per-stage cost tracking in `IterationLog`.

## Architecture

Changes touch three layers:
1. **Bead loop** (`internal/v2/loop/bead_loop.go`) — fix token field, aggregate multi-stage costs
2. **Event bridge** (`internal/v2/event/bridge.go`) — propagate duration to IterationCompleteEvent
3. **Stage artifacts** (`internal/v2/stage/validate/`, `internal/v2/stage/review/`) — add cost/token fields to non-build stage results

The cost data flow is:
```
Provider.StreamInvoke() → LLMResponse{CostUSD, InputTokens, OutputTokens}
  → BuildArtifacts / ValidateArtifacts / ReviewArtifacts
  → BeadLoop aggregates across stages
  → BeadCompletedEvent{CostUSD, InputTokens, OutputTokens}
  → bridge → legacy BeadCompleteEvent + IterationCompleteEvent
```

## Tasks

### Task 1: Fix InputTokens field assignment in bead_loop.go
**Files:** `internal/v2/loop/bead_loop.go`
**Size:** Small (2 lines changed)

Two locations use the wrong field:
- Line 763: `evt.InputTokens = b.lastBuildArtifacts.Tokens` → change to `.InputTokens`
- Line 504: `inputTokens = ba.Tokens` → change to `ba.InputTokens`

Both should use `BuildArtifacts.InputTokens` (actual input count), not `.Tokens` (backward-compat total).

### Task 2: Fix bridge Duration: 0 in IterationCompleteEvent
**Files:** `internal/v2/event/bridge.go`
**Size:** Small (1 line changed)

Line 197: `Duration: 0` → `Duration: e.Duration`

The `BeadCompletedEvent` already carries `Duration` from `lastBuildArtifacts.Duration`. Just propagate it to the `IterationCompleteEvent` instead of hardcoding zero.

### Task 3: Add cost/token fields to ValidateArtifacts
**Files:** `internal/v2/stage/validate/validate.go`
**Size:** Small (10-15 lines)

When the validate stage invokes an LLM for auto-fix (the Claude-based fix path), capture `CostUSD`, `InputTokens`, `OutputTokens`, `Duration` in `ValidateArtifacts`. Currently `ValidateArtifacts` only has command-level info.

Add fields:
```go
type ValidateArtifacts struct {
    // ... existing fields ...
    FixCostUSD      float64
    FixInputTokens  int
    FixOutputTokens int
    FixDuration     time.Duration
}
```

Populate from the LLM response when the fix invocation runs.

### Task 4: Add cost/token fields to ReviewArtifacts
**Files:** `internal/v2/stage/review/review.go`
**Size:** Small (10-15 lines)

The review stage invokes an LLM. Capture cost/tokens in `ReviewArtifacts`:

```go
type ReviewArtifacts struct {
    // ... existing fields ...
    CostUSD      float64
    InputTokens  int
    OutputTokens int
    Duration     time.Duration
}
```

Populate from the LLM response in the review stage's `Run()`.

### Task 5: Aggregate multi-stage costs in BeadCompletedEvent
**Files:** `internal/v2/loop/bead_loop.go`
**Size:** Medium (20-30 lines)

After task 1-4, the bead loop needs to aggregate costs from ALL stages, not just build. In `runStageEntry()` or a new accumulator:

1. Add fields to BeadLoop: `accumulatedCostUSD float64`, `accumulatedInputTokens int`, `accumulatedOutputTokens int`
2. Reset these at the start of each bead in `processBead()`
3. After each stage completes, check for cost-carrying artifacts (BuildArtifacts, ValidateArtifacts, ReviewArtifacts) and add to accumulators
4. In `emitBeadCompleted()`, use accumulated totals instead of only `lastBuildArtifacts`

This mirrors v1's approach where `stampBuildAttribution()` is called after each LLM invocation and costs are summed in the `IterationLog`.

### Task 6: Verify Codex provider cost extraction
**Files:** `internal/provider/codex/` or equivalent provider adapter
**Size:** Investigation + Small fix (5-10 lines)

Verify that the Codex provider adapter extracts `CostUSD` from the provider's stream output. If Codex uses a different stream format than Claude CLI, the cost parsing may silently return zero.

Check:
- Does the Codex provider's stream output include a `result` event with `total_cost_usd`?
- If not, does the provider adapter have its own cost extraction?
- If cost is genuinely unavailable from the provider API, document this and emit a sentinel value or log warning

## Dependencies

```
Task 1 ─── (no deps, immediate fix)
Task 2 ─── (no deps, immediate fix)
Task 3 ─── before Task 5
Task 4 ─── before Task 5
Task 5 ─── after Tasks 3, 4
Task 6 ─── (independent investigation)
```

Tasks 1, 2, and 6 are independent and can be done in parallel.
Tasks 3 and 4 can be done in parallel.
Task 5 depends on Tasks 3 and 4.

## Testing Strategy

### Task 1: InputTokens field fix
- Unit test: create BuildArtifacts with `Tokens: 100, InputTokens: 60, OutputTokens: 40`
- Call `emitBeadCompleted` → verify event has `InputTokens: 60` not `100`
- Call `emitBuildInvocationComplete` → verify same

### Task 2: Bridge Duration fix
- Unit test in bridge_test.go: convert `BeadCompletedEvent{Duration: 5*time.Second}`
- Verify `IterationCompleteEvent.Duration == 5*time.Second` not zero

### Tasks 3-4: Stage artifact cost fields
- Unit test: mock LLM provider returning known cost/tokens
- Run validate/review stage → verify artifacts contain cost/token data
- Verify zero cost when no LLM invocation occurs (pure command validation)

### Task 5: Multi-stage cost aggregation
- Integration test: mock bead loop with build ($0.50) + validate-fix ($0.10) + review ($0.05)
- Verify `BeadCompletedEvent.CostUSD == 0.65` (sum of all stages)
- Verify token counts are also summed

### Task 6: Provider cost verification
- Check Codex provider stream output for cost fields
- If available: verify end-to-end with real invocation
- If unavailable: add test asserting warning log when cost is zero

### Regression
- All existing bead_loop_test.go tests pass
- All existing bridge_test.go tests pass
- Run `go test ./internal/v2/...` for full v2 coverage

## Relationship to Existing Plan

The plan at `.gromit/plans/fix-v2-invocation-metadata-events.md` addresses event *emission* gaps (no ModelSelectedEvent, no BuildInvocationStartEvent, etc.). This plan addresses data *correctness* bugs in the metrics that ARE being emitted. Both plans should be executed — this plan's Tasks 1-2 are quick fixes that can land immediately; Tasks 3-5 complement the existing plan's Task 3 (propagate build artifacts to bead_complete).
