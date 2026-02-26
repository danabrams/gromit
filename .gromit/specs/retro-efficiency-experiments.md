---
id: retro-efficiency-experiments
source_ideas: []
created: 2026-02-07
epic: codebase-health
---

# Retro Efficiency Analysis & Experiments

## Specification

Extend the existing retro system to analyze execution efficiency (cost and time) alongside failure patterns, and introduce a structured experiment loop for continuous process improvement.

### Data Capture

Each iteration already logs duration and model to JSONL (`IterationLog`). Extend this to also capture:

- **Cost (USD)** — from the `total_cost_usd` field in Claude's `result` stream event, which is already parsed into `StreamEvent.TotalCost` but discarded after heartbeat logging
- **Input tokens** and **output tokens** — from Claude's `result` stream event (fields not yet parsed from the JSON)

These fields are added to `IterationLog` and populated by the runner after each Claude invocation completes. The stream logger's `StreamStats` already tracks the result event; it needs to extract and expose cost and token data so the runner can read it after the stream finishes.

### Efficiency Analysis in Retro

The retro prompt (`PROMPT_retro.md`) gains a new section after Run Statistics:

**Current Run Efficiency:**
- Per-iteration table: Bead ID | Model | Duration | Cost | Input Tokens | Output Tokens
- Aggregates by model: avg cost, avg duration, avg tokens per iteration for each model used
- Context window utilization flags: iterations where input tokens exceeded 80% of the model's context window

**Historical Comparison:**
- Same aggregates computed from all previous run logs
- Delta display: "Cost per bead: $0.42 this run vs $0.31 historical avg (+35%)"
- Trend direction for key metrics (improving, stable, degrading)

The retro's Task section gains a new item alongside the existing failure analysis:

6. **Efficiency Analysis**: Identify cost or time anomalies, suggest root causes. When anomalies are found, apply Five Whys analysis to trace surface symptoms (e.g., "this bead cost $3") to root causes (e.g., "the acceptance criteria were ambiguous, causing opus escalation"). Produce efficiency-related learnings.

### Experiment Loop

After completing the standard retro analysis (consolidations, promotions, archives, stuck beads, efficiency analysis), the retro generates **experiment recommendations** — concrete, testable changes to the process with clear measurement criteria.

**Experiment structure:**
- **Name**: Short descriptive label (e.g., "Use haiku for test-only beads")
- **Hypothesis**: What we expect to happen (e.g., "Beads that only modify test files can succeed with haiku, reducing cost by ~60% for those beads")
- **Change**: What to do differently (e.g., "Add label `complexity:low` to beads whose title contains 'test'")
- **Measurement**: How to evaluate success (e.g., "Compare success rate and cost of test-only beads before vs after")
- **Risk**: What could go wrong (e.g., "Test-only beads may fail more on haiku, increasing retries")

The retro should generate 2-4 experiment recommendations based on the efficiency data. During the interactive review session, the user selects **zero or one** experiment to try. Never more than one — running multiple experiments simultaneously makes it impossible to attribute changes.

**Experiment persistence:**

The selected experiment is stored in `.gromit/experiment.json`:

```json
{
  "name": "Use haiku for test-only beads",
  "hypothesis": "...",
  "change": "...",
  "measurement": "...",
  "risk": "...",
  "started_at": "2026-02-07T12:00:00Z",
  "baseline_metrics": {
    "avg_cost_per_bead": 0.42,
    "avg_duration_ms": 45000,
    "avg_input_tokens": 12000,
    "avg_output_tokens": 3000,
    "failure_rate": 0.08
  }
}
```

When no experiment is active, this file does not exist. Only one experiment can be active at a time.

### Experiment Evaluation at Next Retro

When the retro runs and finds an active `experiment.json`, it includes the experiment details and baseline metrics in the prompt. The retro:

1. Computes the same metrics for runs since the experiment started
2. Compares current metrics against the baseline
3. Presents analysis: did the measurement improve, degrade, or stay flat? Were there unexpected side effects?
4. Offers observations — "Cost dropped 20% but failure rate increased 5%, suggesting the model change works for simple beads but not complex ones"
5. Does **not** auto-conclude — the user decides during the interactive review whether to:
   - **Keep** the change (delete experiment.json, the change is now standard practice)
   - **Revert** the change (delete experiment.json, undo whatever was changed)
   - **Extend** the experiment for another cycle (keep experiment.json, gather more data)

### Interactive Review Enhancements

The `LaunchClaudeCode` prompt for the interactive review session is updated to include:

- The efficiency analysis and experiment recommendations
- If an active experiment exists: the evaluation results and the keep/revert/extend decision prompt
- Guidance to use Five Whys when drilling into efficiency anomalies
- The constraint that the user should pick at most one new experiment

## Acceptance Criteria

- `IterationLog` includes `cost_usd`, `input_tokens`, and `output_tokens` fields, populated from Claude stream events
- Retro prompt includes per-iteration efficiency table, per-model aggregates, and historical comparison with deltas
- Retro analysis produces efficiency learnings and 2-4 experiment recommendations with name, hypothesis, change, measurement, and risk
- Interactive review presents experiments and allows user to select 0 or 1; selected experiment is persisted to `.gromit/experiment.json`
- When an active experiment exists, the next retro evaluates it against baseline metrics and presents analysis with observations
- User can keep, revert, or extend an active experiment during review

## Decisions

1. **Single experiment file, not a history** — `.gromit/experiment.json` holds only the active experiment. There's no experiment history log. If we need history later, we can add it, but for now keeping it simple avoids overengineering. The retro analysis itself serves as the record of what was tried.

2. **Efficiency data in IterationLog, not a separate file** — Adding cost/token fields to the existing `IterationLog` struct keeps the data pipeline simple. The JSONL files already hold per-iteration data; adding three fields is cleaner than creating a parallel logging system.

3. **Five Whys in the prompt, not as a separate tool** — The Five Whys analysis is guidance in the retro prompt, not a separate command or interactive flow. The retro's Claude invocation applies it during analysis, and the interactive review session can continue the investigation with the user.

4. **Baseline metrics snapshot at experiment start** — The baseline is computed and frozen when the experiment is selected, not recomputed at evaluation time. This prevents the baseline from shifting as new non-experiment runs accumulate.

5. **Max one experiment at a time** — Running multiple experiments simultaneously makes attribution impossible. This constraint is enforced in the interactive review (the prompt instructs selection of 0 or 1), not in code.

## Research & Context

### Current State

- **Stream events** (`internal/logger/stream.go`): `StreamEvent` already parses `total_cost_usd` from result events (line 26). Token fields are present in Claude's JSON output but not yet in the struct.
- **Iteration logging** (`internal/logger/logger.go`): `IterationLog` (lines 13-26) has `DurationMs` and `Model` but no cost or token fields. `RunStats` (lines 133-146) aggregates only success/failure counts.
- **Retro system** (`internal/retro/retro.go`): `TemplateContext` (lines 29-34) passes `Rules`, `Learnings`, `RunStats`, and `BeadStats` to the template. Will need new fields for efficiency data and active experiment.
- **Retro prompt** (`.gromit/templates/PROMPT_retro.md`): Currently has Run Statistics (total/succeeded/failed/failure rate) and Stuck Beads sections. Efficiency section and experiment recommendations will be added after these.
- **State file** (`.gromit/state.json`): Tracks `last_retro`, `last_review_commit`, `iterations_since_review`. Experiment state is a separate file to keep concerns separated.
- **Stream stats** (`internal/logger/stream.go`): `StreamStats` tracks tool calls and files modified but not cost or tokens. The `HandleEvent` method sees the result event with cost data (line 267) but only logs it to the stream file.

### Data Flow for New Metrics

1. Claude emits `result` event with `total_cost_usd`, `input_tokens`, `output_tokens`
2. `StreamStats.HandleEvent` captures these values (new fields on `StreamStats`)
3. After `StreamRun` completes, runner reads cost/tokens from `StreamStats`
4. Runner writes them to `IterationLog` via `logger.LogIteration()`
5. `ReadAllLogs` / `ReadPerBeadStats` aggregate them for retro consumption
6. Retro template renders efficiency tables from aggregated data
