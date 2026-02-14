---
id: atdd-simplification
source_spec: atdd-simplification
created: 2026-02-14
decomposed: true
---

# ATDD Simplification Implementation Plan

**Goal:** Replace the multi-invocation verify-tests-fail retry chain with a direct test run plus conditional haiku diagnostic, and gate refactoring on TDD-only.

**Architecture:** New `CheckTestsFailWithDiagnostic` method on Executor runs validation commands directly; on unexpected pass, calls a haiku diagnostic that returns ALREADY_DONE or REWRITE verdict. processBead orchestrates a single rewrite retry. Refactor phase gates on `tddActive` only.

**Tech Stack:** Go, text/template

**Spec:** `.gromit/specs/atdd-simplification.md`

---

## Architecture

**Key Components:**

1. **`DiagnosticContext` (prompt package)**: New struct holding bead title, description, acceptance criteria, test diff, and test output. Used by the new diagnostic template.

2. **`PROMPT_atdd_diagnostic.md` (template)**: Haiku-tier prompt that diagnoses whether tests are weak or work is already done. Outputs structured `VERDICT: ALREADY_DONE` or `VERDICT: REWRITE` with feedback.

3. **`CheckTestsFailWithDiagnostic` (methodology/executor.go)**: New method replacing `VerifyTestsFailWithRetry`. Runs validation commands directly; on unexpected pass, calls haiku diagnostic. Returns `nil` (tests fail), `ErrATDDAlreadyDone`, or `ErrATDDRewrite` (new error type carrying feedback).

4. **Updated `processBead` (runner.go)**: Orchestrates new flow: write tests → check tests with diagnostic → handle rewrite (single retry) → build → validate. Refactor gate changes from `atddActive || tddActive` to `tddActive`.

**Integration Points:**
- `makeMethodologyExec` gets a new `diagnosticInvokeFn` callback wired to the router at `TierLow`
- `PromptRenderer` interface gains `RenderATDDDiagnostic` method
- `processBead` replaces `VerifyTestsFailWithRetry` call with `CheckTestsFailWithDiagnostic` + rewrite handling
- Refactor gate narrowed from `atddActive || tddActive` to `tddActive`

**Data Flow (normal case):**
```
processBead → RunAcceptanceTestsWithRetry → (tests written + committed)
           → CheckTestsFailWithDiagnostic → validateFn(commands) → tests fail → return nil
           → RenderATDDBuild → build prompt
           → ExecuteWithRetry → implementation
           → runValidationWithRecovery → validate
           → [TDD only] RunRefactorPhase
```

**Data Flow (pass-before-build):**
```
processBead → RunAcceptanceTestsWithRetry → (tests written)
           → CheckTestsFailWithDiagnostic → validateFn → tests PASS
              → getDiffFn → test diff
              → renderDiagnosticFn → haiku prompt
              → diagnosticInvokeFn(TierLow) → haiku call
              → parseVerdict → REWRITE + feedback
           → inject feedback → RunAcceptanceTests once
           → CheckTestsFailWithDiagnostic again
              → validateFn → tests fail → proceed to build
              OR → tests still pass → ErrATDDAlreadyDone (no second diagnostic)
```

**Tradeoffs:**
- Reuse `RefactorInvokeFn` signature `func(ctx, prompt, tier) (*claude.Result, error)` for diagnostic callback — same pattern, separate field for clarity
- Verdict as error types (like existing `ErrATDDAlreadyDone`) keeps method signature simple and matches codebase conventions
- Keep `VerifyTestsFail` temporarily during migration, clean up in final task

## Test Strategy

**Unit Tests:**
- `CheckTestsFailWithDiagnostic`: tests fail → nil; tests pass + ALREADY_DONE; tests pass + REWRITE; diagnostic fails → fallback to ALREADY_DONE
- `parseVerdict`: ALREADY_DONE, REWRITE + feedback, no marker → default ALREADY_DONE
- `ErrATDDRewrite`: error wrapping, feedback extraction via `AsATDDRewrite`

**Integration Tests:**
- `processBead` ATDD-only: refactor phase does NOT run
- `processBead` ATDD+TDD: refactor phase DOES run
- `processBead` rewrite flow: REWRITE → retry → tests fail → proceeds to build
- `processBead` rewrite flow: REWRITE → retry → still passes → AlreadyDone

**Template Tests:**
- `RenderATDDDiagnostic` renders with sample DiagnosticContext, output contains bead info and verdict instructions

**Mocking Strategy:**
- Mock callbacks on Executor (`diagnosticInvokeFn`, `validateFn`, `renderFn`, `invokeFn`, `getDiffFn`) — existing pattern from `methodology_test.go`
- Mock `PromptRenderer` in runner tests — existing `mockPromptRenderer` pattern

**Test Organization:**
- `internal/runner/methodology/methodology_test.go` — CheckTestsFailWithDiagnostic, verdict parsing
- `internal/runner/runner_test.go` — processBead flow (refactor gate, rewrite retry)
- `internal/prompt/prompt_test.go` — RenderATDDDiagnostic template rendering

## Implementation Tasks

### Task 1: Add diagnostic types, template, and render method

**Files:**
- Create: `.gromit/templates/PROMPT_atdd_diagnostic.md`
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/interfaces.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Add `DiagnosticContext` struct to prompt package with fields: `BeadTitle`, `BeadDescription`, `AcceptanceCriteria` (string), `TestDiff`, `TestOutput`. Create `PROMPT_atdd_diagnostic.md` template that presents the bead context, test diff, and test output, then asks for a structured `VERDICT: ALREADY_DONE` or `VERDICT: REWRITE` with feedback. Add `RenderATDDDiagnostic(ctx *DiagnosticContext) (string, error)` to `Renderer` and to `PromptRenderer` interface. Update mock implementations.

**Acceptance Criteria:**
- `RenderATDDDiagnostic` renders template with all DiagnosticContext fields populated
- Template output includes bead title, test diff, test output, and verdict format instructions
- `PromptRenderer` interface includes `RenderATDDDiagnostic`

**Dependencies:** None

**Notes:** The template should instruct haiku to output exactly `VERDICT: ALREADY_DONE` or `VERDICT: REWRITE` on its own line, followed by feedback text for REWRITE. Keep the template concise — haiku needs clear, simple instructions.

### Task 2: Add ErrATDDRewrite type and verdict parsing

**Files:**
- Modify: `internal/runner/methodology/refactor.go`
- Test: `internal/runner/methodology/methodology_test.go`

**What to Do:**
Add `ErrATDDRewrite` error type with a `Feedback` field alongside existing `ErrATDDAlreadyDone`. Add `AsATDDRewrite(err) (*ErrATDDRewrite, bool)` helper using `errors.As`. Add `parseDiagnosticVerdict(output string) (verdict string, feedback string)` function that scans output for `VERDICT: ALREADY_DONE` or `VERDICT: REWRITE` markers and extracts feedback text after REWRITE. Default to ALREADY_DONE when no marker found.

**Acceptance Criteria:**
- `ErrATDDRewrite` carries feedback string and satisfies `error` interface
- `AsATDDRewrite` extracts the error from wrapped chains
- `parseDiagnosticVerdict` correctly parses both verdict types and extracts feedback
- Missing/malformed verdict defaults to ALREADY_DONE

**Dependencies:** None

### Task 3: Add CheckTestsFailWithDiagnostic to Executor

**Files:**
- Modify: `internal/runner/methodology/executor.go`
- Test: `internal/runner/methodology/methodology_test.go`

**What to Do:**
Add `DiagnosticInvokeFn` type (same signature as `RefactorInvokeFn`), `diagnosticInvokeFn` field, `renderDiagnosticFn` field (type `RenderDiagnosticFn func(*prompt.DiagnosticContext) (string, error)`), and `SetDiagnosticDeps` setter. Implement `CheckTestsFailWithDiagnostic(ctx, bc) error` that: (1) runs `validateFn` to check if tests pass/fail, (2) on fail returns nil, (3) on pass gets test output from validation result and test diff via `getDiffFn`, (4) builds DiagnosticContext from bc.Bead fields, (5) renders diagnostic prompt via `renderDiagnosticFn`, (6) calls `diagnosticInvokeFn` with TierLow, (7) parses verdict, (8) returns `ErrATDDAlreadyDone` or `ErrATDDRewrite`. If diagnostic invocation fails, falls back to `ErrATDDAlreadyDone`.

**Acceptance Criteria:**
- Tests fail (expected) → returns nil
- Tests pass + ALREADY_DONE verdict → returns `ErrATDDAlreadyDone`
- Tests pass + REWRITE verdict → returns `ErrATDDRewrite` with feedback
- Diagnostic invocation failure → falls back to `ErrATDDAlreadyDone`

**Dependencies:** Task 1 (DiagnosticContext), Task 2 (verdict parsing, error types)

### Task 4: Wire diagnostic into processBead and change refactor gate

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
In `makeMethodologyExec`, wire `diagnosticInvokeFn` (calls router at TierLow, same pattern as refactorInvokeFn) and `renderDiagnosticFn` (calls `r.renderer.RenderATDDDiagnostic`). In `processBead`, replace `VerifyTestsFailWithRetry` call with `CheckTestsFailWithDiagnostic`. Handle the result: on nil proceed to build; on `ErrATDDAlreadyDone` return success+AlreadyDone; on `ErrATDDRewrite` inject feedback into `bc.PromptCtx.FailureContext`, call `RunAcceptanceTests` once, then run `CheckTestsFailWithDiagnostic` again — but on second pass, skip the diagnostic (if tests still pass, return AlreadyDone directly). Change refactor gate from `atddActive || tddActive` to `tddActive`. Clean up FailureContext assignment before RenderATDDBuild (remove the hardcoded string, just clear retry flags).

**Acceptance Criteria:**
- ATDD-only beads: refactor phase does NOT run
- ATDD+TDD beads: refactor phase DOES run
- Rewrite flow: REWRITE → retry acceptance tests → tests fail → proceeds to build
- Rewrite flow: REWRITE → retry → still passes → returns AlreadyDone
- Normal flow: tests fail → build proceeds without diagnostic invocation

**Dependencies:** Task 3 (CheckTestsFailWithDiagnostic)

### Task 5: Remove deprecated VerifyTestsFail code

**Files:**
- Modify: `internal/runner/methodology/executor.go`
- Modify: `internal/runner/methodology/refactor.go`
- Modify: `internal/runner/methodology/methodology_test.go`

**What to Do:**
Remove `VerifyTestsFail` method from executor.go. Remove `VerifyTestsFailWithRetry` method from refactor.go. Remove `analyzeFn` field and `SetAnalyzeFn` setter from executor.go (no longer needed for ATDD path — analysis was only used by VerifyTestsFailWithRetry). Remove `NewExecutorWithAnalysis` constructor. Remove `AnalyzeFn` type if no other consumers exist. Remove associated tests. Keep `ErrATDDAlreadyDone`, `IsATDDAlreadyDone`, and all refactor-related code intact.

**Acceptance Criteria:**
- `VerifyTestsFail` and `VerifyTestsFailWithRetry` methods are removed
- `analyzeFn` field and setter removed from Executor
- All tests pass with removed code
- `ErrATDDAlreadyDone` and refactor code remain intact

**Dependencies:** Task 4 (new flow fully wired, old methods no longer called)

### Task 6: Close superseded atdd-test-review-gate beads

**Files:** None (bd CLI operations only)

**What to Do:**
Close the 4 open beads from the atdd-test-review-gate spec as superseded: gromit-1xsu (wire review gate), gromit-r2ys (reviewAcceptanceTests), gromit-pb5u (review prompt template), gromit-nv2p (logging fields and review context). Add comments explaining they are superseded by atdd-simplification.

**Acceptance Criteria:**
- All 4 beads are closed with superseded comments
- No open beads remain for atdd-test-review-gate spec

**Dependencies:** Task 4 (new flow wired, confirming the old approach is replaced)

---

## Notes

- The `RunAcceptanceTestsWithRetry` method is unchanged — it handles the retry/escalation chain for writing tests, which is orthogonal to the verify-fail simplification.
- The `PROMPT_acceptance_tests.md` template is unchanged per spec.
- The `PROMPT_atdd_build.md` template is already clean — the "failure context machinery" is in the processBead code that sets `bc.PromptCtx.FailureContext`, not in the template itself.
- The second pass through `CheckTestsFailWithDiagnostic` after a rewrite should skip the haiku diagnostic — if tests still pass after rewrite, treat as ALREADY_DONE without burning another invocation. This can be done by passing a flag or by having processBead just run a plain validation check on the second pass.
- The `IsTestOnlyDiff` check in the old `VerifyTestsFailWithRetry` is subsumed by the haiku diagnostic — haiku will naturally notice if tests are checking existing behavior.
