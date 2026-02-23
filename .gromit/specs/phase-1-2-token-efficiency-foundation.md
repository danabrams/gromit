---
id: phase-1-2-token-efficiency-foundation
source_ideas: []
created: 2026-02-23
---

# Token Efficiency Foundation (Phase 1-2)

## Specification

Implement the first delivery slice of the Token Efficiency Program: establish trustworthy baselines, then ship low-risk token reductions that do not compromise quality.

This spec intentionally excludes large infrastructure bets (for example, full RAG adoption) until baseline metrics and early wins are validated.

## Goals

1. Measure current token, cost, latency, and quality metrics by phase
2. Reduce prompt bloat from oversized tool output and repeated editor context
3. Preserve current quality levels through explicit regression gates
4. Create rollout/rollback controls so optimizations are reversible

## Non-Goals

1. Building a production semantic retrieval system
2. Major runner architecture rewrites not required for token/cost visibility

## Changes

### A. Baseline Instrumentation

Add/extend run telemetry to report at minimum:
- `input_tokens` and `output_tokens` per invocation and phase
- `cost_usd` per invocation and aggregated per run
- Invocation latency
- Retry counts and validation outcomes
- Prompt-size inputs by source bucket where available (template, rules, learnings, tool output, editor context)

Produce a stable per-run summary artifact for before/after comparison.

### B. Tool Output Pruning (Default-On)

For high-volume tools (shell/test/build outputs), reduce prompt-injected content to:
- Failure/error sections
- Compact pass/fail counts
- Optional short tail excerpt for debugging

Keep full raw output available in logs/files, but avoid injecting full logs into LLM context by default.

Pruning policy requirements:
- Deterministic truncation/summarization strategy
- Explicit marker indicating output was pruned
- Configurable limits with sensible defaults

### C. Differential Context Updates

For IDE/editor context payloads, send deltas (or concise change summaries) between turns instead of full state snapshots whenever possible.

Add fallback behavior:
- If delta cannot be safely applied, send a full snapshot and reset baseline

## Acceptance Criteria

1. Baseline metrics:
- A run summary includes per-phase tokens, cost, and latency
- Summary is deterministic enough to compare repeated runs

2. Tool output pruning:
- Long tool outputs are not fully embedded in prompt history by default
- Failure details required for debugging remain available to the model

3. Differential context:
- Unchanged editor state is not fully resent each turn
- Delta failure paths safely recover with full snapshot fallback

4. Quality gates:
- No increase in validation failure rate beyond agreed threshold
- No material drop in run success rate on comparison workload
5. Operability:
- Each optimization can be enabled/disabled independently via config
- Observability captures whether pruning/delta mode was used per invocation

## Rollout Plan

1. Land instrumentation first and collect baseline data
2. Ship tool-output pruning behind a feature flag; validate on fixed workload
3. Enable by default after gate pass
4. Ship differential context updates behind a feature flag; validate and then default on

## Measurement Plan

Evaluate on a fixed workload sample and report:
- Median input tokens per run (target reduction: >=15%)
- Median run cost (target reduction: >=10%)
- Median run duration (target reduction: non-negative; target >=5%)
- Run success rate and validation retry rate (must not regress materially)

Minimum evaluation protocol:
1. Run baseline workload at least 3 times before change
2. Run same workload at least 3 times after change
3. Compare medians and record variance notes in run report

## Risks and Mitigations

1. Risk: Over-pruning removes needed debugging detail
- Mitigation: preserve structured error blocks and optional tail excerpts

2. Risk: Delta context drift causes model confusion
- Mitigation: checksum/versioned context baseline with full-resync fallback

3. Risk: Metric noise hides true impact
- Mitigation: fixed workload set and repeated trial runs before decisions

## Dependencies

1. Existing iteration logging infrastructure for per-phase persistence
2. Prompt/context builder paths that currently inject tool output and IDE context
3. Config flags for staged rollout and rollback

## Decisions

1. Ship measurement before optimization. No token-saving changes without baseline.
2. Keep full fidelity logs out-of-band. Prompt context receives compact failure-focused views.
3. Favor deterministic transforms over LLM summarization for pruning.

## Related Specs

1. `phase-3-token-efficiency-cache-and-tiering`
2. `phase-4-token-efficiency-rag-evaluation`
