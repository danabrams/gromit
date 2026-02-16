---
id: runner-test-runtime-reduction
source_ideas: []
created: 2026-02-16
---

# Reduce internal/runner Test Runtime With Targeted Acceptance Gating

## Specification

Reduce default `internal/runner` test runtime by moving the heaviest end-to-end paths behind explicit build tags and replacing slow loop/process dependencies with tighter fakes in unit tests.

Current baseline from local profiling:
- `go test ./internal/runner -count=1`: ~23.8s package runtime
- Slowest tests are dominated by full-loop/status integration behavior:
  - `TestRunnerStatusWithLiveRun` (~2.8s, `internal/runner/runner_test.go`)
  - `TestATDDSkippedForTestOnlyBead` (~1.4s, `internal/runner/runner_test.go`)
  - `TestRunner_Status_Integration_IdleWithHistory` (~1.1s, `internal/runner/status_test.go`)
  - `TestTDDPromptSelection` (~1.0s, `internal/runner/runner_test.go`)
  - `TestScopedRun_FullLoopWithLabelFilters` (~0.9s, `internal/runner/runner_test.go`)
  - `TestStatusWithMocks` / `TestStatusWithMocks_NoWork` (~0.8-0.9s, `internal/runner/interfaces_test.go`)

### 1. Split heavy behavior by test intent

Classify tests into:
- **Unit**: validates selection logic, prompt/model routing, formatting, and branch behavior with pure fakes
- **Acceptance/Integration-tagged**: validates full runner loop orchestration, live PID/stale status handling, and multi-file status presentation end-to-end

Add `//go:build acceptance` (or `integration` when appropriate) only for tests that intentionally exercise full-stack behavior.

### 2. Keep fast unit coverage through tighter seams

For scenarios currently paying full-loop costs in unit runs:
- Inject a fake liveness checker for status PID behavior instead of relying on real process probing for most cases
- Extract/target smaller units for status rendering and next-action recommendation checks
- Replace multi-iteration run-loop assertions with focused tests on bead selection and label filtering decisions

Keep one tagged end-to-end test per critical path to protect regression coverage for real orchestration.

### 3. Add explicit runtime visibility

Add a small test-timing helper target/documented command (using `go test -json`) so contributors can quickly list top slow tests and verify improvements before/after changes.

## Acceptance Criteria

- The slowest `internal/runner` tests are inventoried in-repo with file-level ownership and classification (unit vs acceptance/integration-tagged)
- Full-loop/status end-to-end tests are moved behind explicit build tags where they are not required for default unit validation
- Equivalent fast unit tests exist for the behavior moved out of default runs (no net regression in behavior assertions)
- `go test ./internal/runner -count=1` is measurably faster than baseline (target: at least 25% reduction from ~23.8s on the same machine/setup)
- `go test -tags=acceptance ./internal/runner -count=1` (and/or `-tags=integration`) still covers the moved end-to-end paths and passes

## Decisions

1. **Gate by behavior intent, not filename convention alone.** Some existing `_acceptance_test.go` files are unit-like and should stay in default runs; only true end-to-end paths move behind tags.

2. **Preserve regression confidence with dual-layer coverage.** For each moved heavy test, keep:
- one fast unit test for branch/logic correctness
- one tagged acceptance/integration test for orchestration realism

3. **Prioritize top runtime contributors first.** Initial refactor scope targets `runner_test.go`, `status_test.go`, and `interfaces_test.go` because they contain most of the measured wall-clock cost.

4. **Make runtime budgets observable.** Keeping a repeatable timing command prevents runtime creep from reappearing silently.

## Research & Context

### Measured Baseline

Profiled with:
- `go test -json ./internal/runner -count=1`
- `jq` sort of per-test elapsed times from JSON events

Observed package runtime: ~23.8s.

### Existing Tagging Pattern

`internal/runner` already uses both `acceptance` and `integration` build tags in multiple files (for example `integration_test.go`, `globalstats_integration_test.go`, `*_acceptance_test.go`), so this change extends an existing test-segmentation pattern rather than introducing a new one.

### Primary Files

- `internal/runner/runner_test.go` — contains slow full-loop and status-oriented tests
- `internal/runner/status_test.go` — integration-style status rendering and PID lifecycle coverage
- `internal/runner/interfaces_test.go` — end-to-end `Runner.Status()` smoke checks currently in default unit pass
- `internal/runner/integration_test.go` and tagged `*_acceptance_test.go` files — destination for preserved full-path orchestration coverage
