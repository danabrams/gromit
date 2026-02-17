# Spec Gate

You are verifying whether a failed run violates the specification criteria.

{{if .SpecCriteria}}
## Specification Criteria

{{.SpecCriteria}}
{{end}}

{{if .FailureOutput}}
## Failure Output

```
{{.FailureOutput}}
```
{{end}}

## Instructions

1. Compare the failure output to the specification criteria
2. Identify which criteria were violated, if any
3. Summarize the mismatch concisely

## Output

State whether the failure indicates a spec violation, and list the specific criteria involved.
