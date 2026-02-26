---
id: validation-duration-spc
source_ideas: []
created: 2026-02-18
epic: observability-and-diagnostics
---

# Track Validation Duration in SPC Metrics

## Specification

Gromit should track validation command execution time as a separate metric in the SPC (Statistical Process Control) system. Currently, only total iteration duration is recorded. When a bead takes longer than expected, there is no way to determine whether the slowdown is from test execution, Claude invocation, or prompt rendering without manual investigation.

Adding `validation_duration_ms` to the iteration metrics pipeline enables the existing anomaly detection to flag slow test suites automatically, answering "are tests slowing us down?" at a glance from `process_trend.json`.

### What to Capture

Wall-clock time for all validation command execution within one iteration, including:
- Fast validation gate commands (`go test`, `go vet`, `go build`)
- Full validation gate commands (when triggered)
- Recovery attempts (auto-fix retries and Claude fix attempts)
- Methodology validation passes (ATDD verify-tests-fail, verify-tests-pass)

Multiple validation passes within one iteration sum their durations into a single `validation_duration_ms` value.

Iterations that never reach validation (e.g., build failure, skipped bead) omit the field entirely (`omitempty`).

### Changes by Layer

**Validation Runner** (`internal/runner/validation/runner.go`)

`runCommands()` captures `time.Now()` before executing commands and `time.Since()` after. The elapsed duration is returned to callers alongside the existing error. This is the single timing instrumentation point for direct command execution.

For recovery paths in `RunWithRecovery()` / `RunWithRecoveryForCommands()`, the caller in `process.go` accumulates total elapsed time across all validation attempts.

**Iteration Result** (`internal/runner/runtypes/types.go`)

Add one field:
```
ValidationDurationMs int64
```

Callers in `process.go` (`runValidation`, `runFullValidationGate`, methodology verification) accumulate validation duration into this field.

**Iteration Log** (`internal/logger/logger.go`)

Add one field:
```
ValidationDurationMs int64 `json:"validation_duration_ms,omitempty"`
```

**Log Mapping** (`internal/runner/logging.go`)

Map `result.ValidationDurationMs` to `log.ValidationDurationMs` in `writeIterationLog()`.

**Rolling Metrics** (`internal/logger/process_trend.go`)

Add to `IterationMetric`:
- `RollingAvgValidationMs float64 json:"rolling_avg_validation_ms"`
- `RollingP95ValidationMs float64 json:"rolling_p95_validation_ms"`

Computed in `summarizeWindow()` from the `ValidationDurationMs` values of iterations in the window. Iterations with zero/omitted validation duration are excluded from the average and percentile calculations (they did not run validation).

**SPC Window** (`internal/logger/process_trend.go`)

Add to `ProcessTrendWindow`:
- `AvgValidationMs float64 json:"avg_validation_ms"`
- `P95ValidationMs float64 json:"p95_validation_ms"`

**Control Limits** (`internal/logger/process_trend.go`)

Add `rolling_avg_validation_ms` to the list of metrics that `buildProcessTrend()` computes control limits for. The existing `computeControlLimit()` and `detectAnomaly()` functions handle it without modification.

### What Does Not Change

- No new configuration fields
- No new files
- No changes to the validation command execution logic itself
- Existing JSONL entries parse correctly (omitempty means the field is simply absent in old data)
- `process_trend.json` gains new entries in `control_limits[]` but the schema is unchanged

### Acceptance Criteria

- `validation_duration_ms` appears in iteration metrics JSONL for iterations that run validation
- `validation_duration_ms` is omitted for iterations that skip validation
- `rolling_avg_validation_ms` appears in `process_trend.json` control limits
- Anomaly detection flags validation duration spikes (e.g., a 4-sigma increase triggers a "high" severity anomaly)
- Duration includes recovery time (auto-fix + Claude fix retries)
- Multiple validation passes within one iteration sum into a single value
