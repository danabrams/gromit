# Immutable Plan with Additive Remediation

## Problem

The v2 spec loop replans and redecomposes a spec every time it runs. When a user cancels and restarts, the loop discards prior work and generates a fresh plan. When acceptance fails, the remediation runner redecomposes the entire spec instead of creating targeted fix beads. Both behaviors waste LLM invocations and destroy continuity.

### Root Causes

1. **`queryExistingBeads` filters on `Status: "open"`** (`spec_loop.go:458`). Once all beads close, the query returns nothing, and the loop redecomposes from scratch.

2. **`RemediationRunner.executeRemediation` calls decompose directly** (`remediation.go:126`). It creates an entirely new decomposition each cycle instead of adding targeted beads for unmet acceptance criteria.

## Design

### Core Invariant

Plan and decompose run exactly once per spec. After that, the plan is immutable. All subsequent work creates new additive beads — never replans.

### Change 1: Resume-safe decompose guard

`queryExistingBeads` currently filters `Status: "open"`. Change it to query beads with the spec label in **any** status. If any beads have ever been created for this spec, skip decompose entirely.

The bead loop still receives only open beads. The guard prevents re-decomposition; it does not change which beads execute.

### Change 2: Additive remediation via separate plan-decompose cycle

When acceptance fails, the remediation runner:

1. Runs gap analysis (already happens).
2. Creates a **remediation plan** from the gap analysis — a new, focused plan addressing only the gaps.
3. Decomposes that remediation plan into beads tagged with the original `spec:` label.
4. Persists the remediation plan as `remediation-N.md` (where N increments per generation).
5. Runs the bead loop on the new beads.

The original `plan.md` stays untouched. Each remediation cycle produces its own plan file.

### Change 3: Resume goes straight to the bead loop

On restart, the spec loop:

1. Checks for an existing plan file — uses it if present (already works).
2. Checks whether beads have ever been created for this spec — if yes, skips decompose, collects open beads, runs the bead loop.
3. Plans and decomposes only when no prior work exists.

### Files to Change

| File | Change |
|------|--------|
| `internal/v2/loop/spec_loop.go` | `queryExistingBeads`: query all statuses, not just open. Add separate open-bead filter for the bead loop. |
| `internal/v2/remediation/remediation.go` | Replace direct decompose call with plan → decompose → tag cycle. Persist remediation plans as `remediation-N.md`. |
| `internal/v2/loop/spec_loop.go` | Pass worktree/paths context to remediation runner for plan persistence. |

## Verification & Testing Plan

### Unit Tests — `spec_loop_test.go`

**Test 1: `TestSpecLoop_ResumeSkipsDecomposeWhenClosedBeadsExist`**
Setup: Plan file on disk. TaskTracker returns beads with `Status: "closed"` for the spec label.
Assert: `decomposeStage.called == false`. Bead loop receives zero beads. Accept stage runs.

**Test 2: `TestSpecLoop_ResumeSkipsDecomposeWhenMixedStatusBeadsExist`**
Setup: Plan file exists. TaskTracker returns 2 closed beads + 1 open bead.
Assert: `decomposeStage.called == false`. Bead loop receives only the 1 open bead.

**Test 3: `TestSpecLoop_FirstRunDecomposesWhenNoBeadsExist`**
Setup: No plan file, no beads in tracker.
Assert: `planStage.called == true`, `decomposeStage.called == true`. Existing behavior preserved.

**Test 4: `TestSpecLoop_ResumeSkipsPlanWhenPlanFileExists`**
Explicit regression test with new query changes to confirm plan resume still works.

### Unit Tests — `remediation_test.go`

**Test 5: `TestRemediation_CreatesRemediationPlanNotOriginal`**
Setup: Accept fails once, then passes.
Assert: Remediation calls plan stage with gap analysis context. Decompose receives the remediation plan.

**Test 6: `TestRemediation_RemediationBeadsCarrySpecLabel`**
Setup: Accept fails. Remediation creates beads.
Assert: All created beads have `spec:SPECID` label.

**Test 7: `TestRemediation_RemediationPlanPersistedSeparately`**
Setup: Accept fails, remediation runs.
Assert: `remediation-1.md` exists. Original `plan.md` unchanged.

**Test 8: `TestRemediation_SecondRemediationCreatesRemediationPlan2`**
Setup: Accept fails twice.
Assert: Both `remediation-1.md` and `remediation-2.md` exist. `plan.md` unchanged.

**Test 9: `TestRemediation_GenerationCapStillRespected`**
Regression test — cap of N still limits remediation cycles.

### Integration Tests — `integration_test.go`

**Test 10: `TestIntegration_CancelAndResumeSkipsDecompose`**
Run spec, cancel after bead loop starts. Resume with same spec ID.
Assert: Plan stage skipped. Decompose stage skipped. Bead loop runs with remaining open beads.

**Test 11: `TestIntegration_AcceptFailureRemediationAddsBeads`**
Accept fails first time. Remediation creates new beads. Accept passes second time.
Assert: Original beads untouched. New remediation beads carry same spec label.

**Test 12: `TestIntegration_FullCycleWithRemediationAndResume`**
Run spec → accept fails → remediation creates beads → cancel mid-remediation → resume → bead loop picks up remaining open beads → accept passes.

### Manual Verification

```
go test ./internal/v2/loop/...
go test ./internal/v2/remediation/...
```
