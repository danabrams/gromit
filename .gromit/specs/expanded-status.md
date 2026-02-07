---
id: expanded-status
source_ideas: []
created: 2026-02-07
---

# Expanded Status Display

## Specification

`gromit status` becomes the go-to command for understanding where things stand. It shows a rich, human-readable summary organized into four sections: pipeline progress, run state, health, and a recommended next action.

### Pipeline Section

Shows the state of work flowing through the Gromit pipeline (capture → refine → plan → decompose → run). Each stage shows a count and up to 3 item names. If there are more than 3, show "and X more".

```
Pipeline:
  Backlog:  3 unrefined ideas
    - Add rate limiting to API
    - Support webhook notifications
    - Improve error messages
  Specs:    1 unplanned
    - user-profiles
  Plans:    1 undecomposed
    - status-json-staleness
  Beads:    4 ready
```

Stages with zero items show the count but no item list (e.g. `Specs:    0 unplanned`).

### Run Section

When `gromit run` is active, shows current iteration progress, time elapsed, and any limits that were set:

```
Run: iteration 12/50, 18m of 30m elapsed
  Current:  ralph-runner-ja5m — "Create PROMPT_refactor.md template"
  Model:    sonnet
```

When no limits are set, omit the denominators:

```
Run: iteration 68, 3h 30m elapsed
```

When not running, shows that clearly along with info about the last run:

```
Run: not running
  Last run: 2h ago, 12 iterations completed
```

If there has never been a run (no status.json exists), just show:

```
Run: not running
```

### Health Section

Shows maintenance state from `state.json`:

```
Health:
  Last retro:  2h ago
  Last review: 5 iterations ago
```

If retro or review has never happened, show "never" instead of a time.

### Recommendation

Shows the recommended next action from the pipeline analysis — the highest-priority thing that needs attention:

```
Next action: Refine backlog ideas (gromit refine)
```

### Data Changes

**status.json**: No longer deleted on clean exit. Instead, a final entry is written with `running: false` and the completed iteration count. This allows `gromit status` to show last-run info when idle.

**Status struct**: Add `MaxIterations` and `TimeBudgetMinutes` fields so that run limits are persisted and can be displayed.

## Acceptance Criteria

- `gromit status` shows pipeline counts for backlog ideas, unplanned specs, undecomposed plans, and ready beads
- Each pipeline category shows up to 3 item names, with "and X more" when there are more than 3
- When `gromit run` is active, status shows current iteration, elapsed time, current bead, and model — with limit denominators when limits were set
- When `gromit run` is not active, status shows "not running" and last run summary (time ago, iterations completed)
- Status shows last retro and last review timing from state.json
- Status shows a recommended next action based on pipeline state
- `status.json` persists after clean exit with `running: false` instead of being deleted
- `status.json` includes `max_iterations` and `time_budget_minutes` fields when those limits are set on `gromit run`

## Decisions

1. **Don't delete status.json on exit** — Write a final status with `running: false` instead. This preserves last-run info for display and is also compatible with the planned staleness detection (status-json-staleness spec), which handles crash cases where the file is left behind with `running: true`.

2. **Add limit fields to Status struct** — `MaxIterations int` and `TimeBudgetMinutes int` get written alongside other run state so status can show progress against limits (e.g. "iteration 12/50, 18m of 30m elapsed").

3. **Use existing PipelineStatus** — The `internal/pipeline/status.go` package already reads backlog, specs, plans, and bead counts. Wire it into `Runner.Status()` rather than reimplementing.

4. **Show up to 3 items per category** — Keeps output compact for large backlogs while still giving useful context. Consistent across all pipeline stages.

5. **Human-only output** — No `--json` flag for now. This is for human consumption at the terminal.

## Research & Context

### Current State

- `Runner.Status()` in `internal/runner/runner.go:801-824` currently only shows the next ready bead (ID, title, priority, labels, model).
- `internal/runner/status.go` defines `Status` struct and `StatusWriter`. The writer currently deletes `status.json` on clean exit via `Delete()`.
- `internal/pipeline/status.go` has a fully built `PipelineStatus` struct with `ReadStatus()` that already gathers unrefined ideas, unplanned specs, undecomposed plans, ready bead count, and a recommendation string. This is not currently wired into `gromit status`.
- `internal/state/state.go` tracks `LastRetro`, `LastReviewCommit`, `LastReviewIteration`, and `IterationsSinceReview`.
- The `status-json-staleness` spec/plan exists and adds PID-based stale detection to status.json. That work is complementary — it handles the "process crashed and left `running: true`" case, while this spec handles the normal display.
