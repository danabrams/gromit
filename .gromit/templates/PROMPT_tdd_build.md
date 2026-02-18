# Task Execution

You are executing a single task from the work queue using Test-Driven Development (TDD). Focus only on this task.

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

Follow the **red-green-refactor** cycle strictly. Work in small increments.

1. **Red** — Write ONE small failing unit test that addresses a piece of the bead's requirements
2. **Green** — Write the minimum code to make that test pass
3. **Commit** — Commit the test + implementation together with a message describing what was tested and implemented
4. **Repeat** — Move to the next piece of the requirement

**Critical rules:**
- Write ONE test at a time, then implement to make it pass
- Do NOT write multiple tests before implementing
- Focus each test on a single behavior or requirement from the bead
- Commit after each red-green cycle
- Write minimum code to pass - no gold plating
- Before completing, run `go test` and `go vet` scoped to the packages you touched{{if .ScopedTestCommand}} using this exact command: `{{.ScopedTestCommand}}`{{else}} (e.g., `go test ./internal/foo/... ./internal/bar/...`), not the full suite. The separate validation phase runs `go test ./...` to catch cross-package regressions{{end}}. Fix failures before committing
- After all requirements are covered, stop - refactoring will happen in a separate phase

## Completion

When the task is complete:
- All code changes are committed (in multiple small commits, one per red-green cycle)
- All tests pass
- Each commit shows the red-green discipline: test + minimal implementation
- The implementation covers all acceptance criteria

Do NOT output any special completion markers - just complete the task and exit.
