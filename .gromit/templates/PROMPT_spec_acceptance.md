# Spec Acceptance

You are validating whether the specification is sufficient to proceed with implementation.

{{if .Rules}}
## Rules (Non-Negotiable)

{{.Rules}}
{{end}}

{{if .Spec}}
## Specification

{{.Spec}}
{{end}}

## Instructions

1. Review the specification for clarity, completeness, and testability
2. Identify any missing acceptance criteria or ambiguous requirements
3. Suggest concrete improvements if needed

## Output

Provide a concise summary of any issues found. If the specification is adequate, state that it is accepted.
