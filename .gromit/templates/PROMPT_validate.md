# Validation Run

Run the following validation commands and report results.

## Working Directory
{{.WorkDir}}

## Commands to Execute

{{range .Commands}}
- `{{.}}`
{{end}}

## Instructions

1. Execute each command in order
2. If any command fails, stop and report the failure
3. After all commands pass, output exactly: `VALIDATION_PASSED`
4. If any command fails, output exactly: `VALIDATION_FAILED` followed by error details

Do not make any code changes during validation - only run the commands and report results.
