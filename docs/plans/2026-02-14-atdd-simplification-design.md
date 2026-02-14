# ATDD Simplification Design

## Date: 2026-02-14

## Philosophy Change

ATDD is a **specification technique**, not a **discipline technique**. Its value is that acceptance tests define target behavior before implementation starts. The red-green-refactor ceremony was designed for human developers who need discipline guardrails -- Claude doesn't need those guardrails, it needs clear contracts.

The refactor phase is a **TDD concern** (unit-level design quality), not an ATDD concern (behavioral specification). These are orthogonal methodologies that compose cleanly.

## Current State (5 invocations)

```
Write acceptance tests → verify tests fail → build (ATDD prompt) → validate → refactor → validate → review
```

Five separate Claude invocations per ATDD bead, plus a pending review gate spec that would add a sixth.

## New ATDD Flow (2 invocations)

```
Phase 1: Write acceptance tests (separate invocation)
    | commit
Phase 1.5: Run tests (cheap -- just go test)
    |-- FAIL --> proceed to Phase 2
    +-- PASS (~6%) --> haiku diagnostic: "is there a real gap?"
         |-- gap exists --> rewrite tests (retry Phase 1 once)
         +-- no gap --> ErrATDDAlreadyDone
    |
Phase 2: Build implementation (separate invocation, ATDD-aware prompt)
    |
Standard: validate --> review
```

Two invocations (down from 5). Plus one cheap haiku call in the ~6% pass-before-build case.

## Methodology Flows by Combination

| Methodology   | Flow                                                                  | Invocations     |
|---------------|-----------------------------------------------------------------------|-----------------|
| Neither       | build -> validate -> review                                           | 1 + validate    |
| ATDD only     | write acceptance tests -> build -> validate -> review                 | 2 + validate    |
| TDD only      | build -> validate -> refactor -> validate -> review                   | 2 + 2 validates |
| ATDD + TDD    | write acceptance tests -> build -> validate -> refactor -> validate -> review | 3 + 2 validates |

## Haiku Diagnostic (Pass-Before-Build)

When tests unexpectedly pass after Phase 1:

**Input:**
- Bead acceptance criteria
- Git diff of test files just committed
- Current test output (all passing)

**Question:** "These tests were meant to specify new behavior, but they all pass against the current codebase. Is there genuinely new behavior required that these tests aren't covering, or is the work already done?"

**Output:**
- `VERDICT: ALREADY_DONE` -- tests validate existing behavior, bead is complete
- `VERDICT: REWRITE` + feedback -- tests are weak, here's what they should test instead

On `REWRITE`: inject feedback into the test-writing prompt, retry Phase 1 once. If rewritten tests still pass, treat as `ALREADY_DONE`.

## Code Changes

### Remove
- `VerifyTestsFailWithRetry` -- replaced by simple test run + conditional haiku diagnostic
- Separate refactor invocation from ATDD path -- refactoring is TDD-only
- Elaborate failure context machinery from ATDD build prompt

### Simplify
- ATDD build prompt: "Acceptance tests exist, make them pass. Refactor as needed within this session"

### Keep
- Tests written in a separate invocation (structural enforcement)
- Tests committed before build starts (can't be weakened)
- `IsMethodologyActive` label/config resolution
- `ErrATDDAlreadyDone` sentinel
- ATDD and TDD as independent, composable toggles
- `IsTestOnlyBead` -- still skip ATDD for test-only beads

## Template Impact

- **`PROMPT_acceptance_tests.md`** -- Keep as-is
- **`PROMPT_atdd_build.md`** -- Simplify, remove failure context machinery
- **`PROMPT_refactor.md`** -- No change, but only invoked when TDD is active
- **`PROMPT_review_acceptance_tests.md`** -- Not needed (review gate spec retired)

## Spec and Bead Impact

- **Retire** `atdd-test-review-gate` spec + its 4 open beads
- **Update** `atdd-methodology.md` spec to reflect new philosophy and flow
- **No change** to `tdd-methodology.md` -- TDD keeps its refactor phase

## Documentation Updates

- **RULES.md** -- Update methodology patterns section
- **LEARNINGS.md** -- Add learning: ATDD as specification vs discipline
