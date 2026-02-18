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

Respond with a JSON object matching the GateVerdict structure:

```json
{"passed": true, "results": [{"criterion": "...", "passed": true, "evidence": "..."}]}
```

Emit one result per criterion. Set `passed` at the top level to `true` only if all results have `passed: true`.
