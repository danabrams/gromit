---
id: parallel-interactive-command-worktrees
source_ideas: []
created: 2026-02-15
---

# Parallel Interactive Command Worktrees

## Specification

Enable `gromit refine`, `gromit plan`, `gromit decompose --review`, `gromit explore`, and `gromit debug` to run safely in parallel with `gromit run` and with each other by launching each interactive session in its own git worktree.

This extends the existing worktree model (currently focused on retro/review) to all interactive commands that can modify tracked files.

### Problem

Today, these interactive commands launch from the main working tree:

- `gromit refine`
- `gromit plan`
- `gromit explore`
- `gromit debug`
- `gromit decompose --review` (interactive confirmation flow still tied to main tree writes)

When `gromit run` is active, or when multiple interactive commands are launched concurrently, they contend on a single index/worktree and produce git failures (checkout conflicts, lock contention, dirty tree assumptions, and merge friction).

The existing spec `.gromit/specs/concurrent-interactive-worktrees.md` explicitly scoped worktrees to retro/review only. That assumption no longer holds for real usage.

### Goals

1. Interactive commands can run concurrently with `gromit run` without touching the main working tree.
2. Interactive commands can run concurrently with each other.
3. Worktree isolation is transparent for normal users and defaults to on.
4. Merge-back remains governed by existing runner worktree merge policy.

### Non-Goals

- Rewriting the run loop merge strategy.
- Adding a new long-lived daemon for session orchestration.
- Changing command semantics beyond where they execute.

### Design

#### 1. Per-session worktrees (not one shared interactive worktree)

Use a dedicated worktree per interactive session:

- Path pattern: `<project>-gromit-<command>-<session-id>`
- Branch pattern: `gromit/<command>-<timestamp>-<rand>`

A shared persistent worktree is insufficient for parallel interactive commands because each session still shares one index and can conflict. Per-session worktrees remove that contention and make command-level parallelism explicit.

#### 2. Expand command integration scope

Adopt worktree launch for:

- `refine`
- `plan`
- `explore`
- `debug`
- `review` (already intended)
- `retro` (already intended)

`decompose` behavior:

- Default non-interactive decomposition can stay in main worktree (short-lived, deterministic CLI path).
- `decompose --review` interactive Claude session uses per-session worktree.

#### 3. Launch path

For each interactive command invocation:

1. Resolve `gromitDir` and project root.
2. Decide worktree use:
   - Use worktree when `worktree.enabled=true` and either:
     - run loop is active, or
     - `worktree.interactive_always=true` (new optional config).
3. Create session worktree and branch.
4. Launch selected agent with `LaunchInDir(promptPath, worktreePath)`.
5. Record created branch for merge-back.
6. Return command output as today.

#### 4. Session bookkeeping

Extend interactive state to include pending session branches created by interactive commands so merge-back does not rely only on branch name scanning heuristics.

Suggested fields in `interactive-state.json`:

- `pending_worktree_branches: []`
- `last_worktree_sessions: []` (bounded history, optional)

This is additive and backward-compatible.

#### 5. Merge-back policy

Reuse existing `run` merge behavior:

- merge between iterations when `worktree.auto_merge=true`
- conflict handling controlled by `worktree.merge_failure` (`warn`/`stop`)

No semantic changes to merge policy in this spec.

#### 6. Cleanup policy

Per-session worktrees must be cleaned up once their branch is merged, or by a best-effort cleanup pass:

- Successful merge: remove worktree directory, delete branch.
- On conflict/failure: keep worktree for manual resolution.
- Add follow-up command (optional): `gromit worktree cleanup` for stale/merged session worktrees.

### Configuration

Extend existing config:

```yaml
worktree:
  enabled: true
  auto_merge: true
  merge_failure: "warn"
  interactive_always: false  # new: use worktrees for interactive commands even when run loop is not active
```

Defaults preserve current non-run-loop behavior while fixing concurrent scenarios.

### Acceptance Criteria

- `gromit refine`, `gromit plan`, `gromit explore`, and `gromit debug` launch in isolated per-session worktrees when run loop is active.
- Two interactive commands launched concurrently do not share a worktree path or branch.
- Interactive commands use `agent.LaunchInDir(...)` with the session worktree path.
- Existing run loop merge-back picks up and attempts to merge all pending interactive branches.
- On successful merge, corresponding session worktree is removed.
- On merge conflict, session worktree is preserved and conflict is surfaced according to `merge_failure` policy.
- With `interactive_always=true`, interactive commands use worktrees even when run loop is inactive.
- With `worktree.enabled=false`, all commands keep legacy in-place behavior.

## Decisions

1. **Per-session worktrees over shared persistent worktree.** Required for true parallel interactive execution.
2. **Scope includes all interactive authoring commands.** User workflows do not treat refine/plan/debug/explore as second-class compared to retro/review.
3. **Config-default conservative mode.** Worktrees are guaranteed for concurrent runs; optional always-on mode supports teams that prefer strict isolation all the time.
4. **Reuse existing merge policy.** Minimize behavior changes and leverage runner machinery already in place.

## Research & Context

### Current code observations

- `run` wires a worktree manager in `internal/runner/runner.go` and merges pending branches in `internal/runner/lifecycle.go`.
- `refine`, `plan`, `explore`, and `debug` command paths currently launch agents without setting launch directory to a session worktree.
- `agent.LaunchInDir` already exists (`internal/agent/agent.go`) and can be reused for all interactive commands.
- Existing spec `.gromit/specs/concurrent-interactive-worktrees.md` limits worktree usage to retro/review and is now too narrow for concurrent command usage.

### Risk notes

- More worktrees mean higher filesystem churn; cleanup must be explicit and reliable.
- Merge conflicts become visible later (between run iterations), so conflict surfacing must remain clear and actionable.
- Any command that mutates backlog or bead metadata must continue to use shared `.beads` semantics correctly from session worktrees.
