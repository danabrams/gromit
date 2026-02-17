# Parallel Bead Execution Design

## Problem

Gromit processes beads sequentially. Each successful bead takes 9-20 minutes (median 9.2 min for sonnet, 16.4 min for opus). With 50+ ready beads, the backlog takes 24+ hours of wall-clock time to drain. ~80% of each bead's duration is waiting for Claude to respond -- the sequential pipeline cannot be meaningfully optimized beyond that.

Analysis of 15 ready beads shows they are mostly independent: touching different packages, with only mechanical overlap in config/test files. Parallelism is safe and the throughput multiplier is proportional to worker count.

## Goals

1. Process multiple independent beads concurrently using git worktrees.
2. Maintain linear git history on main via serial merge-back.
3. Handle merge conflicts via LLM resolution before discarding work.
4. Preserve backward compatibility: `workers: 1` is identical to today's sequential loop.

## Non-Goals

- Changing the per-bead pipeline (processBead stays the same).
- Dependency-aware scheduling beyond what `bd ready` already provides.
- Speculative execution of blocked beads.

## Architecture

Worker pool + merge coordinator pattern:

```
                +----------------+
                |   Dispatcher   |  <- pulls from bd ready, assigns to idle workers
                +-------+--------+
           +------------+------------+
           v            v            v
     +----------+ +----------+ +----------+
     | Worker 1 | | Worker 2 | | Worker 3 |   <- each has persistent worktree
     | (wt-001) | | (wt-002) | | (wt-003) |
     +-----+----+ +-----+----+ +-----+----+
           |             |             |
           v             v             v
     +-------------------------------------+
     |         Merge Queue (chan)           |
     +------------------+------------------+
                        v
              +------------------+
              | Merge Coordinator|  <- serial: rebase, resolve, validate
              +------------------+
                        |
                        v
                      main
```

### Workers

Each worker is a long-lived goroutine with a dedicated persistent worktree.

**Startup:** Runner creates N worktrees at run start (`<project>-gromit-worker-<N>`), each branched from current main HEAD.

**Per-bead cycle:**
1. Reset worktree to latest main (`git reset --hard main`)
2. Create bead branch (`gromit/bead-<id>-<worker>`)
3. Run `processBead(bead, worktreeDir)` -- the existing pipeline, unchanged
4. On success: send result to merge queue
5. On failure: report failure, return to idle
6. Wait for next assignment

**Why persistent worktrees:** Creating/deleting a worktree per bead is expensive filesystem I/O (full checkout). Persistent worktrees are reset between beads -- fast and avoids churn.

### Dispatcher

Single goroutine that:
1. Calls `bd ready` to get unblocked beads
2. Assigns beads to idle workers (one bead per worker)
3. Refreshes the ready pool after each merge (newly-unblocked beads become available)
4. Respects stop conditions: max iterations, time budget, signals

The dispatcher does not do dependency analysis -- `bd ready` already filters for unblocked beads.

### Merge Coordinator

Single goroutine consuming from a buffered channel. The only goroutine that touches main.

For each completed bead:
1. Rebase bead branch onto current main (in the worker's worktree)
2. If clean rebase: fast-forward merge to main
3. If conflicts: LLM conflict resolution (see below)
4. `bd close <bead-id>` (serialized -- no bd contention)
5. Increment merge counter
6. Every N merges (configurable): run batch validation on main
   - Pass: continue
   - Fail: revert last N merges, re-queue those beads for fresh execution
7. Signal dispatcher: slot available, refresh ready pool

**Why serial merges:** Maintains linear history. Rebase conflicts are deterministic. bd operations are serialized. Simple to debug.

### Conflict Resolution

When rebase onto main produces conflicts:

1. Capture conflict markers and file list
2. Build LLM prompt with bead description, diff summary, and conflict markers
3. Invoke Claude at haiku tier (conflicts between well-scoped beads are mechanical)
4. If LLM resolves: `git add` + `git rebase --continue`, then validate before merging
5. If LLM fails: abort rebase, re-queue bead for fresh execution against updated main

**Cost:** LLM resolution takes 30-60 seconds (haiku). Fresh re-execution takes 10-20 minutes. Always try resolution first.

### Batch Validation

Every N merges (default N=3), run the full validation suite against merged main.

- Catches cross-bead interactions that pass in isolation but fail together
- On failure: revert last N merges (`git reset --hard <pre-batch-commit>`), re-queue those beads
- This is the safety net, not the primary correctness mechanism

## State Management

### bd operations
All `bd close`, `bd sync` calls happen in the merge coordinator goroutine -- never from workers. Workers only read bead data from their assignment. Serialization eliminates bd concurrency concerns.

### state.json
Backport the `withFileLock()` pattern from interactive-state.json. Workers read state (provider cooldowns). Merge coordinator writes state (iteration counters). File locking makes this safe.

### Metrics/logging
Each worker writes to its own JSONL file (`iteration_metrics_worker_<N>.jsonl`). Merge coordinator or run finalization merges worker logs into the canonical `iteration_metrics.jsonl`. No write contention.

### LEARNINGS.md
Workers read a snapshot of LEARNINGS.md taken at bead start. New learnings extracted after merge are appended with flock. Workers in the same batch don't see each other's learnings -- acceptable because they're working on independent tasks. Next batch sees all accumulated learnings.

### validationFailures
Per-worker. Each worker maintains its own failure history for its current bead. No sharing needed.

## Configuration

```yaml
parallel:
  workers: 3                    # concurrent bead workers (1 = sequential)
  merge_validation_interval: 3  # revalidate main after every N merges
  conflict_resolution: true     # LLM conflict resolution before re-queuing
  worktree_prefix: "worker"     # naming: <project>-gromit-worker-<N>
```

`workers: 1` is a full backward-compatibility switch -- behavior is identical to today.

## What Changes vs. What Doesn't

**Changes:**
- `runner.Run()` orchestration: dispatcher + workers + merge coordinator replaces sequential loop
- Worktree lifecycle: persistent worker worktrees created/cleaned per run
- Logger: per-worker files, merged at run end
- state.json: add flock
- Merge pipeline: new code (rebase + LLM resolution + batch validation)

**Doesn't change:**
- `processBead()` -- identical, just runs in a worktree dir
- Escalation logic, model selection -- per-bead, independent
- bd integration -- writes serialized through merge coordinator
- Config loading, prompt rendering, validation commands
- `gromit status`, `gromit add`, other commands

## Prerequisites

Before the parallel runner itself:
1. Add flock to state.json (backport from interactive-state)
2. Make logger concurrency-safe (per-worker files or mutex)
3. Spike bd CLI concurrent read safety (Dolt backend)
4. Add provider semaphore for rate limit fairness

## Expected Impact

- **3x throughput** at `workers: 3`: ~15-17 beads/hour (vs 5.8 today)
- **24hr backlog drops to ~8hr**
- Complementary with per-bead optimizations (first-pass success, model routing)

## Decisions

1. **Worker pool over batch rounds.** Workers don't wait for the slowest bead in a batch.
2. **Persistent worktrees over per-bead.** Avoids filesystem churn, faster reset.
3. **Serial merge coordinator.** Linear history, serialized bd/state operations, simple debugging.
4. **LLM conflict resolution before re-queue.** 30s haiku attempt saves 10-20 min re-execution.
5. **Batch validation as safety net.** Per-bead validation catches most issues; batch catches cross-bead interactions.
6. **bd ready for scheduling.** No custom dependency analysis -- the bead system already handles it.

## Data Supporting This Design

Analysis of 228 iterations (139 successful):
- Median successful bead: 9.2 min, mean 10.4 min
- Sonnet median: 9.8 min, Opus median: 16.4 min
- ~80% of bead time is Claude invocation (not optimizable sequentially)
- 15 ready beads analyzed: mostly independent, minimal file overlap
- Config files are the primary merge friction point (mechanical, LLM-resolvable)
