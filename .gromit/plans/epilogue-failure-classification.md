---
created: 2026-02-27T00:00:00Z
decomposed: true
decomposed_at: "2026-02-27T02:39:10Z"
id: epilogue-failure-classification
source_spec: epilogue-failure-classification
---

# Epilogue Failure Classification Implementation Plan

**Goal:** Ensure epilogue lifecycle failures (`Close`, `Sync`) are authoritative iteration failures, preventing false-success state, logs, and events.

**Architecture:** Keep lifecycle operations in `epilogue` but make them produce explicit success/failure classification that `orchestrator` consumes before emitting success semantics. Preserve existing warning-only behavior for non-critical epilogue operations.

**Tech Stack:** Go (`internal/pipeline`, `internal/runner`, table-driven unit tests, acceptance-style event assertions where applicable)

**Spec:** `.gromit/specs/epilogue-failure-classification.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Promote epilogue lifecycle (`Close`, `Sync`) from warning-only side effects to authoritative completion signals. The epilogue stage will classify lifecycle outcome, and orchestrator success-path semantics will be gated on that classification.

**Key Components:**
1. **`internal/pipeline/epilogue` classification seam**: Detect lifecycle failure during success-path epilogue work and surface a machine-readable outcome to callers.
2. **`internal/runner/orchestrator` success gate**: Only emit "completed successfully" semantics, success-marked iteration state, and success-only follow-ons when epilogue lifecycle outcome is successful.
3. **Iteration log alignment (`logger.IterationLog` usage path)**: Ensure persisted iteration result reflects final lifecycle truth when close/sync fails after build+validate pass.
4. **Event parity guardrails**: Keep run-path event output consistent with final classification (no success event/log semantics for lifecycle-failed iteration).

**Integration Points:**
- `epilogue.Run(...)` currently logs close/sync failures as warnings but always returns `Proceed`; this is the source of false-success continuation.
- `orchestrator.Run(...)` currently logs `"completed successfully"` and triggers success follow-up unconditionally after `runEpilogue(..., true)`.
- Existing optional epilogue actions (status write, thorough review, worktree cleanup warnings, between-iteration command warnings) remain warning-only and non-fatal.

**Data Flow:**
1. Orchestrator enters success path after Build+Validate pass.
2. Epilogue executes lifecycle operations.
3. If `Close` or `Sync` fails, epilogue marks lifecycle classification as failed and returns that classification.
4. Orchestrator consumes epilogue classification:
- does not emit/log success semantics for that iteration;
- records iteration outcome as failed in persisted run data;
- avoids success-only actions for that iteration (for example spec-merge trigger).
5. Non-critical epilogue warnings continue to be emitted without flipping classification.

**Files to Modify:**
- `internal/pipeline/epilogue/epilogue.go` - add lifecycle failure classification output and maintain warning logging behavior.
- `internal/runner/orchestrator.go` - gate success semantics/follow-ons on epilogue lifecycle classification.
- `internal/pipeline/stage.go` - add explicit output field for epilogue lifecycle classification if needed for clean stage-to-orchestrator signaling.
- `internal/pipeline/epilogue/epilogue_test.go` - add regression tests for close/sync failure classification and non-critical warning non-fatality.
- `internal/runner/orchestrator_test.go` - add regression tests proving no false success semantics when lifecycle fails.

**Files to Create:**
- None expected.

**Tradeoffs:**
- **Explicit classification field vs implicit pointer mutation:** Chose explicit stage output classification for clarity and lower coupling to side effects on `Input.Result`.
- **Keep non-critical warnings non-fatal:** Preserves current operational resilience and scope discipline while fixing lifecycle truth.
- **Gate success semantics in orchestrator (not only epilogue):** Ensures all user-visible/run-visible success signals are centrally consistent.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit Tests (`epilogue`)**: Verify `Close` and `Sync` failure paths reclassify iteration outcome while preserving warning output and non-critical warning behavior.
2. **Unit Tests (`orchestrator`)**: Verify success-path logging/state/events are suppressed or switched to failure semantics when epilogue lifecycle classification is failed.
3. **Targeted acceptance/event checks**: If run-path typed event emission is active in this codepath, verify lifecycle-failed iterations never emit success events.

**Key Test Cases:**
- `Close` failure on success-path input yields failure classification.
- `Sync` failure on success-path input yields failure classification.
- Orchestrator does not emit `"completed successfully"` when lifecycle classification fails.
- Persisted iteration result is failure (not success) when lifecycle close/sync fails.
- Success-only follow-on behavior (e.g., spec merge trigger) is not executed for lifecycle-failed iteration.
- Existing non-critical warnings (status write, thorough review, worktree cleanup, between-iteration command) still do not hard-fail iteration by themselves.

**Mocking Strategy:**
- Use existing fake `BeadLifecycle` in `epilogue_test.go` with targeted `closeFn`/`syncFn` errors.
- Use `fakeStage` and controlled epilogue outputs in `orchestrator_test.go` to isolate classification logic.
- Use output buffer assertions for human-facing success/failure text regressions.
- Reuse existing emitter test seams where run-path event behavior is already under test.

**Coverage Goals:**
- Critical lifecycle failure-to-classification path is fully covered.
- No regression of non-critical warning semantics.
- No divergent truth across logs, iteration data, and events.

**Test Organization:**
- Extend existing test files:
- `internal/pipeline/epilogue/epilogue_test.go`
- `internal/runner/orchestrator_test.go`
- Add focused table-driven cases near existing success-path and warning tests.

## Implementation Tasks

### Task 1: Add explicit epilogue lifecycle classification output

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Modify: `internal/pipeline/stage.go` (if explicit output signal is introduced)
- Test: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
Implement explicit lifecycle classification in epilogue success path. `Close`/`Sync` failures must mark lifecycle as failed while still emitting existing warning lines. Keep stage decision and non-critical warning behaviors unchanged.

**Acceptance Criteria:**
- On `BuildSucceeded=true`, `Close` error marks lifecycle classification failed.
- On `BuildSucceeded=true`, `Sync` error marks lifecycle classification failed.
- Non-critical epilogue warnings do not mark lifecycle classification failed.

**Dependencies:**
- None.

**Notes:**
- Preserve warning text compatibility where possible to avoid unrelated snapshot churn.

### Task 2: Gate orchestrator success semantics on epilogue lifecycle outcome

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Update orchestrator success-path completion logic to respect epilogue lifecycle classification. Only emit success semantics and success-only post-epilogue actions when lifecycle succeeded; otherwise classify iteration as failed.

**Acceptance Criteria:**
- No `"completed successfully"` log semantics when epilogue lifecycle fails.
- Iteration final classification is failure when lifecycle fails after build/validate pass.
- Success-only follow-on path (including spec merge trigger) is skipped on lifecycle failure.

**Dependencies:**
- Task 1.

**Notes:**
- Keep existing behavior unchanged for genuine success-path iterations where lifecycle succeeds.

### Task 3: Align persisted iteration data with final lifecycle truth

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Ensure persisted `IterationLog` outcome reflects final post-epilogue truth. Lifecycle failure must not leave success-marked iteration records.

**Acceptance Criteria:**
- Lifecycle-failed success-path iterations are persisted as `Success=false`.
- Failure metadata is present enough to diagnose lifecycle failure origin.
- Existing success metrics remain unchanged for true success iterations.

**Dependencies:**
- Task 1.
- Task 2.

**Notes:**
- Avoid over-broad schema changes unless required; prefer minimal, backward-compatible log adjustments.

### Task 4: Add regression coverage for event/log parity and warning non-regression

**Files:**
- Modify: `internal/runner/orchestrator_test.go`
- Modify: `internal/pipeline/epilogue/epilogue_test.go`
- Modify: `internal/runner/acceptance/event_ordering_acceptance_test.go` (only if run-path event emission for this scenario exists)

**What to Do:**
Add regression tests proving no false success semantics across logs/events and preserving non-critical warning behavior for unrelated epilogue operations.

**Acceptance Criteria:**
- Close-failure and sync-failure regressions are covered end-to-end through orchestrator path.
- No success event/log semantics emitted for lifecycle-failed iterations.
- Optional/non-critical epilogue warning behavior remains unchanged.

**Dependencies:**
- Task 2.
- Task 3.

**Notes:**
- If typed lifecycle events are not emitted in this path today, assert log-level parity now and document event assertion hook for later emitter wiring.

### Task 5: Validate and harden with focused quality gates

**Files:**
- Test targets only.

**What to Do:**
Run targeted package tests for epilogue and orchestrator, then run broader runner/pipeline suites if touched behavior warrants it.

**Acceptance Criteria:**
- `go test ./internal/pipeline/epilogue ./internal/runner -run Epilogue|Orchestrator` passes.
- Full affected package tests pass for touched files.
- No unrelated regression in warning-only optional epilogue features.

**Dependencies:**
- Task 1.
- Task 2.
- Task 3.
- Task 4.

**Notes:**
- Keep feedback loop tight first with targeted tests before broader runs.

---

## Notes

- The spec references `.gromit/reports/debug-20260227-021749.md`, but that file was not present in this workspace snapshot; plan decisions are based on current implementation and tests.
- Current orchestrator run path appears log-driven rather than fully typed-event-driven for lifecycle completion; this plan preserves event-system compatibility by enforcing no contradictory success semantics where outputs exist.
