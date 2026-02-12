---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T09:32:22-05:00"
id: atdd-test-review-gate
source_spec: atdd-test-review-gate
---

# ATDD Test Review Gate Implementation Plan

**Goal:** Insert a lightweight haiku review step between "write acceptance tests" and "verify tests fail" to catch weak/tautological tests before the expensive verify-fail retry chain fires.

**Architecture:** A single haiku invocation reviews the test diff against acceptance criteria and outputs a pass/fail verdict. On failure, tests are rewritten once with feedback, then the flow falls through to verify-tests-fail regardless.

**Tech Stack:** Go, text/template, existing provider/router infrastructure

**Spec:** `.gromit/specs/atdd-test-review-gate.md`

---

## Architecture

**Overview:**
Insert a review gate between Phase 1 (write acceptance tests) and Phase 2 (verify tests fail) in the ATDD workflow. The gate uses a fresh haiku invocation to evaluate whether written tests require new behavior.

**Key Components:**
1. **`reviewAcceptanceTests()` method** (on `Runner` in `process.go`): Gets the test diff, renders the review prompt, runs a haiku invocation, and parses the verdict.
2. **`RenderReviewAcceptanceTests()` method** (on `Renderer` in `prompt.go`): Renders the review template with bead info and test diff.
3. **`PROMPT_review_acceptance_tests.md`** template: Focused prompt for evaluating test quality.
4. **Logging fields**: `ATDDReviewVerdict` and `ATDDReviewRewrite` on `IterationResult` and `IterationLog`.

**Integration Points:**
- `processBead()` in `runner.go` (lines 734-736): Insert review gate call between `runAcceptanceTestsWithRetry` and `verifyTestsFailWithRetry`
- `PromptRenderer` interface in `interfaces.go`: Add `RenderReviewAcceptanceTests()` method
- Router: Use `r.router.Select("review_acceptance_tests", provider.TierLow)` — forces haiku

**Data Flow:**
1. Acceptance tests written (Phase 1, existing)
2. `reviewAcceptanceTests()` called:
   - Gets git diff from `bc.startCommit` via `r.getDiff()`
   - Creates `ReviewAcceptanceTestsContext` with bead info and diff
   - Renders prompt via `r.renderer.RenderReviewAcceptanceTests()`
   - Selects provider via `r.router.Select("review_acceptance_tests", provider.TierLow)`
   - Calls `p.Run()` (non-streaming — output is short)
   - Parses verdict: checks for `"VERDICT: PASS"` / `"VERDICT: FAIL"`
3. On pass: returns nil → proceed to verify-tests-fail
4. On fail: returns error with feedback → `processBead()` triggers rewrite loop (max 1 cycle)

**Tradeoffs:**
- **`Run()` over `StreamRun()`**: Review output is a short verdict, not long code generation. Simpler, no heartbeat/stall detection needed.
- **Dedicated context type over reusing `prompt.Context`**: Review needs test diff (not in `prompt.Context`), doesn't need spec/learnings. Clean separation.
- **String marker parsing over JSON**: Follows `claude.IsValidationPassed()` pattern. Simpler, more robust with haiku than requiring valid JSON.

## Test Strategy

**Unit Tests:**
- `parseReviewVerdict()`: all verdict parsing edge cases (pass, fail, mixed case, missing, empty)
- `reviewAcceptanceTests()`: pass flow, fail flow, diff failure, render failure, provider failure
- `RenderReviewAcceptanceTests()`: template rendering with bead info and diff

**Integration Tests:**
- Review gate in `processBead()` — pass: acceptance tests → review passes → verify-tests-fail
- Review gate in `processBead()` — fail then pass: review fails → rewrite → review passes → continue
- Review gate in `processBead()` — fail twice: review fails → rewrite → fails again → falls through

**Mocking:**
- Mock `PromptRenderer`: add `RenderReviewAcceptanceTests()` to existing mock
- Mock provider via `router.Select()`: return mock with configurable verdict
- Mock `getDiff()` via `r.gitDiffFn` hook: return configurable diff

**Coverage Goals:**
- All verdict branches, rewrite loop branches, error paths, logging propagation

## Implementation Tasks

### Task 1: Add logging fields and review context type

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/runner.go` (IterationResult struct + writeIterationLog)
- Modify: `internal/prompt/prompt.go` (new context type)

**What to Do:**
Add `ATDDReviewVerdict` (string) and `ATDDReviewRewrite` (bool) fields to both `IterationLog` and `IterationResult`. Wire them through `writeIterationLog()`. Add `ReviewAcceptanceTestsContext` struct to the prompt package with fields: `BeadTitle`, `BeadDescription`, `AcceptanceCriteria`, `TestDiff`.

**Acceptance Criteria:**
- `IterationLog` has `atdd_review_verdict` and `atdd_review_rewrite` JSON fields
- `IterationResult` has matching `ATDDReviewVerdict` and `ATDDReviewRewrite` fields
- `writeIterationLog()` populates the new fields from `IterationResult`
- `ReviewAcceptanceTestsContext` exists in the prompt package with bead info and diff fields

**Dependencies:**
- None (foundational)

### Task 2: Add prompt template and render method

**Files:**
- Create: `.gromit/templates/PROMPT_review_acceptance_tests.md`
- Modify: `internal/prompt/prompt.go` (add RenderReviewAcceptanceTests method)
- Modify: `internal/runner/interfaces.go` (add to PromptRenderer interface)
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Create the review prompt template per the spec (bead title, description, acceptance criteria, test diff, review criteria, structured verdict output). Add `RenderReviewAcceptanceTests(ctx *ReviewAcceptanceTestsContext) (string, error)` to the `Renderer` and to the `PromptRenderer` interface. Update any mock implementations of `PromptRenderer` to include the new method.

**Acceptance Criteria:**
- Template exists at `.gromit/templates/PROMPT_review_acceptance_tests.md` with the verdict format specified in the spec
- `RenderReviewAcceptanceTests()` renders the template with context data
- `PromptRenderer` interface includes the new method
- All mock implementations compile with the new method

**Dependencies:**
- Task 1 (provides `ReviewAcceptanceTestsContext`)

### Task 3: Implement `reviewAcceptanceTests()` and verdict parsing

**Files:**
- Modify: `internal/runner/process.go` (add reviewAcceptanceTests method + parseReviewVerdict helper)
- Test: `internal/runner/process_test.go`

**What to Do:**
Add `reviewAcceptanceTests(ctx, bc) error` method on `Runner`. It: (1) gets diff via `r.getDiff(bc.startCommit)`, returning nil on diff failure (skip review gracefully), (2) builds `ReviewAcceptanceTestsContext` from bead info and diff, (3) renders prompt via `r.renderer.RenderReviewAcceptanceTests()`, (4) selects provider via `r.router.Select("review_acceptance_tests", provider.TierLow)`, (5) calls `p.Run()`, (6) parses verdict via `parseReviewVerdict()`. Returns nil on pass, error with feedback on fail. Add `parseReviewVerdict(output string) (passed bool, feedback string)` — checks for "VERDICT: PASS" / "VERDICT: FAIL", defaults to pass on unparseable output.

**Acceptance Criteria:**
- `reviewAcceptanceTests()` calls the provider with TierLow and returns nil on pass, error on fail
- `parseReviewVerdict()` correctly parses pass, fail, and malformed output (defaults to pass)
- Diff failure or provider failure causes the review to be skipped (returns nil), not to block

**Dependencies:**
- Task 2 (provides render method and interface)

### Task 4: Wire review gate into `processBead()` ATDD flow

**Files:**
- Modify: `internal/runner/runner.go` (processBead method)
- Test: `internal/runner/runner_test.go`

**What to Do:**
Insert the review gate between `runAcceptanceTestsWithRetry` and `verifyTestsFailWithRetry` in `processBead()`. The logic: (1) call `reviewAcceptanceTests()`, (2) if it returns nil (pass), proceed to verify-tests-fail, (3) if it returns error (fail), inject feedback into `bc.promptCtx.FailureContext`, set `bc.result.ATDDReviewRewrite = true`, call `runAcceptanceTests()` to rewrite, then call `reviewAcceptanceTests()` again, (4) regardless of second review result, proceed to verify-tests-fail. Set `bc.result.ATDDReviewVerdict` to the final verdict. Clear retry flags before proceeding to verify-tests-fail.

**Acceptance Criteria:**
- When review passes, flow continues to verify-tests-fail without rewriting
- When review fails once, tests are rewritten with feedback and reviewed again
- When review fails twice, flow continues to verify-tests-fail anyway (no hard block)
- Review verdict and rewrite status are recorded on `bc.result` and appear in iteration logs

**Dependencies:**
- Task 3 (provides `reviewAcceptanceTests()`)

---

## Notes

- The review gate is purely additive — the existing `verifyTestsFailWithRetry` retry chain remains as a backstop and is not modified.
- The `parseReviewVerdict()` function defaults to "pass" on unparseable output. This is intentional: the gate is preventive, not a hard block. If haiku produces unexpected output, we'd rather proceed than block.
- The review uses `p.Run()` (non-streaming) since the output is just a verdict. No need for heartbeat monitoring, stall detection, or streaming stats.
- When the review gate is skipped (diff failure, provider unavailable), the `ATDDReviewVerdict` field should be empty string, indicating the review didn't run.
- The prompt template should match the format in the spec exactly, including the "VERDICT: PASS" / "VERDICT: FAIL" output format.
