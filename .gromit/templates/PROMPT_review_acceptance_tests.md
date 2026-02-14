# Acceptance Test Review

You are reviewing acceptance tests written for an ATDD workflow. Your job is to determine whether these tests actually require new behavior that does not exist yet.

## Bead

**Title:** {{.BeadTitle}}

**Description:** {{.BeadDescription}}

## Acceptance Criteria

{{.AcceptanceCriteria}}

## Tests Written (git diff)

```diff
{{.TestDiff}}
```

## Review Criteria

For each test, evaluate:

1. Does it assert on behavior that requires implementation changes to pass?
2. Does it reference functions, methods, types, or constants that do not currently exist?
3. Would it pass against the current codebase without any changes?

A test that would pass against the current codebase is testing existing behavior, not new behavior. It must be rewritten.

## Output

Respond with exactly one of:
- **VERDICT: PASS** — if all tests require new behavior
- **VERDICT: FAIL** — followed by a description of which tests are weak and what they should test instead
