---
id: cost-optimized-routing
created: 2026-02-18
---

# Cost-Optimized Routing

## Specification

Gromit's model selection is priority-based: P0->opus, P1->sonnet, P2->haiku. This overallocates expensive models to beads that are narrow enough for cheap ones. Metrics from 345 iterations show haiku succeeding at 100% on well-scoped beads while opus fails at 50% on broad ones. The cost difference is 60x per token.

This spec introduces a `cost_optimized` routing strategy that inverts the model selection logic: phase determines model (plan->sonnet, build->haiku, validate->haiku) and decomposition replaces model escalation as the primary retry strategy.

### Two-Pronged Decomposition

**Prong 1: Aggressive upfront decomposition.** The existing decompose step targets narrower beads by default. Instead of "natural implementation unit" (1-3 files), target "single concern" -- one function, one method, one test, one config change per bead. Max decomposition depth increases from ~5 to 10. Sonnet performs decomposition.

**Prong 2: Runtime decompose-on-failure.** When a bead fails at haiku after max retries, Gromit auto-decomposes that bead into sub-beads (using sonnet) and retries the sub-beads at haiku. Model escalation (haiku->sonnet->opus) only occurs for atomic beads that cannot be split further.

### Escalation Flow

When `routing.strategy: cost_optimized`:

1. Bead assigned to haiku regardless of priority
2. On failure, retry haiku up to `max_retries_before_decompose` (default 2)
3. If still failing, check atomicity:
   - **Decomposable**: sonnet decomposes into sub-beads, sub-beads run at haiku
   - **Atomic**: escalate to sonnet, then opus as last resort
4. A bead is atomic if: single function/method in single file, OR at max decomposition depth, OR decomposer declares it unsplittable

### Config Changes

New fields in `routing` section:

```yaml
routing:
  strategy: cost_optimized
  cost_optimized:
    build_tier: low
    decompose_tier: medium
    escalation_tier: medium
    max_decomposition_depth: 10
    max_retries_before_decompose: 2
```

`decomposition.target` gains a `single_concern` option alongside the existing `narrow_scope`.

### Codebase Changes

1. **`internal/runner/escalation/`** -- New `DecomposeFirstHandler` that implements the decompose-before-escalate flow. Wraps existing `Handler` for the atomic-bead fallback path.

2. **`internal/config/`** -- `CostOptimized` struct under `Routing`. Defaults: `build_tier: low`, `decompose_tier: medium`, `escalation_tier: medium`, `max_decomposition_depth: 10`, `max_retries_before_decompose: 2`.

3. **`internal/bead/`** -- `IsAtomic(bead, depth)` function. Returns true when the bead targets a single function/method in a single file, OR depth >= max, OR the bead carries an `atomic:true` label.

4. **`SelectTier()` / `SelectModel()`** -- When strategy is `cost_optimized`, `SelectTier()` returns `low` for all build phases regardless of priority. Planning, decomposition, and review phases use `decompose_tier`.

5. **`internal/runner/`** -- Mid-run decomposition: when `DecomposeFirstHandler` decides to decompose, it invokes the decompose pipeline for the failed bead, creates sub-beads, and enqueues them for processing in the current run.

6. **Decompose prompt** -- Add a `single_concern` targeting mode that instructs the decomposer to produce finer-grained beads: one function, one method, one test per bead. The existing grouping rules (never-split patterns from `decomposition-granularity` spec) still apply.

## Acceptance Criteria

- `routing.strategy: cost_optimized` config is parsed and validated; `priority_based` remains the default
- When `cost_optimized`, all implementation beads start at the lowest tier regardless of bead priority
- On bead failure after max retries, `DecomposeFirstHandler` auto-decomposes the bead (via sonnet) and enqueues sub-beads at haiku
- Atomic beads (single function/file, max depth, or labeled) skip decomposition and escalate to `escalation_tier`
- `decomposition.target: single_concern` produces beads scoped to one function/method/test
- Max decomposition depth of 10 is respected; depth-limited beads are treated as atomic
- Existing `priority_based` routing behavior is unchanged when that strategy is selected
- Cost-per-spec metric is tracked and visible in `gromit stats`

## Decisions

1. **Phase over priority.** The model tier serves the phase (plan/build/validate), not the bead's importance. Priority still affects ordering and decomposition depth, but not the model.

2. **Decompose before escalate.** Model escalation is the last resort. The retry chain is: haiku -> haiku retry -> decompose -> haiku on sub-beads -> sonnet (atomic only) -> opus (last resort).

3. **Sonnet decomposes, haiku implements.** Decomposition quality is leverage. One bad decomposition wastes N haiku attempts. Paying $0.30 for sonnet decomposition saves $1.50+ in failed opus attempts.

4. **Additive, not replacing.** This is a new routing strategy alongside the existing one. Teams can opt in via config.

## Research & Context

### Metrics Evidence

From process_trend.json (344 iterations):
- Latest 30-window: 100% success, 100% first-pass, $0.45 avg cost
- Historical mean: 70% success, 24% first-pass, $0.50 avg cost
- Early opus runs: 50% success at $1.50/attempt; haiku on escalation: 100% success at $0.025/attempt

### Cost Model

Haiku: ~$0.025/attempt. Sonnet: ~$0.30/attempt. Opus: ~$1.50/attempt.
60 haiku retries = 1 opus attempt. Retry rate is irrelevant at these ratios when optimizing for cost.

### Related Specs

- `decomposition-granularity` -- Established "natural implementation unit" sizing. This spec pushes further to "single concern."
- `token-budget-guardrail` -- Prevents per-bead timeout from token churn. Complementary: smaller beads naturally use fewer tokens.
- `layered-failure-triage` -- Classifies failures before LLM analysis. Complementary: only code failures trigger the decompose-or-escalate decision.
