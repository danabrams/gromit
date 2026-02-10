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
   - What shared setup helpers already exist — reuse them instead of writing new setup code

2. **Write acceptance tests** that:
   - Cover each acceptance criterion with at least one test
   - Follow existing test patterns and naming conventions in the project
   - Test behavior through the public API or command surface, not internal helper functions
   - Will fail until the feature is implemented (test for the new behavior)
   - Are clear, readable, and maintainable

3. **Only modify test files** - do not write any implementation code:
   - Create new test files following the project's naming conventions
   - Add test cases to existing test files if appropriate
   - Do NOT modify implementation files (e.g., non-test .go files)
   - Do NOT stub out implementations to make tests pass

4. **Commit your changes** with a clear commit message like "test: add acceptance tests for [task title]"

## What Makes a Good Acceptance Test

Acceptance tests verify **user-visible behavior**, not implementation details. Apply these principles:

- **Test through the public surface.** Call the function, method, or command that a user/caller would use. Do NOT test internal helpers directly — if `buildPrompt()` is private, test the command that calls it instead.
- **Test behavior, not mechanics.** Assert on outcomes ("the output contains X", "the file was created", "the error message says Y"), not on how the code achieves them. Never test that stdlib functions work (e.g., don't verify that `os.MkdirAll` creates directories or that `os.WriteFile` writes files).
- **One acceptance criterion per test case, not per test function.** Use table-driven tests or subtests (`t.Run`) to cover multiple scenarios. Multiple test functions that share identical setup are a sign you need a table-driven test instead.
- **Extract shared setup into helpers.** If two or more tests create the same directory structure, config, or mock setup, extract it into a `setupXxx(t *testing.T)` helper. Keep helpers in the same package as the tests.
- **Keep tests concise.** A 100-line test function with 30 lines of setup, 5 lines of action, and 65 lines of assertions is too long. Extract setup, use helpers, and trust that stdlib works.
- **No skipped tests.** Do not write tests with `t.Skip()` for features that don't exist yet or can't run in the test environment. Every test you write must be runnable and must fail for the right reason (missing implementation, not missing infrastructure).
- **Use build tags for true acceptance tests.** If the test requires external dependencies (real binaries, network, etc.), use `//go:build acceptance` so it runs separately from unit tests.

## Anti-Patterns to Avoid

Do NOT write tests that:
- Test Go standard library behavior (file creation, temp files, JSON marshaling)
- Call private/internal helper functions directly instead of the public API
- Duplicate 10+ lines of identical setup across multiple test functions
- Have `t.Skip()` because the test can't actually run
- Verify implementation details like file permissions, temp file naming patterns, or internal data structures that aren't part of the acceptance criteria
- Are actually unit tests labeled as "acceptance" — if it tests a single function with mocked dependencies, it's a unit test

## Completion

When complete:
- Acceptance test files are created/modified
- Each acceptance criterion is covered by at least one test
- Tests are concise — shared setup is extracted, related cases use table-driven tests
- No implementation code has been written
- All changes are committed

Do NOT output any special completion markers - just complete the task and exit.
