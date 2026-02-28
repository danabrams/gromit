---
created: 2026-02-28T00:00:00Z
decomposed: true
decomposed_at: "2026-02-28T03:59:00Z"
id: integration-queue-and-branch-lifecycle
source_spec: integration-queue-and-branch-lifecycle
---

# Integration Queue And Branch Lifecycle Implementation Plan

**Goal:** Introduce a durable, restart-safe FIFO integration queue and explicit branch lifecycle states so session-produced branches are integrated by a single observable coordinator path.

**Architecture:** Session commands create and advance queue entries (`draft -> ready`) while a single run-loop coordinator owns integration (`ready -> integrating -> terminal`) with strict transition validation, deterministic ordering, and persisted diagnostics.

**Tech Stack:** Go, existing runner/worktree/state infrastructure, Cobra CLI status surfaces, JSON on-disk state under `.gromit/`.

**Spec:** `.gromit/specs/integration-queue-and-branch-lifecycle.md`

---

## Architecture

**Overview:**  
Add a durable integration queue subsystem backed by `.gromit/integration-queue.json`, then route session branch handoff through it so session commands only enqueue work while a single coordinator in `gromit run` performs FIFO integration and state transitions.

**Key Components:**
1. **`internal/integrationqueue` (new package):** Owns schema, validation, transition rules, atomic read/write, FIFO ordering, and error classification.
2. **Session Handoff Adapter (extend `cmd/gromit/interactive_worktree.go`):** Replaces direct `MergeBack` as primary path with `draft -> ready` enqueue on successful auto-commit, including metadata (branch/session/changed-files hash).
3. **Integration Coordinator (runner hook):** New coordinator step in run-loop lifecycle to pick oldest `ready`, set `integrating`, run integration sequence, and finalize terminal states.
4. **Status Projection Layer:** Extends status assembly/rendering to include queue summary and entry diagnostics in deterministic order.
5. **Compatibility Bridge:** Keeps `interactive-state.json` pending branch list as transitional/derived data during migration to avoid regressions.

**Integration Points:**
- `cmd/gromit/interactive_worktree.go`  
  enqueue branch state instead of immediate merge-back ownership.
- `internal/runner/*` (`orchestrator` / run-loop seam)  
  invoke coordinator between iterations.
- `internal/runner/print_status.go` + `internal/runner/display/*`  
  add Integration Queue section to human-readable `gromit status`.
- `internal/pipeline/status.go`  
  optionally carry queue summary in structured status model.
- `internal/state/interactive_state.go`  
  migration-safe coexistence with pending branches.

**Data Flow:**
1. Session starts: create queue entry in `draft`.
2. Session ends with auto-commit: update entry to `ready` with commit/head/diff hash metadata.
3. Coordinator cycle: load queue from disk, choose oldest `ready` (`fifo_seq`), mark `integrating`.
4. Coordinator outcome:
- success -> `merged`
- merge/rebase conflict -> `conflict`
- gate failures after one rebase retry -> `failed_gates`
- policy violation -> `lane_violation`
5. `gromit status` reads queue file and renders counts, FIFO positions for `ready`, and latest failure reason.

**Files to Modify:**
- `cmd/gromit/interactive_worktree.go`
- `internal/runner/orchestrator.go` (or closest run-loop integration seam)
- `internal/runner/print_status.go`
- `internal/runner/display/display.go` (and related display types/tests)
- `internal/pipeline/status.go` (if status struct is extended)

**Files to Create:**
- `internal/integrationqueue/types.go`
- `internal/integrationqueue/store.go`
- `internal/integrationqueue/transition.go`
- `internal/integrationqueue/coordinator.go`
- `internal/integrationqueue/*_test.go` (schema/transition/FIFO/restart tests)

**Tradeoffs:**
- **Separate queue file vs extending `interactive-state.json`:** chose dedicated file for explicit schema/versioning and easier validation contracts.
- **Coordinator inside `gromit run` vs daemon:** chose embedded first for simpler rollout and reuse of existing lifecycle hooks.
- **Strict transition validator:** chose fail-closed semantics to prevent silent state corruption, even if it initially surfaces more explicit errors.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** queue schema validation, transition legality, atomic persistence behavior, FIFO selector, error code mapping, retry policy logic.
2. **Integration Tests:** session handoff + coordinator loop with temp git repo fixtures, including restart recovery (reload from disk), conflict path, and gate retry path.
3. **Acceptance Tests:** `gromit status` output contract (queue summary, ordering, diagnostics), and end-to-end branch lifecycle across session completion to merged/blocked terminal states.

**Key Test Cases:**
- Queue file initializes with `schema_version: 1` and survives restart with same entries/states.
- Illegal transitions (example: `ready -> merged`) are rejected with `invalid_transition`.
- FIFO selection always chooses smallest `fifo_seq` among `ready`.
- Coordinator marks `integrating` before attempting merge and always persists terminal state.
- Gate failure path performs exactly one automatic retry (with fresh rebase), then transitions to `failed_gates`.
- Conflict path transitions to `conflict` and preserves error metadata.
- Lane violation transitions to `lane_violation` with policy error code.
- `gromit status` shows queue length, per-state counts, ready positions, and latest failure reason.
- Status ordering contract is deterministic: `integrating` first, then `ready` by FIFO, then blocked by latest update.

**Mocking Strategy:**
- Mock git/integration operations in coordinator unit tests to force success/conflict/gate-failure/lane-violation branches deterministically.
- Use real file IO (temp dirs) for store tests to verify atomic write-rename behavior.
- Use golden-style output tests for status formatting.
- Use real git only in targeted integration tests where branch/rebase semantics matter.

**Coverage Goals:**
- All legal and illegal transition edges.
- Crash/restart durability scenarios (read-after-partial-cycle).
- Retry counter correctness and terminal-state visibility.
- `gromit status` queue section rendering and deterministic ordering.
- Non-loss guarantee: no `ready` entry disappears without terminal transition.

**Test Organization:**
- Queue package tests under `internal/integrationqueue/*_test.go`.
- Status contract tests in `internal/runner/status_test.go` and/or display tests.
- Coordinator integration tests near runner/coordinator seam (`internal/runner/*` or `internal/integrationqueue` integration tests).
- CLI surface assertions in `cmd/gromit/status_test.go`.

## Implementation Tasks

### Task 1: Create Queue Types And Durable Store

**Files:**
- Create: `internal/integrationqueue/types.go`
- Create: `internal/integrationqueue/store.go`
- Test: `internal/integrationqueue/store_test.go`

**What to Do:**
Define schema v1 queue/entry structs and implement durable load/save for `.gromit/integration-queue.json` with atomic write-then-rename semantics, schema validation, and unknown-enum rejection.

**Acceptance Criteria:**
- Store writes are atomic and crash-safe.
- Invalid schema or enum values fail closed.
- Queue reload after restart preserves all entries and timestamps.

**Dependencies:** None

### Task 2: Implement State Machine And Transition Validation

**Files:**
- Create: `internal/integrationqueue/transition.go`
- Test: `internal/integrationqueue/transition_test.go`

**What to Do:**
Implement explicit lifecycle states and allowed transitions (`draft`, `ready`, `integrating`, `merged`, `conflict`, `failed_gates`, `lane_violation`), including transition reasons and standardized error-code assignment for invalid transitions.

**Acceptance Criteria:**
- All allowed transitions pass.
- Disallowed transitions return typed validation errors and map to `invalid_transition`.
- Terminal transitions retain last error details when applicable.

**Dependencies:** Task 1

### Task 3: Add FIFO Ordering And Queue Query Helpers

**Files:**
- Modify: `internal/integrationqueue/types.go`
- Modify/Create: `internal/integrationqueue/store.go` (or helper file)
- Test: `internal/integrationqueue/fifo_test.go`

**What to Do:**
Add deterministic queue ordering helpers, including oldest `ready` selection by `fifo_seq`, queue-position computation for `ready` entries, and stable non-merged entry sorting for status display.

**Acceptance Criteria:**
- Oldest `ready` is always selected first.
- Ready entries expose stable FIFO positions.
- Sorting contract for status projection is deterministic.

**Dependencies:** Task 1, Task 2

### Task 4: Wire Session Branch Handoff To Queue (`draft -> ready`)

**Files:**
- Modify: `cmd/gromit/interactive_worktree.go`
- Modify: `internal/state/interactive_state.go` (if compatibility bridge needed)
- Test: `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
Replace immediate merge-back ownership with queue handoff: create/update integration entry with branch, session metadata, changed-files hash, and transition to `ready` after successful session commit. Preserve pending-branch compatibility behavior during migration.

**Acceptance Criteria:**
- Session completion records durable queue entry.
- Session failures remain visible and do not silently drop branch state.
- Existing pending-branch behavior is preserved or intentionally migrated with coverage.

**Dependencies:** Task 1, Task 2

### Task 5: Implement Coordinator Loop For `ready -> terminal`

**Files:**
- Create: `internal/integrationqueue/coordinator.go`
- Modify: `internal/runner/orchestrator.go` (or run-loop seam)
- Test: `internal/integrationqueue/coordinator_test.go`

**What to Do:**
Add coordinator integration step that reloads queue each cycle, marks one oldest `ready` as `integrating`, runs integration attempt, and transitions to terminal states with standardized error codes and messages.

**Acceptance Criteria:**
- Coordinator processes one branch at a time in strict FIFO.
- Every processed `ready` entry reaches a persisted terminal or retry-visible state.
- One branch failure does not terminate the entire run loop.

**Dependencies:** Task 2, Task 3, Task 4

### Task 6: Enforce Retry Policy For Gate Failures

**Files:**
- Modify: `internal/integrationqueue/coordinator.go`
- Test: `internal/integrationqueue/coordinator_retry_test.go`

**What to Do:**
Implement retry policy: exactly one automatic retry after fresh rebase for gate failures; after retry exhaustion transition to `failed_gates` with preserved diagnostics; no infinite retries.

**Acceptance Criteria:**
- First gate failure triggers one retry path.
- Second gate failure transitions to `failed_gates`.
- Retry/attempt counters and timestamps are correct.

**Dependencies:** Task 5

### Task 7: Add `gromit status` Integration Queue Section

**Files:**
- Modify: `internal/runner/print_status.go`
- Modify: `internal/runner/display/display.go`
- Modify: `internal/runner/display/types.go` (if needed)
- Test: `internal/runner/status_test.go`
- Test: `cmd/gromit/status_test.go`

**What to Do:**
Extend status rendering to show Integration Queue summary: queue length, state counts, FIFO positions for ready entries, and latest error reasons for non-merged entries, with deterministic ordering and bounded entry list.

**Acceptance Criteria:**
- Status includes integration queue summary and entry diagnostics.
- Ordering and formatting are deterministic under repeated calls.
- Empty queue and invalid queue-file scenarios are handled clearly.

**Dependencies:** Task 1, Task 3, Task 5

### Task 8: Add Status JSON Contract Projection

**Files:**
- Modify: `internal/pipeline/status.go` (or equivalent status model seam)
- Modify: `internal/runner/print_status.go` (if plumbed through)
- Test: `internal/pipeline/status_test.go`

**What to Do:**
Add structured `integration_queue` payload projection aligned with schema/status contract, including counts and entry summaries needed for machine-readable status consumers.

**Acceptance Criteria:**
- Structured status includes queue summary fields and entry data.
- Entry ordering matches contract.
- Missing/invalid queue file degrades safely with explicit indicator.

**Dependencies:** Task 1, Task 3, Task 7

### Task 9: Migration, Recovery, And End-to-End Guardrails

**Files:**
- Modify: `internal/integrationqueue/store.go`
- Modify: `internal/runner/orchestrator.go` (startup recovery path)
- Test: `internal/integrationqueue/recovery_test.go`
- Test: `internal/runner/acceptance/*` (new/updated acceptance coverage)

**What to Do:**
Implement recovery handling for pre-existing pending session branches and malformed queue files, ensuring fail-closed behavior (`queue_schema_invalid`) and explicit user-visible diagnostics without dropping entries.

**Acceptance Criteria:**
- Restart recovery preserves entries and in-flight states.
- Malformed queue file pauses integration safely with explicit error code.
- Legacy pending branches are either migrated or surfaced with no silent loss.

**Dependencies:** Task 4, Task 5, Task 7

---

## Notes

- Keep phase-1 scheduling strictly FIFO; defer priorities and multi-lane arbitration beyond enum/state plumbing.
- Prefer fail-closed validation for schema/transition violations to avoid silent queue corruption.
- Ensure non-merged queue entries remain visible until explicit terminal transitions are persisted.
- Coordinate with the related schema/status spec (`integration-queue-schema-and-status-contract`) to avoid contract drift during implementation.
