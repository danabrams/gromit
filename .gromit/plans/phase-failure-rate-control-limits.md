---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T03:15:29Z"
id: phase-failure-rate-control-limits
source_spec: phase-failure-rate-control-limits
---

# Phase-Specific Failure Rate Control Limits Implementation Plan

**Goal:** Add per-phase failure rate series to the SPC control-limit pipeline so anomaly detection covers preflight, build, validation, and timeout failure spikes.

**Architecture:** Add 4 `extractMetric` entries to the existing `series` map in `buildProcessTrend()`. All downstream SPC infrastructure (`isRateMetric`, `computeControlLimit`, `detectAnomaly`) already handles the new entries generically.

**Tech Stack:** Go

**Spec:** `.gromit/specs/phase-failure-rate-control-limits.md`

---

## Architecture

The `series` map in `buildProcessTrend()` (process_trend.go:325-331) currently has 5 entries. Adding 4 phase-rate entries brings it to 9. No other production changes needed because:

- `isRateMetric()` matches any metric containing "rate" — automatic [0,1] clamping
- `computeControlLimit()` and `detectAnomaly()` are generic over any numeric series
- `LatestWindow` already includes the 4 phase-rate fields (populated at lines 319-322)

## Test Strategy

Unit tests against `buildProcessTrend()` in `process_trend_test.go`:

1. **Control limit count and names**: Verify 9 control limits are emitted, including all 4 phase-rate metric names
2. **Spike anomaly detection**: Create a metrics window where one phase rate spikes from 0.0 to 1.0, verify a `high` severity anomaly fires
3. **Rate clamping**: Assert UCL/LCL for phase-rate metrics stay within [0,1]

## Implementation Tasks

### Task 1: Add phase-rate series to buildProcessTrend and tests

**Files:**
- Modify: `internal/logger/process_trend.go`
- Modify: `internal/logger/process_trend_test.go`

**What to Do:**

In `buildProcessTrend()`, add 4 entries to the `series` map after the existing 5:

```go
"rolling_preflight_failure_rate":  extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingPreflightFailureRate }),
"rolling_build_failure_rate":      extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingBuildFailureRate }),
"rolling_validation_failure_rate": extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingValidationFailureRate }),
"rolling_timeout_failure_rate":    extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingTimeoutFailureRate }),
```

In `process_trend_test.go`, add:

1. `TestBuildProcessTrend_ControlLimitsIncludeAllNineMetrics` — build metrics from a mixed-phase window, assert `len(trend.ControlLimits) == 9`, assert all 4 phase-rate metric names are present.

2. `TestBuildProcessTrend_PhaseRateSpikeTriggersAnomaly` — create ~10 all-success entries followed by 1 entry with `FailurePhase: "build"` at 100% rate. Assert an anomaly exists for `rolling_build_failure_rate` with severity `high`.

3. `TestBuildProcessTrend_PhaseRateControlLimitsClamped` — verify UCL <= 1.0 and LCL >= 0.0 for all phase-rate control limits.

**Acceptance Criteria:**
- `buildProcessTrend()` emits exactly 9 control limits
- All 4 phase-rate metric names appear in control limits
- A 4-sigma phase-rate spike triggers a `high` severity anomaly

**Dependencies:** None

---

## Notes

- The spec explicitly states no changes to `summarizeWindow()`, `IterationMetric`, `ProcessTrendWindow`, `isRateMetric()`, `computeControlLimit()`, or `detectAnomaly()`. This plan honors that constraint.
- Existing tests (`TestBuildProcessTrend_PhaseRatesSumToFailureRate`, etc.) continue to pass unchanged.
