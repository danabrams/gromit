---
id: prompt-context-budget
source_ideas: []
created: 2026-02-17
epic: token-efficiency-program
---

# Prompt Context Budget

## Specification

**Principle: prompts should be as small and specific to the task at hand as possible.**

Every LLM invocation assembles a prompt from templates, rules, learnings, specs, and project docs. A typical build prompt reaches 23,000-40,000 characters; retro prompts reach 50,000-80,000. Large prompts slow inference, waste tokens on low-signal context, and dilute the LLM's attention on the actual task.

The existing ATDD prompt context budget spec (atdd-prompt-context-budget) addresses acceptance-test prompts only. This spec reduces baseline context size and adds a dynamic budget mechanism covering all remaining phases: build, review, retro, TDD/ATDD build, and refactor.

### Current context breakdown (build phase)

| Component | Chars | Signal |
|-----------|-------|--------|
| RULES.md | ~13,655 | High (quality floor) |
| CLAUDE.md | ~5,341 | Low (generic orientation, LLM has codebase) |
| Confirmed learnings | ~2,000-5,000 | Medium (failure prevention) |
| Recent learnings | ~500-1,500 | Low (transient) |
| Spec file | 2,000-9,000 | High (task-specific) |
| Template + bead data | ~3,000-8,000 | Critical |

## Goals

1. Minimize prompt size — include only what's specific to the current task.
2. Reduce baseline by trimming source files (CLAUDE.md, RULES.md).
3. Add a dynamic budget mechanism that trims remaining context when over budget.
4. Make dynamic trimming deterministic, testable, and observable.
5. Keep essential context (rules, spec, bead identity) intact.

## Non-Goals

1. Changing ATDD acceptance prompt behavior (covered by atdd-prompt-context-budget spec).
2. LLM-based summarization or compression.

## Design

### 0) Trim source files

Reduce baseline context before any dynamic mechanism.

**CLAUDE.md (5,341 → ~1,200 chars):** Strip to Architecture + Key Principles. Remove Quick Start, Project Structure, Development Commands, Capturing Ideas, bd Integration, Model Selection, Configuration, and Keeping Docs Current — these are orientation content the LLM doesn't need during builds. Remove Bead Sizing + Grouping Rules (duplicated in RULES.md lines 46, 48, 50).

**RULES.md (13,655 → ~9,000-10,000 chars):** Tighten verbose rules:
- Line 19 (router model selection, ~650 chars): condense to essential pattern
- Line 48 (decomposition patterns, ~800 chars): condense splitting recipes
- Line 45 (config field patterns, ~450 chars): shorten
- Line 18 (prompt template patterns, ~400 chars): shorten
- Lines 46-50 (bead sizing, ~1,200 chars combined): consolidate overlapping rules, remove duplication with CLAUDE.md

Target: baseline build prompt drops from ~26K to ~18K chars, fitting a 25K budget without dynamic trimming on simple builds.

### 1) Config

Add a `prompt.budget` block in `internal/config/config.go`:

```yaml
prompt:
  budget:
    max_chars: 20000
    learning_cap_chars: 2000
```

Two fields. `max_chars` applies to all phases uniformly — set aggressively low to enforce minimal context. `learning_cap_chars` caps confirmed learnings before budget enforcement. `SetDefaults()` applies sensible defaults when omitted. After source file trimming, a 20K budget fits simple builds (~18K baseline) and forces trimming only when specs or retry context push over.

### 2) Unified context shaper

Add a single function in `internal/prompt/`:

```go
func ShapeContextForBudget(ctx *Context, cfg PromptBudgetConfig) (*Context, ShapeReport)
```

Measures total char count of context fields. If under budget, returns the context unchanged. If over, applies deterministic trim order until under budget.

Returns a `ShapeReport` with before/after sizes and trim actions taken.

### 3) Deterministic trim order

When over budget, trim in this fixed order:

1. **Drop recent learnings** (~0.5-1.5K freed, lowest signal)
2. **Drop CLAUDE.md** (~5.3K freed, generic orientation)
3. **Cap confirmed learnings** to `learning_cap_chars` (~1-3.5K freed)
4. **Drop confirmed learnings entirely**
5. **Phase-filter rules** to current-phase sections only (~1-3K freed)
6. **Truncate spec** with head/tail + `[...truncated...]` marker (last resort)

Never fully drop rules or spec. If all steps fire and context remains over budget, log a warning and proceed.

Bead identity (ID, title, description) is never trimmed.

### 4) Integration points

Each `RenderXxx` method that uses full `Context` calls `ShapeContextForBudget` before template expansion:

- `RenderBuild`, `RenderATDDBuild`, `RenderTDDBuild`
- `RenderRefactor`
- `RenderReview`, `RenderThoroughReview`
- Retro rendering

Methods with minimal context (`RenderAnalyze`, `RenderValidate`, `RenderScope`, `RenderDecompose`, `RenderLearn`, `RenderPrecheck`) skip shaping.

Existing methodology shapers (ShapeRedPhaseContext, etc.) run first. Budget shaping runs second. They compose: methodology removes sections, budget trims what remains.

### 5) Observability

One log line per shaped invocation, only when trimming fires:

```
Prompt budget: 34,891 -> 28,432 chars (trimmed: recent_learnings, claude_md)
```

`ShapeReport` captures `before_chars`, `after_chars`, `trim_actions []string`, and per-section sizes.

## Acceptance Criteria

1. CLAUDE.md contains only Architecture and Key Principles sections (~1,200 chars).
2. RULES.md is under 10,000 chars with no content duplication from CLAUDE.md.
3. Config loads without errors when budget fields are omitted (defaults apply).
4. `ShapeContextForBudget` is deterministic for identical inputs.
5. Trim steps fire in the specified priority order.
6. Bead identity fields are never trimmed.
7. Under-budget contexts pass through unchanged.
8. Over-budget contexts log trim actions taken.
9. Rules and spec are never fully dropped.

## Implementation Notes

- Phase 0 touchpoints:
  - `CLAUDE.md` (trim to Architecture + Key Principles)
  - `.gromit/RULES.md` (tighten verbose rules, remove duplication)
  - Tests referencing CLAUDE.md content or RULES.md line counts may need updating
- Phase 1-5 touchpoints:
  - `internal/prompt/prompt.go` (shaper function + ShapeReport type)
  - `internal/prompt/prompt.go` (RenderXxx methods call shaper)
  - `internal/config/config.go` (PromptBudgetConfig + defaults)
  - `gromit.yaml` (document new fields)
- Reuse `filterRulesByPhase()` for trim step 5.
- The ATDD acceptance phase remains governed by its own spec's config.
