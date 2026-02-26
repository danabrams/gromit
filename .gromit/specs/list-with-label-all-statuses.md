---
id: list-with-label-all-statuses
source_ideas: [idea-1770805560552]
created: 2026-02-11
epic: codebase-health
---

# Fix ListWithLabel to Return All Statuses and Unlimited Results

## Specification

`ListWithLabel` in `internal/bead/bead.go` currently runs `bd list --json --label <label>` without `--all` or `--limit 0` flags. Since `bd list` defaults to showing only open issues with a 50-result cap, this causes two problems:

1. **Missing closed beads** — `getBeadCounts` in `cmd/gromit/epic.go` counts open vs closed beads for progress display, but the closed count is always 0 because closed beads are never returned.

2. **Truncated results** — If a spec has more than 50 beads, results are silently capped at 50, giving incorrect counts and potentially missing commits in review base detection.

The fix adds `--all` and `--limit 0` to the `ListWithLabel` command invocation, changing it from `bd list --json --label <label>` to `bd list --json --label <label> --all --limit 0`.

All four non-test call sites benefit from this change:
- `epic.go:282` — bead count progress reporting (the primary motivator)
- `review.go:161` — finding earliest commit for spec review base
- `review.go:215` — finding earliest commit for epic review base
- `main.go:298` — building bead ID filter sets

## Acceptance Criteria

- `ListWithLabel` passes `--all` and `--limit 0` to the `bd list` command so that both open and closed beads are returned without a result cap
- `getBeadCounts` correctly reports non-zero closed counts when closed beads exist with the queried label
- Existing unit tests for `ListWithLabel` are updated to reflect the new command arguments

## Decisions

1. **Modify ListWithLabel rather than adding a new method** — Every caller of `ListWithLabel` needs all statuses. No caller intentionally wants only open beads. Adding a separate `ListAllWithLabel` would create unnecessary API surface with no consumer for the open-only variant.

2. **Add --limit 0 alongside --all** — The default 50-result cap is a related correctness issue. Fixing both together ensures `ListWithLabel` returns complete, accurate results. This matches the pattern used by `List()` which already passes `--limit 0`.

## Research & Context

### Current State

**`ListWithLabel`** (`internal/bead/bead.go`, line ~616):
```go
out, err := c.run("list", "--json", "--label", label)
```
Missing `--all` and `--limit 0`.

**`List()`** (`internal/bead/bead.go`, line ~519) — correctly uses explicit flags:
```go
out, err := c.run("list", "--json", "--status", "open", "--sort", "priority", "--limit", "0")
```

**`ListAll()`** (`internal/bead/bead.go`, lines ~547-601) — does separate calls for open and closed.

**`bd list` defaults** (from `bd list --help`):
- `--all`: "Show all issues including closed (overrides default filter)" — confirms default is open-only
- `--limit`: "Limit results (default 50, use 0 for unlimited)"

### Scope

The change is limited to a single line in `ListWithLabel` plus corresponding test updates. No interface changes, no new methods, no behavioral changes for callers — they simply receive complete data where before they received partial data.
