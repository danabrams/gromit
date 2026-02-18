---
id: routing-ratio-reliability-review
source_ideas: []
created: 2026-02-18
---

# Routing Ratio Reliability Review

## Specification

The `gromit.yaml` routing ratio shifted to `openai: 60` / `claude: 40`, sending most LLM invocations through Codex CLI. Debug logs document Codex stream disconnects and transport instability. This spec adds per-provider observability, fixes the Codex cost-tracking gap, and introduces a conservative circuit-breaker that temporarily shifts routing away from an unstable provider.

### Why

Three problems exist today:

1. **No cost visibility for Codex.** All Codex iterations log `cost_usd: 0`. The `processCodexStream` function captures token usage from `turn.completed` events, but the data either fails to propagate to the iteration log or Codex omits `total_cost_usd` under ChatGPT auth. Without cost data, the operator cannot compare provider economics.

2. **No per-provider reliability metrics.** The iteration log records `model` but not `provider`. Existing metrics (`process_trend.json`, `ReliabilityMetrics`) aggregate across all providers. The operator cannot answer "what is Codex's transport failure rate?" without grepping raw logs.

3. **No automated response to transport instability.** When Codex stream disconnects spike, the existing retry-and-fallback logic handles individual invocations, but the routing ratio remains unchanged. Overnight runs continue sending 60% of work to an unstable provider.

### Part 1: Fix Codex Cost and Token Tracking

**Problem:** `IterationLog.CostUSD` is always zero for Codex invocations.

**Root cause investigation:** Trace the data flow:
- `processCodexStream` returns `*codexUsage` with `InputTokens`, `OutputTokens`, `TotalCostUSD`
- `StreamRun` copies these to `Result.CostUSD`, `Result.InputTokens`, `Result.OutputTokens`
- The runner copies `Result` fields to `IterationResult`
- `WriteIterationLog` copies `IterationResult` fields to `IterationLog`

Identify where the chain breaks. Likely causes:
- `TotalCostUSD` is zero in the Codex `turn.completed` event (ChatGPT auth does not report dollar cost)
- Token fields propagate correctly but the iteration metrics JSONL writer only checks `CostUSD`

**Fix:**
- Ensure `InputTokens` and `OutputTokens` propagate from `Result` through `IterationResult` to both `IterationLog` and `IterationMetric`
- If `TotalCostUSD` is zero but tokens are nonzero, estimate cost using published per-token pricing for the model (configurable in `gromit.yaml` under `providers.<name>.cost_per_1k_input` and `providers.<name>.cost_per_1k_output`, defaulting to zero when unknown)
- Add a `provider` field to `IterationMetric` so downstream aggregation does not rely on model-name heuristics

### Part 2: Per-Provider Reliability Metrics

Add a `ProviderMetrics` struct to the metrics system:

```go
type ProviderMetrics struct {
    Name                 string  `json:"name"`
    TotalInvocations     int     `json:"total_invocations"`
    Successes            int     `json:"successes"`
    SuccessRate          float64 `json:"success_rate"`
    TransportFailures    int     `json:"transport_failures"`
    TransportFailureRate float64 `json:"transport_failure_rate"`
    FallbacksTriggered   int     `json:"fallbacks_triggered"`
    AvgDurationMs        int64   `json:"avg_duration_ms"`
    TotalCostUSD         float64 `json:"total_cost_usd"`
    TotalInputTokens     int     `json:"total_input_tokens"`
    TotalOutputTokens    int     `json:"total_output_tokens"`
}
```

**Data source:** The iteration JSONL logs. Requires two new fields on `IterationLog` and `IterationMetric`:
- `provider` (string): Provider name (`"claude"` or `"codex"`). Set by the runner from `Provider.Name()`.
- `failure_category` (string): From `Result.FailureCategory` (e.g., `"transport_disconnect"`, `"rate_limited"`, `"none"`).

**Aggregation:** Compute `[]ProviderMetrics` over the same 30-iteration rolling window used by `ProcessTrendWindow`. Add a `provider_metrics` field to `process_trend.json`.

**Surfacing:** `gromit stats` already reads `process_trend.json`. The new `provider_metrics` array appears in its output with no additional code beyond the JSON structure change.

### Part 3: Conservative Circuit-Breaker

Add a circuit-breaker to the `Router` that temporarily reduces a provider's effective ratio when transport failures spike.

**Mechanism:**
- The Router maintains a sliding window of the last N invocations per provider (configurable, default 10).
- Each invocation records success or failure category.
- When `transport_disconnect` failures in the window exceed a threshold (configurable, default 30%), the Router enters "degraded" mode for that provider.
- In degraded mode, the provider's effective ratio drops to a configurable floor (default 20%). The freed ratio shifts to the healthiest available provider.
- After a recovery window of consecutive successes (configurable, default 5), the Router restores the original configured ratio.

**Configuration:**

```yaml
routing:
  circuit_breaker:
    enabled: true
    window_size: 10            # invocations to track per provider
    failure_threshold: 0.3     # transport failure rate that triggers degradation
    degraded_floor: 20         # minimum ratio % in degraded mode
    recovery_successes: 5      # consecutive successes to restore original ratio
```

**Implementation:**
- Add a `CircuitBreaker` struct to `internal/provider/router.go` (or a new `internal/provider/circuit_breaker.go`).
- `Router.selectByRatio()` checks circuit-breaker state before applying ratios. If a provider is degraded, its effective ratio is clamped to the floor.
- `Router.RecordOutcome(providerName, failureCategory)` updates the sliding window after each invocation.
- The circuit-breaker state is ephemeral (not persisted to `state.json`). It resets each `gromit run` session. Persistence adds complexity without clear value — transport instability is session-scoped.

**Interaction with existing fallback:** The circuit-breaker and the cooldown-based `MarkUnavailable` serve different purposes:
- `MarkUnavailable` handles hard failures (usage limits) — provider is fully blocked for a cooldown period.
- Circuit-breaker handles soft degradation (elevated transport errors) — provider still receives some traffic at a reduced ratio.

Both can be active simultaneously. A provider can be degraded (reduced ratio) without being unavailable (zero ratio).

### Part 4: IterationLog and IterationMetric Schema Changes

Add these fields to `IterationLog`, `IterationResult`, and `IterationMetric`:

| Field | Type | Description |
|---|---|---|
| `provider` | `string` | Provider name from `Provider.Name()` |
| `failure_category` | `string` | From `Result.FailureCategory` (empty string for success) |

The runner sets `provider` from the selected provider's `Name()` method at invocation time. It sets `failure_category` from the `Result.FailureCategory` field.

Backward compatibility: Existing log entries without these fields parse as zero-values. The metrics aggregation code falls back to inferring provider from model name when `provider` is empty (for historical data).

## Acceptance Criteria

- `IterationLog` and `IterationMetric` include `provider` and `failure_category` fields, populated by the runner
- Codex invocations log nonzero `input_tokens` and `output_tokens` when Codex reports token usage
- When `total_cost_usd` is zero but tokens are nonzero, a cost estimate is computed if per-token pricing is configured
- `process_trend.json` includes a `provider_metrics` array with per-provider success rate, transport failure rate, fallback count, duration, cost, and token totals
- Circuit-breaker reduces a provider's effective ratio when transport failures exceed the configured threshold
- Circuit-breaker restores the original ratio after the configured number of consecutive successes
- Circuit-breaker configuration is optional; when omitted, the Router behaves as today
- Existing tests pass; new tests cover circuit-breaker transitions (healthy → degraded → recovered)
- `gromit stats` displays per-provider metrics

## Decisions

1. **Metrics first, then circuit-breaker.** Per-provider observability ships before automated ratio adjustment. Each part is independently useful. The circuit-breaker thresholds can be tuned using the metrics data.

2. **Circuit-breaker state is ephemeral.** Transport instability is session-scoped (network conditions change between runs). Persisting degradation state across sessions would cause stale decisions. Each `gromit run` starts with a clean circuit-breaker.

3. **The configured ratio remains the operator's intent.** The circuit-breaker only applies temporary overrides during a session. It never writes back to `gromit.yaml`. The operator's ratio reflects subscription budget allocation, not quality preference.

4. **Provider field on logs, not model-name inference.** Inferring provider from model name is fragile (new models break the heuristic). An explicit `provider` field is cheaper to add than a mapping table to maintain.

5. **Cost estimation is opt-in.** If the provider CLI does not report dollar cost, Gromit estimates from tokens only when the operator configures per-token pricing. Zero cost is better than a wrong estimate.

6. **Degraded floor, not full block.** The circuit-breaker reduces ratio rather than blocking the provider entirely. This keeps the provider in rotation (some traffic tests whether it has recovered) and avoids over-concentrating on a single provider.

## Research & Context

### Production Data (2026-02-18)

- 298 total iterations logged
- Latest 30-iteration window: 86.7% success rate, 83.3% first-pass success, 3.3% escalation rate
- Codex models (`gpt-5.2-codex`, `gpt-5.3-codex`) appear in ~50% of recent iterations
- All Codex iterations report `cost_usd: 0` and `input_tokens: 0` — confirming the tracking gap
- Transport disconnect handling exists in `codex.go` (retry with backoff) and `callbacks.go` (cross-provider ATDD fallback)

### Existing Infrastructure

- `internal/provider/codex.go`: `classifyCodexFailure()` detects transport disconnects. `runWithRetry()` retries transient failures up to 2 times.
- `internal/provider/router.go`: `Router` with `Select()`, `SelectCross()`, `MarkUnavailable()`. Ratio balancing via `selectByRatio()`.
- `internal/logger/process_trend.go`: `ProcessTrendWindow`, `TrendControlLimit`, 30-iteration rolling window.
- `internal/logger/reliability.go`: `ReliabilityMetrics` with autonomy rate, first-pass success, MTTR, escalation rates.
- `internal/runner/runtypes/types.go`: `IterationResult` — has `Model`, `CostUSD`, `InputTokens`, `OutputTokens`, but no `Provider` or `FailureCategory`.

### Related Specs

- `multi-provider-routing` — Defined the provider abstraction, router, and ratio balancing
- `codex-streaming-parity` — Defined Codex JSONL event parsing and cost extraction
- `wire-router-from-config` — Wired the router from `gromit.yaml` providers section
