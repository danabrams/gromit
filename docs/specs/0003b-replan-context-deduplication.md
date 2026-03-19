DONE 2026-03-19
# Spec 0003b — Replan Context Deduplication

## spec_id
0003b-replan-context-deduplication

## Depends on
None

## Vision
When a run triggers a replan, the failure context passed to the planner can contain dozens of repetitive entries. In an observed incident, the replan_context had 32 entries, most saying variations of "file_contains failed: cannot read write_scenario_tests.go" — the same missing file referenced by different contract assertions. This wastes planner tokens on noise and obscures the actual signal, leading to worse fix task generation. The planner would produce better fix tasks with a clear, deduplicated summary like "15 contract assertions failed because write_scenario_tests.go does not exist."

## Summary
Add failure deduplication logic in the specloop, after collecting failures from a stage's FailureContext and before storing them in `rs.ReplanContext`. The deduplication groups contract failures that share the same root cause (e.g., same missing file, same unreadable file) into a single summary entry with a count, while preserving unique/distinct failures as-is. This runs in the specloop's replan handler, which is the natural choke point where all failure sources (validate, review, accept) converge.

## Goals
### Primary
- Deduplicate replan context entries that share the same root cause
- Reduce token waste in planner prompts
- Improve planner signal quality for better fix task generation

### Secondary
- Make replan context human-readable in run.json for debugging

## Non-goals
- Deduplication within individual stages (validate, review) — dedup happens centrally in specloop
- Changing failure string formats — existing formats are preserved, just grouped
- Semantic/LLM-based dedup — this is pure string pattern matching
- Deferred: infrastructure failure detection (0003a), review degradation (0003c), task escalation (0003d)

## Architecture

The deduplication logic lives in the specloop package, near the existing failure history code (`failure_history.go` or a new `dedup.go` file).

Key design decisions:
- Dedup runs in the specloop's replan handler, after receiving failures from a stage's `FailureContext` and before storing in `rs.ReplanContext`
- Contract failures follow the format: `contract:<scenario-name> — <assertion-type> failed: <details>`
- Grouping key for contract failures: extract the root cause from `<details>` (e.g., the file path that doesn't exist or can't be read)
- Test failures (`--- FAIL: TestName`) are NOT grouped — each test failure is distinct
- Review findings are NOT grouped — each finding is distinct
- Only contract failures with matching root causes are collapsed

```go
// DeduplicateFailures groups contract failures by root cause and collapses
// duplicates into summary entries. Non-contract failures pass through unchanged.
func DeduplicateFailures(failures []string) []string {
    // 1. Separate contract failures from other failures
    // 2. For contract failures, extract root cause (file path from error message)
    // 3. Group by root cause
    // 4. For groups with >1 entry, replace with summary:
    //    "N contract assertions failed: <root cause> (scenarios: A, B, C)"
    // 5. For groups with 1 entry, keep as-is
    // 6. Return: deduplicated contracts + other failures (order: other first, then contract summaries)
}
```

Root cause extraction patterns:
- `cannot read "<path>"` → prefix match that extracts the path between quotes; group key is the file path
- `file "<path>" does not exist` → group key is the file path
- `pattern "<pattern>" not found in "<path>"` → group key is `<path>:<pattern>` (don't group unrelated checks on the same file)

Failures sharing the same file path are grouped together regardless of which pattern matched. For example, `file "X" does not exist` and `cannot read "X": ... no such file or directory` share the same group key (the file path) because both indicate the file is missing.

When multiple error variants map to the same root cause, the summary uses a normalized description: `"file \"<path>\" does not exist"` for missing-file groups, `"pattern \"<pattern>\" not found in \"<path>\""` for pattern-not-found groups.

Note: Only `file_exists`, `file_not_exists`, and `file_contains` error patterns are handled in v1. Other assertion types (`file_not_contains`, `file_not_modified`) pass through ungrouped. This is fine because these assertion types are rare in practice.

The function is called in `specloop.go` in the replan handler block. The ordering is critical because persistent-failure annotation adds `persistent-failure:` hints to individual failures, and those hints must be present before deduplication collapses entries. The flow:

```go
// 1. Failure history extraction runs on original failures
failureKeys := ExtractContractFailureKeys(replanContext.Failures)
UpdateFailureHistory(failureKeys)

// 2. Persistent-failure annotation runs on original failures
//    (attaches "persistent-failure: ..." hints to individual entries)
annotated := AnnotatePersistentFailures(replanContext.Failures, failureHistory)

// 3. Dedup runs on the annotated result for ReplanContext storage
deduplicated := DeduplicateFailures(annotated)
rs.ReplanContext = deduplicated
```

This ordering ensures: (a) failure history sees the original granular failures, (b) persistent-failure hints are attached to individual failures before collapsing, and (c) the deduplicated result stored in ReplanContext carries any hints that were attached. The summary format (`"5 contract assertions failed: ..."`) would not match the `"contract:"` prefix check in annotation logic, so annotation must run first on the original failures.

## Acceptance Criteria

1. When replan context contains multiple contract failures with the same root cause (same missing/unreadable file), they are collapsed into a single summary entry
2. The summary entry includes: count of collapsed failures, the root cause, and the list of affected scenario names
3. Contract failures with distinct root causes remain as separate entries
4. Non-contract failures (test failures, always-run check failures, review findings) pass through unchanged and are not grouped
5. Failure history extraction (as defined in `failure_history.go`) still operates correctly on the original pre-dedup failures — dedup does not break persistent failure tracking
6. Non-contract failures appear before deduplicated contract summaries in the output
7. The deduplication function is a pure function (no side effects, no state) taking `[]string` and returning `[]string`
8. When only 1 contract failure exists for a given root cause, it is kept as-is (no wrapping in summary format)
9. All existing specloop and validate tests continue to pass
10. Persistent-failure annotation (`persistent-failure:` hints) operates on original failures before deduplication, ensuring hints are correctly generated

## Scenarios

### Scenario: Multiple contract failures from same missing file are collapsed
**Given:** A replan context with 5 contract failure entries all caused by the same missing file:
```
contract:Happy path — file_exists failed: file "internal/next/specloop/stages/write_scenario_tests.go" does not exist
contract:Happy path — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory
contract:Self-repair succeeds — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory
contract:Self-repair fails — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory
contract:Replan preserves — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": open .../write_scenario_tests.go: no such file or directory
```
**When:** DeduplicateFailures is called
**Then:** The 5 entries are collapsed into a single entry:
`"5 contract assertions failed: file \"internal/next/specloop/stages/write_scenario_tests.go\" does not exist (scenarios: Happy path, Self-repair succeeds, Self-repair fails, Replan preserves)"`
**Notes:** The root cause is the file path. Both "does not exist" and "cannot read ... no such file or directory" share the same root cause.

### Scenario: Failures from different files remain separate
**Given:** A replan context with 2 contract failures referencing different files:
```
contract:Happy path — file_contains failed: cannot read "internal/next/specloop/stages/write_scenario_tests.go": ...
contract:Scenario test fails — file_contains failed: cannot read "internal/next/runstore/types.go": ...
```
**When:** DeduplicateFailures is called
**Then:** Both entries remain separate (or are each in their own group of 1, kept as-is). The output contains both original strings.
**Notes:** Different file paths = different root causes = no dedup

### Scenario: Mixed contract and test failures
**Given:** A replan context with 3 contract failures (same root cause) and 2 test failures:
```
contract:A — file_contains failed: cannot read "stages/write_scenario_tests.go": ...
contract:B — file_contains failed: cannot read "stages/write_scenario_tests.go": ...
contract:C — file_contains failed: cannot read "stages/write_scenario_tests.go": ...
always-run check "unit-tests" failed: --- FAIL: TestAdd ...
always-run check "vet" failed: pattern ./...: directory prefix . does not contain main module
```
**When:** DeduplicateFailures is called
**Then:** Output contains 3 entries in this order: the 2 test/check failures unchanged first, then 1 collapsed contract summary. Non-contract failures always appear before deduplicated contract summaries. Test failures are never grouped or modified.

### Scenario: Single contract failure for a root cause is kept as-is
**Given:** A replan context with exactly 1 contract failure:
```
contract:Happy path — file_exists failed: file "write_scenario_tests.go" does not exist
```
**When:** DeduplicateFailures is called
**Then:** The output contains the original string unchanged — no summary wrapping

### Scenario: Failure history still works after dedup
**Given:** A replan context with 5 contract failures from 3 scenarios: Happy path (3 failures), Self-repair succeeds (1 failure), Self-repair fails (1 failure)
**When:** The specloop processes the replan: calls DeduplicateFailures for ReplanContext, but calls ExtractContractFailureKeys on the original (pre-dedup) failures
**Then:** FailureHistory correctly tracks 3 keys: `contract:Happy path`, `contract:Self-repair succeeds`, `contract:Self-repair fails` — each incremented by 1. The dedup does not interfere with failure history tracking.
**Notes:** This is critical — dedup is a display/token optimization, not a data loss operation. Failure history must see the original granular failures.

### Scenario: file_contains failures where file exists but pattern not found are deduplicated
**Given:** A replan context with 3 contract failures where the file exists but the expected pattern is not found:
```
contract:Happy path — file_contains failed: pattern "func RunScenarioTests" not found in "internal/next/specloop/stages/write_scenario_tests.go"
contract:Self-repair succeeds — file_contains failed: pattern "func RunScenarioTests" not found in "internal/next/specloop/stages/write_scenario_tests.go"
contract:Self-repair fails — file_contains failed: pattern "func RunScenarioTests" not found in "internal/next/specloop/stages/write_scenario_tests.go"
```
**When:** DeduplicateFailures is called
**Then:** The 3 entries are collapsed into a single entry:
`"3 contract assertions failed: pattern \"func RunScenarioTests\" not found in \"internal/next/specloop/stages/write_scenario_tests.go\" (scenarios: Happy path, Self-repair succeeds, Self-repair fails)"`
**Notes:** The group key is `<path>:<pattern>`. If a different pattern were checked on the same file, it would be a separate group.

### Scenario: Empty replan context
**Given:** An empty failures slice `[]string{}`
**When:** DeduplicateFailures is called
**Then:** Returns an empty slice

## Validation
- `go test ./internal/next/specloop/ -count=1`
- `go vet ./...`
