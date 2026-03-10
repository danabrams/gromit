Good. I have a clear picture. Here's the plan:

---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Fix the 5 failing acceptance criteria — compilation errors, model fallback, duplicate orchestration, gate satisfaction bead-closing, and from-review bypass proof.
**Architecture:** The feature is ~90% implemented. Production code has one compile error; tests have type mismatches and references to non-existent methods/constants; two behavioral gaps need surgical fixes.
**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

The spec-level review stage (`internal/v2/stage/specreview/`) already exists and produces structured findings. The spec loop (`internal/v2/loop/spec_loop.go`) already calls `ensureAcceptance` which gates on both accept and spec-review. The remediation runner already receives findings and the decompose stage already has a findings-based template. From-review bead creation logic exists in both the spec-review stage and the spec loop.

**Remaining gaps:**
1. `cmd/gromit/run2.go` — wrong type name (`adapter.TaskTracker` → `adapter.TaskTrackerAdapter`)
2. `specreview.go` selectModel fallback is `ModelSonnet` instead of `ModelOpus`
3. `spec_loop.go` has duplicate spec-review orchestration after `ensureAcceptance` already handles it
4. Gate stage doesn't close beads when satisfaction check passes
5. No test proving from-review path bypasses spec-level accept/review
6. Multiple test files have compilation errors (wrong types, undefined methods/constants)

**Integration points:** spec_loop.go orchestrates accept → specreview → remediation flow; gate.go integrates with task tracker for bead lifecycle; run2.go wires CLI flags to loop components.

---

## Test Strategy

- **Unit tests:** Fix existing broken tests in `specreview_test.go`, `spec_loop_specreview_test.go`, `spec_loop_test.go`; add `TestSelectModelDefaultsToOpus`; add `TestGateClosesSatisfiedBead`
- **Integration test:** Add test proving from-review bead execution bypasses spec-level accept/review
- **Verification:** `go build ./...` and `go test ./... -count=1` must pass after each task
- **Mocking:** Existing fake/scripted stage patterns in test files; gate tests use mock task tracker

---

## Implementation Tasks

### Task 1: Fix production compilation error in run2.go
**Files:** `cmd/gromit/run2.go`
**What to Do:** Change `adapter.TaskTracker` to `adapter.TaskTrackerAdapter` on line 65. This is the only production compilation error.
**Acceptance Criteria:**
- `go build ./...` exits 0

### Task 2: Fix specreview test compilation and model fallback (Criterion 1)
**Files:** `internal/v2/stage/specreview/specreview.go`, `internal/v2/stage/specreview/specreview_test.go`
**What to Do:** Remove unused `"reflect"` import and add missing `"fmt"` import in test file. Add `TestSelectModelDefaultsToOpus` test. Change `selectModel` fallback from `config.ModelSonnet` to `config.ModelOpus` in specreview.go. Run TDD: red (test fails with sonnet) → green (change to opus).
**Acceptance Criteria:**
- `selectModel` returns `config.ModelOpus` when no model configured
- All specreview tests compile and pass
**Dependencies:** Task 1 (build must compile)

### Task 3: Remove duplicate spec-review orchestration from Run() (Criterion 3)
**Files:** `internal/v2/loop/spec_loop.go`
**What to Do:** The `Run()` method calls `ensureAcceptance()` which already gates on both accept AND spec-review in a retry loop. Lines after `ensureAcceptance` contain duplicate spec-review checks and redundant `runSpecReview`/`runSpecReviewStage` calls. Remove the duplicate blocks, keeping only from-review bead creation and the accept commit. Remove dead methods if they become unreferenced.
**Acceptance Criteria:**
- `ensureAcceptance` is the single orchestration point for accept + spec review
- No duplicate spec-review calls exist in `Run()`
- `go build ./internal/v2/loop/...` succeeds

### Task 4: Fix spec_loop test compilation errors (Criteria 3, 9)
**Files:** `internal/v2/loop/spec_loop_specreview_test.go`, `internal/v2/loop/spec_loop_test.go`
**What to Do:** Fix type mismatches: `[]stage.Finding` → `[]specreview.SpecReviewFinding` and `[]stagepkg.Finding` → `[]finding.Finding`. Fix method reference: `ensureAcceptanceAndReview` → `ensureAcceptance` (returns 3 values). Fix undefined constants: `SpecFindingCategorySecurity` → `SpecFindingCategorySafety`, `SpecFindingCategoryTestGap` → `SpecFindingCategoryQuality`, `SpecFindingCategoryArchitecture` → `SpecFindingCategoryScope`. Fix value-vs-pointer: pass `&stagepkg.Result{...}` instead of `stagepkg.Result{...}` to `newScriptedSpecReviewStage`. Fix undefined `specReviewStage` reference in `spec_loop_test.go`.
**Acceptance Criteria:**
- All tests in `internal/v2/loop/...` compile and pass
- No references to undefined methods or constants
**Dependencies:** Task 3 (Run() changes may affect test expectations)

### Task 5: Add gate satisfaction bead-closing logic (Criterion 8)
**Files:** `internal/v2/stage/gate/gate.go`, `internal/v2/stage/gate/gate_test.go`
**What to Do:** When `checkSatisfaction` returns true in the gate stage's Run method, close the bead via the task tracker's `CloseBead` method before returning `DecisionSkip`. Write a failing test first (`TestGateClosesSatisfiedBead`) that asserts `CloseBead` is called with the bead's ID when satisfaction check passes.
**Acceptance Criteria:**
- When `checkSatisfaction` returns true, the bead is closed via `CloseBead`
- Gate returns skip decision for already-satisfied beads
- All gate tests pass

### Task 6: Add test proving from-review beads bypass spec-level accept/review (Criterion 13)
**Files:** `cmd/gromit/run2_test.go` (or `internal/v2/loop/` test file if more appropriate)
**What to Do:** Write a test demonstrating that `run2FromReview` (or the from-review execution path) calls `BeadLoop.Run`/`RunWithoutReview` directly and never invokes accept or spec-review stages. Use instrumented/fake stages that record whether they were called. Assert accept and spec-review were NOT called while bead loop WAS called.
**Acceptance Criteria:**
- Test demonstrates from-review path does not invoke accept or spec review stages
- Test compiles and passes
**Dependencies:** Task 1 (run2.go must compile)

### Task 7: Final verification
**Files:** (read-only)
**What to Do:** Run `go build ./...` and `go test ./... -count=1 -timeout=300s`. Verify all 5 gap analysis criteria have corresponding working code and passing tests. Run `grep -rn 'os\.Chdir' cmd/ internal/ --include='*_test.go'` to verify project rules compliance.
**Acceptance Criteria:**
- `go build ./...` exits 0
- `go test ./...` exits 0
- All 5 gap analysis criteria resolved