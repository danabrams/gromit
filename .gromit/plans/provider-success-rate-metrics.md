---
id: provider-success-rate-metrics
source_spec: provider-success-rate-metrics
created: 2026-02-27
decomposed: false
---

# Provider Success Rate Metrics Implementation Plan

**Goal:** Add provider-scoped rolling reliability fields to each `iteration_metrics.jsonl` row so operators can tune routing ratios using recent provider outcomes.

**Architecture:** Extend `buildIterationMetrics` to compute provider-only rolling stats from the same per-row rolling window already used for global rolling fields, using existing provider inference logic for backward-compatible attribution.

**Tech Stack:** Go (`internal/logger`), JSONL metrics emission, existing logger/trend analytics helpers.

**Spec:** `.gromit/specs/provider-success-rate-metrics.md`

---

## Architecture

## Architecture Proposal

**Overview:**  
Add provider-scoped rolling counters during `buildIterationMetrics` and map them into new additive fields on `IterationMetric`, reusing existing provider resolution logic so missing `provider` values continue to infer from model.

**Key Components:**
1. **`IterationMetric` schema (`internal/logger/process_trend.go`)**: add four new JSON fields for provider rolling rates/invocations.
2. **Provider rolling calculator (`internal/logger/trend_builder.go`)**: compute provider-scoped totals from the same row window currently used for rolling metrics.
3. **Provider resolution reuse (`internal/logger/trend_analytics.go`)**: reuse `resolveProviderName(provider, model)` for attribution consistency.

**Integration Points:**
- `buildIterationMetrics` remains the single place that constructs per-row rolling values.
- No changes to `buildProcessTrend` `provider_metrics` output.
- No routing behavior changes.

**Data Flow:**
For each iteration row:
1. Build rolling window as today (`entries[windowStart:idx+1]`).
2. Resolve current row provider (`resolveProviderName(entry.Provider, entry.Model)`).
3. Scan the same window, count only entries matching that resolved provider:
- invocations
- successes
- transport failures (`failure_category == transport_disconnect`)
4. Derive:
- success rate = successes / invocations
- failure rate = 1 - success rate (or failures / invocations)
- transport failure rate = transport failures / invocations
5. Store values on the current `IterationMetric` row.

**Files to Modify:**
- `internal/logger/process_trend.go` - add new `IterationMetric` fields.
- `internal/logger/trend_builder.go` - compute provider rolling metrics per row.
- `internal/logger/process_trend_test.go` - add coverage for mixed-provider windows and inference fallback.
- `internal/logger/trend_builder_test.go` - add focused metric-construction assertions for new fields.

**Files to Create:**
- None expected (can stay within existing files).

**Tradeoffs:**
- **Reuse `resolveProviderName` vs duplicate inference logic**: chose reuse to avoid drift and preserve backward compatibility.
- **Per-row window scan vs incremental per-provider rolling state**: chose per-row scan for simplicity and correctness with current architecture; window size is small (default 30), so cost is negligible.
- **Additive schema changes only**: chosen to keep readers backward-compatible and avoid changing existing trend outputs.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit tests**: validate per-row provider rolling math in `buildIterationMetrics` (success/failure/transport/invocations).
2. **Integration-style logger tests**: ensure provider inference fallback (missing provider, model-based inference) flows into the new rolling fields.
3. **Manual verification**: optional spot-check generated `iteration_metrics.jsonl` for field presence and plausible values.

**Key Test Cases:**
- Mixed-provider rolling window computes row-local provider metrics only from matching provider rows.
- Provider rolling rates use the exact same row window boundaries as existing rolling metrics.
- Missing explicit provider with `gpt/codex` model infers `openai` and computes correct provider rolling values.
- Missing explicit provider with non-openai model infers `claude` (or `unknown` when model empty) and computes correct values.
- Transport failure rate counts only `transport_disconnect` failures for that provider.
- Existing process trend provider metrics tests continue to pass unchanged.

**Mocking Strategy:**
- No mocks needed; use direct `[]IterationLog` fixtures and call `buildIterationMetrics`.
- Use deterministic, small table-driven windows to make expected fractions explicit.

**Coverage Goals:**
- Critical path: all four new fields populated on every emitted row.
- Correctness for explicit provider and inferred provider rows.
- Edge cases:
- single-row window
- provider appears once in window
- provider has zero transport failures
- mixed failure categories where only transport_disconnect is counted

**Test Organization:**
- Add table-driven tests near existing rolling metric tests in:
- `internal/logger/trend_builder_test.go`
- `internal/logger/process_trend_test.go`
- Keep naming style: `TestBuildIterationMetrics_<Behavior>`.

## Implementation Tasks

### Task 1: Extend IterationMetric Schema for Provider Rolling Fields

**Files:**
- Modify: `internal/logger/process_trend.go`

**What to Do:**
Add four additive fields to `IterationMetric` with stable JSON tags:
- `provider_rolling_success_rate` (float64)
- `provider_rolling_failure_rate` (float64)
- `provider_rolling_transport_failure_rate` (float64)
- `provider_rolling_invocations` (int)

Keep all existing fields and tags unchanged.

**Acceptance Criteria:**
- `IterationMetric` includes all four new provider rolling fields with the exact JSON names from the spec.
- Existing JSON field names/ordering compatibility are preserved (additive-only change).
- Existing compilation/tests that depend on `IterationMetric` continue to build.

**Dependencies:**
- None.

**Notes:**
- Keep the field naming aligned with current rolling metric naming conventions.

### Task 2: Compute Provider-Scoped Rolling Metrics in buildIterationMetrics

**Files:**
- Modify: `internal/logger/trend_builder.go`

**What to Do:**
Within each row computation in `buildIterationMetrics`, compute provider-scoped rolling stats over the same `window` already used for global rolling fields.

Implementation shape:
- Resolve the current row provider using `resolveProviderName(entry.Provider, entry.Model)`.
- Iterate over `window` and include only rows whose resolved provider matches.
- Count invocations, successes, and transport disconnect failures.
- Compute success/failure/transport rates using safe zero-denominator handling.
- Populate the new provider rolling fields on the emitted `IterationMetric`.

**Acceptance Criteria:**
- Provider rolling values are derived strictly from the row’s existing rolling window boundaries.
- Provider matching uses existing inference behavior, including rows with empty explicit `provider`.
- Global rolling metrics and unrelated behavior remain unchanged.

**Dependencies:**
- Task 1.

**Notes:**
- Avoid introducing route-tuning or ratio mutation side effects.

### Task 3: Add Regression Tests for Provider Rolling Window Math and Inference

**Files:**
- Modify: `internal/logger/trend_builder_test.go`
- Modify: `internal/logger/process_trend_test.go`

**What to Do:**
Add focused tests validating:
- Mixed-provider windows produce row-local provider rolling rates/invocations.
- Transport failure rate counts only `transport_disconnect`.
- Provider inference fallback works when explicit `provider` is missing.
- New fields are present and correct for generated metrics rows.

Use table-driven fixtures with explicit expected fractions for deterministic assertions.

**Acceptance Criteria:**
- Tests fail without Task 2 and pass with it.
- Both explicit-provider and inferred-provider scenarios are covered.
- Existing provider metrics/process trend tests remain green.

**Dependencies:**
- Task 2.

**Notes:**
- Keep tests narrow to avoid brittle coupling with unrelated trend metrics.

### Task 4: Validate End-to-End Metrics Outputs Remain Backward-Compatible

**Files:**
- Modify: `internal/logger/process_trend_test.go` (if additional integration assertions are needed)

**What to Do:**
Run logger test targets and validate:
- `iteration_metrics.jsonl` rows include new provider rolling fields.
- Existing global rolling fields and `process_trend.json` `provider_metrics` behavior are unchanged.

Add or update assertions only where needed to lock these guarantees.

**Acceptance Criteria:**
- New fields appear in newly generated iteration metrics rows.
- No test indicates behavioral change in `process_trend.json` provider metrics.
- Logger/trend test suites pass.

**Dependencies:**
- Task 3.

**Notes:**
- This is observability-only; no routing logic changes are allowed.

---

## Notes

- This plan intentionally keeps provider granularity at provider-only (not per-model/per-phase) per spec decisions.
- The feature is additive telemetry and should not alter orchestration, retry, escalation, or routing ratio behavior.
- During implementation, keep inference consistency by reusing existing provider resolution helper instead of creating parallel logic.
