---
id: tdd-methodology
source_ideas: [idea-1770457558357]
created: 2026-02-07
---

# TDD Methodology Support

## Specification

Gromit gains Test-Driven Development (TDD) support as an independent toggle alongside ATDD. While ATDD defines the behavioral contract through acceptance tests, TDD drives problem decomposition and implementation quality through tight red-green-refactor cycles. Both can be enabled independently or together.

### Configuration

The `methodology` section in `gromit.yaml` uses independent boolean toggles:

```yaml
methodology:
  atdd: false   # See atdd-methodology spec
  tdd: false    # This spec
```

Per-bead override via labels: `tdd:true` or `tdd:false` on a bead overrides the global default. When a bead is automatically decomposed, sub-beads inherit the parent's methodology labels.

### TDD Workflow

When TDD is active for a bead, the build phase changes from a standard single-pass implementation to a red-green-refactor cycle, and a mandatory refactor phase is added after validation.

#### Build Phase (Red-Green Cycles)

A single Claude invocation receives a prescriptive TDD prompt that demands the red-green cycle:

1. **Red** — Write one small failing unit test that addresses a piece of the bead's requirements
2. **Green** — Write the minimum code to make the test pass
3. **Commit** — Commit the test + implementation together
4. **Repeat** — Move to the next piece of the requirement

The prompt explicitly instructs Claude to work in small increments — one test at a time, minimum code to pass, commit after each cycle. The commit trail creates a verifiable record of the TDD discipline.

This is a single Claude invocation (not multiple invocations per cycle). The decomposition benefit comes from Claude thinking in small steps, guided by the prescriptive prompt. Multiple invocations would lose the thread of "where am I in breaking this problem down?"

#### Refactor Phase

After the build phase passes validation, a **separate Claude invocation** (same model as build) runs with the sole purpose of refactoring:

- Improve naming, structure, and clarity
- Extract abstractions where warranted (not prematurely)
- Remove duplication
- Clean up any "minimum to pass" roughness from the green steps
- Do not change behavior — all tests must still pass

The refactor phase is a separate invocation to mechanically enforce it. LLMs tend to skip refactoring when it's part of the same session as implementation. Making it a separate step ensures it actually happens.

After refactoring, validation runs again to confirm no behavior changed.

#### Refactor Applies to ATDD Too

The refactor phase fires whenever either `atdd` or `tdd` is active. Test-driven work of any kind benefits from a dedicated refactoring pass.

### TDD Build Prompt

A new prompt template (`PROMPT_tdd_build.md`) replaces the standard build prompt when TDD is active. It receives the same context as the regular build prompt but with different instructions:

- Follow the red-green-refactor cycle strictly
- Write ONE failing test, then write the minimum code to make it pass
- Commit after each red-green cycle with a message describing what was tested and implemented
- Do not write multiple tests before implementing — one at a time
- Focus each test on a single behavior or requirement from the bead
- After all requirements are covered, stop — refactoring will happen in a separate phase

### Refactor Prompt

A new prompt template (`PROMPT_refactor.md`) is used for the refactor phase. It receives:

- Bead details (title, description, acceptance criteria)
- The git diff of all changes made during the build phase
- CLAUDE.md and rules
- Learnings

Instructions:
- Review the implementation for code quality, naming, structure, and duplication
- Refactor to improve clarity and maintainability
- Do not change behavior — all existing tests must continue to pass
- Commit refactoring changes separately from implementation commits

### Combined ATDD + TDD Flow

When both are active, the full flow becomes:

1. **Write acceptance tests** (ATDD — separate invocation)
2. **Verify acceptance tests fail** (ATDD)
3. **Build with red-green cycles** (TDD — single invocation, one-test-at-a-time)
4. **Validate** (all tests pass, including acceptance tests)
5. **Refactor** (separate invocation — shared by both methodologies)
6. **Validate again** (refactoring didn't break anything)
7. **Review** (as usual)

When only TDD is active (no ATDD):

1. **Build with red-green cycles** (TDD)
2. **Validate**
3. **Refactor** (separate invocation)
4. **Validate again**
5. **Review**

When only ATDD is active (no TDD):

1. **Write acceptance tests** (ATDD)
2. **Verify acceptance tests fail** (ATDD)
3. **Build** (standard single-pass, but with "make acceptance tests pass" instruction)
4. **Validate**
5. **Refactor** (separate invocation)
6. **Validate again**
7. **Review**

### Escalation and Retry Behavior

Build failures under TDD follow the existing retry/escalation pattern:
- Retry with analysis context using the same model
- Escalate model if retries exhausted
- Attempt decomposition if all models exhausted

Refactor failures (tests break after refactoring):
- Revert the refactor changes (git reset to pre-refactor state)
- Retry the refactor phase once with analysis context explaining what broke
- If refactoring fails again, skip the refactor phase and proceed to review — a working implementation without refactoring is better than a broken one

### Model Selection

Both the TDD build phase and the refactor phase use the same model as would be selected for a standard build (by priority and label overrides). Refactoring well requires understanding design intent, so a lighter model would make only superficial improvements.

### Methodology Inheritance

When a bead is automatically decomposed, sub-beads inherit the parent's methodology settings (both `atdd` and `tdd` flags). These signals represent the importance of the behavior and the desired development discipline.

## Acceptance Criteria

- A `methodology.tdd` boolean in `gromit.yaml` that defaults to `false`
- Beads with a `tdd:true` label use TDD workflow regardless of global default
- Beads with a `tdd:false` label use standard workflow regardless of global default
- When TDD is active, the build prompt instructs Claude to follow red-green cycles with one-test-at-a-time commits
- A separate refactor invocation runs after successful validation when either `tdd` or `atdd` is active
- The refactor phase uses the same model as the build phase
- After refactoring, validation runs again to confirm tests still pass
- If refactoring breaks tests, Gromit reverts and retries once, then skips refactoring
- TDD and ATDD can be enabled independently or together
- When both are active, the combined flow runs in order: acceptance tests → verify fail → TDD build → validate → refactor → validate → review
- Automatically decomposed sub-beads inherit both `tdd` and `atdd` settings from their parent

## Decisions

1. **Single invocation for red-green, separate invocation for refactor.** The red-green cycles happen within one Claude session to preserve the decomposition thread. The refactor step is a separate invocation to mechanically enforce it — LLMs skip refactoring when it's bundled with implementation.

2. **Same model for all phases.** Build, refactor, and (if active) acceptance test writing all use the same priority/label-selected model. Refactoring requires understanding design intent, not just surface-level cleanup.

3. **Independent toggles, not mutually exclusive modes.** ATDD and TDD solve different problems (behavioral contract vs. problem decomposition) and can be combined. Independent booleans are cleaner than combo values.

4. **Refactor fires for either methodology.** The refactor concern applies to all test-driven work, not just TDD. When ATDD is active alone, the implementation still benefits from a dedicated refactoring pass.

5. **Revert-and-retry on refactor failure.** If refactoring breaks tests, revert to the working state and retry once. If it fails again, skip — a working implementation without refactoring is better than a broken one.

6. **Prescriptive prompt, not mechanically enforced cycles.** The one-test-at-a-time discipline is enforced through prompt instructions and commit trail, not through multiple invocations per cycle. This preserves Claude's ability to maintain the decomposition thread while creating accountability through the git log.

## Research & Context

### Current State

The current build flow is a single-pass implementation in `internal/runner/process.go`. There is no concept of a refactor phase or iterative test-writing cycles. The build prompt (`PROMPT_build.md`) says "Write tests if the task involves new functionality" but implementation and tests are written together.

Key files for implementation:
- `internal/config/config.go` — Add `MethodologyConfig` struct with `ATDD bool` and `TDD bool` fields
- `internal/runner/process.go` — Orchestrate methodology-aware flow: check toggles, select appropriate prompt, insert refactor phase
- `internal/runner/runner.go` — Handle methodology inheritance on decompose
- `internal/prompt/prompt.go` — Add `RenderTDDBuild()`, `RenderRefactor()` methods and context types
- `.gromit/templates/PROMPT_tdd_build.md` — New template for TDD red-green build phase
- `.gromit/templates/PROMPT_refactor.md` — New template for refactor phase (shared by ATDD and TDD)

### Relationship to ATDD Spec

This spec and `atdd-methodology` share:
- The `methodology` config section (independent toggles in same struct)
- The refactor phase (fires for either)
- Methodology inheritance on decompose
- The `MethodologyConfig` struct in config

The ATDD spec should be updated to reflect:
- Independent boolean toggles instead of `default: standard/atdd`
- Refactor phase added to ATDD-only flow
- Label format: `atdd:true`/`tdd:true` instead of `methodology:atdd`
