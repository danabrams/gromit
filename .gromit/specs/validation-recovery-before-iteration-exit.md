---
id: validation-recovery-before-iteration-exit
title: Validation recovery before iteration exit
status: proposed
created: '2026-02-28T22:58:00Z'
updated: '2026-02-28T22:58:00Z'
---

# Validation Recovery Before Iteration Exit

## Specification
Wire Stage 3 validation to attempt repair before ending an iteration.

Current behavior is fail-fast: the first validation failure blocks the stage and the orchestrator exits the iteration as failed. This spec requires Stage 3 to use existing validation recovery behavior so it:
- runs direct validation commands first
- attempts trivial auto-fix (`gofmt`/`goimports`) on validation failure
- re-runs validation after auto-fix
- attempts Claude-based fix when auto-fix is insufficient
- re-runs validation after Claude fix
- only exits the iteration as validation-failed when recovery is exhausted

Implementation must preserve orchestrator sequencing and event contracts.

### Scope
- Stage 3 validation wiring in orchestrator/constructor paths.
- Validation stage implementation or adapter to call recovery-capable runner APIs.
- Iteration log/result field parity (`validated`, `validation_mode`, `validation_retried`, `trivial_auto_fixed`, failure summaries).
- Tests proving recovery attempts occur before failure exit.

### Out of Scope
- Changing decomposition policy.
- Changing gate-stage stuck detection policy.
- Altering bead close/sync semantics (epilogue closes only on success).

## Acceptance Criteria
1. On first validation command failure, Stage 3 attempts recovery before returning a block decision.
2. Recovery order is deterministic: auto-fix first, then Claude-based fix.
3. Validation passes after recovery: iteration proceeds to Review/Epilogue success path (bead close/sync still happen only on success).
4. Recovery exhausted: Stage 3 returns failure with validation summaries, and iteration follows existing failure epilogue path.
5. `max_validation_retries` config is honored by Stage 3 recovery path.
6. Validation events and logs remain coherent (start/pass/fail semantics unchanged; retry-related result fields populated when applicable).
7. Tests cover both success-after-recovery and fail-after-exhaustion scenarios.

## Decisions
- Reuse existing `internal/runner/validation.Runner` recovery behavior instead of introducing a second recovery implementation.
- Keep fail-fast behavior only as the terminal path after recovery attempts are exhausted.
- Preserve current orchestrator stage order and epilogue ownership boundaries.

## Research & Context
- Investigation report: `.gromit/reports/debug-20260228-225435.md`
- Evidence indicates recovery logic exists but is not wired into Stage 3:
  - `internal/runner/orchestrator.go` calls `Validate.Run(...)` and exits iteration on non-proceed.
  - `internal/pipeline/validate/validate.go` blocks on first failing command.
  - `internal/runner/validation/runner.go` already implements recovery loop (auto-fix + Claude + re-validation).
