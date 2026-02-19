---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T03:15:40Z"
id: tdd-phase-metrics
source_spec: tdd-phase-metrics
---

# Per-Phase Metrics for TDD Cycles — Implementation Plan

**Goal:** Track granular per-phase metrics (tokens, cost, duration, success) for each TDD/ATDD cycle phase, aggregate them per bead, and surface them via `gromit stats --tdd`.

**Architecture:** Accumulate `PhaseMetric` records in `IterationResult` during methodology execution, map them to `TDDPhaseRecord` JSONL entries alongside the existing iteration log, compute a `TDDSummary` aggregate per bead, and read/aggregate in a new `tddstats.go` for the stats command.

**Tech Stack:** Go, existing JSONL logging infrastructure, existing stats command pattern

**Spec:** `.gromit/specs/tdd-phase-metrics.md`

---

## Architecture

**Data Flow:**
```
Phase execution (process_methodology.go, callbacks.go)
  → PhaseMetric appended to bc.Result.PhaseMetrics
  → writeIterationLog() in logging.go
    → maps each PhaseMetric → TDDPhaseRecord, writes to JSONL
    → computes TDDSummary from PhaseMetrics, writes to JSONL
  → BuildContinuousMetrics reads both record types
  → gromit stats --tdd reads and aggregates
```

**Key Design Decisions:**

1. **PhaseMetric lives in runtypes, TDDPhaseRecord lives in logger.** This follows the existing IterationResult → IterationLog pattern and avoids circular imports. The mapping happens in `runner/logging.go`.

2. **Token capture for red/refactor phases.** The ATDD invokeFn in `callbacks.go` currently uses a bare EventHandler. Add a `StreamStats` alongside it to capture token data (same pattern as `runRefactorWithRouter` which already has `StreamStats` but discards the cost data). For refactor, expose the existing `StreamStats.CostData()` through the callback return path.

3. **One PhaseMetric per phase per cycle** (not per retry). The record captures the final attempt's data. Retries within a phase are already tracked by the escalation handler's retry counters.

4. **Green phase tokens from bc.Result.** The green phase writes tokens directly to `bc.Result` via `makeInvokeFn`. Snapshot these values after `executeWithRetry` returns, before the next cycle overwrites them.

**Integration Points:**
- `process_methodology.go:75-102` — After `RunAcceptanceTestsWithRetry()` returns (red phase)
- `process_methodology.go:104-179` — After `executeWithRetry()` returns (green phase), inside the for loop
- `process_methodology.go:234-329` — After `RunRefactorPhase()` returns (refactor phase)
- `process_methodology.go:306-326` — After `VerifyAcceptanceTestsPass()` returns (acceptance verification)
- `callbacks.go:280-442` — ATDD invokeFn needs StreamStats for token capture
- `process.go:280-330` — `runRefactorWithRouter` needs to return `CostData()` alongside `claude.Result`

**Files to Modify:**
- `internal/runner/runtypes/types.go` — Add `PhaseMetric` struct and `PhaseMetrics` field
- `internal/logger/logger.go` — Add `TDDPhaseRecord`, `TDDSummaryRecord`, logging methods
- `internal/runner/process_methodology.go` — Add phase capture hooks at each boundary
- `internal/runner/callbacks.go` — Add StreamStats to ATDD invokeFn, expose token data
- `internal/runner/process.go` — Return StreamStats CostData from `runRefactorWithRouter`
- `internal/runner/logging.go` — Write phase records and summary to JSONL
- `cmd/gromit/stats.go` — Add `--tdd` flag and TDD stats display

**Files to Create:**
- `internal/logger/tddstats.go` — TDD metrics reading and aggregation
- `internal/logger/tddstats_test.go` — Tests for aggregation

**Tradeoffs:**
- **Extend existing JSONL vs new file:** Chose existing file with `type` discriminator per spec decision #1. One queryable stream, but mixed record types.
- **Accumulate then write vs inline logging:** Chose accumulation in `IterationResult.PhaseMetrics`. Enables summary computation from the same data and keeps the write path simple.
- **Token capture for ATDD invokeFn:** Requires adding StreamStats to the ATDD callback in `makeMethodologyExec`. Moderate refactor of that callback, but follows the pattern already used by `runRefactorWithRouter`.

---

## Test Strategy

**Unit Tests:**
- `PhaseMetric` and `TDDPhaseRecord` JSON round-trip
- `TDDSummary` computation from phase records (success rates, token sums, cycle counts)
- TDD stats aggregation from JSONL (per-phase success rates, avg cycles per bead, cost per cycle)
- Stats command output formatting (text and JSON modes)

**Integration Tests:**
- Phase records appended to `IterationResult.PhaseMetrics` during methodology loop
- Phase records and summary written to JSONL after iteration log
- Token data captured for green phase from `bc.Result` snapshot
- Token data captured for red and refactor phases from StreamStats

**Edge Cases:**
- No TDD/ATDD active — zero phase records, no summary written
- Partial completion (red succeeds, green fails) — records for completed phases only
- Empty JSONL — stats command handles gracefully
- Mixed record types in JSONL — tdd_phase records filtered correctly

**Mocking Strategy:**
- Phase capture tests use existing mock pattern with stub `InvokeFn`/`ValidateFn` callbacks
- Stats tests use real JSONL in `t.TempDir()` (existing `modelstats_test.go` pattern)
- Logging tests verify JSONL output via temp file reads

**Test Files:**
- `internal/logger/logger_test.go` — extend with TDD record serialization
- `internal/logger/tddstats_test.go` — aggregation, JSONL reading, summary computation
- `internal/runner/process_methodology_test.go` — phase capture integration
- `internal/runner/logging_test.go` — phase record and summary writing
- `cmd/gromit/stats_test.go` — extend with --tdd flag tests

---

## Implementation Tasks

### Task 1: Add PhaseMetric type and TDD record types

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Test: `internal/logger/logger_test.go`

**What to Do:**
Define the `PhaseMetric` struct in runtypes with all per-phase fields from the spec: phase name, cycle number, bead/spec ID, model, tier, input/output tokens, duration_ms, success, escalated, escalated_from, criteria_targeted, criteria_covered_count, criteria_total. Add `PhaseMetrics []PhaseMetric` field to `IterationResult` (zero-value safe, `omitempty`).

Define `TDDPhaseRecord` in logger with `Type string` field defaulting to `"tdd_phase"`, JSON tags matching IterationLog patterns (snake_case, omitempty). Define `TDDSummaryRecord` with `Type string` defaulting to `"tdd_summary"` and aggregate fields: total_cycles, total_invocations, total_input_tokens, total_output_tokens, total_cost_usd, total_duration_ms, coverage_rate, phase_success_rates (map), avg_tokens_per_cycle, escalation_count, handoff_content_tokens.

Add `LogTDDPhase(*TDDPhaseRecord) error` and `LogTDDSummary(*TDDSummaryRecord) error` methods to Logger using the existing encoder pattern.

**Acceptance Criteria:**
- `PhaseMetric` struct has all spec-required fields; `PhaseMetrics` field exists on `IterationResult`
- `TDDPhaseRecord` and `TDDSummaryRecord` serialize to JSON with correct type discriminators
- `LogTDDPhase` and `LogTDDSummary` write to the same JSONL file as `LogIteration`

**Dependencies:** None

### Task 2: Add token capture to ATDD invokeFn

**Files:**
- Modify: `internal/runner/callbacks.go`
- Test: `internal/runner/callbacks_test.go`

**What to Do:**
In `makeMethodologyExec()`, modify the ATDD `invokeFn` callback (line ~280) to create a `logger.NewStreamStats()` alongside the existing `EventHandler`. Wire the StreamStats into `ParseAndLogEvent` calls (same pattern as `runRefactorWithRouter` in process.go:294-300). After `streamInvoke` completes, extract `stats.CostData()` and store the tokens on `bc.Result` (input_tokens, output_tokens, cost_usd) so the phase capture hook in process_methodology.go can read them.

The key change is replacing the bare `EventHandler` with one that also feeds a `StreamStats`, then propagating cost data to `bc.Result` after the invocation.

**Acceptance Criteria:**
- ATDD invokeFn creates and uses a StreamStats for token tracking
- After ATDD invocation, bc.Result.InputTokens/OutputTokens/CostUSD reflect the red phase's token usage
- Existing ATDD behavior (heartbeat, fallback, error handling) is unchanged

**Dependencies:** Task 1

**Notes:** The invokeFn currently overwrites bc.Result token fields. The phase capture hook (Task 4) will snapshot these values before they get overwritten by subsequent phases.

### Task 3: Expose token data from refactor invocation

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/methodology/refactor.go`
- Test: `internal/runner/process_test.go`

**What to Do:**
`runRefactorWithRouter` (process.go:280) already creates a `StreamStats` but discards the cost data. After the invocation completes (line ~302), extract `stats.CostData()` and return it alongside the `claude.Result`. This requires changing the return type or adding a side channel.

Simplest approach: change `RefactorInvokeFn` signature to return `(*claude.Result, int, int, float64, error)` adding input tokens, output tokens, and cost. Update `RunRefactorPhase` in executor.go to capture these values and store them on `bc.Result` (same pattern as the ATDD invokeFn token write). Update the wiring in `makeMethodologyExec`.

**Acceptance Criteria:**
- `runRefactorWithRouter` returns token/cost data from its StreamStats
- `RunRefactorPhase` propagates refactor token data to `bc.Result`
- Existing refactor behavior (validation, retry, revert) is unchanged

**Dependencies:** Task 1

**Notes:** An alternative to changing the signature is adding a `RefactorStats` field to `BeadContext` that `runRefactorWithRouter` writes to directly. This avoids changing the function type but couples the runner to the callback.

### Task 4: Capture phase metrics at methodology boundaries

**Files:**
- Modify: `internal/runner/process_methodology.go`
- Test: `internal/runner/process_methodology_test.go` (or new file)

**What to Do:**
Add a helper `recordPhaseMetric(bc, phase, cycleNumber, start time.Time, success bool)` that creates a `PhaseMetric` from current `bc` state (model, tier, tokens from bc.Result, bead/spec ID) and appends it to `bc.Result.PhaseMetrics`.

Hook it at four boundaries:
1. **Red phase** (line ~90): After `RunAcceptanceTestsWithRetry` returns, before clearing PromptCtx. Capture `start` before the call (line ~86), record with `success = (err == nil)`.
2. **Green phase** (line ~116): After `executeWithRetry()` returns in `executeBuildAndMethodologyLoop`. Snapshot bc.Result token values into the PhaseMetric.
3. **Refactor phase** (line ~261): After `RunRefactorPhase` returns. Duration from before the call.
4. **Acceptance verification** (line ~319): After `VerifyAcceptanceTestsPass` returns.

Track `cycleNumber` in `executeBuildAndMethodologyLoop` by adding a counter variable to the for loop.

**Acceptance Criteria:**
- PhaseMetric appended after each of the four phases with correct phase name and cycle number
- Duration captured as wall-clock time for each phase
- Token/cost data present for green phase (from bc.Result snapshot)
- No phase records when TDD/ATDD not active

**Dependencies:** Tasks 1, 2, 3

### Task 5: Write phase records and summary to JSONL

**Files:**
- Modify: `internal/runner/logging.go`
- Test: `internal/runner/logging_test.go`

**What to Do:**
Add `writeTDDMetrics(result *IterationResult)` method on Runner. Called from `writeIterationLog` (or alongside it in the caller) when `len(result.PhaseMetrics) > 0`.

For each `PhaseMetric` in `result.PhaseMetrics`, map to a `TDDPhaseRecord` and call `r.logger.LogTDDPhase()`.

Compute the `TDDSummaryRecord` from the phase metrics:
- `total_cycles` = max cycle_number across records
- `total_invocations` = len(PhaseMetrics)
- Token/cost sums across all phases
- `phase_success_rates` = per-phase success count / total count
- `coverage_rate` = last record's criteria_covered_count / criteria_total
- `escalation_count` = count of records where escalated=true

Call `r.logger.LogTDDSummary()` with the computed summary.

**Acceptance Criteria:**
- Phase records written as tdd_phase entries in the same JSONL file
- TDDSummary computed correctly and written as tdd_summary entry
- Nothing written when PhaseMetrics is empty (non-TDD beads)

**Dependencies:** Tasks 1, 4

### Task 6: Add TDD metrics reading and aggregation

**Files:**
- Create: `internal/logger/tddstats.go`
- Create: `internal/logger/tddstats_test.go`

**What to Do:**
Add functions to read and aggregate TDD metrics from JSONL:

`ReadTDDPhaseRecords(logsDir string) ([]TDDPhaseRecord, error)` — glob `run-*.jsonl` files, scan lines, unmarshal records with `"type": "tdd_phase"`, return sorted by timestamp.

`ReadTDDSummaries(logsDir string) ([]TDDSummaryRecord, error)` — same for `"type": "tdd_summary"`.

`TDDStats` struct with display-ready fields: AvgCyclesPerBead, PhaseSuccessRates map[string]float64, AvgCostPerCycle, AvgTokensPerCycle, TotalBeads, TotalCycles, EscalationPatterns map[string]int (phase → count), CoverageRateDistribution.

`AggregateTDDStats(phases []TDDPhaseRecord, summaries []TDDSummaryRecord) *TDDStats` — compute aggregated stats across all beads.

Tests with real JSONL files in `t.TempDir()` following the `modelstats_test.go` pattern.

**Acceptance Criteria:**
- ReadTDDPhaseRecords correctly filters tdd_phase records from mixed JSONL
- AggregateTDDStats computes correct per-phase success rates, avg cycles, cost per cycle
- Handles empty input gracefully (returns zero-value TDDStats, no panic)

**Dependencies:** Task 1

### Task 7: Add --tdd flag to stats command

**Files:**
- Modify: `cmd/gromit/stats.go`
- Test: `cmd/gromit/stats_test.go`

**What to Do:**
Add `--tdd` bool flag to the stats cobra command. When set, call `logger.ReadTDDPhaseRecords()` and `logger.ReadTDDSummaries()`, then `logger.AggregateTDDStats()`.

Text output section:
```
TDD Cycle Metrics:
  Beads with TDD:        12
  Avg cycles per bead:   2.3
  Total cycles:          28

  Per-Phase Success Rates:
    red:                  85.7%
    green:                92.1%
    refactor:             88.4%
    verification:         96.4%

  Cost & Tokens:
    Avg cost per cycle:   $0.042
    Avg tokens per cycle: 2,150

  Escalation Patterns:
    red:    3 escalations
    green:  1 escalation
```

JSON output: add `tdd_metrics` key to the existing JSON output object.

Handle no TDD data: print "No TDD metrics found." and return.

**Acceptance Criteria:**
- `gromit stats --tdd` displays TDD summary in text format
- `gromit stats --tdd --json` includes tdd_metrics in JSON output
- Graceful message when no TDD data exists

**Dependencies:** Task 6

---

## Notes

- **Token capture complexity:** Tasks 2 and 3 modify invoke callbacks to capture token data. These are the riskiest tasks — test carefully that existing ATDD/refactor behavior is unchanged.
- **Criteria state fields** (`criteria_targeted`, `criteria_covered_count`, `criteria_total`) are defined in PhaseMetric but will be zero-valued initially. They require integration with a coverage tracking system that doesn't exist yet (spec: tdd-cycle-coverage-tracker). The fields are included now for forward compatibility.
- **Handoff content tokens** (`handoff_content_tokens` in the summary) can be computed from prompt character counts if the renderer exposes them. Initial implementation can set this to 0 and add prompt-size tracking as a follow-up.
- **Comparison baseline** (estimated single-invocation tokens) is a reporting concern in the stats command. Initial implementation tracks actual tokens; the comparison can be derived from first-cycle input tokens vs subsequent cycles.
- **No feature flag needed.** Phase metrics are pure observability — they capture data about phases that already run. Gating is implicit: no TDD/ATDD active → no phases → no metrics. Works with both current single-invocation TDD and future `fresh_context_per_cycle: true` mode. When `fresh_context_per_cycle` lands, the phase capture hooks in `process_methodology.go` may need minor adjustments if the loop structure changes, but the data model and logging path remain the same.
