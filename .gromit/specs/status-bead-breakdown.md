---
id: status-bead-breakdown
source_ideas: []
created: 2026-02-11
epic: developer-experience
---

# Status Bead Breakdown

## Specification

The Beads line in the `gromit status` Pipeline section currently shows only the count of ready beads. Replace it with a full breakdown showing counts for every bd status: ready, blocked, in-progress, deferred, and closed. When a recent run's start time is available from status.json, also show how many beads were closed during that run.

### Output Format

The Beads line becomes a comma-separated list of non-zero counts:

```
Beads:    14 ready, 5 blocked, 543 closed (23 this run)
```

When there is no recent run info (no status.json), omit the parenthetical:

```
Beads:    14 ready, 5 blocked, 543 closed
```

Statuses with zero count are omitted from the line. The order is always: ready, in-progress, blocked, deferred, closed.

If all counts are zero:

```
Beads:    none
```

The ready bead ID list (up to 3 IDs shown below the Beads line) is preserved as-is.

### How Counts Are Gathered

- **ready**: `bd ready --json --limit 0` (existing `CountReady` or `ListReadyIDs`)
- **in-progress**: `bd list --json --status in_progress --limit 0`
- **blocked**: total open minus ready (open beads whose dependencies aren't met show as status "open" in bd but aren't returned by `bd ready`)
- **deferred**: `bd list --json --status deferred --limit 0`
- **closed**: `bd list --json --status closed --limit 0`
- **closed this run**: `bd list --json --status closed --closed-after <started_at> --limit 0` where `started_at` comes from status.json

### Integration Points

The `PipelineStatus` struct in `internal/pipeline/status.go` gains new count fields. The `formatPipeline` function in `internal/runner/format.go` formats the expanded Beads line. The `Runner.Status()` method in `internal/runner/runner.go` passes the run start time (from status.json) into `ReadStatus` so it can query "closed this run."

## Acceptance Criteria

- `gromit status` Beads line shows counts for all non-zero bd statuses (ready, in-progress, blocked, deferred, closed) in a single comma-separated line
- When status.json exists with a `started_at` time, the closed count includes a "(X this run)" parenthetical showing beads closed since that time
- When status.json does not exist, the closed count has no parenthetical
- Statuses with zero beads are omitted from the line
- The ready bead ID list (up to 3 items) still appears below the Beads line
- When all bead counts are zero, the line reads "Beads:    none"

## Decisions

1. **Use `--closed-after` for "this run" count** Rather than tracking closed bead IDs in status.json, query bd with `--closed-after <started_at>`. This keeps bd as the single source of truth for bead state and requires no runner changes. If someone closes beads manually during a run window, they're counted — which is arguably correct.

2. **Blocked = open minus ready** bd's "blocked" status field is for explicitly blocked beads. But beads that are status "open" with unmet dependencies also aren't ready. Using `total open - ready` captures both cases and matches what users mean by "blocked."

3. **Omit zero-count statuses** Showing "0 in-progress, 0 deferred" is noise. Only display statuses that have beads in them. This keeps the line compact for the common case (ready + blocked + closed).

4. **Fixed display order** Always: ready, in-progress, blocked, deferred, closed. This follows the lifecycle progression and keeps output predictable regardless of which statuses are populated.

## Research & Context

### Current State

- `internal/pipeline/status.go` — `PipelineStatus` struct has `ReadyBeadCount int` and `ReadyBeads []string`. `ReadStatus()` calls `listReadyBeads()` which uses `client.ListReadyIDs()`. New count fields will be added here.
- `internal/runner/format.go` — `formatPipeline()` renders `Beads: X ready` on a single line, followed by `formatItems()` for the ready bead IDs. This function will format the expanded line.
- `internal/runner/runner.go:1097-1151` — `Runner.Status()` reads status.json, then calls `pipeline.ReadStatus()`. The run start time from status.json needs to be passed through to `ReadStatus` for the "closed this run" query.
- `internal/bead/bead.go` — `Client.List()` queries `bd list --json --status open`. New methods or inline queries needed for in-progress, blocked, deferred, and closed counts.
- bd CLI supports `--status` filter (open, in_progress, blocked, deferred, closed) and `--closed-after` date filter.
