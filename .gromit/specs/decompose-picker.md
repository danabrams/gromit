---
id: decompose-picker
source_ideas: [14]
created: 2026-02-07
epic: developer-experience
---

# Decompose Picker

## Specification

When `gromit decompose` is called without arguments, it displays an interactive picker listing all undecomposed plans. The user can select a single plan to decompose, or (when there are 2+ undecomposed plans) choose "Decompose all" to process every undecomposed plan sequentially without pausing between them.

When called with a plan name argument, behavior is unchanged — it decomposes that specific plan directly.

### Picker Behavior

1. Scan `.gromit/plans/` for all plan files.
2. Read frontmatter from each plan and filter to those where `decomposed` is `false` or missing.
3. If no undecomposed plans exist, print a helpful message (e.g., "No undecomposed plans found. Create one with `gromit plan`.") and exit 0.
4. If exactly 1 undecomposed plan exists, show a picker with just that plan (no "Decompose all" option).
5. If 2+ undecomposed plans exist, show a numbered picker with each plan listed, plus a "Decompose all" option as the last entry.
6. The picker displays plan names and titles (extracted from the plan's `# Title` heading) to help the user identify each plan.

### "Decompose All" Behavior

When "Decompose all" is selected:
- Decompose each undecomposed plan sequentially in alphabetical order by plan name.
- Show progress as each plan is processed (e.g., "[1/3] Decomposing api-endpoints...").
- Do not pause between plans.
- If a plan fails to decompose, print the error and continue to the next plan.
- After all plans are processed, summarize results (e.g., "Decomposed 3/3 plans successfully.").
- The `--review` flag is respected for each individual plan when used with "Decompose all."

### Flag Interactions

- `--review`: Works with both single-plan and decompose-all modes. In decompose-all mode, each plan's proposed beads are shown for review individually.
- `--force`: When combined with no-args mode, the picker shows **all** plans (including already-decomposed ones), since force means "re-decompose."
- `--no-chain`: Suppresses the post-decompose chaining prompt, same as today.

## Acceptance Criteria

- Calling `gromit decompose` with no arguments shows a picker of undecomposed plans.
- Selecting a plan from the picker decomposes it (same behavior as passing the name directly).
- When 2+ undecomposed plans exist, a "Decompose all" option appears and processes each plan sequentially without pausing.
- When no undecomposed plans exist, a helpful message is displayed and the command exits cleanly.

## Decisions

1. **Sequential without pausing for "Decompose all"** — Each plan is decomposed one after another automatically. Progress is shown but the user is not prompted between plans. This keeps the workflow fast while still providing visibility.

2. **"Decompose all" only shown with 2+ plans** — When there's exactly one undecomposed plan, the "Decompose all" option would be redundant, so it's omitted.

3. **Continue on failure in batch mode** — If one plan fails during "Decompose all," the rest still get processed. This is more useful than aborting the entire batch for one failure.

4. **`--force` shows all plans in picker** — Since `--force` means "re-decompose even if already done," it makes sense to include already-decomposed plans in the picker when this flag is used.

## Research & Context

### Current State

The decompose command lives in `cmd/gromit/decompose.go` and currently requires exactly one argument (`cobra.ExactArgs(1)`). Plans are stored in `.gromit/plans/` with frontmatter tracking `decomposed: true/false` and `decomposed_at` timestamp.

### Existing Picker Pattern

Both `refine` (`cmd/gromit/refine.go:66-127`) and `plan` (`cmd/gromit/plan.go:70-109`) implement arg-less picker modes:
- Filter items by status (unrefined backlog items, unplanned specs)
- Display numbered list with context
- Handle empty-list case gracefully
- Use `promptui.Select` for the interactive picker

The `plan` command's `filterUnplannedSpecs()` helper in `cmd/gromit/plan.go` is the closest analog — decompose needs a similar `filterUndecomposedPlans()` that reads plan frontmatter and filters by `decomposed` field.

### Frontmatter Parsing

Plan frontmatter is parsed via `internal/frontmatter/frontmatter.go` using `ReadFile()`. The decompose command already uses this to check the `decomposed` flag — the picker just needs to do this across all plans in the directory.
