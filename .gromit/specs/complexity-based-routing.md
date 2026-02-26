---
id: complexity-based-routing
created: 2026-02-20
epic: provider-ecosystem
---

# Complexity-Based Routing

## Specification

Gromit currently maps P0->opus, P1->sonnet, P2->haiku, conflating execution urgency (priority) with task difficulty (complexity). This leads to expensive models running trivial tasks just because they're urgent, and complex low-priority tasks getting under-powered models. A P0 bead titled "rename FooBar to BazQux" runs on opus ($1.50/attempt) when haiku ($0.025/attempt) would succeed first-try. Meanwhile a P2 bead titled "refactor auth middleware to support multi-tenant routing" gets haiku and fails repeatedly before escalation.

This spec introduces a complexity dimension that drives model tier selection independently of priority. Priority continues to control execution ordering in the bead picker. Complexity controls which model runs the bead.

### Complexity Heuristic in SelectTier()

`SelectTier()` in `internal/runner/escalation/tierselect.go` gains a complexity heuristic that evaluates multiple signals on the bead. A bead matching **two or more** low-complexity signals routes to haiku by default.

**Low-complexity signals:**

1. **Title pattern match.** Title matches one of these mechanical-work patterns (case-insensitive):
   - "migrate X to Y"
   - "wire X into Y"
   - "add field" / "add config"
   - "delete"
   - "document"
   - "rename"
   - "add t.Parallel"
   - "add compile-time check"

2. **Test-only bead.** `IsTestOnlyBead()` already returns true for "add tests for", "write tests for" patterns. This counts as one signal.

3. **TDD disabled.** Bead carries a `tdd:false` label, indicating no red-green-refactor ceremony. Without cross-phase reasoning requirements, the task is more mechanical.

4. **Low file count estimate.** Decomposition metadata indicates 3 or fewer files affected. When `ExpectedOutputs` or decomposition annotations list <= 3 files, this counts as a signal.

5. **Leaf bead.** The bead has no downstream dependents (`DependentCount == 0` or nil). Leaf beads carry lower risk because nothing blocks on their specific implementation approach.

### TDD Beads and Haiku

TDD beads CAN be haiku-safe when the work is mechanical (signature changes, field wiring, adding a config key). Gromit runs each TDD phase (red/green/refactor) as a fresh context invocation, so the model does not need cross-phase reasoning within a single invocation. The heuristic treats TDD beads the same as any other bead -- if they match >= 2 low-complexity signals, they route to haiku.

### Label Override

A `complexity:high` label on a bead forces the tier to medium or above, regardless of what the heuristic concludes. This is the operator's escape hatch for beads that look mechanical by title but require nuanced reasoning.

The label check happens before the heuristic, short-circuiting to `cfg.SelectTier(b.Priority, b.Labels)` (the existing priority-based path which maps P0->high, P1->medium).

### Priority Decoupling

After this change, the `Priority` field on a bead controls only:
- **Execution order** in the bead picker (P0 beads run before P1, P1 before P2)
- **Escalation ceiling** (unchanged -- existing escalation chain logic is priority-agnostic)

Priority does NOT control model tier selection. The `Models.P0`, `Models.P1`, `Models.P2` config fields remain for backward compatibility but are only consulted when the complexity heuristic does not produce a low-complexity classification (i.e., the bead matched fewer than 2 signals).

### Experiment Tracking

To enable A/B analysis of the heuristic's effectiveness, the iteration metrics log gains two fields:

- `original_tier`: The tier selected by the complexity heuristic (before any escalation)
- `actual_tier`: The tier that ultimately executed the bead (after escalation, if any)

These fields are added to `IterationLog` and the iteration metrics JSONL. When `original_tier == "low"` and `actual_tier == "medium"` or `"high"`, that indicates a heuristic misclassification that required escalation. Tracking the ratio of misclassifications over time validates whether the heuristic thresholds are well-tuned.

### Escalation Safety Net

The existing escalation chain (haiku -> sonnet -> opus) is unchanged. When the heuristic routes a bead to haiku and haiku fails after retries, the standard escalation fires. This is the primary mitigation for heuristic misclassification -- the cost of a wrong haiku attempt is $0.025, while the cost of a wrong opus attempt is $1.50.

### Codebase Changes

1. **`internal/runner/escalation/tierselect.go`** -- `SelectTier()` gains the complexity heuristic. New helper `isLowComplexity(cfg, b) bool` evaluates the five signals and returns true when >= 2 match. New helper `countLowComplexitySignals(cfg, b) int` for testability.

2. **`internal/bead/bead_helpers.go`** -- New `IsLowComplexityTitle(title string) bool` function with the mechanical-work title patterns. Follows the same pattern as `IsTestOnlyBead()` and `IsProactiveDecompositionCandidate()`.

3. **`internal/bead/bead_helpers.go`** -- New `IsLeafBead(b *Bead) bool` function checking `DependentCount`.

4. **`internal/bead/bead_helpers.go`** -- New `EstimatedFileCount(b *Bead) int` function deriving file count from `ExpectedOutputs` length or returning 0 when unknown.

5. **`internal/logger/`** -- Add `OriginalTier` and `ActualTier` fields to `IterationLog` and iteration metric structs.

6. **`internal/runner/`** -- Runner sets `OriginalTier` from `SelectTier()` at bead start and `ActualTier` from the tier that ultimately succeeded (or the last tier attempted on failure).

## Acceptance Criteria

- `SelectTier()` uses complexity heuristic instead of priority for model selection
- Priority field controls only execution ordering in the bead picker
- Beads matching >= 2 low-complexity signals route to haiku (low tier) by default
- `complexity:high` label overrides heuristic to force medium+ tier
- Escalation chain still fires on haiku failure (existing behavior preserved)
- Experiment tracking: `original_tier` vs `actual_tier` logged in iteration metrics for A/B analysis
- Beads matching fewer than 2 signals fall back to existing priority-based tier selection
- Unit tests cover each signal independently and the >= 2 threshold logic

## Decisions

1. **Complexity over priority for tier selection.** The fundamental insight is that task difficulty and task urgency are orthogonal. A P0 rename is easier than a P2 refactor. The model tier should match the difficulty, not the urgency. Priority retains its role in execution ordering, where urgency genuinely matters.

2. **Heuristic with label escape hatch, not LLM classification.** An LLM call to classify complexity would be accurate but adds latency and cost before every bead. A fast heuristic with a `complexity:high` label override gives the operator control without per-bead overhead. The escalation chain handles the remaining misclassifications.

3. **Threshold of 2 signals, not 1.** Requiring two or more low-complexity signals reduces false positives. A single matching title pattern is not enough -- the bead must also be a leaf, or have few files, or skip TDD. This conservative threshold means some haiku-safe beads still run on sonnet, but avoids the more expensive mistake of sending complex beads to haiku.

4. **TDD beads are not automatically complex.** Fresh context per phase means haiku does not need to hold state across red/green/refactor. The per-phase invocation model makes TDD beads no harder than non-TDD beads of equivalent scope. Only beads requiring nuanced test design (signaled by `complexity:high`) skip haiku.

5. **Additive to existing routing, not replacing.** When the heuristic does not classify a bead as low-complexity, the existing priority-based mapping applies. This change is backward-compatible: projects that never hit the >= 2 signal threshold see no behavior change.

6. **`original_tier` / `actual_tier` tracking for empirical validation.** Rather than assuming the heuristic is correct, logging both tiers enables measuring misclassification rate over time. If the rate is high, the threshold or signals can be adjusted with data.

## Risks

1. **Heuristic misclassification.** The heuristic may route complex beads to haiku, causing failures. **Mitigation:** The escalation chain (haiku -> sonnet -> opus) fires on failure. A misclassified bead costs one extra haiku attempt ($0.025) before escalating. The `original_tier` / `actual_tier` tracking identifies persistent misclassification patterns.

2. **Title-based heuristics are fragile.** If bead naming conventions drift away from the recognized patterns, the title signal loses effectiveness. **Mitigation:** The `complexity:high` label override lets operators force correct routing immediately. The title patterns can be updated as conventions evolve. The >= 2 threshold means title alone is never sufficient.

3. **Mechanical TDD beads on haiku may produce weak tests.** Haiku generates syntactically correct but potentially shallow tests for mechanical changes. **Mitigation:** The review phase catches weak test quality. TDD beads that consistently produce inadequate tests can be labeled `complexity:high`.

## Research & Context

### Current State

- `internal/runner/escalation/tierselect.go` -- `SelectTier()` delegates to `cfg.SelectTier()` which maps priority to tier via `Models.P0`/`P1`/`P2` config. Test-only beads are the only complexity-aware exception today.
- `internal/config/config_accessors.go:82-110` -- `Config.SelectTier()` checks label overrides first, then falls back to priority-based selection.
- `internal/bead/bead_helpers.go` -- `IsTestOnlyBead()`, `IsProactiveDecompositionCandidate()`, `IsMethodologyActive()` provide the pattern for title-based and label-based classification.
- `internal/bead/bead.go:17-37` -- `Bead` struct includes `DependentCount`, `DependencyCount`, `ExpectedOutputs`, and `Labels` -- all inputs to the heuristic.

### Cost Model

From process_trend.json and production data:
- Haiku: ~$0.025/attempt. Sonnet: ~$0.30/attempt. Opus: ~$1.50/attempt.
- A misclassified haiku attempt that escalates costs $0.025 + $0.30 = $0.325 total.
- A correctly classified haiku attempt saves $0.275 vs sonnet or $1.475 vs opus.
- At current volumes, even a 30% misclassification rate is cost-positive: 70% of beads save $0.275+ each, 30% waste $0.025 each.

### Related Specs

- `cost-optimized-routing` -- Introduced decompose-before-escalate strategy with all beads starting at haiku. This spec is complementary: it applies complexity analysis within the existing priority-based routing rather than replacing it.
- `model-success-tracking` -- Tracks per-model success rates. The `original_tier` / `actual_tier` data from this spec feeds directly into that tracking for complexity-aware analysis.
- `decompose-complexity-estimation` -- Estimates bead complexity during decomposition. Outputs from that estimation (file count, scope annotations) can serve as inputs to this spec's heuristic.
