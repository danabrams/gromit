# Enforce Timeout-First Decomposition

## Objective

Reduce timeout failure rate (currently 26.7% vs UCL 13.87%) by enforcing timeout-first decomposition immediately on first timeout and blocking same-scope retries until decomposition or explicit escalation is recorded.

## Problem Statement

Broad/complex work is reaching execution phases long enough to hit timeout budgets (p95 duration ~1,937,366ms). Decomposition and retry controls are not consistently preventing same-scope retries after timeout across model/provider strata. The system currently relies on distributed enforcement points instead of one pre-execution complexity gate with mandatory decomposition compliance.

## Acceptance Criteria

1. When a bead hits its first timeout failure, the runner immediately triggers timeout-first decomposition without waiting for escalation
2. Same-scope retry is blocked with clear error until decomposition creates sub-beads or explicit escalation decision is recorded
3. Decomposition attempt is auditable — logged with timestamp and outcome (success/skipped/failed)
4. Retry logic respects the typed contract: retries are forbidden if decomposition creation is partial/non-idempotent
5. After implementation, rolling timeout failure rate drops below 0.14 (14%) for 3 consecutive tracking windows
6. First-pass success rate exceeds 0.50 (50%)

## Design Approach

### Scope

Modify the escalation handler and retry loop in `internal/runner/escalation/` and `internal/runner/policy/` to:

1. **Detect first timeout**: When elapsed phase time exceeds 75% of budget and timeout occurs
2. **Trigger decomposition**: Call decomposition path immediately, do not escalate tier first
3. **Block same-scope retry**: Add typed error and enforcement point in retry loop to prevent same-scope retries
4. **Record decision**: Log decomposition attempt (success/skipped/failed) in iteration metadata for observability
5. **Idempotency safeguard**: Ensure decomposition either succeeds atomically or fails with clear error; no partial state

### Implementation Steps

1. Update `internal/runner/escalation/handler.go`:
   - Add `firstTimeoutDecompositionThreshold` config (default 75% of budget)
   - In `HandleInvocationTimeout`, check elapsed time vs budget before escalating
   - If threshold exceeded, call `AttemptDecomposition` before tier advancement
   - Record decomposition attempt with outcome in `BeadContext`

2. Update retry loop in `internal/runner/policy/` or orchestrator:
   - Add check: if retry count > 0 AND timeout occurred AND NO decomposition recorded, return typed error
   - Error message must be clear: "Same-scope retry blocked: timeout requires decomposition or escalation decision"
   - Add telemetry field to iteration log: `timeout_decomposition_attempted`, `timeout_decomposition_succeeded`

3. Update `runtypes/BeadContext` or iteration metadata:
   - Add field: `TimeoutDecompositionAttempted bool`
   - Add field: `TimeoutDecompositionSucceeded bool`

4. Add contract tests:
   - Verify first timeout triggers decomposition, not escalation
   - Verify same-scope retry is blocked without decomposition
   - Verify decomposition success unblocks loop continuation
   - Verify partial decomposition state is detected and causes error

5. Update telemetry/process trend:
   - Track `timeout_decomposition_attempts` (count)
   - Track `timeout_decomposition_success_rate`
   - Track `timeout_retry_blocks` (count of retries blocked by enforcement)

## Testing Strategy

- **Unit tests**: Escalation handler timeout detection and decomposition triggering
- **Contract tests**: Retry loop blocking behavior with typed errors
- **Acceptance tests**: End-to-end timeout → decomposition → success workflow
- **Telemetry tests**: Verify timeout decomposition metrics are recorded correctly

## Success Metrics

- Timeout failure rate < 14% (down from 26.7%)
- First-pass success rate > 50% (up from 33.3%)
- Rework rate < 66.7% (down from current level)
- Zero same-scope retries after timeout without decomposition recorded

## Related Learnings

- 2026-02-24 | Decomposition Contract-Field Parity Across Layers (ensure contract fields visible to model)
- 2026-02-24 | Session Worktree and Mergeback Safety Contract (concurrent work isolation)

## Notes

- This is the "local fix" for system action: timeout failure rate control
- The "system fix" (pre-execution complexity gate) builds on this foundation
- Coordinate with telemetry/logger team on metrics schema changes
