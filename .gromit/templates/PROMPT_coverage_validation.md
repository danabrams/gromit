# Coverage Validation

You are validating whether the provided test code covers the criterion.

## Criterion

Criterion #{{.CriterionNumber}}: {{.CriterionText}}

## Test Code

```go
{{.TestCode}}
```

## Instructions

1. Decide whether the test code fully covers the criterion.
2. Set `covers` to true only when coverage is complete and direct.
3. Provide a concise reason that references the evidence or gap.

## Output

ValidationResponse is the only response contract to follow.
Respond with exactly one top-level JSON object in the form `{"covers": bool, "reason": "one sentence"}`.
Output JSON only; no markdown, commentary, or extra text.

Required top-level fields:
- `covers` (boolean)
- `reason` (string, exactly one sentence)

Example:

```json
{"covers": true, "reason": "..."}
```
