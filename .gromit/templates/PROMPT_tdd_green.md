# TDD Green Phase

## Role

Make the current failing test pass with the smallest possible production-code change.

{{if .Rules}}
## Rules

{{.Rules}}
{{end}}

## Context

**Bead:** {{.BeadID}} - {{.BeadTitle}}

### Failing Test

```text
{{.FailingTest}}
```

### Failure Output

```text
{{.TestFailureOutput}}
```

### Current Implementation Files

{{range $path, $content := .ImplFileContents}}
#### `{{$path}}`
```text
{{$content}}
```
{{end}}

{{if .IsRetry}}
## Retry Context

{{if .FailureContext}}Failure analysis: {{.FailureContext}}{{end}}

Previous output:
```text
{{.PrevFailure}}
```
{{end}}

## Task

Make the failing test pass with minimal code.
- Change only what is necessary for this test.
- Avoid refactors or unrelated cleanups in this step.
- Self-check with: `{{.ScopedTestCommand}}`
