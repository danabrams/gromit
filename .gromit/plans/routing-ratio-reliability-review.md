---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T16:03:19Z"
id: routing-ratio-reliability-review
source_spec: routing-ratio-reliability-review
---

# Routing Ratio Reliability Review Implementation Plan

**Goal:** Add per-provider observability (cost tracking, reliability metrics), fix the Codex cost gap, and introduce a conservative circuit-breaker that temporarily shifts routing away from an unstable provider.

**Architecture:** Thread provider name and failure category through the existing IterationResult → IterationLog → IterationMetric pipeline, add opt-in cost estimation from configured per-token pricing, introduce ProviderMetrics aggregation to the rolling-window metrics system, and add an ephemeral circuit-breaker to the Router that reduces a degraded provider's effective ratio.

**Tech Stack:** Go, YAML config, JSONL logging

**Spec:** `.gromit/specs/routing-ratio-reliability-review.md`

---

## Architecture

### Data Flow: Provider Name and Failure Category

Currently, `InvocationResult.ProviderName` and `InvocationResult.ProviderResult.FailureCategory` are captured at invocation time (in `execution/invoker.go:84,198`) but never persisted to iteration logs. The fix threads these values through two hops:

1. **callbacks.go** — After `executeClaudeInvocation()`, copy `invResult.ProviderName` → `bc.Result.Provider` and `invResult.ProviderResult.FailureCategory` → `bc.Result.FailureCategory`
2. **logging.go** — In the `IterationLog` literal (line 46-90), map from `result.Provider` and `result.FailureCategory`

### Cost Estimation

The Codex API under ChatGPT auth returns `TotalCostUSD: 0` but does report `InputTokens` and `OutputTokens`. The estimation is opt-in:

- `ProviderDef` gains `CostPer1kInput` and `CostPer1kOutput` float64 fields
- In `makeInvokeFn()`, when `CostUSD == 0` and tokens are nonzero, look up the provider's pricing config and compute `(inputTokens/1000 * costPer1kIn) + (outputTokens/1000 * costPer1kOut)`
- The runner has access to `r.cfg.Providers` to look up pricing by provider name

### Provider Metrics Aggregation

New `ProviderMetrics` struct computed from the same iteration JSONL data used by `ProcessTrendWindow`. The aggregation:

- Groups entries by the `provider` field (falling back to model-name inference for historical entries without the field)
- Computes per-provider: total invocations, successes, success rate, transport failures, transport failure rate, fallbacks triggered, avg duration, total cost, total tokens
- Uses the same 30-iteration rolling window as `ProcessTrendWindow`
- Added as `[]ProviderMetrics` on `ProcessTrend`, written to `process_trend.json`

### Circuit Breaker

Ephemeral (not persisted to state.json), resets each `gromit run` session.

**State machine:** healthy → degraded → recovered (= healthy)

**Mechanism:**
- Maintains a fixed-size ring buffer of outcomes per provider (default size 10)
- `RecordOutcome(providerName, failureCategory)` appends to the ring
- When transport_disconnect rate in the window exceeds threshold (default 30%), provider enters degraded mode
- In degraded mode, `EffectiveRatio()` returns the configured floor (default 20%) instead of the original ratio
- After N consecutive successes (default 5), restores original ratio

**Integration with Router:**
- `selectByRatio()` calls `circuitBreaker.EffectiveRatio(name, configuredRatio)` instead of using raw configured ratio
- Freed ratio redistributed proportionally to healthy providers through the natural gap calculation
- Circuit breaker is nil-safe: when config omits circuit_breaker, Router behaves identically to today

**Interaction with MarkUnavailable:**
- `MarkUnavailable` = hard block (usage limits) — provider gets zero traffic for cooldown period
- Circuit breaker = soft degradation (transport errors) — provider gets reduced traffic
- Both can be active simultaneously; availability check runs first, then circuit breaker adjusts ratios for available providers

```
Provider.StreamRun() → Result{FailureCategory, CostUSD, Tokens}
       ↓
execution.Invoker.Execute() → InvocationResult{ProviderName, ProviderResult}
       ↓
makeInvokeFn() → bc.Result.Provider, bc.Result.FailureCategory
       ↓                    ↓
Router.RecordOutcome()    writeIterationLog() → IterationLog{Provider, FailureCategory}
       ↓                                              ↓
CircuitBreaker.Record()                    BuildContinuousMetrics()
       ↓                                              ↓
selectByRatio() clamp                    IterationMetric → ProcessTrend.ProviderMetrics
```

**Tradeoffs:**
- **Separate file for circuit-breaker:** Clean separation from router.go which is already substantial
- **Ephemeral circuit-breaker state:** Transport instability is session-scoped; no stale decisions across runs
- **Cost estimation opt-in:** Zero cost is better than wrong estimate
- **Cost estimation in callbacks.go:** Keeps it near other cost-data logic; the runner already has config access
- **ProviderMetrics in ProcessTrend:** `gromit stats` already reads this file — no stats.go changes needed

## Test Strategy

**Unit Tests:**
- Circuit-breaker state transitions: healthy → degraded → recovered, with table-driven cases
- Circuit-breaker edge cases: window not full, threshold boundary (exactly 30%), all providers degraded, nil receiver
- ProviderMetrics aggregation: single provider, multi provider, empty provider fallback, window boundary
- Cost estimation: zero cost + nonzero tokens + pricing → correct estimate; no pricing → stays zero; reported cost → used as-is
- Config: CircuitBreakerConfig defaults, YAML overrides, validation; CostPer1k fields deserialize

**Integration Tests:**
- writeIterationLog includes provider and failure_category in output
- BuildContinuousMetrics produces provider_metrics in process_trend.json
- Router with circuit-breaker selects correctly under degraded state
- Backward compat: old JSONL without provider/failure_category parses as zero-values

**Mocking Strategy:**
- Circuit-breaker: pure struct, no mocks (in-memory sliding window)
- ProviderMetrics: construct `[]IterationMetric` directly
- Router + circuit-breaker: existing `mockProvider` and `mockStateFile` from router_test.go
- Cost estimation: table-driven with direct function calls

**Test Files:**
- `internal/provider/circuit_breaker_test.go` — new
- `internal/provider/router_test.go` — extend
- `internal/logger/process_trend_test.go` — extend
- `internal/runner/logging_test.go` — extend or create
- `internal/runner/callbacks_test.go` — extend
- `internal/config/config_test.go` — extend

## Implementation Tasks

### Task 1: Add Provider and FailureCategory fields to schema structs

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Modify: `internal/logger/process_trend.go`

**What to Do:**
Add `Provider string` and `FailureCategory string` to `IterationResult` (after existing diagnostic fields). Add `Provider string` with json tag `provider,omitempty` and `FailureCategory string` with json tag `failure_category,omitempty` to `IterationLog` (after Model). Add the same two fields with matching json tags to `IterationMetric`. All fields are additive with zero-value empty string, so existing JSONL parses cleanly.

**Acceptance Criteria:**
- `IterationResult`, `IterationLog`, and `IterationMetric` each have `Provider` and `FailureCategory` string fields
- JSON tags use `omitempty` for backward compatibility
- Existing tests pass unchanged (fields are zero-value safe)

**Dependencies:** None (foundation task)

---

### Task 2: Wire Provider and FailureCategory through runner logging pipeline

**Files:**
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/runner/logging.go`
- Test: `internal/runner/logging_test.go`

**What to Do:**
In `makeInvokeFn()` (callbacks.go), after `executeClaudeInvocation()` succeeds and cost data is populated (~line 73-79), propagate `invResult.ProviderName` to `bc.Result.Provider` and `invResult.ProviderResult.FailureCategory` to `bc.Result.FailureCategory` (nil-guard on ProviderResult). In `writeIterationLog()` (logging.go), add `Provider: result.Provider` and `FailureCategory: result.FailureCategory` to the IterationLog struct literal. In `buildIterationMetrics()` (process_trend.go), copy `Provider` and `FailureCategory` from `IterationLog` to `IterationMetric`. Add test verifying both fields appear in logged output.

**Acceptance Criteria:**
- After an invocation, `bc.Result.Provider` contains the provider name (e.g., "claude", "codex")
- After a failed invocation, `bc.Result.FailureCategory` contains the category string
- `writeIterationLog` includes both fields in the IterationLog struct literal
- IterationMetric entries include both fields when computed from logs

**Dependencies:** Task 1

---

### Task 3: Add cost estimation config fields and logic

**Files:**
- Modify: `internal/config/config.go`
- Modify: `gromit.yaml`
- Modify: `internal/runner/callbacks.go`
- Test: `internal/config/config_test.go`
- Test: `internal/runner/callbacks_test.go`

**What to Do:**
Add `CostPer1kInput float64` and `CostPer1kOutput float64` fields (yaml tags `cost_per_1k_input` and `cost_per_1k_output`) to `ProviderDef` in config.go. Add commented-out example values to gromit.yaml under each provider. Add `estimateCost(inputTokens, outputTokens int, costPer1kIn, costPer1kOut float64) float64` helper. In `makeInvokeFn()`, after cost data is merged, if `bc.Result.CostUSD == 0` and `bc.Result.InputTokens > 0 || bc.Result.OutputTokens > 0`, look up the provider's pricing from `r.cfg.Providers[bc.Result.Provider]` and apply `estimateCost()`. Add config test for YAML deserialization and unit test for estimateCost.

**Acceptance Criteria:**
- `ProviderDef` has `CostPer1kInput` and `CostPer1kOutput` fields that deserialize from YAML
- When CostUSD is zero, tokens nonzero, and pricing configured: cost estimated correctly
- When pricing not configured (zero values): cost stays zero
- When CostUSD already nonzero: reported cost used as-is

**Dependencies:** Task 2 (needs Provider field on bc.Result to look up pricing)

---

### Task 4: Add ProviderMetrics struct and aggregation

**Files:**
- Modify: `internal/logger/process_trend.go`
- Test: `internal/logger/process_trend_test.go`

**What to Do:**
Add `ProviderMetrics` struct with fields: `Name string`, `TotalInvocations int`, `Successes int`, `SuccessRate float64`, `TransportFailures int`, `TransportFailureRate float64`, `FallbacksTriggered int`, `AvgDurationMs int64`, `TotalCostUSD float64`, `TotalInputTokens int`, `TotalOutputTokens int` — all with appropriate JSON tags. Add `computeProviderMetrics(entries []IterationMetric) []ProviderMetrics` function that groups by Provider field and computes all metrics per group. For entries with empty Provider field, infer from model name (models containing "codex" or "gpt" → "openai", otherwise → "claude"). Add `ProviderMetrics []ProviderMetrics` field to `ProcessTrend` (json tag `provider_metrics,omitempty`). In `buildProcessTrend()`, call `computeProviderMetrics()` on the latest window entries and set on the trend. Write table-driven tests.

**Acceptance Criteria:**
- `ProcessTrend` includes `provider_metrics` array in JSON output
- Per-provider success rate and transport failure rate computed correctly
- Empty provider field falls back to model-name inference
- `gromit stats --json` automatically includes provider_metrics (no stats.go changes)

**Dependencies:** Task 1 (needs Provider/FailureCategory on IterationMetric)

---

### Task 5: Implement CircuitBreaker with sliding window and state transitions

**Files:**
- Create: `internal/provider/circuit_breaker.go`
- Create: `internal/provider/circuit_breaker_test.go`

**What to Do:**
Implement `CircuitBreaker` struct with: per-provider ring buffers (`map[string]*outcomeWindow`), configurable window size, failure threshold, degraded floor ratio, recovery successes count. `outcomeWindow` is a fixed-size circular buffer of outcome records. Methods: `Record(providerName string, failureCategory string)` — appends to provider's window, updates degraded state and consecutive success counter. `EffectiveRatio(providerName string, configuredRatio int) int` — returns configuredRatio if healthy, floor if degraded. `IsDegraded(providerName string) bool`. A nil `*CircuitBreaker` is safe (all methods return pass-through values). Only `transport_disconnect` failures count toward degradation threshold. Write comprehensive table-driven tests.

**Acceptance Criteria:**
- Transitions from healthy to degraded when transport failure rate exceeds threshold
- Transitions back to healthy after configured consecutive successes
- Nil CircuitBreaker returns pass-through values (no degradation)
- Each provider tracked independently
- Non-transport failures don't trigger degradation

**Dependencies:** None (standalone logic)

**Notes:** Use a ring buffer (fixed-size slice + write index) for the sliding window. Only `transport_disconnect` failures count toward threshold; other categories are recorded but ignored for state transitions.

---

### Task 6: Add CircuitBreakerConfig to config and gromit.yaml

**Files:**
- Modify: `internal/config/config.go`
- Modify: `gromit.yaml`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `CircuitBreakerConfig` struct with fields: `Enabled bool` (yaml: `enabled`), `WindowSize int` (yaml: `window_size`), `FailureThreshold float64` (yaml: `failure_threshold`), `DegradedFloor int` (yaml: `degraded_floor`), `RecoverySuccesses int` (yaml: `recovery_successes`). Add `CircuitBreaker CircuitBreakerConfig` field to `RoutingConfig` (yaml: `circuit_breaker`). In `SetDefaults()`, when Enabled is true but fields are zero, apply defaults: WindowSize=10, FailureThreshold=0.3, DegradedFloor=20, RecoverySuccesses=5. In `Validate()`, reject invalid values when Enabled: WindowSize<1, FailureThreshold outside (0,1), DegradedFloor<0 or >100, RecoverySuccesses<1. Add commented-out circuit_breaker section to gromit.yaml under routing. Add config tests.

**Acceptance Criteria:**
- CircuitBreakerConfig parses from YAML correctly
- Defaults applied when enabled but fields zero
- Validation rejects invalid values when enabled
- When omitted from YAML, config is zero-value (Enabled=false) — no behavior change

**Dependencies:** None (config only)

---

### Task 7: Wire CircuitBreaker into Router

**Files:**
- Modify: `internal/provider/router.go`
- Test: `internal/provider/router_test.go`

**What to Do:**
Add `circuitBreaker *CircuitBreaker` field to Router struct. Accept it in `NewRouter()` constructor (nil when disabled). Add `RecordOutcome(providerName string, failureCategory string)` method on Router that delegates to circuit breaker (no-op when nil). In `selectByRatio()`, replace direct `r.ratio[name]` usage with `r.effectiveRatio(name)` helper that calls `r.circuitBreaker.EffectiveRatio(name, r.ratio[name])`. When a provider's effective ratio is reduced, the gap calculation naturally favors healthy providers. Add tests: router with degraded provider selects alternate more often; router without circuit-breaker behaves identically; RecordOutcome updates state.

**Acceptance Criteria:**
- Router with circuit-breaker reduces degraded provider's selection frequency
- Router without circuit-breaker (nil) behaves identically to today
- RecordOutcome delegates to circuit breaker (no-op when nil)
- Degraded provider still receives traffic at floor ratio
- MarkUnavailable (hard block) takes precedence over degradation

**Dependencies:** Task 5, Task 6

---

### Task 8: Wire RecordOutcome from runner invocations

**Files:**
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/runner/execution/invoker.go`
- Test: `internal/runner/callbacks_test.go`

**What to Do:**
Add `RecordOutcome(providerName string, failureCategory string)` to the `execution.Router` interface (if not already present). In `Invoker.Execute()`, after `StreamRun` completes (~line 142), call `inv.router.RecordOutcome(p.Name(), failureCategory)` where failureCategory comes from `providerResult.FailureCategory` (empty string when nil or success). In `makeInvokeFn()` (callbacks.go), after setting `bc.Result.Provider`/`bc.Result.FailureCategory`, call `r.router.RecordOutcome(bc.Result.Provider, bc.Result.FailureCategory)`. Also wire RecordOutcome in the ATDD invocation path (makeMethodologyExec's invokeFn) after streamInvoke returns. Add test verifying RecordOutcome called with correct arguments.

**Acceptance Criteria:**
- Every build invocation records its outcome with the router
- ATDD invocation outcomes also recorded
- Success invocations record empty failure category
- Failed invocations record the actual failure category
- No-op when circuit-breaker is disabled (RecordOutcome on nil CB is safe)

**Dependencies:** Task 2, Task 7

---

## Notes

- **Parallelism:** Tasks 1, 5, and 6 have no dependencies and can run in parallel. After Task 1 completes, Tasks 2 and 4 can run in parallel. Tasks 5 and 6 are also independent.
- **Backward compatibility is critical.** All new JSONL fields use `omitempty`. Old logs without `provider`/`failure_category` parse as empty strings. The metrics aggregation falls back to model-name inference for historical data.
- **Circuit breaker is ephemeral.** No state.json changes. Resets each `gromit run`. This avoids stale decisions across sessions.
- **gromit stats automatically picks up provider_metrics** because it reads process_trend.json. No stats.go changes needed.
- **Existing bead overlap:** Bead `gromit-2s2q` (Carry provider.Result through InvocationResult) partially overlaps Task 2. If that bead completes first, Task 2 becomes simpler since `ProviderResult` will already be accessible in callbacks.
- **execution.Router interface:** Check whether it already has RecordOutcome or needs the method added. If it's the concrete `*provider.Router`, no interface change needed.
