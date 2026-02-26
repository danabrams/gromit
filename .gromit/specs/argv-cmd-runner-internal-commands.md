---
id: argv-cmd-runner-internal-commands
source_ideas: []
created: 2026-02-19
epic: codebase-health
---

# Argv Cmd Runner For Internal Commands

## Specification

Refine runner command execution so internal, trusted commands no longer execute through `sh -c`.

The runner currently uses a string-based command runner (`defaultCmdRunner`) that invokes `sh -c`. This spec changes internal command execution paths to use explicit executable + argument vectors (`argv`) for safety and predictability, while preserving shell-based execution for intentionally user-configured command strings.

Behavioral requirements:

- Internal runner operations that execute built-in git/session commands use argv-based execution and do not pass through a shell.
- Existing environment behavior remains unchanged for these commands: `GIT_TERMINAL_PROMPT=0`, `CI=1`, `NONINTERACTIVE=1`, and `TERM=dumb`.
- Exit-code and stdout/stderr behavior remains compatible with current handling so existing retry/warning/stop logic in lifecycle flows still works.
- Configured command strings that are intentionally shell-like continue to work as before:
- `cfg.Session.TestCommand`
- `cfg.Loop.BetweenIterationsCommand`

Command-runner API behavior:

- The runner supports structured command execution for internal callers (program + args + workDir).
- A separate shell-string path is retained only for explicit configurable command-string features.
- Internal call sites in lifecycle/spec orchestration are migrated to structured execution and audited to avoid shell invocation.

## Acceptance Criteria

- `defaultCmdRunner` (or replacement internal structured runner) executes internal commands via direct process invocation (argv), not `sh -c`.
- Internal git command sites in `internal/runner/lifecycle.go` and `internal/runner/spec_orchestrator.go` use structured execution and preserve current observable outcomes (success, non-zero exit handling, retries, and warning/stop behavior).
- `cfg.Session.TestCommand` continues accepting shell syntax and still runs through a shell execution path.
- `cfg.Loop.BetweenIterationsCommand` continues accepting shell syntax and still runs through a shell execution path.
- Tests cover at least:
- structured runner success and non-zero exit behavior;
- regression coverage for session completion and acceptance-test commit flows;
- explicit compatibility coverage for shell-configurable commands.

## Decisions

1. **Mixed execution model**
   Internal commands migrate to argv execution for safety, while explicitly user-configured command strings stay shell-based for compatibility and flexibility.

2. **No behavior change for lifecycle policy**
   Retry, failure handling, and logging policy in session completion and artifact commit paths remain unchanged; only command invocation mechanics change.

3. **Audit-driven migration scope**
   Migration focuses on known internal call sites (`lifecycle`, `spec_orchestrator`) and leaves explicitly configurable command features (`epilogue` and loop hook) on the shell path by design.

## Research & Context

### Current State

- `internal/runner/helpers.go` defines `defaultCmdRunner` and currently invokes `exec.CommandContext(ctx, "sh", "-c", command)`.
- `internal/runner/lifecycle.go` uses runner command execution for internal git/session operations:
- session completion (`git pull --rebase`, `git push`, status verification);
- generated metrics/state add+commit flows;
- between-iterations command hook (configurable command string).
- `internal/runner/spec_orchestrator.go` stages/commits acceptance tests via command strings.
- `internal/runner/epilogue.go` executes `cfg.Session.TestCommand` for test-fix loops (configurable command string).

### Risk Framing

- Internal command strings are currently mostly trusted constants, but a shared `sh -c` pattern increases fragility and broadens shell injection surface if future call sites accidentally include untrusted or incorrectly escaped input.
- `git commit -m` message construction currently depends on shell quoting behavior; structured argv avoids quoting pitfalls.

### In-Scope

- Refactor runner command API and internal call sites needed to route built-in commands through argv execution.
- Preserve current env and exit-code semantics.
- Update/add tests for structured execution and compatibility boundaries.

### Out of Scope

- Changing user-facing config schema for command-string settings.
- Removing shell support for configurable commands.
- Broad refactors outside runner command invocation and directly related tests.
