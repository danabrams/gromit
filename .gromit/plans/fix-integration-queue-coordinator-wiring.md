---
created: 2026-03-01T01:10:49Z
decomposed: true
decomposed_at: "2026-03-01T02:18:25Z"
id: fix-integration-queue-coordinator-wiring
spec: debug-integration-queue-growth
---

# Fix Integration Queue Coordinator Wiring

**Goal:** Make integration queue entries drain by replacing no-op coordinator wiring with the real `internal/integrationqueue` coordinator path.

**Architecture:** Keep single-writer queue semantics (`session -> enqueue`, `run-loop -> integrate`) but connect constructor wiring to a concrete coordinator adapter that performs queue transitions and integration operations.

**Related Investigation:** `.gromit/reports/debug-20260301-011049.md`

## Tasks

### Task 1: Add runner adapters for integrationqueue coordinator dependencies

**Files:**
- `internal/runner/constructor_adapters.go`
- `internal/runner/constructor_adapters_test.go`

**Work:**
- Add a production `integrationqueue.GitOps` adapter with argv-safe git operations.
- Add a production `integrationqueue.ScopedGate` adapter that runs the scoped validation/gate flow required before merge.
- Include interface assertions and mock updates in tests.

**Acceptance Criteria:**
- Adapters satisfy `integrationqueue.GitOps` and `integrationqueue.ScopedGate`.
- Unit tests cover success and representative failure mapping.

### Task 2: Wire real coordinator in orchestrator constructor

**Files:**
- `internal/runner/constructor.go`
- `internal/runner/constructor_test.go`

**Work:**
- Replace `NewIntegrationCoordinator()` stub usage with `integrationqueue.NewCoordinator(store, gitopsAdapter, gateAdapter)`.
- Handle store/adapter initialization errors explicitly.

**Acceptance Criteria:**
- Constructor injects real coordinator in production wiring.
- Constructor tests verify coordinator is non-nil and dependency init failures are surfaced.

### Task 3: Add behavior tests for queue drain on successful iteration

**Files:**
- `internal/runner/orchestrator_test.go`
- Optional targeted tests in `internal/integrationqueue/*_test.go`

**Work:**
- Add test proving orchestrator post-iteration call transitions one `ready` entry to a terminal non-ready state via coordinator path.
- Add recovery test proving `RecoverFromCrash` is invoked at startup and can transition stranded `integrating` entries.

**Acceptance Criteria:**
- At least one integration-style test fails against stub behavior and passes with real wiring.
- Queue transition assertions are deterministic.

### Task 4: Guard against regression in status/queue visibility

**Files:**
- `internal/runner/status_test.go` or `internal/runner/acceptance/status_*`

**Work:**
- Add assertion that queue length does not monotonically grow when coordinator succeeds across iterations in a controlled fixture.

**Acceptance Criteria:**
- Regression test demonstrates drain behavior (ready count decreases after successful coordination).

## Dependencies
- Task 1 before Task 2
- Task 2 before Task 3
- Task 3 before Task 4

## Testing Strategy
- Unit: adapter tests for gitops + gate mapping.
- Integration: orchestrator + queue fixture transitions.
- Validation commands:
  - `go test -vet=off -p 4 -parallel 4 ./...`
  - `go vet ./...`
  - `go build ./...`
