---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T07:26:49-05:00"
id: run-scope-flags
source_spec: run-scope-flags
---

# Run Command Scope Flags Implementation Plan

**Goal:** Add `--spec` and `--epic` flags to `gromit run` to restrict which beads the loop processes.

**Architecture:** Pure CLI wiring in `cmd/gromit/main.go`, following the exact pattern established by the `retro` command. All downstream infrastructure (`scope.ValidateFlags`, `scope.ResolveSpec`, `scope.ResolveEpic`, `runner.SetLabelFilters`) already exists and is tested.

**Tech Stack:** Go, cobra (CLI framework)

**Spec:** `.gromit/specs/run-scope-flags.md`

---

## Architecture

**Overview:**
Wire `--spec` and `--epic` flags into the `gromit run` command following the exact pattern established by the `retro` command — flag vars, flag declarations, and label resolution in `runLoop()` before `r.Run()`.

**Key Components:**
1. **`cmd/gromit/main.go`**: The only file that needs changes. Add two package-level vars, two flag declarations in `init()`, and ~15 lines of scope resolution in `runLoop()`.

**Integration Points:**
- `scope.ValidateFlags(epic, spec)` — already exists, validates mutual exclusivity
- `scope.ResolveSpec(name)` — already exists, returns `[]string{"spec:<name>"}`
- `scope.ResolveEpic(id, specsDir)` — already exists, scans spec frontmatter
- `runner.SetLabelFilters(labels)` — already exists, configures bead filtering in the loop

**Data Flow:**
1. User passes `--spec foo` or `--epic bar` to `gromit run`
2. `runLoop()` calls `scope.ValidateFlags()` to check mutual exclusivity
3. If `--spec`: `scope.ResolveSpec()` returns `["spec:foo"]`
4. If `--epic`: `scope.ResolveEpic()` scans specs dir and returns matching labels
5. `r.SetLabelFilters(labels)` configures the runner
6. `r.Run()` uses `getNextBead()` which dispatches to `ReadyWithLabel` when filters are set

**Files to Modify:**
- `cmd/gromit/main.go` — add flag vars, flag declarations, scope resolution wiring, and Long description update

**Tradeoffs:**
- Follow retro pattern exactly over creating a shared helper: the retro command uses `buildBeadFilter()` to resolve labels into bead ID maps, but the run command uses `SetLabelFilters()` which takes label strings directly. Similar but not identical — a shared abstraction would be premature.

## Test Strategy

No new tests needed. All infrastructure is already tested:
- `internal/scope/` tests cover `ValidateFlags`, `ResolveSpec`, `ResolveEpic`
- `internal/runner/` tests cover `SetLabelFilters` and `getNextBead` label filtering
- The only new code is ~15 lines of CLI wiring connecting these tested components

Manual verification: run `gromit run --spec X`, `gromit run --epic Y`, and `gromit run --spec X --epic Y`.

## Implementation Tasks

### Task 1: Wire --spec and --epic flags into gromit run

**Files:**
- Modify: `cmd/gromit/main.go`

**What to Do:**
1. Add two package-level vars: `runSpecFlag`, `runEpicFlag`
2. Add two flag declarations in `init()` on `runCmd`, mirroring the retro pattern at lines 105-106
3. In `runLoop()`, after config loading and before `runner.NewRunner()`:
   - Call `scope.ValidateFlags(runEpicFlag, runSpecFlag)` and return error if invalid
   - Resolve labels via `scope.ResolveSpec()` or `scope.ResolveEpic()` (using `resolveGromitDir(cfg)` for specs path)
   - After creating the runner, call `r.SetLabelFilters(labels)` if labels are non-empty
4. Update the `runCmd` `Long` description to document the new flags

**Acceptance Criteria:**
- `gromit run --spec <name>` calls `SetLabelFilters` with `["spec:<name>"]`
- `gromit run --epic <id>` resolves the epic's specs and calls `SetLabelFilters` with those labels
- `gromit run --spec X --epic Y` returns an error before the loop starts

**Dependencies:** None

---

## Notes

- The retro command pattern (lines 177-224 in main.go) is the direct template for this work
- The retro command resolves labels into bead ID maps via `buildBeadFilter()` for filtering iteration logs; the run command instead passes labels directly to `SetLabelFilters()` for bead selection — different mechanisms, same flag wiring pattern
- Label resolution for `--epic` requires the specs directory path, obtained via `resolveGromitDir(cfg)` + `/specs`
