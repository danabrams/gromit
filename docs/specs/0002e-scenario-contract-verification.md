# Spec 0002e — Scenario Contract Verification

## spec_id
scenario-contract-verification

## Depends on
spec-0002a

## Vision

Today, scenario and contract tests are written manually after implementation — a separate, human-driven step that happens outside the pipeline. The pipeline can mark a spec as `ready_for_review` without ever verifying that the spec's own scenarios actually work as described. The gap between "acceptance criteria pass" (LLM judgment) and "scenarios are mechanically verified" (real tests running real code) is where bugs hide.

We've seen this in practice: code that was written and tested but never wired up, or had small errors that existing validation and acceptance checks didn't catch. These are exactly the kinds of failures that scenario-level tests would have surfaced.

This spec adds backpressure verification to the pipeline — mechanical proof that the spec's scenarios work end-to-end, not just LLM judgment that they should. The goal is a higher standard of proof that the software is correct, enabling the pipeline to execute autonomously without human supervision.

Contract assertions are derived from the spec's scenarios before implementation begins, creating a TDD-style behavioral contract. Go scenario tests are written after implementation, exercising the actual code. Both are checked by the existing Validate gate, and failures trigger replanning like any other validation failure.

## Summary

This spec adds two new pipeline stages — WriteContracts (after Plan) and WriteScenarioTests (after Execute) — and extends the Validate stage to run both. The WriteContracts stage translates the spec's Scenarios section into declarative contract assertions before implementation begins, establishing a TDD-style behavioral contract. The WriteScenarioTests stage writes Go scenario tests after implementation, using the patterns from scenario-tests.md. The Validate stage is extended to run the scenario tests and check contract assertions alongside existing checks. Failures feed back through the existing replan loop.

## Goals

### Primary
- Translate spec Scenarios into contract assertion YAML before execution, creating a behavioral contract the implementation must satisfy
- Write Go scenario tests after execution using implementation details and scenario-tests.md patterns
- Extend Validate to run scenario tests and check contract assertions, with failures triggering replan
- Process scenarios one at a time during test writing (matching the manual workflow that worked)

### Secondary
- On replan cycles, fix the implementation to pass existing tests — do not rewrite the tests

## Non-goals
- Modifying the E2E harness or `e2e/harness_test.go` — this spec uses the contract assertion format but runs assertions in-pipeline, not via the E2E binary invocation
- Writing tests for behaviors outside the current spec's Scenarios section
- Replacing or modifying the existing Review or Accept stages
- Running E2E contracts that invoke the full binary (circular dependency)

## Architecture

### Stage placement

```
Plan → WriteContracts → Execute → WriteScenarioTests → Validate* → Review → Accept
```

### WriteContracts stage

- **Input:** Spec (specifically the Scenarios section), project context, compiled spec packet
- **Output:** Contract assertion file written to the run's evidence directory (e.g., `scenario-contracts.yaml`)
- **Agent:** LLM invocation using the spec's Scenarios section as primary input
- **Prompt pattern:** For each scenario, translate Given/When/Then into declarative assertions using the in-pipeline assertion vocabulary (see below)
- **Batch processing:** All scenarios are processed in a single LLM invocation, producing the complete contract file at once. (Unlike WriteScenarioTests, there is no inter-scenario verification step, so batching saves N-1 LLM invocations.)
- **Idempotency:** Checked via `rs.ContractsWritten` flag on RunState. If true (e.g., on a replan cycle), the stage is a no-op — returns `Continue` without regenerating
- **Empty scenarios:** If the spec has no Scenarios section or it is empty, the stage is a no-op — returns `Continue` with no output file
- **Failure mode:** If the LLM produces unparseable YAML or assertions that use keys outside the valid vocabulary → `blocked` after one retry (this is infrastructure failure, not a fixable problem)

### WriteScenarioTests stage

- **Input:** Spec Scenarios, the implementation (worktree state post-Execute), scenario-tests.md patterns
- **Output:** Go test files written to the appropriate package in the worktree
- **Agent:** LLM invocation with spec Scenarios + implementation code + scenario-tests.md as context
- **One scenario at a time:** Writes tests for each scenario sequentially, verifying each compiles before moving to the next
- **File placement:** Same package as the code under test, following the conventions in `docs/scenario-tests.md` (not `docs/scenario-testing.md`, which covers E2E contract patterns)
- **Idempotency:** Checked via `rs.ScenarioTestsWritten` flag on RunState. If true (e.g., on a replan cycle), the stage is a no-op — returns `Continue` without regenerating
- **Empty scenarios:** If the spec has no Scenarios section or it is empty, the stage is a no-op — returns `Continue` with no output files
- **Failure mode:** If tests don't compile after writing → attempt one self-repair. Still broken → `blocked`

### Validate extension

Validate currently runs `always_run` checks (go test, go vet, etc.). Extended to also:

1. Run the scenario tests written by WriteScenarioTests (already picked up by `go test ./...` — no special handling)
2. Parse `scenario-contracts.yaml` and check each assertion against the worktree state. Contract checking is implemented as direct logic in `ValidateStage.Run()`, not as a `validator.Check` (which is shell-command-only). `ValidateStageConfig` gains an `EvidenceDir string` field so the stage can locate the contract file. Contract failures are appended to the same `failures` slice used by check failures, feeding into the existing ReplanFrom path.
3. Report contract failures as: `"contract:<scenario-name> — <assertion-type> failed: <details>"`. Scenario test failures use standard `go test` output format (already handled by the existing always-run test check). These are illustrative formats for the LLM planner — no parsing dependency.

No new replan mechanism — uses the existing `replan_from` with failure context.

### Replan behavior

On replan triggered by scenario test or contract failures:
- The contract YAML and scenario tests are **not rewritten** — they are the spec of record. This is enforced by RunState flags: both WriteContracts and WriteScenarioTests check `rs.ContractsWritten` / `rs.ScenarioTestsWritten` and return `Continue` immediately when true.
- The planner receives failure context identifying which tests/assertions failed
- Fix tasks target the implementation, not the tests
- If the LLM-generated contracts or tests are semantically wrong (e.g., asserting `Add(2,3) == 6`), no implementation fix can satisfy them. The pipeline will exhaust replan cycles and terminate as `needs_human` — this is the intended fallback for bad test specifications.

### In-pipeline assertion vocabulary

The E2E assertion vocabulary (`e2e/contract.go`) contains ~30 assertion types, most of which require a completed run or binary invocation. The in-pipeline contract evaluator is a **new component** (not a reuse of `e2e/runner.go`) that supports only the subset checkable against worktree state at Validate time:

| Assertion | What it checks |
|-----------|---------------|
| `file_contains` | File at path contains pattern (regex) |
| `file_not_modified` | File has not been changed from git HEAD in the worktree (checked via `git diff --name-only HEAD -- <path>`, matching the E2E harness implementation in `e2e/runner.go`) |

This is intentionally minimal. The E2E assertions for run state (`status`, `validation_pass`), events, evidence, and CLI output are not valid here — they require a completed run. If additional assertion types are needed, they should be added explicitly to this vocabulary with an in-pipeline evaluation implementation.

WriteContracts validates generated assertions against this vocabulary. Unknown assertion keys cause a retry, then `blocked`.

### Key types

```go
// Contract assertion file, written by WriteContracts stage
type ScenarioContract struct {
    Scenarios []ScenarioAssertions `yaml:"scenarios"`
}

type ScenarioAssertions struct {
    Name       string              `yaml:"name"`
    Assertions []ContractAssertion `yaml:"assertions"`
}

// ContractAssertion — typed subset of e2e.Assertion, only filesystem-checkable fields.
// This is a separate type from e2e.Assertion; do not import the e2e package.
type ContractAssertion struct {
    FileContains    *FileContainsAssertion `yaml:"file_contains,omitempty"`
    FileNotModified string                 `yaml:"file_not_modified,omitempty"`
}

type FileContainsAssertion struct {
    Path    string `yaml:"path"`
    Pattern string `yaml:"pattern"`
}
```

### RunState additions

```go
// Added to runstore.RunState for idempotency tracking
ContractsWritten     bool `json:"contracts_written"`
ScenarioTestsWritten bool `json:"scenario_tests_written"`
```

### ValidateStageConfig addition

```go
// Added to ValidateStageConfig for contract file location
EvidenceDir string
```

## Acceptance Criteria

1. A WriteContracts stage runs after Plan and before Execute, producing a `scenario-contracts.yaml` file in the run's evidence directory
2. The WriteContracts stage translates each spec Scenario into declarative assertions using only the in-pipeline assertion vocabulary (`file_contains`, `file_not_modified`)
3. Generated contract assertions are validated against the known vocabulary; unknown assertion keys trigger a retry, then `blocked`
4. All scenarios are processed in a single LLM invocation during contract writing (batch, not sequential)
5. A WriteScenarioTests stage runs after Execute and before Validate, producing Go test files in the worktree
6. The WriteScenarioTests stage follows the Seed → Invoke → Assert pattern from `docs/scenario-tests.md`
7. Scenarios are processed one at a time during scenario test writing
8. Each scenario test compiles before the stage moves to the next scenario; one self-repair attempt on compile failure
9. The Validate stage runs scenario tests as part of its existing `go test ./...` execution (no special handling needed — they're just test files)
10. The Validate stage checks contract assertions via direct logic in `ValidateStage.Run()` (not a `validator.Check`), using `EvidenceDir` from `ValidateStageConfig` to locate the contract file
11. Contract assertion failures produce failure context identifying the scenario name and failed assertion
12. Scenario test failures are reported through the existing always-run test check (standard `go test` output)
13. Scenario test and contract failures trigger replan via the existing `replan_from` mechanism
14. On replan cycles, both WriteContracts and WriteScenarioTests are no-ops when their RunState flags (`ContractsWritten`, `ScenarioTestsWritten`) are true — fix tasks target the implementation, not the tests
15. If WriteContracts produces unparseable YAML or invalid assertions after one retry, the stage returns `blocked`
16. If WriteScenarioTests produces tests that don't compile after one self-repair attempt, the stage returns `blocked`
17. If the spec has no Scenarios section or it is empty, both WriteContracts and WriteScenarioTests are no-ops (return `Continue`)
18. All existing pipeline tests continue to pass

## Scenarios

### Scenario: Happy path — contracts and scenario tests both pass

**Given:** A spec with 2 scenarios (e.g., "Add function works" and "Subtract function works"), a working fixture repo, and a plan that correctly addresses both scenarios
**When:** The pipeline runs through all stages
**Then:**
- WriteContracts produces `scenario-contracts.yaml` with assertions for both scenarios (e.g., `file_contains: "func Add"`, `file_contains: "func Subtract"`)
- Execute implements the spec
- WriteScenarioTests produces Go test files following Seed → Invoke → Assert pattern
- Validate runs `go test ./...` (which includes the new scenario tests) and checks contract assertions — all pass
- Pipeline continues to Review and Accept without replanning
**Notes:** This is the baseline success case

### Scenario: Contract assertion fails, triggers replan

**Given:** A spec with a scenario "Subtract function works", and an executor that implements Add but forgets to wire up Subtract
**When:** Validate checks the contract assertion `file_contains: {path: calc/calc.go, pattern: "func Subtract"}`
**Then:**
- The assertion fails
- Validate returns `replan_from` with failure context: `"contract:subtract-works — file_contains calc/calc.go 'func Subtract' failed"`
- The planner receives this failure and produces a fix task targeting the missing Subtract function
- On the next cycle, the contract assertion passes
**Notes:** This is the exact "written but never wired up" failure that motivated this spec

### Scenario: Scenario test fails, triggers replan

**Given:** A spec with a scenario "Divide returns float64", and an implementation where Divide returns int instead of float64
**When:** Validate runs `go test ./...` which includes the scenario test asserting float64 return
**Then:**
- The scenario test fails with a type assertion or value error
- Validate returns `replan_from` with failure context: `"validation:scenario-test — TestScenario_Divide_ReturnsFloat64: expected 2.5, got 2"`
- The planner produces a fix task targeting the Divide return type
- On the next cycle, the scenario test passes

### Scenario: WriteContracts produces invalid YAML

**Given:** A spec with scenarios, and the LLM produces malformed YAML for the contract file
**When:** WriteContracts attempts to parse the output
**Then:**
- Parse fails, stage retries once with error context
- If the retry also produces invalid YAML, the stage returns `blocked`
- The run terminates with `blocked` status — no Execute, no replan
**Notes:** This is infrastructure failure, not a spec implementation problem

### Scenario: Spec has no scenarios section

**Given:** A spec with no Scenarios section (or an empty one)
**When:** The pipeline reaches WriteContracts
**Then:**
- WriteContracts detects no scenarios, returns `Continue` with no output file
- Execute runs normally
- WriteScenarioTests detects no scenarios, returns `Continue` with no output files
- Validate runs existing checks only (no contract file to parse, no scenario tests to run)
- Pipeline continues to Review and Accept
**Notes:** Specs without scenarios still flow through the pipeline as they do today

### Scenario: Replan preserves contracts and scenario tests

**Given:** A spec where Execute produces an implementation that fails a contract assertion, and WriteContracts and WriteScenarioTests have already produced their artifacts
**When:** Validate triggers replan and the pipeline re-executes from Plan
**Then:**
- Plan produces fix tasks based on the failure context
- WriteContracts detects `rs.ContractsWritten == true`, returns `Continue` (no-op)
- Execute runs the fix tasks
- WriteScenarioTests detects `rs.ScenarioTestsWritten == true`, returns `Continue` (no-op)
- Validate re-runs — contract assertions and scenario tests now pass
**Notes:** This verifies the idempotency guarantee that protects test artifacts across replan cycles

### Scenario: Scenario test doesn't compile, self-repair succeeds

**Given:** A spec with 3 scenarios, and the LLM writes a scenario test for scenario 2 that references a nonexistent function name
**When:** WriteScenarioTests checks compilation after writing scenario 2's test
**Then:**
- Compile check fails
- Stage makes one self-repair attempt with the compile error as context
- Repaired test compiles
- Stage proceeds to write scenario 3's test

## Validation

```bash
# All unit and scenario tests
go test ./... -count=1

# Specifically the new stages
go test ./internal/next/specloop/stages/ -count=1 -run TestWriteContracts
go test ./internal/next/specloop/stages/ -count=1 -run TestWriteScenarioTests

# Validate stage extension
go test ./internal/next/specloop/stages/ -count=1 -run TestValidate

# Contract evaluator
go test ./internal/next/specloop/contract/ -count=1

# Vet
go vet ./...
```
