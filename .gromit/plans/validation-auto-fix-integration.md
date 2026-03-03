---
id: validation-auto-fix-integration
source_spec: validation-auto-fix-integration
created: 2026-03-03
decomposed: false
---

# Integrated Validation Auto-Fixes Implementation Plan

**Goal:** Integrate trivial auto-fixes (gofmt/goimports) into the pipeline Validate stage so formatting issues are resolved locally before triggering LLM-based repair.

**Architecture:** Add an optional `AutoFixFn` to the `Validate` pipeline stage. On command failure, run the fix on changed files (identified via `git diff --name-only <StartCommit>`), then re-validate. Pass `StartCommit` through `pipeline.Input` and `TrivialAutoFixed` through `pipeline.Output`.

**Tech Stack:** Go, gofmt, goimports, git

**Spec:** `.gromit/specs/validation-auto-fix-integration.md`

---

## Architecture

**Overview:**
The pipeline `Validate` stage gains an optional auto-fix step between "command failed" and "return Block." When configured, it runs `gofmt -w` and `goimports -w` on files changed since `StartCommit`, re-runs the failed commands, and returns `Proceed` with `TrivialAutoFixed: true` if the fix resolved the issue.

**Key Components:**

1. **`internal/pipeline/validate/autofix.go`** (new): Production `AutoFixFn` — shells out to `git diff --name-only <startCommit>` to find changed `.go` files, then runs `gofmt -w` and `goimports -w` on each. Errors from the tools are ignored (fail-safe).

2. **`internal/pipeline/validate/validate.go`** (modify): Add `autoFixFn` field and `WithAutoFix` option method. On command failure (exitCode != 0), if autoFixFn is set and `StartCommit` is non-empty, call the fix, re-run all commands, and return `Proceed` + `TrivialAutoFixed: true` on success.

3. **`internal/pipeline/stage.go`** (modify): Add `StartCommit string` to `Input`, `TrivialAutoFixed bool` to `Output`.

4. **`internal/runner/constructor.go`** (modify): Create the production `AutoFixFn` and pass it via `validate.New(...).WithAutoFix(fn)`.

5. **`internal/runner/orchestrator_sequence.go`** (modify): Populate `StartCommit` in `buildInput`. Capture HEAD before build via `git rev-parse HEAD`.

6. **`internal/runner/orchestrator.go`** (modify): Read `validateOut.TrivialAutoFixed` and set it on the success-path `IterationLog`.

**Integration Points:**
- `Validate` struct gains optional `autoFixFn` — nil means no auto-fix (backward compatible)
- Orchestrator captures `git rev-parse HEAD` before build, passes as `StartCommit` in `pipeline.Input`
- Orchestrator reads `TrivialAutoFixed` from `pipeline.Output` and stamps it on `IterationLog`
- Auto-fix runs once per validation attempt (not per retry). The orchestrator retry loop handles repeated failures via LLM.

**Data Flow:**
```
Orchestrator: HEAD = git rev-parse HEAD → baseIn.StartCommit = HEAD
Build stage runs (may create commits)
Validate stage: command fails
  → autoFixFn(StartCommit) runs gofmt/goimports on changed files
  → Re-run all validation commands
  → Pass? → Output{Decision: Proceed, TrivialAutoFixed: true}
  → Fail? → Output{Decision: Block, ValidationFailures: [...]}
```

**Tradeoffs:**
- **`StartCommit` on `pipeline.Input`** vs. on `Validate` struct: Chose `Input` because it's per-iteration state. Other stages (e.g., review's git diff) could reuse it.
- **Single auto-fix attempt per validation call** vs. stale-fix detection: Chose single attempt for simplicity. The orchestrator retry loop already prevents infinite loops.
- **Reusing `runtypes.AutoFixFn` type signature**: Maintains consistency with the existing validation runner subsystem.

## Test Strategy

**Unit Tests** (`internal/pipeline/validate/`):
- `autofix_test.go`: Production `AutoFixFn` — changed file discovery, tool execution, error handling
- `validate_test.go` (extend): Auto-fix/re-validate sequence — fix succeeds → Proceed, fix fails → Block, fix errors → Block (fail-safe), nil autoFixFn → existing behavior, empty StartCommit → no auto-fix

**Key Test Cases:**
1. Auto-fix resolves formatting → `Proceed` + `TrivialAutoFixed: true`
2. Auto-fix doesn't resolve → `Block` with validation failures
3. Auto-fix tool errors → ignored, returns `Block` (fail-safe)
4. No `AutoFixFn` configured → existing behavior unchanged
5. Empty `StartCommit` → auto-fix skipped
6. No changed `.go` files → auto-fix is no-op, returns `Block`
7. `TrivialAutoFixed` flows to `IterationLog`

**Mocking Strategy:**
- Mock `CommandRunner` (already exists in validate tests)
- Mock `AutoFixFn` as `func(string) error` stub
- Production `AutoFixFn` tested with mock exec (don't shell out to real gofmt)

**Test Organization:**
- `internal/pipeline/validate/validate_test.go` — extend with auto-fix scenarios
- `internal/pipeline/validate/autofix_test.go` — new, tests production implementation

## Implementation Tasks

### Task 1: Add StartCommit and TrivialAutoFixed to pipeline types

**Files:**
- Modify: `internal/pipeline/stage.go`

**What to Do:**
Add `StartCommit string` field to `pipeline.Input` (after `TouchedPackages`, with doc comment). Add `TrivialAutoFixed bool` field to `pipeline.Output` (near `ValidationFailures`, with doc comment).

**Acceptance Criteria:**
- `pipeline.Input` has `StartCommit string` field
- `pipeline.Output` has `TrivialAutoFixed bool` field
- Existing tests compile and pass

**Dependencies:** None

### Task 2: Add auto-fix capability to Validate stage

**Files:**
- Modify: `internal/pipeline/validate/validate.go`
- Test: `internal/pipeline/validate/validate_test.go`

**What to Do:**
Add `autoFixFn func(startCommit string) error` field to `Validate` struct. Add `WithAutoFix(fn func(startCommit string) error) *Validate` option method. In `Run`, after a command fails with non-zero exit code:
1. If `autoFixFn` is nil or `in.StartCommit` is empty, fall through to existing Block behavior.
2. Call `autoFixFn(in.StartCommit)`. If it errors, ignore and fall through.
3. Re-run all validation commands from the beginning.
4. If all pass, return `Output{Decision: Proceed, TrivialAutoFixed: true}` and emit `ValidationPassEvent`.
5. If any still fail, return `Block` with the new failure summaries.

Add tests:
- Auto-fix resolves issue → Proceed + TrivialAutoFixed
- Auto-fix doesn't resolve → Block with failures
- Auto-fix errors → Block (fail-safe, same as no auto-fix)
- Nil autoFixFn → existing behavior
- Empty StartCommit → no auto-fix attempt

**Acceptance Criteria:**
- `Validate` accepts optional `AutoFixFn` via `WithAutoFix`
- On failure + auto-fix success, returns `Proceed` with `TrivialAutoFixed: true`
- On failure + auto-fix failure, returns `Block` with validation failures
- Auto-fix errors are swallowed (fail-safe)
- All existing validate tests still pass

**Dependencies:** Task 1

**Notes:** Extract the "run all commands" logic into a helper to avoid duplicating the command-execution loop.

### Task 3: Implement production AutoFixFn

**Files:**
- Create: `internal/pipeline/validate/autofix.go`
- Create: `internal/pipeline/validate/autofix_test.go`

**What to Do:**
Implement `NewAutoFixFn(runner func(ctx context.Context, name string, args ...string) error) func(startCommit string) error` that:
1. Runs `git diff --name-only <startCommit>` to get changed files.
2. Filters to `.go` files only.
3. Runs `gofmt -w <file>` on each.
4. Runs `goimports -w <file>` on each (if `goimports` is on PATH; skip gracefully if not).
5. Returns nil on success, error only if git diff itself fails.

The `runner` dependency allows testing without shelling out. In production, it will wrap `exec.CommandContext`.

Test with mock runner:
- Changed files discovered and formatted
- Non-`.go` files filtered out
- No changed files → no-op
- `goimports` not found → skipped gracefully
- `git diff` fails → returns error

**Acceptance Criteria:**
- Runs `gofmt -w` and `goimports -w` on changed `.go` files since `startCommit`
- Filters non-Go files
- Gracefully handles missing `goimports`
- Unit tests verify behavior with mock runner

**Dependencies:** Task 1

### Task 4: Wire auto-fix into the orchestrator

**Files:**
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/orchestrator_sequence.go`
- Modify: `internal/runner/orchestrator.go`

**What to Do:**

1. **constructor.go**: Create the production `AutoFixFn` using `validate.NewAutoFixFn(...)` with a real exec wrapper. Wire it into the validate stage via `validate.New(...).WithAutoFix(autoFixFn)`.

2. **orchestrator_sequence.go**: In `buildInput`, add a `startCommit` parameter and set `StartCommit: startCommit` on the returned `Input`.

3. **orchestrator.go**: Before calling `Build.Run`, capture the current HEAD via `exec.Command("git", "rev-parse", "HEAD")`. Pass it to `buildInput`. On the success path (line ~1104), set `baseIn.Result.TrivialAutoFixed = validateOut.TrivialAutoFixed` on the `IterationLog`.

**Acceptance Criteria:**
- Production `AutoFixFn` is created and wired to the Validate stage
- `StartCommit` is captured before build and passed through `pipeline.Input`
- `TrivialAutoFixed` from validate output is stamped on `IterationLog` on success path
- Existing orchestrator tests compile and pass

**Dependencies:** Tasks 2, 3

**Notes:** The `getGitHead` function pattern already exists in `internal/runner/methodology/refactor.go` — follow the same approach. Consider adding a `GetGitHeadFn` field to `OrchestratorConfig` for testability, defaulting to a real `git rev-parse HEAD` implementation.

---

## Notes

- The existing `internal/runner/validation/runner.go` `RunWithRecovery` is a reference implementation but operates on `BeadContext` (old runner subsystem). This plan ports the auto-fix concept to the pipeline architecture without touching the old runner.
- Auto-fix counts as part of the validate stage's single invocation — it does NOT consume a `MaxValidationRetries` attempt. The orchestrator retry loop is orthogonal.
- The `TrivialAutoFixed` field already exists on `IterationLog` (line 65 of `logger.go`) and `IterationResult` (line 90 of `runtypes/types.go`). We only need to set it from the pipeline path.
- `ComputeQualityScore` in `logger/quality_score.go` already penalizes `TrivialAutoFixed` iterations — no changes needed there.
