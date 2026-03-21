# Spec 0004f — Contract Specificity Validation

## spec_id
0004f-contract-specificity-validation

## Depends on
- 0003g-contract-loop-detection
- 0003h-contract-file-self-correction

## Vision

Review findings can contradict contract assertions, causing infinite replan loops. A real example: a contract asserts `file_contains: {path: types.go, pattern: ModelTier}` — satisfied by `DistillationResult.ModelTier`. A review finding says "remove ModelTier from DistillerInputs" (different struct, same file). Fix tasks are coarse-grained and sometimes rewrite the whole file, dropping ModelTier from DistillationResult too. The contract fails, replan adds it back, review flags it again — infinite loop.

Layer 1 (0003g) detects repeated identical failures and bails to needs_human. Layer 2 (runtime review-contract contradiction suppression) filters contradictory review findings at runtime. This spec is Layer 3: preventing the problem at the source by catching ambiguous contracts before they cause thrash.

Single-identifier patterns like `ModelTier` are ambiguous when a file contains multiple type definitions. A pattern like `ModelTier  string` or a separate assertion for the containing struct is unambiguous. This spec adds a specificity validation pass to the WriteContracts stage that flags low-specificity patterns and gives the LLM one chance to fix them.

## Summary

After the LLM generates `scenario-contracts.yaml` in the WriteContracts stage, a new `ValidateContractSpecificity()` function scores each `file_contains` pattern for ambiguity risk. Single Go identifier patterns (one word matching `^[A-Z][a-zA-Z0-9]*$`) are flagged as low-specificity. The validation produces warnings — not hard rejections — and feeds them back to the LLM for one retry attempt. The WriteContracts prompt is also updated with instructions to prefer struct-scoped patterns when multiple types share a file.

## Goals

### Primary
- Detect low-specificity `file_contains` patterns that are likely to cause review-contract contradiction thrash
- Feed specificity warnings back to the LLM with actionable guidance, allowing one retry to produce better patterns
- Add prompt instructions to the contract writer encouraging struct-scoped patterns

### Secondary
- Score patterns to distinguish low-specificity (single exported identifier) from high-specificity (multi-token, includes type context)
- Log specificity warnings as events for observability

### Tertiary
- When a contract failure occurs during validation AND the failing contract has a low-specificity pattern, auto-tighten the pattern before triggering a replan (builds on 0003h self-correction)

## Non-goals
- Cross-assertion analysis (e.g., detecting that two assertions in different scenarios contradict each other)
- Modifying how `file_not_contains` assertions are scored — only `file_contains` patterns are checked

## Architecture

### New function: `ValidateContractSpecificity`

Location: `internal/next/contract/specificity.go`

```go
package contract

// SpecificityWarning describes a low-specificity pattern in a contract assertion.
type SpecificityWarning struct {
    ScenarioName string
    AssertionIdx int
    Pattern      string
    Path         string
    Reason       string // e.g., "single exported identifier — ambiguous if file contains multiple types"
}

// ValidateContractSpecificity checks file_contains assertions for patterns
// that are likely to cause review↔contract contradiction thrash.
// Returns warnings for low-specificity patterns. An empty slice means all
// patterns are adequately specific.
func ValidateContractSpecificity(c ScenarioContract) []SpecificityWarning
```

A pattern is classified as **low-specificity** when all of the following hold:
1. It is a single token (no spaces, no punctuation other than `_`)
2. It matches an exported Go identifier: `^[A-Z][a-zA-Z0-9_]*$`
3. It does not include type context (no surrounding whitespace-separated tokens like `struct`, field type suffixes, etc.)

Multi-token patterns (e.g., `ModelTier  string`, `type DistillationResult struct`), patterns containing operators or punctuation beyond `_` (e.g., `func ValidateContract(`), and unexported identifiers (e.g., `modelTier`) are classified as **high-specificity** and not flagged.

The function does not inspect the target file — it is a static analysis of pattern text only.

### Integration into WriteContracts stage

In `internal/next/specloop/stages/write_contracts.go`, within the existing retry loop, after `ValidateContract()` passes with no errors:

```text
1. result, err = s.writer.WriteContracts(ctx, scenarios, specPacket)
2. validationErrors = contract.ValidateContract(*result)
3. if len(validationErrors) == 0:
4.     specificityWarnings = contract.ValidateContractSpecificity(*result)   // NEW
5.     if len(specificityWarnings) > 0 && !specificityCorrectionAttempted:   // NEW
6.         inject warnings into specPacket as retry context                  // NEW
7.         specificityCorrectionAttempted = true                             // NEW
8.         continue  // one more attempt                                    // NEW
9.     // If warnings remain after retry, accept the contract anyway (warn-only)
```

Key behaviors:
- Specificity validation runs only after structural validation (`ValidateContract`) passes — no point scoring patterns on a malformed contract
- At most one retry for specificity — tracked by a `specificityCorrectionAttempted` bool, not counted against the existing 3-attempt structural retry budget
- If low-specificity patterns remain after retry, the contract is accepted and an event is logged — this is a quality gate, not a hard gate
- The retry shares the same LLM call path (`s.writer.WriteContracts`) — no new interfaces needed

### Prompt additions

Add to the contract writer's system prompt (the prompt template used by the `ContractWriter` implementation):

```text
When writing file_contains assertions for files that will contain multiple
type definitions, use struct-scoped patterns instead of bare identifiers.

BAD:  file_contains: {path: types.go, pattern: ModelTier}
GOOD: file_contains: {path: types.go, pattern: "ModelTier  string"}
GOOD: file_contains: {path: types.go, pattern: "type DistillationResult struct"}

A bare exported identifier like "ModelTier" is ambiguous — it could match
any of several structs in the file, making it fragile when review findings
cause targeted edits to one struct but not another.
```

### Event type

Add to `internal/next/runstore/events.go`:

```go
type ContractSpecificityWarningEvent struct {
    BaseEvent
    Warnings []string `json:"warnings"` // human-readable warning strings
    Retried  bool     `json:"retried"`  // true if a retry was attempted
}
```

Event type string: `"contract_specificity_warning"`. Emitted once per WriteContracts invocation that encounters low-specificity patterns, after the retry decision is made.

### Runtime auto-correction of low-specificity contracts

When the validate stage encounters a contract failure for a `file_contains` assertion with a low-specificity pattern (as defined by `ValidateContractSpecificity`), and the target file exists but the pattern match failed because the implementation placed the identifier in a different struct than expected, the system should auto-tighten the contract rather than triggering a replan.

Auto-correction logic in `internal/next/specloop/stages/validate.go`, after contract evaluation:

1. For each `file_contains` failure, check if the pattern is low-specificity (single exported identifier)
2. If the file exists and contains the pattern in a different location than expected (e.g., `ModelTier` exists in `DistillationResult` but not `DistillerInputs`), the contract is satisfied — the pattern is just too broad
3. Read the file, find where the pattern actually appears, extract the surrounding struct context, and rewrite the contract pattern to be more specific (e.g., `ModelTier` → `ModelTier  string` with the struct's field signature)
4. Write the updated `scenario-contracts.yaml` and re-evaluate

This builds on 0003h (contract file self-correction) but operates on pattern specificity rather than file paths. The auto-correction is bounded: at most one rewrite per assertion per cycle, and only for patterns classified as low-specificity.

## Acceptance Criteria

1. `ValidateContractSpecificity` returns warnings for `file_contains` patterns that are single exported Go identifiers (e.g., `ModelTier`, `Proposal`, `RunState`)
2. `ValidateContractSpecificity` does not warn on multi-token patterns (e.g., `ModelTier  string`), patterns with punctuation (e.g., `func Validate(`), or unexported identifiers (e.g., `modelTier`)
3. When `ValidateContractSpecificity` returns warnings and no prior specificity retry has been attempted, the WriteContracts stage injects warning context into the prompt and retries once
4. The specificity retry does not count against the existing 3-attempt structural validation retry budget
5. If low-specificity patterns remain after the retry, the contract is accepted (warn-only) and a `contract_specificity_warning` event is emitted with `retried: true`
6. If the first attempt has no specificity warnings, no retry occurs and no event is emitted
7. The contract writer prompt includes instructions to prefer struct-scoped patterns over bare identifiers
8. `file_not_contains` assertions are not checked by specificity validation
9. All existing WriteContracts and ValidateContract tests continue to pass
10. `ValidateContractSpecificity` is a pure function — it does not read files or invoke the LLM
11. When a contract failure has a low-specificity pattern and the pattern exists elsewhere in the target file, the validate stage auto-tightens the pattern and re-evaluates instead of replanning
12. Auto-tightening is bounded to one rewrite per assertion per cycle
13. Auto-tightened contracts are persisted to `scenario-contracts.yaml` so subsequent cycles use the improved version

## Scenarios

### Scenario: single-identifier pattern triggers specificity warning
**Given:** The LLM generates a contract with `file_contains: {path: internal/next/reviewdistiller/types.go, pattern: ModelTier}`
**When:** `ValidateContractSpecificity` runs on the contract
**Then:** A warning is returned for that assertion with reason indicating the pattern is a single exported identifier

### Scenario: struct-scoped pattern passes specificity check
**Given:** The LLM generates a contract with `file_contains: {path: internal/next/reviewdistiller/types.go, pattern: "ModelTier  string"}`
**When:** `ValidateContractSpecificity` runs on the contract
**Then:** No warning is returned for that assertion

### Scenario: low-specificity pattern triggers one retry in WriteContracts
**Given:** A spec with scenarios that produce contracts. The LLM's first attempt generates a `file_contains` pattern `ModelTier` (single identifier).
**When:** The WriteContracts stage runs
**Then:** After structural validation passes, specificity validation flags the pattern. The stage injects the warning into the prompt and calls the LLM once more. If the second attempt produces `ModelTier  string`, the contract is accepted with no specificity event.

### Scenario: low-specificity pattern persists after retry — accepted with warning
**Given:** A spec with scenarios that produce contracts. The LLM generates `ModelTier` as the pattern on both the first attempt and the specificity retry.
**When:** The WriteContracts stage runs
**Then:** The contract is accepted despite the low-specificity pattern. A `contract_specificity_warning` event is emitted with `retried: true` and the warning text. The stage returns `Continue`.

### Scenario: multiple low-specificity patterns in one contract
**Given:** The LLM generates a contract with three `file_contains` assertions: `ModelTier`, `Proposal`, and `type DistillationResult struct`
**When:** `ValidateContractSpecificity` runs on the contract
**Then:** Warnings are returned for `ModelTier` and `Proposal` but not for `type DistillationResult struct`

### Scenario: unexported identifier not flagged
**Given:** The LLM generates a contract with `file_contains: {path: internal/foo/bar.go, pattern: modelTier}`
**When:** `ValidateContractSpecificity` runs on the contract
**Then:** No warning is returned — unexported identifiers are less likely to be ambiguous across multiple type definitions

### Scenario: runtime auto-correction tightens low-specificity contract
**Given:** A run in cycle 2 where validation finds `file_contains: {path: types.go, pattern: ModelTier}` fails because `ModelTier` was removed from `DistillerInputs`. But `DistillationResult` in the same file still has `ModelTier  string`.
**When:** The validate stage checks the failure
**Then:** The pattern is classified as low-specificity. The validator reads `types.go`, finds `ModelTier  string` in `DistillationResult`, rewrites the contract pattern from `ModelTier` to `ModelTier  string`, persists the updated `scenario-contracts.yaml`, and re-evaluates. The tightened contract passes. No replan is triggered for this assertion.

### Scenario: file_not_contains assertion not checked
**Given:** The LLM generates a contract with `file_not_contains: {path: types.go, pattern: DeprecatedField}`
**When:** `ValidateContractSpecificity` runs on the contract
**Then:** No warning is returned — only `file_contains` assertions are checked

## Validation

### Automatic
- `go test ./internal/next/contract/... -count=1 -timeout 60s`
- `go test ./internal/next/specloop/stages/... -count=1 -timeout 60s`
- `go test ./internal/next/runstore/... -count=1 -timeout 60s`
- `go vet ./...`

### Manual
1. Run a spec that targets a multi-type file (e.g., `types.go` with several structs). Verify that the WriteContracts stage logs specificity warnings if the LLM produces bare-identifier patterns.
2. Verify that the retry produces more specific patterns by inspecting the final `scenario-contracts.yaml`.
3. Verify that a contract with only multi-token patterns passes specificity validation without retry.
