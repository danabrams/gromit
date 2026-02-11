---
id: run-scope-flags
source_ideas: [integrate-spec-epic-scope-flags]
created: 2026-02-11
---

# Run Command Scope Flags

## Specification

The `gromit run` command gains `--spec <name>` and `--epic <id>` flags that restrict which beads the run loop processes. When `--spec` is provided, the runner only picks up beads labeled `spec:<name>`. When `--epic` is provided, the runner resolves all specs belonging to that epic and only picks up beads labeled with any of those specs. The two flags are mutually exclusive.

The integration follows the same pattern already established by the `retro` command: flag declaration, `scope.ValidateFlags()` call, label resolution via `scope.ResolveSpec()` or `scope.ResolveEpic()`, and `runner.SetLabelFilters()` to configure filtering before the loop starts.

All downstream infrastructure already exists and is tested — this feature is purely CLI wiring in `cmd/gromit/main.go`.

## Acceptance Criteria

- `gromit run --spec <name>` only processes beads labeled `spec:<name>`
- `gromit run --epic <id>` resolves the epic's specs and only processes beads labeled with those specs
- `gromit run --spec X --epic Y` returns an error (mutually exclusive)

## Decisions

1. **Follow the retro command pattern exactly.** The `retro` command already wires `--spec` and `--epic` through `scope.ValidateFlags()` and label resolution. The `run` command uses the same approach for consistency.

2. **No --since flag.** Time-based filtering is out of scope; only `--spec` and `--epic` are added.

3. **No new structs or refactoring.** The existing `SetLabelFilters()` API on the runner is sufficient. No `RunConfig` struct or other abstraction is needed.

## Research & Context

### Current State

All infrastructure is built and tested:

- **`internal/scope/scope.go`** — `ValidateFlags(epic, spec)` validates mutual exclusivity; `ResolveSpec(name)` returns `[]string{"spec:<name>"}`; `ResolveEpic(id, specsDir)` scans spec files and returns matching labels.
- **`internal/bead/bead.go`** — `ReadyWithLabel(label)` fetches ready beads filtered by label via `bd ready --json --limit 10 --label <label>`.
- **`internal/runner/runner.go`** — `SetLabelFilters(labels)` stores labels; `getNextBead()` dispatches to `ReadyWithLabel` when filters are set, selecting the highest-priority bead across all labels.
- **`cmd/gromit/main.go`** — `retro` command (lines 105-106, 177-224) demonstrates the exact wiring pattern needed.

### What Needs to Change

Only `cmd/gromit/main.go`:
1. Two package-level vars (`runSpecFlag`, `runEpicFlag`)
2. Two flag declarations in `init()` on `runCmd`
3. ~15 lines in `runLoop()`: validate flags, resolve labels, call `r.SetLabelFilters()`
