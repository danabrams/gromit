---
created: 2026-02-06T00:00:00Z
decomposed: true
decomposed_at: "2026-02-06T22:03:51-05:00"
id: plan-picker-filter
source_spec: plan-picker-filter
---

# Filter Already-Planned Specs from Plan Picker — Implementation Plan

**Goal:** When `gromit plan` runs without arguments, only show specs that don't yet have a corresponding plan file.

**Architecture:** Add a filter function in `plan.go` that checks each spec against the plans directory and excludes specs with existing plans. Follow the same pattern as `refine.go`'s unrefined-item filter.

**Tech Stack:** Go

**Spec:** `.gromit/specs/plan-picker-filter.md`

---

## Architecture

**Overview:**
Filter the specs list in the interactive picker by checking for corresponding plan files in the plans directory, following the same pattern used by `refine.go` for filtering already-refined backlog items.

**Key Changes:**
1. **`cmd/gromit/plan.go` - Filter logic**: After `getSpecFiles()` returns all specs, a new `filterUnplannedSpecs` function checks `os.Stat(filepath.Join(plansDir, basename))` for each spec and excludes those with matching plan files.

**Integration Points:**
- Uses existing `getSpecFiles()` (defined in `refine.go`, shared via package `main`)
- Uses existing `resolvePlansDir()` (already in `plan.go`)
- No new packages, imports, or files beyond the test file

**Data Flow:**
1. `getSpecFiles(specsDir)` returns all `.md` files in specs directory
2. `filterUnplannedSpecs(specs, plansDir)` checks each spec basename against plans directory via `os.Stat`
3. Only specs without a matching plan file pass through to the picker
4. If the filtered list is empty but unfiltered list was not, print "all planned" message

**Files to Modify:**
- `cmd/gromit/plan.go` — add `filterUnplannedSpecs` function, call it in picker path, add "all planned" message

**Files to Create:**
- `cmd/gromit/plan_test.go` — unit tests for `filterUnplannedSpecs`

**Tradeoffs:**
- **Inline filter vs. helper function**: Chose a small named function (`filterUnplannedSpecs`) for testability, even though the logic is ~6 lines.
- **`os.Stat` per spec vs. reading plans dir**: `os.Stat` per spec is simpler and avoids listing the plans directory. Performance is irrelevant at this scale.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Table-driven tests for `filterUnplannedSpecs` using real temp directories
2. **Manual Testing**: Run `gromit plan` with various spec/plan combinations

**Key Test Cases:**
- Empty specs list → empty result
- No plans exist → all specs pass through
- All specs have plans → empty result
- Mix of planned and unplanned → only unplanned returned
- Explicit `gromit plan <name>` path → unchanged (no filtering)
- `gromit plan <name> --force` → unchanged

**Test Organization:**
- `cmd/gromit/plan_test.go` with `TestFilterUnplannedSpecs`
- Uses `t.TempDir()` for real filesystem setup, no mocks

**Coverage Goals:**
- All branches of `filterUnplannedSpecs` covered
- "All specs planned" message path exercised

## Implementation Tasks

### Task 1: Extract filter function and add filtering to picker

**Files:**
- Modify: `cmd/gromit/plan.go`

**What to Do:**
1. Add a `filterUnplannedSpecs(specs []string, plansDir string) []string` function that iterates over specs, checks `os.Stat(filepath.Join(plansDir, filepath.Base(spec)))` for each, and returns only those where the stat fails (no plan file exists).
2. In `runPlan`, after the `getSpecFiles(specsDir)` call (line 70) and before the empty check (line 75), call `filterUnplannedSpecs(specs, plansDir)` to narrow the list.
3. Update the empty-specs check to distinguish two cases:
   - No specs at all (original `getSpecFiles` returned empty) → keep existing message
   - All specs have plans (original list was non-empty but filtered list is empty) → new message: "All specs already have plans." + "Use 'gromit plan <spec-name> --force' to re-plan an existing one."

**Acceptance Criteria:**
- Picker only shows specs without a corresponding plan file
- "All specs already have plans" message shown when all are planned
- Explicit name and --force paths unchanged

**Dependencies:** None

### Task 2: Add unit tests for filter function

**Files:**
- Create: `cmd/gromit/plan_test.go`

**What to Do:**
Write a `TestFilterUnplannedSpecs` function with table-driven subtests. For each case, create a temp directory structure with spec files and (optionally) plan files, call `filterUnplannedSpecs`, and assert the result.

Test cases:
1. **Empty input**: empty specs slice → empty result
2. **No plans exist**: 3 specs, 0 plans → all 3 returned
3. **All have plans**: 3 specs, 3 matching plans → empty result
4. **Mixed**: 3 specs, 1 matching plan → 2 returned

**Acceptance Criteria:**
- All four scenarios tested and passing
- Tests use `t.TempDir()` for filesystem isolation
- `go test ./cmd/gromit/...` passes

**Dependencies:** Task 1

---

## Notes

- The `--force` flag only applies when a spec name is given explicitly (not in the picker path), so no changes needed for the force logic.
- The existing plan-exists check at line 121 still serves as a safety net for the explicit-name path.
- This is a small, focused change — two tasks mapping to ~2 beads during decompose.
