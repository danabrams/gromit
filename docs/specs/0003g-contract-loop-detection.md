# Spec 0003g — Contract Loop Detection

## spec_id
0003g-contract-loop-detection

## Vision
Gromit can enter an infinite replan loop when a contract assertion fails for a reason the implementation cannot fix — most commonly a wrong file path in the generated contract. Each cycle ends in the same failure, a new replan fires, and the cycle count climbs until budget is exhausted. The system has no memory of having seen this exact failure before.

## Summary
When the validate stage produces the same set of contract failures on two consecutive cycles, the run transitions to `needs_human` instead of triggering another replan. The validate stage compares the current contract failure strings against `rs.LastContractFailures` stored from the previous cycle. On a match, it emits a `needs_human` action with a clear reason. On any difference (new failures, resolved failures, or first occurrence), it updates `rs.LastContractFailures` and proceeds normally.

## Goals
### Primary
- Detect repeated identical contract failures across consecutive validation cycles and escalate to `needs_human`
- Store last contract failure set on `RunState` so it survives cycle boundaries
- Emit a human-readable reason identifying which contract is looping

## Non-goals
- Shell check failures (always-run, project checks) — not tracked for loop detection; contract loops are the observed problem
- Fuzzy matching of "similar" failures — exact string equality only
- Auto-correcting the contract (deferred to spec 0003h)

## Architecture

### RunState
One new field in `internal/next/runstore/types.go`:

```go
type RunState struct {
    // ... existing fields ...
    LastContractFailures []string `json:"last_contract_failures,omitempty"`
}
```

Added to `NormalizeNilFields()`. NOT reset between cycles (like `TaskLineage` and `ScenarioTestsWritten`) — must survive the cycle boundary to detect consecutive failures.

### Validate Stage
In `internal/next/specloop/stages/validate.go`, after collecting `contractFailures`:

```go
if len(contractFailures) > 0 && slicesEqual(contractFailures, rs.LastContractFailures) {
    return specloop.NextAction{
        Kind: specloop.NeedsHuman,
        Context: &specloop.FailureContext{
            Failures: append([]string{"repeated contract failures — same failures on consecutive cycles:"}, contractFailures...),
            Cycle:    rs.Cycle,
        },
    }, nil
}
rs.LastContractFailures = contractFailures
```

`slicesEqual` is a small unexported helper — same length, same elements in same order. Order is deterministic because contract assertions are evaluated in YAML order.

## Acceptance Criteria
1. When the validate stage produces the same non-empty set of contract failure strings on two consecutive cycles, the run transitions to `needs_human` and does not trigger a replan
2. When contract failures differ between consecutive cycles, the run replans as normal
3. When contract failures are absent, `LastContractFailures` is set to empty and loop detection does not fire
4. `LastContractFailures` is persisted to `run.json` and survives across cycle boundaries
5. `LastContractFailures` is NOT reset by the store's cycle-reset logic (alongside `TaskLineage`, `ScenarioTestsWritten`)
6. The `needs_human` reason message identifies the repeated failures explicitly
7. Shell check failures do not contribute to loop detection
8. All existing validate stage tests continue to pass

## Scenarios

### Scenario: same contract failure on consecutive cycles escalates to needs_human
**Given:** A run on cycle 2. `rs.LastContractFailures` contains `["contract:spec picker — file_contains failed: pattern \"feature/foo\" not found in \"cmd/gromit-next/exec_test.go\""]`
**When:** The validate stage runs and produces the identical contract failure string
**Then:** The stage returns `NeedsHuman`. The failure context includes `"repeated contract failures — same failures on consecutive cycles:"` followed by the failure string. `rs.LastContractFailures` is unchanged.

### Scenario: different contract failure on second cycle replans normally
**Given:** A run on cycle 2. `rs.LastContractFailures` contains a failure for `feature/foo`.
**When:** The validate stage runs and produces a different contract failure string
**Then:** The stage returns `ReplanFrom`. `rs.LastContractFailures` is updated to the new failure set.

### Scenario: contract failure resolves on second cycle
**Given:** A run on cycle 2. `rs.LastContractFailures` has one entry from cycle 1.
**When:** The validate stage runs and finds no contract failures and shell checks pass
**Then:** Validation passes. `rs.LastContractFailures` is set to empty.

### Scenario: first-cycle contract failure replans normally
**Given:** A fresh run on cycle 1. `rs.LastContractFailures` is empty.
**When:** The validate stage runs and finds a contract failure
**Then:** The stage returns `ReplanFrom`. `rs.LastContractFailures` is set to the failure strings.

## Validation
- `go test ./internal/next/specloop/stages/... -count=1 -timeout 60s`
- `go test ./internal/next/runstore/... -count=1 -timeout 60s`
- `go vet ./...`
