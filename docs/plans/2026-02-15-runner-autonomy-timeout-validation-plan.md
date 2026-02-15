# 2026-02-15 Runner Autonomy Plan: Retry/Timeout Hardening + Validation Strategy

## Goal
Make `gromit run` stable for unattended background execution by reducing timeout/retry cascades and eliminating avoidable validation bottlenecks.

## Scope
This plan covers:
1. Retry/timeout policy hardening (item 1)
2. Validation strategy optimization (item 2)

This plan does **not** include token observability implementation details (item 3), which is tracked separately in code changes.

## Problem Summary
Observed run logs show:
- Excessive runtime lost to timeout/deadline paths (`invocation timed out`, `context deadline exceeded`)
- Retry/escalation loops multiplying work after failures
- Validation steps occasionally blocking/over-consuming budget

## Design Principles
- Fail fast on low-signal retries
- Spend budget on one high-quality attempt, not many repeated attempts
- Keep per-bead loop lightweight; reserve heavy verification for controlled checkpoints
- Preserve code health via deterministic gates, not repeated expensive loops

## Phase 1: Retry/Timeout Hardening (Item 1)
### 1.1 Timeout-class-aware retry policy
- Add timeout/deadline classification branch in escalation flow:
  - `invocation timeout`: no same-tier retry; escalate tier once (if available)
  - `bead timeout` / parent deadline: terminate bead immediately
  - `stall timeout`: allow at most one retry if no tool activity occurred
- Cap timeout-triggered escalations per bead to 1.

### 1.2 Retry budget by bead
- Enforce explicit per-bead budget:
  - max total attempts across tiers
  - max wall-clock per bead (already exists; tighten behavior when exceeded)
- If budget exceeded, return structured failure quickly (no further analysis/decompose loops in same iteration).

### 1.3 Escalation chain simplification
- Default to lower retry amplification:
  - reduce `max_retries_per_model`
  - prefer shorter chain for timeout-origin failures
- Keep existing chain for non-timeout functional failures when analysis says recoverable.

### 1.4 Expected outcomes
- Fewer 20-45 minute failed runs
- Lower wasted token spend from repeated near-identical retries
- More predictable unattended throughput

## Phase 2: Validation Strategy Optimization (Item 2)
### 2.1 Split fast vs full validation
- Introduce/standardize two gate sets:
  - fast per-bead gate (default in loop): targeted package tests/lint/build checks
  - full verification gate: `go test ./...` and full-quality suite at controlled checkpoints
- Run full gate:
  - at session completion / landing workflow
  - optionally every N successful beads (configurable)

### 2.2 Non-interactive validation guarantees
- Ensure all validation commands run non-interactively in run loop context
- Set CI-safe environment defaults for run-loop command runner
- Fail with explicit message if a validation command attempts to prompt

### 2.3 Validation recovery tightening
- On validation failure, allow one fix attempt before escalation or bead failure
- Avoid repeated validation-recovery cycles when failing command output is unchanged

### 2.4 Expected outcomes
- Lower per-bead latency
- Reduced deadline collisions in validation/revalidation phases
- Better autonomous progress while retaining code quality checkpoints

## Configuration Changes (Planned)
- `gromit.yaml`:
  - tighten retry defaults (`escalation.max_retries_per_model`, optional per-error policy)
  - add/clarify fast vs full validation command groups
  - optional `full_validation_every_n_successes`

## Observability and Acceptance
### Metrics to compare before/after
- timeout/deadline failure rate
- average duration of failed runs
- average duration of successful validated runs
- retries per bead
- total cost/run (where available)

### Acceptance criteria
- Timeout/deadline runs reduced materially vs baseline
- Mean failed-run duration significantly reduced
- No regression in final full-suite pass rates
- Run loop continues autonomously without interactive stalls

## Rollout Sequence
1. Implement timeout-class-aware retry and budget hard stops
2. Adjust defaults in `gromit.yaml`
3. Implement fast/full validation split and non-interactive guarantees
4. Add tests for new retry and validation behaviors
5. Monitor metrics and tune thresholds

## Risks and Mitigations
- Risk: too-aggressive fail-fast increases false negatives
  - Mitigation: allow controlled single escalation on timeout class
- Risk: fast gate misses cross-package regressions
  - Mitigation: enforce periodic + final full verification
- Risk: config complexity
  - Mitigation: keep sensible defaults and clear inline comments

## Future Extensions
- Adaptive retries based on recent success probability per model/tier
- Dynamic validation selection based on changed packages
- Automatic quarantine of beads with repeated timeout signatures
