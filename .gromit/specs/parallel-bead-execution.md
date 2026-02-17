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

Per-bead: reset worktree to main HEAD, create bead branch, run `processBead`, send result to merge queue or report failure.

Persistent worktrees avoid per-bead filesystem churn; reset between beads is a fast `git reset`.

#### Merge coordinator

Serial processing of completed beads:
1. Rebase bead branch onto current main
2. Fast-forward merge if clean; LLM conflict resolution if not
3. `bd close` (serialized -- no bd contention)
4. Every N merges: batch validation on main; revert + re-queue on failure

#### Conflict resolution

On rebase conflict: capture markers, invoke haiku with bead context + conflict markers, validate after resolution. Falls back to discarding and re-queuing the bead for fresh execution.

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
- Stop conditions (max iterations, time budget, signals) halt all workers gracefully.

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
