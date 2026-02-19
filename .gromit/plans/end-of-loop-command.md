---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T18:22:23Z"
id: end-of-loop-command
source_spec: end-of-loop-command
---

# End-of-Loop Command Implementation Plan

**Goal:** Add a configurable end-of-loop shell command that runs as the final step of `gromit run`, with fail-loud semantics and early-exit prompting behavior.

**Architecture:** Extend `LoopConfig` with `end_of_loop_command`, add a dedicated runner execution path for this command, call it as the absolute last step in `finishRun()`, and handle early-error exits in `Run()` with interactive confirmation (skip in non-interactive mode).

**Tech Stack:** Go, existing runner lifecycle/state architecture, YAML config loading, Go test framework.

**Spec:** `.gromit/specs/end-of-loop-command.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a new `loop.end_of_loop_command` config field and a dedicated runner execution path that runs this command as the final step after successful `finishRun()`, while handling early-error exits with interactive confirmation.

**Key Components:**
1. **`internal/config/config.go` (`LoopConfig`)**: Add `EndOfLoopCommand string \`yaml:"end_of_loop_command"\``.
2. **`internal/runner/lifecycle.go`**: Add a reusable command runner method for end-of-loop behavior (prints stdout/stderr, returns error on non-zero/runner error).
3. **`internal/runner/run_init.go` (`finishRun`)**: Invoke end-of-loop command as the absolute last step, after epilogue + session completion.
4. **`internal/runner/runner.go` (`Run`)**: On early error exits, decide whether to run end-of-loop command:
   - interactive stdin: prompt user yes/no
   - non-interactive stdin: skip
   - if user says yes and command fails: propagate failure (non-zero run result)
5. **Prompt/TTY helper (runner package)**: Small internal helper for “interactive stdin?” and yes/no parsing, test-injectable where needed.

**Integration Points:**
- Reuse existing `runCmd(..., "sh -c")` behavior via `defaultCmdRunner`; no new subprocess mechanism.
- Keep between-iterations behavior unchanged (warn-and-continue); end-of-loop is fail-loud by design.
- Preserve current `finishRun()` ordering; append end-of-loop command after `runSessionCompletion()`.

**Data Flow:**
- **Clean completion**: loop finishes -> `finishRun()` runs validation/stats/epilogue/session completion -> end-of-loop command runs -> error propagates if command fails.
- **Early error exit**: `Run()` hits error return path -> if command configured:
  - interactive: prompt user to run it
  - non-interactive: skip
  - chosen execution failure overrides/returns error.
- **No config/empty string**: no-op, no prompt.

**Files to Modify:**
- `internal/config/config.go` - add loop field.
- `internal/config/config_test.go` - mirror existing between-iterations config tests for new field.
- `internal/runner/lifecycle.go` - add end-of-loop command execution helper.
- `internal/runner/run_init.go` - call end-of-loop command at end of `finishRun()`.
- `internal/runner/runner.go` - hook early-exit prompt/decision path.
- `internal/runner/runner_test.go` and/or `internal/runner/epilogue_test.go` - add behavioral tests for clean and early exits.

**Files to Create:**
- `internal/runner/end_of_loop_command_test.go` (optional if scenario count is large; otherwise extend existing runner tests).

**Tradeoffs:**
- **Prompt in runner vs config flag**: prompt-in-runner keeps behavior local to exit conditions and matches spec; no new config complexity.
- **TTY detection via stdin mode**: simple and dependency-free; avoids adding external terminal packages.
- **Early-exit hook in `Run()`**: ensures coverage of error paths that bypass `finishRun()`; minimal change to existing lifecycle ordering.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit Tests (config parsing)**
- Verify `loop.end_of_loop_command` defaults to empty when absent.
- Verify YAML deserialization for simple, complex, and empty command strings.
- Verify coexistence with other loop settings (mirroring current `between_iterations_command` tests).

2. **Runner Behavior Tests (core)**
- **Clean completion path**: command runs after session completion, and as last step.
- **Failure propagation**: non-zero end-of-loop command causes `finishRun()`/`Run()` to return error.
- **Output passthrough**: stdout/stderr from command are written to runner output.
- **No-op path**: missing/empty config causes no execution and no prompt.

3. **Early Exit Tests**
- Early error + interactive stdin + user confirms yes => command runs.
- Early error + interactive stdin + user answers no => command skipped.
- Early error + non-interactive stdin => command skipped (no prompt).
- Early error + user confirms yes + command fails => `Run()` returns failure.

**Key Test Cases:**
- `finishRun` executes command strictly after `runSessionCompletion` succeeded.
- `finishRun` returns command failure error when command exits non-zero.
- `Run` error path prompts on interactive and respects response.
- `Run` error path skips command in non-interactive mode.
- Command output appears in captured runner output buffer for both stdout/stderr.

**Mocking Strategy:**
- Use existing `cmdRunnerFn` injection to avoid real shell execution.
- For prompt behavior, inject lightweight reader/interactive-check function into runner (or test helper seams) rather than touching real `os.Stdin`.
- Reuse existing mock bead/router patterns in `internal/runner/runner_test.go`.

**Coverage Goals:**
- Cover both lifecycle endpoints:
  - normal completion via `finishRun()`
  - early-return error path via `Run()`
- Cover failure semantics difference from between-iterations command.
- Cover interactive vs non-interactive branching deterministically.

**Test Organization:**
- Add config tests near existing between-iterations tests in `internal/config/config_test.go`.
- Add runner tests either:
  - alongside existing command/session tests in `internal/runner/runner_test.go`, or
  - in a focused file `internal/runner/end_of_loop_command_test.go` with table-driven scenarios.
- Keep naming aligned with existing convention: `TestRunEndOfLoopCommand...`, `TestFinishRun...`, `TestRun...EarlyExit...`.

## Implementation Tasks

### Task 1: Add Config Surface for End-of-Loop Command

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `EndOfLoopCommand` to `LoopConfig` with YAML tag `end_of_loop_command`. Add parsing/default tests mirroring the existing `BetweenIterationsCommand` coverage style.

**Acceptance Criteria:**
- `loop.end_of_loop_command` deserializes from YAML exactly as provided.
- Missing field defaults to empty string with no behavior change.
- Existing loop fields continue to parse normally alongside the new field.

**Dependencies:**
- None.

**Notes:**
Use existing between-iterations tests as the pattern baseline for consistency.

### Task 2: Implement End-of-Loop Command Execution Primitive

**Files:**
- Modify: `internal/runner/lifecycle.go`
- Test: `internal/runner/runner_test.go` (or `internal/runner/end_of_loop_command_test.go`)

**What to Do:**
Add a dedicated method (for example `runEndOfLoopCommand() error`) that:
- no-ops when config/runner is nil or command is empty
- logs command invocation
- executes via `runCmd(context.Background(), command, "")`
- writes stdout/stderr to runner output
- returns error for command runner failure or non-zero exit code

**Acceptance Criteria:**
- Empty command produces no output and no execution.
- Stdout/stderr from the command are visible in run output.
- Non-zero exit and execution failures are returned as errors.

**Dependencies:**
- Task 1.

**Notes:**
Deliberately differs from `runBetweenIterationsCommand` by propagating failures.

### Task 3: Wire Clean-Completion Invocation as Final `finishRun()` Step

**Files:**
- Modify: `internal/runner/run_init.go`
- Test: `internal/runner/epilogue_test.go` (or runner lifecycle tests)

**What to Do:**
Call the new end-of-loop command method at the end of `finishRun()`, after epilogue/review suggestion and `runSessionCompletion()` (when not stop-line blocked).

**Acceptance Criteria:**
- On clean completion, command executes after session completion logic.
- If command fails, `finishRun()` returns an error.
- If `st.l3StopLine` is active, command still follows the chosen policy from Task 4 logic (no accidental premature invocation inside loop body).

**Dependencies:**
- Task 2.

**Notes:**
Preserve existing order and behavior of epilogue/session completion steps.

### Task 4: Add Early-Exit Prompt/Skip Flow in `Run()`

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/helpers.go` (or `internal/runner/lifecycle.go`)
- Test: `internal/runner/runner_test.go` (or dedicated new test file)

**What to Do:**
Introduce centralized early-exit handling for `Run()` errors:
- detect whether end-of-loop command is configured
- if early exit occurs and command is configured:
  - interactive stdin: prompt whether to run command
  - non-interactive stdin: skip command automatically
- if user confirms, run command and propagate command failure

**Acceptance Criteria:**
- Interactive early exit prompts user and respects yes/no response.
- Non-interactive early exit skips command without prompting.
- Command failure after “yes” causes `Run()` to return error.
- No prompt appears when command is empty or absent.

**Dependencies:**
- Task 2.

**Notes:**
Keep prompt parsing simple (`y/yes` true, `n/no/empty` false) and test-injectable.

### Task 5: Complete Regression Coverage and Ordering Assertions

**Files:**
- Modify: `internal/runner/runner_test.go`
- Modify: `internal/runner/epilogue_test.go` (if needed)

**What to Do:**
Add table-driven tests covering:
- clean completion success/failure behavior
- early-exit interactive/non-interactive branches
- command ordering relative to session completion commands
- no-op behavior with absent/empty config

**Acceptance Criteria:**
- All new acceptance criteria from the spec are represented by automated tests.
- Existing runner command tests remain green and unchanged in semantics.
- `go test` on touched packages passes.

**Dependencies:**
- Tasks 1-4.

**Notes:**
Prefer command-sequence assertions by capturing `cmdRunnerFn` invocations.

---

## Notes

- This feature intentionally has stricter failure semantics than `between_iterations_command`.
- Keep implementation local to runner lifecycle to avoid CLI-layer coupling.
- Ensure no additional overhead when command is absent/empty (fast no-op checks).
- During implementation, validate behavior against both normal completion and error-short-circuit control paths in `Run()`.
