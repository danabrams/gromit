---
id: atdd-simplification
source_ideas: []
created: 2026-02-14
supersedes: [atdd-test-review-gate]
updates: [atdd-methodology]
epic: spec-first-atdd-execution
---

# ATDD Simplification

## Specification

Reframe ATDD from a discipline technique (red-green-refactor ceremony) to a specification technique (tests define target behavior). Reduce ATDD invocations from 5 to 2 per bead. Move the refactor phase to TDD-only. Retire the atdd-test-review-gate spec. Replace the verify-tests-fail retry chain with a cheap haiku diagnostic that only fires when tests unexpectedly pass.

### Philosophy

ATDD's value is that acceptance tests define target behavior before implementation starts. The strict red-green-refactor cycle was designed for human developers who need discipline guardrails -- humans skip tests, write implementation first, procrastinate on refactoring. Claude does what it's told. The ceremony solves a human motivation problem that Claude doesn't have.

What Claude needs is clear behavioral contracts. ATDD provides that through tests-as-specification. The refactor phase is a unit-level design concern that belongs to TDD, not ATDD.

### New ATDD Flow

```
Phase 1: Write acceptance tests (separate invocation, same model as build)
    | commit tests
Phase 1.5: Run tests (go test, no Claude invocation)
    |-- FAIL --> proceed to Phase 2
    +-- PASS --> haiku diagnostic
         |-- VERDICT: REWRITE + feedback --> retry Phase 1 once
         +-- VERDICT: ALREADY_DONE --> ErrATDDAlreadyDone
Phase 2: Build implementation (separate invocation, ATDD-aware prompt)
    |
Standard: validate --> review
```

Two Claude invocations in the normal case. One additional haiku call in the ~6% pass-before-build case.

### What Changes

**Remove from ATDD path:**
- `VerifyTestsFailWithRetry` -- replaced by a simple test run plus conditional haiku diagnostic
- Separate refactor invocation -- refactoring belongs to TDD only
- Elaborate failure context machinery in the ATDD build prompt

**Simplify:**
- ATDD build prompt (`PROMPT_atdd_build.md`): core message becomes "Acceptance tests are committed. Implement to make them pass. Refactor as needed within this session"

**Keep unchanged:**
- Tests written in a separate invocation (structural enforcement -- can't be weakened)
- Tests committed before build starts
- `IsMethodologyActive` label/config resolution
- `ErrATDDAlreadyDone` sentinel for beads where work is already done
- ATDD and TDD as independent, composable toggles
- `IsTestOnlyBead` -- skip ATDD for test-only beads
- `PROMPT_acceptance_tests.md` -- no changes to how tests are written

### Methodology Flow Matrix

| Methodology | Flow | Claude invocations |
|-------------|------|--------------------|
| Neither | build -> validate -> review | 1 |
| ATDD only | write tests -> build -> validate -> review | 2 |
| TDD only | build -> validate -> refactor -> validate -> review | 2 |
| ATDD + TDD | write tests -> build -> validate -> refactor -> validate -> review | 3 |

### Haiku Diagnostic (Pass-Before-Build)

When tests unexpectedly pass after Phase 1, a single haiku invocation diagnoses the situation.

**Input:**
- Bead acceptance criteria and description
- Git diff of test files just committed
- Test output showing all tests passing

**Prompt:** "These tests were meant to specify new behavior, but they all pass against the current codebase. Is there genuinely new behavior required that these tests aren't covering, or is the work already done?"

**Output (structured verdict):**
- `VERDICT: ALREADY_DONE` -- tests validate existing behavior, bead is complete
- `VERDICT: REWRITE` + feedback -- tests are weak, feedback explains what they should test instead

**On REWRITE:** inject feedback into the acceptance test prompt's FailureContext, retry Phase 1 once. If rewritten tests still pass, treat as ALREADY_DONE.

**Cost:** one haiku call on ~6% of ATDD beads. Replaces the current 3-5 invocation retry chain.

### Specs Affected

- **Supersedes** `atdd-test-review-gate` -- the haiku-on-pass diagnostic replaces the pre-verify review gate. Close the spec and its 4 open beads.
- **Updates** `atdd-methodology` -- the canonical ATDD flow changes. Acceptance criteria 6-8 (verify-fail, retry on pass, refactor phase) are replaced by this spec's simplified flow.

## Acceptance Criteria

- ATDD beads use exactly two Claude invocations in the normal case: write acceptance tests, then build
- After writing tests, Gromit runs validation commands (no Claude invocation) to check if tests fail
- When tests fail (expected), Gromit proceeds directly to the build phase
- When tests pass (unexpected), a single haiku invocation diagnoses whether tests are weak or work is already done
- On haiku REWRITE verdict, acceptance tests are rewritten once with the feedback, then re-checked
- On haiku ALREADY_DONE verdict (or second pass still passes), the bead completes with ErrATDDAlreadyDone
- The refactor phase does not run for ATDD-only beads; it runs only when TDD is active
- The ATDD build prompt tells Claude that acceptance tests exist and must be made to pass, without the verify-fail failure context machinery

## Decisions

1. **Tests-as-specification, not red-green-refactor.** ATDD's value is defining behavioral contracts before implementation. The ceremony of verify-fail, separate build, and separate refactor was borrowed from human TDD discipline that doesn't apply to an AI builder.

2. **Structural enforcement through separate invocations.** Writing tests in a separate invocation from implementation prevents Claude from retroactively weakening tests. The tests are committed to git before the build session starts. This is structural, not disciplinary.

3. **Haiku diagnostic only on unexpected pass.** Running a haiku review on every ATDD bead (as atdd-test-review-gate proposed) adds fixed cost. Running it only when tests pass (~6% of beads) is cheaper and targets the actual problem.

4. **Refactor belongs to TDD.** Refactoring is a unit-level design concern about code quality after tests pass. It's not part of the behavioral specification workflow. Moving it to TDD-only cleanly separates the two methodologies.

5. **One rewrite maximum.** If rewritten tests still pass, treat as already done rather than retrying indefinitely. The haiku diagnostic is preventive, not a hard gate.

## Research & Context

### Current State

The ATDD workflow currently runs 5 invocations per bead:
1. Write acceptance tests (methodology executor)
2. Verify tests fail with retry (3-5 invocations on false positive)
3. Build with ATDD prompt (separate from standard build)
4. Validate
5. Refactor (separate invocation, shared with TDD)

The atdd-test-review-gate spec (pending, 4 open beads) would add a 6th invocation (haiku review before verify-fail on every bead).

### Key Files

- `internal/runner/methodology/executor.go` -- RunAcceptanceTests, VerifyTestsFail, refactor orchestration
- `internal/runner/methodology/refactor.go` -- VerifyTestsFailWithRetry, refactor retry logic
- `internal/runner/runner.go` -- processBead() ATDD flow sequencing
- `internal/prompt/prompt.go` -- RenderATDDBuild, RenderAcceptanceTests
- `.gromit/templates/PROMPT_atdd_build.md` -- ATDD build template
- `.gromit/templates/PROMPT_acceptance_tests.md` -- acceptance test writing template
- `.gromit/templates/PROMPT_refactor.md` -- refactor template (moves to TDD-only)
