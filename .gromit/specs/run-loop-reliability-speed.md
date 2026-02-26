---
id: run-loop-reliability-speed
source_ideas: []
created: 2026-02-15
epic: run-loop-reliability
---

# Improve Run Loop Reliability and Speed

## Specification

Recent run logs show the primary reliability bottleneck is acceptance-test phase instability when routed to Codex, and the primary latency bottleneck is full-repo validation during per-bead fast gates. This spec improves both by changing provider fallback behavior, tightening preflight checks, and reducing unnecessary validation scope.

### Problem Statement

1. **Acceptance-test reliability is dominated by provider transport failures.**
   Most recent failures are `acceptance tests retry failed` with Codex transport/startup errors (`stream disconnected`, `failed to start codex command`, invalid `CODEX_HOME`).

2. **Per-bead latency is inflated by broad validation commands.**
   Fast validation frequently runs `go test ./...` for every bead, creating 20-40 minute tails when retries or heavy package tests are involved.

3. **Observability is insufficient during early invocation failures.**
   Stream logs are often empty for failed iterations, which slows diagnosis and causes repeated blind retries.

## Goals

1. Increase successful completion rate for `gromit run` iterations under transient provider/network failures.
2. Reduce p50 and p90 iteration wall-clock time without weakening final quality gates.
3. Improve failure diagnosability from logs in a single pass.

## Non-Goals

1. Replacing the multi-provider routing architecture.
2. Changing task selection, bead decomposition semantics, or review policy.
3. Removing full validation gates entirely.

## Proposed Changes

### A. Transport-aware fallback in ATDD execution

**Current issue:** ATDD fallback logic handles some failure paths, but transport/startup failures still often terminate the phase after expensive retries.

**Changes:**

1. Add explicit transport/startup error classification in the ATDD invocation error path.
2. If primary provider fails with transport-disconnect semantics:
   - Mark provider unavailable for cooldown window.
   - Retry once on alternate provider at same tier.
3. Emit structured log lines indicating:
   - primary provider/model
   - fallback provider/model
   - failure class (`transport_disconnect`, `startup_error`, `other`)

**Expected impact:** avoid repeated same-provider retries for transient network/session failures.

### B. Streaming retry parity for Codex provider

**Current issue:** non-streaming Codex path has transient retry/backoff, while streaming path lacks parity.

**Changes:**

1. Apply the same transient retry policy to `StreamRun` as `Run` for transient failure classes:
   - transport disconnect
   - rate limited
2. Keep retry budget small (e.g., 2-3 attempts max including initial) with bounded backoff.
3. Ensure retries preserve:
   - context cancellation behavior
   - usage/cost accounting
   - failure category in terminal result

**Expected impact:** reduce false-hard failures from short-lived transport interruptions.

### C. Codex preflight hardening (`CODEX_HOME` readiness)

**Current issue:** run loop passes preflight but later fails due to missing/unwritable `CODEX_HOME` and helper setup issues.

**Changes:**

1. During preflight, validate effective `CODEX_HOME` path used by provider:
   - path exists or can be created
   - writable by current process
   - not an unsafe temp-only path that Codex helper setup rejects
2. If invalid, fail fast with actionable remediation in error output.
3. Log final resolved `CODEX_HOME` path once per run.

**Expected impact:** convert repeated mid-run startup failures into one clear preflight failure.

### D. Fast-gate validation scoping

**Current issue:** fast validation is too broad for per-bead loop cadence.

**Changes:**

1. Keep full validation for periodic/final gates.
2. Scope fast gate to touched packages when available:
   - convert `go test ./...` to targeted package set for changed code paths.
3. Add per-command timeout for validation commands to avoid hanging a bead until bead-level timeout.
4. Preserve existing behavior as fallback when touched packages are unknown.

**Expected impact:** significant p50/p90 latency reduction with unchanged final correctness bar.

### E. Minimal stream-log baseline for failed invocations

**Current issue:** empty stream logs make failure triage difficult.

**Changes:**

1. Always emit invocation lifecycle markers to stream log:
   - invocation start
   - provider/model/tier selection
   - invocation end/failure summary
2. Keep JSON event logging unchanged when events exist.

**Expected impact:** every failed run has a usable trace even when provider emits no stream events.

## Acceptance Criteria

1. ATDD phase performs alternate-provider fallback for transport/startup failures and logs the fallback decision.
2. Codex streaming path retries transient failures with bounded backoff and preserves cancellation semantics.
3. Preflight fails before loop start when effective `CODEX_HOME` is not usable.
4. Fast validation can run package-scoped commands and retains full-gate behavior for periodic/final validation.
5. Stream log files for failed iterations contain non-empty lifecycle markers even without JSON stream events.
6. Post-change measurement over at least 30 iterations shows:
   - improved success rate for affected phase(s), and
   - reduced median iteration duration.

## Rollout Plan

### Phase 1: Reliability first

1. Implement A (ATDD transport-aware fallback).
2. Implement B (Codex streaming retry parity).
3. Implement C (preflight hardening).
4. Add targeted tests for each failure mode.

### Phase 2: Speed improvements

1. Implement D (fast-gate validation scoping + per-command timeout).
2. Validate no regression in final full gate.

### Phase 3: Observability

1. Implement E (stream lifecycle markers).
2. Add stats/reporting hooks for fallback counts and retry outcomes.

## Metrics

Track per day and per 30-iteration window:

1. Iteration success rate.
2. ATDD phase failure rate and top failure categories.
3. Median and p90 iteration duration.
4. Count of provider fallback events and their success rate.
5. Validation command timeout count.
6. Percentage of failed runs with non-empty stream logs.

## Decisions

1. **Preserve final full validation gate.**
   Speed gains should come from smarter fast gates, not weaker correctness guarantees.

2. **Fail fast on environment misconfiguration.**
   A deterministic preflight failure is better than repeated expensive retries.

3. **Prefer bounded retries over deep retry loops.**
   Retries are for transient faults, not for masking persistent provider/environment problems.

4. **Improve logs before adding more retry complexity.**
   Better diagnostics reduce repeated blind changes and shorten incident resolution time.

