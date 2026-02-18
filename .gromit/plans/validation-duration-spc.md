---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T16:20:50Z"
id: validation-duration-spc
source_spec: validation-duration-spc
---

# Track Validation Duration in SPC Metrics — Implementation Plan

**Goal:** Track validation command execution time as a separate metric in the SPC system so anomaly detection can flag slow test suites automatically.

**Architecture:** Add wall-clock timing to `runCommands()` in the validation runner, accumulate duration in `IterationResult`, propagate through the logging pipeline into rolling metrics and control limits. No new files; no new config.

**Tech Stack:** Go, existing SPC pipeline (`process_trend.go`)

**Spec:** `.gromit/specs/validation-duration-spc.md`

---

## Architecture

**Overview:**
Instrument `runCommands()` with `time.Now()`/`time.Since()`, expose accumulated elapsed time via a runner method, and let callers in `process.go` copy it into `bc.Result.ValidationDurationMs`. The existing logging pipeline carries it through `IterationLog` → `IterationMetric` → `ProcessTrendWindow` → control limits.

**Integration Points:**
- Validation runner already accumulates `failures []string` with `Failures()` accessor — same pattern for elapsed duration
- `writeIterationLog()` already maps ~30 fields from result to log — one more field
- `summarizeWindow()` already computes rolling avg/p95 for duration — same pattern for validation duration
- `buildProcessTrend()` already builds control limits from a `map[string][]float64` series — one more entry

**Data Flow:**
```
runCommands() captures time.Since()
  → Runner.elapsed accumulates across calls
  → process.go copies runner.ElapsedMs() into bc.Result.ValidationDurationMs
  → writeIterationLog() maps to IterationLog.ValidationDurationMs
  → buildIterationMetrics() copies to IterationMetric and computes rolling fields
  → buildProcessTrend() populates LatestWindow and control limits
  → detectAnomaly() flags spikes
```

**Files to Modify:**
- `internal/runner/validation/runner.go` — Add timing, elapsed accumulator, accessor
- `internal/runner/runtypes/types.go` — Add `ValidationDurationMs` field
- `internal/logger/logger.go` — Add `ValidationDurationMs` field
- `internal/runner/logging.go` — Map result → log
- `internal/logger/process_trend.go` — Rolling fields, window fields, control limit entry, percentile helper
- `internal/runner/process.go` — Accumulate elapsed from runner into result

**Tradeoffs:**
- Timing in `runCommands()` (single point) vs. wrapping each caller: chose single point to avoid duplication
- Runner method (`ElapsedMs()`) vs. return value: chose method to avoid breaking existing call chain signatures

## Test Strategy

**Unit Tests:**
- Validation runner: elapsed > 0 after commands, accumulates across recovery, resets correctly, 0 when disabled
- Process trend: `summarizeWindow()` avg/p95 correct, zero entries excluded, control limits include validation metric, anomaly detection fires on spike
- Logging: `writeIterationLog()` maps field, omitted when zero

**Integration Test:**
- Write JSONL with `validation_duration_ms` → `BuildContinuousMetrics` → verify `process_trend.json` control limits

**Mocking:** Existing `cmdRunner` function mock and in-memory JSONL patterns. No new mocks.

## Implementation Tasks

### Task 1: Add timing instrumentation to validation runner

**Files:**
- Modify: `internal/runner/validation/runner.go`
- Test: `internal/runner/validation/validation_test.go`

**What to Do:**
Add an `elapsed time.Duration` field to `Runner`. In `runCommands()`, capture `start := time.Now()` before the command loop and `r.elapsed += time.Since(start)` after. Add `ElapsedMs() int64` method returning `r.elapsed.Milliseconds()`. Add `ResetElapsed()` method. The field accumulates across multiple calls (recovery path sums all validation passes).

**Acceptance Criteria:**
- `ElapsedMs()` returns > 0 after `RunDirect` with a passing command
- `ElapsedMs()` accumulates across `RunWithRecovery` with auto-fix retry (sum of both passes)
- `ResetElapsed()` zeroes the accumulator

**Dependencies:** None

**Notes:** Follow the existing `failures []string` / `Failures()` / `ResetFailures()` pattern exactly.

### Task 2: Add ValidationDurationMs to IterationResult and IterationLog

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/logging.go`

**What to Do:**
Add `ValidationDurationMs int64` to `IterationResult` (after the `ValidationMode` field). Add `ValidationDurationMs int64 \`json:"validation_duration_ms,omitempty"\`` to `IterationLog` (after `ValidationMode`). In `writeIterationLog()`, add `ValidationDurationMs: result.ValidationDurationMs` to the `IterationLog` literal.

**Acceptance Criteria:**
- `IterationResult` has `ValidationDurationMs int64` field
- `IterationLog` has `ValidationDurationMs` with correct JSON tag and omitempty
- `writeIterationLog()` copies the field from result to log

**Dependencies:** None

### Task 3: Wire validation duration from runner into IterationResult

**Files:**
- Modify: `internal/runner/process.go`

**What to Do:**
In `runValidation()`, `runValidationWithRecoveryForStage()`, and `runFullValidationGate()`: after the validation runner call returns, copy `r.validationRunner.ElapsedMs()` into `bc.Result.ValidationDurationMs` (accumulating with `+=` since multiple validation passes can occur within one iteration). Call `r.validationRunner.ResetElapsed()` at the start of each validation call to scope accumulation to that call sequence.

For methodology verification passes (ATDD `VerifyAcceptanceTestsPass` in `process_methodology.go`): these delegate to the same validation runner, so their elapsed time is already captured. The `runValidationWithRecoveryForStage` calls in `executeBuildAndMethodologyLoop` and `runRefactorAndPostChecks` will accumulate correctly since they all go through the runner.

**Acceptance Criteria:**
- `bc.Result.ValidationDurationMs` is populated after `runValidation` completes
- Multiple validation passes within one iteration (fast gate + post-refactor) sum into a single value
- Iterations that skip validation have `ValidationDurationMs == 0`

**Dependencies:** Task 1, Task 2

### Task 4: Add rolling validation metrics and control limits to SPC pipeline

**Files:**
- Modify: `internal/logger/process_trend.go`
- Test: `internal/logger/process_trend_test.go`

**What to Do:**
Add to `IterationMetric`:
- `ValidationDurationMs int64 \`json:"validation_duration_ms,omitempty"\``
- `RollingAvgValidationMs float64 \`json:"rolling_avg_validation_ms"\``
- `RollingP95ValidationMs float64 \`json:"rolling_p95_validation_ms"\``

Add to `ProcessTrendWindow`:
- `AvgValidationMs float64 \`json:"avg_validation_ms"\``
- `P95ValidationMs float64 \`json:"p95_validation_ms"\``

In `buildIterationMetrics()`: copy `entry.ValidationDurationMs` to metric, set rolling fields from window summary.

In `summarizeWindow()`: compute avg and p95 for `ValidationDurationMs`, excluding entries where the value is 0 (no validation ran). Add a `percentileFloat64` helper or collect durations as `int64` and reuse `percentileInt64`.

In `buildProcessTrend()`: populate `LatestWindow.AvgValidationMs` and `P95ValidationMs` from the latest metric. Add `"rolling_avg_validation_ms"` to the `series` map in `buildProcessTrend()` so `computeControlLimit` and `detectAnomaly` run on it.

**Acceptance Criteria:**
- `summarizeWindow()` returns correct `AvgValidationMs` and `P95ValidationMs` for a window with mixed zero/nonzero entries
- `buildProcessTrend()` includes `rolling_avg_validation_ms` in `ControlLimits`
- Anomaly detection flags a 4-sigma validation duration spike as "high" severity
- `BuildContinuousMetrics` end-to-end: JSONL with `validation_duration_ms` → `process_trend.json` has control limit entry

**Dependencies:** Task 2 (IterationLog field must exist for JSONL reading)

**Notes:** The existing `percentileInt64` works on `[]int64`. Validation durations are `int64` (ms), so collect them the same way as `durations` for `P95DurationMs`.

---

## Notes

- No new configuration fields — this is purely additive metrics instrumentation
- Backward compatibility: `omitempty` on all new JSON fields means old JSONL entries parse fine (field is absent, zero value)
- The `process_trend.json` control limits array gains new entries but the schema is unchanged
- Recovery time (auto-fix + Claude fix retries) is included because timing wraps the entire `runCommands()` call, and callers accumulate across retry loops
