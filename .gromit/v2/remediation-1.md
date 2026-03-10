Now I have a complete picture. Here's the plan:

---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Spec-Level Review and Targeted Remediation — Remediation Plan

**Goal:** Fix all compilation errors in the partially-implemented spec-level review and targeted remediation feature so the code compiles, tests pass, and all acceptance criteria are satisfied.

**Architecture:** The feature is partially implemented across ~30 commits but accumulated compilation errors from overlapping bead iterations. Two leaf packages (`specreview`, `accept`) have hard compilation errors; all downstream packages (`loop`, `cmd/gromit`) fail transitively. Fix leaf packages first, then verify orchestration and wiring.

**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

### Root Causes of Compilation Failures

Two packages block the entire build:

1. **`internal/v2/stage/specreview/specreview.go`** — Three errors from orphaned early-generation code:
   - `computeVerdict` (line 382) and `convertFindings` (line 391) reference undefined `rawFinding` type
   - Duplicate `createFromReviewBeads` method at line 405 (first definition at line 338 is correct)

2. **`internal/v2/stage/accept/accept.go`** — Multiple errors from removed methods still being called:
   - `batchDiffThreshold` constant removed but still referenced by `diffThreshold()` (line 114)
   - `runBatchEvaluation` and `runTargetedEvaluation` methods removed but called by `Run()` (lines 212, 216)
   - `buildFailureFindings` returns `[]stage.Finding` but `AcceptArtifacts.Findings` expects `[]stage.SpecFinding` (lines 221, 239)
   - `runPerCriterionEvaluation` body returns `(*stagepkg.Result, error)` but signature declares `([]AcceptanceResult, []string, error)` (lines 314-319)
   - Undefined `root` variable at line 314

### Fix Strategy

Fix in dependency order: specreview first (simplest — just delete dead code), then accept (restore missing code from main + fix type mismatches), then verify the full build compiles and all tests pass.

## Test Strategy

- After specreview fix: `go build ./internal/v2/stage/specreview/...` then `go test ./internal/v2/stage/specreview/...`
- After accept fix: `go build ./internal/v2/stage/accept/...` then `go test ./internal/v2/stage/accept/...`
- Final: `go build ./...` and `go test ./... -count=1 -timeout=300s`

## Implementation Tasks

### Task 1: Remove orphaned `rawFinding` functions and duplicate `createFromReviewBeads` from specreview.go

**Files:** `internal/v2/stage/specreview/specreview.go`

**What to Do:**
1. Delete `computeVerdict` function (lines 382-389) — references undefined `rawFinding` type; the working implementation is `verdictFromFindings` (line 110)
2. Delete `convertFindings` function (lines 391-403) — references undefined `rawFinding` type; the working implementation is `parseSpecReviewOutput` (line 52)
3. Delete the second `createFromReviewBeads` method (lines 405-429) — has signature `(ctx, specID string, findings []stagepkg.Finding) ([]*trackertypes.Bead, error)` which conflicts with the first definition at line 338 that has signature `(ctx, specID string, artifacts *SpecReviewArtifacts, decision stagepkg.Decision) error`. The first is called by `Run()` at line 223 and is correct.
4. Keep helper functions `labelsForFinding` (431-439), `beadTitleForFinding` (441-452), `priorityForSeverity` (454-465) — not causing errors and may be used by the first `createFromReviewBeads`.

**Acceptance Criteria:**
- `go build ./internal/v2/stage/specreview/...` succeeds
- `go test ./internal/v2/stage/specreview/...` passes
- No references to `rawFinding` remain in the file

---

### Task 2: Fix accept.go — restore missing constant, methods, and fix type mismatches

**Files:** `internal/v2/stage/accept/accept.go`

**What to Do:**

1. **Restore `batchDiffThreshold` constant.** It's referenced by `diffThreshold()` at line 114 but was removed. Add to the const block: `batchDiffThreshold = 50000 // ~12K tokens` (matching the value on main).

2. **Restore `runBatchEvaluation` and `runTargetedEvaluation` methods from main.** Run `git show main:internal/v2/stage/accept/accept.go` to extract these methods and their helpers (`mapCriteriaToFiles`, `splitDiffByFile`, `buildTargetedDiff`, `parseBatchEvaluation`, `parseCriteriaMapping`, `buildBatchPrompt`). Copy them into the current file. These are called at lines 212 and 216.

3. **Fix `buildFailureFindings` return type.** Currently returns `[]stage.Finding` but `AcceptArtifacts.Findings` expects `[]stage.SpecFinding`. Change the function to return `[]stage.SpecFinding`, constructing `SpecFinding` values with `Severity: SpecFindingSeverityCritical`, `Category: SpecFindingCategoryAcceptance`, `Scope: SpecFindingScopeSpec`.

4. **Fix `runPerCriterionEvaluation` return statements.** Lines 310-319 return `(*stagepkg.Result, error)` but the method signature declares `([]AcceptanceResult, []string, error)`. The method body correctly builds `results` and `failures` slices (lines 255-308). Replace lines 310-319 with: `return results, failures, nil` — the caller in `Run()` at line 234 already handles wrapping results into artifacts and writing gap analysis. Also remove the unused `findings` slice at line 257 and the findings-append block at lines 293-299 (since `buildFailureFindings` in the `Run()` caller handles this). Fix the undefined `root` variable reference at line 314 by removing that code block.

5. **Fix targeted evaluation indentation.** Lines 216-231 are de-indented from the `if threshold > 0` block. Re-indent to be properly nested.

**Acceptance Criteria:**
- `go build ./internal/v2/stage/accept/...` succeeds
- `go test ./internal/v2/stage/accept/...` passes
- `buildFailureFindings` returns `[]stagepkg.SpecFinding`

**Dependencies:** None (independent of Task 1, can be done in parallel)

---

### Task 3: Verify full build compiles and all tests pass

**Files:** Any files with secondary issues discovered after Tasks 1-2

**What to Do:**
1. Run `go build ./...` — if errors remain in `spec_loop.go`, `run2_components.go`, or `run2.go`, fix them (the existing plan identified potential issues: duplicate struct fields in `SpecLoop`, duplicate fragment loads in `run2_components`, duplicate variable declarations in `run2.go`)
2. Run `go test ./... -count=1 -timeout=300s` — fix any test failures
3. Verify project rules: `grep -rn 'os\.Chdir' cmd/ internal/ --include='*_test.go'` returns zero hits
4. Cross-reference each gap analysis criterion to confirm the fix didn't regress any behavioral paths

**Acceptance Criteria:**
- `go build ./...` exits 0
- `go test ./...` exits 0
- All 12 acceptance criteria from the spec have corresponding compilable code paths

**Dependencies:** Task 1, Task 2