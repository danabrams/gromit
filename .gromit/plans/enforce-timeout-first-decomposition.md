---
id: enforce-timeout-first-decomposition
source_spec: enforce-timeout-first-decomposition
created: 2026-02-25
decomposed: false
---

# Enforce Timeout-First Decomposition Implementation Plan

**Goal:** Reduce timeout-driven failures by enforcing decomposition on first timeout, blocking unsafe same-scope retries, and making decomposition decisions auditable and measurable.

**Architecture:** Strengthen the escalation/retry contract so first timeout always records an explicit decomposition/escalation decision path, enforce retry blocking with typed errors (including partial-state protection), and extend iteration/process-trend metrics for timeout decomposition outcomes.

**Tech Stack:** Go

**Spec:** `.gromit/specs/enforce-timeout-first-decomposition.md`

---

## Architecture

**Overview:**
Add a stricter timeout decomposition contract in escalation: first timeout triggers decomposition-first policy with a configurable threshold gate, retry blocking uses typed errors and explicit decision state, and decomposition outcomes become auditable and aggregated into process-trend metrics.

**Key Components:**
1. **Timeout Decomposition Policy (`internal/runner/escalation`)**: Add first-timeout threshold config (default `0.75`) and enforce decomposition-first behavior on first timeout, with explicit outcome recording.
2. **Typed Retry Gate (`ExecuteWithRetry`)**: Replace coarse retry blocking with typed error contracts for blocked retries and partial/non-idempotent decomposition state.
3. **Decomposition Audit State (`internal/runner/runtypes`)**: Extend iteration result with structured timeout decomposition decision data (attempt timestamp, outcome, reason) while keeping compatibility booleans.
4. **Decomposition Idempotency Safeguard (`decomposerAdapter`)**: Detect and surface partial child-creation state as a typed error that blocks retries until explicit escalation decision.
5. **Telemetry Extension (`internal/logger`)**: Add timeout decomposition and retry-block aggregates to rolling metrics for observability and trend tracking.

**Integration Points:**
- `internal/runner/escalation/handler.go` for timeout-first enforcement and decision recording
- `internal/runner/escalation/retry_routing.go` and/or `ExecuteWithRetry` loop gate for typed blocking
- `internal/runner/runtypes/types.go`, `internal/runner/logging.go`, and `internal/logger/logger.go` for audit field propagation
- `internal/runner/constructor_adapters.go` for partial-state detection in decomposition creation
- `internal/logger/process_trend.go` for rolling metrics from iteration logs

**Data Flow:**
- Timeout occurs (`invocation`/`bead` timeout types)
- Escalation handler records timeout decomposition attempt metadata and evaluates threshold/decomposition path
- Decomposition returns outcome: success, skipped, failed, or partial-state failure
- Retry loop checks typed decision contract before any same-scope retry and blocks when unresolved/unsafe
- Iteration logging persists decision fields; process trend computes attempt count, success rate, and retry-block count

**Files to Modify:**
- `internal/runner/escalation/handler.go`
- `internal/runner/escalation/retry_routing.go`
- `internal/runner/runtypes/types.go`
- `internal/runner/logging.go`
- `internal/logger/logger.go`
- `internal/logger/process_trend.go`
- `internal/runner/constructor_adapters.go`
- `internal/runner/escalation/handler_test.go`
- `internal/logger/process_trend_test.go`
- `internal/runner/constructor_test.go`

**Files to Create:**
- `internal/runner/escalation/errors.go`
- `internal/runner/escalation/errors_test.go`

**Tradeoffs:**
- **Typed errors over string matching:** stronger contracts and easier `errors.Is` checks, at the cost of modest plumbing updates.
- **Structured audit fields over booleans only:** better observability and acceptance traceability, with a slightly wider log schema.
- **Adapter-level partial-state detection:** catches unsafe decomposition state at source, but requires careful compatibility with existing dedupe/create behavior.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** timeout threshold checks, first-timeout decomposition behavior, and typed retry-block contracts.
2. **Integration/Contract Tests:** `ExecuteWithRetry` and decomposition adapter behavior for success/skipped/failed/partial outcomes.
3. **Metrics Tests:** iteration-log to process-trend aggregation for decomposition attempts, success rate, and timeout retry blocks.

**Key Test Cases:**
- First invocation timeout triggers decomposition immediately without tier escalation.
- Same-scope retry after timeout is blocked with typed error when neither decomposition success nor explicit escalation decision exists.
- Decomposition success records auditable outcome and allows valid continuation semantics.
- Decomposition skipped/failed records auditable outcome and keeps retry blocked unless explicit escalation decision exists.
- Partial decomposition creation state returns typed partial-state error and prevents same-scope retry.
- Compatibility booleans (`timeout_decomposition_attempted`, `timeout_decomposition_succeeded`) remain correctly populated.
- Process trend aggregates: decomposition attempt count, decomposition success rate, timeout retry block count.

**Mocking Strategy:**
- Mock `decomposeFn` and `createSubFn` in escalation tests for deterministic policy coverage.
- Use adapter-focused tests for partial creation state with controlled bead client failures.
- Use synthetic iteration log fixtures for process trend aggregation tests.

**Coverage Goals:**
- Critical path: timeout-first decomposition decision, retry-block enforcement, partial-state handling.
- Boundary path: threshold edges around 75% budget.
- Compatibility path: unchanged behavior for non-timeout retry routing and existing failure-phase classification.

**Test Organization:**
- Escalation tests in `internal/runner/escalation/handler_test.go` and new `errors_test.go`
- Decomposer contract tests in `internal/runner/constructor_test.go`
- Metric aggregation tests in `internal/logger/process_trend_test.go`

## Implementation Tasks

### Task 1: Define typed timeout-retry/decomposition error contracts

**Files:**
- Create: `internal/runner/escalation/errors.go`
- Test: `internal/runner/escalation/errors_test.go`

**What to Do:**
Introduce typed errors for timeout retry blocking and unsafe decomposition state (including partial/non-idempotent creation). Provide exported helpers for `errors.Is` usage from retry loop and tests.

**Acceptance Criteria:**
- Typed errors exist for same-scope retry blocked and partial decomposition state.
- `errors.Is` works for all new contracts.
- Existing block message text remains available in returned error path.

**Dependencies:**
- None

### Task 2: Add timeout decomposition audit fields to runtypes and logging schema

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/runner/logging.go`
- Modify: `internal/logger/logger.go`

**What to Do:**
Add structured timeout decomposition audit fields on `IterationResult`/`IterationLog` (attempt timestamp, outcome, reason/decision marker) while preserving existing booleans. Map fields through result-to-log conversion.

**Acceptance Criteria:**
- Iteration result includes structured timeout decomposition audit fields.
- Iteration log JSON includes new fields with `omitempty` behavior.
- Existing timeout decomposition booleans remain unchanged and still logged.

**Dependencies:**
- Task 1

### Task 3: Enforce first-timeout decomposition policy with threshold in escalation handler

**Files:**
- Modify: `internal/runner/escalation/handler.go`
- Modify: `internal/runner/escalation/handler_test.go`

**What to Do:**
Implement first-timeout threshold handling (default 75% budget) and ensure first timeout attempts decomposition before tier escalation. Record decomposition outcomes (`success`/`skipped`/`failed`) with timestamp and reason in result metadata.

**Acceptance Criteria:**
- First timeout follows decomposition-first path.
- Threshold defaults to 75% budget and is applied consistently.
- Audit fields are populated for every timeout decomposition attempt.

**Dependencies:**
- Task 2

### Task 4: Replace coarse same-scope block with typed retry gate enforcement

**Files:**
- Modify: `internal/runner/escalation/handler.go`
- Modify: `internal/runner/escalation/retry_routing.go`
- Modify: `internal/runner/escalation/handler_test.go`

**What to Do:**
Update retry-loop enforcement to check explicit decomposition/escalation decision state and return typed blocked-retry errors when contract is not satisfied. Ensure retries remain forbidden for partial/non-idempotent decomposition states.

**Acceptance Criteria:**
- Same-scope retry after timeout is blocked unless decomposition succeeded or explicit escalation decision is recorded.
- Blocked-retry path returns typed error and existing user-visible message.
- Partial/non-idempotent decomposition state always blocks retry.

**Dependencies:**
- Task 1
- Task 3

### Task 5: Add decomposition partial-state detection in decomposer adapter

**Files:**
- Modify: `internal/runner/constructor_adapters.go`
- Modify: `internal/runner/constructor_test.go`

**What to Do:**
Strengthen decomposition create path to detect partial child creation outcomes and return a typed unsafe-state error. Ensure this state is explicit for retry-loop enforcement rather than silently continuing.

**Acceptance Criteria:**
- Partial child creation is detected and returned as typed contract error.
- Fully successful decomposition behavior remains unchanged.
- Existing dedupe behavior remains intact for already-created child labels.

**Dependencies:**
- Task 1

### Task 6: Add timeout decomposition and retry-block telemetry aggregation

**Files:**
- Modify: `internal/logger/process_trend.go`
- Modify: `internal/logger/process_trend_test.go`

**What to Do:**
Aggregate timeout decomposition attempts, decomposition success rate, and timeout retry-block counts from iteration logs into rolling trend outputs. Keep naming aligned with existing metric conventions.

**Acceptance Criteria:**
- Process trend exposes decomposition attempt count.
- Process trend exposes decomposition success rate.
- Process trend exposes timeout retry block count/rate.

**Dependencies:**
- Task 2
- Task 4

### Task 7: End-to-end contract validation sweep for timeout-first decomposition

**Files:**
- Modify: `internal/runner/escalation/handler_test.go`
- Modify: `internal/logger/process_trend_test.go`
- Modify: `internal/runner/constructor_test.go`

**What to Do:**
Add/adjust contract-style tests to verify full timeout -> decomposition decision -> retry gate -> telemetry flow, including partial-state and skipped decomposition outcomes.

**Acceptance Criteria:**
- Tests cover first-timeout decomposition, blocked same-scope retry, and unblocked path after valid decision.
- Tests cover partial-state contract and typed error assertions.
- Tests validate telemetry fields in iteration logs and process trend outputs.

**Dependencies:**
- Task 3
- Task 4
- Task 5
- Task 6

---

## Notes

- Acceptance criteria targeting rolling timeout failure rate (<14% for 3 windows) and first-pass success (>50%) require post-merge production/benchmark observation windows; this plan includes instrumentation needed to measure them.
- Keep string constants for user-facing retry-block messaging stable to avoid breaking existing status/log parsing.
- Prefer additive JSON fields with `omitempty` to preserve backward compatibility for existing log consumers.
