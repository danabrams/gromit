---
id: atdd-methodology
source_ideas: [idea-1770457558357]
created: 2026-02-07
---

# ATDD Methodology Support

## Specification

Gromit gains a configurable methodology system that allows users to enable Acceptance Test-Driven Development (ATDD) for their beads. When ATDD is active, the build workflow splits into two distinct phases: first writing acceptance tests that verify the bead's acceptance criteria, then implementing code to make those tests pass. This enforces the discipline of defining the behavioral contract before writing implementation.

### Configuration

A new `methodology` section in `gromit.yaml` with independent boolean toggles:

```yaml
methodology:
  atdd: false   # This spec
  tdd: false    # See tdd-methodology spec
```

Per-bead override via labels: `atdd:true` or `atdd:false` on a bead overrides the global default. When a bead is automatically decomposed, sub-beads inherit the parent's methodology labels. ATDD and TDD are independent — both can be enabled simultaneously (see tdd-methodology spec for combined flow).

### ATDD Workflow

When ATDD is active for a bead, the processing flow becomes:

1. **Write acceptance tests** — A separate Claude invocation (same model as the build phase) receives the bead's acceptance criteria, spec, and full codebase context. It explores the codebase to understand existing test patterns, frameworks, and conventions. It writes acceptance/integration tests that cover the bead's acceptance criteria. It must only create or modify test files — no implementation code.

2. **Verify tests fail** — Gromit runs the configured validation commands. The tests must fail. If they pass (meaning the behavior already exists or the tests are trivial/tautological), Gromit runs failure analysis to understand why, then retries the acceptance test phase once with that analysis context. If the tests still pass on the second attempt, the bead fails with the message: "acceptance tests passed before implementation — tests may not be covering new behavior."

3. **Build** — A separate Claude invocation (same model) receives the bead context plus an explicit instruction: "Acceptance tests have been written and committed. Your job is to make them pass." The build proceeds as normal.

4. **Validate** — Standard validation: run the configured commands, expect all tests to pass.

5. **Refactor** — A separate Claude invocation focused on code quality improvements (see tdd-methodology spec for details). This phase fires whenever either `atdd` or `tdd` is active. After refactoring, validation runs again to confirm no behavior changed.

6. **Review** — Standard review as configured.

### Acceptance Test Prompt

The acceptance test phase uses a new prompt template (`PROMPT_acceptance_tests.md`) that receives the same context as the build prompt (bead details, spec, learnings, CLAUDE.md, rules) but with different instructions:

- Explore the codebase to understand test patterns, frameworks, file structure, and naming conventions
- Write acceptance/integration tests based on the bead's acceptance criteria
- Each acceptance criterion should map to at least one test
- Follow existing test conventions in the project
- Only create or modify test files — do not write any implementation code
- Commit the test files with a clear commit message

### Escalation and Retry Behavior

When the build phase fails under ATDD:

- **Preserve the acceptance tests.** They represent the behavioral contract. Retry and escalation apply only to the build phase — the tests stay as-is.
- **Exception:** If failure analysis determines the tests themselves are the problem (wrong assertions, untestable design, testing the wrong thing), then Gromit may rewrite the acceptance tests before retrying the build.
- Standard retry/escalation logic applies: retry with analysis context, then escalate model (haiku → sonnet → opus), then attempt decomposition if all models exhausted.

When the acceptance test phase itself fails (tests don't compile, Claude writes implementation code instead of tests, etc.):

- Retry with analysis context using the same model
- Escalate model if retries exhausted
- Fail the bead if all models exhausted

### Methodology Inheritance

When a bead is automatically decomposed into sub-beads, the sub-beads inherit the parent's methodology. ATDD being present signals the importance of the behavior — this importance carries through to sub-tasks.

## Acceptance Criteria

- A `methodology.atdd` boolean in `gromit.yaml` that defaults to `false`
- Beads with an `atdd:true` label use ATDD workflow regardless of global default
- Beads with an `atdd:false` label skip ATDD workflow regardless of global default
- When ATDD is active, a separate Claude invocation writes acceptance tests before the build phase
- The acceptance test phase only produces test files, no implementation code
- After acceptance tests are written, Gromit verifies they fail before proceeding to build
- If acceptance tests pass before implementation, Gromit retries once with analysis context, then fails the bead
- The build phase under ATDD receives context indicating acceptance tests exist and must be made to pass
- Build failures preserve acceptance tests and only retry/escalate the build phase
- Automatically decomposed sub-beads inherit the methodology from their parent
- The acceptance test phase uses the same model as the build phase (selected by priority/labels)
- A refactor phase runs after validation when ATDD is active (shared with TDD; see tdd-methodology spec)

## Decisions

1. **Separate invocation for acceptance tests.** The acceptance test phase runs as its own Claude invocation, not part of the build. This enforces the ATDD discipline of committing tests before implementation and gives the build phase a concrete "make these tests pass" target. The extra invocation cost is offset by more focused build phases and fewer retries.

2. **Same model for both phases.** The acceptance test phase uses the same model selected for the build (by priority and label overrides). If a bead is complex enough for opus, the tests should be written by opus too.

3. **Verify tests fail before building.** True ATDD discipline requires failing tests before implementation. If tests pass before implementation, it means the behavior already exists or the tests aren't meaningful. Gromit retries once with analysis, then fails — surfacing this information early.

4. **Preserve tests across build retries.** Acceptance tests are the behavioral contract. When the build fails, the tests stay and only the implementation is retried/escalated. Tests are only rewritten if failure analysis identifies them as the root cause.

5. **Independent boolean toggles.** ATDD and TDD are independent toggles (`methodology.atdd`, `methodology.tdd`) rather than mutually exclusive modes. They solve different problems and can be combined. Per-bead labels (`atdd:true`/`atdd:false`) override the global default.

6. **Sub-beads inherit methodology.** ATDD signals the importance of the behavior, not just a workflow preference. When a bead is decomposed, that signal of importance carries through to the sub-tasks.

## Research & Context

### Current State

The current bead processing flow in `internal/runner/process.go` follows: scope check → build → validate → review. The build prompt (`PROMPT_build.md`) tells Claude to "write tests if the task involves new functionality" but tests and implementation are written together in one pass.

Key files for implementation:
- `internal/config/config.go` — Config structs; add `MethodologyConfig` struct and `Methodology` field to `Config`
- `internal/runner/process.go` — Bead processing pipeline; insert acceptance test phase before `executeWithRetry()`
- `internal/runner/runner.go` — Main loop orchestration; handle methodology inheritance on decompose
- `internal/prompt/prompt.go` — Prompt renderer; add `RenderAcceptanceTests()` method and `AcceptanceTestContext` type
- `.gromit/templates/PROMPT_build.md` — Current build template; needs ATDD-aware variant or conditional section
- `internal/bead/bead.go` — Bead client; may need helper to check methodology labels

### Existing Patterns

- Config follows the pattern of typed sub-structs with `setDefaults()` filling zero values (see `ReviewConfig`, `ScopeCheckConfig`)
- Per-bead label overrides already exist for model selection (`complexity:high`, `complexity:low`); `methodology:atdd` follows the same pattern
- Validation is already a separate Claude invocation (`claude.RunValidation()`), so adding another phase follows the established architecture
- Failure analysis and retry-with-context is well-established in `analyzeAndHandleFailure()`

### Companion Spec: TDD

See `tdd-methodology` for the TDD spec. The two specs share the `methodology` config section (independent boolean toggles), the refactor phase, and methodology inheritance on decompose. When both are active, the combined flow is: acceptance tests → verify fail → TDD build (red-green cycles) → validate → refactor → validate → review.
