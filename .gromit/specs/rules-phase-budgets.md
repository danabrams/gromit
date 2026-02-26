---
id: rules-phase-budgets
source_ideas: []
created: 2026-02-20
epic: token-efficiency-program
---

# Per-Phase RULES.md Budget Tests and Section Re-Tagging

## Specification

RULES.md sections are tagged with `<!-- phases: X, Y -->` annotations so `filterRulesByPhase()` returns only relevant rules for each pipeline phase. The `## Process` section previously bundled build, decomposition, and retro rules under a single `phases: build` tag, sending ~1000 chars of irrelevant decomposition/retro guidance to the build phase.

This feature splits `## Process` into three phase-specific sections and adds a table-driven test that enforces per-phase character budgets, replacing the deleted `TestRULESMD_UnderTenThousandChars` global limit that broke whenever retro promoted new rules.

**Section re-tagging (already applied):**

| Old section | New section(s) | Phase tag |
|---|---|---|
| `## Process` (all build) | `## Build Process` | `phases: build` |
| | `## Decomposition` | `phases: plan` |
| | `## Retro Formatting` | `phases: retro` |

**Trimming and accuracy fixes (already applied):**
All sections were condensed to preserve constraints while reducing verbosity. Four accuracy errors were fixed:
1. `agents.ResolveByName` → `agent.Resolve()` (function didn't exist)
2. Removed false `final_verification_test.go` enforcement claim from acceptance budget rule
3. Architecture production file limit: 500 → 550 to match actual enforcing test
4. T5 softened from "Do not use" to "Prefer ... over" (violated by ~30 existing tests)

**Budget test (to be implemented):**
A table-driven test in `internal/prompt/` loads the real `.gromit/RULES.md`, calls `LoadRulesForPhase()` for each phase, and asserts per-phase character budgets:

| Phase | Budget (chars) |
|---|---|
| build | 8000 |
| review | 6000 |
| plan | 2000 |
| refine | 2000 |
| retro | 2000 |
| validate | 2000 |

The test uses `len()` on the filtered output string. Phases that return no matching sections (empty string) trivially pass.

## Acceptance Criteria

- `## Process` section no longer exists in RULES.md; replaced by `## Build Process`, `## Decomposition`, and `## Retro Formatting` with correct phase tags
- `LoadRulesForPhase("build")` returns Build Process content but not Decomposition or Retro Formatting content
- `LoadRulesForPhase("plan")` returns Decomposition and Terminology content
- `LoadRulesForPhase("retro")` returns Retro Formatting content
- Table-driven test `TestRulesPhaseCharBudgets` in `internal/prompt/` loads real RULES.md and asserts all six phase budgets
- All four accuracy fixes are present in RULES.md
- All existing `LoadRulesForPhase` tests continue to pass

## Decisions

1. **Per-phase budgets replace global limit** The global 10k char test was brittle because retro promotions could push the total over the limit, breaking unrelated beads. Per-phase budgets are more targeted — a retro-only rule addition only affects the retro budget, not build.

2. **Budget values are generous** Build at 8000 and review at 6000 leave headroom for future rule additions. The current build phase is ~6,500 chars after trimming. Plan/refine/retro/validate at 2000 each are conservative since these phases currently receive very few sections.

3. **Test loads real RULES.md** Rather than testing with synthetic fixture data, the budget test reads the actual RULES.md file. This ensures the test catches real budget violations from rule changes, matching the intent of the deleted global limit test.

4. **Red/green/refactor sub-phase splitting deferred** Further splitting the build phase into TDD sub-phases was considered but the savings (~1,500 chars per sub-phase) didn't justify the annotation complexity, since Code Style/Safety/Test Quality/Architecture apply to all build sub-phases equally.

5. **Architecture line limit updated to 550** The rule said "500 lines" but the enforcing test (`runner_split_final_verification_reclassified_test.go`) uses 550. Updated the rule to match reality rather than changing the test.

## Research & Context

### Current State

- `filterRulesByPhase()` in `internal/prompt/rules_filter.go` parses `<!-- phases: X, Y -->` annotations on `##` headers
- `LoadRulesForPhase()` in `internal/prompt/prompt.go` is the public API, caching RULES.md content
- Existing tests in `internal/prompt/load_rules_for_phase_test.go` use synthetic fixture data via `rulesWithAnnotations()` — the new budget test is the first to load the real file
- The deleted `TestRULESMD_UnderTenThousandChars` lived in `cmd/gromit/bead_sizing_docs_test.go` (removed in commit `777314b`)
- Phase constants `promptPhaseBuild` and `promptPhaseReview` exist in `prompt.go`; other phases are used as string literals
