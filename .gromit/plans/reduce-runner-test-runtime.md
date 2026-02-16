---
id: reduce-runner-test-runtime
source_spec: reduce-runner-test-runtime
created: 2026-02-16
decomposed: false
---

# Reduce `internal/runner` Test Runtime — Implementation Plan

**Goal:** Reduce default `go test ./internal/runner/... -count=1` runtime by 30%+ (from ~24.5s baseline) by moving heavy integration tests behind acceptance tags, replacing sleeps with deterministic patterns, and tightening mock test fixtures.

**Architecture:** Three-pillar approach — (1) relocate cross-component choreography tests to `//go:build acceptance`, (2) eliminate `time.Sleep` calls in default-suite tests, (3) replace full `r.Run()` calls with focused `r.processBead()` in mock tests that only assert single-bead behavior.

**Tech Stack:** Go, build tags (`//go:build acceptance`), existing mock infrastructure (`interfaces_test.go`)

**Spec:** `.gromit/specs/reduce-runner-test-runtime.md`

---

## Architecture

### Pillar 1: Acceptance Tag Migration (~7-8s savings)

Move tests that verify cross-component behavior (PID lifecycle, multi-bead loops, broad Status I/O) to `//go:build acceptance` files. These tests are valuable regression coverage but don't belong in the fast inner-loop suite.

**Tests to relocate:**

| Test | File | Time | Reason |
|------|------|------|--------|
| `TestRunnerStatusWithLiveRun` (3 subtests) | status_test.go | 3.0s | PID lifecycle probing, stale file cleanup |
| `TestRunner_Status_Integration_ActiveRun` | status_test.go | 0.97s | Multi-file I/O, full Status() parse |
| `TestRunner_Status_Integration_IdleWithHistory` | status_test.go | 0.82s | Multi-file I/O + Status() |
| `TestRunner_Status_LivePID` | status_test.go | 0.92s | Real PID detection |
| `TestRunner_Status_DeadPID` | status_test.go | 0.71s | PID detection |
| `TestScopedRun_FullLoopWithLabelFilters` | runner_test.go | 0.9s | Full 4-bead loop |
| `TestRunner_UsesLabelFiltersInLoop` | runner_test.go | 1.21s | Full loop with label routing |

**Destination files:**
- `internal/runner/status_acceptance_test.go` — PID/Status integration tests
- `internal/runner/loop_acceptance_test.go` — full-loop tests

### Pillar 2: Sleep Replacement (~3s savings)

| Location | Sleep | Replacement |
|----------|-------|-------------|
| `globalstats_integration_test.go:168` | 2s | Use explicit timestamp offset in log filenames or move test to acceptance |
| `status_test.go:132,568,635` | 100ms x3 | Direct state setup — write desired state files instead of waiting for async writes |
| `process_test.go:1463` | 50ms | Channel-based synchronization |
| `runner_test.go:487,533,574,605,629,635,663` | 50-200ms x7 | These heartbeat tests already use custom configs with small delays — keep as-is unless measurably slow; replace time.Sleep-then-check with channel-signaled assertions where feasible |

### Pillar 3: Focused Test Rewrites (~2s savings)

`TestRunWithMocks_*` tests take 0.7-0.9s each despite using mocks. Several only assert single-bead behavior but invoke the full `r.Run()` loop. Replace with direct `r.processBead()` calls to eliminate loop overhead (ticker creation, iteration bookkeeping, status file writes).

## Test Strategy

**Default suite (`go test`):** Mock-based unit tests for wiring, logic branching, single-function behavior. Each test <100ms target.

**Acceptance suite (`go test -tags acceptance`):** Relocated integration tests preserving full behavioral coverage for PID lifecycle, multi-bead loops, file-based Status parsing, and global stats aggregation.

**Verification:** No test deleted. Every behavior present in either default or acceptance suite. Runtime measured before/after with `-json` output.

**Mocking:** Existing infrastructure in `interfaces_test.go` is sufficient. No new mock types. For sleep replacement, use channel signaling rather than introducing clock abstractions.

## Implementation Tasks

### Task 1: Move status/PID integration tests to acceptance

**Files:**
- Modify: `internal/runner/status_test.go`
- Create: `internal/runner/status_acceptance_test.go`

**What to Do:**
Move `TestRunnerStatusWithLiveRun` (and its 3 subtests), `TestRunner_Status_Integration_ActiveRun`, `TestRunner_Status_Integration_IdleWithHistory`, `TestRunner_Status_LivePID`, and `TestRunner_Status_DeadPID` from `status_test.go` to a new `status_acceptance_test.go` with `//go:build acceptance` tag. Move any helper functions these tests depend on — if helpers are shared with remaining default tests, keep them in `status_test.go` and duplicate only what's needed. Add a minimal mock-only smoke test in `status_test.go` that verifies `Status()` returns the expected structure without file I/O.

**Acceptance Criteria:**
- `go test ./internal/runner/...` passes without the relocated tests
- `go test -tags acceptance ./internal/runner/...` passes with them
- A mock-only Status smoke test exists in the default suite

**Dependencies:** None

### Task 2: Move full-loop tests to acceptance

**Files:**
- Modify: `internal/runner/runner_test.go`
- Create: `internal/runner/loop_acceptance_test.go`

**What to Do:**
Move `TestScopedRun_FullLoopWithLabelFilters` and `TestRunner_UsesLabelFiltersInLoop` from `runner_test.go` to a new `loop_acceptance_test.go` with `//go:build acceptance` tag. These tests exercise multi-bead `r.Run()` loops with label filtering — they validate orchestration correctness but are too heavy for the default suite. Move any helper functions (e.g., `makeReadyFromQueue`) if only used by these tests, otherwise keep shared.

**Acceptance Criteria:**
- `go test ./internal/runner/...` passes without the relocated tests
- `go test -tags acceptance ./internal/runner/...` passes with them

**Dependencies:** None

### Task 3: Replace time.Sleep calls in default suite tests

**Files:**
- Modify: `internal/runner/globalstats_integration_test.go`
- Modify: `internal/runner/status_test.go`
- Modify: `internal/runner/process_test.go`

**What to Do:**
1. `globalstats_integration_test.go:168` — The 2s sleep ensures different timestamps between two runs. Replace by injecting an explicit time offset: either mock the timestamp source, use a sub-second file rename, or change the test to compare run IDs instead of timestamps. If no clean deterministic fix exists, move this test to acceptance.
2. `status_test.go` — After Task 1 removes the integration tests, check which sleeps remain. For any remaining 100ms sleeps, replace with direct file/state writes instead of waiting for async operations.
3. `process_test.go:1463` — Replace the 50ms sleep with channel-based synchronization (send a signal when the async operation completes, select on the signal with a test timeout).

**Acceptance Criteria:**
- No `time.Sleep` calls remain in default-suite test files (excluding heartbeat tests which use intentionally small custom delays)
- `go test ./internal/runner/...` passes

**Dependencies:** Task 1 (determines which status_test.go sleeps remain)

### Task 4: Tighten RunWithMocks tests to use processBead

**Files:**
- Modify: `internal/runner/runner_test.go`

**What to Do:**
Audit `TestRunWithMocks_*` tests (PrecheckError, ClosesBeadOnSuccess, PrecheckVerificationError, PrecheckVerificationRejects, ConsecutiveSkipCounterResetsAfterRealBuild) and `TestIterationLogWithMocks`. For tests that only assert single-bead behavior, replace the `r.Run()` call with a direct `r.processBead()` call to eliminate loop overhead (ticker creation, status file writes, iteration bookkeeping). Keep `r.Run()` only in tests that specifically verify loop-level behavior (e.g., consecutive skip counter which needs multiple iterations). Extract a shared setup helper if 3+ tests share identical runner construction.

**Acceptance Criteria:**
- Tests that only assert single-bead behavior call `processBead()` instead of `Run()`
- All modified tests pass
- No behavioral coverage is lost

**Dependencies:** Task 2 (determines which runner_test.go tests remain)

**Notes:** `processBead()` is an unexported method on `*Runner` — since tests are in the same package, this is fine. Check whether mock setup differs (processBead needs a `beadContext` vs Run needs mock `ReadyFn`). If processBead requires significant setup changes, the overhead savings may not justify the rewrite — measure first.

### Task 5: Verify runtime reduction and document hotspot baseline

**Files:**
- Create or update: `.gromit/specs/reduce-runner-test-runtime.md` (append baseline snapshot)

**What to Do:**
1. Run `go test ./internal/runner/... -count=1 -json`, parse top-10 slow tests, compare with baseline.
2. Confirm 30%+ reduction from 24.5s baseline.
3. Run `go test -tags acceptance ./internal/runner/... -count=1` and confirm all relocated tests pass.
4. Run `go test ./internal/runner/... -count=1` (without -json) to confirm clean pass.
5. Append a "Post-optimization Baseline" section to the spec with the new top-10 slow tests snapshot.

**Acceptance Criteria:**
- Default suite runtime is <17s (30%+ reduction from 24.5s)
- Acceptance suite passes with all relocated tests
- Hotspot baseline is documented in the spec

**Dependencies:** Tasks 1-4

---

## Notes

- **Heartbeat tests (runner_test.go lines 430-670):** These already use custom `heartbeatConfig` with small delays (10-50ms). The `time.Sleep` calls in these tests are intentional timing windows for stall detection verification. Leave them alone unless they show up as measurably slow after other optimizations — they're deterministic enough at current delay values.
- **Relationship to existing beads:** The `spec:acceptance-runtime-optimization` beads (gromit-gdsr, gromit-kk13, etc.) optimize the *acceptance* suite by reclassifying tests. This plan optimizes the *default* suite by moving heavy tests *to* acceptance. The two efforts are complementary.
- **gromit-egpb overlap:** That bead deletes 13 disabled (t.Skip) integration tests in `integration_test.go`. Those tests don't contribute to runtime since they skip, but the cleanup is independent of this plan.
- **Risk:** Moving tests to acceptance means they won't run in the default `go test` invocation. CI must run both `go test ./...` and `go test -tags acceptance ./...` to maintain full coverage. Verify CI configuration includes acceptance runs.
