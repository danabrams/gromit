---
id: atdd-test-review-gate
source_ideas: []
created: 2026-02-12
---

# ATDD Test Review Gate

## Specification

After acceptance tests are written (Phase 1 of the ATDD workflow) and before verifying that they fail, Gromit inserts a lightweight review step that evaluates whether the tests actually require new behavior. This catches weak or tautological tests early — before they trigger the expensive verify-fail retry chain — and reduces the 6.2% false positive rate where acceptance tests pass before implementation.

### The Problem

In 13 of 210 observed ATDD iterations, acceptance tests passed before implementation began. The current mitigation (`verifyTestsFailWithRetry`) is reactive: it discovers the problem after running validation, then retries test writing with analysis context, checks the git diff, and potentially retries again — consuming 3-5 invocations before concluding `errATDDAlreadyDone`. This wastes tokens and time, and worse, some false positives may silently close beads whose work was never done.

### The Solution

A single haiku invocation between "write acceptance tests" and "verify tests fail" that reviews the written tests against the bead's acceptance criteria. The review answers one question: **do these tests require behavior that does not currently exist in the codebase?**

### Workflow Change

Current ATDD flow:
1. Write acceptance tests (build-tier model)
2. Verify tests fail (direct shell validation)
3. Build (build-tier model)

New ATDD flow:
1. Write acceptance tests (build-tier model)
2. **Review acceptance tests (haiku)** — new step
3. Verify tests fail (direct shell validation)
4. Build (build-tier model)

### Review Step Behavior

The review invocation receives:
- The bead's acceptance criteria and description
- The git diff of test files written in Phase 1 (from `bc.startCommit`)
- A focused prompt asking it to evaluate each test against two criteria:
  1. Does the test assert on behavior that requires code changes to pass?
  2. Does the test reference functions, methods, types, or constants that do not yet exist?

The review outputs a structured verdict: `pass` (tests look good, proceed to verify-fail) or `fail` with a description of which tests are weak and why.

### On Review Failure

When the review identifies weak tests:
- The review's feedback is injected into `bc.promptCtx.FailureContext`
- Acceptance tests are rewritten via `runAcceptanceTests` with the review feedback as context
- The review runs again on the rewritten tests
- If the second review also fails, proceed to verify-tests-fail anyway (don't block indefinitely — the existing retry chain serves as a backstop)

Maximum one rewrite cycle through the review gate. The gate is preventive, not a hard block.

### Review Prompt

A new template `PROMPT_review_acceptance_tests.md` with a narrow, focused job:

```
You are reviewing acceptance tests written for an ATDD workflow. Your job is to determine whether these tests actually require new behavior that does not exist yet.

## Bead
**Title:** {{.Bead.Title}}
**Description:** {{.Bead.Description}}

## Acceptance Criteria
{{.AcceptanceCriteria}}

## Tests Written (git diff)
{{.TestDiff}}

## Review Criteria

For each test, evaluate:
1. Does it assert on behavior that requires implementation changes to pass?
2. Does it reference functions, methods, types, or constants that do not currently exist?
3. Would it pass against the current codebase without any changes?

A test that would pass against the current codebase is testing existing behavior, not new behavior. It must be rewritten.

## Output

Respond with exactly one of:
- VERDICT: PASS — if all tests require new behavior
- VERDICT: FAIL — followed by a description of which tests are weak and what they should test instead
```

### Cost Model

- The review uses haiku (cheapest tier), regardless of the bead's build model
- It receives only the diff and bead context, not the full codebase — small input
- One haiku call per ATDD bead (~100% of ATDD beads)
- Eliminates the expensive retry chain on false positives (~6.2% of ATDD beads, 3-5 invocations each)
- Net effect: small fixed cost per bead, large savings on false positive beads

### Logging

The review result is logged in the iteration JSONL with:
- `atdd_review_verdict`: "pass" or "fail"
- `atdd_review_rewrite`: boolean, whether a rewrite was triggered

This provides data to track the false positive rate over time and evaluate whether the gate is effective.

## Acceptance Criteria

- After acceptance tests are written, a haiku review invocation evaluates whether tests require new behavior
- The review receives the bead's acceptance criteria and the git diff of written tests
- When the review verdict is "pass", processing continues to verify-tests-fail as before
- When the review verdict is "fail", acceptance tests are rewritten with the review feedback as context
- After one rewrite, if the review still fails, processing continues to verify-tests-fail anyway (no hard block)
- The review step uses the haiku tier regardless of the bead's priority or label overrides
- Review verdict and rewrite status are logged in the iteration JSONL
- The review prompt template exists at `.gromit/templates/PROMPT_review_acceptance_tests.md`

## Decisions

1. **Separate haiku invocation, not self-review.** The model that wrote the tests cannot reliably evaluate its own work — it's checking its own homework. A fresh context with a different (cheaper) model provides genuine second-pair-of-eyes review. This aligns with Gromit's core principle of fresh context per invocation.

2. **Always haiku for the review.** The review task is narrow and well-defined — read a diff, compare to criteria, output a verdict. It doesn't need opus-level reasoning. Keeping it on haiku minimizes the per-bead cost increase.

3. **Diff-based input, not full codebase.** The review only needs to see what tests were written and what they should test. Feeding the full codebase would increase cost and dilute focus. The diff from `bc.startCommit` provides exactly the right scope.

4. **One rewrite maximum.** The gate is preventive, not a hard block. If the review fails twice, we fall through to the existing verify-tests-fail machinery, which serves as a backstop. This prevents the review gate from becoming its own infinite loop.

5. **Run on every ATDD bead, not just suspected false positives.** We can't predict which beads will false-positive. The cost of a haiku call is low enough that running it universally is cheaper than the retry chain it prevents on 6.2% of beads.

## Research & Context

### Current State

The ATDD workflow in `internal/runner/runner.go:728-746` sequences: `runAcceptanceTestsWithRetry` → `verifyTestsFailWithRetry`. The review gate inserts between these two steps.

The false positive handling in `verifyTestsFailWithRetry` (`internal/runner/process.go:791-848`) currently:
1. Runs analysis on unexpected pass
2. Retries acceptance test writing with analysis context
3. Checks if only test files changed (diff-aware detection)
4. Falls back to `errATDDAlreadyDone` sentinel

With the review gate, most false positives should be caught before reaching `verifyTestsFailWithRetry`, making the existing retry chain fire less often. The existing chain remains as a backstop — it is not removed.

### Key Files

- `internal/runner/runner.go` — `processBead()`: insert review call between lines 734 and 737
- `internal/runner/process.go` — add `reviewAcceptanceTests()` method alongside existing ATDD methods
- `internal/runner/interfaces.go` — add `RenderReviewAcceptanceTests()` to `PromptRenderer` interface
- `internal/prompt/prompt.go` — add render method and context type for the review template
- `.gromit/templates/PROMPT_review_acceptance_tests.md` — new template
- `internal/runner/runner.go` — iteration logging: add review verdict fields

### Existing Patterns

- The review follows the same provider routing pattern as other phases: `r.router.Select(phase, tier)` with tier forced to haiku
- Structured verdict parsing can follow the pattern used by `claude.IsValidationPassed()` — check output for a known marker string
- The `FailureContext` injection for rewrites follows the same pattern already used in `verifyTestsFailWithRetry` (process.go:814, 834)
