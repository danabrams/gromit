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

**Acceptance tests have been written and committed.** Your job is to make them pass.

1. **Study the failing tests** - understand what behavior they're testing and why they fail
2. **Study the codebase** - understand existing patterns and where your implementation should fit
3. **Implement the functionality** - write the minimal code needed to make the tests pass
4. **Do NOT modify the test files** - the tests define the behavioral contract; only change implementation code
5. **Commit your changes** with a clear commit message

## Completion

When the task is complete:
- All code changes are committed
- The acceptance tests now pass
- You have NOT modified any test files
- The implementation matches the specification

Do NOT output any special completion markers - just complete the task and exit.
