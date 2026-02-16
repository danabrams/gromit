---
id: reduce-runner-test-runtime
source_ideas: []
created: 2026-02-16
---

# Reduce `internal/runner` Test Runtime

## Specification

`go test ./internal/runner/... -count=1` currently spends most wall-clock time in the root `internal/runner` package (~22.2s of ~22.8s total). The test output shows a small set of long-running tests (roughly 0.7s-2.3s each) dominating runtime, especially status/live-run and full-loop behavior tests.

This spec reduces default test runtime while preserving regression coverage by:

1. Identifying and tracking the top slow tests in `internal/runner`.
2. Moving heavy end-to-end style coverage behind `acceptance` build tags where appropriate.
3. Replacing avoidable integration-style setup in default tests with tighter fakes, deterministic clocks, and narrower fixtures.

### 1. Baseline and Hotspot Tracking

Add a repeatable profiling step for runner tests:

- Command: `go test ./internal/runner/... -count=1 -json`
- Parse and sort test-level elapsed time.
- Keep a short "top 10 slow tests" snapshot in a checked-in doc (`.gromit/specs` companion notes or comments in this spec) so runtime changes are visible over time.

Initial hotspots from baseline include:

- `TestRunnerStatusWithLiveRun`
- `TestATDDSkippedForTestOnlyBead`
- `TestTDDPromptSelection`
- `TestRunner_Status_Integration_*` variants
- `TestScopedRun_FullLoopWithLabelFilters`
- `TestProcessBead_SkipsATDDForTestOnlyBead`

### 2. Split Heavy Integration Paths Behind `acceptance`

Tests that validate behavior through broad loop execution, filesystem state, pid/status probing, or multi-phase orchestration should be tagged `//go:build acceptance` when they are not required for fast inner-loop correctness.

Scope:

- Focus first on slow status/run-loop tests and broad multi-phase flow tests in `internal/runner`.
- Keep a minimal smoke subset in default unit tests for wiring/logic correctness.
- Preserve full behavior coverage in `go test -tags acceptance ./internal/runner/...`.

Guideline:

- Default suite (`go test`) should prioritize deterministic unit-level confidence and fast feedback.
- Acceptance suite should own cross-component choreography, process-like lifecycle, and full-loop scenario coverage.

### 3. Tighten Fakes and Time Control in Default Tests

For tests that remain in default suite:

- Replace sleeps/time-based waiting with injected clocks or explicit signaling hooks.
- Replace broad fixture setup with focused in-memory fakes where possible.
- Avoid full runner loops when a single function-level behavior check is sufficient.
- Reuse helper constructors to reduce repeated expensive setup and improve readability.

Targets:

- Status-related tests currently simulating live/stale PID/file behavior.
- Prompt/methodology tests that trigger broader orchestration than needed for the asserted behavior.
- Loop tests that can be reduced to table-driven unit scenarios around selection/filter logic.

## Acceptance Criteria

- A documented hotspot baseline exists for `internal/runner` test runtime using `-json` output.
- The top slow integration-style tests are either:
  - moved to `acceptance`-tagged files, or
  - rewritten with tighter deterministic fakes so elapsed time materially drops.
- Default suite runtime for `go test ./internal/runner/... -count=1` decreases measurably (target: at least 30% reduction from baseline on the same machine).
- `go test ./internal/runner/...` passes.
- `go test -tags acceptance ./internal/runner/...` passes.
- Regression coverage for run-loop/status behavior remains present in either default or acceptance suite (no behavior is dropped entirely).

## Decisions

1. **Preserve behavior, shift where it is verified.** Heavy path tests are not deleted; they are reclassified to acceptance where they no longer tax every local/unit run.
2. **Prefer deterministic design over timing hacks.** Injected clocks and explicit hooks are more stable and faster than sleep-based assertions.
3. **Optimize by hotspot-first triage.** Start with measured slow tests, not broad speculative rewrites.
4. **Keep a fast default feedback loop.** The default suite should remain suitable for frequent local and CI execution.

## Research & Context

### Baseline (2026-02-16)

- Command run: `go test ./internal/runner/... -count=1 -json`
- Package elapsed:
  - `internal/runner`: ~22.179s
  - Other runner subpackages combined: <1s
- Test-level elapsed aggregate:
  - ~839 tests
  - total test elapsed ~27.54s
  - avg ~0.033s
- Long tail concentrated in a small set of `internal/runner` tests (0.7s-2.3s each), mostly status/live-run and broader orchestration scenarios.

### Relevant Files

- `internal/runner/status_test.go`
- `internal/runner/runner_test.go`
- `internal/runner/process_test.go`
- `internal/runner/integration_test.go`
- Existing acceptance-tagged examples in `internal/runner/*_acceptance_test.go`

