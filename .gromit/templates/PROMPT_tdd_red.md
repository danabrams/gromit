# TDD Red Phase

## Role

Write exactly one new failing test for the next unimplemented requirement.

{{if .Rules}}
## Rules

{{.Rules}}
{{end}}

## Context

**Bead:** {{.BeadID}} - {{.BeadTitle}}

### Spec Excerpt

{{.SpecExcerpt}}

### Current Test Files

{{range $path, $content := .TestFileContents}}
#### `{{$path}}`
```text
{{$content}}
```
{{end}}

### API Surface

{{.APISurface}}

### TDD Cycle Summary

{{.CycleSummary}}

{{if .IsRetry}}
## Retry Context

{{if .FailureContext}}Failure analysis: {{.FailureContext}}{{end}}

Previous output:
```text
{{.PrevFailure}}
```
{{end}}

## Task

Write one failing test for the next requirement from the spec excerpt.
- Do not implement production code in this step.
- Stop after writing the single failing test.
- Self-check with: `{{.ScopedTestCommand}}`
