---
id: plan-picker-filter
source_ideas: []
created: 2026-02-06
---

# Filter Already-Planned Specs from Plan Picker

## Specification

When a user runs `gromit plan` without arguments, the interactive picker should only show specs that do not yet have a corresponding plan file. This matches the pattern used by `gromit refine`, which filters out already-refined backlog items.

The picker checks for a matching plan file in the plans directory (same basename as the spec). If a plan file exists, that spec is excluded from the list. If all specs have plans, the command prints a message indicating there are no unplanned specs and suggests using `--force` to re-plan an existing one.

When a user wants to re-plan a spec, they use the explicit form: `gromit plan <spec-name> --force`.

## Acceptance Criteria

- Running `gromit plan` with no arguments only lists specs that have no corresponding file in the plans directory
- If all specs have plans, the command prints a helpful message and exits without error
- `gromit plan <spec-name>` and `gromit plan <spec-name> --force` continue to work unchanged

## Decisions

1. **Filter rather than annotate** Showing only unplanned specs keeps the picker clean and follows the precedent set by `gromit refine`, which filters out refined items rather than annotating them.

2. **Use existing `--force` flag for re-planning** No new flags needed. The `--force` flag already exists and handles the re-plan case when a spec name is provided explicitly.

## Research & Context

### Current State

The picker in `cmd/gromit/plan.go:68-104` calls `getSpecFiles(specsDir)` and lists all specs regardless of plan status. If the user picks one that already has a plan, they get an error at line 121: `"plan already exists: %s\nUse --force to regenerate"`.

The plans directory is resolved by `resolvePlansDir()` in the same file. A plan exists when a file with the same basename exists in the plans directory.

### Precedent

`cmd/gromit/refine.go:72-83` filters backlog items by status, only showing items where `idea.Status != "refined"`. The plan picker should follow the same pattern: check for a corresponding plan file and exclude specs that have one.
