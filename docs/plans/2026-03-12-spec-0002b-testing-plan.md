# Spec 0002b -- Testing and Verification Plan

LLM Review, Acceptance Evaluation, and Fix-Cycle Replanning for Gromit Next.

Two new packages under `internal/next/`: `review`, `acceptor`. Plus extensions to `execpolicy`, `specloop`, and `evidence`.

## Overview

This document defines every test required to satisfy the Spec 0002b acceptance criteria. Each criterion maps to at least one named test function. All tests are deterministic -- no network calls, no timing dependencies, no flaky assertions.

Spec 0002b builds on top of the fully-implemented Spec 0002a execution loop. Tests in this plan cover only the new packages (`review/`, `acceptor/`) and extensions to existing packages (`execpolicy/`, `specloop/`, `evidence/`). All 0002a tests continue to pass unchanged.

## Test Strategy

### Unit test approach

- Table-driven tests for all parsing, validation, severity-comparison, and matching logic.
- Test files co-located with implementation: `foo.go` -> `foo_test.go`.
- Each test function name describes the behavior under test, not the implementation detail.
- Assertions use `t.Errorf` / `t.Fatalf` with descriptive messages; no assertion libraries.

### Integration test approach

- Build tag `//go:build integration` on all integration test files.
- Integration tests use real filesystem via `t.TempDir()`.
- Integration tests wire real packages together but stub the LLM provider via `FakeAgent`.
- Run separately: `go test -tags integration ./internal/next/...`
- Integration scenarios 8-15 extend the 7 scenarios from Spec 0002a.

### Fixture/fake strategy

Inherits all fakes from Spec 0002a (FakeAgent, FakeGit, FakeClock, FakeCmdRunner). Adds:

| Boundary | Interface | Fake |
|----------|-----------|------|
| Review facet invocation | `ReviewAgent` | `FakeReviewAgent` -- returns canned findings per facet |
| Acceptance evaluation | `AcceptanceEvaluator` | `FakeAcceptanceEvaluator` -- returns canned batch `AcceptanceResult` |

All new fakes live in `internal/next/testutil/`.

## Structural Prerequisites

Before any tests in this plan will compile, the following struct/field additions are required in existing code:

- **`RunState`** (`internal/next/runstore/`): Add `FinalReviewPassed bool`, `FinalAcceptancePassed bool`, `ReviewFindings []string`, and `AcceptanceResults []string`. These are `[]string` carrying human-readable summaries (per the design spec), not structured types. They are referenced by finalize-stage tests and `NormalizeNilFields` tests.
- **`Policy`** (`internal/next/execpolicy/`): Add `Review` sub-struct (with `Facets []string`, `Tiers map[string]string`, `ReplanThreshold string`) and `Models.Evaluator string`. These fields are referenced by policy-loading tests and reviewer/evaluator configuration.
- **`ReplanTriggeredEvent`** (`internal/next/specloop/`): Add `Source string` field (currently only has `Reason`). Tests assert `source: "review"` or `source: "acceptance"` on replan events; they will not compile without this field.

## Package-by-Package Test Coverage

### review/ tests

File: `internal/next/review/registry_test.go`

**`TestRegistry_DefaultFacets`**
- Input: Call `NewRegistry()` with default facet names.
- Assert: Registry contains exactly `spec_alignment` and `code_quality`.
- Assert: Both facets have non-empty prompt templates.
- **Evidence**: AC 6.

**`TestRegistry_AllBuiltInFacets`**
- Input: Call `NewRegistry()` with all built-in facet names.
- Assert: Registry contains all 5 built-in facets: `spec_alignment`, `code_quality`, `logic_gaps`, `test_coverage`, `architecture_drift`.
- Assert: Each facet has a non-empty `Name`, `Description`, and `PromptTemplate`.
- **Evidence**: AC 6.

**`TestRegistry_GetFacet_Exists`**
- Input: `registry.Get("spec_alignment")`.
- Assert: Returns the facet definition. `ok == true`.

**`TestRegistry_GetFacet_NotFound`**
- Input: `registry.Get("nonexistent")`.
- Assert: Returns zero value. `ok == false`.

**`TestRegistry_SelectFacets_AllValid`**
- Input: `registry.Select([]string{"spec_alignment", "code_quality"})`.
- Assert: Returns 2 facet definitions. No error.

**`TestRegistry_SelectFacets_UnknownFacet_ReturnsError`**
- Input: `registry.Select([]string{"spec_alignment", "does_not_exist"})`.
- Assert: Returns error mentioning `"does_not_exist"`.

**`TestRegistry_SelectFacets_Empty_ReturnsError`**
- Input: `registry.Select([]string{})`.
- Assert: Returns error mentioning "at least one facet".

File: `internal/next/review/finding_test.go`

**`TestSeverity_Ordering`**
- Table-driven with all pairs:
  - `SeverityError > SeverityWarning`
  - `SeverityWarning > SeveritySuggestion`
  - `SeveritySuggestion > SeverityInfo`
- Assert: `severity.Rank()` returns strictly decreasing integers as severity decreases.
- **Evidence**: AC 7.

**`TestSeverity_Parse_ValidValues`**
- Table-driven: `"error"`, `"warning"`, `"suggestion"`, `"info"`.
- Assert: Each parses to the correct `Severity` constant. No error.

**`TestSeverity_Parse_Invalid`**
- Input: `ParseSeverity("critical")`.
- Assert: Returns error mentioning "unknown severity".

**`TestSeverity_String_RoundTrips`**
- For each severity constant, `ParseSeverity(s.String())` returns the same constant.

**`TestFinding_NormalizeNilFields`**
- Input: `Finding{}` with zero-value fields.
- Assert: After `NormalizeNilFields()`, the method exists and does not panic. (`SuggestedFix` is a `string`, not a pointer, so it is already `""` at zero value.)

**`TestParseFindingsJSON_ValidOutput`**
- Input: JSON string matching the finding format from the spec:
  ```json
  {
    "facet": "code_quality",
    "findings": [
      {"severity": "suggestion", "file": "runner.go", "line": 42, "description": "nil pointer", "suggested_fix": "add check"}
    ]
  }
  ```
- Assert: Parsed result has 1 finding with correct fields. `Cycle` defaults to 0. `Disposition` defaults to `""`.

**`TestParseFindingsJSON_NoFindings`**
- Input: `{"facet": "spec_alignment", "findings": []}`.
- Assert: Parsed result has 0 findings. No error.

**`TestParseFindingsJSON_InvalidJSON_ReturnsError`**
- Input: Malformed JSON string.
- Assert: Returns parse error.

**`TestParseFindingsJSON_MissingSeverity_ReturnsError`**
- Input: Finding with empty `severity` field.
- Assert: Returns validation error mentioning "severity".

**`TestParseFindingsJSON_MissingFile_ReturnsError`**
- Input: Finding with empty `file` field.
- Assert: Returns validation error mentioning "file".

**`TestParseFindingsJSON_MissingDescription_ReturnsError`**
- Input: Finding with empty `description` field.
- Assert: Returns validation error mentioning "description".

File: `internal/next/review/threshold_test.go`

**`TestThreshold_IsBlocking`**
- Table-driven with `(threshold Severity, finding Severity, want bool)` triples:
  - `(SeverityError, SeverityError, true)` — error blocks at error threshold
  - `(SeverityError, SeverityWarning, false)` — warning does not block at error threshold
  - `(SeverityError, SeveritySuggestion, false)` — suggestion does not block at error threshold
  - `(SeverityError, SeverityInfo, false)` — info never blocks
  - `(SeverityWarning, SeverityWarning, true)` — warning blocks at warning threshold
  - `(SeverityWarning, SeverityError, true)` — error blocks at warning threshold
  - `(SeverityWarning, SeveritySuggestion, false)` — suggestion does not block at warning threshold
  - `(SeveritySuggestion, SeveritySuggestion, true)` — suggestion blocks at suggestion threshold
  - `(SeveritySuggestion, SeverityError, true)` — error blocks at suggestion threshold
  - `(SeveritySuggestion, SeverityInfo, false)` — info never blocks at any threshold
- Assert: `IsBlocking(threshold, finding) == want` for each row.
- **Evidence**: AC 3.

**`TestParseSeverity_ValidValues`**
- Table-driven: `"error"`, `"warning"`, `"suggestion"`, `"info"`.
- Assert: Each parses correctly to corresponding `Severity` constant. No error.

**`TestParseSeverity_Unknown_Invalid`**
- Input: `ParseSeverity("critical")`.
- Assert: Returns error.

**`TestPolicyThresholdValidation`**
- Input: Policy with `replan_threshold: "info"`.
- Assert: Policy validation rejects `"info"` as a threshold (info never blocks). This is a policy-level validation, not a parse-level restriction — `ParseSeverity("info")` succeeds, but the policy rejects it.
- **Evidence**: AC 3.

**`TestFilterBlockingFindings_MixedSeverities`**
- Input: 4 findings (one of each severity). Threshold: `"warning"`.
- Assert: Returns only the error and warning findings. Suggestion and info excluded.
- **Evidence**: AC 3, 7.

**`TestFilterBlockingFindings_AllBelowThreshold`**
- Input: 2 info findings. Threshold: `"suggestion"`.
- Assert: Returns empty slice. No findings block.

**`TestFilterBlockingFindings_EmptyInput`**
- Input: Empty findings slice. Any threshold.
- Assert: Returns empty slice. No error.

File: `internal/next/review/matching_test.go`

**`TestMatchPreexisting_ExactMatch`**
- Input: Prior finding `{File: "handler.go", Description: "nil pointer if commands list is empty"}`. Current finding has identical file and description.
- Assert: `IsPreexisting == true`.

**`TestMatchPreexisting_SubstringMatch`**
- Input: Prior finding `{File: "handler.go", Description: "nil pointer if commands list is empty"}`. Current finding `{File: "handler.go", Description: "nil pointer if commands list is empty; also affects reset path"}`.
- Assert: `IsPreexisting == true`. Substring of prior description appears in current description.

**`TestMatchPreexisting_ReverseSubstringMatch`**
- Input: Prior finding has longer description, current finding has shorter description that is a substring.
- Assert: `IsPreexisting == true`. Matching is bidirectional (either direction substring).

**`TestMatchPreexisting_DifferentFile`**
- Input: Same description text but different file paths.
- Assert: `IsPreexisting == false`. File path must match.

**`TestMatchPreexisting_DifferentDescription`**
- Input: Same file path but completely different description text.
- Assert: `IsPreexisting == false`.

**`TestMatchPreexisting_LineNumberShift_StillMatches`**
- Input: Prior finding `{File: "handler.go", Line: 42, Description: "nil pointer"}`. Current finding `{File: "handler.go", Line: 58, Description: "nil pointer"}`.
- Assert: `IsPreexisting == true`. Line number differences are ignored.

**`TestLabelDispositions_MixedNewAndPreexisting`**
- Input: Prior findings from cycle 1: `[{File: "a.go", Desc: "dup logic"}]`. Current findings: `[{File: "a.go", Desc: "dup logic"}, {File: "b.go", Desc: "missing test"}]`.
- Assert: First finding labeled `disposition: "pre-existing"`. Second labeled `disposition: "new"`.
- Assert: Both findings have `cycle` set to current cycle number.

**`TestLabelDispositions_AllNew`**
- Input: Empty prior findings. Current findings: 2 findings.
- Assert: Both labeled `disposition: "new"`.

**`TestLabelDispositions_AllPreexisting`**
- Input: Prior findings match all current findings.
- Assert: All labeled `disposition: "pre-existing"`.

**`TestFilterNewBlockingFindings_OnlyNewAboveThreshold`**
- Input: 3 findings: one new-error, one preexisting-error, one new-info. Threshold: `"suggestion"`.
- Assert: Returns only the new-error finding. Preexisting findings excluded even if above threshold. New-info excluded because info never blocks.
- **Evidence**: AC 4.

File: `internal/next/review/runner_test.go`

**`TestRunner_InvokesAllEnabledFacets`**
- Input: Reviewer configured with 2 facets (`spec_alignment`, `code_quality`). FakeAgent returns clean findings for both.
- Assert: FakeAgent called exactly twice. Each call's prompt contains the corresponding facet's template.
- **Evidence**: AC 6.

**`TestRunner_ParallelInvocation`**
- Input: Reviewer configured with 3 facets. FakeAgent has artificial latency (tracked via timestamps).
- Assert: All 3 calls recorded. Total wall-clock time is closer to 1x latency than 3x (indicates parallel execution).
- Note: This test is inherently timing-sensitive. Use generous tolerance (e.g., < 2x single-call duration).

**`TestRunner_AggregatesFindings`**
- Input: FakeAgent returns 1 finding for `spec_alignment` and 2 findings for `code_quality`.
- Assert: Aggregated result contains 3 total findings across both facets.

**`TestRunner_SetsModelTierFromPolicy`**
- Input: Policy has `tiers.spec_alignment: "high"`, `tiers.code_quality: "medium"`. Runner constructed with these tiers.
- Assert: Runner internally selects the correct model tier for each facet at construction time. Verified by inspecting the runner's per-facet configuration (tiers are injected at construction, not passed per-call).

**`TestRunner_RetryOnInvalidOutput`**
- Input: FakeAgent returns unparseable JSON first, valid JSON second (for one facet).
- Assert: Review completes successfully. Agent called twice for that facet.

**`TestRunner_CycleAndDispositionSet`**
- Input: Cycle 1 review. No prior findings.
- Assert: All findings have `Cycle == 1` and `Disposition == "new"`.

### acceptor/ tests

File: `internal/next/acceptor/criterion_test.go`

**`TestParseCriteria_FromSpecText`**
- Input: Spec markdown with an "Acceptance Criteria" section containing 3 numbered items.
- Assert: Returns 3 `Criterion` structs, each with the criterion text captured.

**`TestParseCriteria_NoSection_ReturnsError`**
- Input: Spec markdown without an "Acceptance Criteria" section.
- Assert: Returns error mentioning "acceptance criteria".

**`TestParseCriteria_EmptySection_ReturnsEmptySlice`**
- Input: Spec markdown with "Acceptance Criteria" section but no items.
- Assert: Returns empty slice. No error. (Aligns with `TestParseAcceptanceCriteria_EmptySection` and execution plan Task 16a.)

**`TestParseAcceptanceCriteria_FromMarkdown`**
- **Clarification**: `ParseCriteria` (tested above) is the primary API, returning `[]Criterion` structs. `ParseAcceptanceCriteria` is a convenience wrapper returning `[]string` (criterion text only). If both functions are not needed, consolidate into `ParseCriteria` alone and remove the `ParseAcceptanceCriteria` tests below. If both are kept, `ParseAcceptanceCriteria` should delegate to `ParseCriteria` internally.
- Input: Markdown string with `## Acceptance Criteria` section containing bullet points (`- criterion text`).
- Assert: Parses each bullet into a criterion string. Returns correct count.

**`TestParseAcceptanceCriteria_MissingSection`**
- Input: Markdown string without `## Acceptance Criteria` heading.
- Assert: Returns error indicating the section was not found.

**`TestParseAcceptanceCriteria_EmptySection`**
- Input: Markdown string with `## Acceptance Criteria` heading but no bullet points or items below it.
- Assert: Returns empty slice. No error.

**`TestParseCriteria_NumberedList`**
- Input: Criteria in `1. First criterion\n2. Second criterion` format.
- Assert: Parses correctly. Criterion text does not include the number prefix.

**`TestParseCriteria_BoldPrefix`**
- Input: `1. **Review gate** -- ready_for_review impossible if...`
- Assert: Criterion text is the full text including the bold label.

File: `internal/next/acceptor/evaluator_test.go`

**`TestEvaluateResult_ParsePass`**
- Input: JSON output:
  ```json
  {"criterion": "Zero repo pollution", "status": "pass", "rationale": "No files in repo", "evidence_refs": ["evidence/diff-summary.md"]}
  ```
- Assert: Parsed `CriterionResult` has `Status == "pass"`, non-empty `Rationale`, non-empty `EvidenceRefs`.
- **Evidence**: AC 2.

**`TestEvaluateResult_ParseFail`**
- Input: JSON with `"status": "fail"`.
- Assert: `Status == "fail"`. `Rationale` present.
- **Evidence**: AC 2.

**`TestEvaluateResult_ParseUnclear`**
- Input: JSON with `"status": "unclear"`.
- Assert: `Status == "unclear"`. `Rationale` present.
- **Evidence**: AC 2.

**`TestEvaluateResult_InvalidStatus_ReturnsError`**
- Input: JSON with `"status": "maybe"`.
- Assert: Returns validation error mentioning "status".

**`TestEvaluateResult_MissingRationale_ReturnsError`**
- Input: JSON with empty `rationale` field.
- Assert: Returns validation error mentioning "rationale".

**`TestEvaluateResult_MissingEvidenceRefs_ReturnsError`**
- Input: JSON with empty `evidence_refs` array.
- Assert: Returns validation error mentioning "evidence_refs".

**`TestEvaluateResult_InvalidJSON_ReturnsError`**
- Input: Malformed JSON.
- Assert: Returns parse error.

**`TestEvaluator_InvokesAgentPerCriterion`**
- Input: 3 acceptance criteria. FakeAgent returns pass for each.
- Assert: FakeAgent called exactly 3 times. Each call prompt includes the criterion text plus validation results, review findings, and diff summary.
- **Evidence**: AC 2.

**`TestEvaluator_AggregatesResults`**
- Input: 3 criteria. FakeAgent returns pass, fail, unclear.
- Assert: Aggregated `AcceptanceResult.Results` has 3 entries. `AllPass == false`. `HasFailOrUnclear == true`.

**`TestEvaluator_AllPass`**
- Input: 3 criteria, all pass.
- Assert: `AcceptanceResult.Results` has 3 entries. `AllPass == true`. `HasFailOrUnclear == false`.

**`TestEvaluator_RetryOnInvalidOutput`**
- Input: FakeAgent returns invalid JSON first, valid result second (for one criterion).
- Assert: Evaluation completes successfully.

**`TestCriterionResult_NormalizeNilFields`**
- Input: `CriterionResult{}` with nil `EvidenceRefs`.
- Assert: After `NormalizeNilFields()`, `EvidenceRefs` is `[]string{}`.

### execpolicy/ tests (extensions)

File: `internal/next/execpolicy/policy_test.go` (additional tests)

**`TestLoadFromFile_ReviewConfig_AllFields`**
- Input: JSON with full review config:
  ```json
  {"review": {"facets": ["spec_alignment", "code_quality"], "tiers": {"spec_alignment": "high", "code_quality": "medium"}, "replan_threshold": "warning"}}
  ```
- Assert: `Policy.Review.Facets` has 2 entries. `Policy.Review.Tiers["spec_alignment"] == "high"`. `Policy.Review.ReplanThreshold == "warning"`.

**`TestLoadFromFile_ReviewConfig_Defaults`**
- Input: JSON with no `review` section.
- Assert: `Policy.Review.Facets` defaults to `["spec_alignment", "code_quality"]`. `Policy.Review.ReplanThreshold` defaults to `"warning"`. `Policy.Review.Tiers` defaults with `spec_alignment: "high"`, `code_quality: "medium"`.

**`TestLoadFromFile_EvaluatorTier`**
- Input: JSON with `"models": {"evaluator": "high"}`.
- Assert: `Policy.Models.Evaluator == "high"`.

**`TestLoadFromFile_EvaluatorTier_Default`**
- Input: JSON with no `evaluator` in models.
- Assert: `Policy.Models.Evaluator` defaults to `"high"`.

**`TestValidate_ReviewConfig_InvalidThreshold`**
- Input: `Policy.Review.ReplanThreshold = "info"`.
- Assert: Returns validation error. `"info"` is not a valid threshold.

**`TestValidate_ReviewConfig_InvalidFacet`**
- Input: `Policy.Review.Facets = ["spec_alignment", "nonexistent_facet"]`.
- Assert: Returns validation error mentioning `"nonexistent_facet"`.
- **Evidence**: AC 6.

**`TestValidate_ReviewConfig_EmptyFacets`**
- Input: `Policy.Review.Facets = []`.
- Assert: Returns validation error mentioning "at least one facet".

**`TestValidate_ReviewConfig_ValidCustomSelection`**
- Input: `Policy.Review.Facets = ["spec_alignment", "logic_gaps", "test_coverage"]`.
- Assert: No validation error. All are valid built-in facets.
- **Evidence**: AC 6.

### specloop/ tests (extensions) — AcceptStage NeedsHuman behavior

File: `internal/next/specloop/stages/accept_test.go` (additional tests)

**`TestAcceptStage_MissingCriteriaSection_ReturnsNeedsHuman`**
- Input: Spec markdown without `## Acceptance Criteria` heading. `ParseAcceptanceCriteria` returns error.
- Assert: AcceptStage returns `NeedsHuman` with message containing "acceptance criteria".
- **Evidence**: Fails fast, no wasted cycles.

**`TestAcceptStage_EmptyCriteriaSection_ReturnsNeedsHuman`**
- Input: Spec markdown with `## Acceptance Criteria` heading but no items. `ParseAcceptanceCriteria` returns empty slice.
- Assert: AcceptStage returns `NeedsHuman` with message "spec lacks acceptance criteria section — cannot evaluate acceptance."

### specloop/ tests (extensions) — ReviewStage GitOps and parallel failure

File: `internal/next/specloop/stages/review_test.go` (additional tests)

**`TestReviewStage_ComputesDiffFromGitOps`**
- Input: ReviewStageConfig with a FakeGitOps that returns a known diff string.
- Assert: The diff is passed to the review runner's RunInput.DiffSummary.
- **Evidence**: ReviewStage computes diff at runtime, not from static config.

**`TestReviewStage_FacetError_ContinuesWithSuccessfulFacets`**
- Input: Review runner returns one successful facet and one errored facet.
- Assert: ReviewStage continues with partial results. Errored facet is recorded in evidence. Successful facet findings are evaluated for blocking.
- **Evidence**: Parallel facet failure handling.

**`TestReviewStage_FacetError_ErroredFacetInEvidence`**
- Input: One facet times out.
- Assert: `review.json` contains an error marker for the failed facet. The human reviewer can see which facets completed and which errored.

### specloop/ tests (extensions) — SpecLoop pipeline

File: `internal/next/specloop/specloop_test.go` (additional tests)

**`TestSpecLoop_ReviewStageInPipelineOrder`**
- Input: Stages in order: init, compile, plan, execute, validate, review, accept, evidence, finalize.
- Assert: Stages execute in that order. Review runs after validate. Accept runs after review.

**`TestSpecLoop_ReviewStage_ContinueOnCleanReview`**
- Input: ReviewStage returns `Continue` (no blocking findings).
- Assert: AcceptStage runs next. Terminal state is `ready_for_review`.

**`TestSpecLoop_ReviewStage_ReplanOnBlockingFindings`**
- Input: ReviewStage returns `ReplanFrom` with FailureContext containing blocking findings.
- Assert: Loop replans from PlanStage. Cycle incremented. ReviewStage's FailureContext passed to planner.
- **Evidence**: AC 1, 4.

**`TestSpecLoop_ReviewStage_FailureContext_CarriesFindings`**
- Input: ReviewStage returns `ReplanFrom` with `FailureContext` containing 2 blocking findings.
- Assert: `FailureContext.Failures` has 2 entries. Each entry describes the finding (facet, severity, file, description).
- **Evidence**: AC 4.

**`TestReviewFailuresToStrings`**
- Input: FailureContext with review findings.
- Assert: Each failure formats as `"review:<facet>:<severity>:<file>:<line> — <description>"`.
- **Evidence**: AC 4.

**`TestAcceptanceFailuresToStrings_Fail`**
- Input: FailureContext with acceptance result where status is `"fail"`.
- Assert: Each failure formats as `"acceptance:fail: <criterion> — implement missing behavior"`.
- **Evidence**: AC 5.

**`TestAcceptanceFailuresToStrings_Unclear`**
- Input: FailureContext with acceptance result where status is `"unclear"`.
- Assert: Each failure formats as `"acceptance:unclear: <criterion> — add tests or evidence to prove/disprove"`.
- **Evidence**: AC 5.

**`TestSpecLoop_AcceptStage_ContinueOnAllPass`**
- Input: AcceptStage returns `Continue` (all criteria pass).
- Assert: EvidenceStage runs next. Terminal state is `ready_for_review`.

**`TestSpecLoop_AcceptStage_ReplanOnFail`**
- Input: AcceptStage returns `ReplanFrom` with FailureContext listing 1 failed criterion.
- Assert: Loop replans. FailureContext contains criterion text, status "fail", and rationale.
- **Evidence**: AC 5.

**`TestSpecLoop_AcceptStage_ReplanOnUnclear`**
- Input: AcceptStage returns `ReplanFrom` with FailureContext listing 1 unclear criterion.
- Assert: Loop replans. FailureContext indicates "unclear" status with guidance to add evidence.
- **Evidence**: AC 5.

**`TestSpecLoop_AcceptStage_FailureContext_CarriesCriteria`**
- Input: AcceptStage returns `ReplanFrom` with 2 failed criteria.
- Assert: FailureContext includes criterion text, status, and rationale for each.
- **Evidence**: AC 5.

**`TestSpecLoop_ReviewFindingsTriggerReplan_AcceptanceDoesNotRun`**
- Input: ReviewStage returns `ReplanFrom`.
- Assert: AcceptStage does NOT run during this cycle. Pipeline jumps back to Plan.

**`TestSpecLoop_BudgetSharing_ValidationThenReview`**
- Input: `max_spec_cycles: 3`. Cycle 1: initial execution. Cycle 2: validation fix cycle. Cycle 3: review fix cycle.
- Assert: Budget exhausted after cycle 3. All three cycle types consume from the same counter.
- **Evidence**: AC 9.

**`TestSpecLoop_BudgetSharing_ValidationThenAcceptance`**
- Input: `max_spec_cycles: 3`. Cycle 1: initial. Cycle 2: validation fix. Cycle 3: acceptance fix.
- Assert: Budget exhausted. `needs_human` terminal state.
- **Evidence**: AC 9.

**`TestSpecLoop_BudgetSharing_ReviewThenAcceptance`**
- Input: `max_spec_cycles: 3`. Cycle 1: initial. Cycle 2: review fix. Cycle 3: acceptance fix.
- Assert: Budget exhausted. `needs_human` terminal state.
- **Evidence**: AC 9.

**`TestSpecLoop_BudgetExhausted_ReviewCycles_NeedsHuman`**
- Input: `max_spec_cycles: 2`. ReviewStage always returns `ReplanFrom`.
- Assert: Terminal state is `needs_human`. Exactly 2 cycles ran.
- **Evidence**: AC 9.

**`TestSpecLoop_NewOnlyFindings_DontRetrigger`**
- Input: Cycle 1 review finds 1 error finding. Cycle 2 fix resolves it. Cycle 2 review finds 1 info finding (new).
- Assert: No replan triggered from cycle 2 review. Info findings are recorded but do not block.
- **Evidence**: AC 7.

**`TestSpecLoop_PreexistingFindings_DontBlock`**
- Input: Cycle 1 review finds 1 warning. Cycle 2 fix resolves targeted issue but review still reports the same warning (preexisting).
- Assert: Preexisting warning does not trigger another replan. Pipeline proceeds to acceptance.
- **Evidence**: AC 4.

**`TestFinalizeStage_ReadyForReview_RequiresAllThreeGates`**
- Input: allDone && FinalValidationPassed && FinalReviewPassed && FinalAcceptancePassed.
- Assert: Terminal state is `ready_for_review`.
- Assert: If any of the four conditions is false, terminal state is NOT `ready_for_review`.
- **Evidence**: AC 1.

**`TestFinalizeStage_NeedsHuman_WhenReviewFails`**
- Input: allDone && FinalValidationPassed && FinalReviewPassed == false.
- Assert: Terminal state is `needs_human`.
- **Evidence**: AC 1.

**`TestFinalizeStage_NeedsHuman_WhenAcceptanceFails`**
- Input: allDone && FinalValidationPassed && FinalReviewPassed && FinalAcceptancePassed == false.
- Assert: Terminal state is `needs_human`.
- **Evidence**: AC 1.

**`TestFinalizeStage_PreservesWorktreeForBlocked`**
- **Prerequisite**: The current `FinalizeStage` actively removes worktrees for blocked runs (`finalize.go` lines 31-33). The `RemoveWorktree` call must be removed before this test can pass.
- Input: FinalizeStage reaches `blocked` terminal state.
- Assert: Worktree directory is NOT cleaned up (preserved for human inspection).
- Assert: RunState records the worktree path.

**`TestInitStage_CleansBlockedWorktreesForSameSpec`**
- Input: Store contains a prior run with terminal state `blocked` and spec_id matching the current spec.
- Assert: InitStage scans the store, finds the prior blocked run.
- Assert: Prior blocked run's worktree directory is removed.
- Assert: Prior blocked run's worktree path is cleared in the store.

**`TestInitStage_IgnoresNonBlockedPriorRuns`**
- Input: Store contains prior runs with terminal states `ready_for_review` and `needs_human`, same spec_id.
- Assert: InitStage does NOT clean those worktrees.
- Assert: Their worktree paths remain intact in the store.

**`TestRunState_NormalizeNilFields_IncludesNewFields`**
- Input: `RunState{}` with nil `ReviewFindings` and nil `AcceptanceResults`.
- Assert: After `NormalizeNilFields()`, `ReviewFindings` is empty slice (not nil) and `AcceptanceResults` is empty slice (not nil).

**`TestSpecLoop_VisionLabelNotSet`**
- Input: Run completes with `ready_for_review`.
- Assert: No VISION review outcome label is recorded in RunState or evidence. The field does not exist or is empty.
- **Evidence**: AC 8.

File: `internal/next/specloop/event_contract_test.go` (additional tests)

**`TestEventContract_ReviewAndAcceptanceEvents`**
- Input: Full pipeline with review and acceptance stages emitting events.
- Assert: `events.jsonl` contains new event types:
  - `review_result`
  - `acceptance_result`
  - (Optional, may also emit: `review_started`, `review_finding`, `acceptance_started`, `acceptance_criterion_result`)
- Assert: Event ordering: `final_validation_result` before `review_result`, `review_result` before `acceptance_result`, `acceptance_result` before `terminal_state`.

**`TestEventContract_ReviewResultEvent`**
- Input: ReviewStage completes with 2 facets reviewed: `spec_alignment` (0 findings), `code_quality` (1 warning, 1 error).
- Assert: `events.jsonl` contains a `review_result` event emitted by ReviewStage.
- Assert: Event payload includes:
  - `facets_reviewed`: list of facet names that were evaluated (e.g., `["spec_alignment", "code_quality"]`).
  - `finding_counts_by_severity`: map of severity to count (e.g., `{"error": 1, "warning": 1, "suggestion": 0, "info": 0}`).
  - `has_blocking_findings`: boolean indicating whether any findings exceed the replan threshold.
  - `errored_facets`: list of facet names that returned errors (empty list when all succeed).
- **Evidence**: AC 1, 7.

**`TestEventContract_AcceptanceResultEvent`**
- Input: AcceptStage completes with 3 criteria: 2 pass, 1 fail.
- Assert: `events.jsonl` contains an `acceptance_result` event emitted by AcceptStage.
- Assert: Event payload includes:
  - `criteria_evaluated`: integer count of criteria evaluated (e.g., `3`).
  - `per_criterion_status`: list of objects with `criterion` (text) and `status` (`"pass"`, `"fail"`, or `"unclear"`) for each criterion.
  - `all_pass`: boolean (`false` in this case because one criterion failed).
- **Evidence**: AC 2, 5.

**`TestEventContract_ReplanTriggeredEvent_HasSource`**
- Input: Three scenarios exercised across sub-tests:
  - (a) ValidationStage triggers replan.
  - (b) ReviewStage triggers replan due to blocking findings.
  - (c) AcceptStage triggers replan due to failed criterion.
- Assert: In each case, `events.jsonl` contains a `replan_triggered` event.
- Assert: Each `replan_triggered` event includes a `source` field with value:
  - `"validation"` for scenario (a).
  - `"review"` for scenario (b).
  - `"acceptance"` for scenario (c).
- Assert: The `source` field is always present and is one of the three allowed values.
- **Evidence**: AC 4, 5.

**`TestEventContract_ReviewReplanEvent`**
- Input: ReviewStage triggers replan.
- Assert: `events.jsonl` contains `replan_triggered` event with `source: "review"` and list of blocking findings in metadata.

**`TestEventContract_AcceptanceReplanEvent`**
- Input: AcceptStage triggers replan.
- Assert: `events.jsonl` contains `replan_triggered` event with `source: "acceptance"` and list of failed/unclear criteria in metadata.

**`TestEventContract_BlockedWorktreeCleanedEvent`**
- Input: InitStage finds and cleans a prior blocked worktree for the same spec_id.
- Assert: `events.jsonl` contains a `blocked_worktree_cleaned` event.
- Assert: Event metadata includes `prior_run_id`, `spec_id`, and `worktree_path` of the cleaned worktree.

### evidence/ tests (extensions)

File: `internal/next/evidence/bundle_test.go` (additional tests)

**`TestBundler_WriteReviewFindings`**
- Input: Review results with findings across 2 facets (spec_alignment: 1 finding, code_quality: 2 findings).
- Assert: `review.json` exists. Parses to correct schema. Keyed by facet name. Finding fields include `severity`, `file`, `line`, `description`, `suggested_fix`, `cycle`, `disposition`.

**`TestBundler_WriteReviewFindings_EmptyFindings`**
- Input: Review results with all facets producing 0 findings.
- Assert: `review.json` exists. Each facet key maps to empty array `[]`, not `null`.

**`TestBundler_WriteReviewFindings_CycleAndDisposition`**
- Input: Findings from cycle 1 (all `new`) and cycle 2 (mix of `new` and `pre-existing`).
- Assert: `review.json` preserves `cycle` and `disposition` fields for each finding.

**`TestBundler_WriteAcceptance`**
- Input: Acceptance results with 3 criteria: pass, fail, unclear.
- Assert: `acceptance.json` exists. Top-level is envelope object with `results` array, `all_pass` bool, `has_fail_or_unclear` bool. Each entry in `results` has `criterion`, `status`, `rationale`, `evidence_refs`. `all_pass == false` and `has_fail_or_unclear == true` (because of fail and unclear).
- **Evidence**: AC 2.

**`TestBundler_WriteAcceptance_AllPass`**
- Input: 3 criteria all passing.
- Assert: `acceptance.json` envelope has `all_pass == true`, `has_fail_or_unclear == false`. `results` array has 3 entries, all with `status: "pass"`.

**`TestBundler_WriteAcceptance_NilEvidenceRefs_SerializesAsEmptyArray`**
- Input: `CriterionResult` with nil `EvidenceRefs`.
- Assert: Inside the envelope's `results` array, JSON serializes `evidence_refs` as `[]`, not `null`.

**`TestBundler_WriteReview_IncludesReviewFindingsSection`**
- Input: `ReviewInput` with review findings populated.
- Assert: `review.md` contains a "## Review Findings" section listing findings by facet.

**`TestBundler_WriteReview_IncludesAcceptanceTable`**
- Input: `ReviewInput` with acceptance results populated.
- Assert: `review.md` contains a "## Acceptance Criteria" section with a table showing criterion, status, rationale, evidence refs.

## Integration Test Scenarios

All integration tests in `internal/next/specloop/specloop_integration_test.go` with build tag `//go:build integration`.

### Scenario 8: Happy path with review + acceptance pass -> ready_for_review

**`TestIntegration_ReviewAcceptance_HappyPath_ReadyForReview`**

Setup:
- Create fixture project in temp dir with `go.mod`, `main.go`, passing test file.
- Initialize git repo. Create project cell.
- Write spec file with 3 acceptance criteria.
- Wire FakeAgent to return:
  - Valid plan (2 tasks).
  - Valid implementation output for each task.
  - Passing targeted checks.
  - Clean review findings for both `spec_alignment` and `code_quality` (0 findings each).
  - Pass result for each of the 3 acceptance criteria (status: "pass", rationale, evidence_refs).

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`.
- `evidence/review.json` exists and contains empty findings arrays for both facets.
- `evidence/acceptance.json` exists and parses as envelope: `all_pass == true`, `has_fail_or_unclear == false`, `results` contains 3 entries all with `status: "pass"`.
- `evidence/review.md` contains "Review Findings" section (empty/clean).
- `evidence/review.md` contains "Acceptance Criteria" table with 3 pass entries.
- `events.jsonl` contains `review_result`, `acceptance_result`, `terminal_state`.
- No VISION review outcome label recorded.

**Evidence**: AC 1, 2, 8.

### Scenario 9: Review finding above threshold blocks ready_for_review, triggers fix cycle

**`TestIntegration_ReviewFinding_TriggersFixCycle`**

Setup:
- Policy: `review.replan_threshold: "warning"` (default), `max_spec_cycles: 3`.
- Cycle 1: FakeAgent returns valid plan, tasks complete, validation passes.
- Cycle 1 review: FakeAgent returns 1 `error`-severity finding for `spec_alignment` (`"Spec requires idempotency, handler does not check"`).
- Cycle 2 (fix): FakeAgent returns fix plan targeting the finding. Fix task completes. Validation passes.
- Cycle 2 review: FakeAgent returns 0 findings.
- Acceptance: FakeAgent returns all criteria pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`.
- Exactly 2 planning cycles occurred.
- `events.jsonl` contains `replan_triggered` with source `"review"` (new field) after cycle 1.
- `evidence/review.json` contains the cycle 1 finding with `cycle: 1, disposition: "new"`.
- Fix plan prompt (captured from FakeAgent) includes the finding description.
- Cycle 2 review shows clean findings.

**Evidence**: AC 1, 4.

### Scenario 10: Acceptance fail triggers fix cycle, passes on retry

**`TestIntegration_AcceptanceFail_FixCycle_ThenPass`**

Setup:
- Policy: `max_spec_cycles: 3`.
- Cycle 1: Plan, execute, validate all pass. Review clean.
- Cycle 1 acceptance: FakeAgent returns 2 pass, 1 fail (`"supports multi-currency": fail, "Only handles USD"`).
- Cycle 2 (fix): Fix plan targets the failed criterion. Fix task completes. Validation passes. Review clean.
- Cycle 2 acceptance: FakeAgent returns all 3 pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`.
- 2 planning cycles occurred.
- `events.jsonl` contains `replan_triggered` with source `"acceptance"` (new field) after cycle 1.
- Fix plan prompt includes the failed criterion text and rationale.
- `evidence/acceptance.json` reflects final all-pass results.

**Evidence**: AC 5.

### Scenario 11: Acceptance unclear -> fix cycle adds evidence -> passes

**`TestIntegration_AcceptanceUnclear_FixAddsEvidence_ThenPass`**

Setup:
- Policy: `max_spec_cycles: 3`.
- Cycle 1: Plan, execute, validate, review all pass/clean.
- Cycle 1 acceptance: 2 pass, 1 unclear (`"audit log entry created": unclear, "No test verifies audit log creation"`).
- Cycle 2 (fix): Fix plan targets adding evidence (test for audit log). Fix task completes. Validation passes. Review clean.
- Cycle 2 acceptance: All 3 pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`.
- Fix plan prompt for cycle 2 includes guidance about adding evidence/tests (not re-implementing).
- `events.jsonl` contains `replan_triggered` with source `"acceptance"` (new field).
- `evidence/acceptance.json` reflects final all-pass.

**Evidence**: AC 5.

### Scenario 12: Budget exhausted across review/acceptance cycles -> needs_human

**`TestIntegration_BudgetExhausted_ReviewAcceptance_NeedsHuman`**

Setup:
- Policy: `max_spec_cycles: 3`.
- Cycle 1: Initial plan, execute, validate pass.
- Cycle 1 review: 1 error finding -> triggers fix (cycle 2).
- Cycle 2: Fix completes, validate passes, review clean.
- Cycle 2 acceptance: 1 fail criterion -> triggers fix (cycle 3).
- Cycle 3: Fix completes, validate passes, review clean.
- Cycle 3 acceptance: Same criterion still fails.
- Budget exhausted (3 cycles used).

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `needs_human`.
- Blocker summary describes the persistent acceptance failure.
- `events.jsonl` shows cycle progression: cycle 1 (initial), cycle 2 (review fix), cycle 3 (acceptance fix).
- Exactly 3 planning cycles occurred.
- `evidence/review.md` contains blocker summary section.

**Evidence**: AC 9.

### Scenario 13: Configurable threshold -- warnings non-blocking

**`TestIntegration_ThresholdError_WarningsNonBlocking`**

Setup:
- Policy: `review.replan_threshold: "error"`, `max_spec_cycles: 2`.
- Cycle 1: Plan, execute, validate pass.
- Cycle 1 review: FakeAgent returns 2 `warning`-severity findings and 0 `error`-severity findings.
- Acceptance: All criteria pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`. Warnings did not trigger a fix cycle.
- Only 1 planning cycle occurred (no replan).
- `evidence/review.json` contains the 2 warning findings (recorded but not blocking).
- `events.jsonl` does NOT contain `replan_triggered`.

**Evidence**: AC 3.

### Scenario 13a: Default warning threshold -- suggestions non-blocking

**`TestIntegration_DefaultThresholdWarning_SuggestionsNonBlocking`**

Setup:
- Policy: `review.replan_threshold: "warning"` (default), `max_spec_cycles: 2`.
- Cycle 1: Plan, execute, validate pass.
- Cycle 1 review: FakeAgent returns 2 `suggestion`-severity findings and 0 `warning`/`error` findings.
- Acceptance: All criteria pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`. Suggestions did not trigger a fix cycle.
- Only 1 planning cycle occurred (no replan).
- `evidence/review.json` contains the 2 suggestion findings (recorded but not blocking).
- `events.jsonl` does NOT contain `replan_triggered`.

**Evidence**: AC 3. Validates the default threshold prevents churn from subjective LLM style preferences.

### Scenario 13b: Missing acceptance criteria section -- NeedsHuman

**`TestIntegration_MissingAcceptanceCriteria_NeedsHuman`**

Setup:
- Spec file with NO `## Acceptance Criteria` section.
- All other stages (plan, execute, validate, review) pass clean.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `needs_human`.
- AcceptStage returned `NeedsHuman` with message containing "acceptance criteria".
- No acceptance evaluation was attempted (no LLM calls for acceptance).
- `events.jsonl` contains the needs_human event with descriptive message.

**Evidence**: Fails fast, no wasted cycles on specs that cannot be evaluated.

### Scenario 13c: Parallel facet failure -- partial results

**`TestIntegration_FacetFailure_PartialResults`**

Setup:
- Policy: `review.facets: ["spec_alignment", "code_quality"]`, `max_spec_cycles: 2`.
- Cycle 1: Plan, execute, validate pass.
- Review: FakeAgent returns clean findings for `spec_alignment` but returns an error for `code_quality` (simulating API timeout).
- Acceptance: All criteria pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review` (partial review results do not block if no blocking findings from successful facets).
- `evidence/review.json` contains `spec_alignment` findings (empty/clean) and `code_quality` entry with error marker.
- `events.jsonl` review event references the partial failure.
- Human reviewer can see which facets completed and which errored.

**Evidence**: Parallel facet failure handling -- partial results preserved.

### Scenario 14: Facet enabled via config, no code change

**`TestIntegration_FacetEnabledViaConfig`**

Setup:
- Policy: `review.facets: ["spec_alignment", "code_quality", "logic_gaps"]`.
- Policy: `review.tiers: {"spec_alignment": "high", "code_quality": "medium", "logic_gaps": "high"}`.
- Cycle 1: Plan, execute, validate pass.
- Review: FakeAgent returns clean findings for all 3 facets.
- Acceptance: All pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`.
- FakeAgent was called for 3 facet reviews (not just 2).
- The `logic_gaps` facet call used model tier `"high"`.
- `evidence/review.json` contains entries for all 3 facets.
- No code changes were needed to enable the third facet -- only config.

**Evidence**: AC 6.

### Scenario 15: Fix-cycle review distinguishes new vs preexisting findings

**`TestIntegration_FixCycle_NewVsPreexistingFindings`**

Setup:
- Policy: `review.replan_threshold: "warning"` (default), `max_spec_cycles: 3`.
- Cycle 1: Plan, execute, validate pass.
- Cycle 1 review: FakeAgent returns 1 error finding (`{file: "handler.go", desc: "missing idempotency check"}`).
- Cycle 2 (fix): Fix task completes. Validation passes.
- Cycle 2 review: FakeAgent returns 2 findings:
  - `{file: "handler.go", desc: "missing idempotency check"}` -- same as cycle 1 (preexisting).
  - `{file: "middleware.go", desc: "duplicated logic"}` -- new finding.
- Since only the new finding is blocking (preexisting is excluded), replan triggered for the new finding.
- Cycle 3 (fix): Fix task resolves `middleware.go` issue. Validation passes.
- Cycle 3 review: FakeAgent returns 1 finding: `{file: "handler.go", desc: "missing idempotency check"}` -- preexisting.
- No new blocking findings -> proceeds to acceptance. All pass.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`.
- 3 planning cycles occurred.
- `evidence/review.json` contains findings with correct `cycle` and `disposition` labels:
  - Cycle 1 finding: `disposition: "new"`.
  - Cycle 2 preexisting finding: `disposition: "pre-existing"`.
  - Cycle 2 new finding: `disposition: "new"`.
  - Cycle 3 preexisting finding: `disposition: "pre-existing"`.
- Cycle 2 replan was triggered by the new finding only, not the preexisting one.
- Cycle 3 review did not trigger replan because only preexisting findings remained.

**Evidence**: AC 4.

## Evidence Checklist

Mapping each spec acceptance criterion to the test(s) that satisfy it.

| # | Acceptance Criterion | Test(s) |
|---|---------------------|---------|
| 1 | Review gate -- `ready_for_review` impossible if review finds findings above threshold | `TestSpecLoop_ReviewStage_ReplanOnBlockingFindings`, `TestFinalizeStage_ReadyForReview_RequiresAllThreeGates`, `TestFinalizeStage_NeedsHuman_WhenReviewFails`, `TestFinalizeStage_NeedsHuman_WhenAcceptanceFails`, `TestIntegration_ReviewAcceptance_HappyPath_ReadyForReview`, `TestIntegration_ReviewFinding_TriggersFixCycle` |
| 2 | Acceptance evidence -- every criterion has explicit pass/fail/unclear with rationale and evidence refs | `TestEvaluateResult_ParsePass`, `TestEvaluateResult_ParseFail`, `TestEvaluateResult_ParseUnclear`, `TestEvaluator_InvokesAgentPerCriterion`, `TestBundler_WriteAcceptance`, `TestIntegration_ReviewAcceptance_HappyPath_ReadyForReview` |
| 3 | Configurable threshold -- `review.replan_threshold` controls which severities trigger replanning | `TestThreshold_IsBlocking` (table-driven, all severity combos), `TestPolicyThresholdValidation`, `TestFilterBlockingFindings_MixedSeverities`, `TestIntegration_ThresholdError_WarningsNonBlocking` |
| 4 | Fix-cycle from review -- findings above threshold trigger fix-plan targeting specific findings | `TestSpecLoop_ReviewStage_FailureContext_CarriesFindings`, `TestReviewFailuresToStrings`, `TestFilterNewBlockingFindings_OnlyNewAboveThreshold`, `TestSpecLoop_PreexistingFindings_DontBlock`, `TestIntegration_ReviewFinding_TriggersFixCycle`, `TestIntegration_FixCycle_NewVsPreexistingFindings` |
| 5 | Fix-cycle from acceptance -- fail/unclear results trigger fix-plan targeting specific gaps | `TestSpecLoop_AcceptStage_ReplanOnFail`, `TestSpecLoop_AcceptStage_ReplanOnUnclear`, `TestSpecLoop_AcceptStage_FailureContext_CarriesCriteria`, `TestAcceptanceFailuresToStrings_Fail`, `TestAcceptanceFailuresToStrings_Unclear`, `TestIntegration_AcceptanceFail_FixCycle_ThenPass`, `TestIntegration_AcceptanceUnclear_FixAddsEvidence_ThenPass` |
| 6 | Facet configurability -- facets selected from built-in registry, enabled/disabled via policy | `TestRegistry_DefaultFacets`, `TestRegistry_AllBuiltInFacets`, `TestRegistry_SelectFacets_AllValid`, `TestRegistry_SelectFacets_UnknownFacet_ReturnsError`, `TestRunner_InvokesAllEnabledFacets`, `TestValidate_ReviewConfig_InvalidFacet`, `TestValidate_ReviewConfig_ValidCustomSelection`, `TestIntegration_FacetEnabledViaConfig` |
| 7 | Severity levels -- error/warning/suggestion/info with distinct blocking behavior | `TestSeverity_Ordering`, `TestSeverity_Parse_ValidValues`, `TestFilterBlockingFindings_MixedSeverities`, `TestSpecLoop_NewOnlyFindings_DontRetrigger` |
| 8 | VISION label deferral -- system does not auto-label as accepted | `TestSpecLoop_VisionLabelNotSet`, `TestIntegration_ReviewAcceptance_HappyPath_ReadyForReview` |
| 9 | Budget sharing -- validation/review/acceptance cycles consume from same `max_spec_cycles` | `TestSpecLoop_BudgetSharing_ValidationThenReview`, `TestSpecLoop_BudgetSharing_ValidationThenAcceptance`, `TestSpecLoop_BudgetSharing_ReviewThenAcceptance`, `TestSpecLoop_BudgetExhausted_ReviewCycles_NeedsHuman`, `TestIntegration_BudgetExhausted_ReviewAcceptance_NeedsHuman` |
| 10 | Blocked worktree preservation -- FinalizeStage preserves worktrees for `blocked` runs; InitStage cleans stale blocked worktrees on re-run | `TestFinalizeStage_PreservesWorktreeForBlocked`, `TestInitStage_CleansBlockedWorktreesForSameSpec`, `TestInitStage_IgnoresNonBlockedPriorRuns`, `TestEventContract_BlockedWorktreeCleanedEvent` |

## Test Fixtures

### New canned agent responses

Located in `internal/next/testutil/responses/`. JSON files with canned LLM outputs for review and acceptance:

```
testutil/responses/
  # Review responses
  review_clean_spec_alignment.json      # {"facet": "spec_alignment", "findings": []}
  review_clean_code_quality.json        # {"facet": "code_quality", "findings": []}
  review_clean_logic_gaps.json          # {"facet": "logic_gaps", "findings": []}
  review_1error_spec_alignment.json     # 1 error-severity finding
  review_2warnings_code_quality.json    # 2 warning-severity findings
  review_1info_code_quality.json        # 1 info-severity finding
  review_mixed_findings.json            # Mix of severities across findings
  review_invalid_malformed.json         # Unparseable JSON

  # Acceptance responses
  acceptance_pass.json                  # {"criterion": "...", "status": "pass", "rationale": "...", "evidence_refs": [...]}
  acceptance_fail.json                  # status: "fail"
  acceptance_unclear.json               # status: "unclear"
  acceptance_invalid_malformed.json     # Unparseable JSON
```

### New execution policies

```
testutil/policies/
  review_threshold_error.json           # review.replan_threshold: "error"
  review_threshold_warning.json         # review.replan_threshold: "warning" (default)
  review_threshold_suggestion.json      # review.replan_threshold: "suggestion"
  review_3facets.json                   # facets: ["spec_alignment", "code_quality", "logic_gaps"]
  tight_cycles_3.json                   # max_spec_cycles: 3
```

### Spec fixtures

```
testutil/fixtures/
  specs/
    sample_with_3ac.md                  # Spec with 3 acceptance criteria
    sample_with_no_ac.md                # Spec missing acceptance criteria section
    sample_with_empty_ac.md             # Spec with empty acceptance criteria section
```

## Test Utilities

### `internal/next/testutil/fake_review_agent.go`

```go
// FakeReviewAgent returns canned review findings per facet.
type FakeReviewAgent struct {
    mu        sync.Mutex
    Results   map[string]FacetResult // keyed by facet name
    Calls     []ReviewFacetCall
    CallOrder []string
}

type FacetResult struct {
    Findings []review.Finding
    Err      error
}

type ReviewFacetCall struct {
    FacetName string
    Prompt    string
}

func (f *FakeReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]review.Finding, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.Calls = append(f.Calls, ReviewFacetCall{FacetName: facetName, Prompt: prompt})
    f.CallOrder = append(f.CallOrder, facetName)
    result, ok := f.Results[facetName]
    if !ok {
        return nil, fmt.Errorf("FakeReviewAgent: no result for facet %q", facetName)
    }
    if result.Err != nil {
        return nil, result.Err
    }
    return result.Findings, nil
}
```

### `internal/next/testutil/fake_acceptance_evaluator.go`

```go
// FakeAcceptanceEvaluator returns a canned batch AcceptanceResult.
// Matches the batch API: Evaluate(ctx, EvaluateInput) -> (AcceptanceResult, error).
type FakeAcceptanceEvaluator struct {
    mu     sync.Mutex
    Result acceptor.AcceptanceResult
    Err    error
    Calls  []acceptor.EvaluateInput
}

func (f *FakeAcceptanceEvaluator) Evaluate(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.Calls = append(f.Calls, input)
    return f.Result, f.Err
}
```

### `internal/next/testutil/assertions.go` (additions)

```go
// AssertReviewJSON reads review.json and validates its structure.
func AssertReviewJSON(t *testing.T, evidenceDir string) map[string][]review.Finding {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(evidenceDir, "review.json"))
    if err != nil {
        t.Fatalf("reading review.json: %v", err)
    }
    var result map[string][]review.Finding
    if err := json.Unmarshal(data, &result); err != nil {
        t.Fatalf("parsing review.json: %v", err)
    }
    return result
}

// AcceptanceEnvelope is the top-level schema for acceptance.json.
type AcceptanceEnvelope struct {
    Results          []acceptor.CriterionResult `json:"results"`
    AllPass          bool                       `json:"all_pass"`
    HasFailOrUnclear bool                       `json:"has_fail_or_unclear"`
}

// AssertAcceptanceJSON reads acceptance.json and validates its envelope structure.
func AssertAcceptanceJSON(t *testing.T, evidenceDir string) AcceptanceEnvelope {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(evidenceDir, "acceptance.json"))
    if err != nil {
        t.Fatalf("reading acceptance.json: %v", err)
    }
    var envelope AcceptanceEnvelope
    if err := json.Unmarshal(data, &envelope); err != nil {
        t.Fatalf("parsing acceptance.json: %v", err)
    }
    return envelope
}

// AssertAllCriteriaPass fails if any criterion in acceptance.json does not have status "pass".
// Also validates the envelope summary fields (all_pass, has_fail_or_unclear).
func AssertAllCriteriaPass(t *testing.T, evidenceDir string) {
    t.Helper()
    envelope := AssertAcceptanceJSON(t, evidenceDir)
    if !envelope.AllPass {
        t.Fatalf("expected all_pass == true, got false")
    }
    if envelope.HasFailOrUnclear {
        t.Fatalf("expected has_fail_or_unclear == false, got true")
    }
    for _, r := range envelope.Results {
        if r.Status != "pass" {
            t.Fatalf("criterion %q has status %q, want pass", r.Criterion, r.Status)
        }
    }
}

// AssertNoVisionLabel fails if any VISION review outcome label is set.
func AssertNoVisionLabel(t *testing.T, runDir string) {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
    if err != nil {
        t.Fatalf("reading run.json: %v", err)
    }
    var m map[string]any
    if err := json.Unmarshal(data, &m); err != nil {
        t.Fatalf("parsing run.json: %v", err)
    }
    for _, key := range []string{"vision_label", "review_outcome", "vision_review_outcome"} {
        if v, ok := m[key]; ok && v != nil && v != "" {
            t.Fatalf("expected no VISION label, but found %q = %v", key, v)
        }
    }
}

// AssertReviewMDContains reads review.md and asserts it contains the given substring.
func AssertReviewMDContains(t *testing.T, evidenceDir string, substr string) {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(evidenceDir, "review.md"))
    if err != nil {
        t.Fatalf("reading review.md: %v", err)
    }
    if !strings.Contains(string(data), substr) {
        t.Fatalf("review.md does not contain %q", substr)
    }
}

// AssertFindingDisposition checks that a finding in review.json has the expected disposition.
func AssertFindingDisposition(t *testing.T, findings []review.Finding, file, descSubstr, wantDisposition string) {
    t.Helper()
    for _, f := range findings {
        if f.File == file && strings.Contains(f.Description, descSubstr) {
            if f.Disposition != wantDisposition {
                t.Fatalf("finding {file: %q, desc contains %q}: disposition = %q, want %q",
                    file, descSubstr, f.Disposition, wantDisposition)
            }
            return
        }
    }
    t.Fatalf("no finding found with file=%q and description containing %q", file, descSubstr)
}
```

## Running Tests

```bash
# Unit tests -- new packages
go test ./internal/next/review/...
go test ./internal/next/acceptor/...

# Unit tests -- extended packages
go test ./internal/next/execpolicy/...
go test ./internal/next/specloop/...
go test ./internal/next/evidence/...

# All unit tests (includes 0002a and 0002b)
go test ./internal/next/...

# Integration tests (scenarios 8-15, plus existing 1-7)
go test -tags integration ./internal/next/specloop/...

# All tests
go test -tags integration ./...

# Race detection (recommended for parallel facet invocation tests)
go test -race ./internal/next/review/...
go test -race -tags integration ./internal/next/specloop/...
```
