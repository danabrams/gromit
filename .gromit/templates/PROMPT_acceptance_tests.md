# Acceptance Test Writing

You are writing acceptance tests for a task before implementation begins. This is the ATDD (Acceptance Test-Driven Development) workflow.

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

Your job is to write acceptance tests that verify the acceptance criteria for this task. **You must NOT write any implementation code.** Only create or modify test files.

1. **Explore the codebase** to understand:
   - What test framework and conventions are used (e.g., Go's testing package, table-driven tests)
   - Where test files are located (e.g., `*_test.go` files alongside implementation)
   - How existing tests are structured and named
   - What helper functions or test utilities exist

2. **Write acceptance tests** that:
   - Cover each acceptance criterion with at least one test
   - Follow existing test patterns and naming conventions in the project
   - Are integration/acceptance level tests, not unit tests (test behavior, not implementation details)
   - Will fail until the feature is implemented (test for the new behavior)
   - Are clear, readable, and maintainable

3. **Only modify test files** - do not write any implementation code:
   - Create new test files following the project's naming conventions
   - Add test cases to existing test files if appropriate
   - Do NOT modify implementation files (e.g., non-test .go files)
   - Do NOT stub out implementations to make tests pass

4. **Commit your changes** with a clear commit message like "test: add acceptance tests for [task title]"

## Important Notes

- These tests MUST fail when first run - they test behavior that doesn't exist yet
- If you find the behavior already exists, note this clearly in your response
- Focus on WHAT the system should do (acceptance criteria), not HOW it does it
- Each acceptance criterion should map to at least one test case
- Tests should be deterministic and not flaky
- Follow the project's testing conventions (table-driven tests, helper functions, etc.)

## Completion

When complete:
- Acceptance test files are created/modified
- Each acceptance criterion is covered by at least one test
- No implementation code has been written
- All changes are committed

Do NOT output any special completion markers - just complete the task and exit.
