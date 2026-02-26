---
id: bead-client-runfn-consolidation
source_ideas: []
created: 2026-02-19
epic: codebase-health
---

# Bead Client RunFn Consolidation

## Specification

`internal/bead.Client` exposes two function-injection fields with the same signature: exported `RunFn` and unexported `runFn`. The command execution path currently checks `RunFn`, then `runFn`, then falls back to spawning `bd` via `exec.Command`.

This spec consolidates command injection to a single field so behavior is unambiguous and test setup is consistent across packages.

Required behavior:
- `Client` supports exactly one command-execution injection hook: exported `RunFn func(args ...string) (string, error)`.
- `Client.run(args...)` uses `RunFn` when set, and otherwise executes the `bd` subprocess using current behavior (including `Dir` handling and stderr wrapping for exit errors).
- Internal tests that currently set `runFn` are updated to set `RunFn` instead, with no behavioral regressions.
- No second fallback injection path remains in `run()` after consolidation.

Out of scope:
- Changing the `RunFn` function signature.
- Altering `bd` command arguments or business logic in `Ready`, `ListWithLabel`, `Close`, or similar methods.
- Refactoring unrelated `RunFn`/`StreamRunFn` fields in non-`bead` packages.

## Acceptance Criteria

- `internal/bead/bead.go` defines only one function hook on `Client` for command injection, and it is exported as `RunFn`.
- `internal/bead.Client.run` checks only `RunFn` before subprocess fallback.
- Repository search for `runFn:` under `internal/bead` returns zero usages in production and test code.
- Existing tests that verify bead client command behavior continue to pass after switching test setup from `runFn` to `RunFn`.
- Tests outside `internal/bead` that construct `bead.Client{RunFn: ...}` continue to compile and pass unchanged.

## Decisions

1. **Keep `RunFn` as the canonical hook**  
   `RunFn` is already used by external-package tests (for example under `cmd/gromit`), so keeping it avoids reducing testability across package boundaries.

2. **Remove `runFn` instead of documenting dual semantics**  
   Two identical hooks with precedence rules add confusion and maintenance risk. A single hook gives deterministic behavior and simpler mental models.

3. **Treat this as a behavior-clarity change, not a CLI behavior change**  
   The subprocess fallback and command-level semantics remain the same; the change is focused on API clarity and consistency in test injection.

## Research & Context

### Current State

- `internal/bead/bead.go` defines both `RunFn` and `runFn` on `Client`, and `run()` currently checks `RunFn` first, then `runFn`, then subprocess fallback.
- In-package tests in `internal/bead` heavily use struct literals with `runFn: ...`.
- Cross-package tests already use `RunFn`, including:
  - `cmd/gromit/mock_bead_client_test.go`
  - `cmd/gromit/review_spec_base_commit_client_test.go`

### Why This Matters

The duplicated hook shape creates ambiguity about which field should be used and why precedence exists. Consolidating to one hook reduces accidental misuse and makes future maintenance and test authoring more predictable.
