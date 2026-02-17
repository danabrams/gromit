---
id: prompt-context-budget
source_spec: prompt-context-budget
created: 2026-02-17
decomposed: true
---

# Prompt Context Budget Implementation Plan

**Goal:** Reduce prompt sizes across all phases by trimming source files and adding a dynamic budget mechanism that deterministically trims low-signal context when over budget.

**Architecture:** Two-layer approach — static source file trimming (CLAUDE.md/RULES.md) cuts baseline by ~8K chars, then a dynamic budget shaper enforced inside Renderer methods deterministically trims remaining context using a fixed priority order. Methodology shapers compose naturally (they run first, budget shaper trims what remains).

**Tech Stack:** Go, text/template, YAML config

**Spec:** `.gromit/specs/prompt-context-budget.md`

---

## Architecture

**Key Components:**

1. **PromptBudgetConfig** (`internal/config/config.go`): Two-field struct (`MaxChars int`, `LearningCapChars int`) with defaults (20000, 2000) in `SetDefaults()`.

2. **ShapeContextForBudget** (`internal/prompt/budget.go`): Standalone function taking `*Context` + `PromptBudgetConfig` + phase string. Measures total char count, applies deterministic trim order if over budget, returns shaped `*Context` + `ShapeReport`. Separate variants for `ReviewContext` and `ThoroughReviewContext`.

3. **Automatic Renderer integration**: Qualifying `RenderXxx` methods call `ShapeContextForBudget` internally before template expansion when budget config is set. No interface changes. `ShapeReport` logged to stderr when trimming fires.

**Integration Points:**
- Renderer stores `PromptBudgetConfig` via `SetBudgetConfig()`, called from `NewRunner`
- `RenderBuild`, `RenderATDDBuild`, `RenderTDDBuild`, `RenderRefactor`, `RenderAcceptanceTests` — shape `*Context`
- `RenderReview`, `RenderThoroughReview` — shape their respective context types
- Retro `renderPrompt` — shape rules/learnings before template expansion
- Methodology phase shapers run first; budget shaping runs second

**Deterministic Trim Order (when over budget):**
1. Drop recent learnings (~0.5-1.5K freed)
2. Drop CLAUDE.md (~1.2K freed after static trim)
3. Cap confirmed learnings to `LearningCapChars` (~1-3.5K freed)
4. Drop confirmed learnings entirely
5. Phase-filter rules to current-phase sections only (reuses existing `filterRulesByPhase`)
6. Truncate spec with head/tail + `[...truncated...]` marker (last resort)

Never fully drop rules or spec. Bead identity (ID, title, description) never trimmed.

**Composition with methodology shapers:**
```
BuildContext()  →  ShapeRedPhaseContext()  →  RenderATDDBuild()
                   (clears ClaudeMD,           ↓
                    learnings)            ShapeContextForBudget()
                                              (skips already-empty
                                               fields, trims remaining)
                                               ↓
                                          template.Execute()
```

## Test Strategy

**Unit Tests** (`internal/prompt/budget_test.go`): Table-driven tests for `ShapeContextForBudget` covering each trim step individually and in combination. Tests for review context shapers and retro shaping.

**Config Tests** (`internal/config/config_test.go`): Verify `PromptBudgetConfig` defaults, YAML deserialization, and zero-value sentinel.

**Integration Tests** (existing files): Verify render methods produce output without errors when budget config is set.

**Key Test Cases:**
- Under-budget context passes through unchanged
- Each trim step fires alone when it's the one that brings context under budget
- All steps fire and context still over budget → warning logged, context returned
- Bead identity preserved in all scenarios
- Rules and spec never fully dropped
- Composition with methodology shapers (already-cleared fields skipped)
- Config defaults applied when omitted
- ShapeReport correctly records before/after sizes and trim actions

**Mocking Strategy:**
- ShapeContextForBudget tested directly (pure function, no mocks needed)
- Config tests use YAML parsing
- Render integration uses temp dirs with template files (existing pattern)

## Implementation Tasks

### Task 1: Trim CLAUDE.md to Architecture + Key Principles

**Files:**
- Modify: `CLAUDE.md`
- Modify: `cmd/gromit/bead_sizing_docs_test.go`

**What to Do:**
Strip CLAUDE.md to only the header, Architecture section, and Key Principles section. Remove: Quick Start, Project Structure, Development Commands, Bead Sizing, Grouping Rules, Capturing Ideas, bd Integration, Model Selection, Configuration, and Keeping Docs Current. These are orientation content the LLM doesn't need during builds; Bead Sizing and Grouping Rules are duplicated in RULES.md.

Update `bead_sizing_docs_test.go`: remove `TestCLAUDEMD_BeadSizingSection` and `TestCLAUDEMD_GroupingRulesSubsection` (these validate content deliberately removed from CLAUDE.md — the same content is still enforced in RULES.md, SKILL.md, and PROMPT_decompose.md by the remaining tests). Update `TestAllDocuments_ConsistentFileLimits` and `TestAllDocuments_ConsistentGroupingRules` to skip the CLAUDE.md check.

**Acceptance Criteria:**
- CLAUDE.md contains only header, Architecture, and Key Principles sections (~1,200 chars)
- All tests in `bead_sizing_docs_test.go` pass
- `go test ./cmd/gromit/...` passes

**Dependencies:** None

### Task 2: Tighten RULES.md verbose rules

**Files:**
- Modify: `.gromit/RULES.md`
- Modify: `internal/prompt/load_rules_for_phase_test.go` (if assertions reference condensed text)

**What to Do:**
Condense verbose rules to reduce total size from ~13,655 to ~9,500 chars:
- Rule at line 19 (router model selection, ~947 chars): condense to essential pattern — tier vs model distinction, escalation package ownership. Target ~400 chars.
- Rule at line 48 (decomposition patterns, ~1,015 chars): condense splitting recipes into a concise list. Target ~500 chars.
- Rule at line 45 (config field patterns, ~738 chars): shorten to key requirements (setDefaults, NormalizeNilFields, omitempty). Target ~400 chars.
- Rule at line 18 (prompt template patterns, ~575 chars): shorten to essential pattern reference. Target ~300 chars.
- Rules at lines 46-50 (bead sizing in Process, ~1,200 chars combined): consolidate overlapping rules, remove duplication with remaining CLAUDE.md content. Target ~600 chars.

Remove the "Bead sizing rules are enforced across four documentation files" rule (line 56) — this cross-reference rule becomes stale after CLAUDE.md trimming.

Update any tests that assert on specific condensed text (check `load_rules_for_phase_test.go` line 123 for "Beads that touch 6+ files should be split" — may need to match condensed wording).

**Acceptance Criteria:**
- RULES.md under 10,000 chars with no content duplication from CLAUDE.md
- All existing test suites pass (`go test ./...`)
- Phase annotations preserved on all section headers

**Dependencies:** Task 1 (CLAUDE.md trimmed first so we know what duplication to remove)

### Task 3: Add PromptBudgetConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add `PromptBudgetConfig` struct with `MaxChars int` (yaml: `max_chars`) and `LearningCapChars int` (yaml: `learning_cap_chars`). Embed in `Config` as `Prompt PromptConfig` with `Budget PromptBudgetConfig` nested. In `SetDefaults()`, set `MaxChars` to 20000 and `LearningCapChars` to 2000 when zero. Add commented `prompt.budget` section to gromit.yaml after the `learnings:` section (line 205), following existing comment style.

**Acceptance Criteria:**
- Config loads without errors when budget fields are omitted (defaults 20000/2000 apply)
- YAML with explicit budget values overrides defaults
- `go test ./internal/config/...` passes

**Dependencies:** None

### Task 4: Implement ShapeContextForBudget with deterministic trim order

**Files:**
- Create: `internal/prompt/budget.go`
- Create: `internal/prompt/budget_test.go`

**What to Do:**
Implement the core budget shaper:

```go
type ShapeReport struct {
    BeforeChars   int
    AfterChars    int
    TrimActions   []string
    SectionSizes  map[string]int  // per-field sizes after shaping
}

func ShapeContextForBudget(ctx *Context, maxChars int, learningCapChars int, phase string) (*Context, *ShapeReport)
```

The function clones the context (reuse `cloneMethodologyContext`), measures total char count of context fields (ClaudeMD + Rules + confirmed learnings text + recent learnings text + Spec + bead description/title + FailureContext + PrevFailure). If under budget, returns unchanged. If over, applies trim steps in order until under budget:
1. Drop RecentLearnings → report "recent_learnings"
2. Drop ClaudeMD → report "claude_md"
3. Cap ConfirmedLearnings to `learningCapChars` (by char count of formatted content, most recent first) → report "cap_confirmed_learnings"
4. Drop ConfirmedLearnings entirely → report "confirmed_learnings"
5. Phase-filter Rules via `filterRulesByPhase(rules, phase)` → report "phase_filter_rules"
6. Truncate Spec: keep first 500 chars + `\n[...truncated...]\n` + last 500 chars → report "truncate_spec"

Add `ShapeReviewContextForBudget` and `ShapeThoroughReviewContextForBudget` with analogous logic for those context types (simpler — no learnings fields, just ClaudeMD/Rules/Spec/Diff).

Comprehensive table-driven tests covering: under-budget passthrough, each trim step individually, all steps firing, bead identity preservation, rules/spec never fully dropped, ShapeReport correctness.

**Acceptance Criteria:**
- ShapeContextForBudget is deterministic for identical inputs
- Trim steps fire in the specified priority order
- Bead identity fields (ID, title, description) are never trimmed
- Under-budget contexts pass through unchanged
- Rules and spec are never fully dropped

**Dependencies:** Task 3 (uses PromptBudgetConfig field values, though the function takes primitive params)

**Notes:** The function takes primitive params (maxChars, learningCapChars, phase) rather than the config struct directly, keeping it testable without config dependency. The Renderer will unpack config values when calling it.

### Task 5: Integrate budget shaping into Renderer and Runner

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/interfaces.go` (if mock needs SetBudgetConfig)

**What to Do:**
Add to Renderer: `budgetMaxChars int`, `budgetLearningCapChars int` fields. Add `SetBudgetConfig(maxChars, learningCapChars int)` setter method.

In each qualifying render method, before calling `r.render(templateName, ctx)`, call `ShapeContextForBudget` if `r.budgetMaxChars > 0`. Map render method to phase: RenderBuild/RenderATDDBuild/RenderTDDBuild → "build", RenderRefactor → "build", RenderAcceptanceTests → "build", RenderReview → "review", RenderThoroughReview → "review". If ShapeReport shows trimming, log to stderr: `fmt.Fprintf(os.Stderr, "Prompt budget: %d -> %d chars (trimmed: %s)\n", ...)`.

In `internal/runner/runner.go` `NewRunner` (or wherever Renderer is configured), call `r.renderer.SetBudgetConfig(cfg.Prompt.Budget.MaxChars, cfg.Prompt.Budget.LearningCapChars)`.

Update mock implementations in `interfaces.go` if `SetBudgetConfig` is added to the `PromptRenderer` interface (likely not needed — setter can stay on concrete Renderer type, called from NewRunner which has the concrete type).

**Acceptance Criteria:**
- Over-budget contexts log trim actions taken
- Under-budget contexts produce identical output to pre-change behavior
- All existing tests pass (`go test ./...`)

**Dependencies:** Task 4

### Task 6: Add budget shaping for retro rendering

**Files:**
- Modify: `internal/prompt/budget.go`
- Modify: `internal/retro/retro.go`
- Modify: `internal/prompt/budget_test.go`

**What to Do:**
Add `ShapeRetroForBudget(rules, learnings string, maxChars int) (string, string, *ShapeReport)` to budget.go. This shapes the two text fields used by retro's `TemplateContext`: if combined size exceeds maxChars, trim learnings first (cap to half budget, then drop entirely), then truncate rules with `[...truncated...]` marker.

In retro.go `renderPrompt`, call `ShapeRetroForBudget` before building `TemplateContext` when budget config is available. Pass budget config into Retro struct (add field + constructor param, plumbed from runner or CLI).

Add tests for `ShapeRetroForBudget` in budget_test.go.

**Acceptance Criteria:**
- Retro prompts are shaped when over budget
- Retro rendering still works correctly when budget config is zero/unset (no shaping)
- Learnings trimmed before rules in retro context

**Dependencies:** Task 4

---

## Notes

- Tasks 1-2 (source file trimming) and Task 3 (config) are independent and can be parallelized.
- Tasks 5-6 are independent of each other and can be parallelized after Task 4.
- The `bead_sizing_docs_test.go` file is the most brittle test — it asserts specific content strings exist in CLAUDE.md. Task 1 must update these tests.
- The existing `filterRulesByPhase` function in prompt.go is reused by trim step 5 — no duplication needed.
- The existing `cloneMethodologyContext` function can be reused for context cloning in the budget shaper.
- Review context types (`ReviewContext`, `ThoroughReviewContext`) don't have learnings fields, so their shapers are simpler (skip trim steps 1, 3, 4).
- The ATDD acceptance prompt budget (covered by separate spec) is not affected — its own config/shaping remains independent.
