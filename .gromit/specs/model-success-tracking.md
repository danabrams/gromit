---
id: model-success-tracking
source_ideas: []
created: 2026-02-11
---

# Model Success Tracking

## Specification

Track per-model success and failure rates at two levels — project-local and global (cross-project) — and surface the data in the CLI so users can make informed decisions about model selection configuration.

### Project-Level Tracking

Computed on demand from existing JSONL log files in `.gromit/logs/`. No new data collection needed — `IterationLog` already records `model`, `success`, `escalated`, `escalated_to`, and `cost_usd` per iteration. The system aggregates these into per-model success rates, cost-per-completed-bead, and escalation frequency.

### Global Tracking

A single file at `~/.gromit/stats.json` stores aggregate totals across all projects. After each run completes (at least one bead processed), gromit appends that run's per-model outcomes to the global totals. The file stores only running totals (iterations, successes, failures, total cost per model) — not per-project breakdowns. Project-level detail is available from each project's own logs.

### Surfacing

**`gromit status`** — adds a brief model performance section showing per-model success rates for the current project. Example:

```
Model Performance (this project):
  opus    91% success  (10/11)  avg $2.04/iter
  sonnet  38% success  (3/8)   avg $0.46/iter
  haiku   75% success  (6/8)   avg $0.12/iter
```

**`gromit stats`** — a new dedicated command showing detailed model performance:

- Per-model success rate, failure rate, and iteration count (project + global)
- Cost per completed bead (factoring in retries and escalation)
- Escalation frequency (how often each model escalates, and to what)
- Comparison of project rates vs. global rates to highlight project-specific anomalies
- Cost-effectiveness ranking (effective cost per completed bead, accounting for retry/escalation chains)

### Global Stats File Format

`~/.gromit/stats.json` structure:

```json
{
  "version": 1,
  "updated": "2026-02-11T14:30:00Z",
  "models": {
    "opus": {
      "iterations": 42,
      "successes": 38,
      "failures": 4,
      "total_cost_usd": 84.50,
      "escalations_from": 0,
      "escalations_to": 12
    },
    "sonnet": {
      "iterations": 65,
      "successes": 24,
      "failures": 41,
      "total_cost_usd": 29.90,
      "escalations_from": 30,
      "escalations_to": 0
    },
    "haiku": {
      "iterations": 30,
      "successes": 22,
      "failures": 8,
      "total_cost_usd": 3.60,
      "escalations_from": 5,
      "escalations_to": 0
    }
  }
}
```

### No Automatic Routing Changes

This feature is advisory only. The system tracks and displays data but does not alter model selection behavior. Users read the stats and adjust their `gromit.yaml` model configuration manually. Automatic routing based on historical data is a future enhancement.

## Acceptance Criteria

- `gromit stats` displays per-model success rate, iteration count, cost-per-completed-bead, and escalation frequency for both project and global scopes
- `gromit status` includes a brief model performance summary (success rate and avg cost per model) from project logs
- After each run that processes at least one bead, global stats in `~/.gromit/stats.json` are updated with that run's per-model outcomes
- Global stats file uses atomic write (write-then-rename) to avoid corruption from concurrent runs
- When `~/.gromit/stats.json` does not exist, `gromit stats` shows project-only data without error and the first completed run creates the file

## Decisions

1. **Project stats are computed on demand, not cached.** The JSONL log files are small and fast to read. Caching would add complexity and staleness risk for negligible performance gain.

2. **Global stats store aggregate totals only.** Per-project breakdowns are not stored globally — they're available from each project's logs. This keeps the global file simple and avoids privacy concerns about project paths leaking into a shared file.

3. **Advisory only, no automatic routing.** Building the data foundation first lets us validate that the metrics are meaningful before using them to change behavior. Automatic routing is a separate future spec.

4. **Cost-per-completed-bead factors in the full chain.** When a bead requires 2 sonnet attempts then escalates to opus, all three iterations' costs are attributed to that bead's completion cost. This gives the true effective cost per model strategy, not just per-iteration cost.

5. **Atomic file writes for global stats.** Multiple gromit instances across different projects could finish runs concurrently. Write to a temp file then rename to avoid partial writes.

## Research & Context

### Current State

- `internal/logger/logger.go` — `IterationLog` already captures `Model`, `Success`, `Escalated`, `EscalatedTo`, `CostUSD` per iteration
- `internal/logger/efficiency.go` — `ModelEfficiency` aggregates cost/duration/tokens per model but does NOT track success rates
- `internal/config/config.go:351-374` — `SelectModel()` maps priority → model with label overrides
- `internal/runner/process.go:102-136` — scope check auto-escalates to opus on `complexity == "high"`
- `cmd/gromit/` — one file per subcommand; `gromit stats` would be a new file here

### Key Extension Points

- `ModelEfficiency` struct in `efficiency.go` is the natural place to add success/failure counts
- `ReadEfficiencyReport()` already reads all JSONL logs — extending it to track success rates is straightforward
- The runner's `Close()` or post-run hook is where global stats updates should happen
- `cmd/gromit/status.go` already exists and can be extended with the model performance section
