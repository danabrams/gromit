# Spec 0003e — Persistent Failure Contract Audit

## spec_id
0003e-persistent-failure-contract-audit

## Depends on
0003b-replan-context-deduplication

## Vision
When the same contract assertion fails across multiple consecutive cycles, the persistent-failure hint fires and says "may indicate a bad test specification." But the fix planner's prompt buries this hint in the general failures list with instructions that push toward implementation fixes. The signal is present but structurally invisible — the planner has no reason to act on it differently. This spec makes persistent failures a first-class signal in the fix planner: they get a dedicated section with targeted guidance to consider fixing the contract assertion itself, not just the implementation.

## Summary
When `buildFixPrompt` receives failures that include `persistent-failure:` hints, it extracts them into a dedicated `## Persistent Failures — Possible Bad Contracts` section with explicit instructions to audit the contract assertion YAML before creating implementation fix tasks. The persistent failures also remain in the main `## Validation Failures to Fix` section so the planner can still generate implementation fix tasks if it judges the contract to be correct.

## Goals
### Primary
- Give the fix planner structured, targeted guidance when persistent failures are present
- Instruct the planner to check whether the pattern in scenario-contracts.yaml actually matches the file before assuming the implementation is wrong

### Secondary
- Reduce wasted replan cycles caused by unsatisfiable contract assertions

## Non-goals
- Not automatically fixing contracts — the planner still decides
- Not changing when or how persistent-failure hints are generated (that's 0003b)
- Not adding new terminal states or escalation paths (that's 0003d)
- Not validating contracts at write time (a future spec)

## Architecture
The change is entirely in `buildFixPrompt` in `internal/next/planner/planner.go`.

Currently all failures (including `persistent-failure:` hints) flow into one of two buckets: `reviewFindings` or `otherFailures`. The new logic adds a third pass that extracts `persistent-failure:` lines into `persistentFailures` — but does NOT remove them from `otherFailures`. Both lists are rendered.

```go
var reviewFindings, otherFailures, persistentFailures []string
for _, f := range req.Failures {
    if strings.HasPrefix(f, "review:") {
        reviewFindings = append(reviewFindings, f)
    } else {
        otherFailures = append(otherFailures, f)
    }
    if strings.HasPrefix(f, "persistent-failure:") {
        persistentFailures = append(persistentFailures, f)
    }
}
```

When `persistentFailures` is non-empty, a new section is rendered **before** `## Validation Failures to Fix`:

```
## Persistent Failures — Possible Bad Contracts
The following failures have repeated across multiple consecutive cycles.
This strongly suggests the contract assertion itself is wrong, not the implementation.

BEFORE creating any implementation fix task for these failures:
1. Find the assertion in scenario-contracts.yaml that corresponds to this failure
2. Verify the pattern actually appears in the target file (run grep manually in your head)
3. If the pattern looks like a regex (contains .*  \w+  \[  etc.) but the file uses
   literal Go syntax, the pattern may need to be a literal substring instead
4. Prefer creating a contract fix task (editing scenario-contracts.yaml) unless you
   have high confidence the implementation is wrong

Persistent failures:
- <list>
```

No new types, no new fields on `PlanRequest`. Pure prompt construction change.

## Acceptance Criteria

1. When `req.Failures` contains one or more `persistent-failure:` prefixed entries, `buildFixPrompt` renders a `## Persistent Failures — Possible Bad Contracts` section before `## Validation Failures to Fix`
2. The persistent failures section includes explicit instructions to audit the contract assertion in `scenario-contracts.yaml` before creating implementation fix tasks
3. Persistent failures still appear in `## Validation Failures to Fix` — they are not removed from the main list
4. When `req.Failures` contains no `persistent-failure:` entries, no persistent failures section is rendered and existing behavior is unchanged
5. Non-persistent contract failures and review findings are unaffected by this change
6. All existing planner tests continue to pass

## Scenarios

### Scenario: persistent failure triggers contract audit section
**Given:** A replan context containing one contract failure and its corresponding persistent-failure hint:
```
contract:first-failure-no-escalation — file_contains failed: pattern "ChainIDs.*\[\]string" not found in "internal/next/runstore/types.go"
persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug
```
**When:** `buildFixPrompt` is called with these as `req.Failures`
**Then:** The rendered prompt contains a `## Persistent Failures — Possible Bad Contracts` section listing the persistent-failure hint, with instructions to check `scenario-contracts.yaml` and consider whether the pattern is a regex being matched literally. The contract failure also appears in `## Validation Failures to Fix`.

### Scenario: no persistent failures — prompt unchanged
**Given:** A replan context with only ordinary contract and test failures, no `persistent-failure:` entries
**When:** `buildFixPrompt` is called
**Then:** No `## Persistent Failures` section appears. Prompt is identical to current behavior.

### Scenario: multiple persistent failures across different contracts
**Given:** Three contract failures, two of which have persistent-failure hints
**When:** `buildFixPrompt` is called
**Then:** Both persistent failures appear in the dedicated section. All three contract failures appear in `## Validation Failures to Fix`. The non-persistent failure does not appear in the persistent section.

### Scenario: persistent-failure hint without corresponding contract failure
**Given:** A replan context containing only a persistent-failure hint, with no corresponding `contract:` failure entry (e.g. the original failure was deduplicated into a summary that no longer carries the `contract:` prefix):
```
persistent-failure: contract:first-failure-no-escalation has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug
```
**When:** `buildFixPrompt` is called
**Then:** The persistent-failure hint still appears in `## Persistent Failures — Possible Bad Contracts` with the full audit instructions. `## Validation Failures to Fix` may be empty or contain only the deduplicated summary — either way, the planner still receives the signal to audit the contract.

## Validation
```
go test ./internal/next/planner/... -count=1 -timeout 60s
go vet ./internal/next/planner/...
```
