---
id: failure-reason-metrics
source_ideas: []
created: 2026-02-18
epic: observability-and-diagnostics
---

# Add Failure Phase and Category to Iteration Metrics

## Specification

Gromit should record *where* and *why* each iteration failed in the iteration metrics pipeline. Currently, iteration metrics track only whether an iteration succeeded (`success`, `first_pass_success`, `escalated`). When failures cluster, retro cannot distinguish preflight breakdowns from build failures from validation regressions without manually inspecting detailed logs.

Adding `failure_phase` and `failure_category` to both `IterationLog` and `IterationMetric` enables retro to calculate phase-specific failure rates and target interventions precisely (e.g., "validation failures doubled in the last 30 iterations").

### Failure Phase Values

Four phases, set by the runner at the point of failure:

| Phase | Meaning |
|-------|---------|
| `preflight` | Compilation or environment check failed before Claude invocation |
| `build` | Claude code generation failed (non-zero exit, errors in output) |
| `validation` | Test or lint execution failed after successful build |
| `timeout` | Any phase hit its time limit (existing `TimeoutPhase` retains sub-phase detail) |

Empty string on success.

### Failure Category Values

The analyzer's existing `Category` type, propagated from the `Analysis` result the runner already computes:

`syntax`, `logic`, `environment`, `unclear_spec`, `missing_context`, `test_flake`, `task_too_complex`, `hard_stop_action`

Empty string on success or when no analysis ran.

### Changes by Layer

**Iteration Result** (`internal/runner/runtypes/types.go`)

Add two fields:
```
FailurePhase    string // "preflight", "build", "validation", "timeout", or ""
FailureCategory string // analyzer category, or ""
```

The runner sets `FailurePhase` at each failure path. `FailureCategory` comes from the analyzer result that the runner already holds after failure analysis.

**Iteration Log** (`internal/logger/logger.go`)

Add two fields:
```
FailurePhase    string `json:"failure_phase,omitempty"`
FailureCategory string `json:"failure_category,omitempty"`
```

**Log Mapping** (`internal/runner/logging.go`)

Map `result.FailurePhase` to `log.FailurePhase` and `result.FailureCategory` to `log.FailureCategory` in `writeIterationLog()`.

**Iteration Metric** (`internal/logger/process_trend.go`)

Add to `IterationMetric`:
```
FailurePhase                 string  `json:"failure_phase,omitempty"`
FailureCategory              string  `json:"failure_category,omitempty"`
RollingPreflightFailureRate  float64 `json:"rolling_preflight_failure_rate"`
RollingBuildFailureRate      float64 `json:"rolling_build_failure_rate"`
RollingValidationFailureRate float64 `json:"rolling_validation_failure_rate"`
RollingTimeoutFailureRate    float64 `json:"rolling_timeout_failure_rate"`
```

**Metrics Computation** (`internal/logger/process_trend.go`)

In `buildIterationMetrics()`, copy `FailurePhase` and `FailureCategory` from the source `IterationLog`. In `summarizeWindow()`, count iterations matching each phase and divide by window size. Same pattern as the existing `RollingFailureRate` calculation.

Add to `ProcessTrendWindow`:
```
PreflightFailureRate  float64 `json:"preflight_failure_rate"`
BuildFailureRate      float64 `json:"build_failure_rate"`
ValidationFailureRate float64 `json:"validation_failure_rate"`
TimeoutFailureRate    float64 `json:"timeout_failure_rate"`
```

**SPC Control Limits** (`internal/logger/process_trend.go`)

Add `rolling_preflight_failure_rate`, `rolling_build_failure_rate`, `rolling_validation_failure_rate`, and `rolling_timeout_failure_rate` to the list of metrics that `buildProcessTrend()` computes control limits for. The existing `computeControlLimit()` and `detectAnomaly()` functions handle them without modification.

**Runner Failure Paths** (`internal/runner/`)

At each failure handling point in the runner:
- Preflight failure (`CompilationErrors` or preflight check failure): set `result.FailurePhase = "preflight"`
- Build failure (Claude invocation fails): set `result.FailurePhase = "build"`
- Validation failure (test/lint fails): set `result.FailurePhase = "validation"`
- Timeout (any phase): set `result.FailurePhase = "timeout"`

The runner already holds the `analyzer.Analysis` result after failure analysis. Set `result.FailureCategory = string(analysis.Category)` when the analysis is non-nil.

**Retro Prompt** (`internal/retro/`)

The retro prompt template gains a "Failure Breakdown" section showing per-phase rolling rates from the latest `ProcessTrendWindow`. No changes to retro's data loading; the new fields appear automatically in the `IterationMetric` records retro already reads.

### What Does Not Change

- No new configuration fields
- No new files
- No changes to the analyzer's classification logic
- No changes to the runner's control flow (just setting string fields at existing failure points)
- Existing JSONL entries parse correctly (`omitempty` means the fields are absent in old data)
- Historical iterations without the new fields contribute to `rolling_failure_rate` but not to per-phase rates; they age out of the window naturally

### Acceptance Criteria

- `failure_phase` appears in iteration log JSONL for failed iterations
- `failure_phase` is omitted for successful iterations
- `failure_category` appears in iteration log JSONL when analyzer produces a result
- `rolling_preflight_failure_rate`, `rolling_build_failure_rate`, `rolling_validation_failure_rate`, and `rolling_timeout_failure_rate` appear in `iteration_metrics.jsonl`
- Per-phase rolling rates appear in `process_trend.json` control limits
- Anomaly detection flags phase-specific failure rate spikes (e.g., a 4-sigma increase in validation failure rate triggers a "high" severity anomaly)
- Retro prompt includes failure breakdown by phase
- Sum of per-phase failure rates equals total failure rate for iterations where all failures have a phase assigned
