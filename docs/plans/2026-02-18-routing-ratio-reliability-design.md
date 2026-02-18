# Routing Ratio Reliability Review — Design

**Date:** 2026-02-18
**Spec:** `.gromit/specs/routing-ratio-reliability-review.md`
**Approach:** Metrics-first (observability before automation)

## Problem

The `gromit.yaml` routing ratio is `openai: 60` / `claude: 40`. Three gaps exist:

1. Codex iterations log `cost_usd: 0` and `input_tokens: 0` — a tracking bug
2. No per-provider reliability metrics — cannot answer "what is Codex's failure rate?"
3. No automated response when transport failures spike during overnight runs

## Design

### Part 1: Fix Codex Cost/Token Tracking

Trace `processCodexStream` → `Result` → `IterationResult` → `IterationLog`. The `codexUsage` struct captures tokens from `turn.completed` events, but the data does not reach the iteration metrics JSONL. Fix the propagation chain. Add opt-in per-token cost estimation for providers that do not report dollar cost.

### Part 2: Per-Provider Reliability Metrics

Add `provider` and `failure_category` fields to `IterationLog`, `IterationResult`, and `IterationMetric`. Compute `ProviderMetrics` (success rate, transport failure rate, fallback count, cost, tokens) per provider over the existing 30-iteration rolling window. Surface in `process_trend.json` and `gromit stats`.

### Part 3: Conservative Circuit-Breaker

Sliding window of last 10 invocations per provider. When transport failures exceed 30%, reduce that provider's effective ratio to a 20% floor. Restore original ratio after 5 consecutive successes. State is ephemeral (resets each session). Does not override the operator's configured ratio permanently.

### Part 4: Schema Changes

| Field | Added To | Type | Source |
|---|---|---|---|
| `provider` | IterationResult, IterationLog, IterationMetric | string | `Provider.Name()` |
| `failure_category` | IterationResult, IterationLog, IterationMetric | string | `Result.FailureCategory` |

## Key Decisions

- **Metrics before automation.** Ship observability first so circuit-breaker thresholds can be tuned with data.
- **Configured ratio is operator intent.** Circuit-breaker applies temporary session-scoped overrides only.
- **Degraded floor, not full block.** Keeps the degraded provider in rotation to test recovery.
- **Cost estimation is opt-in.** Zero is better than a wrong estimate.
