---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T04:50:23-05:00"
id: skip-decompose-if-done
source_spec: skip-decompose-if-done
---

# Skip Decompose Prompt If Plan Already Decomposed — Implementation Plan

**Goal:** Stop prompting "Run 'gromit decompose'?" when the plan was already decomposed during the interactive session.

**Architecture:** Add an `isPlanDecomposed` helper that reads plan frontmatter, then guard both `chainAfterPlan` and `chainAfterRefine` Phase 2 with it.

**Tech Stack:** Go

**Spec:** `.gromit/specs/skip-decompose-if-done.md`

---

## Architecture

**Overview:** Two code paths unconditionally prompt to decompose. Both need a frontmatter check before prompting. A shared helper centralizes the check.

**Helper function — `isPlanDecomposed(plansDir, planName string) bool`:**
- Lives in `cmd/gromit/chain.go` (chain-concern utility)
- Reads `<plansDir>/<planName>.md` via `frontmatter.ReadFile`
- Returns `true` only if frontmatter has `decomposed: true`
- Returns `false` for missing file, parse error, missing field, or `decomposed: false`
- Matches the pattern in `decompose.go:97`: `planFrontmatter["decomposed"].(bool)`

**`chainAfterPlan` changes (`plan.go`):**
- Signature changes from `chainAfterPlan(planName string)` to `chainAfterPlan(planName, plansDir string)`
- Call site at line 222 updated to pass `plansDir`
- First line checks `isPlanDecomposed(plansDir, planName)` — if true, return immediately (silent skip)

**`chainAfterRefine` Phase 2 changes (`chain.go`):**
- Inside the `for _, planName := range plannedNames` loop (line 121), add check before `confirmPrompt`
- If `isPlanDecomposed(plansDir, planName)`: increment `decomposedCount++` and `continue`
- This ensures Phase 3 ("Run gromit run?") still triggers correctly even if all plans were decomposed during planning

**Files to Modify:**
- `cmd/gromit/chain.go` — Add `isPlanDecomposed` helper, update Phase 2 loop
- `cmd/gromit/plan.go` — Update `chainAfterPlan` signature and call site

## Test Strategy

**Unit tests for `isPlanDecomposed`** in `cmd/gromit/chain_test.go`:
- Table-driven tests using `t.TempDir()` with real plan files
- Cases: `decomposed: true`, `decomposed: false`, no `decomposed` field, missing file
- Follows existing patterns in `plan_test.go` (temp dirs, real files)

**No new integration tests needed:**
- The helper is pure logic (file read + type assertion)
- The wiring into `chainAfterPlan`/`chainAfterRefine` is a single `if` guard
- Existing chain integration tests (noted as documentary) are not affected

## Implementation Tasks

### Task 1: Add `isPlanDecomposed` helper and tests

**Files:**
- Modify: `cmd/gromit/chain.go`
- Modify: `cmd/gromit/chain_test.go`

**What to Do:**
Add `isPlanDecomposed(plansDir, planName string) bool` to `chain.go`. It reads `filepath.Join(plansDir, planName+".md")` via `frontmatter.ReadFile`, checks `fm["decomposed"].(bool)`, returns the result. Returns `false` on any error. Add `frontmatter` import to `chain.go`.

Add `TestIsPlanDecomposed` table-driven test in `chain_test.go` with cases:
- Plan with `decomposed: true` → returns true
- Plan with `decomposed: false` → returns false
- Plan with no `decomposed` field → returns false
- Missing plan file → returns false

**Acceptance Criteria:**
- `isPlanDecomposed` returns `true` only when frontmatter has `decomposed: true`
- `isPlanDecomposed` returns `false` for all other cases without panicking
- All test cases pass

**Dependencies:** None

### Task 2: Wire helper into chainAfterPlan and chainAfterRefine Phase 2

**Files:**
- Modify: `cmd/gromit/plan.go`
- Modify: `cmd/gromit/chain.go`

**What to Do:**

In `plan.go`:
- Change `chainAfterPlan(specName)` call (line 222) to `chainAfterPlan(specName, plansDir)`
- Change `chainAfterPlan` function signature to `chainAfterPlan(planName, plansDir string)`
- Add `if isPlanDecomposed(plansDir, planName) { return }` as the first line of `chainAfterPlan`

In `chain.go` Phase 2 loop (line 121):
- Add at the top of the loop body, before `confirmPrompt`:
  ```go
  if isPlanDecomposed(plansDir, planName) {
      decomposedCount++
      continue
  }
  ```

**Acceptance Criteria:**
- `chainAfterPlan` silently returns when plan is already decomposed
- `chainAfterRefine` Phase 2 skips prompt for decomposed plans but increments `decomposedCount`
- Non-decomposed plans continue to prompt as before

**Dependencies:** Task 1

---

## Notes

- The `frontmatter` package import needs to be added to `chain.go` — it's already imported in `plan.go`
- The silent skip (no message) is intentional per spec — the user saw decomposition happen during the interactive session
- This matches the existing pattern in `decompose.go:97` for checking the `decomposed` field
