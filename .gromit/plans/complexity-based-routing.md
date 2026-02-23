---
created: 2026-02-20T00:00:00Z
decomposed: true
decomposed_at: "2026-02-23T20:01:05Z"
id: complexity-based-routing
source_spec: complexity-based-routing
---

# Complexity-Based Routing Implementation Plan

**Goal:** Decouple model tier selection from priority by introducing a complexity heuristic that routes mechanically simple beads to haiku regardless of priority, saving cost without sacrificing quality.

**Architecture:** Add a 5-signal complexity heuristic to `SelectTier()` in the escalation package. Beads matching >= 2 low-complexity signals route to `TierLow` (haiku). Priority continues to control execution ordering only. A `complexity:high` label provides an operator escape hatch. `original_tier` / `actual_tier` tracking in iteration logs enables empirical validation.

**Tech Stack:** Go, existing bead/runner/logger packages

**Spec:** `.gromit/specs/complexity-based-routing.md`

---

## Architecture

### Complexity Heuristic in SelectTier()

`SelectTier()` in `internal/runner/escalation/tierselect.go` gains a complexity heuristic evaluating 5 signals on a bead. A bead matching **two or more** low-complexity signals routes to `TierLow` by default.

**Low-complexity signals:**
1. **Title pattern match** — mechanical-work patterns: "migrate X to Y", "wire X into Y", "add field", "add config", "delete", "document", "rename", "add t.Parallel", "add compile-time check"
2. **Test-only bead** — `IsTestOnlyBead()` returns true (existing function)
3. **TDD disabled** — bead carries `tdd:false` label
4. **Low file count** — `EstimatedFileCount(b) > 0 && <= 3`
5. **Leaf bead** — `DependentCount == nil || *DependentCount == 0`

**SelectTier() flow:**
1. Nil guard → TierMedium (existing)
2. `complexity:high` label check → fall through to `cfg.SelectTier(priority, labels)` (priority-based path)
3. `isLowComplexity()` → return `TierLow`
4. Existing test-only bead check (subsumed by heuristic for >= 2 signals, preserved for single-signal test-only beads with label overrides)
5. Fall through to `cfg.SelectTier(priority, labels)` (existing priority-based path)

### Helper Functions

Three new functions in `internal/bead/bead_helpers.go`:
- `IsLowComplexityTitle(title string) bool` — compiled regex for mechanical patterns
- `IsLeafBead(b *Bead) bool` — checks DependentCount
- `EstimatedFileCount(b *Bead) int` — returns len(ExpectedOutputs)

### Experiment Tracking

Two new fields on `IterationResult` and `IterationLog`:
- `OriginalTier` — tier from heuristic before execution
- `ActualTier` — tier that ultimately ran (after escalation if any)

Misclassification = `original_tier == "low" && actual_tier != "low"`.

### Priority Decoupling

After this change, `Priority` controls only execution ordering in the bead picker. `Models.P0`/`P1`/`P2` config fields are only consulted when the heuristic does not classify as low-complexity (< 2 signals).

---

## Test Strategy

### Unit Tests — Bead Helpers
- `IsLowComplexityTitle`: table-driven with all 8 patterns, mixed case, non-matching, partial matches
- `IsLeafBead`: nil/zero/positive DependentCount
- `EstimatedFileCount`: nil/empty/populated ExpectedOutputs

### Unit Tests — Tier Selection Heuristic
- `countLowComplexitySignals`: each signal independently
- `isLowComplexity`: threshold behavior (0, 1, 2, 3+ signals)
- `SelectTier`: complexity:high override, low-complexity routing, fallback to priority, interaction with test-only bead logic

### Unit Tests — Logging
- `OriginalTier` and `ActualTier` appear in iteration log output

### Key Test Cases
- 0 signals → priority-based tier (existing behavior)
- 1 signal → priority-based tier (threshold not met)
- 2 signals → TierLow
- `complexity:high` + 3 signals → medium+ (label overrides)
- P0 "rename FooBar to BazQux" leaf bead → TierLow
- P2 "refactor auth middleware" 5+ files → NOT TierLow
- Test-only + tdd:false → 2 signals → TierLow

---

## Implementation Tasks

### Task 1: Add bead complexity helper functions

**Files:**
- Modify: `internal/bead/bead_helpers.go`
- Modify: `internal/bead/bead_helpers_test.go`

**What to Do:**
Add three new exported helper functions following `IsTestOnlyBead()` pattern:
- `IsLowComplexityTitle(title string) bool` — compiled regex matching 8 mechanical-work patterns (case-insensitive)
- `IsLeafBead(b *Bead) bool` — true when `DependentCount == nil || *DependentCount == 0`
- `EstimatedFileCount(b *Bead) int` — `len(ExpectedOutputs)` or 0

Add table-driven tests for each function.

**Acceptance Criteria:**
- All 8 title patterns match case-insensitively
- Non-matching titles return false
- IsLeafBead handles nil, zero, positive DependentCount
- EstimatedFileCount handles nil, empty, populated ExpectedOutputs

**Dependencies:** None

### Task 2: Add complexity heuristic to SelectTier()

**Files:**
- Modify: `internal/runner/escalation/tierselect.go`
- Modify: `internal/runner/escalation/tierselect_test.go`

**What to Do:**
Add `countLowComplexitySignals(cfg *config.Config, b *bead.Bead) int` evaluating 5 signals. Add `isLowComplexity(cfg, b) bool` returning `count >= 2`. Update `SelectTier()` to: (1) check `complexity:high` label → priority path, (2) check `isLowComplexity()` → TierLow, (3) existing fallback.

The existing test-only bead special case is subsumed by the heuristic when >= 2 signals match. Keep the existing code path for single-signal test-only beads.

**Acceptance Criteria:**
- >= 2 signals routes to TierLow
- < 2 signals falls through to priority-based tier
- `complexity:high` label forces medium+ regardless of heuristic
- Each signal tested independently
- Threshold boundary (1 vs 2) tested

**Dependencies:** Task 1

### Task 3: Add OriginalTier/ActualTier to data model and logging

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/logging.go`
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/logging_test.go`

**What to Do:**
Add `OriginalTier` and `ActualTier` string fields to `IterationResult` and `IterationLog` (with `original_tier,omitempty` / `actual_tier,omitempty` json tags). In `setupBeadContext`, set `bc.Result.OriginalTier = tier`. After execution, set `bc.Result.ActualTier = bc.Tier`. Wire both through `writeIterationLog`.

**Acceptance Criteria:**
- OriginalTier records heuristic-selected tier before execution
- ActualTier records tier that ultimately ran
- Both fields in JSONL iteration log output
- Existing tests pass unchanged

**Dependencies:** Task 2

---

## Notes

- The escalation chain (haiku → sonnet → opus) is completely unchanged. It's the safety net for heuristic misclassification.
- The `complexity:high` label is checked by looking for it directly in `b.Labels`, not through `cfg.Models.Labels`. This is a routing decision, not a model-override label.
- Cost impact: at current volumes, even 30% misclassification rate is cost-positive. Each correct haiku routing saves $0.275+ vs sonnet, each misclassification wastes $0.025.
- The existing `hasComplexityLabelOverride` helper in `process.go` handles model-level label overrides via config. The `complexity:high` check in `SelectTier()` is a different concern — it's a routing override, not a model override.
