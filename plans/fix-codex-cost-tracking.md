---
id: fix-codex-cost-tracking
spec: null
created: 2026-02-18
decomposed: false
---

# Fix: Cost tracking for Codex models

## Problem

`gromit status` shows $0.00 for all Codex model iterations despite significant token usage (~118M tokens across 42 recent iterations). Three bugs compound: Codex CLI omits `total_cost_usd` from JSONL events, the fallback config lookup uses a mismatched provider name, and the pricing fields are commented out.

See investigation report: `.gromit/reports/debug-20260218-231500.md`

## Architecture

The cost flow is: Provider CLI → `provider.Result.CostUSD` → `StreamStats.MergeCostData` → `estimatedCostUSD()` fallback → `IterationLog.CostUSD`.

The fallback path (`estimatedCostUSD` in `callbacks.go`) is the only way to get cost when the provider CLI doesn't report it. It needs two things to work: (a) finding the right `ProviderDef` by provider name, and (b) that `ProviderDef` having non-zero `CostPer1kInput`/`CostPer1kOutput`.

Currently (a) fails because `CodexProvider.Name()` returns `"codex"` but the config key is `"openai"`, and (b) fails because the pricing values are commented out.

## Research & Context

- `internal/runner/callbacks.go:110-120` — `estimatedCostUSD()` does `r.cfg.Providers[providerName]` lookup
- `internal/runner/constructor.go:192-217` — `buildProvidersFromConfig()` stores providers by config key, but `p.Name()` returns a different runtime name
- `internal/config/config.go:293-313` — `ProviderDef` struct and `EstimateCost()` method
- `internal/provider/codex.go:580-586` — `codexUsage` struct with `TotalCostUSD` field (always 0 from CLI)
- `gromit.yaml:21-42` — provider config with commented-out pricing

## Tasks

### 1. Add provider cost lookup map to Runner

**Files**: `internal/runner/constructor.go`, `internal/runner/runner.go` (or wherever Runner struct lives)

Build a `map[string]config.ProviderDef` keyed by **runtime provider name** (`p.Name()`) during router construction. Store it on the Runner. This decouples cost estimation from the config key naming.

In `buildProvidersFromConfig`, after creating each provider, record `providerCostDefs[provider.Name()] = def`. Return this map alongside the providers map. Thread it into the Runner struct.

**Size**: ~20 lines changed across 2 files.

### 2. Update `estimatedCostUSD` to use the new lookup

**Files**: `internal/runner/callbacks.go`

Replace `r.cfg.Providers[providerName]` with `r.providerCostDefs[providerName]`. This fixes the name mismatch: the map is keyed by `p.Name()` ("codex") not the config key ("openai").

**Size**: ~5 lines changed.

### 3. Uncomment and set pricing in gromit.yaml

**Files**: `gromit.yaml`

Uncomment `cost_per_1k_input` and `cost_per_1k_output` for both providers. Use current pricing:

- Claude: `cost_per_1k_input: 0.015`, `cost_per_1k_output: 0.075` (blended across opus/sonnet/haiku — or use sonnet as baseline since it handles most iterations)
- OpenAI/Codex: `cost_per_1k_input: 0.003`, `cost_per_1k_output: 0.012` (verify current rates)

Note: These are fallback estimates for when the provider CLI doesn't report cost. Claude CLI already reports accurate per-model cost, so the Claude pricing here only matters if stream parsing fails.

**Size**: 4 lines uncommented + values updated.

### 4. Add tests for cost estimation with provider name indirection

**Files**: `internal/runner/callbacks_provider_result_test.go` (or new test file)

Test that `estimatedCostUSD` correctly estimates cost when:
- Provider reports cost directly (returns reported cost)
- Provider reports tokens but no cost, with pricing configured (returns estimated cost)
- Provider name differs from config key (the bug being fixed)
- No pricing configured (returns 0)

**Size**: ~40 lines of test code.

### 5. Verify provider field populated in iteration logs

**Files**: `internal/runner/process.go` (or wherever `writeIterationLog` is called)

Confirm `bc.Result.Provider` is being set before the iteration log is written. Current logs show the `provider` field is empty. If it's not set, wire it from `invResult.ProviderName`. This isn't strictly required for cost tracking but aids debugging.

**Size**: ~5 lines if a fix is needed, 0 if already wired.

## Dependencies

- Task 1 must complete before Task 2 (Task 2 uses the map from Task 1)
- Task 3 is independent (config-only)
- Task 4 depends on Tasks 1+2 (tests the new behavior)
- Task 5 is independent

## Testing Strategy

1. Unit tests (Task 4) verify the cost estimation logic in isolation
2. After all tasks, run a short `gromit run` with a mix of Claude and Codex beads, then check `gromit status` confirms non-zero cost for Codex models
3. Verify existing tests pass: `go test ./internal/runner/... ./internal/config/... ./internal/provider/...`
