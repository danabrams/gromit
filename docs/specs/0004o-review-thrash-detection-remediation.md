## spec_id

0004o-review-thrash-detection-remediation

## Depends on

0004o-review-thrash-detection

## Summary

Cleanup items from the 0004o-review-thrash-detection review: 23 findings across 5 categories.

## Goals

- Fix all non-blocking review findings

## Non-goals

- No behavior changes

## Architecture

### cmd/gromit-next/exec_scenario_escalation_fails_run_blocked_test.go

**Line 173:** `architecture_drift` (architecture_drift)

Scenario test forces `Blocked` by rewriting the review action in `blockedRunCaptureReviewStage` when `rs.Cycle >= 3`, so it bypasses the real review-stage thrash-threshold path.

**Line 174:** `code_quality` (code_quality)

The scenario forces `Blocked` by mutating the review action in test scaffolding (`blockedRunCaptureReviewStage`) when cycle >= 3, so it does not validate the real thrash-threshold blocking path end-to-end.

**Line 171:** `logic_gaps` (logic_gaps)

The test forces `Blocked` by rewriting a `ReplanFrom` review action when `rs.Cycle >= 3`, so it does not validate the real review-stage thrash threshold path end-to-end.

**Line 173:** `spec_alignment` (spec_alignment)

The scenario forces a blocked outcome by rewriting a `ReplanFrom` review action to `Blocked` in test scaffolding (`blockedRunCaptureReviewStage`), so it does not validate the real review-stage thrash-threshold path (count >= 3) end-to-end.

**Line 171:** `test_coverage` (test_coverage)

The blocked outcome is forced by test scaffolding (`blockedRunCaptureReviewStage` rewrites `ReplanFrom` to `Blocked` at cycle >= 3), so the real thrash-threshold blocking path is not actually validated end-to-end.

### cmd/gromit-next/exec_scenario_review_thrash_escalation_test.go

**Line 95:** `architecture_drift` (architecture_drift)

Core escalation assertion is too loose (`len(thrashRuns) < 3`), allowing extra unexpected executions/cycle drift without failing the test.

**Line 96:** `code_quality` (code_quality)

Core escalation assertion is too loose (`len(thrashRuns) < 3`), so extra unexpected executions/cycle drift can still pass and mask regressions.

**Line 96:** `logic_gaps` (logic_gaps)

Core escalation assertion is too loose (`len(thrashRuns) < 3`), so extra unexpected executions or cycle drift can still pass without failing the test.

**Line 99:** `spec_alignment` (spec_alignment)

Core escalation assertion is too loose: `len(thrashRuns) < 3` allows extra unexpected executions/cycle drift to pass, weakening validation of targeted escalation timing.

**Line 99:** `test_coverage` (test_coverage)

Core escalation assertion is too loose (`len(thrashRuns) < 3`), which can pass even with extra/unexpected executions or cycle drift.

### docs/specs/0004p-criterion-eval-structured-output.md

**Line 1:** `architecture_drift` (architecture_drift)

The change set deletes multiple unrelated architecture/spec documents (`0004p`, `0004p2`, `0004q`, `0004q2`, `0004r`, `0004r2`) while implementing spec `0004o`, causing out-of-scope structural drift and traceability loss.

**Line 1:** `code_quality` (code_quality)

This change set deletes multiple unrelated spec documents (`0004p`, `0004p2`, `0004q`, `0004q2`, `0004r`, `0004r2`) while implementing review-thrash detection (`0004o`), causing out-of-scope churn and traceability loss.

**Line 1:** `logic_gaps` (logic_gaps)

This change set includes deletion of multiple unrelated spec documents (`0004p`, `0004p2`, `0004q`, `0004q2`, `0004r`, `0004r2`) while implementing `0004o`, creating out-of-scope churn and traceability risk.

**Line 1:** `spec_alignment` (spec_alignment)

The change set deletes multiple unrelated spec documents (`0004p`, `0004p2`, `0004q`, `0004q2`, `0004r`, `0004r2`) while implementing spec `0004o`, creating out-of-scope churn and traceability loss for this facet.

### internal/next/specloop/stages/execute.go

**Line 220:** `architecture_drift` (architecture_drift)

Dead/duplicated escalation logic remains: local helper `taskIntersectsEscalated` duplicates `specloop.TaskIntersectsEscalated` but is not used by `Run`, creating contract-drift risk.

**Line 221:** `code_quality` (code_quality)

Dead/duplicated logic remains: local helper `taskIntersectsEscalated` duplicates `specloop.TaskIntersectsEscalated`, but `Run` calls the shared helper instead. This creates maintenance drift risk.

**Line 221:** `logic_gaps` (logic_gaps)

Dead/duplicated escalation matcher remains: local `taskIntersectsEscalated` duplicates `specloop.TaskIntersectsEscalated`, but `Run` uses the shared helper instead. This creates contract drift risk if one implementation changes.

**Line 221:** `spec_alignment` (spec_alignment)

Dead/duplicated escalation logic remains: local helper `taskIntersectsEscalated` duplicates `specloop.TaskIntersectsEscalated` but is not used by `Run`, creating drift risk against the single exact-match contract in the spec.

### internal/next/specloop/stages/execute_test.go

**Line 1:** `test_coverage` (test_coverage)

Targeted-escalation tests are concentrated on success flows; there is no focused test proving escalated-failure behavior on non-success execute outcomes (e.g., all tasks fail and `ReplanFrom` path).

### internal/next/specloop/stages/review_test.go

**Line 1725:** `architecture_drift` (architecture_drift)

Acceptance Criterion 5 coverage is under-asserted: `TestReviewThrashConsecutiveErrorCountTwoEscalates` explicitly avoids verifying `review_thrash_escalated` event emission details.

**Line 1598:** `code_quality` (code_quality)

Thrash test coverage is highly duplicated across multiple large tests that repeat near-identical setup and assertions, increasing maintenance overhead and making intent harder to follow.

**Line 1766:** `logic_gaps` (logic_gaps)

`TestReviewThrashConsecutiveErrorCountTwoEscalates` explicitly skips direct verification of `review_thrash_escalated` event emission details, leaving Acceptance Criterion 5 under-asserted.

**Line 1832:** `test_coverage` (test_coverage)

Acceptance Criterion 5 coverage is incomplete in `TestReviewThrashConsecutiveErrorCountTwoEscalates`; the test comment explicitly skips direct verification of `review_thrash_escalated` event emission details.

## Acceptance Criteria

1. [cmd/gromit-next/exec_scenario_escalation_fails_run_blocked_test.go] Remove the action override and let `ReviewStage` naturally return `Blocked` from count>=3 thrash logic, then assert the resulting terminal state.
2. [cmd/gromit-next/exec_scenario_review_thrash_escalation_test.go] Assert exact expected executions and cycle-specific tiers (for example, exactly 3 `task-thrash` runs with cycle 3 set to `high`).
3. [docs/specs/0004p-criterion-eval-structured-output.md] Restore unrelated spec files in this change and isolate any spec lifecycle cleanup into a separate, explicitly scoped change.
4. [internal/next/specloop/stages/execute.go] Remove the local `taskIntersectsEscalated` helper and keep a single shared matcher (`specloop.TaskIntersectsEscalated`) as the only implementation.
5. [internal/next/specloop/stages/review_test.go] Inject a readable event log in this test and assert one `review_thrash_escalated` event with exact file, description, and `consecutive_count: 2`.
6. [cmd/gromit-next/exec_scenario_escalation_fails_run_blocked_test.go] Remove the action override and drive the blocked outcome through the real review-stage logic (count >= 3) using only staged inputs/findings.
7. [cmd/gromit-next/exec_scenario_review_thrash_escalation_test.go] Assert exact expected executions and cycle-specific tiers (for example, exactly 3 `task-thrash` runs and exact tier per cycle).
8. [docs/specs/0004p-criterion-eval-structured-output.md] Revert unrelated spec deletions from this change and land them separately (with their own rationale/PR) if intentional.
9. [internal/next/specloop/stages/execute.go] Delete the local `taskIntersectsEscalated` function and keep a single canonical matcher (`specloop.TaskIntersectsEscalated`). If stage-local behavior is required, route all call sites to one implementation.
10. [internal/next/specloop/stages/review_test.go] Consolidate repeated scenarios into table-driven tests with shared helpers for cycle progression/assertions; keep only one focused test per acceptance criterion variant.
11. [cmd/gromit-next/exec_scenario_escalation_fails_run_blocked_test.go] Remove the action override in `blockedRunCaptureReviewStage` and assert blocked behavior from the actual review stage logic (count >= 3).
12. [cmd/gromit-next/exec_scenario_review_thrash_escalation_test.go] Assert exact expected executions/cycles (e.g., `len(thrashRuns) == 3` and exact cycle/tier expectations per run).
13. [docs/specs/0004p-criterion-eval-structured-output.md] Revert unrelated spec deletions from this PR and land them separately with dedicated rationale/review.
14. [internal/next/specloop/stages/execute.go] Delete the local helper and keep a single matcher implementation in `internal/next/specloop/escalation.go`.
15. [internal/next/specloop/stages/review_test.go] Use a real event log in this test and assert emitted event type and payload (`finding_file`, `finding_description`, `consecutive_count`).
16. [cmd/gromit-next/exec_scenario_escalation_fails_run_blocked_test.go] Remove the action override and assert blocked behavior from the real review-stage thrash logic by driving three consecutive matching error findings through the normal stage flow.
17. [cmd/gromit-next/exec_scenario_review_thrash_escalation_test.go] Assert exact expected cycle/task executions (e.g., exactly 3 thrash-task runs with explicit cycle checks and expected tiers per cycle).
18. [docs/specs/0004p-criterion-eval-structured-output.md] Restore the deleted unrelated spec files in this change and keep the `0004o` implementation PR scoped to review-thrash detection artifacts only.
19. [internal/next/specloop/stages/execute.go] Remove the local `taskIntersectsEscalated` function and rely exclusively on `specloop.TaskIntersectsEscalated` (or switch `Run` to the local helper and delete the shared one, but keep only one implementation).
20. [cmd/gromit-next/exec_scenario_escalation_fails_run_blocked_test.go] Remove the action rewrite hook and drive the run to `Blocked` via the real review-stage count>=3 logic, then assert terminal state/events from unmodified stage behavior.
21. [cmd/gromit-next/exec_scenario_review_thrash_escalation_test.go] Assert exact expected executions and cycle-specific tiers (e.g., exactly 3 runs for `task-thrash`, with cycle 1/2 medium and cycle 3 high).
22. [internal/next/specloop/stages/execute_test.go] Add an execute-stage test where an escalated task fails, verify returned `ReplanFrom` context/failure aggregation, and confirm escalation matching still applies under failure paths.
23. [internal/next/specloop/stages/review_test.go] Use a readable event log in this test and assert emitted `review_thrash_escalated` event fields (`finding_file`, `finding_description`, `consecutive_count`) explicitly.

## Validation

```
go test ./... -count=1
go vet ./...
```
