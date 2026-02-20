# Spec Gate

You are verifying whether the implementation satisfies the specification acceptance criteria.

## Acceptance Criteria

AcceptanceCriteria:
{{if .AcceptanceCriteria}}
{{.AcceptanceCriteria}}
{{else}}
[none]
{{end}}

{{if .SpecCriteria}}
## Specification Criteria

{{.SpecCriteria}}
{{end}}

## Test Output

TestOutput:
```
{{if .TestOutput}}
{{.TestOutput}}
{{else}}
[none]
{{end}}
```

{{if .FailureOutput}}
## Failure Output

```
{{.FailureOutput}}
```
{{end}}

## Cumulative Diff

CumulativeDiff:
```diff
{{if .CumulativeDiff}}
{{.CumulativeDiff}}
{{else}}
[none]
{{end}}
```

## Instructions

1. Compare the test output and diff against the acceptance criteria
2. For each criterion, determine whether it passes or fails based on the evidence
3. Set `passed` to true only when all criteria pass
4. Return your result using the `GateVerdict` response contract

## GateVerdict Response Contract

The model **must** follow the `GateVerdict` response contract.
`GateVerdict` is the only response contract to follow. Do not use any other format or response contract.
Every response must conform to the `GateVerdict` schema at the schema level.
Respond with a JSON object that exactly matches the GateVerdict schema. Include every required field with the exact field names and value types, and include no additional fields.
All required keys must be present. Preserve the required GateVerdict structure and nesting exactly.
Any omission of required keys, type mismatches, structural changes, or extra fields is invalid.
Output JSON only and include no narrative text.

Your response must be one top-level JSON object and nothing else. Do not include markdown, commentary, or any explanation outside the JSON.

## GateVerdict Schema

Required top-level fields:
- `passed` (boolean)
- `results` (array of objects)

Required result fields:
- `criterion` (string)
- `passed` (boolean)
- `evidence` (string)

Required structure:
- Top-level value must be an object containing `passed` and `results`
- `results` must be an array
- Each element in `results` must be an object containing `criterion`, `passed`, and `evidence`

## Example Output

```json
{"passed": true, "results": [{"criterion": "...", "passed": true, "evidence": "..."}]}
```

Emit one result per criterion. Set `passed` at the top level to `true` only if all results have `passed: true`.
