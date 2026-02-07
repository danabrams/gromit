---
id: skip-decompose-if-done
source_ideas: []
created: 2026-02-07
---

# Skip Decompose Prompt If Plan Already Decomposed

## Specification

When the `gromit plan` skill runs interactively, Claude sometimes decomposes the plan during the planning session itself — creating beads and marking the plan's frontmatter as `decomposed: true`. When control returns to the Go CLI after the Claude session exits, the chaining logic currently always asks "Run 'gromit decompose <name>'?" regardless of whether decomposition already happened.

The fix: before offering to decompose, read the plan file's frontmatter and check if `decomposed` is `true`. If so, silently skip the prompt — no message, no question.

This applies to two code paths:

1. **`chainAfterPlan(planName)`** in `cmd/gromit/plan.go` — called after a single `gromit plan` session completes. Should read the plan file at `.gromit/plans/<name>.md`, check frontmatter, and skip the decompose prompt if already decomposed.

2. **`chainAfterRefine()` Phase 2** in `cmd/gromit/chain.go` — iterates planned specs and offers to decompose each one. Should check each plan's frontmatter before prompting and silently skip any that are already decomposed.

## Acceptance Criteria

- After `gromit plan` completes, if the plan file has `decomposed: true` in its frontmatter, the "Run 'gromit decompose'?" prompt is not shown
- In `chainAfterRefine` Phase 2, plans with `decomposed: true` are silently skipped — no prompt, no error
- Plans with `decomposed: false` (or no `decomposed` field) continue to prompt as before — no change to the happy path

## Decisions

1. **Silent skip, no message** The user saw decomposition happen during the interactive session, so printing "already decomposed" would be noise. Just don't ask.

2. **Check frontmatter, not bead existence** The `decomposed` boolean in the plan's YAML frontmatter is the canonical signal. This is the same field that `gromit decompose` itself checks (in `decompose.go` lines 96-99) and updates after successful decomposition (lines 188-196). Reusing it keeps the logic consistent.

## Research & Context

### Current State

**`chainAfterPlan()`** (`cmd/gromit/plan.go:243-251`): Unconditionally prompts to decompose. Needs a frontmatter check before the `confirmPrompt` call.

**`chainAfterRefine()` Phase 2** (`cmd/gromit/chain.go:119-137`): Iterates `plannedNames` and prompts for each. Needs a per-plan frontmatter check, skipping decomposed plans. The `decomposedCount` should still increment for already-decomposed plans so Phase 3 (offer to run) works correctly.

**Frontmatter package** (`internal/frontmatter/frontmatter.go`): Already provides `ReadFile(path)` which returns parsed frontmatter as `map[string]interface{}` and body as `string`. The decompose command already uses this pattern to check `decomposed`.

**Plan files**: Written by the plan skill with frontmatter including `decomposed: false`. Updated by `gromit decompose` to `decomposed: true` with a `decomposed_at` timestamp.

### Plans directory resolution

Both `chainAfterPlan` and `chainAfterRefine` need access to the plans directory path to read plan files. `chainAfterPlan` currently doesn't receive it — it will need the path passed in or resolved internally. `chainAfterRefine` already receives `plansDir` as a parameter.
