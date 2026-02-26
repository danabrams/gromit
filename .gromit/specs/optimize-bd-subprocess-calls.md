---
id: optimize-bd-subprocess-calls
source_ideas: []
created: 2026-02-12
epic: codebase-health
---

# Optimize bd Subprocess Calls

## Specification

The runner's main loop makes redundant and inefficient bd subprocess calls during each iteration. Three specific inefficiencies exist:

### 1. Redundant GetParent() calls

`GetParent(b)` is called up to 3 times per iteration for the same bead — in `runPrecheck()` (process.go:1617), `checkScope()` (runner.go:1683), and `setupBeadContext()` (process.go:82). Each call spawns `bd show <parent-id> --json`. The parent bead does not change within an iteration, so all but the first call are wasted subprocess forks.

The fix: fetch the parent once in the main loop immediately after `getNextBead()` returns a bead with a non-empty `Parent` field. Pass the fetched parent to `runPrecheck`, `checkScope`, and `setupBeadContext` instead of having each fetch it independently.

### 2. Unbounded HasOpenChildren() scan

`HasOpenChildren(parentID)` calls `c.List()` which runs `bd list --json --status open --sort priority --limit 0`, fetching **every open bead** regardless of parent. It then iterates through all beads client-side checking `b.Parent == parentID`. This is O(n) in total open beads when it could be O(1).

The fix: replace the `List()` call with a targeted query using bd's `--parent` flag: `bd list --json --status open --parent <id> --limit 1`. This filters server-side and short-circuits after finding one match. The function only needs a boolean answer — does at least one open child exist — so `--limit 1` is sufficient.

### 3. Oversized Ready() batch

`Ready()` runs `bd ready --json --limit 10` but only returns the first non-epic bead. The batch of 10 exists because bd can't exclude epics with a single `--type` flag, so the code fetches extras to ensure at least one non-epic is in the result. However, 10 is unnecessarily large — consecutive epics at the top of the ready queue is unlikely in practice.

The fix: reduce the limit from 10 to 3. This still provides a comfortable margin for epic filtering while reducing serialization and parsing overhead.

## Acceptance Criteria

- GetParent is called at most once per iteration for any given bead, regardless of which phases (precheck, scope check, build) execute
- HasOpenChildren uses `bd list --parent <id> --limit 1` instead of fetching all open beads
- Ready uses `--limit 3` instead of `--limit 10`
- All existing tests pass without modification (behavior is unchanged, only performance characteristics differ)

## Decisions

1. **Fetch parent in the main loop, not in a lazy cache** — A lazy-loading cache on the Client struct would persist across iterations and risk serving stale data. Fetching once per iteration in the main loop is explicit, scoped correctly, and requires no cache invalidation logic.

2. **Pass parent as parameter rather than caching on Bead struct** — The Bead struct is a data object parsed from JSON. Adding cached resolved references to it would blur the line between data and state. Passing the parent as a function parameter keeps the data model clean.

3. **Use --limit 1 for HasOpenChildren, not --limit 0 with --parent** — Even with `--parent` filtering, `--limit 0` (unlimited) would serialize all matching children. Since we only need a boolean, `--limit 1` is optimal.

4. **Reduce Ready limit to 3, not 1** — Using `--limit 1` would break epic filtering (if the single result is an epic, Ready returns nil even though non-epic beads exist). 3 provides a safe margin while still being a meaningful reduction from 10.

## Research & Context

### Current State

**GetParent call sites** (all in `internal/runner/`):
- `process.go:82` — `setupBeadContext()`, called via `processBead()` for every iteration
- `runner.go:1402` — `runDecompose()`, called when a bead needs decomposition (separate code path)
- `runner.go:1617` — `runPrecheck()`, called at the top of each iteration
- `runner.go:1683` — `checkScope()`, called when scope gate is enabled

In the normal iteration flow: `runPrecheck` → `checkScope` → `setupBeadContext` — all three call GetParent on the same bead.

**HasOpenChildren** (`bead/bead.go:689`):
- Uses `c.List()` which runs `bd list --json --status open --sort priority --limit 0`
- Called at `runner.go:602` after bead success, guarded by `b.Parent != ""` and thorough review config
- bd's `list` command supports `--parent <id>` flag (confirmed via `bd list --help`)

**Ready** (`bead/bead.go:188`):
- Runs `bd ready --json --limit 10`
- Returns first non-epic via `parseBeadOutputExcluding(out, "epic")`
- Called from `getNextBead()` at `runner.go:2149` when no label filters are active

### Affected Files

- `internal/bead/bead.go` — HasOpenChildren implementation, Ready limit
- `internal/runner/runner.go` — Main loop (parent fetching), runPrecheck/checkScope/runDecompose signatures
- `internal/runner/process.go` — setupBeadContext signature
- Test files for the above packages
