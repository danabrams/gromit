# Cost-Optimized Routing Design

**Date:** 2026-02-18
**Status:** Approved

## Problem

Gromit currently selects model tier based on bead priority (P0->opus, P1->sonnet, P2->haiku). This means most beads run on expensive models even when they're narrow enough for cheap ones. Metrics show opus at $1.50/attempt with ~50% failure rate on broad beads, while haiku succeeds at 100% on well-scoped beads for $0.025/attempt. The cost gap is 60x per attempt.

The insight: with sufficient decomposition, implementation becomes mechanical pattern-following. The intelligence investment belongs in *planning and decomposition*, not in implementation.

## Evidence

From 345 iterations of Gromit metrics (Feb 13-18, 2026):

**Early runs (broad beads, opus):** ~50% success rate, $1.50 avg/attempt, 18min avg duration. When opus failed and haiku got the escalation, haiku went 5/5.

**Recent runs (narrow beads, mixed models):** 100% success rate over last 30 iterations, $0.45 avg/attempt, 6min avg duration. Codex and sonnet handling everything; opus barely needed.

The improvement came from better decomposition, not from bigger models.

## Design: Decompose-First Escalation

### Core Principle

Phase determines model, not priority:
- **Plan/decompose/review** -> sonnet (pay for intelligence where it's leverage)
- **Build (all beads)** -> haiku (cheap pattern-following)
- **Validate** -> haiku (cheap verification)

### Two-Pronged Decomposition

**Prong 1: Aggressive upfront decomposition**
- Push decomposition deeper by default, targeting single-concern beads
- One function, one method, one test, one config change per bead
- Depth limit increases from ~5 levels to 10
- Accept more beads as the trade-off for cheaper beads
- Sonnet does the decomposition (quality matters here)

**Prong 2: Runtime decompose-on-failure**
- When haiku fails a bead, decompose it further instead of escalating the model
- Sonnet does the mid-run decomposition (~$0.30, cheap insurance)
- Only escalate to a bigger model for truly atomic beads that can't be split

### Escalation Flow

```
bead fails at haiku
  -> retry haiku (up to max_retries, default 2)
  -> still failing? is this bead decomposable?
    -> YES: sonnet decomposes into sub-beads -> run sub-beads at haiku
    -> NO (atomic): escalate to sonnet
      -> sonnet fails? escalate to opus (absolute last resort)
```

### Atomicity Detection

A bead is "atomic" (can't be decomposed further) when ANY of:
- Targets a single function/method in a single file
- Already at max decomposition depth (configurable, default 10)
- Decomposer explicitly declares "cannot split further"

### Config Shape

```yaml
routing:
  strategy: cost_optimized     # vs "priority_based" (current default)
  cost_optimized:
    build_tier: low             # haiku for all implementation
    decompose_tier: medium      # sonnet for decomposition steps
    escalation_tier: medium     # sonnet for atomic bead failures
    max_decomposition_depth: 10
    max_retries_before_decompose: 2

decomposition:
  target: single_concern        # vs "narrow_scope" (current)
```

### Codebase Changes

| Area | Change |
|------|--------|
| `internal/runner/escalation/` | New `DecomposeFirstHandler` -- on failure, decompose before escalating |
| `internal/config/` | New `CostOptimized` routing config section |
| `internal/bead/` | `IsAtomic()` detection |
| `internal/runner/` | Mid-run decomposition capability |
| `SelectTier()` / `SelectModel()` | Respect `cost_optimized` strategy |
| Decompose prompt | New "single-concern" targeting mode |

### Expected Economics (per spec)

| Phase | Model | Cost |
|-------|-------|------|
| Plan | sonnet x1 | ~$0.30 |
| Decompose (deeper) | sonnet x1-2 | ~$0.60 |
| Build (~30 beads x ~1.3 attempts) | haiku x39 | ~$0.98 |
| Runtime re-decompose (~10% of beads) | sonnet x3 | ~$0.90 |
| Validate | haiku | ~$0.15 |
| **Total** | | **~$2.93/spec** |

vs. current ~$4.50/spec, widening as decomposition quality improves.

## Decisions

1. **Phase determines model, not priority.** Priority still affects decomposition depth and ordering, but all implementation beads start at the cheapest tier.

2. **Decomposition is the primary retry strategy.** Model escalation is the last resort, only for atomic beads. You can retry haiku 60 times for the cost of one opus attempt.

3. **Sonnet for decomposition, haiku for implementation.** Decomposition quality is leverage -- one bad decomposition wastes N haiku attempts. The $0.30 per decomposition call is cheap insurance.

4. **Aggressive upfront + runtime safety net.** Push decomposition deeper from the start AND add decompose-on-failure as a fallback. The two approaches compound.

5. **Single-concern targeting.** New decomposition target: one function, one method, one test per bead. Coarser than file-count limits but finer than current "natural implementation unit."
