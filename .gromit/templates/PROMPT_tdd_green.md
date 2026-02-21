# TDD Green Phase — Make the Failing Test Pass

## Role

You are in the **GREEN phase** of TDD. Write the MINIMUM production code to make the failing test pass. Nothing more.

**After you finish, the orchestrator will run tests automatically. Tests MUST PASS.** If tests still fail, the cycle is marked as failed.

**What you CAN do:**
- Create or modify production `.go` files (not `_test.go`)
- Add the types, functions, methods, or constants that the failing test needs
- Add imports needed by your production code

**What you MUST NOT do:**
- Modify any `_test.go` file — the test was written in the red phase and must not change
- Write more code than necessary to pass the test — no "while I'm here" additions
- Add features, helpers, or abstractions beyond what this one test requires
- Refactor existing code — that happens in the refactor phase

{{if .Rules}}
## Rules

{{.Rules}}
{{end}}

## Context

**Bead:** {{.BeadID}} - {{.BeadTitle}}

### Failing Test

```text
{{.FailingTest}}
```

### Failure Output

```text
{{.TestFailureOutput}}
```

### Current Implementation Files

{{range $path, $content := .ImplFileContents}}
#### `{{$path}}`
```text
{{$content}}
```
{{end}}

{{if .IsRetry}}
## Retry Context

{{if .FailureContext}}Failure analysis: {{.FailureContext}}{{end}}

Previous output:
```text
{{.PrevFailure}}
```
{{end}}

## Task

1. Read the failing test and its error output
2. Write the minimum production code to make that test pass
3. Do NOT add anything beyond what this test requires — the next red-green cycle will handle the next requirement
4. Commit with message: `green: implement <what you added>`
5. **STOP.** Do not write the next test. The next red phase handles that.

Self-check before committing: `{{.ScopedTestCommand}}`
- All tests MUST pass, including the new one from the red phase.
- If the test still fails, fix your implementation — do not modify the test.
