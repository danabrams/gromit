---
id: integration-queue-and-branch-lifecycle
source_ideas: []
created: 2026-02-28
epic: codebase-health
accepted: true
---

# Integration Queue And Branch Lifecycle

## Specification

Define a durable branch lifecycle and FIFO integration queue for session-produced changes. All session branches transition through explicit states so integration is observable, restart-safe, and non-lossy.

## Problem

Without explicit queue state, branch handoff from session completion to automated integration is fragile: branches can be retried inconsistently, errors are hard to diagnose, and users cannot see what is pending or blocked.

## Goals

1. Durable queue state with crash recovery.
2. Deterministic FIFO ordering.
3. Clear branch states and transition reasons.
4. Status visibility for queue position and failure causes.

## Non-Goals

- Priority scheduling in phase 1.
- Manual cherry-pick orchestration UX.
- Replacing git branch truth with an in-memory queue.

## Design

### 1. Queue storage

- Add a durable integration queue file under `.gromit/` (JSON format).
- Queue entries contain:
  - branch name
  - originating command/session id
  - changed-file summary hash
  - created/updated timestamps
  - attempt counters
  - last error and classification

### 2. Branch states

- `draft`: session active, not ready.
- `ready`: session committed and queued.
- `integrating`: coordinator currently processing.
- `merged`: integrated and pushed.
- `conflict`: rebase/merge conflict requiring user action.
- `failed_gates`: validation failed after allowed retries.
- `lane_violation`: hard safety policy violation.

### 3. Transition rules

1. Session start creates `draft`.
2. Successful auto-commit moves to `ready`.
3. Coordinator picks oldest `ready` entry (FIFO), marks `integrating`.
4. On success: `merged`.
5. On conflict: `conflict`.
6. On gate failure after retry policy: `failed_gates`.
7. On hard safety violation: `lane_violation`.

### 4. Retry policy

- One automatic retry after a fresh rebase for gate failures.
- No silent infinite retries.
- Failed entries remain visible for explicit user requeue.

### 5. Status UX

`gromit status` must show:
- queue length
- each branch state
- FIFO position for `ready`
- latest failure reason for non-merged states

## Acceptance Criteria

- Restarting gromit preserves queue entries and in-flight branch states.
- FIFO ordering is enforced for `ready` branches.
- No ready branch is dropped without terminal state transition.
- One gate retry occurs automatically, then branch moves to `failed_gates`.
- `gromit status` reports queue states and last error details.

## Decisions

1. FIFO is the default and only ordering policy in phase 1.
2. Queue state is durable on disk, not ephemeral in memory.
3. Branch lifecycle states are explicit and user-visible.

## Research & Context

- Current merge-back behavior already has deferred branch processing points.
- This spec formalizes lifecycle and observability needed for dependable automation.

