---
id: phase-3-token-efficiency-cache-and-tiering
source_ideas: []
created: 2026-02-23
epic: token-efficiency-program
---

# Token Efficiency Cache and Tiering (Phase 3)

## Specification

Implement provider-aware caching and model-tier routing optimizations after Phase 1-2 instrumentation is in place.

This spec converts high-level token-saving guidance into controlled, measurable behavior for:
1. Stable cacheable prompt prefixes
2. Context/prompt caching
3. Utility-task routing to lower-cost models

## Preconditions

1. `phase-1-2-token-efficiency-foundation` metrics are available in run telemetry
2. Baseline cache hit rate is measurable (or explicitly zero before rollout)
3. Feature flags exist for staged enablement

## Changes

### A. Cache-Stable Prompt Structure

Refactor prompt construction so static sections are deterministic and front-loaded:
- System/rules/tool definitions first
- Dynamic task content after static preamble
- Stable ordering of sections and fields

Guarantee equivalent prompts produce equivalent cache keys.

### B. Provider Caching Integration

Add cache lifecycle controls (provider-specific adapters where needed):
- Create/reuse cache entries for static context blocks
- Explicit invalidation when key inputs change (rules/template/tool schema versions)
- TTL and capacity controls
- Telemetry for cache hit/miss/write rates

### C. Model Tier Routing for Utility Tasks

Route non-code-generation utility tasks to lower-cost models by default:
- History compression/summarization
- Tool-output masking/transforms
- Broad codebase discovery and indexing prep

Keep complex execution/editing tasks on higher-capability models.

### D. Guardrails

Add kill switches and fallback behavior:
- If provider cache is unavailable, continue without cache
- If low-tier utility output quality degrades, route task type back to higher tier

## Acceptance Criteria

1. Prompt structure:
- Static preamble ordering is deterministic for cacheable prompt classes
- Prompt-key instability from non-semantic ordering differences is eliminated

2. Caching behavior:
- Cache hit/miss/write metrics are emitted per run
- Invalidations occur when configured version keys change
- Cache failure does not fail the run

3. Routing behavior:
- Utility task categories are explicitly mapped to tier/model in config
- Execution/code-generation paths remain on existing high-fidelity tiers unless explicitly overridden

4. Outcomes:
- Measurable reduction in median input tokens on repeated workloads
- Measurable reduction in median cost with no material success-rate regression

## Config Additions

Introduce/extend config for:
1. `token_efficiency.cache.enabled`
2. `token_efficiency.cache.ttl`
3. `token_efficiency.cache.capacity`
4. `token_efficiency.routing.utility_tier`
5. Per-task routing overrides and kill switches

Defaults should preserve current behavior until explicitly enabled.

## Measurement Plan

Report before/after for fixed workloads:
1. Cache hit rate by prompt class
2. Input token reduction per invocation type
3. Cost reduction per run
4. Success rate, validation retry rate, and rollback-trigger events

## Risks and Mitigations

1. Risk: Cache invalidation bugs return stale context
- Mitigation: Versioned keys + explicit invalidation tests

2. Risk: Utility-tier downgrades reduce output quality
- Mitigation: Task-level fallback to higher tier via kill switch

3. Risk: Prompt-key churn prevents cache reuse
- Mitigation: Deterministic serialization and stable section ordering

## Decisions

1. Caching is optimization-only, never a correctness dependency.
2. Routing is task-category based, not ad-hoc per run.
3. Rollout is progressive: observe in shadow mode, then enforce.

## Related Specs

1. `phase-1-2-token-efficiency-foundation`
2. `phase-4-token-efficiency-rag-evaluation`
