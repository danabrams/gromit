# Spec 0004t — go_test_pass Contract Assertion Type

## spec_id
go-test-pass-contract-assertion

## Vision
When the contract writer translates spec scenarios into assertions, it reaches for `file_contains` —
the only tool that can encode behavior. But `file_contains` doesn't verify behavior; it verifies
syntax. A valid implementation using constants instead of literals, a renamed variable, or an
extracted helper all fail the contract while satisfying the spec. The pipeline then burns cycles
trying to make correct code match a brittle pattern, eventually escalating to `needs_human`. The
contract system needs an assertion type that verifies what the code *does*, not how it *looks*. Go
tests already provide this — the contract just needs to run them.

## Summary
Add a `go_test_pass` contract assertion type that verifies scenario behavior by running a named Go
test. Contracts are generated in two passes: the existing LLM-driven pass writes structural
assertions (`file_exists`, `file_not_exists`, `file_not_modified`), then a second automatic pass
scans scenario test files and generates individual `go_test_pass` assertions for each test function.
When a scenario has `go_test_pass` coverage, `file_contains` assertions for that scenario are
dropped.

## Goals
### Primary
- `go_test_pass` assertion type in the contract evaluator that runs a specified `go test -run`
  command and passes/fails based on exit code
- Second-pass contract generation that auto-discovers scenario test files and emits `go_test_pass`
  assertions per test function
- When `go_test_pass` assertions exist for a scenario, `file_contains` assertions for that scenario
  are replaced, not supplemented

### Secondary
- `go_test_pass` failures include the test's stdout/stderr in the contract failure details (so the
  replan stage has diagnostic context)

## Non-goals
- Changing stage ordering (second pass runs after scenario tests are written, which is already after
  contract writing)
- Classifying `go_test_pass` failures differently from other contract failures (deferred — all
  failures flow through existing replan path)
- Removing `file_contains` as an assertion type (it remains for structural checks and scenarios
  without test coverage)
- Supporting non-Go test runners

## Architecture
**New assertion type (`internal/next/contract/types.go`):**

```go
type ContractAssertion struct {
    FileExists      string              `yaml:"file_exists,omitempty"`
    FileNotExists   string              `yaml:"file_not_exists,omitempty"`
    FileContains    *FileContainsAssert `yaml:"file_contains,omitempty"`
    FileNotContains *FileContainsAssert `yaml:"file_not_contains,omitempty"`
    FileNotModified string              `yaml:"file_not_modified,omitempty"`
    GoTestPass      *GoTestPassAssert   `yaml:"go_test_pass,omitempty"` // NEW
}

type GoTestPassAssert struct {
    Pkg      string `yaml:"pkg"`       // e.g. "./internal/next/planner/..."
    TestName string `yaml:"test_name"` // e.g. "TestScenario_PlanStage_ReworkResume"
}
```

**Evaluator (`internal/next/contract/evaluator.go`):**

When evaluating a `go_test_pass` assertion, run:
```
go test <pkg> -run ^<test_name>$ -count=1 -timeout 60s
```
Pass if exit code 0, fail otherwise. Capture combined stdout+stderr as the failure detail (truncated
to a reasonable limit like 2000 chars).

**Second-pass generation (`internal/next/contract/augment.go` — new file):**

```go
// AugmentWithTestAssertions scans scenario test files in the worktree,
// matches them to existing scenario contracts by name, and replaces
// file_contains assertions with go_test_pass assertions for matched scenarios.
func AugmentWithTestAssertions(contract *ScenarioContract, workDir string) error
```

Logic:
1. Glob for `*_scenario_*_test.go` files in the worktree
2. Parse each file to extract test function names (`func Test...`)
3. Match test functions to scenario names using normalized string similarity (strip underscores,
   lowercase)
4. For each matched scenario: drop `file_contains` and `file_not_contains` assertions, add
   `go_test_pass` per matched test function
5. Retain `file_exists`, `file_not_exists`, `file_not_modified` unchanged

**Integration point (`internal/next/specloop/stages/validate.go`):**

Call `AugmentWithTestAssertions` after loading the contract YAML but before evaluation. This runs
every validation cycle, so if new scenario tests appear in later cycles, they get picked up
automatically.

**No changes to:** contract writer stage, contract writer prompt, scenario test writer, replan
logic, failure reporting format.

## Acceptance Criteria
1. The contract evaluator supports a `go_test_pass` assertion type that runs
   `go test <pkg> -run ^<test_name>$ -count=1` and passes when exit code is 0.
2. When a `go_test_pass` assertion fails, the contract failure detail includes the test's
   stdout/stderr output (truncated to 2000 chars).
3. `AugmentWithTestAssertions` discovers `*_scenario_*_test.go` files in the worktree, extracts
   test function names, and matches them to scenario contracts.
4. When a scenario has at least one `go_test_pass` assertion after augmentation, all `file_contains`
   and `file_not_contains` assertions for that scenario are removed.
5. `file_exists`, `file_not_exists`, and `file_not_modified` assertions are never removed by
   augmentation.
6. Augmentation runs before contract evaluation in the validate stage, so newly written scenario
   tests are picked up automatically.
7. When no scenario test files exist (cycle 1 before tests are written), augmentation is a no-op and
   existing contract assertions are preserved.
8. All existing contract evaluator and validate-stage tests continue to pass.

## Scenarios

### Scenario: Behavioral contract replaces brittle file_contains
**Given:** A scenario contract with `file_contains` assertions checking for `"rework_vision_change"`
in `plan.go`, and a scenario test file `plan_scenario_vision_change_resume_test.go` containing
`func TestScenario_VisionChangeResume`
**When:** `AugmentWithTestAssertions` runs before validation
**Then:** The `file_contains` assertions for that scenario are replaced with a single `go_test_pass`
assertion: `{pkg: "./internal/next/specloop/stages/...", test_name: "TestScenario_VisionChangeResume"}`

### Scenario: go_test_pass assertion passes on green test
**Given:** A `go_test_pass` assertion referencing `TestScenario_Foo` in `./pkg/...`, and that test
passes
**When:** The contract evaluator evaluates the assertion
**Then:** The assertion passes, no contract failure is recorded

### Scenario: go_test_pass assertion fails with diagnostic output
**Given:** A `go_test_pass` assertion referencing `TestScenario_Bar` in `./pkg/...`, and that test
fails with output `"expected 'Reviewer Instructions' section, got empty prompt"`
**When:** The contract evaluator evaluates the assertion
**Then:** A contract failure is recorded with detail containing the test's failure output

### Scenario: No scenario tests yet (cycle 1)
**Given:** A scenario contract with `file_contains` assertions, and no `*_scenario_*_test.go` files
exist in the worktree
**When:** `AugmentWithTestAssertions` runs
**Then:** The contract is unchanged — all original assertions are preserved

### Scenario: Mixed scenarios — some with tests, some without
**Given:** Two scenarios: "Rework resume" has a matching scenario test file, "Edge case timeout"
does not
**When:** `AugmentWithTestAssertions` runs
**Then:** "Rework resume" gets `go_test_pass` (file_contains dropped). "Edge case timeout" retains
its original `file_contains` assertions.

### Scenario: Structural assertions survive augmentation
**Given:** A scenario with `file_exists: "internal/next/planner/types.go"` and
`file_contains: {path: plan.go, pattern: "rework_vision_change"}`, and a matching scenario test
exists
**When:** `AugmentWithTestAssertions` runs
**Then:** `file_exists` is retained, `file_contains` is replaced with `go_test_pass`

## Validation
```
go test ./internal/next/contract/... -count=1
go test ./internal/next/specloop/stages/... -count=1
go vet ./internal/next/contract/... ./internal/next/specloop/stages/...
go build ./...
```
