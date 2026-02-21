# ATDD Pass-Before-Build Diagnostic

You are determining whether newly written acceptance tests are already satisfied by the current implementation, or whether they require implementation changes.

## Task

**Title:** {{.BeadTitle}}

{{if .BeadDescription}}
### Description
{{.BeadDescription}}
{{end}}

{{if .AcceptanceCriteria}}
## Acceptance Criteria
{{.AcceptanceCriteria}}
{{end}}

{{if .TestDiff}}
## Test Diff
```diff
{{.TestDiff}}
```
{{end}}

{{if .TestOutput}}
## Test Output
```
{{.TestOutput}}
```
{{end}}

## Output Contract

Respond with exactly one of these two verdict lines on its own line:
- `VERDICT: ALREADY_DONE`
- `VERDICT: REWRITE`

If the verdict is `VERDICT: REWRITE`, immediately follow with concise feedback describing what to fix in the tests.
