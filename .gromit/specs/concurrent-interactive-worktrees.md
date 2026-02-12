---
id: concurrent-interactive-worktrees
source_ideas: []
created: 2026-02-12
---

# Concurrent Interactive Sessions via Git Worktrees

## Specification

Enable `gromit retro` and `gromit review` (interactive) to run concurrently with `gromit run` by isolating them in a separate git worktree. The run loop keeps the main worktree undisturbed while interactive commands operate in a persistent secondary worktree with their own branch. Changes merge back into the main worktree between run loop iterations.

### Problem

Today, running `gromit retro` or `gromit review` while the run loop is active causes git conflicts and file overwrites because both operations modify the same working tree and can make competing git commits.

### Architecture

Three layers:

1. **Worktree manager** (`internal/worktree/`) — creates, reuses, and cleans up a persistent worktree. Handles branch creation and merge-back operations.
2. **Command integration** — retro and review commands detect whether to use a worktree (based on whether the run loop is active or a flag) and launch their Claude Code session in that worktree.
3. **Run loop coordination** — between iterations, the run loop checks for completed interactive branches and merges them.

### Worktree Manager

New package `internal/worktree/` with a `Manager` that handles lifecycle:

```go
type Manager struct {
    MainDir     string // path to main worktree (project root)
    WorktreeDir string // path to interactive worktree
}

// EnsureWorktree creates the worktree if it doesn't exist, or
// verifies it's healthy if it does. Returns the worktree path.
func (m *Manager) EnsureWorktree() (string, error)

// CreateBranch creates a new branch in the worktree for an
// interactive session. Branch name: gromit/<command>-<timestamp>
func (m *Manager) CreateBranch(command string) (string, error)

// MergeBack merges a completed interactive branch into the current
// branch of the main worktree. Fast-forward when possible, merge
// commit otherwise.
func (m *Manager) MergeBack(branch string) error

// PendingBranches returns branches created by interactive sessions
// that haven't been merged yet.
func (m *Manager) PendingBranches() ([]string, error)

// Cleanup removes the worktree and prunes stale branches.
func (m *Manager) Cleanup() error
```

The worktree lives at a sibling directory: if the project is at `/home/user/myproject`, the worktree is at `/home/user/myproject-gromit-interactive`. This avoids nesting inside the project (which would require gitignore entries) and keeps the path predictable.

### bd Integration

bd already supports worktrees via a `redirect` file and the `bead.Client` already has a `Dir` field. When operating in a worktree:

- Set `bead.Client.Dir` to the worktree path so bd commands resolve correctly
- bd's redirect mechanism points the worktree's `.beads/` back to the main repo's `.beads/`
- This means bead state is shared (both worktrees see the same beads)

### agent.Launch() Changes

`agent.Launch()` currently always uses the current working directory. Add a `Dir` option:

```go
type LaunchOptions struct {
    Dir string // working directory for the agent process
}

func (a *Agent) LaunchInDir(promptPath string, dir string) error
```

When retro/review launch in a worktree, they pass the worktree path as the working directory.

### State Split

Split `state.json` into two files to eliminate concurrent-write conflicts:

| File | Owner | Fields |
|------|-------|--------|
| `state.json` | Run loop | `clean_exit`, `iterations_since_review`, `provider_counts`, `provider_unavailable_until`, `updated_at` |
| `interactive-state.json` | Interactive commands | `last_retro`, `last_review_commit`, `last_review_iteration`, `filtered_learning_hashes`, `updated_at` |

Both files live in `.gromit/` in the main worktree. The interactive worktree reads them via symlink or explicit path rather than maintaining its own copy.

### Retro Integration

When `gromit retro` runs:

1. Check if run loop is active (via `status.json` PID + liveness check)
2. If active: use worktree manager to ensure worktree exists, create branch `gromit/retro-<timestamp>`, launch Claude Code in the worktree directory
3. If not active: run in the main worktree as today (no behavior change)
4. On session end: commit changes, record branch name for merge-back

Retro modifies LEARNINGS.md and RULES.md. These changes merge cleanly because the run loop only reads these files (never writes them during iterations).

### Review Integration

When `gromit review` (interactive) runs:

1. Same detection logic as retro
2. Create branch `gromit/review-<timestamp>`, launch in worktree
3. Review may modify source files — these could conflict with run loop changes
4. On session end: commit changes, record branch name for merge-back

Review has higher conflict potential than retro since it touches source files. The merge-back step may require conflict resolution.

### Run Loop Merge-Back

Between iterations (after bead completion, after git push, before the between-iterations command), the run loop:

1. Calls `worktree.Manager.PendingBranches()`
2. For each pending branch, attempts `MergeBack()`
3. If merge succeeds: log it, delete the branch
4. If merge conflicts: log a warning, skip (user resolves manually)

This is a best-effort integration. The run loop never blocks on interactive work.

### Configuration

New config section in `gromit.yaml`:

```yaml
worktree:
  enabled: true              # Enable worktree isolation for interactive commands (default: true)
  auto_merge: true           # Merge interactive branches between iterations (default: true)
  merge_failure: "warn"      # What to do on merge conflict: "warn" | "stop" (default: "warn")
```

### Commands That Use Worktrees

| Command | Worktree | Reason |
|---------|----------|--------|
| `gromit retro` | Yes | Modifies LEARNINGS.md, RULES.md |
| `gromit review` (interactive) | Yes | Modifies source files |
| `gromit explore` | No | Creates specs/epics/backlog — doesn't conflict with run loop |
| `gromit refine` | No | Creates/modifies beads only |
| `gromit plan` | No | Creates/modifies beads only |
| `gromit decompose` | No | Creates/modifies beads only |

## Acceptance Criteria

- `gromit retro` runs in a worktree when the run loop is active, without conflicts.
- `gromit review` (interactive) runs in a worktree when the run loop is active, without conflicts.
- When the run loop is not active, retro and review run in the main worktree (no behavior change).
- Completed interactive branches are merged back between run loop iterations.
- Merge conflicts are warned about, not fatal (unless configured otherwise).
- state.json is split so run loop and interactive commands never corrupt each other's state.
- bd commands in the worktree correctly resolve beads from the main repo.
- `gromit retro` and `gromit review` work correctly when run outside of a concurrent run loop (no regression).

## Decisions

1. **Persistent worktree, not ephemeral.** Creating worktrees is cheap but Claude Code session context has overhead. A persistent worktree at `<project>-gromit-interactive` is reused across sessions.

2. **Sibling directory placement.** The worktree lives next to the project, not inside it. Avoids gitignore pollution and keeps the project directory clean.

3. **Auto-merge between iterations by default.** Most interactive changes (learnings, rules, review fixes) should flow back promptly. Users who want manual control can disable `auto_merge`.

4. **Warn-and-continue on merge conflict.** A merge conflict shouldn't kill a productive run. The user can resolve conflicts at their convenience.

5. **Worktree only when run loop is active.** No reason to add worktree overhead when there's no concurrency. Detection uses `status.json` PID + process liveness.

6. **State split over file locking.** Two files with clear ownership is simpler and more robust than flock-based locking, especially across worktrees where flock semantics can be surprising.

7. **bd redirect mechanism for bead sharing.** Both worktrees see the same bead state. This is safe because retro/review read beads but don't modify them in ways that conflict with the run loop's bead operations (close, create).

## Research & Context

### Existing Infrastructure

- `bd` already supports worktrees via a `redirect` file in `.beads/` and a `Dir` field on `bead.Client`
- `agent.Launch()` currently has no Dir support — needs a Dir parameter added
- `status.json` already tracks PID for stale process detection — reusable for "is run loop active?" checks
- The run loop already has a between-iterations hook point (after git push, before between-iterations command) — natural place for merge-back

### Git Worktree Semantics

- Worktrees share the same `.git` database — commits from either side are visible to both
- Each worktree has its own working tree and index — no file-level conflicts during operation
- Branches can be checked out in only one worktree at a time — interactive branches must differ from the main branch
- `git worktree add <path> -b <branch>` creates both the worktree and a new branch atomically

### Conflict Surface

- LEARNINGS.md and RULES.md: low conflict risk (run loop reads, retro writes)
- Source files: medium conflict risk (run loop's Claude and review's Claude may touch the same files)
- state.json: eliminated by split
- .beads/: shared via redirect, low conflict risk (different bead operations)
