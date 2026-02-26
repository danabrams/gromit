---
id: between-iteration-command
source_ideas: []
created: 2026-02-07
epic: run-loop-reliability
---

# Between-Iteration Command

## Specification

After each successful bead completion, Gromit can run a user-configured shell command before moving on to the next iteration. This enables dogfooding workflows where the tool being developed by Gromit is also the tool running the loop — rebuilding between iterations ensures each subsequent bead uses the latest version.

The command is a single string configured under `loop` in `gromit.yaml`:

```yaml
loop:
  between_iterations_command: "make"
```

When set, the command runs after a bead is closed and bd state is synced, but before thorough review checks. It runs via the shell (`sh -c`). Stdout and stderr are shown in the Gromit output. If the command fails (non-zero exit), Gromit logs a warning and continues to the next iteration — it does not stop the loop or affect bead status.

When the field is empty or absent, no command runs (current behavior, zero overhead).

## Acceptance Criteria

- A command configured in `loop.between_iterations_command` runs after each successful bead closure
- The command does not run after failed iterations
- If the command exits non-zero, a warning is logged and the loop continues
- Command stdout/stderr are visible in Gromit's output

## Decisions

1. **Single command string, not a list** — The immediate use case is `make`. A single string keeps config simple. Users can invoke a script or Makefile target for complex sequences. Can be expanded to a list later if needed.

2. **Run via `sh -c`** — Allows shell features (pipes, `&&` chaining) without requiring users to wrap everything in a script.

3. **Warn-and-continue on failure** — The between-iteration command is a convenience, not a correctness gate. A build failure here shouldn't block progress on other beads. Validation commands are the correctness gate.

4. **Runs after bead close, before thorough review** — The command runs at line ~361 in `runner.go`, after `beads.Close()` and `beads.Sync()` but before epic-completion and periodic review checks. This ensures the bead is fully committed before the command runs, and the rebuilt binary is available for any subsequent thorough review.

## Research & Context

### Current State

The main loop lives in `internal/runner/runner.go:Run()`. The success path after a bead completes (lines 345-387) currently:
1. Closes the bead (`beads.Close`)
2. Syncs bd state (`beads.Sync`)
3. Updates `status.json`
4. Checks epic completion for thorough review trigger
5. Increments review counter and checks periodic thorough review trigger

The new command runs between steps 3 and 4.

Config structs are in `internal/config/config.go`. `LoopConfig` (line 44) currently has `MaxIterations`, `StopOnFailure`, `StuckBeadThreshold`, and `LearnFromSuccess`. The new field `BetweenIterationsCommand` would be added here.

### This Project

The Gromit repo's `Makefile` runs `go build -o gromit ./cmd/gromit && go install ./cmd/gromit`. The project's `gromit.yaml` should be updated to set `loop.between_iterations_command: "make"` so that dogfooding picks up improvements each iteration.
