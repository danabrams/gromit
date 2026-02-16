# Pre-Compilation Check Design

## Problem

Beads can fail repeatedly when the codebase has pre-existing compilation errors. `gromit-bfx0` wasted 3 retries because acceptance tests referenced `config.DefaultInvocationTimeoutSeconds` — a constant that didn't exist. Each retry hit the same compilation error without any agent seeing the error output.

## Solution

Run `go build ./...` before each Claude build invocation. If compilation fails, append the errors to the build prompt so Claude can fix them as part of its work.

## Flow

```
processBead():
  ├─ setupBeadContext()
  ├─ buildPromptForBead()
  ├─ [NEW] runCompilationCheck() ← go build ./..., append errors to prompt
  ├─ ATDD phases
  ├─ escalationHandler.ExecuteWithRetry()
```

## Implementation

- New method `runCompilationCheck(ctx, bc)` on the Runner
- Runs `go build ./...` with a 30-second timeout
- On failure: appends `<compilation-errors>` section to `bc.BuildPrompt`
- On success: no-op
- Non-blocking — compilation failure never skips the bead
- Logged to iteration log (`CompilationErrors bool` field)

## Configuration

Single toggle under `preflight`:

```yaml
preflight:
  compile_check: true  # default true
```

## What this prevents

The exact `gromit-bfx0` scenario — wasted retries on pre-existing compilation failures that Claude could fix if told about them.

## What this doesn't do

No scope differentiation, no bead skipping, no new templates. Surfaces existing errors in the prompt, nothing more.
