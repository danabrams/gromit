---
id: runner-test-runtime-reduction
source_spec: runner-test-runtime-reduction
created: 2026-02-16
decomposed: false
---

# Reduce internal/runner Test Runtime Implementation Plan

**Goal:** Reduce default `internal/runner` test runtime by ≥25% from ~23.8s by moving full-pipeline tests behind build tags and injecting fake dependencies for OS-level operations.

**Architecture:** Add injectable `processChecker` function field to Runner for PID liveness, move 4 full-pipeline tests behind `//go:build acceptance`, add equivalent fast unit tests targeting the same decisions through lighter seams.

**Tech Stack:** Go, build tags, existing mock infrastructure

**Spec:** `.gromit/specs/runner-test-runtime-reduction.md`

---

## Architecture

**Overview:**
Reduce default test runtime by (a) moving full-pipeline integration tests behind `//go:build acceptance`, (b) injecting a fake liveness checker to eliminate real OS process probing in Status tests, and (c) adding focused unit tests for migrated behaviors.

**Key Components:**

1. **`processChecker` function field on Runner** — Injectable liveness function (`func(int) bool`) defaulting to `IsProcessAlive`. Status tests inject a fake that returns instantly.

2. **Tagged acceptance test files** — Full-pipeline tests moved behind `//go:build acceptance` to new files, preserving orchestration regression coverage for CI.

3. **Fast unit replacement tests** — Focused tests validating the same decisions through lighter seams: direct function calls, minimal Runner construction, targeted method calls without full pipeline.

4. **Timing helper** — Documented `go test -json` command for per-test runtime profiling.

**Integration Points:**
- `internal/runner/runner.go` — Add `processChecker` field, wire default in `NewRunnerWithDeps`
- `internal/runner/lifecycle.go` — Replace direct `IsProcessAlive()` call with `r.processChecker()`
- `internal/runner/status.go` — `IsProcessAlive` remains as default implementation

**Files to Modify:**
- `internal/runner/runner.go` — Add processChecker field and default wiring
- `internal/runner/lifecycle.go` — Use injected processChecker
- `internal/runner/runner_test.go` — Remove moved tests, add fast replacements, inject fake processChecker
- `internal/runner/status_test.go` — Remove moved test, add fast replacements
- `internal/runner/interfaces_test.go` — Inject fake processChecker in Status tests

**Files to Create:**
- `internal/runner/runner_pipeline_acceptance_test.go` — Moved full-pipeline tests
- `internal/runner/status_integration_acceptance_test.go` — Moved status integration test

**Tradeoffs:**
- **Function field over interface**: `func(int) bool` follows existing `cmdRunner` pattern, avoids interface proliferation
- **Acceptance tag over integration**: Consistent with majority of existing tagged tests (19 acceptance vs 5 integration)
- **Keep tagged copies**: Preserves orchestration regression coverage for CI `-tags=acceptance` runs

**Estimated savings:**

| Test | Current (s) | After |
|------|-------------|-------|
| TestATDDSkippedForTestOnlyBead | ~1.4 | → acceptance (0s default) |
| TestTDDPromptSelection | ~1.0 | → acceptance (0s default) |
| TestScopedRun_FullLoopWithLabelFilters | ~0.9 | → acceptance (0s default) |
| TestRunner_Status_Integration_IdleWithHistory | ~1.1 | → acceptance (0s default) |
| TestRunnerStatusWithLiveRun | ~2.8 | ~0.5 (fake PID) |
| TestStatusWithMocks pair | ~1.7 | ~0.5 (fake PID) |
| **Total saved** | **~8.9** | **~7.9s → ~33% reduction** |

## Test Strategy

**Unit Tests (default run):**
- ATDD skip decision: Focused test verifying skip log and render suppression without full processBead
- TDD prompt routing: Focused test verifying RenderTDDBuild vs RenderBuild selection without full pipeline
- Status with fake PID: Existing tests updated with injected processChecker
- Label filtering: Already covered by label_filter_test.go

**Acceptance Tests (tagged):**
- Full-pipeline ATDD skip through processBead (moved from runner_test.go)
- Full-pipeline TDD prompt routing through processBead (moved from runner_test.go)
- Full multi-iteration loop with label filtering through Run (moved from runner_test.go)
- Full status integration with history files (moved from status_test.go)

**Coverage Goals:**
- Every behavioral assertion from moved tests has a fast unit equivalent
- No net regression in assertion count
- `go test ./internal/runner -count=1` achieves ≥25% reduction
- `go test -tags=acceptance ./internal/runner -count=1` passes all moved tests

## Implementation Tasks

### Task 1: Add processChecker field to Runner and wire into Status

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/lifecycle.go`
- Test: `internal/runner/status_test.go`

**What to Do:**
Add a `processChecker func(int) bool` field to the `Runner` struct. In `NewRunnerWithDeps`, default it to `IsProcessAlive`. In `lifecycle.go`, replace the direct `IsProcessAlive(status.PID)` call with `r.processChecker(status.PID)`. Add a unit test verifying fake processChecker controls alive/dead PID behavior in Status.

**Acceptance Criteria:**
- Runner has processChecker field defaulting to IsProcessAlive
- Status() uses r.processChecker instead of direct IsProcessAlive call
- New test verifies fake processChecker controls alive/dead PID behavior

**Dependencies:** None

### Task 2: Move full-pipeline tests behind acceptance tag

**Files:**
- Modify: `internal/runner/runner_test.go`
- Create: `internal/runner/runner_pipeline_acceptance_test.go`
- Modify: `internal/runner/status_test.go`
- Create: `internal/runner/status_integration_acceptance_test.go`

**What to Do:**
Move `TestATDDSkippedForTestOnlyBead`, `TestTDDPromptSelection`, `TestScopedRun_FullLoopWithLabelFilters` from `runner_test.go` to new `runner_pipeline_acceptance_test.go` with `//go:build acceptance`. Move `TestRunner_Status_Integration_IdleWithHistory` from `status_test.go` to `status_integration_acceptance_test.go` with `//go:build acceptance`. Ensure moved tests compile and pass with `-tags=acceptance`.

**Acceptance Criteria:**
- Four tests removed from default run files
- `go test ./internal/runner -count=1` no longer runs the moved tests
- `go test -tags=acceptance ./internal/runner -count=1` passes all moved tests

**Dependencies:** None

### Task 3: Add fast unit tests for ATDD skip and TDD prompt routing

**Files:**
- Modify: `internal/runner/runner_test.go`

**What to Do:**
Add focused unit tests verifying ATDD skip and TDD prompt selection decisions without full `processBead()` pipeline. For ATDD: construct minimal Runner, track `RenderAcceptanceTests` calls via mock renderer, verify skip log for test-only titles. For TDD: verify `RenderTDDBuild` called when TDD active, `RenderBuild` when inactive. Use minimal config and mock setup.

**Acceptance Criteria:**
- ATDD skip test covers: test-only skips, regular doesn't skip, disabled doesn't log
- TDD routing test covers: global TDD, label TDD, no TDD
- Tests run in <0.5s combined

**Dependencies:** Task 2

### Task 4: Inject fake processChecker into existing Status tests

**Files:**
- Modify: `internal/runner/runner_test.go`
- Modify: `internal/runner/interfaces_test.go`

**What to Do:**
Update `TestRunnerStatusWithLiveRun` to inject fake processChecker returning controlled alive/dead results. Update `TestStatusWithMocks` and `TestStatusWithMocks_NoWork` to inject fake processChecker. Eliminates real `os.FindProcess`/`syscall.Signal` overhead.

**Acceptance Criteria:**
- TestRunnerStatusWithLiveRun uses fake processChecker, no real PID probing
- TestStatusWithMocks pair uses fake processChecker
- All existing assertions still pass

**Dependencies:** Task 1

### Task 5: Add runtime profiling target and verify improvement

**Files:**
- Modify: `Makefile` (add target or document command)

**What to Do:**
Add a `test-profile` target or documented command using `go test -json ./internal/runner -count=1` piped through `jq` to list per-test elapsed times sorted descending. Verify ≥25% runtime reduction from ~23.8s baseline.

**Acceptance Criteria:**
- Documented command outputs per-test timing sorted by duration
- Measured default runtime is at least 25% lower than 23.8s baseline

**Dependencies:** Tasks 1-4

---

## Notes

- `test_only_atdd_skip_test.go` already tests ATDD skip via `processBead()` — overlaps with moved `TestATDDSkippedForTestOnlyBead`. Consider consolidating during Task 2 (move both to acceptance, or keep the smaller one in default run since it's only 3 subtests vs 5).
- The `contains` helper function at runner_test.go:2607 may need to stay if other tests use it. Check before removing.
- `TestRunnerStatusWithLiveRun` constructs Runner directly (not via `NewRunnerWithDeps`), so the processChecker injection requires setting the field on the struct literal. This is straightforward since the test already uses `r := &Runner{...}`.
- Existing `TestProcessBead_SkipsATDDForTestOnlyBead` in `test_only_atdd_skip_test.go` also goes through full `processBead()` — it's a candidate for acceptance tagging in a follow-up if further runtime reduction is needed.
