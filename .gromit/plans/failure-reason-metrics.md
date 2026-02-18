---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T18:00:21Z"
id: failure-reason-metrics
source_spec: failure-reason-metrics
---

# Failure Phase and Category Metrics Implementation Plan

**Goal:** Record where (phase) and why (category) each iteration failed in the metrics pipeline so retro can target interventions precisely.

**Architecture:** Add `FailurePhase` and `FailureCategory` fields through the full iteration pipeline (result → log → metric → trend), with per-phase rolling failure rates computed via existing window-based patterns and fed into SPC control limits. The runner sets the phase at each existing failure point; the category comes from the analyzer result already computed.

**Tech Stack:** Go, JSONL logging, SPC control limits

**Spec:** `.gromit/specs/failure-reason-metrics.md`

---

## Architecture

**Data Flow:**
```
Runner failure → result.FailurePhase = "preflight"|"build"|"validation"|"timeout"
Analyzer result → result.FailureCategory = string(analysis.Category)
     ↓
writeIterationLog() → log.FailurePhase, log.FailureCategory  →  JSONL
     ↓
buildIterationMetrics() → metric.FailurePhase, metric.FailureCategory
                        → metric.RollingPreflightFailureRate, etc.
     ↓
summarizeWindow() → window.PreflightFailureRate, etc.
     ↓
buildProcessTrend() → control limits for each phase rate
     ↓
Retro template → "Failure Breakdown" section from ProcessTrendWindow
```

**Integration Points:**
- `writeIterationLog()` maps result → log (existing pattern, 2 new field copies)
- `buildIterationMetrics()` copies fields and computes rolling rates (existing window pattern)
- `summarizeWindow()` counts phase-specific failures (same as existing success/failure counting)
- `buildProcessTrend()` adds 4 new metrics to control limit series
- Runner failure paths set phase at each existing failure handling point
- Retro template renders new fields from ProcessTrendWindow

**Tradeoffs:**
- Phase assignment distributed at each failure point (not centralized) — matches existing runner structure where failure paths are already distinct
- Category propagated from analyzer (not re-classified) — reuses existing classification, per spec

## Test Strategy

**Unit Tests:**
- `process_trend_test.go`: rolling rate calculations for each phase, window counting, control limits, anomaly detection
- `logging_test.go`: field mapping from result to log, omitempty behavior

**Integration Tests:**
- Runner failure paths set correct phase values
- End-to-end: failure → log → metric → trend with phase fields populated

**Key Test Cases:**
- Mixed-phase window produces correct per-phase rates
- Per-phase rates sum to total failure rate
- All-success window has zero phase rates
- 4-sigma phase rate spike triggers "high" anomaly
- Old JSONL entries without new fields parse correctly (backward compat)
- Each failure phase value has a test setting it

**Mocking:** Uses existing mock patterns — no new mocks needed. Process trend tests are pure computation.

## Implementation Tasks

### Task 1: Add FailurePhase and FailureCategory to schema structs

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`

**What to Do:**
Add `FailurePhase string` and `FailureCategory string` to `IterationResult` in `runtypes/types.go`. Add the same two fields with `json:"failure_phase,omitempty"` and `json:"failure_category,omitempty"` tags to `IterationLog` in `logger.go`.

**Acceptance Criteria:**
- Both fields exist on `IterationResult` and `IterationLog`
- JSONL tags use `omitempty` for backward compatibility
- Code compiles

**Dependencies:** None

### Task 2: Wire FailurePhase and FailureCategory through logging pipeline

**Files:**
- Modify: `internal/runner/logging.go`
- Modify: `internal/runner/logging_test.go`

**What to Do:**
In `writeIterationLog()`, map `result.FailurePhase` → `log.FailurePhase` and `result.FailureCategory` → `log.FailureCategory`. Follow the existing pattern used for `FailureClass` and `AndonLevel`. Add tests verifying the mapping for each phase value, category propagation, and omission on success.

**Acceptance Criteria:**
- `writeIterationLog()` copies both fields from result to log
- Test covers preflight/build/validation/timeout phase mapping
- Test verifies empty strings on success

**Dependencies:** Task 1

### Task 3: Set FailurePhase at runner failure paths

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/runner/escalation/handler.go`

**What to Do:**
At each failure handling point in the runner:
- Preflight: In `runCompilationCheck()` failure path and preflight check failure, set `result.FailurePhase = "preflight"`
- Build: In `makeInvokeFn()` when Claude invocation fails (non-timeout), set `result.FailurePhase = "build"`
- Validation: In `makeValidationExecuteFn()` when test/lint fails, set `result.FailurePhase = "validation"`
- Timeout: In escalation handler timeout paths (`HandleStallTimeout`, `HandleInvocationTimeout`, `HandleBeadTimeout`), set `result.FailurePhase = "timeout"`

Set `result.FailureCategory = string(analysis.Category)` when the analyzer produces a non-nil result (in `AnalyzeAndHandleFailure()` or wherever the runner holds the analysis result after failure).

**Acceptance Criteria:**
- Each failure path sets the correct `FailurePhase`
- `FailureCategory` set from analyzer result when available
- Existing tests continue to pass (fields are additive)

**Dependencies:** Task 1

**Notes:** The runner already has distinct code paths for each failure type. The changes are minimal — just setting a string field at each existing failure point. Be careful not to set phase on success paths.

### Task 4: Add per-phase rolling rates to IterationMetric and ProcessTrendWindow

**Files:**
- Modify: `internal/logger/process_trend.go`
- Modify: `internal/logger/process_trend_test.go`

**What to Do:**
Add to `IterationMetric`:
- `FailurePhase string` with `json:"failure_phase,omitempty"`
- `FailureCategory string` with `json:"failure_category,omitempty"`
- `RollingPreflightFailureRate float64` with `json:"rolling_preflight_failure_rate"`
- `RollingBuildFailureRate float64` with `json:"rolling_build_failure_rate"`
- `RollingValidationFailureRate float64` with `json:"rolling_validation_failure_rate"`
- `RollingTimeoutFailureRate float64` with `json:"rolling_timeout_failure_rate"`

Add to `ProcessTrendWindow`:
- `PreflightFailureRate float64` with `json:"preflight_failure_rate"`
- `BuildFailureRate float64` with `json:"build_failure_rate"`
- `ValidationFailureRate float64` with `json:"validation_failure_rate"`
- `TimeoutFailureRate float64` with `json:"timeout_failure_rate"`

In `buildIterationMetrics()`, copy `FailurePhase` and `FailureCategory` from the source `IterationLog`. In `summarizeWindow()`, count iterations matching each phase and divide by window size — same pattern as existing `RollingFailureRate`.

Add tests:
- Mixed-phase window → correct per-phase rates
- All-success → all phase rates are 0.0
- Single-phase failures → that phase rate equals total failure rate
- Per-phase rates sum to total failure rate

**Acceptance Criteria:**
- Per-phase rolling rates computed correctly in `summarizeWindow()`
- `FailurePhase` and `FailureCategory` copied in `buildIterationMetrics()`
- Tests verify arithmetic consistency (phase rates sum to total)

**Dependencies:** Task 1

### Task 5: Add per-phase control limits and anomaly detection

**Files:**
- Modify: `internal/logger/process_trend.go`
- Modify: `internal/logger/process_trend_test.go`

**What to Do:**
In `buildProcessTrend()`, add `rolling_preflight_failure_rate`, `rolling_build_failure_rate`, `rolling_validation_failure_rate`, and `rolling_timeout_failure_rate` to the list of metrics that get control limits computed. The existing `computeControlLimit()` and `detectAnomaly()` functions handle them without modification.

Add tests:
- 4 new metrics appear in control limits output
- 4-sigma spike in validation failure rate triggers "high" severity anomaly
- Phase rate control limits are clamped to [0, 1]

**Acceptance Criteria:**
- Per-phase rates appear in `ProcessTrend.ControlLimits`
- Anomaly detection flags phase-specific failure rate spikes
- Existing control limit tests still pass

**Dependencies:** Task 4

### Task 6: Add failure breakdown to retro prompt template

**Files:**
- Modify: retro prompt template (find via Grep for retro template in `.gromit/templates/`)

**What to Do:**
Add a "Failure Breakdown" section to the retro prompt template showing per-phase rolling rates from the latest `ProcessTrendWindow`. Display each phase's failure rate. No changes to retro's data loading — the new fields appear automatically in the `ProcessTrendWindow` that retro already receives via `ProcessTrend`.

**Acceptance Criteria:**
- Retro prompt includes "Failure Breakdown" section
- Per-phase failure rates rendered from `ProcessTrendWindow`
- Template compiles and renders without error

**Dependencies:** Task 4

**Notes:** The retro template receives the full `ProcessTrend` struct via `TemplateContext.ProcessTrend`. The new `ProcessTrendWindow` fields are available at `.ProcessTrend.LatestWindow.PreflightFailureRate` etc.

---

## Notes

- **Backward compatibility**: All new JSONL fields use `omitempty`, so old entries parse correctly. Historical iterations without phase fields contribute to `rolling_failure_rate` but not to per-phase rates — they age out of the window naturally.
- **No new files**: The spec explicitly requires no new files.
- **No control flow changes**: The runner's control flow is unchanged — we're just setting string fields at existing failure points.
- **Phase assignment convention**: Only set `FailurePhase` on failure paths. Success paths leave it empty (zero value). The `omitempty` tag keeps it out of JSONL for successes.
