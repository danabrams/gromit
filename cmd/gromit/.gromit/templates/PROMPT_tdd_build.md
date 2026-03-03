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

{{if .MidBuildReviewFindings}}
## Mid-build Review Findings

The mid-build review surfaced the following concerns. Please resolve them before proceeding:

{{range .MidBuildReviewFindings}}
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

You are following Test-Driven Development (TDD) with strict red-green-refactor cycles. Your implementation must proceed incrementally through repeated cycles.

### TDD Discipline: Red-Green-Refactor

Work in small increments, following this cycle for each piece of functionality:

1. **Red — Write ONE failing test:**
   - Focus on a single behavior or requirement piece
   - Write a test that expresses what the code should do
   - The test must fail because the behavior doesn't exist yet
   - Do NOT write multiple tests before implementing

2. **Green — Write minimal code to pass:**
   - Write the simplest code that makes this one test pass
   - Do NOT write "proper" or "complete" implementations yet
   - Do NOT write code for functionality you haven't tested
   - It's okay if the code feels hacky or incomplete — that's the point

3. **Commit — Record the cycle:**
   - Commit the test and implementation together
   - Use a clear message describing what was tested and implemented
   - The commit trail creates accountability for the TDD discipline

4. **Repeat — Move to the next piece:**
   - Continue with the next behavior or requirement piece
   - Each cycle should be small — aim for 5-15 minutes of work
   - Do NOT skip ahead to write multiple tests or "complete" implementations

### Key Principles

- **One test at a time** — Resist the urge to write multiple tests upfront
- **Minimum to pass** — Don't write more implementation than needed for the current test
- **Small steps** — Break requirements into the smallest testable pieces
- **Commit each cycle** — Your git log should show the progression of red-green cycles
- **Stop when done** — When all requirements are covered by tests, stop — refactoring will happen in a separate phase

### What About Refactoring?

Do NOT refactor during this phase. Focus only on the red-green cycles. A separate refactoring phase will run after validation passes, where you can:
- Improve names and structure
- Remove duplication
- Extract abstractions
- Clean up "minimum to pass" roughness

### Completion

When the task is complete:
- Each requirement piece is covered by at least one test
- All tests pass
- Each red-green cycle is recorded in a separate commit
- All changes are committed

Do NOT output any special completion markers - just complete the task and exit.
