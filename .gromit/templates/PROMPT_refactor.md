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

## CRITICAL CONSTRAINT

You are in the **REFACTOR phase** of TDD. All tests are passing. You may improve code structure but you MUST NOT change behavior.

**After you finish, the orchestrator will run tests automatically. Tests MUST still PASS.** If any test fails, your refactoring is reverted.

**What you CAN do:**
- Rename variables, functions, or types for clarity
- Extract helpers or reduce duplication
- Simplify control flow or error handling
- Add constants for magic values
- Reorganize code within files

**What you MUST NOT do:**
- Add new features or new test cases — that's the next red phase
- Change what any function returns for a given input
- Delete or skip tests
- Make large rewrites — keep changes small and safe

## Instructions

1. **Review** the implementation for readability, duplication, naming, and adherence to project patterns
2. **Refactor** only what genuinely improves clarity — if the code is already clean, make no changes and say so
3. **Verify** by running scoped tests: `go test` and `go vet` on touched packages only (not `./...`)
4. **Commit** with message: `refactor: <what you improved>`

## Important Notes

- Only refactor code touched by this task, not the entire codebase
- Follow the project's existing patterns — don't introduce new conventions
- Small, safe improvements only — not wholesale rewrites

## Completion

When complete:
- Code quality improvements are committed (if any were needed)
- All tests still pass (behavior unchanged)
- Changes follow project conventions

Do NOT output any special completion markers - just complete the task and exit.
