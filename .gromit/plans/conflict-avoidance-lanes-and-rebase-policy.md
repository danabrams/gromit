---
id: conflict-avoidance-lanes-and-rebase-policy
source_spec: conflict-avoidance-lanes-and-rebase-policy
created: 2026-02-28
decomposed: false
---

# Conflict Avoidance Lanes And Rebase Policy Implementation Plan

**Goal:** Add file-based integration lanes and lane-aware rebase/gate automation so source-changing interactive branches are first-class while hard safety violations are strictly blocked.

**Architecture:** Extend the integration queue/coordinator path with lane classification from changed files, narrow hard-safety blocking (`lane_violation`), and automated `code_lane` rebase + touched-package gates + one retry while preserving FIFO progress.

**Tech Stack:** Go, existing session worktree flow, existing git/specbranch operations, runner orchestration/status surfaces, `.gromit` rules and queue metadata.

**Spec:** `.gromit/specs/conflict-avoidance-lanes-and-rebase-policy.md`

---

## Architecture

**Overview:**
Implement lane-aware integration inside a new `internal/integrationqueue` subsystem that classifies branches by changed files (not command), enforces hard safety blocking from `.gromit/RULES.md`, and runs automated rebase plus touched-package gates for `code_lane` entries while preserving queue throughput on conflict or terminal failure.

**Key Components:**
1. **`internal/integrationqueue` package:** Queue schema helpers, lane enum, classifier, safety validator, retry tracking, and coordinator execution helpers.
2. **Lane classifier:** Determines `safe_lane` vs `code_lane` from changed files; command names like `debug`, `review`, and `retro` do not affect lane choice.
3. **Hard safety validator:** Applies prohibited artifact rules and emits explicit `lane_violation` state with reason code.
4. **Lane-aware coordinator executor:** Runs `code_lane` rebase/gate/retry automation and `safe_lane` fast-path validation while preserving hard safety checks.
5. **Run-loop integration hook:** Processes one oldest `ready` entry at a time and continues queue processing after conflict/failure.
6. **Status projection updates:** Shows lane, state, retry attempt, and failure reason in text and JSON status output.

**Integration Points:**
- `cmd/gromit/interactive_worktree.go` for queue handoff instead of immediate merge ownership.
- `internal/runner/orchestrator.go` for per-cycle coordinator processing.
- `internal/runner/specbranch/git_ops.go` for rebase/conflict operations.
- `internal/runner/print_status.go` and display/status JSON seams for queue diagnostics.
- `.gromit/RULES.md` as source of hard safety artifact policy.

**Data Flow:**
1. Session completion records branch metadata and changed files into queue entry.
2. Classifier assigns lane from changed-file analysis.
3. Hard safety validation runs first and transitions violations to `lane_violation`.
4. Coordinator picks oldest `ready` entry:
   - `safe_lane`: minimal validation path + integration.
   - `code_lane`: rebase onto latest main, run touched-package gates, retry once after fresh rebase on gate failure.
5. Coordinator writes terminal state (`merged`, `conflict`, `failed_gates`, `lane_violation`) and proceeds to next FIFO entry.

**Files to Modify:**
- `cmd/gromit/interactive_worktree.go`
- `internal/runner/orchestrator.go`
- `internal/runner/print_status.go`
- `internal/runner/display/display.go` and related display/status wiring
- `internal/pipeline/status.go` or existing status JSON seam
- `internal/state/interactive_state.go` (migration/compatibility bridge as needed)

**Files to Create:**
- `internal/integrationqueue/types.go`
- `internal/integrationqueue/classifier.go`
- `internal/integrationqueue/safety.go`
- `internal/integrationqueue/coordinator.go`
- `internal/integrationqueue/retry.go`
- `internal/integrationqueue/*_test.go`

**Tradeoffs:**
- **File-based lanes vs command allowlists:** file-based aligns with real branch content and avoids false blocks on interactive commands that modify code.
- **Strict blocking scope:** hard blocking is limited to safety violations; normal source overlap is handled via automation.
- **Retry policy:** exactly one retry balances recovery value with deterministic failure handling.
- **Queue behavior:** fail-closed per branch while fail-open for overall queue throughput.

## Test Strategy

**Test Levels:**
1. **Unit tests:** lane classification, safety violation detection, transition mapping, and retry logic.
2. **Integration tests:** coordinator behavior across lane types with deterministic git/gate outcomes.
3. **Acceptance tests:** status contract and queue progression when one branch conflicts/fails.

**Key Test Cases:**
- `.gromit/**` and docs/spec metadata-only changes classify as `safe_lane`.
- Source/test/config/build changes classify as `code_lane`, regardless of command origin (`debug`, `review`, `retro`, etc.).
- Mixed changes classify as `code_lane`.
- Prohibited runtime/local-state artifacts trigger `lane_violation` with explicit reason.
- `code_lane` executes rebase + touched-package gates and performs exactly one retry after fresh rebase.
- Rebase or merge conflict transitions to `conflict` with preserved branch for manual resolution.
- After conflict or terminal failure, coordinator still processes subsequent FIFO entries.
- `safe_lane` path remains minimal but still enforces hard safety checks.
- Text and JSON status output include deterministic lane/state/attempt/failure details.

**Mocking Strategy:**
- Mock git and validation executors in coordinator tests to force success/conflict/gate-failure/lane-violation outcomes.
- Use real temp-dir file I/O for queue persistence and restart safety tests.
- Use deterministic formatting tests for status output contracts.

**Coverage Goals:**
- Full lane-classification matrix coverage.
- One-retry-only guarantee for gate failure.
- Hard safety fail-closed coverage for prohibited artifacts.
- Queue throughput guarantee under mixed success/failure states.

**Test Organization:**
- `internal/integrationqueue/*_test.go` for core lane/safety/coordinator behavior.
- `internal/runner/status*_test.go` and display tests for rendering/JSON projection.
- `internal/runner/acceptance/*` for end-to-end queue continuation and non-stall behavior.

## Implementation Tasks

### Task 1: Introduce Lane And Safety Types

**Files:**
- Create: `internal/integrationqueue/types.go`
- Test: `internal/integrationqueue/types_test.go`

**What to Do:**
Define lane enums (`safe_lane`, `code_lane`), lane-related metadata fields, and canonical safety/error reason constants used by the coordinator and status projection.

**Acceptance Criteria:**
- Lane constants and reason codes match the spec contract.
- Queue entry type supports lane metadata and diagnostics needed by status.
- Tests verify JSON shape and enum validation behavior.

**Dependencies:**
- None

### Task 2: Implement Changed-File Lane Classification

**Files:**
- Create: `internal/integrationqueue/classifier.go`
- Test: `internal/integrationqueue/classifier_test.go`

**What to Do:**
Implement file-based lane classification that maps metadata-only paths to `safe_lane` and all source/test/config/build/mixed changes to `code_lane` independent of command type.

**Acceptance Criteria:**
- Classification uses changed-file analysis only, not command allowlists.
- `debug`, `review`, and `retro` branches touching source resolve to `code_lane`.
- Mixed safe+code changes resolve to `code_lane`.

**Dependencies:**
- Task 1

### Task 3: Implement Hard Safety Violation Detection

**Files:**
- Create: `internal/integrationqueue/safety.go`
- Test: `internal/integrationqueue/safety_test.go`

**What to Do:**
Add validator logic that checks changed files against prohibited runtime/local-state artifacts and returns lane-violation diagnostics used for terminal transition.

**Acceptance Criteria:**
- Prohibited artifacts are detected and classified consistently.
- Safety failures map to `lane_violation` terminal path.
- Non-prohibited source changes are not blocked by safety validator.

**Dependencies:**
- Task 1
- Task 2

### Task 4: Wire Session Handoff To Lane Metadata

**Files:**
- Modify: `cmd/gromit/interactive_worktree.go`
- Modify: `internal/state/interactive_state.go` (if compatibility bridge is required)
- Test: `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
Capture changed-file metadata during session completion and persist queue-ready lane inputs instead of immediate merge ownership, while preserving migration-safe pending branch behavior.

**Acceptance Criteria:**
- Session completion persists enough metadata for deterministic lane classification.
- Branches are handed off to queue lifecycle, not directly merged by session command path.
- Migration bridge behavior remains explicit and covered.

**Dependencies:**
- Task 1
- Task 2

### Task 5: Build Lane-Aware Coordinator Execution

**Files:**
- Create/Modify: `internal/integrationqueue/coordinator.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/integrationqueue/coordinator_test.go`

**What to Do:**
Implement coordinator branch processing that classifies lane, enforces hard safety checks first, runs lane-specific integration flow, and persists terminal state transitions.

**Acceptance Criteria:**
- `code_lane` runs rebase + gates path.
- `safe_lane` runs minimal validation fast path with safety checks.
- Conflict and failure outcomes persist terminal state and do not crash coordinator loop.

**Dependencies:**
- Task 2
- Task 3
- Task 4

### Task 6: Enforce One-Retry Gate Policy For `code_lane`

**Files:**
- Create/Modify: `internal/integrationqueue/retry.go`
- Modify: `internal/integrationqueue/coordinator.go`
- Test: `internal/integrationqueue/coordinator_retry_test.go`

**What to Do:**
Add one automatic retry after fresh rebase on gate failure for `code_lane`; if second gate attempt fails, transition to `failed_gates` with diagnostics.

**Acceptance Criteria:**
- Exactly one retry occurs for eligible gate failures.
- Retry path performs a fresh rebase before re-running gates.
- Second failure transitions to `failed_gates` with preserved context.

**Dependencies:**
- Task 5

### Task 7: Add Lane-Aware Status Projection

**Files:**
- Modify: `internal/runner/print_status.go`
- Modify: `internal/runner/display/display.go` and related display types/tests
- Modify: `internal/pipeline/status.go` or status JSON seam
- Test: `internal/runner/status_test.go`
- Test: `cmd/gromit/status_test.go`

**What to Do:**
Extend status text and JSON views to include lane, queue state, FIFO position, retry attempt, and latest error reason in deterministic order.

**Acceptance Criteria:**
- Status output includes lane diagnostics for non-merged entries.
- Ordering remains deterministic and contract-stable.
- Invalid queue/safety metadata surfaces explicit diagnostics without panic.

**Dependencies:**
- Task 5
- Task 6

### Task 8: Throughput And Conflict Non-Stall Hardening

**Files:**
- Modify: `internal/integrationqueue/coordinator.go`
- Test: `internal/integrationqueue/coordinator_test.go`
- Test: `internal/runner/acceptance/*` (add/update targeted acceptance coverage)

**What to Do:**
Harden processing loop semantics so one branch in `conflict`, `failed_gates`, or `lane_violation` does not stall FIFO progression for remaining ready branches.

**Acceptance Criteria:**
- Queue continues processing subsequent ready entries after terminal failure on prior entry.
- Conflict path preserves branch for manual resolution.
- Terminal branch failures are fail-closed per entry and fail-open for queue progress.

**Dependencies:**
- Task 5
- Task 6
- Task 7

---

## Notes

- This plan intentionally reuses the emerging integration queue lifecycle direction from related specs/plans; it should not introduce a second merge/integration path.
- Hard safety policy should reference centralized rule semantics from `.gromit/RULES.md` to avoid drift between docs and enforcement.
- Keep lane classification deterministic and explainable in status output to simplify manual conflict recovery.
