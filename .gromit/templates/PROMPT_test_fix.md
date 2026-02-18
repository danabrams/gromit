# Fix Failing Tests

Your task is to fix the implementation so the failing tests pass. Do NOT modify tests or change test expectations.

{{if .Rules}}
## Rules (Non-Negotiable)

{{.Rules}}
{{end}}

## Project Context

{{if .ClaudeMD}}
{{.ClaudeMD}}
{{end}}

## Test Command

```
{{.TestCommand}}
```

## Test Failure Output

```
{{.TestFailureOutput}}
```

## Instructions

1. **Read the failing tests** to understand what behavior is expected
2. **Fix the implementation** to satisfy the test expectations
3. Do NOT modify test files or change test assertions
4. Do NOT change what the tests expect — fix the code to match the tests
5. Run the test command above to verify your fix passes
6. Commit your changes with a clear message

## Completion

When done:
- The failing tests now pass
- No test files were modified
- The fix is committed

Do NOT output any special completion markers - just complete the task and exit.
