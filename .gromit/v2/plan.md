---
id: immutable-pipeline
source_spec: immutable-pipeline
created: 2026-03-08
---

# Implementation Plan: Immutable Pipeline

## Overview

Implement per-stage git commits and event log preservation for v2 spec execution. Every meaningful stage (Plan, Decompose, Build, Validate, Review, Accept, Present) commits code changes and the event log to the spec's worktree branch. Structured commit messages encode bead ID, stage name, and iteration for machine parsing.

## Implementation Tasks

### Task 1: Add structured commit message formatting and parsing

Create utilities to encode and decode commit messages in the format `[bead:<id>/<stage>/iter:<n>] <Decision>` for spec-level stages (e.g., `[spec/plan/iter:1] Proceed`). Include validation that messages contain required fields (bead ID or "spec", stage name, iteration number, decision).

Files: `internal/v2/commit/message.go`, `internal/v2/commit/message_test.go`

Acceptance Criteria:
- Format function produces correct structured messages for both bead and spec stages
- Parse function extracts bead ID, stage name, iteration, and decision from valid messages
- Invalid messages are rejected with clear error messages

### Task 2: Wire git commit hook into the run loop after each stage

Modify the spec loop and bead loop to create commits after each stage completes (Gate excluded). Each commit should include code changes plus the updated event log. The loop decides when to commit, not the stage.

Files: `internal/v2/loop/spec_loop.go`, `internal/v2/loop/bead_loop.go`, `internal/v2/adapter/git/commit.go`

Acceptance Criteria:
- After Plan, Decompose, Build, Validate, Review, Accept, Present stages complete, a git commit is created
- Each commit includes both code artifacts and the updated `.gromit/v2/events.jsonl`
- Commit message follows structured format with stage name and iteration number
- Gate and Epilogue stages do not trigger commits

### Task 3: Implement event log file subscriber for v2 event system

Create a new event subscriber that writes events to `.gromit/v2/events.jsonl` in the spec's worktree. Wire it alongside existing CLI and API subscribers.

Files: `internal/v2/event/file_subscriber.go`, `internal/v2/event/file_subscriber_test.go`, `internal/v2/event/wiring.go`

Acceptance Criteria:
- File subscriber appends events to `.gromit/v2/events.jsonl` in JSONL format
- Events are written in cumulative order (prior events preserved on each stage commit)
- Subscriber handles file creation and rotation correctly
- Events are correlated by bead ID, stage name, and iteration number

### Task 4: Implement per-bead squash logic in Present stage

When Present creates a PR, combine per-stage commits into one commit per bead with the bead title as the message. The full stage-level history remains on the worktree branch.

Files: `internal/v2/stage/present/squash.go`, `internal/v2/stage/present/squash_test.go`, `internal/v2/stage/present/present.go`

Acceptance Criteria:
- Per-bead squash combines all stage commits for each bead into one commit
- Squashed commit message is derived from the bead's title
- Full stage-level history is preserved on the worktree branch after squash
- PR shows one clean commit per bead

### Task 5: Preserve worktree branch on failure, delete on success

Modify the spec loop to preserve the worktree branch when Andon is triggered or generation cap is hit. On successful completion (PR merged), delete the worktree branch.

Files: `internal/v2/loop/spec_loop.go`, `internal/v2/adapter/git/worktree.go`

Acceptance Criteria:
- On failure (Andon or generation cap), worktree branch is preserved with full commit and event history
- On success (PR merged), worktree branch is deleted
- Branch preservation/deletion decisions are logged with clear messaging

### Task 6: Implement gromit debug command (debug2)

Create `cmd/gromit/debug2.go` - the entry point for diagnosing failed spec executions. It finds preserved worktree branches, reads the event log and git history, diagnoses root cause, applies code fixes, and either adds a LEARNINGS entry (for patterns) or presents a recommendation (for systemic changes).

Files: `cmd/gromit/debug2.go`, `internal/v2/debug/diagnose.go`, `internal/v2/debug/fix.go`, `internal/v2/debug/learn.go`

Acceptance Criteria:
- Debug command finds preserved worktree branch by spec name
- Reads event log to identify failure point and stage where it occurred
- Traces failure to root cause (bad build output, flaky test, unclear description, incorrect decomposition)
- For learnable patterns, adds entry to LEARNINGS.md and applies autonomous fix
- For systemic changes, fixes immediate code problem and presents recommendation for human review
- Validates fixes by re-running relevant stage

### Task 7: Add integration tests for commit-per-stage flow

Write integration tests using a real git repo (no LLM invocation) to verify:
- Each stage creates a commit with proper message format
- Event log is cumulative across commits
- Retries preserve prior attempt commits in history
- Per-bead squash works correctly on successful completion

Files: `internal/v2/loop/integration_immutable_test.go`

Acceptance Criteria:
- Test verifies stage sequence produces correctly formatted commits
- Test confirms event log is cumulative and includes all prior events
- Test validates retry flow preserves prior attempt in git history
- Test confirms per-bead squash combines stage commits correctly

### Task 8: Add integration tests for branch lifecycle

Test that worktree branch is preserved on failure (Andon or generation cap) and deleted on success (PR merged).

Files: `internal/v2/loop/integration_branch_lifecycle_test.go`

Acceptance Criteria:
- On simulated failure, worktree branch exists with full history preserved
- On simulated success, worktree branch is cleaned up
- Event log remains accessible on preserved branch for debugging

### Task 9: Wire debug command into gromit CLI

Register the debug command as a top-level gromit CLI command alongside plan, decompose, etc.

Files: `cmd/gromit/main.go`

Acceptance Criteria:
- `gromit debug <spec-name>` is a valid command
- Help text explains the three debug jobs (diagnose, fix, learn)
- Debug command is discoverable via `gromit --help`

## Implementation Notes

- No new packages required; extend existing `internal/v2/loop/`, `internal/v2/event/`, `internal/v2/stage/present/`, and `internal/v2/adapter/git/`
- Commits are cheap; per-stage granularity captures exact state for debugging without significant cost
- Event log subscriber wiring follows same pattern as existing CLI and API subscribers
- Debug command architecture: diagnose via events + git history, fix by checkout + re-run stage, learn by pattern detection and LEARNINGS.md update
- Branch lifecycle is controlled by spec loop, not stages (stages remain stateless)

## Dependencies

- Depends on v2 run loop infrastructure (`internal/v2/loop/` and `internal/v2/stage/`)
- Depends on event system (`internal/v2/event/`)
- Reuses existing git adapter for commit operations
