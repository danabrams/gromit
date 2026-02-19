---
id: retro-review-worktree-launch-dir
source_ideas: []
created: 2026-02-19
---

# Retro And Review Worktree Launch Directory

## Specification

When `gromit retro` and interactive `gromit review` launch agent sessions, they should set the agent working directory based on run-loop activity:

- If the run loop is active, launch the agent in the interactive worktree directory.
- If the run loop is not active, launch without an explicit directory override (current/main worktree behavior).

Run-loop activity is determined from `.gromit/status.json` using PID-backed liveness checks (the existing `worktree.IsRunLoopActive(gromitDir)` behavior).

This spec is limited to retro and interactive review launch integration. It does not change broader multi-command worktree policy, merge policy, or non-interactive review behavior.

### Expected behavior

1. `gromit retro` runs analysis exactly as today.
2. Interactive `gromit review` builds and launches its interactive review prompt exactly as today.
3. On each interactive launch path, resolve launch directory:
   - Active run loop: use interactive worktree directory.
   - Inactive run loop: use empty directory override.
4. Both launch paths use directory-aware launch (`LaunchInDir`) so sessions execute in the resolved directory.
5. Existing retro state recording (`RecordRetro`) and output behavior stay unchanged.

### Out of scope

- Adding or changing worktree creation/cleanup policy.
- Extending this behavior to `refine`, `plan`, `debug`, `explore`, or non-interactive review.
- Changing run-loop merge-back behavior.
- Altering retro prompt content or analysis model routing.
- Altering review finding parsing, bead/backlog creation, or non-interactive review execution.

## Acceptance Criteria

- During `gromit retro` interactive mode, when `.gromit/status.json` indicates `running: true` and the PID is alive, the agent launch path uses a non-empty worktree directory.
- During interactive `gromit review`, when `.gromit/status.json` indicates `running: true` and the PID is alive, the agent launch path uses a non-empty worktree directory.
- During `gromit retro` and interactive `gromit review`, when run-loop status is missing, invalid, stale, or `running: false`, the launch path uses an empty directory override (legacy behavior).
- `gromit retro --non-interactive` does not attempt interactive launch and is unaffected by directory logic.
- `gromit review --interactive=false` (non-interactive mode) is unaffected by directory logic.
- Retro analysis generation and result printing remain unchanged relative to current behavior.
- Retro still records last-retro timestamp in state after successful interactive session.
- No behavior changes are introduced for commands other than `retro` and interactive `review`.

## Decisions

1. **Retro + interactive review scope** The refinement covers both user-facing interactive command paths that are expected to coexist with the run loop.

2. **Status-based detection** Run-loop detection uses existing `.gromit/status.json` + PID liveness semantics via `worktree.IsRunLoopActive`, avoiding new status mechanisms.

3. **Directory selection at launch boundary** The change is applied where each command starts the agent process, so prompt generation and downstream command behavior remain unchanged.

4. **Preserve inactive behavior** When run loop is not active, retro keeps current main-worktree launch behavior to avoid unnecessary workflow changes.

## Research & Context

### Current state in codebase

- `cmd/gromit/main.go` (`runRetro`) currently writes a temp prompt and calls `selectedAgent.Launch(promptPath)` for interactive retro sessions.
- `cmd/gromit/review.go` routes interactive review through `pipeline.ReviewInteractive(...)`.
- `internal/pipeline/pipeline.go` (`ReviewInteractive`) currently resolves an agent and calls `agent.Launch(promptPath)`.
- `internal/agent/agent.go` already supports `LaunchInDir(promptPath, dir)` and treats empty `dir` as current behavior.
- `internal/worktree/detect.go` already implements `IsRunLoopActive(gromitDir)` using `.gromit/status.json` fields (`running`, `pid`) plus process liveness checks.
- `internal/retro/retro.go` already includes `LaunchClaudeCode(..., dir string)`, but `runRetro` currently launches through resolved `agent` directly.

### Related existing specs

- `.gromit/specs/concurrent-interactive-worktrees.md` defines broader retro/review worktree concurrency goals.
- `.gromit/specs/parallel-interactive-command-worktrees.md` expands worktree isolation to more interactive commands.

This spec intentionally narrows to closing the retro/review launch-directory integration gap without reopening broader command-scope decisions.
