# Spec-Level Review and Targeted Remediation — Remediation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all compilation errors in the partially-implemented spec-level review and targeted remediation feature so the code compiles, tests pass, and acceptance criteria are satisfied.

**Architecture:** The feature is partially implemented across ~30 commits but accumulated compilation errors from overlapping bead iterations. Each task fixes one compilation error cluster, progressing from leaf packages (specreview, accept) to orchestration (spec_loop) to wiring (run2_components, run2).

**Tech Stack:** Go 1.24, cobra CLI, TDD with `go test`

---

## Architecture

### Root Causes of Compilation Failures

1. **Duplicate code from conflicting bead generations:** `spec_loop.go` has two `specReviewStage` struct fields (lines 218 and 234), two `runSpecReviewStage` methods (lines 722 and 757), and duplicated spec-review orchestration in `Run()` (lines 446-512 duplicate the work done by `ensureAcceptance` at line 431).

2. **Removed methods still referenced:** `accept.go` removed `runBatchEvaluation`, `runTargetedEvaluation`, `batchDiffThreshold` constant, and helpers (`splitDiffByFile`, `mapCriteriaToFiles`, `buildTargetedDiff`) — but `Run()` and `diffThreshold()` still call them.

3. **Type mismatches:** `buildFailureFindings` returns `[]stage.Finding` but `AcceptArtifacts.Findings` expects `[]stage.SpecFinding`. `runPerCriterionEvaluation` body returns `(*stagepkg.Result, error)` but signature is `([]AcceptanceResult, []string, error)`.

4. **Orphaned early-generation code:** `specreview.go` has functions referencing undefined `rawFinding` type (lines 382-403), and a duplicate `createFromReviewBeads` method (lines 338 and 405).

5. **Duplicate variable declarations:** `run2.go` declares `specsDir` twice (lines 91, 102). `run2_components.go` loads `specReviewFragment` twice (lines 107, 223) and creates `specReviewStage` twice (lines 177-186, 229-237).

### Fix Strategy

Fix in dependency order — leaf packages first, then orchestration, then wiring. Run `go build` after each task to confirm progress.

## Test Strategy

- After each task: `go build ./path/to/package/...` to verify compilation
- After specreview fix: `go test ./internal/v2/stage/specreview/...`
- After accept fix: `go test ./internal/v2/stage/accept/...`
- After spec_loop fix: `go test ./internal/v2/loop/...`
- After run2 fixes: `go test ./cmd/gromit/...`
- Final: `go test ./...` to confirm everything compiles and passes

---

## Implementation Tasks

### Task 1: Remove orphaned `rawFinding` functions and duplicate `createFromReviewBeads` from specreview.go

**Files:**
- Modify: `internal/v2/stage/specreview/specreview.go`
- Test: `internal/v2/stage/specreview/specreview_test.go`

**Step 1: Read the file to confirm orphaned functions**

Read `internal/v2/stage/specreview/specreview.go` lines 382-466.

**Step 2: Delete orphaned functions**

Remove `computeVerdict` (lines 382-389) and `convertFindings` (lines 391-403) — both reference undefined `rawFinding` type. The working implementations are `verdictFromFindings` (line 110) and `parseSpecReviewOutput` (line 52).

**Step 3: Delete the duplicate `createFromReviewBeads` method**

Remove the second `createFromReviewBeads` definition (lines 405-429). It has a different signature `(ctx, specID string, findings []stagepkg.Finding) ([]*trackertypes.Bead, error)` from the first (lines 338-380) which has signature `(ctx, specID string, artifacts *SpecReviewArtifacts, decision stagepkg.Decision) error`. The first is called by `Run()` at line 223 and is correct.

Keep the helper functions `labelsForFinding` (431-439), `beadTitleForFinding` (441-452), `priorityForSeverity` (454-465) — they may be useful but aren't causing compilation errors.

**Step 4: Verify compilation**

Run: `go build ./internal/v2/stage/specreview/...`
Expected: PASS

**Step 5: Run tests**

Run: `go test ./internal/v2/stage/specreview/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/v2/stage/specreview/specreview.go
git commit -m "fix: remove duplicate createFromReviewBeads and orphaned rawFinding functions from specreview"
```

**Acceptance Criteria:**
- `go build ./internal/v2/stage/specreview/...` succeeds
- `go test ./internal/v2/stage/specreview/...` passes
- No references to `rawFinding` remain

---

### Task 2: Fix accept.go — restore `batchDiffThreshold` constant and removed methods

**Files:**
- Modify: `internal/v2/stage/accept/accept.go`
- Test: `internal/v2/stage/accept/accept_test.go`

**Step 1: Restore the `batchDiffThreshold` constant**

Add to the const block (after `outputPreviewMaxLen`):
```go
batchDiffThreshold = 50000 // ~12K tokens
```
This is referenced by `diffThreshold()` at line 114.

**Step 2: Restore removed methods from main branch**

Run `git show main:internal/v2/stage/accept/accept.go` to extract the following methods that were removed but are still called by `Run()`:
- `runBatchEvaluation`
- `runTargetedEvaluation`
- `mapCriteriaToFiles`
- `splitDiffByFile`
- `buildTargetedDiff`
- `parseBatchEvaluation`
- `parseCriteriaMapping`
- `buildBatchPrompt`

Copy them back into `accept.go`. Ensure they compile with the current imports and types.

**Step 3: Fix `buildFailureFindings` return type**

`AcceptArtifacts.Findings` is `[]stagepkg.SpecFinding` but `buildFailureFindings` returns `[]stagepkg.Finding`. Change `buildFailureFindings` to return `[]stagepkg.SpecFinding`:

```go
func buildFailureFindings(failures []string) []stagepkg.SpecFinding {
    if len(failures) == 0 {
        return nil
    }
    findings := make([]stagepkg.SpecFinding, 0, len(failures))
    for _, failure := range failures {
        findings = append(findings, stagepkg.SpecFinding{
            Title:       failure,
            Description: failure,
            Severity:    stagepkg.SpecFindingSeverityCritical,
            Category:    stagepkg.SpecFindingCategoryAcceptance,
            Scope:       stagepkg.SpecFindingScopeSpec,
        })
    }
    return findings
}
```

**Step 4: Fix `runPerCriterionEvaluation` return**

The method signature says `([]presentation.AcceptanceResult, []string, error)` but lines 310-319 try to return `(*stagepkg.Result, error)`. Also references undefined `root` variable and has wrong return arity.

Replace lines 310-319 with:
```go
    return results, failures, nil
```

Remove the `findings` variable (line 257) and the findings-append block (lines 293-299) since `buildFailureFindings` in the `Run()` caller handles this from the `failures` slice.

**Step 5: Fix targeted evaluation indentation**

Lines 216-231 have broken indentation (de-indented from inside the `if threshold > 0` block). Re-indent to be properly nested inside the `if` block.

**Step 6: Update restored batch/targeted methods**

The restored methods from main may use old `AcceptArtifacts` without `Findings` field. Update them to include `Findings: buildFailureFindings(failures)` in the artifacts they return, consistent with the per-criterion path.

**Step 7: Verify compilation and tests**

Run: `go build ./internal/v2/stage/accept/...`
Run: `go test ./internal/v2/stage/accept/... -v`

**Step 8: Commit**

```bash
git add internal/v2/stage/accept/accept.go
git commit -m "fix: restore batch/targeted evaluation methods and fix type mismatches in accept stage"
```

**Acceptance Criteria:**
- `go build ./internal/v2/stage/accept/...` succeeds
- `go test ./internal/v2/stage/accept/...` passes
- `buildFailureFindings` returns `[]stagepkg.SpecFinding`
- `runPerCriterionEvaluation` returns `(results, failures, nil)` matching its declared signature
- `batchDiffThreshold` constant exists and `diffThreshold()` compiles

---

### Task 3: Fix spec_loop.go — remove duplicate struct field, method, and orchestration

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`
- Test: `internal/v2/loop/spec_loop_test.go`

**Step 1: Remove duplicate `specReviewStage` struct field**

Line 234 duplicates line 218. Delete line 234 (`specReviewStage stagepkg.Stage`).

**Step 2: Remove duplicate `runSpecReviewStage` method**

Lines 757-766 duplicate lines 722-734. The version at 722 is correct (returns `DecisionProceed` result when stage is nil, wraps errors, checks nil result). Delete lines 757-766.

**Step 3: Remove duplicated spec-review orchestration in `Run()`**

After `ensureAcceptance` (line 431), the `Run()` method has:
- Lines 446-452: A spec-review check using `specReviewRes` from `ensureAcceptance` — this is correct, keep it.
- Lines 456-512: A SECOND spec-review block that calls `runSpecReview` and `runSpecReviewStage` again, handles findings/remediation — this duplicates what `ensureAcceptance` already does. Delete lines 456-512.

The correct flow after `ensureAcceptance` returns:
1. Check `acceptRes` passed (already at line 439)
2. Check `specReviewRes` passed (already at lines 447-449)
3. Handle from-review beads if spec review created them (keep lines 504-508 logic but move to after line 452)
4. Commit, present, cleanup

**Step 4: Verify that package-level helper functions are still used**

After removing the duplicate orchestration, check whether these functions are still referenced:
- `mergeFindings` (line 932) — used by the now-deleted block; may be dead code. Check if `ensureAcceptance` uses it.
- `extractAcceptFindings` package-level (line 921) — used by `mergeFindings`
- `extractSpecReviewFindings` package-level (line 789) — used by `mergeFindings` and the from-review bead path
- `createFromReviewBeads` method (line 944) — still needed for from-review bead creation

If `mergeFindings` and `extractAcceptFindings` (package-level) are now dead code, remove them.

**Step 5: Verify compilation**

Run: `go build ./internal/v2/loop/...`

**Step 6: Run tests**

Run: `go test ./internal/v2/loop/... -v -count=1`

**Step 7: Commit**

```bash
git add internal/v2/loop/spec_loop.go
git commit -m "fix: remove duplicate specReviewStage field, method, and orchestration block in spec_loop"
```

**Acceptance Criteria:**
- No duplicate struct fields or method definitions
- `ensureAcceptance` is the single point of accept+review orchestration
- `go build ./internal/v2/loop/...` succeeds
- `go test ./internal/v2/loop/...` passes

---

### Task 4: Fix run2_components.go — remove duplicate fragment load and stage creation

**Files:**
- Modify: `internal/v2/loop/run2_components.go`

**Step 1: Remove duplicate `specReviewFragment` load**

Lines 223-227 load `review_spec_v2.md` again (first load is at lines 107-111). Delete lines 223-227.

**Step 2: Remove the first conditional `specReviewStage` creation**

Lines 177-186 conditionally create `specReviewStage` via router. Lines 229-237 unconditionally create it and wrap with `newTieredSpecReviewStage`. The unconditional version is correct — `newTieredSpecReviewStage` already handles nil router. Delete lines 177-186.

**Step 3: Fix the re-declaration of `specReviewStage`**

Line 236 (`var specReviewStage stagepkg.Stage = specReviewStageBase`) re-declares a variable that was defined at line 177 (now deleted). Change to simple assignment:
```go
specReviewStage := newTieredSpecReviewStage(specReviewStageBase, router)
```
This becomes the only declaration of `specReviewStage`, used in the return struct at line 256.

**Step 4: Check for `WithTypedEmitter` on specreview.Stage**

Line 234 calls `specReviewStageBase = specReviewStageBase.WithTypedEmitter(typedEmitter)`. Check if `specreview.Stage` has a `WithTypedEmitter` method. If not, either add it or remove this call (the typed emitter may not be needed by the spec review stage).

**Step 5: Verify compilation and tests**

Run: `go build ./internal/v2/loop/...`
Run: `go test ./internal/v2/loop/... -v -count=1`

**Step 6: Commit**

```bash
git add internal/v2/loop/run2_components.go
git commit -m "fix: remove duplicate specReviewFragment load and specReviewStage creation in run2_components"
```

**Acceptance Criteria:**
- `specReviewFragment` loaded exactly once
- `specReviewStage` created exactly once with tiered wrapper
- `go build ./internal/v2/loop/...` succeeds

---

### Task 5: Fix run2.go — remove duplicate `specsDir`, duplicate `WithSpecReviewStage`, and dead code

**Files:**
- Modify: `cmd/gromit/run2.go`
- Test: `cmd/gromit/run2_test.go`

**Step 1: Remove duplicate `specsDir` and dead code block**

Lines 102-109 are a second `specsDir := resolveSpecsDir(cfg)` plus a dead code block from an earlier from-review implementation. Line 91 already declares `specsDir`. Delete lines 102-109.

**Step 2: Remove duplicate `WithSpecReviewStage`**

Lines 211 and 213 both add `loop.WithSpecReviewStage(components.SpecReviewStage)` to `baseOpts`. Remove line 213 (the duplicate).

**Step 3: Remove unreachable `runFromReview` call**

Lines 200-202 call `runFromReview` when `fromReview` is true, but `fromReview` causes early return at line 88 (calling `run2FromReview`). Lines 200-202 are unreachable. Delete them.

**Step 4: Verify compilation and tests**

Run: `go build ./cmd/gromit/...`
Run: `go test ./cmd/gromit/... -v -count=1`

**Step 5: Commit**

```bash
git add cmd/gromit/run2.go
git commit -m "fix: remove duplicate specsDir, duplicate WithSpecReviewStage, and unreachable from-review code in run2"
```

**Acceptance Criteria:**
- No duplicate variable declarations
- `WithSpecReviewStage` appears exactly once in `baseOpts`
- `go build ./cmd/gromit/...` succeeds
- `go test ./cmd/gromit/...` passes

---

### Task 6: Fix remaining compilation errors and test failures

**Files:**
- Any files with secondary compilation issues discovered after Tasks 1-5

**Step 1: Run full build**

Run: `go build ./...`
If errors remain, fix each one:
- Missing `WithTypedEmitter` method on specreview.Stage (add it or remove the call)
- Import alias conflicts in test files
- Interface mismatches after method signature changes
- Test files referencing removed/renamed functions

**Step 2: Run full test suite**

Run: `go test ./... -count=1 -timeout=300s`
Fix any test failures:
- Tests that inject `AcceptStage` into `RemediationRunnerConfig` (remove those injections)
- Tests that assert accept is called by the remediation runner (update expectations)
- Tests that use old `remediationRunner.Run` signature (update to new signature with findings param)
- Tests with duplicate import aliases

**Step 3: Verify project rules compliance**

Run: `grep -rn 'os\.Chdir' cmd/ internal/ --include='*_test.go'` — must return zero hits
Check that no package-level injectable vars are overridden in tests without `t.Cleanup` restoration.

**Step 4: Commit**

```bash
git add -u
git commit -m "fix: resolve remaining compilation errors and test failures"
```

**Acceptance Criteria:**
- `go build ./...` exits 0
- `go test ./...` exits 0
- `grep -rn 'os\.Chdir' cmd/ internal/ --include='*_test.go'` returns zero hits

---

### Task 7: Verify acceptance criteria against gap analysis

**Files:** (read-only verification)

**Step 1: Verify each gap analysis criterion**

Cross-reference each criterion from `.gromit/v2/gap-analysis.md`:

1. **Spec review evaluates cumulative diff with highest-tier model** — Verify `newTieredSpecReviewStage` wraps with `routing.TierHigh` in `run2_components.go`
2. **Structured findings with severity/category/scope/description/affected_files** — Verify `SpecReviewFinding` type and `parseSpecReviewOutput` in specreview.go
3. **Spec succeeds only when both accept and review pass** — Verify `ensureAcceptance` checks both in spec_loop.go
4. **Critical findings force fail verdict** — Verify `decisionFromArtifacts` returns `DecisionFail` for critical severity
5. **Warning/suggestion findings allow pass** — Verify `decisionFromArtifacts` returns `DecisionProceed` otherwise
6. **Findings become remediation input** — Verify `ensureAcceptance` passes merged findings to `remediationRunner.Run`
7. **Remediation creates targeted beads from findings** — Verify `selectTemplate` in decompose.go uses `templateKindFindings` when `req.Findings` is populated
8. **Gate satisfaction closes satisfied beads** — Verify gate/satisfaction.go exists (separate feature, pre-existing)
9. **Spec-scoped from-review beads labeled with spec** — Verify `createFromReviewBeads` in specreview.go adds spec label
10. **General findings without spec label** — Verify `labelsForFinding` omits spec label for non-spec scope
11. **`run2 --from-review` runs from-review beads** — Verify flag exists, queries by label, runs bead loop
12. **From-review beads don't trigger accept/review** — Verify `RunWithoutReview` skips review stage

**Step 2: Run final test suite**

Run: `go test ./... -count=1 -timeout=300s`

**Acceptance Criteria:**
- All tests pass
- Code compiles cleanly
- Each gap analysis criterion has corresponding working code paths
