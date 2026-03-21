# Spec 0004f-remediation — Contract Specificity Cleanup

## spec_id
0004f-remediation

## Depends on
0004f-contract-specificity-validation

## Summary

Cleanup items from the 0004f review: promote a compiled regex to package level, extract a duplicated format helper, deduplicate test files (keeping scenario-named tests as canonical, but retaining tests with no scenario counterpart), fix a test that exercises the wrong code path, tighten a loose assertion, and add a missing content assertion on the retry prompt.

## Goals

### Primary
- Fix all non-blocking review findings from the 0004f acceptance review
- Reduce test maintenance burden by removing duplicate test suites where scenario equivalents exist

## Non-goals
- Changing any behavior — this is purely cleanup, no functional changes
- Adding new features or capabilities to specificity validation
- Budget checking before specificity retry (deferred — would change behavior)

## Architecture

All changes are localized refactors within existing files. No new files, types, or interfaces.

### `internal/next/contract/specificity.go`
- Promote `regexp.MustCompile(...)` from inside `ValidateContractSpecificity` to a package-level `var exportedIdentifierRegex`

### `internal/next/specloop/stages/write_contracts.go`
- Extract `func specificityWarningString(w contract.SpecificityWarning) string` helper to replace the duplicated `fmt.Sprintf` format string and loop pattern used at both the retry-prompt construction block and the event-emission block
- Add clarifying comment above `result = preRetryResult` on the structural-regression branch (inside the `if len(retryValidationErrors) > 0` block) explaining that `specificityWarnings` still describes the restored pre-retry result
- Remove unreachable `if result != nil` guard at the start of the specificity phase (dead code — the nil case returns earlier in both the nil+nil early return and the terminal failure block)

### Test files — `internal/next/contract/`
- Remove `specificity_test.go` — 5 of its 6 tests are duplicated by the 5 `specificity_scenario_*_test.go` files. The 6th test (`TestValidateContractSpecificity_PunctuationPatternNoWarning`) has no scenario counterpart — extract it to a new `specificity_scenario_punctuation_test.go` file before deleting `specificity_test.go`

### Test files — `internal/next/specloop/stages/`
- Remove the 3 `TestWriteContracts_Specificity*` tests that have scenario-file counterparts:
  - `TestWriteContracts_SpecificityNoWarningsNoRetry` (covered by `write_contracts_scenario_high_specificity_test.go`)
  - `TestWriteContracts_SpecificityRetryFixesPattern` (covered by `stages_scenario_specificity_retry_test.go`)
  - `TestWriteContracts_SpecificityRetryPersistsWarning` (covered by `write_contracts_scenario_specificity_persists_test.go`)
- **Keep** `TestWriteContracts_SpecificityRetryStructuralRegression` and `TestWriteContracts_SpecificityRetryLLMError` — these have no scenario counterparts and would lose coverage if deleted
- Fix `SpecificityRetryStructuralRegression`: replace `&contract.ScenarioContract{}` with a non-empty contract that fails `ValidateContract` (e.g., a `ScenarioContract` with one scenario containing a `ContractAssertion{}` where all fields are nil — this has `len(Scenarios) > 0` so it enters the structural regression branch, and `ValidateContract` returns errors because no assertion type is set). Also tighten its `writerCalls < 2` assertion to `writerCalls != 2`
- Add `strings.Contains(retrySpecPacket, "Specificity")` assertion to `stages_scenario_specificity_retry_test.go` if not already present

## Acceptance Criteria

1. The `exportedIdentifierRegex` is compiled once at package level in `specificity.go`, not inside `ValidateContractSpecificity`
2. A `specificityWarningString` helper exists and is called from both the retry-prompt and event-emission blocks in `write_contracts.go`
3. The dead-code `if result != nil` guard is removed from the specificity phase in `write_contracts.go`
4. A comment on the structural-regression fallback path (inside `if len(retryValidationErrors) > 0`) explains that `specificityWarnings` describes the restored pre-retry result
5. `specificity_test.go` is deleted; the punctuation-pattern test is extracted to a `specificity_scenario_*_test.go` file; all `specificity_scenario_*_test.go` files pass and cover the same behaviors as the deleted file
6. The 3 duplicate `TestWriteContracts_Specificity*` tests with scenario counterparts are deleted from `write_contracts_test.go`; `StructuralRegression` and `LLMError` tests are kept
7. The structural regression test uses a non-empty contract (with at least one scenario containing a `ContractAssertion{}` with no assertion type set) that fails `ValidateContract`, exercising the actual regression detection branch (not the empty-result branch)
8. Both the `StructuralRegression` and any remaining persists-warning tests use `writerCalls != 2` (exactly 2), not `writerCalls < 2`
9. The retry-fixes scenario test asserts that the retry spec packet contains specificity warning text (e.g., `strings.Contains(retrySpecPacket, "Specificity")`)
10. All existing tests continue to pass

## Scenarios

### Scenario: regex compiled once at package level
**Given:** `exportedIdentifierRegex` is a `var` at package scope in `specificity.go`
**When:** `ValidateContractSpecificity` is called twice in a test
**Then:** Both calls succeed, and `go vet` reports no issues

### Scenario: duplicate test files removed without losing coverage
**Given:** `specificity_test.go` is deleted from `internal/next/contract/`, the punctuation test is extracted to `specificity_scenario_punctuation_test.go`, and the 3 duplicate `TestWriteContracts_Specificity*` tests (NoWarnings, RetryFixes, RetryPersists) are deleted from `write_contracts_test.go`
**When:** `go test ./internal/next/contract/... ./internal/next/specloop/stages/...` runs
**Then:** All remaining tests pass; `specificity_test.go` no longer exists; all behaviors from the deleted file have scenario-named counterparts; `StructuralRegression` and `LLMError` tests still exist and pass in `stages/`

### Scenario: structural regression test exercises correct branch
**Given:** The `SpecificityRetryStructuralRegression` test provides a `ScenarioContract` with one scenario containing a `ContractAssertion{}` with no assertion type set
**When:** The specificity retry returns this contract
**Then:** `len(retryResult.Scenarios) > 0` is true, `ValidateContract` returns errors, and the stage falls back to `preRetryResult` via the structural regression branch

### Scenario: helper function deduplicates warning formatting
**Given:** `specificityWarningString` is defined in `write_contracts.go`
**When:** Both the retry-prompt block and the event-emission block call it
**Then:** The `fmt.Sprintf` format string appears only once (in the helper), and both blocks produce identical output for the same warning

## Validation

### Automatic
- `go test ./internal/next/contract/... -count=1 -timeout 60s`
- `go test ./internal/next/specloop/stages/... -count=1 -timeout 60s`
- `go vet ./...`
