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
{{if .CoverageState}}
## Coverage State
{{if .TargetCriterion}}
Target criterion: {{.TargetCriterion}}
{{end}}
{{.CoverageState}}
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

## Instructions — Red-Green-Refactor Discipline

You MUST follow red-green-refactor strictly. Each cycle is small and committed separately.

### The Cycle (repeat for each requirement)

**1. RED — Write ONE failing test**
- Write a single test function or test case that calls code which doesn't exist yet or doesn't behave correctly yet
- Run tests: they MUST fail (compilation errors count as failing)
- Commit: `red: test for <what the test verifies>`
- Do NOT write any production code in this step

**2. GREEN — Write minimum production code**
- Write only enough production code to make the failing test pass
- Do NOT modify the test you just wrote
- Do NOT add anything beyond what this one test requires
- Run tests: they MUST pass
- Commit: `green: implement <what you added>`

**3. COMMIT and move to next requirement** — refactoring happens in a separate phase

### Non-Negotiable Rules

- ONE test per red step. Stop after writing it.
- MINIMUM code per green step. No "while I'm here" additions.
- SEPARATE commits for red and green. Each commit message starts with `red:` or `green:`.
- Do NOT batch multiple requirements into one cycle.
- Before completing, run `go test` and `go vet` scoped to touched packages{{if .ScopedTestCommand}} using: `{{.ScopedTestCommand}}`{{else}} (e.g., `go test ./internal/foo/...`), not `./...`{{end}}. Fix any failures before committing.
- After all requirements are covered, stop — refactoring happens in a separate phase.

## Completion

When complete:
- Multiple small commits exist, alternating `red:` and `green:` prefixes
- All tests pass
- Each requirement has a corresponding test
- No gold plating — minimum viable implementation only

Do NOT output any special completion markers - just complete the task and exit.
