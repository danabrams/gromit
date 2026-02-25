---
id: process-trend-split
source_spec: process-trend-split
created: 2026-02-25
decomposed: false
---

# Process Trend Split Implementation Plan

**Goal:** Split `internal/logger/process_trend.go` into focused same-package files to satisfy line-count limits without changing behavior.

**Architecture:** Apply a pure move refactor by relocating existing functions and SPC configuration into three new `internal/logger/` files while preserving the current public API and orchestration in `process_trend.go`.

**Tech Stack:** Go (`internal/logger` package), standard library (`encoding/json`, `os`, `filepath`, `sort`, `math`, etc.), existing `failurephase` and `prompt` internal packages.

**Spec:** `.gromit/specs/process-trend-split.md`

---

## Architecture

**Overview:**
Perform a pure same-package move refactor: split `process_trend.go` into four files organized by responsibility, with function bodies moved verbatim and no behavior or public API changes.

**Key Components:**
1. **`process_trend.go`**: Keep schema/data model types, public entry points, orchestration, and file I/O helpers.
2. **`trend_builder.go`**: Move iteration log reading/sorting, per-iteration metric construction, failure attribution, and rolling window summarization.
3. **`trend_spc.go`**: Move SPC config (`trendControlLimitSeries`, `trendEWMASeries` and their type defs) plus EWMA/control-limit/anomaly/pattern/stratification functions.
4. **`trend_analytics.go`**: Move provider aggregates and prompt token summary functions.

**Integration Points:**
- Same `logger` package across all files, so call sites and tests remain unchanged.
- `BuildContinuousMetrics` in `process_trend.go` still orchestrates by calling moved helpers.
- Shared utilities from `math_helpers.go` stay as-is and are still used by moved code.

**Data Flow:**
`BuildContinuousMetrics` -> `readAllIterationLogsSorted` -> `buildIterationMetrics` -> `buildProcessTrend` -> SPC/analytics helpers -> write JSON outputs via `writeIterationMetrics`/`writeProcessTrend`.

**Files to Modify:**
- `internal/logger/process_trend.go` - reduce to model/public API/orchestration/I/O.

**Files to Create:**
- `internal/logger/trend_builder.go` - computation pipeline.
- `internal/logger/trend_spc.go` - SPC machinery/config.
- `internal/logger/trend_analytics.go` - aggregate analytics.

**Tradeoffs:**
- Chose same-package split over subpackages to avoid API/call-site churn.
- Chose verbatim function moves over cleanup edits to minimize regression risk.
- Chose to move SPC series vars with SPC functions to keep ownership cohesive and line counts compliant.

## Test Strategy

**Test Levels:**
1. **Unit/Package tests (existing):** Re-run `process_trend_test.go` unchanged to validate moved helpers still behave identically.
2. **Build verification:** `go build ./...` to ensure package wiring/imports remain correct after file split.
3. **Targeted package verification:** `go test ./internal/logger/...` to validate no regressions in trend generation, SPC, analytics, and I/O behavior.

**Key Test Cases:**
- `BuildContinuousMetrics` generates files and preserves fields (including token/file metrics).
- `buildIterationMetrics` preserves failure phase/category/attribution and EWMA population.
- `buildProcessTrend` still computes latest window, limits, anomalies, violations, and summaries.
- SPC helpers (`detectPatternViolations`, control limits, stratified limits) retain current behavior.
- `ReadProcessTrend` nil normalization and parsing behavior is unchanged.
- `computeProviderMetrics` and prompt token summary outputs remain stable.

**Mocking Strategy:**
- No new mocking needed; rely on existing real-data/unit fixtures and temp-dir based tests.
- Keep tests in same package (`package logger`) so moved unexported functions remain directly testable.

**Coverage Goals:**
- Preserve full behavior equivalence for all moved functions.
- Ensure acceptance criteria gates:
  - Four production files exist.
  - Each is <= 550 lines.
  - Public surface remains in `process_trend.go`.
  - Unchanged behavior validated by existing tests.

**Test Organization:**
- No test file split in this plan.
- Keep `internal/logger/process_trend_test.go` untouched per spec decision.

## Implementation Tasks

### Task 1: Move Computation Pipeline into `trend_builder.go`

**Files:**
- Modify: `internal/logger/process_trend.go`
- Create: `internal/logger/trend_builder.go`
- Test: `internal/logger/process_trend_test.go` (no edits expected)

**What to Do:**
Move the computation pipeline functions verbatim from `process_trend.go` to `trend_builder.go`: `readAllIterationLogsSorted`, `buildIterationMetrics`, `buildBeadEntryIndices`, failure attribution helpers (`classifyFailureAttribution`, `deriveDefectOriginPhase`, `isTransientFailureSignal`, `indexInSeries`, `isSingleFailureThenSameTierSuccess`, `hasRepeatedCrossTierFailures`, `resolvedTier`), and window summarization (`summarizeWindow`, `beadCostAccum`, `updateBeadCostAccum`, `averageCompletedBeadCost`). Keep signatures and call sites unchanged.

**Acceptance Criteria:**
- All pipeline and failure-attribution functions listed in the spec compile from `trend_builder.go`.
- `BuildContinuousMetrics` and `buildProcessTrend` continue to call these helpers without signature or behavior changes.
- No logic differences from pre-split implementation (verbatim move only).

**Dependencies:**
- None.

**Notes:**
- Preserve existing import usage; only relocate functions.
- Keep `trend_updater.go` and `math_helpers.go` untouched.

### Task 2: Move SPC Configuration and Functions into `trend_spc.go`

**Files:**
- Modify: `internal/logger/process_trend.go`
- Create: `internal/logger/trend_spc.go`
- Test: `internal/logger/process_trend_test.go` (no edits expected)

**What to Do:**
Move SPC-related type defs/vars and functions verbatim into `trend_spc.go`: `metricSeriesDefinition`, `ewmaSeriesDefinition`, `trendControlLimitSeries`, `trendEWMASeries`, `computeEWMAState`, `controlLimitFromEWMAState`, `computeControlLimit`, `detectAnomaly`, `detectPatternViolations`, `newRule2Violation`, `trailingRunLength`, `buildStratifiedControlLimits`, `partitionMetricsByStratum`, `providerStratumKey`, `modelStratumKey`, `resolveModelStratumName`, and helpers `extractMetric`, `boolToFloat64`, `isRateMetric`, `clamp`.

**Acceptance Criteria:**
- SPC configuration and all listed SPC/anomaly/stratification helpers compile from `trend_spc.go`.
- `buildProcessTrend` still produces control limits/anomalies/pattern violations using same logic.
- No non-move behavior edits are introduced.

**Dependencies:**
- Task 1 (recommended first to reduce merge churn in `process_trend.go`, but not strictly required).

**Notes:**
- Keep constant ownership consistent with existing package-level constants in `process_trend.go` unless a pure move requires relocation.

### Task 3: Move Aggregate Analytics into `trend_analytics.go`

**Files:**
- Modify: `internal/logger/process_trend.go`
- Create: `internal/logger/trend_analytics.go`
- Test: `internal/logger/process_trend_test.go` (no edits expected)

**What to Do:**
Move analytics functions verbatim into `trend_analytics.go`: `computeProviderMetrics`, `fraction`, `averageInt64`, `resolveProviderName`, and `summarizePromptTokens`. Maintain sorting and summary semantics exactly.

**Acceptance Criteria:**
- Provider metrics and prompt token summary functions compile and are referenced without call-site changes.
- Output ordering/aggregation behavior remains unchanged.
- No test updates are needed to accommodate function moves.

**Dependencies:**
- Task 1 and Task 2 (for stable final `process_trend.go` composition).

**Notes:**
- Ensure imports for `math`, `sort`, and `strings` are present in the new file as needed.

### Task 4: Recompose `process_trend.go` Public Surface and Validate Limits

**Files:**
- Modify: `internal/logger/process_trend.go`
- Verify: `internal/logger/trend_builder.go`
- Verify: `internal/logger/trend_spc.go`
- Verify: `internal/logger/trend_analytics.go`

**What to Do:**
Finalize `process_trend.go` so it retains only the data model/public API/orchestration/write helpers specified: type declarations/constants, `newPromptTokenSummary`, `BuildContinuousMetrics`, `ReadProcessTrend`, `(*ProcessTrend).normalizeNilFields`, `buildProcessTrend`, and write helpers (`writeIterationMetrics`, `writeProcessTrend`, `writeAtomic`). Confirm all four production files are <= 550 lines.

**Acceptance Criteria:**
- `process_trend.go`, `trend_builder.go`, `trend_spc.go`, and `trend_analytics.go` all exist under `internal/logger/` and are <= 550 lines each.
- `process_trend.go` retains required public surface: `BuildContinuousMetrics`, `ReadProcessTrend`, `ProcessTrend`, `IterationMetric`, `EWMAMetricState`, `ProcessTrendWindow`, `TrendControlLimit`, `TrendAnomaly`, `PatternViolation`, `ProviderMetrics`, `PromptTokenSummary`.
- `trend_updater.go` and `math_helpers.go` are unchanged.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
- This task is structural verification, not behavior modification.

### Task 5: Run Verification Gates for No-Behavior-Change Refactor

**Files:**
- Verify only: `internal/logger/*.go`

**What to Do:**
Run required checks after split: `go build ./...` and `go test ./internal/logger/...`. Confirm tests pass with no test modifications and no file changes outside `internal/logger/` for this refactor.

**Acceptance Criteria:**
- `go build ./...` succeeds.
- `go test ./internal/logger/...` succeeds.
- No logic changes observed; refactor remains a pure move.

**Dependencies:**
- Task 4.

**Notes:**
- If failures occur, fix only structural/import issues introduced by the split; do not alter algorithmic behavior.

---

## Notes

- This plan intentionally excludes splitting `process_trend_test.go`; test refactoring is a separate concern.
- Keep package `logger` unchanged across all new files to preserve unexported function test access.
- Scope is constrained to `internal/logger/`; avoid opportunistic cleanup beyond move refactor goals.
