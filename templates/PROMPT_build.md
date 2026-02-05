# Task Execution

You are executing a single task from the work queue. Focus only on this task.

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

The previous attempt with a different model failed. Here's what happened:

```
{{.PrevFailure}}
```

Please analyze the failure and try a different approach.
{{end}}

## Instructions

1. **Study the codebase** before making changes - don't assume code is missing
2. **Implement the task** following existing patterns in the codebase
3. **Write tests** if the task involves new functionality
4. **Commit your changes** with a clear commit message

## Completion

When the task is complete:
- All code changes are committed
- Tests pass (if applicable)
- The implementation matches the specification

Do NOT output any special completion markers - just complete the task and exit.
