DONE 2026-03-19
# Spec 0002e — Contract Assertions

## spec_id
contract-assertions

## Depends on
spec-0002a

## Vision

Today, scenario and contract tests are written manually after implementation — a separate, human-driven step that happens outside the pipeline. The pipeline can mark a spec as `ready_for_review` without ever verifying that the spec's own scenarios actually work as described. The gap between "acceptance criteria pass" (LLM judgment) and "scenarios are mechanically verified" (real tests running real code) is where bugs hide.

We've seen this in practice: code that was written and tested but never wired up, or had small errors that existing validation and acceptance checks didn't catch. These are exactly the kinds of failures that contract-level assertions would have surfaced.

This spec adds the contract assertion half of backpressure verification — declarative YAML assertions derived from the spec's scenarios, checked mechanically at Validate time. A companion spec (0002f — Scenario Test Writing) adds Go scenario tests that exercise actual behavior, closing the gap between structural checks and semantic correctness.

## Summary

This spec adds a WriteContracts pipeline stage (after Plan) and extends the Validate stage to check contract assertions. The WriteContracts stage translates the spec's Scenarios section into declarative contract assertions before implementation begins, establishing a TDD-style behavioral contract. The Validate stage is extended to check these assertions alongside existing checks. Failures feed back through the existing replan loop.

## Goals

### Primary
- Translate spec Scenarios into contract assertion YAML before execution, creating a behavioral contract the implementation must satisfy
- Extend Validate to check contract assertions, with failures triggering replan

### Secondary
- On replan cycles, fix the implementation to pass existing contracts — do not rewrite the contracts

## Non-goals
- Modifying the E2E harness or `e2e/harness_test.go` — this spec uses the contract assertion format but runs assertions in-pipeline, not via the E2E binary invocation
- Writing Go scenario tests (covered by spec 0002f)
- Replacing or modifying the existing Review or Accept stages
- Running E2E contracts that invoke the full binary (circular dependency)

## Architecture

### Stage placement

```
Plan → WriteContracts → Execute → Validate* → Review → Accept
```

Note: Spec 0002f will add a `WriteScenarioTests` stage between Execute and Validate.

### Scenario parsing from markdown

Scenarios are parsed from the raw spec markdown file (not the compiled spec-packet) by matching `### Scenario:` headers. Each scenario's Given/When/Then/Notes blocks are extracted as raw text. This parsing is done by the stage before invoking the LLM, not by the LLM itself. The spec file path is provided via `WriteContractsStageConfig.SpecPath`. Scenarios missing a When or Then block are skipped with a warning log — a scenario must have at least When and Then to be included. Given and Notes are optional.

### WriteContracts stage

- **Input:** Spec (specifically the Scenarios section), project context, compiled spec packet (read from the run directory as `spec-packet.md`, written by the existing Compile stage)
- **Output:** Contract assertion file written to the run's evidence directory as `scenario-contracts.yaml`
- **Agent:** LLM invocation via the `ContractWriter` interface (injected dependency, matching the `PlanCreator`/`ReviewRunner` pattern). The spec's Scenarios section is the primary input
- **Prompt pattern:** For each scenario, translate Given/When/Then into declarative assertions using the in-pipeline assertion vocabulary (see below)
- **Batch processing:** All scenarios are processed in a single LLM invocation, producing the complete contract file at once. (Unlike scenario test writing, there is no inter-scenario verification step, so batching saves N-1 LLM invocations.)
- **Idempotency:** Checked via `rs.ContractsWritten` flag on RunState. If true (e.g., on a replan cycle), the stage is a no-op — returns `Continue` without regenerating. On success, the stage sets `rs.ContractsWritten = true` before returning.
- **Empty scenarios:** If the spec has no Scenarios section or it is empty, the stage is a no-op — returns `Continue` with no output file
- **Events:** Emits `contracts_written` event on success (with scenario count), or `contracts_blocked` on terminal failure
- **Failure mode:** If the LLM produces unparseable YAML or assertions that use keys outside the valid vocabulary -> blocked after 2 retries (3 total attempts: 1 initial + 2 retries) (this is infrastructure failure, not a fixable problem)
- **Budget checking:** Since WriteContracts is a single LLM invocation, budget is checked once before the invocation. If budget is exhausted, the stage returns `blocked` with a budget-exhausted message.
- **EvidenceDir assumption:** The stage assumes `EvidenceDir` exists (created by the Init stage or run setup). If it does not exist, the stage returns an error (infrastructure failure).
- **Model tier:** Uses Sonnet (P1) — contract generation is translation, not creative problem-solving

### Validate extension

Validate currently runs `always_run` checks (go test, go vet, etc.). Extended to also:

1. Check contract assertions via an injected `ContractEvaluator` interface (not a `validator.Check`, which is shell-command-only). This follows the existing delegation pattern — `ValidateStage` delegates to `FinalValidator` for checks and to `ContractEvaluator` for contracts. `ValidateStageConfig` gains an `EvidenceDir string` field so the stage can locate the contract file. Contract failures are appended to the same `failures` slice used by check failures, feeding into the existing ReplanFrom path.
2. **Missing contract file handling:** If `scenario-contracts.yaml` does not exist in `EvidenceDir` (e.g., spec had no scenarios, or WriteContracts was blocked), contract checking is skipped silently — this is not a failure.
3. Report contract failures in two parts: a **failure key** used for persistent failure tracking (spec 0002f) and a **human-readable message** for the LLM planner. The failure key is `contract:<scenario-name>` (e.g., `contract:subtract-works`). The full failure message is `"contract:<scenario-name> — <assertion-type> failed: <details>"` (e.g., `"contract:subtract-works — file_contains failed: pattern 'func Subtract' not found in calc/calc.go"`). The failure key is extracted by splitting on ` — ` and taking the prefix. No other parsing is required.
4. **Execution order:** Contract assertions are checked first, then always-run shell checks (`go test`, `go vet`, etc.) run regardless of contract results. All failures (contract and shell) are collected into the same `failures` slice before deciding the next action. This ensures both contract and test failures are visible in a single replan cycle, avoiding back-and-forth between fixing contracts and fixing tests.

No new replan mechanism — uses the existing `replan_from` with failure context.

### Replan behavior

On replan triggered by contract failures:
- The contract YAML is **not rewritten** — it is the spec of record. This is enforced by the RunState flag: WriteContracts checks `rs.ContractsWritten` and returns `Continue` immediately when true.
- The planner receives failure context identifying which assertions failed
- Fix tasks target the implementation, not the contracts
- Contract failures replan from the configured `ReplanStage` (which is `plan` in the standard pipeline configuration).
- If the LLM-generated contracts are semantically wrong (e.g., asserting `file_contains: "func Multiply"` when the spec says Add), no implementation fix can satisfy them. The pipeline will exhaust replan cycles and terminate as `needs_human` — this is the intended fallback for bad contract specifications.
- **Persistent failure tracking:** Spec 0002f introduces a `FailureHistory` map on RunState that tracks consecutive failures across replan cycles for both contract and scenario test failures. When 0002f is implemented, contract failures will also benefit from the `persistent-failure` diagnostic hint. This spec does not implement the tracking — it is additive and backward-compatible.

### In-pipeline assertion vocabulary

The E2E assertion vocabulary (`e2e/contract.go`) contains ~25 assertion types, most of which require a completed run or binary invocation. The in-pipeline contract evaluator is a **new component** (not a reuse of `e2e/runner.go`) that shares 2 assertion types with the E2E vocabulary (`file_contains`, `file_not_modified`) and introduces 3 new types (`file_exists`, `file_not_exists`, `file_not_contains`) that are only meaningful in-pipeline:

| Assertion | What it checks |
|-----------|---------------|
| `file_exists` | File exists at the given path in the worktree |
| `file_contains` | File at path contains literal substring (`strings.Contains`), matching the E2E harness (`e2e/runner.go`) behavior. |
| `file_not_modified` | File has not been changed from git HEAD in the worktree (checked via `git diff --name-only HEAD -- <path>`, matching the E2E harness implementation in `e2e/runner.go`). Assumes Execute's changes are uncommitted in the worktree at Validate time. |
| `file_not_exists` | File does NOT exist at the given path in the worktree |
| `file_not_contains` | File at path does NOT contain literal substring (`strings.Contains` negated). Useful for verifying removal of deprecated code. |

The vocabulary contains 5 assertion types, intentionally minimal but sufficient to verify the core motivating cases: function existence, file creation, unintended modifications, expected file absence, and removal of deprecated code. The E2E assertions for run state (`status`, `validation_pass`), events, evidence, and CLI output are not valid here — they require a completed run. If additional assertion types are needed (e.g., `dir_exists`), they should be added explicitly to this vocabulary with an in-pipeline evaluation implementation.

Each `ContractAssertion` must have exactly one field set. Assertions with zero or multiple fields set are treated as vocabulary violations (retry, then blocked).

WriteContracts validates generated assertions against this vocabulary. Unknown assertion keys cause up to 2 retries (3 total attempts), then `blocked`.
**Failure message format contract:** Contract failures MUST use the format `contract:<scenario-name> — <assertion-type> failed: <details>` so that FailureHistory key extraction (splitting on ` — ` and taking the first segment) works correctly. This format is shared with spec 0002f's persistent failure tracking.

**Assumption dependency:** The `file_not_modified` assertion relies on Execute not committing its changes to git. If a future change auto-commits after Execute, this assertion type must be updated to compare against the pre-Execute commit SHA rather than HEAD.

### Key types

These types live in `internal/next/contract/`. The `SpecScenario` type and `ParseScenarios` function are exported for reuse by spec 0002f's WriteScenarioTests stage.

```go
// SpecScenario represents a single scenario parsed from the spec's Scenarios section.
// Extracted from spec markdown by matching "### Scenario:" headers and Given/When/Then blocks.
type SpecScenario struct {
    Name  string // Scenario title (from ### header, minus "Scenario: " prefix)
    Given string // Given block text
    When  string // When block text
    Then  string // Then block text
    Notes string // Notes block text (optional)
}

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
// Single-key map — exactly one field must be set per assertion.
type ContractAssertion struct {
    FileExists      string                 `yaml:"file_exists,omitempty"`
    FileContains    *FileContainsAssertion `yaml:"file_contains,omitempty"`
    FileNotModified string                 `yaml:"file_not_modified,omitempty"`
    FileNotExists   string                 `yaml:"file_not_exists,omitempty"`
    FileNotContains *FileContainsAssertion `yaml:"file_not_contains,omitempty"`
}

type FileContainsAssertion struct {
    Path    string `yaml:"path"`
    Pattern string `yaml:"pattern"` // Literal substring, matched via strings.Contains
}

// ContractFailure represents a single failed contract assertion.
type ContractFailure struct {
    ScenarioName  string // e.g., "subtract-works"
    AssertionType string // e.g., "file_contains"
    Details       string // Human-readable failure description
}
```

### Example contract YAML

```yaml
scenarios:
  - name: add-function-works
    assertions:
      - file_exists: "calc/calc.go"
      - file_contains:
          path: "calc/calc.go"
          pattern: "func Add"
  - name: subtract-function-works
    assertions:
      - file_contains:
          path: "calc/calc.go"
          pattern: "func Subtract"
```

### Stage interfaces

```go
// ContractWriter — injected into WriteContractsStage (matches PlanCreator/ReviewRunner pattern)
type ContractWriter interface {
    WriteContracts(ctx context.Context, scenarios []SpecScenario, specPacket string) (*ScenarioContract, error)
}

// ContractEvaluator — injected into ValidateStage for contract assertion checking
type ContractEvaluator interface {
    Evaluate(ctx context.Context, contract *ScenarioContract, workDir string) ([]ContractFailure, error)
}
```

### RunState additions

```go
// Added to runstore.RunState for idempotency tracking.
// IMPORTANT: This flag must NOT be added to the per-cycle reset block in
// specloop.go (which resets FinalValidationPassed, FinalReviewPassed, etc.).
// It intentionally persists across replan cycles to prevent regeneration.
ContractsWritten bool `json:"contracts_written"`
```

### ValidateStageConfig addition

```go
// Added to ValidateStageConfig for contract file location
EvidenceDir string
```

### WriteContractsStageConfig

```go
// WriteContractsStageConfig — provides access to the spec file, spec packet, and evidence directory.
// Follows the pattern established by PlanStage (which reads spec-packet.md via Store).
type WriteContractsStageConfig struct {
    Store       runstore.Store // For reading spec-packet.md from the run directory
    SpecPath    string         // Path to the raw spec markdown file (for scenario parsing)
    EvidenceDir string         // Where to write scenario-contracts.yaml
}
```

### Pipeline wiring

`BuildStages` in `cmd/gromit-next/stage_provider.go` must be updated to construct and insert the WriteContracts stage at the correct position:

```go
return []specloop.Stage{
    initStage,
    compileStage,
    planStage,
    writeContractsStage,    // NEW — after plan, before execute
    executeStage,
    // WriteScenarioTests will be added here by spec 0002f
    validateStage,
    reviewStage,
    acceptStage,
    evidenceStage,
    finalizeStage,
}, nil
```

The `dryRunStages` map in `cmd/gromit-next/exec.go` should NOT include `WriteContracts` — dry-run stops after Plan, and contracts are a pre-execution artifact that depends on a finalized plan.

This includes creating an LLM adapter for `ContractWriter` and providing a noop implementation for testing. The noop implementation (`noopContractWriter`) lives in `cmd/gromit-next/stage_provider.go`, following the existing pattern of `noopAcceptEvaluator` and `noopReviewRunner` in that file.

### Package placement

The contract evaluator lives at `internal/next/contract/` — consistent with sibling domain packages (`acceptor/`, `validator/`, `review/`) under `internal/next/`.

## Acceptance Criteria

1. A WriteContracts stage runs after Plan and before Execute, producing a `scenario-contracts.yaml` file in the run's evidence directory
2. The WriteContracts stage translates each spec Scenario into declarative assertions using only the in-pipeline assertion vocabulary (`file_exists`, `file_contains`, `file_not_modified`, `file_not_exists`, `file_not_contains`)
3. Generated contract assertions are validated against the known vocabulary; unknown assertion keys trigger a retry, then `blocked`
4. All scenarios are processed in a single LLM invocation during contract writing (batch, not sequential)
5. The Validate stage checks contract assertions via an injected `ContractEvaluator` interface (not a `validator.Check`), using `EvidenceDir` from `ValidateStageConfig` to locate the contract file
6. Contract assertion failures produce failure context identifying the scenario name and failed assertion
7. Contract failures trigger replan via the existing `replan_from` mechanism
8. On replan cycles, WriteContracts is a no-op when its RunState flag (`ContractsWritten`) is true — fix tasks target the implementation, not the contracts. WriteContracts sets `rs.ContractsWritten = true` on success.
9. If WriteContracts produces unparseable YAML or invalid assertions after two retries, the stage returns `blocked`
10. If the spec has no Scenarios section or it is empty, WriteContracts is a no-op (returns `Continue`) and Validate skips contract checking
11. If `scenario-contracts.yaml` does not exist in `EvidenceDir` at Validate time, contract checking is skipped silently (not a failure)
12. The WriteContracts stage uses an injected `ContractWriter` interface matching the existing `PlanCreator`/`ReviewRunner` pattern for testability
13. `BuildStages` in `cmd/gromit-next/stage_provider.go` is updated to include WriteContracts in the correct pipeline position
14. `ContractsWritten` RunState flag is NOT reset in the per-cycle reset block in `specloop.go`
15. WriteContracts uses Sonnet (P1) model tier
16. All existing pipeline tests continue to pass
17. The `SpecScenario` type and `ParseScenarios` function are exported from `internal/next/contract/` for reuse by downstream specs (specifically 0002f)
18. WriteContracts emits events: `contracts_written` on success (with scenario count), `contracts_blocked` on terminal failure
19. The in-pipeline assertion vocabulary includes `file_not_exists` and `file_not_contains` in addition to `file_exists`, `file_contains`, and `file_not_modified`
20. `file_contains` and `file_not_contains` use literal substring matching (`strings.Contains`), matching the E2E harness behavior
21. Contract failure messages use the format `contract:<scenario-name> — <assertion-type> failed: <details>` — this format is a shared contract with spec 0002f's FailureHistory key extraction

## Scenarios

### Scenario: Happy path — contracts pass

**Given:** A spec with 2 scenarios (e.g., "Add function works" and "Subtract function works"), a working fixture repo, and a plan that correctly addresses both scenarios
**When:** The pipeline runs through all stages
**Then:**
- WriteContracts produces `scenario-contracts.yaml` with assertions for both scenarios (e.g., `file_contains: "func Add"`, `file_contains: "func Subtract"`)
- Execute implements the spec
- Validate checks contract assertions — all pass
- Pipeline continues to Review and Accept without replanning
**Notes:** This is the baseline success case. Scenario test writing (spec 0002f) is not yet wired in; this validates contracts alone through the pipeline.

### Scenario: Contract assertion fails, triggers replan

**Given:** A spec with a scenario "Subtract function works", and an executor that implements Add but forgets to wire up Subtract
**When:** Validate checks the contract assertion `file_contains: {path: calc/calc.go, pattern: "func Subtract"}`
**Then:**
- The assertion fails
- Validate returns `replan_from` with failure context: `"contract:subtract-works — file_contains failed: pattern 'func Subtract' not found in calc/calc.go"`
- The planner receives this failure and produces a fix task targeting the missing Subtract function
- On the next cycle, the contract assertion passes
**Notes:** This is the exact "written but never wired up" failure that motivated this spec

### Scenario: WriteContracts produces invalid YAML

**Given:** A spec with scenarios, and the LLM produces malformed YAML for the contract file
**When:** WriteContracts attempts to parse the output
**Then:**
- Parse fails, stage retries up to twice with error context
- If retries also produce invalid YAML, the stage returns `blocked`
- The run terminates with `blocked` status — no Execute, no replan
**Notes:** This is infrastructure failure, not a spec implementation problem

### Scenario: Spec has no scenarios section

**Given:** A spec with no Scenarios section (or an empty one)
**When:** The pipeline reaches WriteContracts
**Then:**
- WriteContracts detects no scenarios, returns `Continue` with no output file
- Execute runs normally
- Validate runs existing checks only (no contract file to parse)
- Pipeline continues to Review and Accept
**Notes:** Specs without scenarios still flow through the pipeline as they do today

### Scenario: Contract file missing at Validate time

**Given:** A spec where WriteContracts returned `blocked` (e.g., due to repeated invalid YAML), so `scenario-contracts.yaml` was never written to the evidence directory. Due to a bug or race condition, the pipeline continues to Validate despite the block.
**When:** Validate runs and attempts to check contract assertions
**Then:**
- ContractEvaluator looks for `scenario-contracts.yaml` in `EvidenceDir`
- The file does not exist
- Contract checking is skipped silently (not treated as a failure)
- Validate continues with remaining checks (go test, go vet, etc.)
**Notes:** This ensures the pipeline degrades gracefully when contract generation fails

### Scenario: Replan preserves contracts

**Given:** A spec where Execute produces an implementation that fails a contract assertion, and WriteContracts has already produced its artifact
**When:** Validate triggers replan and the pipeline re-executes from the configured ReplanStage (plan)
**Then:**
- Plan produces fix tasks based on the failure context
- WriteContracts detects `rs.ContractsWritten == true`, returns `Continue` (no-op)
- Execute runs the fix tasks
- Validate re-runs — contract assertions now pass
**Notes:** This verifies the idempotency guarantee that protects contract artifacts across replan cycles

### Scenario: WriteContracts produces valid YAML with unknown assertion key

**Given:** A spec with scenarios, and the LLM produces structurally valid YAML but uses an assertion key outside the vocabulary (e.g., `status: completed`)
**When:** WriteContracts validates the generated assertions against the known vocabulary
**Then:**
- Vocabulary validation fails (distinct from YAML parse failure)
- Stage retries up to 2 times (3 total attempts: 1 initial + 2 retries) with the vocabulary violation as error context (listing the valid keys: `file_exists`, `file_contains`, `file_not_modified`, `file_not_exists`, `file_not_contains`)
- If retries also produce unknown keys, the stage returns `blocked`
- The run terminates with `blocked` status
**Notes:** This is a separate code path from YAML parse failure — the YAML is valid but semantically invalid

### Scenario: Contract uses file_not_exists and file_not_contains assertions

**Given:** A spec with a scenario "Legacy cleanup removes deprecated module", where the implementation should delete `legacy/old.go` and remove all `//go:deprecated` annotations from `calc/calc.go`
**When:** WriteContracts generates contract assertions and Validate checks them
**Then:**
- WriteContracts produces assertions including `file_not_exists: "legacy/old.go"` and `file_not_contains: {path: "calc/calc.go", pattern: "go:deprecated"}`
- If the implementation correctly deletes the file and removes annotations, both assertions pass
- If `legacy/old.go` still exists or `calc/calc.go` still contains `go:deprecated`, the assertion fails and triggers replan
**Notes:** Exercises the `file_not_exists` and `file_not_contains` vocabulary (AC19)

### Scenario: WriteContracts blocked, pipeline continues without contracts (illustrative)

**Given:** A spec with scenarios, but the LLM repeatedly produces invalid YAML for contract generation, causing WriteContracts to return `blocked`
**When:** In a subsequent run where WriteContracts is skipped (e.g., via operator configuration or a future skip-on-prior-block feature), Execute proceeds without a contract file
**Then:**
- Execute runs normally
- WriteScenarioTests (spec 0002f) runs normally — it does not depend on contract artifacts
- Validate skips contract checking (no `scenario-contracts.yaml` file exists)
- Validate runs `go test ./...` which includes any scenario tests written by 0002f
- Pipeline continues with only scenario test coverage, no contract coverage
**Notes:** This scenario is illustrative — it describes desired degradation behavior for future work. The skip-on-prior-block feature is not implemented by this spec. The two verification mechanisms (contracts and scenario tests) degrade independently.

### Scenario: WriteContracts blocked by budget exhaustion

**Given:** A spec with scenarios, but the budget is exhausted before the WriteContracts LLM invocation
**When:** WriteContracts checks budget before invoking the LLM
**Then:**
- The stage detects budget is exhausted
- Returns `blocked` with a budget-exhausted message
- No `scenario-contracts.yaml` file is written
- Validate skips contract checking (no contract file)
**Notes:** Budget checking prevents runaway costs. The single-invocation nature of WriteContracts means budget is checked once, unlike the per-scenario checking in spec 0002f.

### Scenario: Multiple assertions for one scenario with partial failure

**Given:** A spec with a scenario "Calculator module exists" that generates two contract assertions: `file_exists: "calc/calc.go"` and `file_contains: {path: "calc/calc.go", pattern: "func Multiply"}`. The implementation creates calc.go with Add and Subtract but not Multiply.
**When:** Validate checks the contract assertions for this scenario
**Then:**
- `file_exists` passes (calc.go exists)
- `file_contains` fails (pattern "func Multiply" not found)
- Both results are evaluated — all assertions are checked, not short-circuited on first failure
- The failure context includes only the failed assertion: `"contract:calculator-module-exists — file_contains failed: pattern 'func Multiply' not found in calc/calc.go"`
- Replan is triggered with this failure context
**Notes:** Verifies that the evaluator checks all assertions per scenario and collects all failures, rather than stopping at the first failure.

## Validation

```bash
# All unit and scenario tests
go test ./... -count=1

# WriteContracts stage (unit tests with mocked interfaces)
go test ./internal/next/specloop/stages/ -count=1 -run TestWriteContracts

# Validate stage extension (including contract evaluation delegation)
go test ./internal/next/specloop/stages/ -count=1 -run TestValidate

# Contract evaluator (unit tests for each assertion type + edge cases:
# nonexistent file paths, empty assertions list, etc.)
go test ./internal/next/contract/ -count=1

# Integration test — exercises Plan → WriteContracts → Execute → Validate with replan
go test ./internal/next/specloop/ -count=1 -run TestIntegration

# Vet
go vet ./...
```
