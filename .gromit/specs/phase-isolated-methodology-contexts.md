---
id: phase-isolated-methodology-contexts
source_ideas: []
created: 2026-02-16
epic: codebase-health
---

# Phase-Isolated Contexts for Red/Green/Refactor/Validation

## Specification

Gromit should execute methodology phases (ATDD/TDD red, green/build, refactor, and validation) using isolated contexts rather than a single bead-scoped context shared across all phases. Each phase gets its own timeout budget and cancellation boundary, derived from the run-level context. This prevents one overlong phase from pre-canceling later phases and causing cascading failures.

The immediate behavior target is reliability: if refactor times out, validation should still be able to run within its own context (subject to remaining run budget), and the system should classify failures by phase rather than collapsing them into a single generic timeout.

### Phase Context Model

For methodology-enabled beads, Gromit creates explicit per-phase contexts in sequence:

1. **Red context** (`acceptance_tests` and/or `verify_tests_fail`)
2. **Green context** (`build` implementation pass)
3. **Refactor context** (`refactor` quality pass)
4. **Validation context** (`fast_commands` direct execution, and any recovery loop)

Rules:
- All phase contexts are children of the run-level context (`ParentCtx`), not children of each other.
- Phase contexts must not reuse a canceled context from a prior phase.
- A phase timeout produces a phase-local result and logging record with explicit phase attribution.
- Optional phases (for example refactor) may timeout without forcing bead failure unless policy explicitly requires failure.
- Required phases (for example green/build, required validation gate) preserve existing fail/stop semantics.

### Timeout Configuration

Add optional phase-level timeouts in `validation`/`methodology` config with sensible defaults:

```yaml
methodology:
  atdd: false
  tdd: false
  phase_timeouts:
    red_seconds: 600
    green_seconds: 1200
    refactor_seconds: 600

validation:
  enabled: true
  command_timeout: 5m
  phase_timeout_seconds: 900
```

Behavior:
- If a phase timeout is unset/zero, fallback to current defaults (backward compatibility).
- Existing `claude.bead_timeout` remains the fallback default for model-invocation phases when phase timeout is not configured.
- `validation.phase_timeout_seconds` caps the full validation phase (including retries), while `command_timeout` remains per-command.

### Minimal Phase Prompts

Each model-driven phase should use a minimal, phase-scoped prompt assembly strategy:

- **Red prompt**: acceptance criteria, relevant spec snippet, test conventions, explicit “tests only” guardrails.
- **Green prompt**: failing-test signal, changed files/packages, required behavior, no redundant full-history payload.
- **Refactor prompt**: diff summary + constraints (no behavior change), no full build prompt duplication.

Prompt builder requirements:
- Keep shared non-negotiables (`RULES.md` constraints) available.
- Trim low-signal context (large historical sections) by default.
- Preserve compatibility with existing template registration pattern.

### Execution and Error Semantics

- `runRefactorAndPostChecks` and related methodology orchestration must construct a fresh validation context from `ParentCtx`.
- Validation timeout errors after refactor must be classified as `validation` phase timeouts, not bead-context exhaustion.
- Logs and iteration results include phase name and phase timeout/cancellation reason.

### Out of Scope

- Redesign of escalation tier policy.
- New provider routing semantics.
- Full prompt-template rewrite outside phase-specific trimming.

## Acceptance Criteria

- Methodology phases run with independent contexts derived from `ParentCtx`, not a shared bead context.
- Canceling or timing out refactor context does not automatically cancel validation context.
- Validation phase has its own configurable timeout budget and preserves per-command timeout behavior.
- Phase timeout/cancellation messages identify the exact phase (`red`, `green`, `refactor`, `validation`).
- Existing behavior remains backward compatible when new phase timeout fields are omitted.
- Unit tests cover:
  - refactor timeout followed by successful validation execution in separate context
  - validation phase timeout classification independent from refactor timeout
  - fallback to default timeouts when phase timeout config is unset
- Prompt rendering for methodology phases uses phase-scoped/minimal context inputs (validated by renderer/unit tests).

## Decisions

1. **Phase isolation over shared bead context.** Shared cancellation domains create cascading failures. Isolated phase contexts localize failures and improve loop resilience.

2. **Run-level parent as the root for phase contexts.** This preserves global stop/time-budget behavior while preventing sibling phase interference.

3. **Validation gets a dedicated phase budget.** Validation is deterministic and should have predictable runtime/cancellation semantics independent of model invocation timing.

4. **Minimal phase prompts by default.** Smaller prompts reduce latency and timeout risk while increasing phase focus.

5. **Backward-compatible rollout.** New fields are optional; existing configs continue to work without edits.

## Research & Context

### Problem Evidence

Recent failures show a recurring pattern:
- Refactor invocation times out (`context deadline exceeded`)
- Post-refactor validation immediately aborts with timeout/canceled context
- Bead fails repeatedly despite refactor being intended as non-blocking

This indicates context coupling across phases instead of independent phase budgets.

### Relevant Code Areas

- `internal/runner/process_methodology.go` — methodology phase orchestration and post-refactor validation flow
- `internal/runner/methodology/refactor.go` — refactor phase behavior and internal re-validation hooks
- `internal/runner/validation/runner.go` — validation execution/recovery and timeout handling
- `internal/config/config.go` — timeout config schema/defaulting
- `internal/prompt/prompt.go` and templates — phase prompt rendering and context shaping

### Constraints and Patterns

- Keep existing escalation ownership in `internal/runner/escalation/`.
- Keep `validation.Runner` as the command execution authority for direct validation.
- Preserve config defaulting + nil normalization conventions in `setDefaults()` / `NormalizeNilFields()`.
