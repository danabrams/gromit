# Spec-Level Review Compilation Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the 4 compilation errors blocking the spec-level review and targeted remediation feature, then address the 6 failed acceptance criteria from the gap analysis.

**Architecture:** The branch has ~277 commits with a partial implementation. Three files have type mismatches or unused variables: `specreview.go` (Category/Scope are typed strings, not plain strings), `accept.go` (runTargetedEvaluation returns 3 values but callers expect 4), `decompose.go` (unused `planText` variable). After compilation fixes, the gap analysis criteria need targeted fixes for duplicate invocation paths and missing gate satisfaction logic.

**Tech Stack:** Go 1.24+, TDD with `go test`

---

## Architecture

The compilation errors fall into three categories:

1. **Type mismatch in specreview.go:396,399** — `stage.Category` and `stage.Scope` are named string types, but `strings.TrimSpace()` requires plain `string`. Fix: cast to `string()` before passing.

2. **Return value mismatch in accept.go** — `runTargetedEvaluation` returns `(results, failures, error)` (3 values) but its body returns 4 values including `findings`, and its caller at line 220 expects 4 values. The function signature needs to match `runPerCriterionEvaluation` which already returns `(results, failures, findings, error)`.

3. **Unused variable in decompose.go:204** — `planText` declared but `planContent` used instead on line 207. Remove the duplicate.

After compilation, 6 gap-analysis criteria need fixes — primarily around ensuring spec-review runs exactly once after bead loop (not 3+ times), and implementing the gate satisfaction check for closing stale beads.

## Test Strategy

- Run `go build ./...` after each fix to verify compilation progress
- Run existing test suites after all compilation fixes: `go test ./internal/v2/stage/specreview/... ./internal/v2/stage/accept/... ./internal/v2/stage/decompose/... ./internal/v2/loop/...`
- For gap-analysis criteria fixes: TDD with existing integration test files

## Implementation Tasks

### Task 1: Fix specreview.go type casts for Category and Scope

**Files:**
- Modify: `internal/v2/stage/specreview/specreview.go:396,399`

**Step 1: Write the failing build check**

Run: `go build ./internal/v2/stage/specreview/...`
Expected: FAIL with `cannot use f.Category (variable of string type stage.Category) as string value`

**Step 2: Fix the type casts**

Change line 396 from:
```go
if trimmed := strings.TrimSpace(f.Category); trimmed != "" {
```
to:
```go
if trimmed := strings.TrimSpace(string(f.Category)); trimmed != "" {
```

Change line 399 from:
```go
if trimmed := strings.TrimSpace(f.Scope); trimmed != "" {
```
to:
```go
if trimmed := strings.TrimSpace(string(f.Scope)); trimmed != "" {
```

**Step 3: Verify build passes for this package**

Run: `go build ./internal/v2/stage/specreview/...`
Expected: PASS (no errors from this package)

**Step 4: Commit**

```bash
git add internal/v2/stage/specreview/specreview.go
git commit -m "fix: cast Category and Scope to string for TrimSpace in specreview"
```

---

### Task 2: Fix accept.go runTargetedEvaluation return signature

**Files:**
- Modify: `internal/v2/stage/accept/accept.go:310,323-325,334,354,361,380`

**Step 1: Write the failing build check**

Run: `go build ./internal/v2/stage/accept/...`
Expected: FAIL with `assignment mismatch: 4 variables but s.runTargetedEvaluation returns 3 values`

**Step 2: Update runTargetedEvaluation signature to return findings**

Change line 310 from:
```go
func (s *Stage) runTargetedEvaluation(ctx context.Context, provider llmtypes.LLMProvider, specID string, criteria []coverage.Criterion, diff string, cfg *config.Config, req *stagepkg.Request) ([]presentation.AcceptanceResult, []string, error) {
```
to:
```go
func (s *Stage) runTargetedEvaluation(ctx context.Context, provider llmtypes.LLMProvider, specID string, criteria []coverage.Criterion, diff string, cfg *config.Config, req *stagepkg.Request) ([]presentation.AcceptanceResult, []string, []stagepkg.Finding, error) {
```

This aligns it with `runPerCriterionEvaluation` at line 258 which already has the 4-return-value signature. The function body already builds and returns `findings` — the signature just needs to declare it.

**Step 3: Fix the caller at line 220**

The caller already expects 4 return values:
```go
results, failures, findings, evalErr := s.runTargetedEvaluation(...)
```
Remove the `declared and not used: findings` error by ensuring `findings` is used in the `AcceptArtifacts` construction at line 225. Check that it already is:
```go
artifacts := &AcceptArtifacts{Results: results, Findings: buildFailureFindings(failures)}
```
If `findings` from `runTargetedEvaluation` is not used, replace `buildFailureFindings(failures)` with `findings` (matching how `runPerCriterionEvaluation`'s findings are used). Or if `buildFailureFindings` is the canonical builder, use `_ = findings` suppression — but check consistency with the per-criterion path first.

Look at lines 238-250 (per-criterion fallback path) to see how findings are handled there, and make the targeted path consistent.

**Step 4: Verify build passes**

Run: `go build ./internal/v2/stage/accept/...`
Expected: PASS

**Step 5: Run accept tests**

Run: `go test ./internal/v2/stage/accept/... -v -count=1`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/v2/stage/accept/accept.go
git commit -m "fix: align runTargetedEvaluation return signature with runPerCriterionEvaluation"
```

---

### Task 3: Fix decompose.go unused planText variable

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go:204`

**Step 1: Write the failing build check**

Run: `go build ./internal/v2/stage/decompose/...`
Expected: FAIL with `declared and not used: planText`

**Step 2: Remove the duplicate variable**

Delete line 204:
```go
planText := string(planBody)
```

Line 207 already has `planContent := string(planBody)` which is the variable actually used in the templates.

**Step 3: Verify build passes**

Run: `go build ./internal/v2/stage/decompose/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/v2/stage/decompose/decompose.go
git commit -m "fix: remove unused planText variable in decompose"
```

---

### Task 4: Verify full compilation

**Files:** None (verification only)

**Step 1: Build entire project**

Run: `go build ./...`
Expected: PASS — zero compilation errors

**Step 2: Run full test suite for affected packages**

Run: `go test ./internal/v2/stage/specreview/... ./internal/v2/stage/accept/... ./internal/v2/stage/decompose/... ./internal/v2/loop/... -count=1 2>&1 | tail -30`
Expected: All tests pass

**Step 3: Commit (if any additional fixes needed)**

Only if step 1 or 2 revealed additional issues.

---

### Task 5: Fix gap criterion 1 — spec-review invoked redundantly in 3-4 places

**Files:**
- Modify: `internal/v2/loop/spec_loop.go` — consolidate spec-review to one call site after bead loop
- Test: `internal/v2/loop/spec_loop_specreview_test.go`

**Step 1: Read spec_loop.go and identify all spec-review invocation sites**

Grep for `specReview`, `spec-review`, `SpecReview`, `runSpecReview` in spec_loop.go. Map each call site. The spec says: "After the bead loop completes... Spec-Level Review evaluates the cumulative diff holistically." There should be exactly ONE call, after bead loop and after accept.

**Step 2: Write a test asserting single invocation**

In `spec_loop_specreview_test.go`, add a test that counts how many times the spec-review stage's `Run` method is called during a full spec loop execution. Use a counting mock. Assert count == 1.

Run: `go test ./internal/v2/loop/... -run TestSpecReviewInvokedOnce -v`
Expected: FAIL (currently invoked 3+ times)

**Step 3: Consolidate to single call site**

Remove duplicate spec-review invocations. Keep only the one in the canonical post-bead-loop, post-accept position. The flow should be: `bead loop → accept → spec-review → (if fail) remediation`.

**Step 4: Run test**

Run: `go test ./internal/v2/loop/... -run TestSpecReviewInvokedOnce -v`
Expected: PASS

**Step 5: Run all loop tests**

Run: `go test ./internal/v2/loop/... -count=1 -v 2>&1 | tail -40`
Expected: All pass

**Step 6: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_specreview_test.go
git commit -m "fix: consolidate spec-review to single invocation after bead loop"
```

---

### Task 6: Fix gap criterion 3 — spec succeeds only when both accept AND review pass

**Files:**
- Modify: `internal/v2/loop/spec_loop.go` — gating logic combining both verdicts
- Test: `internal/v2/loop/spec_loop_specreview_test.go`

**Step 1: Write test for combined gating**

Test cases:
- accept=pass, review=pass → spec succeeds
- accept=pass, review=fail → spec fails, triggers remediation
- accept=fail, review=not-run → spec fails, triggers remediation
- accept=fail, review=fail → spec fails, both findings passed to remediation

**Step 2: Run tests to verify failure**

Run: `go test ./internal/v2/loop/... -run TestSpecGatingCombined -v`
Expected: FAIL

**Step 3: Implement combined gating logic**

In spec_loop.go, after accept and spec-review both run, combine their decisions:
```go
specPasses := acceptResult.Decision == DecisionProceed && reviewResult.Decision == DecisionProceed
if !specPasses {
    findings := collectFindings(acceptResult, reviewResult)
    return s.runRemediation(ctx, specID, worktree, findings)
}
```

**Step 4: Run tests**

Run: `go test ./internal/v2/loop/... -run TestSpecGatingCombined -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_specreview_test.go
git commit -m "fix: gate spec success on both accept pass and review pass"
```

---

### Task 7: Fix gap criterion 6 — findings become input to remediation decompose

**Files:**
- Modify: `internal/v2/loop/spec_loop.go` — collect findings from both accept and review
- Modify: `internal/v2/remediation/remediation.go` — receive and forward findings
- Test: `internal/v2/loop/spec_loop_specreview_integration_test.go`

**Step 1: Write integration test**

Test that when accept fails with criterion findings and review fails with code findings, the remediation decompose stage receives ALL findings combined.

**Step 2: Run test**

Run: `go test ./internal/v2/loop/... -run TestRemediationReceivesCombinedFindings -v`
Expected: FAIL

**Step 3: Implement findings collection and forwarding**

Ensure the remediation runner's `Run` method receives the combined findings list and passes it through to the decompose stage's request as `req.Findings`. Remove any path that falls back to re-decomposing the original plan when findings are available.

**Step 4: Run test**

Expected: PASS

**Step 5: Run full integration suite**

Run: `go test ./internal/v2/loop/... -count=1 -v 2>&1 | tail -40`
Expected: All pass

**Step 6: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/remediation/remediation.go internal/v2/loop/spec_loop_specreview_integration_test.go
git commit -m "fix: forward combined accept+review findings to remediation decompose"
```

---

### Task 8: Fix gap criterion 8 — gate satisfaction check closes stale open beads

**Files:**
- Modify: `internal/v2/loop/spec_loop.go` or `internal/v2/stage/gate/satisfaction.go`
- Test: `internal/v2/loop/spec_loop_specreview_integration_test.go`

**Step 1: Write test for stale bead closure**

Test that when a bead's acceptance criteria are already satisfied by the cumulative diff (DiffFromBase), the gate satisfaction check closes it instead of rebuilding.

**Step 2: Run test**

Expected: FAIL

**Step 3: Implement stale bead closure**

At gate time (before the bead loop or during remediation), query open beads and run `checkSatisfaction` against DiffFromBase. Close beads that pass. This uses the existing `satisfaction.go` infrastructure — wire it into the spec loop or remediation runner at the point where beads are queried for the next generation.

**Step 4: Run test**

Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/stage/gate/satisfaction.go internal/v2/loop/spec_loop_specreview_integration_test.go
git commit -m "fix: gate satisfaction check closes open beads already satisfied by cumulative diff"
```

---

### Task 9: Fix gap criteria 9 & 10 — from-review bead labeling (compilation-blocked)

**Files:**
- Test: `internal/v2/stage/specreview/specreview_test.go`

**Step 1: Verify from-review labeling tests pass after compilation fixes**

The gap analysis says the labeling logic is "correct in intent" but blocked by compilation errors. After Tasks 1-4 fix compilation, run:

Run: `go test ./internal/v2/stage/specreview/... -v -count=1`
Expected: All tests pass, including from-review bead creation tests

**Step 2: If tests fail, debug and fix**

Check that:
- Spec-scoped findings get labels `["from-review", "spec:<spec-id>"]`
- General findings get labels `["from-review"]` only
- The `createFromReviewBeads` function uses `f.Scope == ScopeSpec` (or string comparison `"spec"`) correctly

**Step 3: Run integration tests**

Run: `go test ./internal/v2/loop/... -run Integration -v -count=1`
Expected: All pass

**Step 4: Commit (if changes needed)**

```bash
git add internal/v2/stage/specreview/specreview.go internal/v2/stage/specreview/specreview_test.go
git commit -m "fix: from-review bead labeling for spec-scoped and general findings"
```

---

### Task 10: Final verification — all acceptance criteria

**Files:** None (verification only)

**Step 1: Full build**

Run: `go build ./...`
Expected: PASS

**Step 2: Full test suite**

Run: `go test ./internal/v2/... ./cmd/gromit/... -count=1 2>&1 | tail -50`
Expected: All pass

**Step 3: Verify each gap-analysis criterion manually**

Walk through each of the 6 failed criteria and confirm the fix addresses it:
1. Spec-review invoked once after bead loop ✓
2. (Was passing) ✓
3. Spec succeeds only when both accept and review pass ✓
4. (Was passing) ✓
5. (Was passing) ✓
6. Findings from both become remediation input ✓
7. (Was passing) ✓
8. Gate satisfaction closes stale open beads ✓
9. From-review spec-scoped labeling works ✓
10. From-review general labeling works ✓

**Step 4: Commit verification notes if needed**
