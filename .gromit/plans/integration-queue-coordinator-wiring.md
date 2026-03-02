---
created: 2026-03-01T00:00:00Z
decomposed: true
decomposed_at: "2026-03-02T04:15:49Z"
id: integration-queue-coordinator-wiring
source_spec: integration-queue-coordinator-wiring
---

# Integration Queue Coordinator Wiring Implementation Plan

**Goal:** Replace stub run-loop coordinator wiring with the real integration queue coordinator so `ready` entries are actually processed and startup recovery executes persisted queue recovery.

**Architecture:** Wire `internal/integrationqueue.NewCoordinator` in production constructor paths using runner adapters for `integrationqueue.GitOps` and `integrationqueue.ScopedGate`, while preserving existing orchestrator coordination call points.

**Tech Stack:** Go, runner orchestrator construction, integration queue store/coordinator, git subprocess adapters, validation command configuration.

**Spec:** `.gromit/specs/integration-queue-coordinator-wiring.md`

---

## Architecture

**Overview:**
Constructor wiring in `newRunnerImpl` will instantiate the real integration queue coordinator and inject it into orchestrator config. Runner-level adapters will satisfy `integrationqueue.GitOps` and `integrationqueue.ScopedGate`, allowing existing orchestrator `RecoverFromCrash(...)` and `Coordinate(...)` calls to drive durable queue state transitions.

**Key Components:**
1. **Production coordinator wiring in constructor:** Build queue store from `gromitDir`, create adapter instances, and inject `integrationqueue.NewCoordinator(...)`.
2. **Runner GitOps adapter:** Implement `FetchAndRebase`, `MergeToMain`, `Push`, and `Cleanup` via argv-safe git subprocess calls aligned with existing process-capacity and process-group safety patterns.
3. **Runner ScopedGate adapter:** Implement queue-entry gate execution using existing validation config, including touched-package scoping from queue entry changed files where applicable.
4. **Orchestrator seam preservation:** Keep current lifecycle hooks (`RecoverFromCrash` on startup, `Coordinate` after successful iterations) unchanged so behavior changes are wiring-only.
5. **Regression guard tests:** Constructor and orchestrator seam tests that fail if no-op coordinator behavior is reintroduced.

**Integration Points:**
- `internal/runner/constructor.go`: replace `NewIntegrationCoordinator()` with real `integrationqueue` coordinator construction.
- `internal/runner/constructor_adapters.go` (or new `internal/runner/integration_queue_adapters.go`): define production adapters implementing queue interfaces.
- `internal/runner/orchestrator.go`: no call-site logic changes; rely on existing calls.
- `internal/runner/orchestrator_test.go` and adapter tests: prove queue drain and crash recovery with real wiring.

**Data Flow:**
1. Session paths enqueue branches into `.gromit/integration-queue.json` with `state=ready`.
2. `newRunnerImpl` wires real coordinator dependencies.
3. At orchestrator startup, `RecoverFromCrash` transitions stranded `integrating` entries to `ready`.
4. After each successful iteration, `Coordinate` selects the oldest `ready` entry and executes one integration attempt.
5. Queue entry transitions persist terminal outcomes (`merged`, `conflict`, `failed_gates`, `lane_violation`) in queue store.

**Files to Modify:**
- `internal/runner/constructor.go`
- `internal/runner/constructor_adapters.go`
- `internal/runner/constructor_adapters_interface_checks_test.go`
- `internal/runner/orchestrator_test.go`
- `internal/runner/acceptance/integration_recovery_test.go` (if seam-level startup assertions need strengthening)

**Files to Create:**
- `internal/runner/integration_queue_adapters.go` (if adapters are split out for clarity)
- `internal/runner/integration_queue_adapters_test.go`

**Tradeoffs:**
- **Adapter integration vs embedding logic in coordinator package:** adapter approach keeps runner dependencies local and avoids circular coupling.
- **Wiring fix vs orchestration redesign:** minimum-risk change that addresses current bug directly.
- **Single-entry per cycle behavior:** preserves deterministic FIFO progress and current run-loop throughput model.

## Test Strategy

**Test Levels:**
1. **Unit tests:** runner adapter contract tests (`integrationqueue.GitOps`, `integrationqueue.ScopedGate`).
2. **Constructor seam tests:** verify production constructor uses real coordinator wiring.
3. **Orchestrator integration tests:** queue-backed tests proving `ready` drains and startup recovery runs.

**Key Test Cases:**
- Constructor-produced orchestrator has non-nil coordinator and real queue behavior (not no-op semantics).
- A queue containing a single `ready` entry transitions out of `ready` during orchestrator execution.
- Startup with `integrating` entries transitions them to `ready` via real `RecoverFromCrash`.
- Coordinator still runs only after successful iterations (existing run-loop guard preserved).
- Queue status/projection remains compatible with persisted schema after real coordinator transitions.

**Mocking Strategy:**
- Use temp queue store files for durable state assertions.
- Use deterministic command/gate doubles in adapter tests to avoid flaky git/environment dependencies.
- Reuse existing fake stages and bead fetch callbacks in orchestrator tests; keep scope limited to coordination seams.

**Coverage Goals:**
- Detect regression to no-op coordinator injection in constructor.
- Verify real coordinate path mutates queue state from `ready`.
- Verify startup recovery path executes real state recovery.

**Test Organization:**
- `internal/runner/integration_queue_adapters_test.go`: adapter behavior and interface contract coverage.
- `internal/runner/orchestrator_test.go`: constructor/orchestrator seam behavior for queue drain.
- `internal/runner/acceptance/integration_recovery_test.go`: startup recovery assertions with persisted queue data.

## Implementation Tasks

### Task 1: Replace Stub Constructor Wiring With Real Coordinator

**Files:**
- Modify: `internal/runner/constructor.go`

**What to Do:**
Replace `NewIntegrationCoordinator()` injection with real constructor wiring:
- `integrationqueue.NewStore(gromitDir)`
- production `integrationqueue.GitOps` adapter
- production `integrationqueue.ScopedGate` adapter
- `integrationqueue.NewCoordinator(store, gitops, gate)`

**Acceptance Criteria:**
- Production constructor no longer wires the no-op runner coordinator.
- Constructor handles queue store init errors explicitly.
- Orchestrator config receives the real coordinator instance.

**Dependencies:**
- Task 2
- Task 3

### Task 2: Implement Runner GitOps Adapter For Integration Queue

**Files:**
- Modify/Create: `internal/runner/constructor_adapters.go` or `internal/runner/integration_queue_adapters.go`
- Test: `internal/runner/integration_queue_adapters_test.go`
- Test: `internal/runner/constructor_adapters_interface_checks_test.go`

**What to Do:**
Add adapter implementing `integrationqueue.GitOps`:
- `FetchAndRebase(ctx, entry)`
- `MergeToMain(ctx, entry)`
- `Push(ctx)`
- `Cleanup(ctx, entry)`

Use argv-safe subprocess execution and existing process safety conventions.

**Acceptance Criteria:**
- Adapter satisfies `integrationqueue.GitOps` at compile time.
- Git command ordering and error mapping are deterministic.
- Adapter tests cover success and representative failure classifications.

**Dependencies:**
- None

### Task 3: Implement Runner ScopedGate Adapter For Integration Queue

**Files:**
- Modify/Create: `internal/runner/constructor_adapters.go` or `internal/runner/integration_queue_adapters.go`
- Test: `internal/runner/integration_queue_adapters_test.go`
- Possibly modify: `internal/config/config_accessors.go` usage-only (no schema changes)

**What to Do:**
Add adapter implementing `integrationqueue.ScopedGate` that runs configured validation gates with queue-entry context, including scoping of `go test ./...` commands from changed files-derived package scope where applicable.

**Acceptance Criteria:**
- Adapter satisfies `integrationqueue.ScopedGate` at compile time.
- Gate failures return actionable errors consumed by coordinator transitions.
- Scoped command behavior is deterministic for changed-file-driven entries.

**Dependencies:**
- None

### Task 4: Add Constructor And Orchestrator Regression Tests For Real Coordination

**Files:**
- Modify: `internal/runner/orchestrator_test.go`
- Modify: `internal/runner/constructor_test.go` and/or existing constructor seam tests

**What to Do:**
Add tests proving that constructor wiring executes real coordination behavior:
- Seed queue with a `ready` entry.
- Run orchestrator through one successful iteration.
- Assert the seeded entry transitions out of `ready`.

**Acceptance Criteria:**
- Test fails if coordinator is swapped back to no-op.
- Test verifies one-entry-per-iteration queue processing behavior.
- Existing coordinator error-isolation semantics remain intact.

**Dependencies:**
- Task 1
- Task 2
- Task 3

### Task 5: Strengthen Crash-Recovery Startup Coverage At Seam

**Files:**
- Modify: `internal/runner/acceptance/integration_recovery_test.go`
- Possibly modify: `internal/runner/orchestrator_test.go`

**What to Do:**
Add startup seam coverage using persisted queue data containing `integrating` entries and assert real recovery transitions to `ready` (with recovery error metadata preserved).

**Acceptance Criteria:**
- Startup path verifies real `RecoverFromCrash` behavior against store-backed queue state.
- Tests fail if startup recovery becomes no-op.
- Recovery assertions align with existing queue transition contract.

**Dependencies:**
- Task 1

### Task 6: Validate Status Compatibility And Run Quality Gates

**Files:**
- Test-only checks in existing status seams:
- `internal/runner/status_test.go`
- `internal/pipeline/status_test.go`
- `cmd/gromit/status_test.go`

**What to Do:**
Run and update targeted tests if needed to confirm real coordinator transitions do not regress status rendering or schema expectations.

**Acceptance Criteria:**
- Queue status tests pass without schema contract changes.
- No regressions in queue state projection after real coordination.
- Required test commands for touched packages pass.

**Dependencies:**
- Task 4
- Task 5

---

## Notes

- This plan is intentionally a wiring/adapters fix and does not redesign queue lifecycle semantics.
- Preserve single-writer ownership in the run loop; do not reintroduce direct interactive/session merge ownership.
- If adapter implementation discovers additional queue policy gaps, file follow-up issues linked as `discovered-from` the tracking bead before expanding scope.
