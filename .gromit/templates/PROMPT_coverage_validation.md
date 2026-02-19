# Coverage Validation

You are validating whether the provided test code covers the criterion.

## Criterion

{{.Criterion}}

## Test Code

```go
{{.TestCode}}
```

## Instructions

1. Decide whether the test code fully covers the criterion.
2. Set `covers` to true only when coverage is complete and direct.
3. Provide a concise reason that references the evidence or gap.

## Output

Respond with a JSON object that exactly matches the schema below. Output JSON only; no markdown, commentary, or extra text.

Required top-level fields:
- `covers` (boolean)
- `reason` (string)

Example:

```json
{"covers": true, "reason": "..."}
```
