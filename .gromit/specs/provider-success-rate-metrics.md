---
id: provider-success-rate-metrics
source_ideas: []
created: 2026-02-27
accepted: true
---

# Provider Success Rate Metrics

## Specification

Add provider-level rolling reliability metrics to each row in `iteration_metrics.jsonl` so routing ratio tuning can be informed by recent per-provider outcomes.

This feature is metrics-only. It does not change provider selection behavior and does not auto-adjust `routing.ratio`.

### Behavior

- The system continues generating `iteration_metrics.jsonl` from iteration logs as it does today.
- For each emitted iteration metric row, compute provider-specific rolling metrics over the same rolling window already used for existing rolling fields (default 30).
- Provider-specific calculations are scoped to the row's resolved provider and include all providers (Claude, OpenAI, Gemini, and future providers).
- Historical entries that do not include explicit `provider` continue to resolve provider via existing model-to-provider inference rules.

### New Iteration Metric Fields

Add these additive fields to each `IterationMetric` JSON object in `iteration_metrics.jsonl`:

- `provider_rolling_success_rate` (float): `provider successes / provider invocations` in the row's rolling window
- `provider_rolling_failure_rate` (float): `1 - provider_rolling_success_rate` (or direct failure fraction)
- `provider_rolling_transport_failure_rate` (float): `provider transport_disconnect failures / provider invocations`
- `provider_rolling_invocations` (int): number of window entries attributed to the row's provider

### Existing Outputs

- Keep existing global rolling fields (for example `rolling_success_rate`) unchanged.
- Keep `process_trend.json` `provider_metrics` unchanged.
- This work complements existing trend metrics by embedding provider-specific rolling context into each iteration row.

## Acceptance Criteria

- `iteration_metrics.jsonl` contains `provider_rolling_success_rate`, `provider_rolling_failure_rate`, `provider_rolling_transport_failure_rate`, and `provider_rolling_invocations` for newly generated rows.
- Each new field is computed from the same rolling window used for other `IterationMetric` rolling fields.
- Metrics are provider-scoped and work for all providers, not just Gemini.
- Rows with missing explicit provider attribution still receive provider rolling metrics via existing provider inference behavior.
- No automatic routing-ratio adjustment behavior is introduced.
- Existing logger/trend tests continue to pass, and new tests verify provider-rolling calculations and provider-inference fallback paths.

## Decisions

1. **Provider-only granularity** Provider rolling metrics are aggregated by provider only, without per-phase or per-model slicing, to keep schema size and computation complexity low while directly supporting routing ratio tuning.

2. **All-provider coverage** Metrics are generalized for every provider instead of introducing Gemini-specific fields so the telemetry remains consistent as providers are added or routing ratios change.

3. **Rolling-window semantics** Provider rolling metrics use the same rolling window basis as existing iteration metrics so consumers can compare values directly without mixing time horizons.

4. **Advisory-only output** The feature only improves observability; tuning remains a manual operator decision in `gromit.yaml`.

## Research & Context

### Current State

- `internal/logger/process_trend.go` defines `IterationMetric` and writes `iteration_metrics.jsonl`.
- `internal/logger/trend_builder.go` builds per-iteration rolling metrics from windowed iteration logs.
- `internal/logger/trend_analytics.go` already computes provider aggregates for `process_trend.json` (`computeProviderMetrics`), including success rate and transport failure rate.
- `IterationMetric` currently includes per-iteration `provider` attribution but does not include provider-specific rolling aggregate fields.

### Integration Notes

- Reuse existing provider resolution (`provider` field first, model inference fallback) to preserve backward compatibility.
- Keep field additions additive to avoid breaking historical JSONL readers.
- Add focused tests for mixed-provider windows and transport-failure-rate math.
