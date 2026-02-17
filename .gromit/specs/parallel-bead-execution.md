---
id: parallel-bead-execution
source_ideas: []
created: 2026-02-17
---

# Parallel Bead Execution

## Specification

Replace the sequential run loop with a worker pool that executes multiple independent beads concurrently in separate git worktrees, merging results back to main via a serial merge coordinator with LLM-assisted conflict resolution.

### Problem

Gromit processes beads one at a time. Each successful bead takes 9-20 minutes (median 9.2 min), with ~80% of that time waiting for Claude to respond. With 50+ ready beads, draining the backlog takes 24+ hours of wall-clock time. Analysis of ready beads shows they are mostly independent -- touching different packages with minimal file overlap. The sequential constraint wastes throughput.

### Goals

1. Process N beads concurrently using persistent git worktrees (configurable, default 3).
2. Maintain linear git history on main via serial merge-back with rebase.
3. Handle merge conflicts via haiku-tier LLM resolution before discarding work.
4. Catch cross-bead interactions via periodic batch validation after every N merges.
5. Full backward compatibility: `workers: 1` produces identical behavior to today.

### Non-Goals

- Changing the per-bead pipeline (`processBead` runs unchanged in a worktree dir).
- Custom dependency scheduling beyond what `bd ready` already provides.
- Speculative execution of blocked beads.
- Parallel merge (merges remain serial for linear history).

### Design

#### Architecture

Three concurrent components:

1. **Dispatcher** -- pulls unblocked beads from `bd ready`, assigns to idle workers, refreshes pool after each merge.
2. **Worker pool** (N goroutines) -- each worker has a persistent worktree, runs the existing `processBead` pipeline, sends successful results to the merge queue.
3. **Merge coordinator** (1 goroutine) -- consumes from merge queue, rebases onto main, resolves conflicts via LLM, runs batch validation, closes beads via bd.

#### Worker lifecycle

Workers are long-lived goroutines with persistent worktrees (`<project>-gromit-worker-<N>`).

Per-bead cycle:
1. Reset worktree to latest main (`git reset --hard main`).
2. Create bead branch (`gromit/bead-<id>-<worker>`).
3. Run `processBead(bead, worktreeDir)` -- the existing pipeline, unchanged.
4. On success: send result to merge queue, wait for next assignment.
5. On failure: report failure to dispatcher (counts toward stuck-bead threshold), return to idle, receive next assignment immediately.

Persistent worktrees avoid per-bead filesystem churn; reset between beads is a fast `git reset`.

#### Dispatcher

Single goroutine managing the work queue:
1. Calls `bd ready` to get the pool of unblocked beads.
2. Assigns one bead per idle worker (no bead is assigned to multiple workers).
3. After each merge-back, refreshes the ready pool -- newly-unblocked beads (whose blockers were just closed) become available.
4. Respects all existing stop conditions: max iterations (total across workers), time budget, signal (SIGINT/SIGTERM), L3 stop line.
5. On stop: cancels worker contexts, waits for in-flight beads to finish or timeout, drains merge queue for any completed work before shutting down.

#### Merge coordinator

Single goroutine consuming from a buffered channel. Serial processing of completed beads:

1. Rebase bead branch onto current main (conflict resolution in next section).
2. Merge to main (fast-forward after clean rebase).
3. `bd close <bead-id>` (serialized -- no bd contention).
4. Increment merge counter. Every N merges: run full validation suite against main. On failure, revert last N merges via `git reset --hard <pre-batch-commit>` and re-queue those beads for fresh execution.
5. Signal dispatcher: merge complete, slot available. Dispatcher refreshes the `bd ready` pool -- previously-blocked beads may now be unblocked since their blockers were just closed.

#### Conflict resolution

Full merge-back flow per completed bead:

1. Bead completes in worker worktree, passes validation locally.
2. Rebase bead branch onto current main.
3. If clean rebase: fast-forward merge to main.
4. If conflicts: capture conflict markers + file list, invoke Claude at haiku tier with bead description + diff summary + conflict markers.
5. If LLM resolution succeeds: run validation in worktree to verify the resolution. If validation passes, merge to main. If validation fails, abort and re-queue.
6. If LLM resolution fails: abort rebase, re-queue bead for fresh execution against updated main.

Haiku handles mechanical conflicts (different hunks in config.go, different test functions) in 30-60 seconds. Fresh re-execution costs 10-20 minutes. Always try resolution first.

#### Worktree lifecycle and cleanup

Worker worktrees are created at `Run()` start and cleaned up at `Run()` end (in `finishRun`).

- Normal shutdown: all worker worktrees removed via `git worktree remove`.
- Crash recovery: on next `Run()` start, detect and clean up orphaned worker worktrees matching the `<project>-gromit-worker-*` pattern before creating fresh ones.
- Reuse the existing `internal/worktree/Manager` infrastructure. Extend it with methods for persistent worker worktree creation, reset-to-main, and batch cleanup. Do not build a separate worktree management layer.

#### State safety

- **bd operations**: serialized in merge coordinator (never from workers)
- **state.json**: add flock (backport from interactive-state.json)
- **Metrics**: per-worker JSONL files, merged at run end
- **LEARNINGS.md**: snapshot at bead start, append-with-flock after merge
- **validationFailures**: per-worker, no sharing needed

### Prerequisites

These must be completed before the parallel runner:

1. Add flock to state.json read/write operations
2. Make logger support per-worker output files with merge-at-end
3. Verify bd CLI safety under concurrent reads (Dolt backend spike)
4. Add provider rate-limit semaphore for fair API access

### Configuration

```yaml
parallel:
  workers: 3
  merge_validation_interval: 3
  conflict_resolution: true
  worktree_prefix: "worker"
```

### Acceptance Criteria

- With `workers: 3`, three beads execute concurrently in separate worktrees with independent Claude invocations.
- Merge coordinator processes completed beads serially, maintaining linear git history.
- When rebase produces conflicts, LLM resolution is attempted before re-queuing the bead.
- After every `merge_validation_interval` merges, full validation runs against main. On failure, affected merges are reverted and beads re-queued.
- `bd close` and `bd sync` operations are never called concurrently.
- state.json uses file locking for all reads and writes.
- Each worker writes metrics to its own JSONL file; files are merged at run completion.
- With `workers: 1`, behavior is identical to the current sequential loop.
- Worker worktrees are created at run start and cleaned up at run end.
- Orphaned worker worktrees from a previous crashed run are detected and cleaned up on next run start.
- Stop conditions (max iterations, time budget, signals) cancel worker contexts, drain in-flight work, and shut down without losing completed merges.
- Failed beads in workers count toward the existing stuck-bead threshold; workers immediately receive new assignments after failures.
- Dispatcher refreshes the `bd ready` pool after each merge, making newly-unblocked beads available.
- LLM conflict resolution runs validation after resolution and before merging -- failed validation aborts the merge and re-queues the bead.
- The parallel runner extends `internal/worktree/Manager`, not a separate worktree implementation.

## Decisions

1. Worker pool over batch rounds -- avoids idle-worker waste.
2. Persistent worktrees over per-bead -- avoids filesystem churn.
3. Serial merge coordinator -- linear history, serialized state operations.
4. LLM conflict resolution before re-queue -- 30s haiku saves 10-20 min re-execution.
5. Batch validation as safety net -- per-bead validation is primary; batch catches cross-bead interactions.
6. `bd ready` for scheduling -- no custom dependency analysis needed.

## Research & Context

### Data

Analysis of 228 iterations (139 successful) from iteration_metrics.jsonl:
- Median successful bead: 9.2 min (mean 10.4 min)
- By model: haiku 38s, sonnet 9.8 min, opus 16.4 min, codex 17-19 min
- Current throughput: ~5.8 beads/hour
- Expected with 3 workers: ~15-17 beads/hour

Analysis of 15 ready beads:
- 10 touch completely separate files/packages
- 3 share config.go but modify different struct fields (mechanical merge)
- Test reorganization beads touch different source/destination files
- No semantic conflicts identified

### Risk notes

- bd CLI concurrent read safety depends on Dolt backend guarantees (spike needed)
- Provider rate limits may cause wasted retries under high concurrency (semaphore mitigates)
- Batch validation failure triggers multi-bead revert -- could waste significant work if cross-bead conflicts are frequent (mitigated by bead independence analysis)

### Related specs

- `parallel-interactive-command-worktrees` -- worktrees for interactive commands (different scope; this spec covers the run loop itself)
- `phase-isolated-methodology-contexts` -- phase timeout isolation (compatible; per-worker contexts are independent)

### Design document

Full design with rationale: `docs/plans/2026-02-17-parallel-bead-execution-design.md`
