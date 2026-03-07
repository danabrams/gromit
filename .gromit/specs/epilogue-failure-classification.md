---
id: epilogue-failure-classification
source_ideas: []
created: 2026-02-27
epic: multi-interface-architecture
accepted: true
---

# Epilogue Failure Classification

## Specification

When bead lifecycle operations fail in epilogue (specifically `Close` or `Sync`), the iteration must not be treated as successful.

Current behavior allows success-path completion to continue even after lifecycle failures are logged as warnings, which can produce a false "completed successfully" outcome and success-marked iteration data.

This spec changes classification semantics so lifecycle failure is authoritative:

1. If `Close` fails, iteration outcome is failure.
2. If `Sync` fails, iteration outcome is failure.
3. Success-path completion logging/state updates only occur when epilogue lifecycle operations succeed.
4. Non-critical epilogue activities (status writes, optional thorough review, worktree cleanup warnings, between-iteration command warnings) remain non-fatal unless explicitly reclassified in a future spec.

### Event-System Compatibility

This change must remain compatible with `.gromit/specs/event-system.md`:

- Lifecycle truth in events must match runtime truth: failures in bead close/sync cannot emit success semantics.
- Event payloads and ordering must remain aligned with existing event types (`IterationCompleteEvent.Success`, `BeadCompleteEvent`, `BeadFailedEvent`, and `LogEvent` fallback usage).
- Transitional `LogEvent` usage is allowed only for unmapped diagnostics; it must not contradict typed lifecycle events.

## Acceptance Criteria

- If epilogue `Close` fails on a success-path iteration, the iteration is recorded as failed (not success).
- If epilogue `Sync` fails on a success-path iteration, the iteration is recorded as failed (not success).
- Orchestrator must not emit or log "completed successfully" semantics when epilogue lifecycle operations fail.
- If event emission is present for the run path, event stream reflects failure classification consistently with event spec (no success event for lifecycle-failed iteration).
- Regression tests cover both failure cases (close failure, sync failure) and prove no false success classification.
- Existing non-critical warning behavior remains unchanged for unrelated epilogue optional operations.

## Decisions

1. **`Close`/`Sync` are critical lifecycle operations.** They define whether the bead actually completed and was persisted.
2. **Classification over cosmetic output.** Accurate success/failure state is prioritized over preserving legacy warning-only behavior.
3. **Event parity is mandatory.** Any event output path must match final classification; no divergent text/event truth.

## Research & Context

- Investigation report: `.gromit/reports/debug-20260227-021749.md`
- Related architecture spec: `.gromit/specs/event-system.md`
- Affected implementation areas:
  - `internal/pipeline/epilogue/epilogue.go`
  - `internal/runner/orchestrator.go`
  - `internal/pipeline/epilogue/epilogue_test.go`
  - `internal/runner/orchestrator_test.go`
