---
epic_id: token-efficiency-program
created: 2026-02-23
---

# Token Efficiency Program

## Problem

Token spend is driven by repeated static context, oversized tool output, and expensive-model usage on utility work. Current plans identify useful ideas but do not define a measurable rollout sequence, risk controls, or clear stop/go gates.

## Vision

Reduce per-iteration token cost and latency without reducing delivery quality by implementing token controls in phases:
- Measure first
- Ship low-risk reductions first
- Gate higher-risk changes with A/B evidence

## Scope

### In Scope

1. Instrumentation and baselines for token, cost, latency, and quality metrics
2. Tool-output pruning and differential context updates
3. Provider caching and model-tier routing improvements
4. Controlled evaluation of retrieval-assisted code discovery (RAG)

### Out of Scope

1. Large architecture rewrites unrelated to token/cost performance
2. Replacing core run-loop behavior that is not tied to token efficiency

## Phases

### Phase 1: Measurement Foundation

Define and persist per-phase metrics:
- Input/output tokens
- Cost per run and per bead
- Latency per invocation
- Success/failure and retry rates
- Cache hit/miss rates (once caching is enabled)

### Phase 2: Low-Risk Context Reduction

Deliver lowest-risk optimizations first:
- Aggressive tool-output pruning defaults
- Differential IDE/editor context updates instead of full-state resend
- Prompt shaping for maximum cacheable prefix stability

### Phase 3: Caching and Tiering

Implement provider-aware caching and routing:
- Gemini context cache lifecycle with explicit invalidation strategy
- Claude prompt-cache-friendly prompt structure for static preamble
- Route utility tasks to lower-cost models; preserve high-tier use for complex execution tasks

### Phase 4: RAG Experiment (Gated)

Run a bounded experiment for semantic retrieval:
- Local index + retrieval tool for code entry-point discovery
- Compare against baseline grep/read flow on fixed workloads
- Adopt only if token/cost gains are material with no meaningful quality regression

## Planned Specs

1. `phase-1-2-token-efficiency-foundation` (Phase 1-2)
2. `phase-3-token-efficiency-cache-and-tiering` (Phase 3)
3. `phase-4-token-efficiency-rag-evaluation` (Phase 4)

## Success Criteria

1. Measurable reduction in median input tokens per run
2. Measurable reduction in median run cost
3. No statistically meaningful drop in task success rate
4. No increase in unresolved validation failures

## Program Rules

1. No optimization ships without baseline and post-change measurement
2. Low-risk changes ship before high-complexity infrastructure additions
3. Any change that reduces context must define quality/regression gates
4. If a change fails quality gates, rollback or disable behind config
