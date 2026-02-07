---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T05:10:39-05:00"
id: between-iteration-command
source_spec: between-iteration-command
---

# Between-Iteration Command Implementation Plan

**Goal:** Run a user-configured shell command after each successful bead completion, enabling dogfooding workflows where Gromit rebuilds itself between iterations.

**Architecture:** Add a `BetweenIterationsCommand` string field to `LoopConfig` and a `runBetweenIterationsCommand()` helper on `Runner` that executes it via `sh -c`. The command runs after bead close/sync/status.json update but before thorough review checks.

**Tech Stack:** Go, `os/exec`, YAML config

**Spec:** `.gromit/specs/between-iteration-command.md`

---

## Architecture

**Overview:**
A single config field and a single runner method. When the field is set, the method runs the command via `sh -c` after each successful bead closure. When empty/absent, nothing happens (zero overhead).

**Integration Points:**
- `LoopConfig` in `internal/config/config.go` gets a new `BetweenIterationsCommand string` field
- `Runner.Run()` in `internal/runner/runner.go` calls `r.runBetweenIterationsCommand()` at line ~361, between the status.json write and the epic-completion check
- The method uses `exec.Command("sh", "-c", cmd)` with stdout/stderr piped to `r.output`
- On non-zero exit, logs a warning via `r.log()` and continues — does not stop the loop or affect bead status

**Data Flow:**
1. Config loaded from `gromit.yaml` → `cfg.Loop.BetweenIterationsCommand` populated
2. Bead completes successfully → Close → Sync → status.json update
3. `runBetweenIterationsCommand()` checks if command is non-empty, runs it, logs output
4. Loop continues to thorough review checks and next iteration

**Files to Modify:**
- `internal/config/config.go` — Add field to `LoopConfig`
- `internal/runner/runner.go` — Add method and call site
- `gromit.yaml` — Add `between_iterations_command: "make"` to `loop` section
- `internal/config/config_test.go` — Test YAML parsing
- `internal/runner/runner_test.go` — Test method behavior and loop integration

## Test Strategy

**Unit Tests:**

1. **Config parsing** — `BetweenIterationsCommand` loaded from YAML, empty/absent results in empty string, works alongside other loop settings
2. **`runBetweenIterationsCommand` method** — Empty string no-op, successful command output visible, failed command logs warning without returning error, shell features work via `sh -c`
3. **Loop integration** — Successful bead triggers command, failed bead skips it

**Mocking Strategy:**
- No new mocks needed — use real `exec.Command` with simple shell commands (`echo`, `false`)
- Existing `NewRunnerWithDeps` + mock infrastructure for loop integration tests

**Test Organization:**
- `internal/config/config_test.go` for config parsing (table-driven)
- `internal/runner/runner_test.go` for method unit tests and loop integration

## Implementation Tasks

### Task 1: Add BetweenIterationsCommand to LoopConfig

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `BetweenIterationsCommand string` field to `LoopConfig` struct with YAML tag `between_iterations_command`. No default value needed — empty string means no command. No changes to `setDefaults()` or `normalizeNilFields()`.

Add config tests: load from YAML with the field set, verify it's parsed; load without the field, verify empty string; load alongside other loop settings, verify all preserved.

**Acceptance Criteria:**
- `LoopConfig` has `BetweenIterationsCommand string` with correct YAML tag
- Config tests verify parsing from YAML and empty-when-absent behavior

**Dependencies:** None

### Task 2: Add runBetweenIterationsCommand method and wire into loop

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
Add a `runBetweenIterationsCommand()` method to `Runner` that:
1. Returns immediately if `r.cfg.Loop.BetweenIterationsCommand` is empty
2. Logs that it's running the command
3. Creates `exec.Command("sh", "-c", command)` with `cmd.Stdout = r.output` and `cmd.Stderr = r.output`
4. Runs the command
5. If error (non-zero exit), logs a warning via `r.log()` — does NOT return the error

Call this method in `Run()` at line ~361, after the status.json update block and before the epic-completion check.

Add unit tests for the method: empty command no-op, successful command output visible in buffer, failed command logs warning. Add a loop integration test using `NewRunnerWithDeps` that verifies the command runs after successful bead completion and does not run after failed bead.

**Acceptance Criteria:**
- Command runs after successful bead closure, before thorough review checks
- Empty/absent command is a no-op
- Non-zero exit logs warning and loop continues

**Dependencies:** Task 1

### Task 3: Update gromit.yaml with between_iterations_command

**Files:**
- Modify: `gromit.yaml`

**What to Do:**
Add `between_iterations_command: "make"` under the `loop` section in `gromit.yaml`, with a comment explaining its purpose. Place it after the existing `stuck_bead_threshold` line.

**Acceptance Criteria:**
- `gromit.yaml` has `between_iterations_command: "make"` in the `loop` section
- A comment explains the field's purpose

**Dependencies:** None (but logically follows Task 1)

---

## Notes

- The command runs via `sh -c` so shell features (pipes, `&&` chaining) work without users needing wrapper scripts.
- This is a convenience feature, not a correctness gate — validation commands are the correctness gate. That's why failure is warn-and-continue.
- The Gromit repo's own `gromit.yaml` should use `"make"` so that dogfooding picks up code improvements each iteration.
