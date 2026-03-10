# Spec-Level Review and Targeted Remediation — Gap Remediation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the 5 failing acceptance criteria from the gap analysis: compilation errors in tests, model fallback defaulting to sonnet instead of opus, missing spec review gate after `ensureAcceptance`, gate satisfaction not closing stale beads, and from-review beads not bypassing spec-level accept/review.

**Architecture:** The feature is partially implemented and the production code mostly compiles (single error: `adapter.TaskTracker` → `adapter.TaskTrackerAdapter`). Tests have compilation errors from type mismatches and references to non-existent methods. The fixes are surgical — one compile fix, test corrections, and two behavioral gaps.

**Tech Stack:** Go 1.24, cobra CLI, TDD with `go test`

---

## Architecture

### Current State

Production code compiles with exactly one error:
- `cmd/gromit/run2.go:65` — `adapter.TaskTracker` should be `adapter.TaskTrackerAdapter`

Test code fails to compile in two packages:
- `internal/v2/stage/specreview/specreview_test.go` — unused `reflect` import, missing `fmt` import
- `internal/v2/loop/spec_loop_specreview_test.go` — references `ensureAcceptanceAndReview` (doesn't exist, should be `ensureAcceptance`), uses `[]stage.Finding` where `[]finding.Finding` is expected, references undefined category constants
- `internal/v2/loop/spec_loop_test.go` — `newFakeSpecReviewStage` passes `stagepkg.Result` by value instead of pointer, `newScriptedSpecReviewStage` also has value-vs-pointer issues

### Gap Analysis Criteria to Fix

| # | Criterion | Root Cause | Fix |
|---|-----------|-----------|-----|
| 1 | Spec review uses highest-tier model | `selectModel` fallback is `config.ModelSonnet` | Change to `config.ModelOpus` |
| 3 | Spec succeeds only when both accept and review pass | `Run()` has duplicate spec-review blocks after `ensureAcceptance` already handles it | Remove redundant blocks; `ensureAcceptance` already gates both |
| 8 | Gate satisfaction closes open beads | `checkSatisfaction` returns bool but doesn't close beads | Add bead-closing logic to gate stage when satisfaction check passes |
| 9 | Spec-scoped from-review beads labeled with spec | Core logic works; tests don't compile | Fix test compilation errors |
| 13 | From-review beads don't trigger spec-level accept/review | `run2FromReview` calls bead loop directly (correct for CLI path); `runFromReview` uses `RunWithoutReview` (skips per-bead review but that's different from spec-level review) | The from-review path already bypasses spec-level accept/review since it doesn't call `SpecLoop.Run()` — it calls `BeadLoop` directly. Need test proving this. |

### Key Files

| File | Issues |
|------|--------|
| `cmd/gromit/run2.go:65` | `adapter.TaskTracker` → `adapter.TaskTrackerAdapter` |
| `internal/v2/stage/specreview/specreview.go:316` | `config.ModelSonnet` → `config.ModelOpus` |
| `internal/v2/stage/specreview/specreview_test.go` | Unused `reflect`, missing `fmt` |
| `internal/v2/loop/spec_loop.go:445-511` | Duplicate spec-review blocks after `ensureAcceptance` |
| `internal/v2/loop/spec_loop_specreview_test.go` | Wrong method name, wrong types, undefined constants |
| `internal/v2/loop/spec_loop_test.go` | Value-vs-pointer mismatch in `newFakeSpecReviewStage`/`newScriptedSpecReviewStage` calls |
| `internal/v2/stage/gate/gate.go` | Gate stage needs to close beads when satisfaction check passes |

---

## Test Strategy

- After each task: `go build ./path/to/package/...` to verify compilation
- Run package-level tests after each fix: `go test ./path/to/package/... -v -count=1`
- Final: `go test ./... -count=1 -timeout=300s` to confirm everything passes

---

## Implementation Tasks

### Task 1: Fix production compilation error in run2.go

**Files:**
- Modify: `cmd/gromit/run2.go:65`

**Step 1: Fix the type reference**

Change `adapter.TaskTracker` to `adapter.TaskTrackerAdapter` on line 65:

```go
newTaskTrackerFn = func(client *bead.Client) (adapter.TaskTrackerAdapter, error) {
```

**Step 2: Verify compilation**

Run: `go build ./cmd/gromit/...`
Expected: PASS (zero errors)

**Step 3: Commit**

```bash
git add cmd/gromit/run2.go
git commit -m "fix: use adapter.TaskTrackerAdapter type in run2.go"
```

**Acceptance Criteria:**
- `go build ./...` exits 0

---

### Task 2: Fix specreview model fallback to use opus (Criterion 1)

**Files:**
- Modify: `internal/v2/stage/specreview/specreview.go:316`
- Test: `internal/v2/stage/specreview/specreview_test.go`

**Step 1: Write a failing test**

Add a test to `specreview_test.go` that verifies `selectModel` returns `config.ModelOpus` when no model is configured:

```go
func TestSelectModelDefaultsToOpus(t *testing.T) {
	cfg := &config.Config{}
	s := &Stage{cfg: cfg}
	got := s.selectModel(cfg, &stagepkg.Request{})
	if got != config.ModelOpus {
		t.Fatalf("selectModel default = %q, want %q", got, config.ModelOpus)
	}
}
```

**Step 2: Also fix the unused import and missing fmt in specreview_test.go**

Remove the unused `"reflect"` import (line 8). Add `"fmt"` import (needed by `fakeTaskTracker.CreateBead` at line 374).

**Step 3: Run the test to verify it fails**

Run: `go test ./internal/v2/stage/specreview/... -run TestSelectModelDefaultsToOpus -v`
Expected: FAIL — returns "sonnet", want "opus"

**Step 4: Change the fallback**

In `specreview.go` line 316, change:
```go
return config.ModelSonnet
```
to:
```go
return config.ModelOpus
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/v2/stage/specreview/... -v -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add internal/v2/stage/specreview/specreview.go internal/v2/stage/specreview/specreview_test.go
git commit -m "fix: default spec review model to opus (highest tier)"
```

**Acceptance Criteria:**
- `selectModel` returns `config.ModelOpus` when no model is configured
- All specreview tests pass

---

### Task 3: Remove duplicate spec-review orchestration from Run() (Criterion 3)

The `Run()` method in `spec_loop.go` calls `ensureAcceptance()` at line 430, which already runs both accept AND spec review in a retry loop. After that, lines 445-511 contain duplicate spec-review checks and another `runSpecReview`/`runSpecReviewStage` call. This duplicate code must be removed.

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`

**Step 1: Understand the correct flow**

After `ensureAcceptance` returns successfully:
1. `acceptRes` has passed (or remediation fixed it)
2. `specReviewRes` has passed (ensureAcceptance gates on both)
3. From-review beads may need creation from spec review findings
4. Commit, present, cleanup

The duplicate blocks (lines 445-511) repeat accept/review checks and call spec review again. They should be replaced with just the from-review bead creation logic.

**Step 2: Remove the duplicate blocks**

Replace the entire section from line 438 (`if s.acceptFailed(acceptRes)`) through line 511 (the closing `}` of the second specreview block) with:

```go
	// Create from-review beads for non-critical findings if spec review passed
	// but had actionable findings. The specreview stage itself may have already
	// created them (via its own createFromReviewBeads), so check first.
	if !specReviewCreatedBeads(specReviewRes) {
		if err := s.createFromReviewBeads(ctx, specID, extractSpecReviewFindings(specReviewRes)); err != nil {
			return fmt.Errorf("create from-review beads: %w", err)
		}
	}

	if err := s.commitStage(ctx, worktree, "accept", 0, "proceed"); err != nil {
		return fmt.Errorf("commit after accept: %w", err)
	}
```

This preserves:
- From-review bead creation (Criterion 9)
- Accept commit
- No duplicate spec review calls

**Step 3: Remove the now-unused `runSpecReview` method if dead**

Check if `runSpecReview` (line 604) is called anywhere else. If only called by the deleted block, remove it. Keep `runSpecReviewStage` (line 708) since it's used by `ensureAcceptance`.

**Step 4: Verify compilation**

Run: `go build ./internal/v2/loop/...`
Expected: PASS

**Step 5: Run tests**

Run: `go test ./internal/v2/loop/... -v -count=1`
Expected: Compilation may fail due to test issues (fixed in next task)

**Step 6: Commit**

```bash
git add internal/v2/loop/spec_loop.go
git commit -m "fix: remove duplicate spec-review orchestration from Run(); ensureAcceptance already gates both"
```

**Acceptance Criteria:**
- `ensureAcceptance` is the single orchestration point for accept + spec review
- No duplicate spec-review calls in `Run()`
- `go build ./internal/v2/loop/...` succeeds

---

### Task 4: Fix spec_loop test compilation errors (Criteria 3, 9)

**Files:**
- Modify: `internal/v2/loop/spec_loop_specreview_test.go`
- Modify: `internal/v2/loop/spec_loop_test.go`

**Step 1: Fix spec_loop_specreview_test.go errors**

Error 1 (line 47): `[]stage.Finding` used where `[]specreview.SpecReviewFinding` expected:
```go
// Change from:
reviewFindings := []stagepkg.Finding{...}
// To:
reviewFindings := []specreview.SpecReviewFinding{...}
```
And update the struct fields to use `specreview.SpecReviewFinding` field types (Severity as `stagepkg.SpecFindingSeverity`, etc.)

Error 2 (line 58): `s.ensureAcceptanceAndReview` → `s.ensureAcceptance`:
```go
// Change from:
if _, err := s.ensureAcceptanceAndReview(ctx, &req, specID); err != nil {
// To:
if _, _, err := s.ensureAcceptance(ctx, &req, specID); err != nil {
```
Note: `ensureAcceptance` returns 3 values: `(*Result, *Result, error)`

Error 3 (line 102): `[]stagepkg.Finding` → `[]finding.Finding`:
```go
// Change from:
findings := []stagepkg.Finding{...}
// To:
findings := []finding.Finding{...}
```
Add import for `"github.com/danabrams/gromit/internal/v2/stage/finding"`

Error 4 (line 204): `stagepkg.SpecFindingCategorySecurity` → `stagepkg.SpecFindingCategorySafety`:
```go
// The constant is SpecFindingCategorySafety, not Security
```

Error 5 (line 237): `stagepkg.SpecFindingCategoryTestGap` → `stagepkg.SpecFindingCategoryQuality`:
```go
// TestGap is not a valid category; use Quality
```

Error 6 (line 246): `stagepkg.SpecFindingCategoryArchitecture` → `stagepkg.SpecFindingCategoryScope`:
```go
// Architecture is not a valid category; use Scope
```

**Step 2: Fix spec_loop_test.go errors**

Error at line 183: `specReviewStage` undefined — check if `newFakeSpecReviewStage` function exists. If it was removed, it needs to be re-added or the test needs updating.

Errors at lines 689, 697, 705: `stagepkg.Result{...}` passed by value to `newScriptedSpecReviewStage` which expects `*stagepkg.Result`:
```go
// Change from:
review := newScriptedSpecReviewStage(stagepkg.Result{...})
// To:
review := newScriptedSpecReviewStage(&stagepkg.Result{...})
```

Also check `newFakeSpecReviewStage` — if it takes values but `newScriptedSpecReviewStage` takes pointers, align them. `newScriptedSpecReviewStage` is defined in `spec_loop_specreview_test.go` (line 344) and takes `...*stagepkg.Result` (pointers). `newFakeSpecReviewStage` needs to be found/fixed similarly.

**Step 3: Verify compilation and tests**

Run: `go test ./internal/v2/loop/... -v -count=1`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/v2/loop/spec_loop_specreview_test.go internal/v2/loop/spec_loop_test.go
git commit -m "fix: correct test type mismatches and method references in spec loop tests"
```

**Acceptance Criteria:**
- All tests in `internal/v2/loop/...` compile and pass
- No references to `ensureAcceptanceAndReview`
- No references to undefined category constants

---

### Task 5: Add gate satisfaction bead-closing logic (Criterion 8)

The gate satisfaction check (`checkSatisfaction` in `satisfaction.go`) returns true when all acceptance criteria are already met, but it doesn't close the bead. The gate stage that calls it needs to close the bead when satisfaction is confirmed.

**Files:**
- Modify: `internal/v2/stage/gate/gate.go` (the gate stage's Run method)
- Test: `internal/v2/stage/gate/gate_test.go`

**Step 1: Read the gate stage to understand the satisfaction check integration**

Read `internal/v2/stage/gate/gate.go` to find where `checkSatisfaction` is called and how the result is used.

**Step 2: Write a failing test**

Add a test that verifies: when `checkSatisfaction` returns true, the gate stage closes the bead via the task tracker's `CloseBead` method.

```go
func TestGateClosesSatisfiedBead(t *testing.T) {
	// Setup: bead with acceptance criteria, satisfaction check returns true
	// Assert: CloseBead was called with the bead's ID
	// Assert: decision is DecisionSkip (or equivalent "already satisfied" signal)
}
```

**Step 3: Run the test to verify it fails**

Run: `go test ./internal/v2/stage/gate/... -run TestGateClosesSatisfiedBead -v`
Expected: FAIL

**Step 4: Add bead-closing logic**

In the gate stage's Run method, after `checkSatisfaction` returns true, call `CloseBead` on the task tracker to close the satisfied bead:

```go
if satisfied {
    if g.tracker != nil {
        _, err := g.tracker.CloseBead(ctx, trackertypes.TaskTrackerCloseBeadRequest{
            ID: req.Bead.ID,
        })
        if err != nil {
            return nil, fmt.Errorf("close satisfied bead %s: %w", req.Bead.ID, err)
        }
    }
    return &stagepkg.Result{Decision: stagepkg.DecisionSkip}, nil
}
```

**Step 5: Run tests**

Run: `go test ./internal/v2/stage/gate/... -v -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add internal/v2/stage/gate/gate.go internal/v2/stage/gate/gate_test.go
git commit -m "feat: gate stage closes beads when satisfaction check confirms criteria are met"
```

**Acceptance Criteria:**
- When `checkSatisfaction` returns true, the bead is closed via `CloseBead`
- Gate returns skip decision for already-satisfied beads
- All gate tests pass

---

### Task 6: Add test proving from-review beads bypass spec-level accept/review (Criterion 13)

The from-review execution path (`run2FromReview` in `run2.go`) calls `BeadLoop.Run` (or `RunWithoutReview`) directly — it never enters `SpecLoop.Run()`, so accept/review stages never execute. This is the correct behavior but needs a test to prove it.

**Files:**
- Modify: `cmd/gromit/run2_test.go` (or create if needed)

**Step 1: Write a test that verifies from-review path skips spec-level accept/review**

```go
func TestFromReviewBeadsSkipSpecLevelAcceptReview(t *testing.T) {
	// Setup: from-review beads, instrumented accept and spec-review stages
	// Execute: run2FromReview (or runFromReview)
	// Assert: accept stage was NOT called
	// Assert: spec review stage was NOT called
	// Assert: bead loop WAS called with the from-review beads
}
```

The key assertion is that `run2FromReview` calls `runBeadLoopFn(components.BeadLoop, ...)` directly and never constructs or calls a `SpecLoop`.

**Step 2: Run the test**

Run: `go test ./cmd/gromit/... -run TestFromReviewBeadsSkipSpecLevelAcceptReview -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/gromit/run2_test.go
git commit -m "test: prove from-review beads bypass spec-level accept/review"
```

**Acceptance Criteria:**
- Test demonstrates from-review path does not invoke accept or spec review stages
- `go test ./cmd/gromit/...` passes

---

### Task 7: Final verification

**Files:** (read-only verification)

**Step 1: Run full build**

Run: `go build ./...`
Expected: PASS

**Step 2: Run full test suite**

Run: `go test ./... -count=1 -timeout=300s`
Expected: ALL PASS

**Step 3: Verify project rules compliance**

Run: `grep -rn 'os\.Chdir' cmd/ internal/ --include='*_test.go'`
Expected: zero hits

**Step 4: Cross-reference gap analysis criteria**

1. **Criterion 1** — `selectModel` defaults to `config.ModelOpus` ✓ (Task 2)
2. **Criterion 3** — `ensureAcceptance` gates both accept and review; no duplicate orchestration ✓ (Task 3)
3. **Criterion 8** — Gate closes beads when satisfaction check passes ✓ (Task 5)
4. **Criterion 9** — Tests compile with correct types; from-review beads labeled correctly ✓ (Task 4)
5. **Criterion 13** — Test proves from-review path bypasses accept/review ✓ (Task 6)

**Acceptance Criteria:**
- `go build ./...` exits 0
- `go test ./...` exits 0
- All 5 gap analysis criteria have corresponding working code and passing tests
