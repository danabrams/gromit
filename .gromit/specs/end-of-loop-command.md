---
id: end-of-loop-command
source_ideas: []
created: 2026-02-19
---

# End-of-Loop Command

## Specification

After the entire run loop completes — including epilogue (test-fix, review, retro) and session completion (sync, push) — Gromit can run a user-configured shell command. This enables workflows like sending notifications, triggering downstream CI jobs, or updating external dashboards when a full run finishes.

The command is a single string configured under `loop` in `gromit.yaml`:

```yaml
loop:
  end_of_loop_command: "notify-send 'Gromit run complete'"
```

When set, the command runs as the very last step of `finishRun()`, after session completion (git push, bd sync). It runs via the shell (`sh -c`). Stdout and stderr are shown in the Gromit output. If the command exits non-zero, `gromit run` itself exits with a non-zero status.

When the loop exits early due to an error (context cancelled, stuck beads, max consecutive skips), Gromit prompts the user interactively asking whether to still run the end-of-loop command. If stdin is not a terminal (non-interactive mode), the command is skipped on early exit.

When the field is empty or absent, no command runs (zero overhead).

## Acceptance Criteria

- A command configured in `loop.end_of_loop_command` runs after a clean loop completion, as the very last step of `finishRun()`
- The command runs after all epilogue steps and session completion (git push) have finished
- If the command exits non-zero, `gromit run` exits with a non-zero status
- Command stdout/stderr are visible in Gromit's output
- On early loop exit (error/cancellation), the user is prompted whether to run the command
- In non-interactive mode, the command is skipped on early loop exit
- When the field is empty or absent, no command runs and no prompt appears

## Decisions

1. **Single command string, not a list** Consistent with `between_iterations_command`. Users can invoke a script or chain commands with `&&` for complex sequences.

2. **Run via `sh -c`** Allows shell features (pipes, `&&` chaining) without requiring users to wrap everything in a script. Consistent with `between_iterations_command`.

3. **Fail-loud on command failure** Unlike `between_iterations_command` (which warns and continues), this command's failure propagates to `gromit run`'s exit code. The between-iterations command is a convenience mid-loop, but the end-of-loop command is a deliberate "final step" — if it fails, the caller should know.

4. **Prompt on early exit, skip in non-interactive** Early exits are ambiguous — the user may still want the notification, or it may be inappropriate (e.g., deploying after a failed run). Prompting lets the user decide. In CI/non-interactive contexts, the safe default is to skip.

5. **Runs after everything, including git push** The command is the absolute last step. This means the pushed state is final and any external action (notification, deployment) reflects the committed result.

## Research & Context

### Current State

The loop completion sequence lives in `finishRun()` in `internal/runner/run_init.go:177-206`:

1. `maybeRunFinalFullValidation()`
2. `updateGlobalStats()`
3. Set `clean_exit` flag in `state.json`
4. `runSessionEpilogue()` (test-fix, review, retro)
5. `checkRetroSuggestion()`
6. `runSessionCompletion()` (git sync, push)

The new command runs after step 6.

The between-iterations command (`runBetweenIterationsCommand()` in `internal/runner/lifecycle.go:394-423`) provides the implementation pattern: read the config field, bail if empty, run via `runCmd()`, handle output. The key difference is error handling — this command propagates failures instead of warning.

Config structs are in `internal/config/config.go`. `LoopConfig` (lines 142-151) already has `BetweenIterationsCommand`. The new field `EndOfLoopCommand` would be added alongside it.
