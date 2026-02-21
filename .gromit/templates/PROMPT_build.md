# Task Execution

You are executing a single task from the work queue. Focus only on this task.

{{if .Rules}}
## Rules (Non-Negotiable)

{{.Rules}}
{{end}}

{{if .ConfirmedLearnings}}
## Learnings (Confirmed Patterns)

These patterns have been observed multiple times in this project:

{{formatLearnings .ConfirmedLearnings}}
{{end}}

{{if .RecentLearnings}}
## Recent Learnings

Recent observations that may be relevant:

{{formatLearnings .RecentLearnings}}
{{end}}

{{if .RecentValidationFailures}}
## Recent Validation Issues

The following validation failures occurred in recent beads during this run. Avoid repeating these mistakes:

{{range .RecentValidationFailures}}
- {{.}}
{{end}}
{{end}}

{{if .CommonReviewFindings}}
## Common Review Findings To Avoid

Recent reviews repeatedly found these categories. Proactively check your changes for them before finishing:

{{range .CommonReviewFindings}}
- {{.}}
{{end}}
{{end}}

## Project Context

{{if .ClaudeMD}}
{{.ClaudeMD}}
{{end}}

## Current Task

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}
{{if .Bead.Labels}}**Labels:** {{join .Bead.Labels ", "}}{{end}}

{{if .Bead.Description}}
### Description
{{.Bead.Description}}
{{end}}

{{if .Spec}}
## Specification

The following specification provides detailed requirements for this task:

{{.Spec}}
{{end}}

{{if .ParentBead}}
## Parent Context

This task is part of: **{{.ParentBead.Title}}**
{{if .ParentBead.Description}}
{{.ParentBead.Description}}
{{end}}
{{end}}

{{if .IsRetry}}
## Previous Attempt Failed

{{if .FailureContext}}
Analysis suggests: {{.FailureContext}}
{{end}}

Previous output:
```
{{.PrevFailure}}
```

Please analyze the failure and try a different approach.
{{end}}

## Instructions

1. **Study the codebase** before making changes - don't assume code is missing
2. **Implement the task** following existing patterns in the codebase
3. **Write tests** if the task involves new functionality
4. **Self-check** — Before completing, run `go test` and `go vet` scoped to the packages you touched{{if .ScopedTestCommand}} using this exact command: `{{.ScopedTestCommand}}`{{else}} (e.g., `go test ./internal/foo/... ./internal/bar/...`), not the full suite. The separate validation phase runs `go test ./...` to catch cross-package regressions{{end}}. Fix failures before committing
5. **Commit your changes** with a clear commit message

## Completion

When the task is complete:
- All code changes are committed
- Tests pass (if applicable)
- The implementation matches the specification

Do NOT output any special completion markers - just complete the task and exit.
