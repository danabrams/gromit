# Spec 0002f — Scenario Test Writing

## spec_id
scenario-test-writing

## Depends on
spec-0002e

## Vision

Contract assertions (spec 0002e) verify structural properties — that files exist, contain expected patterns, remain unmodified. But structural checks can't catch semantic bugs: a function with the right name but the wrong return type, an off-by-one error in a calculation, a method that silently swallows errors instead of propagating them. These are exactly the kinds of failures we've seen in practice.

Scenario tests exercise actual behavior. They call the real code, pass real inputs, and assert real outputs. Together with contracts, they close the gap between "LLM says it works" and "it actually works," enabling the pipeline to execute autonomously with a higher standard of proof.

This spec adds a WriteScenarioTests pipeline stage (after Execute, before Validate) that writes Go scenario tests using the patterns from `docs/scenario-tests.md`. Tests are run by the existing `go test ./...` at Validate time, and failures trigger replanning like any other validation failure.

## Summary

This spec adds a WriteScenarioTests stage after Execute and before Validate. The stage writes Go scenario tests one at a time, verifying each compiles before moving to the next. Tests follow the Seed/Invoke/Assert pattern from `docs/scenario-tests.md`. The Validate stage already runs `go test ./...`, which picks up the new test files automatically. Failures feed back through the existing replan loop. A persistent failure tracker (shared across both contract and scenario test failures) provides diagnostic hints when the same failures repeat across replan cycles.

## Goals

### Primary
- Write Go scenario tests after execution using implementation details and `docs/scenario-tests.md` patterns
- Process scenarios one at a time, verifying compilation between each
- Track persistent failures across replan cycles to surface likely bad test specifications

### Secondary
- On replan cycles, fix the implementation to pass existing tests — do not rewrite the tests

## Non-goals
- Redefining scenario parsing or the `SpecScenario` type (defined in spec 0002e)
- Redefining contract assertion types or the `ContractEvaluator` interface (defined in spec 0002e)
- Modifying the E2E harness or `e2e/harness_test.go`
- Writing tests for behaviors outside the current spec's Scenarios section
- Replacing or modifying the existing Review or Accept stages

## Architecture

### Stage placement

```
Plan → WriteContracts → Execute → WriteScenarioTests → Validate* → Review → Accept
```

WriteContracts and the Validate contract-checking extension are defined in spec 0002e. This spec adds WriteScenarioTests between Execute and Validate.

### WriteScenarioTests stage

- **Input:** Spec Scenarios (parsed using the `SpecScenario` type from spec 0002e), the implementation (worktree state post-Execute — implementation files aggregated as a deduplicated union from per-task `FilesChanged` across all tasks with `Status == "done"` in the current run (i.e., `rs.Tasks[*].FilesChanged`, merged into a unique set)), `docs/scenario-tests.md` patterns
- **Output:** Go test files written to the appropriate package in the worktree. A manifest of written test file paths is also recorded in the run's evidence directory as `scenario-test-manifest.json` for post-mortem debugging. The manifest is written incrementally — updated after each scenario's test successfully compiles.
The manifest format is:
```json
{
  "scenarios": [
    {"name": "Add function works", "test_file": "calc/calc_scenario_add_test.go"},
    {"name": "Subtract function works", "test_file": "calc/calc_scenario_subtract_test.go"}
  ]
}
```
Each entry maps a scenario name to its dedicated test file. Each scenario gets its own test file — this avoids the footgun where deleting a shared file to retry one scenario destroys another scenario's working tests.
- **Agent:** LLM invocation via the `ScenarioTestWriter` interface (injected dependency). Receives spec Scenarios + implementation code (from `rs.Tasks[*].FilesChanged`) + `docs/scenario-tests.md` as context. The content of `docs/scenario-tests.md` is wired at the adapter construction level (part of the system prompt, not a per-call parameter), following the pattern used by ReviewStage and AcceptStage. `docs/scenario-tests.md` covers CLI-layer in-process synthetic-store tests; the LLM should adapt the three-phase Seed/Invoke/Assert structure to implementation testing (e.g., calling `calc.Add(2, 3)` directly rather than invoking CLI commands). `docs/scenario-testing.md` (created as part of the E2E harness setup; if this file does not yet exist, only `docs/scenario-tests.md` is required for this spec) covers full E2E contract YAML patterns — this stage uses the former.
- **One scenario at a time:** Writes tests for each scenario sequentially, verifying each compiles before moving to the next
- **File placement:** Same package as the code under test, following the conventions in `docs/scenario-tests.md`
- **Idempotency:** Checked via `rs.ScenarioTestsWritten` flag on RunState. If true (e.g., on a replan cycle), the stage is a no-op — returns `Continue` without regenerating. On success (all scenarios written), the stage sets `rs.ScenarioTestsWritten = true` before returning.
- **Empty scenarios:** If the spec has no Scenarios section or it is empty, the stage is a no-op — returns `Continue` with no output files
- **Partial completion:** If scenario N fails self-repair, the stage returns `blocked`. Tests written for scenarios 1..N-1 remain in the worktree but `rs.ScenarioTestsWritten` is NOT set, so the stage will re-run on retry (all scenarios must succeed for the flag to be set).
- **Re-run behavior:** On retry when `ScenarioTestsWritten` is false, the stage reads the partial manifest (`scenario-test-manifest.json`) to determine which scenarios already have test files. The manifest is written incrementally — after each scenario's test successfully compiles, the manifest is updated (appended). If no manifest exists, all scenarios are processed from scratch. For each scenario listed in the manifest, if the test file exists and compiles, it is skipped. If it exists but does not compile, it is deleted and re-written (counting as a fresh attempt with the standard self-repair budget). This avoids re-writing working tests from a prior partial run.
- **Failure mode:** If tests don't compile after writing -> attempt up to 2 self-repairs (3 total attempts: 1 initial + 2 retries) with compile errors as context. Compilation is checked via `go test -c -o /dev/null ./<package-path>` scoped to the package containing the test file. Self-repair is orchestrated by the `WriteScenarioTestsStage`, not internal to the `ScenarioTestWriter` — the stage calls `WriteScenarioTest` (with empty `compileErrors`), uses the returned `testFilePath` to run `go test -c -o /dev/null ./<package-path>` scoped to the containing package, and on failure calls `WriteScenarioTest` again with the build output as `compileErrors`. Still broken after 2 retries → `blocked`
- **Events:** Emits `scenario_tests_written` event after each successful scenario test, and `scenario_tests_complete` when all scenarios are done, or `scenario_tests_blocked` on terminal failure
- **Budget checking:** The stage checks remaining budget between scenario iterations (the specloop budget check only runs between stages). If budget is exhausted mid-stage, the stage returns `blocked` with a budget-exhausted message.
- **Model tier:** Uses Sonnet (P1) — test writing benefits from seeing implementation code but doesn't require Opus-level reasoning

### Validate integration

No special handling is needed for running scenario tests (they are picked up by `go test ./...`). However, the persistent failure tracking mechanism requires changes to the specloop replan path — see FailureHistory implementation below. Scenario test failures are reported through the existing always-run test check (standard `go test` output format, already handled by the existing always-run test check).

### Replan behavior

On replan triggered by scenario test failures:
- The scenario tests are **not rewritten** — they are the spec of record. This is enforced by the RunState flag: WriteScenarioTests checks `rs.ScenarioTestsWritten` and returns `Continue` immediately when true.
- The planner receives failure context identifying which tests failed
- Fix tasks target the implementation, not the tests
- If the LLM-generated tests are semantically wrong (e.g., asserting `Add(2,3) == 6`), no implementation fix can satisfy them. The pipeline will exhaust replan cycles and terminate as `needs_human` — this is the intended fallback for bad test specifications.

### Persistent failure tracking

When the same contract or scenario test failures persist across 2+ consecutive replan cycles with different implementations, the failure context includes a hint: `"persistent-failure: <test/contract> has failed N consecutive cycles — may indicate a bad test specification rather than an implementation bug"`. This helps humans triage `needs_human` runs.

This tracking covers both contract failures (from spec 0002e) and scenario test failures, using a shared `FailureHistory` map on RunState.

### FailureHistory implementation

The specloop replan path (in `specloop.go`, where `ReplanFrom` is handled) is responsible for incrementing `FailureHistory`:

1. After Validate returns failures, the replan handler extracts failure keys:
   - Test function names from `--- FAIL: TestName` lines (for scenario test failures)
   - Contract keys from the `contract:<name>` prefix (for contract assertion failures)
2. For each failure key present in the current cycle: increment its count in `FailureHistory`
3. For each key in `FailureHistory` NOT present in the current cycle's failures: reset to zero (the failure was resolved)
4. The diagnostic hint is appended to failure context when `FailureHistory[key] >= 2`

This implementation lives in `specloop.go` as helper functions (exported for testing), not in the Validate stage or WriteScenarioTests stage. Spec 0002f owns the full `FailureHistory` implementation for both contract and scenario test failures.

**Failure message format contract:** Contract failures use the format `contract:<scenario-name> — <assertion-type> failed: <details>` (defined in spec 0002e). The key extraction function splits on ` — ` and takes the first segment. Test failure keys are extracted by matching `--- FAIL: TestFunctionName` from standard `go test` output. Both extraction functions must be well-defined and testable.

### Stage interface

```go
// ScenarioTestWriter — injected into WriteScenarioTestsStage.
// On self-repair retries, compileErrors contains the output from the failed
// `go test -c` invocation; on the initial call it is empty.
type ScenarioTestWriter interface {
    WriteScenarioTest(ctx context.Context, scenario SpecScenario, implFiles []string, workDir string, compileErrors string) (testFilePath string, err error)
}
```

The `SpecScenario` type is defined in spec 0002e and lives in `internal/next/contract/`. Import it from there.

Model tier selection (P1/Sonnet) is configured at the adapter wiring level in `stage_provider.go`, not in the interface contract, following the existing pattern (e.g., `planAdapter` uses `policy.Models.Planner`).

### RunState additions

```go
// Added to runstore.RunState for idempotency tracking.
// IMPORTANT: This flag must NOT be added to the per-cycle reset block in
// specloop.go (which resets FinalValidationPassed, FinalReviewPassed, etc.).
// It intentionally persists across replan cycles to prevent regeneration.
ScenarioTestsWritten bool `json:"scenario_tests_written"`

// Track per-test/contract failure counts across replan cycles for persistent-failure diagnostics.
// Key: test function name (for scenario tests) or "contract:<scenario-name>" (for contract assertions).
// Value: consecutive cycle count.
// NOT reset in the per-cycle reset block — accumulates across cycles.
// "Same failure" is determined by matching the test function name (for scenario tests)
// or contract scenario name (for contract assertions), not the full failure message.
// Test function names are extracted from `go test` output by matching the standard
// Go test failure line format: "--- FAIL: TestFunctionName (duration)".
// Contract failure keys use the `contract:<scenario-name>` prefix, extracted by
// splitting the failure message on " — " and taking the first segment.
FailureHistory map[string]int `json:"failure_history,omitempty"`
```

### Pipeline wiring

`BuildStages` in `cmd/gromit-next/stage_provider.go` must be updated to include WriteScenarioTests at the correct position (building on the pipeline from spec 0002e):

```go
return []specloop.Stage{
    initStage,
    compileStage,
    planStage,
    writeContractsStage,     // from spec 0002e
    executeStage,
    writeScenarioTestsStage, // NEW — after execute, before validate
    validateStage,
    reviewStage,
    acceptStage,
    evidenceStage,
    finalizeStage,
}, nil
```

WriteScenarioTests is naturally excluded from `dryRunStages` since it comes after Execute.

This includes creating an LLM adapter for `ScenarioTestWriter` and providing a noop implementation for testing. The noop implementation (`noopScenarioTestWriter`) lives in `cmd/gromit-next/stage_provider.go`, following the existing pattern of `noopAcceptEvaluator` and `noopReviewRunner` in that file.

## Acceptance Criteria

1. A WriteScenarioTests stage runs after Execute and before Validate, producing Go test files in the worktree
2. The WriteScenarioTests stage follows the Seed/Invoke/Assert pattern from `docs/scenario-tests.md`
3. Scenarios are processed one at a time during scenario test writing
4. Each scenario test compiles before the stage moves to the next scenario; up to two self-repair attempts on compile failure
5. Scenario test failures detected by the Validate stage's always-run `go test` check trigger replan via the existing `replan_from` mechanism
6. On replan cycles, WriteScenarioTests is a no-op when its RunState flag (`ScenarioTestsWritten`) is true — fix tasks target the implementation, not the tests. WriteScenarioTests sets `rs.ScenarioTestsWritten = true` only when all scenarios succeed.
7. If WriteScenarioTests produces tests that don't compile after two self-repair attempts, the stage returns `blocked`. Tests for previously completed scenarios remain in the worktree but `ScenarioTestsWritten` is not set.
8. If the spec has no Scenarios section or it is empty, WriteScenarioTests is a no-op (returns `Continue`)
9. The WriteScenarioTests stage uses an injected `ScenarioTestWriter` interface matching the existing `PlanCreator`/`ReviewRunner` pattern for testability
10. `BuildStages` in `cmd/gromit-next/stage_provider.go` is updated to include WriteScenarioTests in the correct pipeline position
11. `ScenarioTestsWritten` RunState flag is NOT reset in the per-cycle reset block in `specloop.go`
12. WriteScenarioTests uses Sonnet (P1) model tier
13. All existing pipeline tests continue to pass
14. WriteScenarioTests emits events: `scenario_tests_written` per scenario, `scenario_tests_complete` on full success, `scenario_tests_blocked` on failure
15. WriteScenarioTests checks remaining budget between scenario iterations and returns `blocked` if budget is exhausted mid-stage
16. WriteScenarioTests records a manifest of written test file paths in the evidence directory as `scenario-test-manifest.json`
17. On retry when `ScenarioTestsWritten` is false, WriteScenarioTests skips scenarios whose test files already exist and compile, avoiding redundant LLM invocations
18. Persistent failure tracking uses `FailureHistory` on RunState (keyed by test function name or `contract:<scenario-name>`) to count consecutive cycle failures; threshold of 2+ triggers the diagnostic hint
19. When the same contract or scenario test failures persist across 2+ consecutive replan cycles, failure context includes a `persistent-failure` diagnostic hint
20. Each scenario gets its own dedicated test file — no shared test files between scenarios
21. Compilation is checked via `go test -c -o /dev/null ./<package-path>` (not `go build`, which skips `_test.go` files)

## Scenarios

### Scenario: Happy path — scenario tests pass

**Given:** A spec with 2 scenarios (e.g., "Add function works" and "Subtract function works"), a working implementation produced by Execute, and contracts already passing (per spec 0002e)
**When:** The pipeline runs WriteScenarioTests and then Validate
**Then:**
- WriteScenarioTests produces Go test files following Seed/Invoke/Assert pattern for both scenarios
- Validate runs `go test ./...` (which includes the new scenario tests) — all pass
- Pipeline continues to Review and Accept without replanning
**Notes:** This validates that scenario tests work end-to-end alongside contracts from 0002e

### Scenario: Scenario test fails, triggers replan

**Given:** A spec with a scenario "Divide returns float64", and an implementation where Divide returns int instead of float64
**When:** Validate runs `go test ./...` which includes the scenario test asserting float64 return
**Then:**
- The scenario test fails with a type assertion or value error
- Validate returns `replan_from` with failure context: `"validation:scenario-test — TestScenario_Divide_ReturnsFloat64: expected 2.5, got 2"`
- The planner produces a fix task targeting the Divide return type
- On the next cycle, the scenario test passes

### Scenario: Scenario test doesn't compile, self-repair succeeds

**Given:** A spec with 3 scenarios, and the LLM writes a scenario test for scenario 2 that references a nonexistent function name
**When:** WriteScenarioTests checks compilation after writing scenario 2's test
**Then:**
- Compile check fails
- Stage makes one self-repair attempt with the compile error as context
- Repaired test compiles
- Stage proceeds to write scenario 3's test

### Scenario: Scenario test doesn't compile, self-repair fails

**Given:** A spec with 3 scenarios, and the LLM writes a scenario test for scenario 2 that imports a nonexistent package
**When:** WriteScenarioTests checks compilation after writing scenario 2's test
**Then:**
- Compile check fails
- Stage makes two self-repair attempts with compile errors as context
- Both repairs still fail to compile
- Stage returns `blocked`
- Tests for scenario 1 (which compiled successfully) remain in the worktree
- `rs.ScenarioTestsWritten` is NOT set (stage will re-run on retry)
**Notes:** Partial completion — earlier scenarios' tests persist but the flag is not set

### Scenario: Replan preserves scenario tests

**Given:** A spec where Execute produces an implementation that fails a scenario test, and WriteScenarioTests has already produced its artifacts (all compiled successfully)
**When:** Validate triggers replan and the pipeline re-executes from Plan
**Then:**
- Plan produces fix tasks based on the failure context
- WriteScenarioTests detects `rs.ScenarioTestsWritten == true`, returns `Continue` (no-op)
- Execute runs the fix tasks
- Validate re-runs — scenario tests now pass
**Notes:** This verifies the idempotency guarantee that protects test artifacts across replan cycles

### Scenario: WriteScenarioTests re-runs after partial failure

**Given:** A spec with 3 scenarios. On the first attempt, WriteScenarioTests successfully wrote tests for scenarios 1 and 2, but scenario 3 failed self-repair and the stage returned `blocked`. The pipeline was retried.
**When:** WriteScenarioTests runs again (since `ScenarioTestsWritten` is false)
**Then:**
- The stage checks for existing test files for scenario 1 — finds it, confirms it compiles, skips it
- The stage checks for existing test files for scenario 2 — finds it, confirms it compiles, skips it
- The stage invokes the LLM only for scenario 3
- If scenario 3's test now compiles, `ScenarioTestsWritten` is set to true
**Notes:** This avoids wasting LLM invocations re-writing tests that already work from the prior partial run

### Scenario: Budget exhausted during scenario test writing

**Given:** A spec with 3 scenarios, and budget is nearly exhausted after scenario 1's test is written
**When:** WriteScenarioTests checks budget before starting scenario 2
**Then:**
- The stage detects budget is exhausted
- Returns `blocked` with a budget-exhausted message
- Test for scenario 1 remains in the worktree
- `rs.ScenarioTestsWritten` is NOT set
**Notes:** Budget checking between scenario iterations prevents runaway costs when the pipeline is running low on budget

### Scenario: Persistent failures trigger diagnostic hint

**Given:** A spec where WriteScenarioTests wrote a test asserting `Add(2,3) == 6` (semantically wrong — correct answer is 5), and the implementation correctly returns 5
**When:** Validate fails on the scenario test, triggers replan, Execute produces a new implementation, and Validate fails again with the same test
**Then:**
- After the second consecutive failure of the same test, the failure context includes: `"persistent-failure: TestScenario_Add has failed 2 consecutive cycles — may indicate a bad test specification rather than an implementation bug"`
- The pipeline continues replanning (if cycles remain) or terminates as `needs_human`
**Notes:** This diagnostic helps humans triage `needs_human` runs by distinguishing bad tests from bad implementations. The same mechanism covers contract failures from spec 0002e.

### Scenario: Scenario test manifest is recorded

**Given:** A spec with 2 scenarios, and WriteScenarioTests successfully writes tests for both
**When:** The stage completes
**Then:**
- `scenario-test-manifest.json` is written to the evidence directory
- The manifest contains the file paths of both written test files
- `rs.ScenarioTestsWritten` is set to true
**Notes:** The manifest supports post-mortem debugging by recording which test files were written and where

### Scenario: WriteContracts blocked but WriteScenarioTests succeeds (illustrative)

**Given:** A spec with scenarios where WriteContracts returned `blocked` (no `scenario-contracts.yaml` exists), but Execute produced a working implementation
**When:** The pipeline is restarted from Execute, and WriteScenarioTests runs
**Then:**
- WriteScenarioTests writes Go test files for all scenarios (it does not depend on contract artifacts)
- Validate skips contract checking (no contract file) but runs `go test ./...` including scenario tests
- If scenario tests pass, the pipeline continues to Review and Accept
- The run proceeds with scenario test coverage only, without contract coverage
**Notes:** This scenario is illustrative — it describes desired degradation behavior. The mechanism for continuing past a blocked WriteContracts stage (e.g., operator intervention, non-blocking stage configuration) is outside the scope of this spec. Contract assertions and scenario tests are independent verification mechanisms.

### Scenario: Two scenarios testing the same package get separate files

**Given:** A spec with 2 scenarios ("Add function works" and "Subtract function works") that both test code in the `calc` package.
**When:** WriteScenarioTests writes tests for both scenarios
**Then:**
- Scenario 1's test is written to a dedicated file (e.g., `calc/calc_scenario_add_test.go`)
- Scenario 2's test is written to a separate dedicated file (e.g., `calc/calc_scenario_subtract_test.go`)
- The compilation check (`go test -c -o /dev/null ./<package-path>`) covers the entire package, verifying both files compile together
- If scenario 2's file has an issue, deleting and retrying it does not affect scenario 1's file
**Notes:** One file per scenario avoids the footgun where retrying one scenario destroys another's working test.

## Validation

```bash
# All unit and scenario tests
go test ./... -count=1

# WriteScenarioTests stage (unit tests with mocked interfaces)
go test ./internal/next/specloop/stages/ -count=1 -run TestWriteScenarioTests

# Integration tests — exercise the full stage sequence with mocked LLM responses:
# Plan → WriteContracts → Execute → WriteScenarioTests → Validate,
# including replan cycles, idempotency flag behavior, and persistent failure tracking
go test ./internal/next/specloop/ -count=1 -run TestIntegration

# Vet
go vet ./...
```
