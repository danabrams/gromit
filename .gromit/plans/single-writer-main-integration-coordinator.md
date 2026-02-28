---
id: single-writer-main-integration-coordinator
source_spec: single-writer-main-integration-coordinator
created: 2026-02-28
decomposed: false
---

# Single-Writer Main Integration Coordinator Implementation Plan

**Goal:** Enforce a single-writer policy for `main` by moving all integration ownership into one run-loop coordinator, while interactive sessions auto-commit to isolated branches and enqueue for integration.

**Architecture:** Session commands produce committed branch artifacts and queue entries only; coordinator code running between `gromit run` iterations performs rebase/gate/integrate/push/cleanup and records terminal queue state.

**Tech Stack:** Go, existing worktree/session flow, runner orchestration, JSON-backed queue state under `.gromit/`.

**Spec:** `.gromit/specs/single-writer-main-integration-coordinator.md`

---

## Architecture

**Overview:**  
Introduce a durable integration queue and remove direct merge-to-main behavior from interactive session and epilogue paths. Interactive commands end with auto-stage + auto-commit and queue handoff. A coordinator in the run loop is the only path allowed to mutate `main`.

**Key Components:**
1. **`internal/integrationqueue` package:** Owns queue schema, load/save, transitions, ordering, and integration coordinator logic.
2. **Session completion handoff in interactive launcher:** Performs auto-stage + auto-commit and marks branch ready for integration (or blocked when commit fails).
3. **Run-loop integration coordinator hook:** Invoked between iterations to process one queued branch deterministically.
4. **Integration git/gate operations adapter:** Runs fetch/rebase, scoped quality gates, integration merge, push, and cleanup with typed outcomes.
5. **Queue status projection:** Keeps branch lifecycle visible and debuggable for users/operators.

**Integration Points:**
- `cmd/gromit/interactive_worktree.go` stops calling direct merge ownership and performs commit + queue handoff.
- `internal/pipeline/epilogue/epilogue.go` removes legacy pending-branch merge-to-main behavior.
- `internal/runner/orchestrator.go` invokes coordinator between loop iterations and treats coordinator failures as non-fatal iteration warnings.
- `internal/runner/constructor.go` / `internal/runner/constructor_adapters.go` wires queue store and coordinator dependencies.
- `internal/state/interactive_state.go` remains compatibility state during migration and cleanup.

**Data Flow:**
1. Session command creates isolated branch/worktree.
2. Session callback returns; gromit auto-stages and auto-commits in session worktree.
3. Branch is marked `ready` in integration queue (or `blocked` with recovery details on commit failure).
4. Run-loop coordinator selects oldest `ready`, marks `integrating`, performs integration sequence:
   - fetch/rebase on latest `origin/main`
   - scoped quality gates
   - integrate into `main`
   - push remote
5. Coordinator marks terminal state (`merged`, `conflict`, `failed_gates`, `push_failed`, etc.) and performs cleanup/removal from pending lifecycle.
6. On branch failure, coordinator records status and continues the run loop.

**Files to Modify:**
- `cmd/gromit/interactive_worktree.go`
- `cmd/gromit/interactive_worktree_test.go`
- `internal/pipeline/epilogue/epilogue.go`
- `internal/pipeline/epilogue/epilogue_test.go`
- `internal/runner/orchestrator.go`
- `internal/runner/orchestrator_test.go`
- `internal/runner/constructor.go`
- `internal/runner/constructor_adapters.go`

**Files to Create:**
- `internal/integrationqueue/types.go`
- `internal/integrationqueue/store.go`
- `internal/integrationqueue/transitions.go`
- `internal/integrationqueue/coordinator.go`
- `internal/integrationqueue/gitops.go`
- `internal/integrationqueue/types_test.go`
- `internal/integrationqueue/store_test.go`
- `internal/integrationqueue/transitions_test.go`
- `internal/integrationqueue/coordinator_test.go`

**Tradeoffs:**
- **Queue + coordinator vs immediate merge:** added subsystem complexity, but explicit single-writer ownership and safer concurrency.
- **Embedded coordinator vs daemon in phase 1:** simpler rollout and lower operational overhead now, less decoupled from run availability.
- **Fail-closed transition rules:** stricter behavior surfaces more explicit errors early but prevents silent lifecycle corruption.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** queue schema/validation, transition matrix, auto-commit helpers, coordinator state transitions.
2. **Integration Tests:** temp git repo scenarios for rebase/merge/push and queue persistence across restarts.
3. **Acceptance Tests:** interactive sessions no longer require manual merge/push; only coordinator path mutates `main`.

**Key Test Cases:**
- Session completion auto-stage + auto-commit succeeds with modified files and enqueues branch.
- Session completion with no staged changes behaves as non-fatal and still records queue readiness semantics.
- Commit creation failure marks branch blocked and includes clear recovery guidance.
- Interactive session flow no longer invokes `MergeBack` directly.
- Epilogue no longer performs legacy merge-back integration into `main`.
- Coordinator selects the oldest `ready` entry (FIFO) and processes one branch per invocation.
- Coordinator success path records merged state and cleanup metadata.
- Rebase conflict and gate failure paths mark deterministic failure states while preserving queue entry diagnostics.
- Push failure marks branch as retryable/failed according to policy without crashing run loop.
- Coordinator failure does not terminate overall run-loop execution.

**Mocking Strategy:**
- Mock git integration ops for deterministic coordinator unit tests.
- Use real filesystem temp dirs for queue store atomic-write and restart-safety tests.
- Use mocked session dependencies in command tests to verify call ordering and removal of direct merges.
- Use targeted real-git integration tests for one end-to-end happy path and one conflict path.

**Coverage Goals:**
- Full state transition legality matrix.
- Session completion outcomes (changed/no-change/commit-failure).
- Coordinator outcomes (success/conflict/gate-fail/push-fail/non-fatal continuation).
- Architectural invariant: only coordinator code path mutates `main`.

**Test Organization:**
- `internal/integrationqueue/*_test.go` for core queue/coordinator behavior.
- `cmd/gromit/interactive_worktree_test.go` for session completion and handoff behavior.
- `internal/pipeline/epilogue/epilogue_test.go` for legacy merge path removal.
- `internal/runner/orchestrator_test.go` for between-iteration coordinator behavior and error isolation.

## Implementation Tasks

### Task 1: Add Integration Queue Schema And Durable Store

**Files:**
- Create: `internal/integrationqueue/types.go`
- Create: `internal/integrationqueue/store.go`
- Test: `internal/integrationqueue/types_test.go`
- Test: `internal/integrationqueue/store_test.go`

**What to Do:**
Define queue entry/state model and implement atomic load/save for `.gromit/integration-queue.json`, including validation hooks and deterministic serialization.

**Acceptance Criteria:**
- Queue file persists and reloads without entry loss.
- Unknown/invalid schema values fail closed with typed errors.
- Entry ordering metadata is preserved across process restarts.

**Dependencies:** None

### Task 2: Implement Transition Rules And Queue Lifecycle Helpers

**Files:**
- Create: `internal/integrationqueue/transitions.go`
- Test: `internal/integrationqueue/transitions_test.go`

**What to Do:**
Implement explicit branch lifecycle states and allowed transitions (`draft`, `ready`, `integrating`, terminal states) with reason/error-code capture.

**Acceptance Criteria:**
- Allowed transitions succeed; disallowed transitions return `invalid_transition`-style errors.
- Transition helper updates timestamps and keeps diagnostics intact.
- Terminal states retain integration failure context.

**Dependencies:**
- Task 1

### Task 3: Update Session Completion To Auto-Commit And Enqueue

**Files:**
- Modify: `cmd/gromit/interactive_worktree.go`
- Test: `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
Replace direct `MergeBack` ownership in interactive session completion with auto-stage + auto-commit in session worktree, then queue handoff to `ready`. On commit failure, preserve branch and record blocked queue state with recovery command guidance.

**Acceptance Criteria:**
- Successful session completion creates commit and queue-ready entry.
- Direct merge to `main` is not attempted in session command path.
- Commit failure path is explicit, non-silent, and recoverable.

**Dependencies:**
- Task 1
- Task 2

### Task 4: Remove Legacy Epilogue Merge-To-Main Path

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Test: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
Disable/remove epilogue’s pending-worktree branch merge behavior so run-loop epilogue no longer performs integration ownership intended for the new coordinator.

**Acceptance Criteria:**
- Epilogue no longer calls `MergeBack` for pending branches.
- Existing epilogue responsibilities (status/log/review hooks) remain intact.
- Tests assert legacy merge behavior is not reintroduced.

**Dependencies:**
- Task 3

### Task 5: Implement Coordinator Integration Sequence

**Files:**
- Create: `internal/integrationqueue/coordinator.go`
- Create: `internal/integrationqueue/gitops.go`
- Test: `internal/integrationqueue/coordinator_test.go`

**What to Do:**
Implement coordinator execution for one queued branch at a time: select oldest `ready`, mark `integrating`, run fetch/rebase, run scoped gates, integrate to `main`, push, then finalize queue state and cleanup metadata.

**Acceptance Criteria:**
- Coordinator processes at most one `ready` branch per cycle in FIFO order.
- Success path marks entry merged and records cleanup completion.
- Any branch failure records deterministic terminal/blocked state with reason.

**Dependencies:**
- Task 1
- Task 2

### Task 6: Wire Coordinator Into Run Loop Between Iterations

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/constructor_adapters.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Inject coordinator dependency in runner construction and invoke it between iterations. Ensure coordinator errors are surfaced as warnings and do not terminate the whole run loop.

**Acceptance Criteria:**
- Coordinator executes during run lifecycle between iterations.
- A coordinator error does not abort `gromit run`.
- `main` updates in run mode are mediated by coordinator path only.

**Dependencies:**
- Task 5
- Task 4

### Task 7: Enforce Single-Writer Policy With Regression Guards

**Files:**
- Modify: `cmd/gromit/interactive_worktree_test.go`
- Modify: `internal/pipeline/epilogue/epilogue_test.go`
- Modify: `internal/runner/orchestrator_test.go`

**What to Do:**
Add invariant tests that session commands and epilogue never integrate directly to `main`, and coordinator remains the sole integration owner.

**Acceptance Criteria:**
- Regression tests fail if any non-coordinator path merges to `main`.
- Session and run-loop concurrency scenario keeps isolation without index/worktree contention.
- Invariants are explicit and easy to audit.

**Dependencies:**
- Task 3
- Task 4
- Task 6

### Task 8: Status And Recovery Visibility For Queue Outcomes

**Files:**
- Modify: `internal/runner/print_status.go` (or existing status projection seam)
- Modify: related status display tests
- Test: queue/coordinator recovery tests as needed

**What to Do:**
Expose integration queue outcomes (ready/integrating/blocked/merged summaries and last error reasons) and ensure blocked session failures include clear operator recovery instructions.

**Acceptance Criteria:**
- Status surfaces integration queue state clearly for operators.
- Failed/blocked entries include actionable reason text.
- Startup/reload retains and re-displays unresolved entries.

**Dependencies:**
- Task 1
- Task 2
- Task 6

---

## Notes

- Existing plans/specs for integration queue schema/lifecycle can be reused as implementation references, but this plan is authoritative for enforcing the single-writer architectural boundary.
- Migration should preserve compatibility with existing `interactive-state.json` pending branch data until queue-first flow is fully stable.
- Keep coordinator processing bounded (one branch per cycle) in phase 1 to reduce contention and simplify reasoning.
