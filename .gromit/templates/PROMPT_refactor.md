# Refactoring Phase

You are refactoring the implementation after tests pass. Your goal is to improve code quality without changing behavior.

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

**All tests are passing.** Your job is to review the implementation for quality improvements without changing behavior.

1. **Review the implementation** looking for:
   - Code duplication or unnecessary complexity
   - Unclear variable or function names
   - Missing or misleading comments/documentation
   - Opportunities to follow established project patterns better
   - Error handling that could be clearer
   - Long functions that could be decomposed
   - Magic numbers or strings that should be constants

2. **Make refactoring changes** that:
   - Improve readability and maintainability
   - Do NOT change external behavior (tests must still pass)
   - Follow existing project conventions and patterns
   - Are small, focused improvements (not large rewrites)
   - Make the code clearer, not just different

3. **Do NOT refactor if the code is already clear and follows project conventions** - refactoring for its own sake adds no value

4. **Commit your changes** with a clear commit message like "refactor: improve clarity in [component/function]" - keep refactoring commits separate from implementation commits

## Important Notes

- Refactoring must preserve all existing behavior - verify by running `go test` and `go vet` scoped to the packages you touched, not the full suite. The separate validation phase runs `go test ./...` to catch cross-package regressions
- Only refactor the code touched by this task, not the entire codebase
- If the implementation is already clear and well-structured, say so and make no changes
- Focus on readability and maintainability, not premature optimization
- Follow the project's existing patterns - don't introduce new styles or conventions

## Completion

When complete:
- Code quality improvements are committed (if any were needed)
- All tests still pass (behavior unchanged)
- Changes follow project conventions

Do NOT output any special completion markers - just complete the task and exit.
