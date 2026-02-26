---
id: stats-merge-regression-tests
source_spec: stats-merge-regression-tests
created: 2026-02-26
decomposed: false
---

# Stats Merge Regression Tests Implementation Plan

**Goal:** Extend orchestrator global-stats merge coverage to catch regressions for empty initial state, multi-model independence, and zero-value additive semantics.

**Architecture:** Keep production merge behavior unchanged and expand `TestOrchestrator_MergesGlobalStatsPreservingExistingData` into subtests that each construct deterministic in-memory fixtures, execute one orchestrator run, and assert merged global stats outcomes.

**Tech Stack:** Go (`testing`, `t.Run`), existing orchestrator test fakes, `internal/logger` stats models.

**Spec:** `.gromit/specs/stats-merge-regression-tests.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Augment the existing orchestrator regression test with table-like subtests under one parent test to validate merge invariants against the current additive merge implementation in `logger.UpdateGlobalStats`.

**Key Components:**
1. **`internal/runner/orchestrator_test.go` parent test**: Continues to own end-to-end orchestration of one run that triggers `mergeGlobalStats()`.
2. **Subtest scenarios (`t.Run`)**: Isolated setup/assertion blocks for each edge case so failures point to a specific merge invariant.
3. **`logger.UpdateGlobalStats` integration (read-only)**: Existing additive (`+=`) field semantics remain the source of truth; tests lock this behavior to prevent accidental overwrite logic changes.

**Integration Points:**
- Reuse existing helper `writeOrchestratorTestLogFile` to generate per-run JSONL inputs consumed by `logger.ReadRunModelStats`.
- Continue invoking real `Orchestrator.Run(...)` so test coverage includes the production call chain:
  `Orchestrator.Run -> mergeGlobalStats -> ReadRunModelStats -> UpdateGlobalStats -> ReadGlobalStats`.
- Modify only `internal/runner/orchestrator_test.go`; no production code changes expected.

**Data Flow:**
Each subtest creates temp `logsDir` + `statsPath`, seeds optional existing global stats JSON, writes run logs for a deterministic `runID`, runs orchestrator with `GlobalStatsPath` configured, then reads merged stats and validates model-specific counters/cost/escalation totals.

**Files to Modify:**
- `internal/runner/orchestrator_test.go` - Expand `TestOrchestrator_MergesGlobalStatsPreservingExistingData` into targeted subtests and shared setup helpers inside the test.

**Files to Create:**
- None.

**Tradeoffs:**
- **End-to-end orchestrator test vs direct unit of `UpdateGlobalStats`**: Chose orchestrator-level coverage to prevent regressions in wiring/config preconditions, not just math logic.
- **Single parent test with subtests vs separate top-level tests**: Chose subtests for grouped intent and shared fixtures while preserving focused failure output.
- **Document additive zero semantics vs overwrite semantics**: Chose to enforce additive behavior because current implementation accumulates all numeric fields and zero incoming values should not reduce existing totals.

## Test Strategy

## Test Strategy Proposal

**Test Levels:**
1. **Unit-style orchestrator regression subtests**: Validate merge behavior through one deterministic orchestrator run per scenario.
2. **Integration coverage via real logger merge path**: Use actual JSONL/stat file parsing and merge code without mocks for stats math.
3. **Manual testing**: Not required; deterministic automated tests are sufficient.

**Key Test Cases:**
- **Empty initial stats**: Start from no pre-existing model data (missing or empty stats file) and verify first merge populates all expected fields for the run model.
- **Multiple model entries**: Seed existing stats for two models, merge a run including updates/new model, verify each model accumulates independently and no cross-model contamination occurs.
- **Zero-value fields**: Seed non-zero existing values, merge run stats with zero in selected fields, verify accumulated values remain unchanged for those fields while non-zero incoming fields still add.

**Mocking Strategy:**
- Keep stage mocks/fakes minimal (`fakeStage`, `GetBead` no-op) only to execute `Orchestrator.Run` quickly.
- Do not mock `logger.ReadRunModelStats` or `logger.UpdateGlobalStats`; use real implementations to validate production semantics.

**Coverage Goals:**
- Lock expected additive merge semantics for iterations/successes/failures/cost/escalations.
- Verify model-key isolation when multiple model names exist.
- Ensure timestamp update behavior remains intact after merge.

**Test Organization:**
- All additions stay in `internal/runner/orchestrator_test.go` under `TestOrchestrator_MergesGlobalStatsPreservingExistingData` using `t.Run("...")` case names.
- Use deterministic values and avoid sleeps/timing-sensitive assertions.

## Implementation Tasks

### Task 1: Refactor Existing Merge Regression Test Into Scenario Subtests

**Files:**
- Modify: `internal/runner/orchestrator_test.go`

**What to Do:**
Restructure `TestOrchestrator_MergesGlobalStatsPreservingExistingData` into subtests with shared local setup helpers for temp dirs, optional seeded global stats, run log creation, and orchestrator invocation. Preserve the current baseline scenario as one explicit subtest.

**Acceptance Criteria:**
- Existing baseline assertions are preserved under a named `t.Run` scenario.
- Shared setup avoids duplicated orchestrator bootstrap logic across scenarios.
- Subtests remain parallel-safe and deterministic.

**Dependencies:**
- None.

**Notes:**
- Keep helper scope local to this test block unless reuse outside this test clearly improves readability.

### Task 2: Add Edge-Case Subtests for Empty Initial Stats, Multi-Model Independence, and Zero-Value Additive Semantics

**Files:**
- Modify: `internal/runner/orchestrator_test.go`

**What to Do:**
Add three new `t.Run` scenarios implementing the spec edge cases and asserting per-field outcomes for all relevant numeric stats plus timestamp update behavior where applicable.

**Acceptance Criteria:**
- Empty-initial-stats scenario verifies first merge populates expected model entry and values.
- Multiple-model scenario verifies independent accumulation for each model and unchanged unrelated model fields.
- Zero-value scenario explicitly documents and enforces additive semantics (`+=`): zero incoming values do not overwrite/reduce existing non-zero totals.

**Dependencies:**
- Task 1 (shared subtest structure and setup helpers).

**Notes:**
- Use exact numeric assertions with clear failure messages describing existing + incoming = expected.

### Task 3: Validate Runner Package Regression Coverage

**Files:**
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Run targeted tests for runner package stats merge path and ensure no flakiness or timing dependencies were introduced.

**Acceptance Criteria:**
- `go test ./internal/runner/...` passes.
- New subtests pass consistently across repeated local runs (at least one rerun of target test).
- No production code changes are required to satisfy the regression cases.

**Dependencies:**
- Task 2.

**Notes:**
- If runtime is high, start with `-run TestOrchestrator_MergesGlobalStatsPreservingExistingData` before full package sweep.

---

## Notes

- Current merge semantics in `internal/logger/globalstats.go` are strictly additive for all numeric fields; this plan codifies that contract in orchestrator-facing regression tests.
- Scope is intentionally limited to test coverage hardening in one file (`internal/runner/orchestrator_test.go`) per spec.
