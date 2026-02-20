---
created: 2026-02-20T00:00:00Z
decomposed: true
decomposed_at: "2026-02-20T12:53:35Z"
id: rules-phase-budgets
source_spec: rules-phase-budgets
---

# Per-Phase RULES.md Budget Tests Implementation Plan

**Goal:** Add a table-driven test enforcing per-phase character budgets on RULES.md, replacing the deleted global 10k char limit.

**Architecture:** Single test file loading the real RULES.md through the existing `Renderer.LoadRulesForPhase()` API, asserting six phase-specific budgets.

**Tech Stack:** Go, standard testing package

**Spec:** `.gromit/specs/rules-phase-budgets.md`

---

## Architecture

The RULES.md section re-tagging (splitting `## Process` into `## Build Process`, `## Decomposition`, `## Retro Formatting`) and all four accuracy fixes are already applied. The only remaining work is the budget test.

A new test file `internal/prompt/rules_phase_budget_test.go` creates a `Renderer` pointing at the real `.gromit/RULES.md` (via relative path `../../.gromit`), calls `LoadRulesForPhase()` for each phase, and asserts `len(result) <= budget`.

## Test Strategy

- **Table-driven**: One subtest per phase, six rows
- **Real file**: Loads actual `.gromit/RULES.md` (not fixture data) to catch real budget violations
- **Separate file**: Keeps real-file budget test cleanly separated from fixture-based unit tests in `load_rules_for_phase_test.go`
- **Failure diagnostics**: Reports actual size vs budget on failure

## Implementation Tasks

### Task 1: Add TestRulesPhaseCharBudgets

**Files:**
- Create: `internal/prompt/rules_phase_budget_test.go`

**What to Do:**
Create table-driven test with six phase/budget pairs. Construct Renderer with real `.gromit/` dir. For each phase, call LoadRulesForPhase and assert len(result) <= maxChars.

**Acceptance Criteria:**
- Test loads the real `.gromit/RULES.md`
- All six phase budgets asserted (build:8000, review:6000, plan:2000, refine:2000, retro:2000, validate:2000)
- Test passes with current RULES.md content

**Dependencies:** None

---

## Notes

- The existing tests in `load_rules_for_phase_test.go` use `rulesWithAnnotations()` fixture data and still reference the old `## Process` section name. Those tests remain valid (they test filtering behavior with synthetic data). The new budget test is complementary, testing real content sizes.
- Budget values are generous: build is ~6500 chars currently with 8000 budget; review ~5500 with 6000 budget. Plan/refine/retro/validate are all well under 2000.
