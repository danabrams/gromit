---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T18:27:47Z"
id: argv-cmd-runner-internal-commands
source_spec: argv-cmd-runner-internal-commands
---

# Argv Cmd Runner for Internal Commands — Implementation Plan

**Goal:** Migrate internal runner command execution from `sh -c` to direct argv invocation for safety and predictability, while preserving shell-based execution for user-configured command strings.

**Architecture:** Dual-path execution model — a new `runArgv` method for internal git/system commands using `exec.CommandContext(ctx, program, args...)`, alongside the existing `runCmd`/`defaultCmdRunner` shell path retained exclusively for user-configured commands (`TestCommand`, `BetweenIterationsCommand`, `CompileCommand`).

**Tech Stack:** Go, `os/exec`

**Spec:** `.gromit/specs/argv-cmd-runner-internal-commands.md`

---

## Architecture

### Overview

Add a structured argv execution path parallel to the existing shell execution path. Both paths share identical environment setup (`GIT_TERMINAL_PROMPT=0`, `CI=1`, `NONINTERACTIVE=1`, `TERM=dumb`) and return the same `(stdout, stderr, exitCode, error)` tuple, so all existing retry/warning/stop logic works unchanged.

### Key Components

1. **`ArgvRunnerFn` type** (`runtypes/types.go`): `func(ctx, program string, args []string, workDir string) (stdout, stderr string, exitCode int, err error)` — injectable function type matching the existing `CmdRunnerFn` pattern.

2. **`defaultArgvRunner`** (`helpers.go`): Direct process invocation via `exec.CommandContext(ctx, program, args...)` with identical env setup and exit-code handling as `defaultCmdRunner`.

3. **`runArgv` method** (`helpers.go`): Runner method that delegates to injectable `argvRunnerFn`, falling back to `defaultArgvRunner` — mirrors `runCmd` pattern.

4. **`Deps.ArgvRunner`** (`constructor_with_deps.go`): Injectable field for test mockability.

### Integration Points

- `lifecycle.go`: ~10 internal git commands migrate from `r.runCmd` to `r.runArgv`
- `spec_orchestrator.go`: 2 git add/commit calls migrate
- `codex_preflight.go`: 1 `codex login status` call migrates
- `spec_gate.go`: internal test/diff commands migrate
- `epilogue.go`, `process.go`, `run_iteration.go`: **no changes** — user-configured commands stay on shell path

### Data Flow

```
Internal:  r.runArgv(ctx, "git", ["push"], "") → exec.Command("git", "push") → (stdout, stderr, exitCode, err)
User cmd:  r.runCmd(ctx, "npm test", "")        → exec.Command("sh", "-c", "npm test") → (stdout, stderr, exitCode, err)
```

### Tradeoffs

- **Two execution methods**: Explicit separation prevents accidental shell use for internal commands. The cost is two similar-but-distinct code paths — justified by the different trust levels.
- **Same env on both paths**: Keeps observable behavior identical during migration.

---

## Test Strategy

### Unit Tests (`helpers_test.go`)

- `defaultArgvRunner` success: captures stdout, stderr, returns exit code 0
- `defaultArgvRunner` non-zero exit: exit code returned separately, not as error
- `defaultArgvRunner` failure: program not found returns -1 + error
- `defaultArgvRunner` environment: verifies all four env vars set
- `runArgv` injection: delegates to `argvRunnerFn` when set
- `runArgv` fallback: uses `defaultArgvRunner` when nil

### Migration Regression Tests

- Session completion: verify git pull/push/status go through `argvRunnerFn`
- Metrics/state commit: verify git status/add/commit go through `argvRunnerFn`
- Spec orchestrator: git add/commit for acceptance tests go through `argvRunnerFn`
- Non-zero exit handling preserved in retry loops

### Shell-Path Compatibility Tests

- `BetweenIterationsCommand` still uses `cmdRunnerFn` (existing tests pass unchanged)
- `TestCommand` in epilogue still uses `cmdRunnerFn`
- `CompileCommand` still uses `cmdRunnerFn`

### Audit Test

- Update the subprocess audit in `runner_test.go` to track both `r.runCmd` and `r.runArgv` known counts

### Mocking Strategy

- Inject `ArgvRunnerFn` via `Deps` — capture `(program, args)` tuples to assert exact argv vectors
- Existing `CmdRunner` mocks unchanged for user-configurable command tests

---

## Implementation Tasks

### Task 1: Add ArgvRunnerFn type and defaultArgvRunner

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/runner/helpers.go`
- Test: `internal/runner/helpers_test.go`

**What to Do:**
Add `ArgvRunnerFn` type to `runtypes/types.go` with signature `func(ctx context.Context, program string, args []string, workDir string) (stdout, stderr string, exitCode int, err error)`. Implement `defaultArgvRunner` in `helpers.go` using `exec.CommandContext(ctx, program, args...)` with the same env setup and exit-code handling as `defaultCmdRunner`. Add `runArgv` method on Runner that delegates to `argvRunnerFn` field, falling back to `defaultArgvRunner`. Add `argvRunnerFn` field to Runner struct in `runner.go`.

**Acceptance Criteria:**
- `defaultArgvRunner` executes a program with explicit args without shell interpretation
- `defaultArgvRunner` sets `GIT_TERMINAL_PROMPT=0`, `CI=1`, `NONINTERACTIVE=1`, `TERM=dumb`
- Exit-code and error handling matches `defaultCmdRunner` behavior (non-zero exit as exitCode, exec failure as -1 + error)
- Unit tests cover success, non-zero exit, exec failure, env vars, injection, and fallback

**Dependencies:** None

### Task 2: Wire ArgvRunner into Deps and constructor

**Files:**
- Modify: `internal/runner/constructor_with_deps.go`

**What to Do:**
Add `ArgvRunner ArgvRunnerFn` field to the `Deps` struct. In `newRunnerWithDepsImpl`, default to `defaultArgvRunner` when `deps.ArgvRunner` is nil, and wire it to `r.argvRunnerFn`. Also wire `argvRunnerFn` into `SpecOrchestrator` construction (add matching field to SpecOrchestrator, add `runArgv` method).

**Acceptance Criteria:**
- `Deps.ArgvRunner` is injectable and defaults to `defaultArgvRunner`
- `SpecOrchestrator` receives and uses the argv runner
- Existing tests compile and pass without changes (nil `ArgvRunner` falls back correctly)

**Dependencies:** Task 1

### Task 3: Migrate lifecycle.go internal commands to runArgv

**Files:**
- Modify: `internal/runner/lifecycle.go`
- Test: `internal/runner/runner_test.go` (or `lifecycle_test.go` if it exists)

**What to Do:**
Replace all `r.runCmd(ctx, "<git command string>", workDir)` calls for internal git operations with `r.runArgv(ctx, "git", []string{...}, workDir)`. This covers: `git pull --rebase`, `git push`, `git status --short --branch`, `git status --porcelain -- .gromit/metrics`, `git add .gromit/metrics`, `git commit -m ...` (metrics), `git status --porcelain -- .gromit/state.json`, `git add .gromit/state.json`, `git commit -m ...` (state). Leave `BetweenIterationsCommand` on `r.runCmd`. For commit messages, pass `-m` and the message as separate args to avoid shell quoting.

**Acceptance Criteria:**
- All internal git commands in lifecycle.go use `r.runArgv` with explicit argv vectors
- `BetweenIterationsCommand` remains on `r.runCmd` (shell path)
- Session completion retry loop still works with non-zero exit codes from argv runner
- Tests verify argv runner receives correct program and args for at least git pull, git push, and git commit flows

**Dependencies:** Task 2

**Notes:** The `git commit -m "chore: ..."` calls currently rely on shell quoting. With argv, pass `"-m"` and the message as separate args — `exec.Command` handles this correctly without quoting.

### Task 4: Migrate spec_orchestrator.go and remaining internal commands

**Files:**
- Modify: `internal/runner/spec_orchestrator.go`
- Modify: `internal/runner/codex_preflight.go`
- Modify: `internal/runner/spec_gate.go`
- Test: corresponding test files

**What to Do:**
Migrate `spec_orchestrator.go` git add/commit calls to use `o.runArgv`. Migrate `codex_preflight.go` `codex login status` to argv. Migrate internal commands in `spec_gate.go` (git diff, go test invocations that are not user-configurable). For spec_gate, audit each command: if it's an internal constant, migrate; if it's derived from user config, leave on shell path.

**Acceptance Criteria:**
- Spec orchestrator git add and git commit use argv execution
- Codex preflight login check uses argv execution
- Spec gate internal commands use argv execution
- All existing tests pass; new tests cover argv vectors for migrated call sites

**Dependencies:** Task 2

### Task 5: Update subprocess audit and verify shell-path preservation

**Files:**
- Modify: `internal/runner/runner_test.go`

**What to Do:**
Update the subprocess call audit test (currently at ~line 2276) that grep-counts `r.runCmd` usage. Add a parallel audit for `r.runArgv` call sites. Adjust `knownCount` for `r.runCmd` to reflect only the remaining shell-path call sites (`BetweenIterationsCommand`, `TestCommand`, `CompileCommand`). Add `knownCount` for `r.runArgv` covering all migrated internal command sites. Add explicit assertions that user-configurable command call sites (`epilogue.go`, `process.go` compile check, `run_iteration.go` between-iterations) still use `r.runCmd`.

**Acceptance Criteria:**
- Audit test passes with correct known counts for both `r.runCmd` and `r.runArgv`
- Any future addition of `r.runCmd` for an internal command or `r.runArgv` for a user command will fail the audit
- `go test ./internal/runner/...` passes clean

**Dependencies:** Tasks 3, 4

---

## Notes

- The `git commit -m` calls in lifecycle.go currently construct the message inline in the command string (e.g., `fmt.Sprintf("git commit -m %q ...", msg)`). With argv, these become `[]string{"commit", "-m", msg}` — simpler and quoting-safe.
- The `git add -- ':(glob)**/*_acceptance_test.go'` pathspec in spec_orchestrator.go uses a git pathspec glob. With argv this becomes `[]string{"add", "--", ":(glob)**/*_acceptance_test.go"}` — the pathspec is interpreted by git, not the shell, so it works the same way.
- The spec_gate.go commands need individual audit — some are internal constants, some may be derived from config. Task 4 notes this explicitly.
- `codex login status` in codex_preflight.go should become `runArgv(ctx, "codex", []string{"login", "status"}, "")`.
