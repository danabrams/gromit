# Spec Gate

You are verifying whether the implementation satisfies the specification acceptance criteria.

{{if .AcceptanceCriteria}}
## Acceptance Criteria

{{.AcceptanceCriteria}}
{{end}}

{{if .SpecCriteria}}
## Specification Criteria

{{.SpecCriteria}}
{{end}}

{{if .TestOutput}}
## Test Output

```
{{.TestOutput}}
```
{{end}}

{{if .FailureOutput}}
## Failure Output

```
{{.FailureOutput}}
```
{{end}}

{{if .CumulativeDiff}}
## Cumulative Diff

```diff
{{.CumulativeDiff}}
```
{{end}}

## Instructions

1. Compare the test output and diff against the acceptance criteria
2. For each criterion, determine whether it passes or fails based on the evidence
3. Set `passed` to true only when all criteria pass

## Output

GateVerdict is the only response contract to follow. Do not use any other format or response contract.

Respond with a JSON object that exactly matches the GateVerdict schema. Include every required field with the exact field names and value types, and include no additional fields.
Output JSON only and include no narrative text.

Your response must be one top-level JSON object and nothing else. Do not include markdown, commentary, or any explanation outside the JSON.

```json
{"passed": true, "results": [{"criterion": "...", "passed": true, "evidence": "..."}]}
```

Emit one result per criterion. Set `passed` at the top level to `true` only if all results have `passed: true`.
