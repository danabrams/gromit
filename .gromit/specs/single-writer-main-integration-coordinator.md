---
id: single-writer-main-integration-coordinator
source_ideas: []
created: 2026-02-28
epic: codebase-health
accepted: true
---

# Single-Writer Main Integration Coordinator

## Specification

Introduce a single-writer integration model where no interactive command and no run-loop worker merges directly to `main`. All sessions write commits to isolated session branches, and one coordinator path performs integration into `main`.

This removes the recurring requirement for users to manually "commit to main and push" at the end of interactive sessions while preserving safe, deterministic merge behavior.

## Problem

Interactive sessions and run-loop work both produce valid changes, but integration ownership is currently split across multiple command paths. In practice, users still have to remember manual end-of-session git steps, and concurrent activity creates friction around merge timing and conflict handling.

## Goals

1. `main` has one integration owner.
2. Interactive sessions auto-commit their work and hand off automatically.
3. `gromit run` and interactive sessions can proceed concurrently without index/worktree contention.
4. Integration to `main` is automatic and safe by default.

## Non-Goals

- Perfectly eliminating all possible source conflicts.
- Introducing a required external daemon in phase 1.
- Replacing existing session worktree isolation patterns.

## Design

### 1. Single writer contract

- Session commands (`explore`, `refine`, `plan`, `decompose --review`, `debug`, `review`, `retro`) and run-loop workers write only to session branches.
- Only the integration coordinator updates `main`.

### 2. Session completion contract

- At session end, gromit performs auto-stage + auto-commit on the session branch.
- The branch is marked ready for integration; users are not asked to merge/push to `main`.
- If commit creation fails (for example unresolved conflicts in session), mark session as blocked and surface a clear recovery command.

### 3. Coordinator placement (phase 1)

- Coordinator runs inside the `gromit run` process between iterations.
- It reads integration queue state, attempts one-branch-at-a-time integration, updates status, and continues.

### 4. Integration sequence

For each queued branch:
1. Fetch/rebase branch on latest `origin/main`.
2. Run quality gates scoped to touched packages.
3. Integrate into `main`.
4. Push to remote.
5. Mark queue state and cleanup merged session branch/worktree metadata.

## Acceptance Criteria

- Interactive sessions do not require users to manually merge/push to `main`.
- `main` is updated only through coordinator code paths.
- Session commands and run-loop workers can run concurrently with isolated branches/worktrees.
- A successful integration records merged state and removes branch from pending queue.
- Errors in one branch integration do not terminate the entire run loop.

## Decisions

1. Auto-commit happens at session end, not in coordinator.
2. Coordinator is embedded in run-loop lifecycle first, daemonization deferred.
3. Single writer to `main` is enforced as architectural policy, not only convention.

## Research & Context

- Existing session worktree specs establish branch/worktree isolation and deferred merge hooks.
- Existing pain point is integration ownership, not worktree mechanics alone.
- This spec defines the ownership boundary needed to make automation reliable.

