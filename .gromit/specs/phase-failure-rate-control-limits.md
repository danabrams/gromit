---
id: phase-failure-rate-control-limits
source_ideas: []
created: 2026-02-19
epic: observability-and-diagnostics
---

# Add Phase-Specific Failure Rates to SPC Control Limits

## Specification

The `failure-reason-metrics` spec introduced phase-specific rolling failure rates (`preflight`, `build`, `validation`, `timeout`) in `IterationMetric` and `ProcessTrendWindow`. Those rates are computed in `summarizeWindow()`, but `buildProcessTrend()` does not include them in the SPC `series` map. As a result, control limits and anomaly detection are not generated for phase-specific failure spikes.

Add these four series entries in `internal/logger/process_trend.go` inside `buildProcessTrend()`:

```go
"rolling_preflight_failure_rate":  extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingPreflightFailureRate }),
"rolling_build_failure_rate":      extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingBuildFailureRate }),
"rolling_validation_failure_rate": extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingValidationFailureRate }),
"rolling_timeout_failure_rate":    extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingTimeoutFailureRate }),
```

No additional production logic changes are required:
- `isRateMetric()` already applies rate clamping for metrics containing `rate`
- `computeControlLimit()` and `detectAnomaly()` already work for numeric series

## Files Changed

- `internal/logger/process_trend.go` - add 4 phase-rate entries to the SPC series map in `buildProcessTrend()`
- `internal/logger/process_trend_test.go` - add coverage that control limits are emitted for all 9 metrics and phase-rate spikes trigger anomalies

## What Does Not Change

- `summarizeWindow()`, `IterationMetric`, and `ProcessTrendWindow` field definitions
- `isRateMetric()`, `computeControlLimit()`, and `detectAnomaly()` behavior
- `process_trend.json` structure (only additional metric keys appear)
- Configuration surface and file layout

## Acceptance Criteria

- `process_trend.json` `control_limits` contains:
  - `rolling_preflight_failure_rate`
  - `rolling_build_failure_rate`
  - `rolling_validation_failure_rate`
  - `rolling_timeout_failure_rate`
- Total control-limit metric count increases from 5 to 9
- A 4-sigma spike in any phase-specific failure rate triggers a `high` severity anomaly

## Related Specs

- `failure-reason-metrics`

## Design Document

- `docs/plans/2026-02-19-phase-failure-rate-control-limits-design.md`
