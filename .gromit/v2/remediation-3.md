Now I have a complete understanding. Let me generate the implementation plan.

---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Fix the two remaining acceptance criteria gaps — the combined accept+review gating logic (criterion 3) and the gate satisfaction check visibility in the diff (criterion 8).
**Architecture:** Most of the spec is already implemented across 52 changed files. Two criteria remain failing in gap analysis: criterion 3 (verdict gating) has a terminology mismatch where `specReviewVerdictFailed` checks for `"fail"` but the spec-review stage emits `"issue"`; criterion 8 (gate satisfaction) has implementation on main but the test added in this branch references implementation not visible in the diff.
**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

The spec-level review stage (`internal/v2/stage/specreview/`) is fully implemented and wired. The spec loop calls `ensureAcceptance()` which runs both accept and spec-review stages, triggers remediation on failure, and creates from-review beads on pass-with-improvements. The `--from-review` flag, findings-based decompose, and structured accept output are all working.

**Criterion 3 gap:** `specReviewVerdictFailed()` in `spec_loop.go:802` checks `strings.EqualFold(verdict, "fail")` but the spec-review stage sets verdict to `"issue"` (not `"fail"`) via `verdictFromFindings()` at `specreview.go:110`. The behavior is accidentally correct because `decisionFromArtifacts()` converts `"issue"` to `DecisionFail`, and `specReviewFailed()` checks `Decision` first. But the verdict-level check is dead code — it can never trigger. The spec says `"verdict: pass | fail"` so either the stage should emit `"fail"` or the check should match `"issue"`.

**Criterion 8 gap:** `WithSatisfactionCheck` and the `CloseBead` call in `gate.go` were implemented in a prior feature branch already merged to main. This branch adds `TestGateClosesSatisfiedBead` which validates the behavior, but the gap analysis (evaluating only the diff) can't see the implementation. This may require either: (a) a no-op acknowledgment that the feature is satisfied via the dependency chain, or (b) wiring the satisfaction check into the spec loop's remediation path if not already done.

## Test Strategy

- Unit test for verdict normalization: confirm `"issue"` verdicts are treated as failures
- Unit test confirming `specReviewVerdictFailed` matches the actual verdict values emitted by the stage
- Integration test: full accept → spec-review → gating path where accept passes but review fails, confirming spec does not succeed
- Verify existing `TestGateClosesSatisfiedBead` passes with the mainline gate implementation

## Implementation Tasks

### Task 1: Normalize spec-review verdict to match spec contract
**Files:** `internal/v2/stage/specreview/specreview.go`, `internal/v2/stage/specreview/specreview_test.go`
**What to Do:** Change `verdictFromFindings()` to return `"fail"` instead of `"issue"` when any finding has verdict `"issue"`. The stage should emit verdicts matching the spec contract (`"pass"` or `"fail"`). Update `decisionFromArtifacts()` to check for `"fail"` instead of `"issue"` on the overall verdict. Keep individual finding-level verdict values as `"issue"` (they come from the LLM) but normalize the aggregate `SpecReviewArtifacts.Verdict` to `"pass"` or `"fail"`. Update tests that assert `verdict == "issue"` to assert `verdict == "fail"`.
**Acceptance Criteria:**
- `verdictFromFindings()` returns `"fail"` (not `"issue"`) when any finding has verdict `"issue"`
- `decisionFromArtifacts()` checks for `"fail"` on the aggregate verdict
- `specReviewVerdictFailed()` in `spec_loop.go` now correctly matches the emitted verdict value
**Dependencies:** None

### Task 2: Add integration test for combined accept+review gating
**Files:** `internal/v2/loop/spec_loop_specreview_test.go` or `internal/v2/loop/spec_loop_test.go`
**What to Do:** Add a test case that verifies the spec fails when accept passes but spec-review returns a `"fail"` verdict (after Task 1's normalization). This test should confirm that `ensureAcceptance()` returns the combined failure, remediation is triggered (or error returned if no remediation runner), and the spec does not proceed to present. If such a test already exists (check `TestSpecLoopPostBeadPipelineSpecReviewFailureSkipsPresent`), verify it covers the verdict-level failure path specifically and add coverage if missing.
**Acceptance Criteria:**
- Test proves spec fails when accept passes and spec-review verdict is `"fail"`
- Test proves remediation is triggered on combined accept-pass + review-fail
**Dependencies:** Task 1

### Task 3: Verify gate satisfaction check is wired and passing
**Files:** `internal/v2/stage/gate/gate_test.go`, `internal/v2/loop/run2_components.go`
**What to Do:** Run the existing `TestGateClosesSatisfiedBead` test to confirm it passes against the mainline gate implementation. Verify that `WithSatisfactionCheck` is wired in `run2_components.go` (check the gate stage creation). If the wiring is missing in this branch's components, add it. If it's already wired (from main), confirm the test passes and document why criterion 8 is satisfied despite not appearing in the diff.
**Acceptance Criteria:**
- `TestGateClosesSatisfiedBead` passes
- `WithSatisfactionCheck` is wired in the run2 components gate stage creation
**Dependencies:** None