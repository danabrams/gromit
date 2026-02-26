---
id: real-tests-for-chain-logic
source_ideas: []
created: 2026-02-07
epic: test-quality
---

# Real Tests for Chain Logic

## Specification

Replace documentary tests that re-implement production logic inline with proper tests that call actual functions. Documentary tests give false coverage — they verify the test author's understanding of the code, not the code itself. If production code changes, these tests still pass because they never call it.

There are 10 documentary tests across 4 files. Each must be deleted and replaced with a test that exercises the real production function. Where production code is too tightly coupled to test directly (stdin/stdout/exec), extract testable helpers first, then test those.

### File 1: `cmd/gromit/chain_integration_test.go` (5 tests, lines 135-327)

**Problem**: Five tests manually re-implement `chainAfterRefine`'s three-phase logic — file-existence checks, counter increments, loop-break-on-decline — without ever calling `chainAfterRefine` or any helper it uses.

**Fix**: Refactor `chainAfterRefine` in `cmd/gromit/chain.go` to accept injected dependencies for user confirmation and subprocess execution. Extract the function signature to something like:

```go
func chainAfterRefine(specNames []string, plansDir string, confirm func(string, bool) bool, execute func(...string) error)
```

Then the 5 documentary tests become proper tests that call `chainAfterRefine` with:
- A `confirm` func that returns canned yes/no responses per spec name
- An `execute` func that records calls and returns success/failure
- A real filesystem (t.TempDir) for plan file existence checks

The existing call site in `chain.go` passes `confirmPrompt(reader, ...)` and `execGromit` as the real implementations.

**Tests to delete and replace**:
- `TestChainAfterRefinePhase1Planning` — replace with test that calls `chainAfterRefine` with confirm=always-yes, execute=creates-plan-files, verifies planned names tracked correctly
- `TestChainAfterRefinePhase2Decompose` — replace with test that calls `chainAfterRefine` through Phase 2, verifies decompose offered for planned specs
- `TestChainAfterRefinePhase3RunOnlyIfDecomposed` — replace with test that verifies `gromit run` is only offered when decomposedCount > 0, by controlling execute outcomes
- `TestChainAfterRefineDecomposedCountIncrementsIncorrectly` — replace with test that verifies the actual bug: execute returning nil for exit failures causes incorrect count. If the bug is fixed, update the test expectation.
- `TestChainAfterRefineBreakOnDecline` — replace with test that passes confirm=decline-on-second, verifies remaining specs are skipped

**Keep**: `TestChainAfterRefineThreePhasesEmptyInput` (line 110-130) already calls `chainAfterRefine` directly. `TestExecGromitSuccessExitZero`, `TestExecGromitNonZeroExit`, `TestExecGromitLaunchFailure`, and `TestConfirmPromptDefaultBehavior` all call actual functions. Leave these alone.

### File 2: `internal/bead/bead_test.go` (2 tests, lines 645-913)

**Problem**: `TestReadyVsReadyAny` and `TestReadyExcludesEpics` manually parse JSON and filter epics using the same algorithm as `parseBeadOutput` / `parseBeadOutputExcluding`, instead of calling those functions.

**Fix**: Rewrite both tests to call the actual functions directly. These are unexported but the tests are in the same package, so no refactoring needed.

- `TestReadyVsReadyAny` → call `parseBeadOutput(tt.jsonOutput)` and assert on the returned bead
- `TestReadyExcludesEpics` → call `parseBeadOutputExcluding(tt.jsonOutput, "epic")` and assert on the returned bead

The test table data (JSON inputs and expected outputs) is good and can be reused — only the test body that manually re-implements parsing needs to change.

### File 3: `cmd/gromit/review_test.go` (2 tests, lines 12-84)

**Problem**: `TestReviewPassesClaudeFlags` and `TestReviewWithoutFlags` construct a `config.Config`, then assert the struct fields equal the values just assigned. They never call `runReviewInteractive` or `runReviewNonInteractive`.

**Fix**: Extract the arg-building logic from `runReviewInteractive` (lines 314-317 of `review.go`) into a testable function:

```go
func buildReviewArgs(flags []string, initialPrompt string) []string
```

Then replace the documentary tests with tests that call `buildReviewArgs` and verify the output includes configured flags in the correct order. The test data (flags vs no-flags configs) is reusable.

### File 4: `internal/runner/process_test.go` (1 test, lines 380-409)

**Problem**: `TestBeadContextRetryCounters` manually increments and resets integer fields to "simulate" escalation. It tests that `i++` makes `i` equal `i+1`.

**Fix**: Delete this test. The actual escalation behavior is already thoroughly tested in `internal/runner/integration_test.go` via tests like `TestIntegration_EscalationChainFullFlow` that exercise the real `processBead` → escalation path. This documentary test is redundant with those integration tests.

If there's value in a unit test for counter reset behavior specifically, write one that calls the actual escalation function (or the method that resets counters on model change) rather than manually manipulating struct fields.

## Acceptance Criteria

- All 10 documentary tests identified above are deleted
- `bead_test.go`: 2 replacement tests call `parseBeadOutput`/`parseBeadOutputExcluding` directly with the same test table inputs
- `chain_integration_test.go`: `chainAfterRefine` accepts injected confirm/execute dependencies; 5 replacement tests call it with fakes
- `review_test.go`: arg-building logic extracted into a testable function; 2 replacement tests call it
- `process_test.go`: documentary counter test deleted (covered by existing integration tests)
- All existing tests that already call real functions (`TestExecGromitSuccessExitZero`, `TestConfirmPromptDefaultBehavior`, etc.) remain unchanged and passing
- `go test ./...` passes

## Decisions

1. **Dependency injection over test doubles for `chainAfterRefine`**. Rather than mocking stdin/exec at the OS level (brittle), we inject `confirm` and `execute` functions as parameters. This is the simplest refactor that makes the function testable — it doesn't require interfaces, structs, or new types. The real call site just passes the existing `confirmPrompt` and `execGromit` functions.

2. **Extract `buildReviewArgs` rather than testing through exec**. The review functions shell out to `claude`, which makes end-to-end testing expensive. The arg-building is the only logic worth unit-testing. Extracting it is minimal churn and tests exactly what the documentary tests were trying to verify.

3. **Delete rather than replace the retry counter test**. The counter manipulation test is redundant with `integration_test.go`'s escalation tests, which exercise the real retry/escalation path. A unit test for `i++` adds no value.

4. **Reuse existing test table data**. The JSON inputs and expected outputs in `bead_test.go` are well-crafted. Only the test bodies (which re-implement parsing logic) change — the table-driven structure stays.

5. **Duplicated test helpers are out of scope**. `replaceOrAppend`, `runGromitWithStdin`, and `findRealGit` are copy-pasted across 3 packages. This is a maintenance concern but a different problem — it doesn't create false coverage the way documentary tests do.

## Research & Context

### Current State

**Documentary tests exist in**:
- `cmd/gromit/chain_integration_test.go` — 5 tests (lines 135-327)
- `internal/bead/bead_test.go` — 2 tests (lines 645-913)
- `cmd/gromit/review_test.go` — 2 tests (lines 12-84)
- `internal/runner/process_test.go` — 1 test (lines 380-409)

**Production functions they should be testing**:
- `chainAfterRefine()` in `cmd/gromit/chain.go:105`
- `parseBeadOutput()` in `internal/bead/bead.go:139`
- `parseBeadOutputExcluding()` in `internal/bead/bead.go:164`
- `runReviewInteractive()` in `cmd/gromit/review.go:~290` (arg-building at lines 314-317)

**Gold-standard tests for reference**:
- `internal/runner/integration_test.go` — exercises `r.Run()` with injected mock dependencies
- `test/e2e/e2e_test.go` — full binary tests with fake CLIs
- `internal/config/config_test.go` — calls actual `SelectModel()`, `NextEscalationModel()`, `Load()`

### Why Documentary Tests Exist

The production functions `chainAfterRefine` and `runReviewInteractive` are tightly coupled to stdin/stdout/exec. The test authors wrote documentary tests as a workaround. The proper fix is to make the functions testable through dependency injection, which this spec prescribes.
