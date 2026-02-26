---
id: process-trend-split
created: 2026-02-21
epic: observability-and-diagnostics
---

# process_trend.go Split

## Specification

`internal/logger/process_trend.go` is 1,357 lines — 147% over the 550-line production limit. It grew incrementally as metrics were added. The code contains five natural responsibility clusters that are currently conflated:

1. **Data model** — type declarations and constants that describe the metrics schema
2. **Computation pipeline** — reading log files, building per-iteration metrics, failure attribution, and window summarization
3. **SPC machinery** — EWMA, control limits, anomaly detection, Nelson pattern violations, and stratification
4. **Aggregate analytics** — provider-level metrics and prompt token summaries
5. **Orchestration and I/O** — the public entry point, file reading/writing, and top-level trend assembly

The fix is a pure move refactor: split `process_trend.go` into four files within the same `logger` package. No logic changes, no new abstractions, no import changes at call sites.

**`process_trend.go`** (~464 lines) — data model, public API, orchestration, I/O. Keeps all type declarations, the public functions `BuildContinuousMetrics` and `ReadProcessTrend`, `normalizeNilFields`, `buildProcessTrend`, and the write helpers (`writeIterationMetrics`, `writeProcessTrend`, `writeAtomic`, `newPromptTokenSummary`).

**`trend_builder.go`** (~372 lines) — the computation pipeline. Contains `readAllIterationLogsSorted`, `buildIterationMetrics`, `buildBeadEntryIndices`, the full failure attribution cluster (`classifyFailureAttribution`, `deriveDefectOriginPhase`, `isTransientFailureSignal`, `indexInSeries`, `isSingleFailureThenSameTierSuccess`, `hasRepeatedCrossTierFailures`, `resolvedTier`), and window summarization (`summarizeWindow`, `beadCostAccum`, `updateBeadCostAccum`, `averageCompletedBeadCost`).

**`trend_spc.go`** (~320 lines) — SPC machinery. Moves the series configuration variables (`trendControlLimitSeries`, `trendEWMASeries` and their type definitions `metricSeriesDefinition`, `ewmaSeriesDefinition`) alongside the SPC functions: `computeEWMAState`, `controlLimitFromEWMAState`, `computeControlLimit`, `detectAnomaly`, `detectPatternViolations`, `newRule2Violation`, `trailingRunLength`, `buildStratifiedControlLimits`, `partitionMetricsByStratum`, `providerStratumKey`, `modelStratumKey`, `resolveModelStratumName`, and helpers `extractMetric`, `boolToFloat64`, `isRateMetric`, `clamp`.

**`trend_analytics.go`** (~163 lines) — aggregate summaries. Contains `computeProviderMetrics`, `fraction`, `averageInt64`, `resolveProviderName`, and `summarizePromptTokens`.

The existing `trend_updater.go` and `math_helpers.go` are untouched. The existing `process_trend_test.go` is untouched — same package, all tests continue to compile and run without changes.

## Acceptance Criteria

- `process_trend.go`, `trend_builder.go`, `trend_spc.go`, and `trend_analytics.go` all exist under `internal/logger/`.
- All four production files are ≤ 550 lines.
- `process_trend.go` retains the original file's package declaration and public surface: `BuildContinuousMetrics`, `ReadProcessTrend`, `ProcessTrend`, `IterationMetric`, `EWMAMetricState`, `ProcessTrendWindow`, `TrendControlLimit`, `TrendAnomaly`, `PatternViolation`, `ProviderMetrics`, `PromptTokenSummary`.
- `go build ./...` passes with zero changes outside `internal/logger/`.
- `go test ./internal/logger/...` passes with no test modifications.
- No logic is changed — this is a move refactor only.
- `trend_updater.go` and `math_helpers.go` are unchanged.

## Decisions

1. **Same package, new files over sub-package extraction.** A sub-package (e.g., `internal/logger/spc/`) would require changing call sites and interface boundaries throughout the logger package. A same-package split achieves the line-count goal with zero blast radius outside the logger package.

2. **Series config variables move to `trend_spc.go`.** `trendControlLimitSeries` and `trendEWMASeries` are SPC configuration, not schema. Moving them with the SPC functions keeps `process_trend.go` safely under 550 lines and groups configuration with the code that consumes it.

3. **Test file is not split.** `process_trend_test.go` (1,411 lines) is in the same package and requires no changes for the production split to compile and pass. A test-file split is a separate concern and is not part of this spec.

4. **No logic changes.** Every function body is moved verbatim. This eliminates the risk of introducing regressions during a refactor that has no behavioral intent.
