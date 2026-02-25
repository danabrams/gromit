# TDD Fresh Context Telemetry Fix Report

**Date:** 2026-02-24
**Bead:** gromit-kkst — "Fix DiffFiles returning nil slice instead of empty slice"
**Model:** gpt-5.1-codex-mini (all modes)
**Manifest:** tdd-vs-single-pass-low.yaml

## Bug Summary

TDD fresh-context iteration logs showed `cost_usd: 0` and `duration_ms: 0` even on successful runs that consumed 181K+ tokens. Both fields are now correctly populated.

## Root Causes

### 1. duration_ms: 0

**Problem:** The `TDDCycleResult` struct had no `DurationMs` field. The TDD fresh-context path in `build.go` constructed a `pipeline.Output` without setting `DurationMs`. The orchestrator then read `buildOut.DurationMs` and got the zero value.

The single-pass path got duration from `provider.Result.Duration` (wall-clock time of the single StreamRun call), but the TDD path makes multiple StreamRun calls across phases and had no equivalent.

**Fix:** Added `DurationMs int64` to `TDDCycleResult`, timed the entire `RunCycles` call in `TDDPipelineAdapter` using `time.Since(startTime).Milliseconds()`, and wired it through `build.go` into `pipeline.Output.DurationMs`. Also added `bc.Result.Duration` accumulation in `buildInvokeFnWithTelemetry` for per-phase duration tracking.

### 2. cost_usd: 0

**Problem:** The codex provider's SSE events do not include `total_cost_usd`, so `provider.Result.CostUSD` is always 0 after StreamRun. The single-pass path compensated for this via `applyCostFallback()` in `constructor_adapters.go:81-98`, which estimates cost from token counts using `config.ProviderDef.EstimateCostForModel()`. The TDD fresh-context path called `provider.StreamRun()` directly (bypassing the `invokerAdapter`) and never applied this fallback.

**Fix:** Threaded `map[string]config.ProviderDef` (costDefs) through `optionalTDDCycleRunner` → `buildTDDCycleRunner` → `buildInvokeFnWithTelemetry`. Applied `applyCostFallback(result, p.Name(), costDefs)` after each phase's StreamRun call, before accumulating into `bc.Result.CostUSD`.

## Files Modified

| File | Change |
|------|--------|
| `internal/pipeline/execute/build.go` | Added `DurationMs int64` to `TDDCycleResult`; wired `result.DurationMs` → `out.DurationMs` in TDD path |
| `internal/runner/tdd_pipeline_adapter.go` | Added `time` import; timed `RunCycles` with `time.Since`; populated `DurationMs` in result |
| `internal/runner/callbacks_tdd.go` | Added `costDefs` parameter to `buildTDDCycleRunner`, `optionalTDDCycleRunner`, `buildInvokeFnWithTelemetry`; applied `applyCostFallback` after each StreamRun; accumulated `bc.Result.Duration` |
| `internal/runner/constructor.go` | Passed `costDefs` to `optionalTDDCycleRunner` |
| `internal/pipeline/execute/build_test.go` | Added `TestBuildRun_TDD_FreshContext_ReturnsDurationAndCostInOutput` |
| `internal/runner/constructor_test.go` | Updated `buildTDDCycleRunner` and `optionalTDDCycleRunner` calls with `nil` costDefs |

## Test Results

All 11 packages pass:

```
ok  github.com/danabrams/gromit/internal/runner             1.086s
ok  github.com/danabrams/gromit/internal/runner/andon        0.012s
ok  github.com/danabrams/gromit/internal/runner/escalation   0.030s
ok  github.com/danabrams/gromit/internal/runner/execution    0.139s
ok  github.com/danabrams/gromit/internal/runner/methodology  0.021s
ok  github.com/danabrams/gromit/internal/runner/policy       0.014s
ok  github.com/danabrams/gromit/internal/runner/reviewpkg    0.015s
ok  github.com/danabrams/gromit/internal/runner/runtypes     0.013s
ok  github.com/danabrams/gromit/internal/runner/tdd          0.007s
ok  github.com/danabrams/gromit/internal/runner/validation   0.156s
ok  github.com/danabrams/gromit/internal/pipeline/execute    0.010s
```

## Benchmark Comparison (gromit-kkst, all gpt-5.1-codex-mini)

| Metric | Single Pass | TDD Shared Context | TDD Fresh Context |
|--------|-------------|--------------------|--------------------|
| **Success** | true | true | true |
| **Duration** | 348s (5m 48s) | 57s (0m 57s) | 179s (2m 59s) |
| **Cost** | $4.62 | $0.67 | $1.46 |
| **Input Tokens** | 2,367,392 | 356,644 | 739,121 |
| **Output Tokens** | 33,821 | 3,482 | 11,960 |
| **Tier** | low | low | medium |
| **Cache Hit** | yes | yes | no |

### Relative to Single Pass

| Metric | TDD Shared Context | TDD Fresh Context |
|--------|--------------------|--------------------|
| **Duration** | 6.2x faster | 1.9x faster |
| **Cost** | 6.9x cheaper | 3.2x cheaper |
| **Input Tokens** | 6.6x fewer | 3.2x fewer |
| **Output Tokens** | 9.7x fewer | 2.8x fewer |

### Observations

- **TDD shared context** is the most efficient mode for this bead: 6x cheaper and faster than single pass, completing in under a minute.
- **TDD fresh context** is a middle ground: 3x cheaper than single pass but 2x more expensive than shared context, due to the overhead of multiple fresh LLM invocations (each requiring full context re-injection).
- Single pass consumed 2.4M input tokens — likely hitting multiple validation-retry cycles that inflated cost and duration.
- TDD fresh context used a "medium" tier while the others used "low", which may reflect the TDD tier selection logic defaulting differently when PhaseMetrics is empty.
- TDD fresh context did not get cache hits (no `cache_hit` field), likely because the fresh-context path bypasses the prompt cache registry used by the `invokerAdapter`.

## Before/After Telemetry (TDD Fresh Context only)

| Field | Before Fix | After Fix |
|-------|-----------|-----------|
| `duration_ms` | 0 | 179,204 |
| `cost_usd` | 0 | 1.46 |
| `input_tokens` | 181,460 (already worked) | 739,121 |
| `output_tokens` | 2,989 (already worked) | 11,960 |
