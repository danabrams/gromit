---
id: tdd-phase-metrics
source_ideas: []
created: 2026-02-19
depends_on:
  - tdd-fresh-context-per-cycle
---

# Per-Phase Metrics for TDD Cycles

## Specification

Gromit tracks granular metrics for each phase invocation within TDD cycles. This enables comparison of the fresh-context-per-cycle approach against the old single-invocation model and surfaces per-phase reliability and cost data for ongoing optimization.

### Metrics Collected

For each phase invocation (red, green, refactor, coverage-validation):

| Metric | Type | Description |
|--------|------|-------------|
| `phase` | string | Phase name: `red`, `green`, `refactor`, `coverage_validation` |
| `cycle_number` | int | Which TDD cycle (1-indexed) |
| `bead_id` | string | Parent bead identifier |
| `spec_id` | string | Spec being implemented |
| `model` | string | Model used for this invocation |
| `tier` | string | Tier selected: `low`, `medium`, `high` |
| `input_tokens` | int | Prompt tokens sent |
| `output_tokens` | int | Response tokens received |
| `duration_ms` | int | Wall clock time for the invocation |
| `success` | bool | Whether the phase succeeded |
| `escalated` | bool | Whether model was escalated for this phase |
| `escalated_from` | string | Original model before escalation (empty if not escalated) |
| `criteria_targeted` | int | Criterion number targeted (red/green phases) |
| `criteria_covered_count` | int | Total criteria checked off after this cycle |
| `criteria_total` | int | Total criteria in the spec |

### Aggregate Metrics Per Bead

After all cycles complete, the runner computes and logs:

| Metric | Description |
|--------|-------------|
| `total_cycles` | Number of red-green-refactor cycles executed |
| `total_invocations` | Total Claude invocations (cycles * 3 + coverage validations) |
| `total_input_tokens` | Sum of input tokens across all phases |
| `total_output_tokens` | Sum of output tokens across all phases |
| `total_cost_usd` | Total cost across all invocations |
| `total_duration_ms` | Wall clock time for all cycles |
| `coverage_rate` | Fraction of criteria covered (checked / total) |
| `phase_success_rates` | Per-phase success rate (red, green, refactor) |
| `avg_tokens_per_cycle` | Average input + output tokens per cycle |
| `escalation_count` | Number of phase escalations |

### Storage

Phase-level metrics are appended to the existing iteration metrics JSONL file (`.gromit/metrics/iteration_metrics.jsonl`) as records with `"type": "tdd_phase"`. Aggregate metrics are included in the bead's iteration result with `"type": "tdd_summary"`.

This follows the existing pattern: JSONL for granular append-only data, iteration results for per-bead summaries.

### Comparison Baseline

To compare against the old single-invocation approach, the runner logs:

- **Estimated single-invocation tokens**: For the first cycle, input tokens approximate what the old approach would use. For subsequent cycles, the delta shows the "context pollution tax" that the fresh-context approach avoids.
- **Discovery overhead**: In the old model, Claude spent tokens reading files. In the new model, file contents are in the prompt. The metric `handoff_content_tokens` tracks how many tokens the handoff content adds to each phase prompt.

### Dashboard / Reporting

Metrics are queryable via `gromit stats` with a new `--tdd` flag that summarizes:

- Average cycles per bead
- Per-phase success rates
- Cost per cycle vs estimated single-invocation cost
- Coverage rate distribution
- Most common escalation patterns (which phase, which direction)

### Out of Scope

- Real-time dashboards or web UI
- Metrics for non-TDD builds (existing metrics cover those)
- Automated tuning of cycle limits based on metrics (future work)

## Acceptance Criteria

- Each TDD phase invocation logs a `tdd_phase` record to the iteration metrics JSONL
- Phase records include: phase name, cycle number, bead/spec ID, model, tokens, duration, success, escalation, criteria state
- After all cycles, a `tdd_summary` record logs aggregate metrics per bead
- Aggregate metrics include total cycles, invocations, tokens, cost, duration, coverage rate, per-phase success rates
- `handoff_content_tokens` tracks the token cost of carrying file contents in phase prompts
- `gromit stats --tdd` displays a summary of TDD cycle metrics
- Metrics follow existing JSONL + JSON patterns (snake_case fields, explicit tags, append-only)
- Unit tests cover:
  - phase metric recording for each phase type
  - aggregate computation from phase records
  - JSONL serialization round-trip

## Decisions

1. **Extend existing JSONL, don't create a new file.** The iteration metrics file already handles granular per-invocation data. Adding a `type` discriminator keeps everything in one queryable stream.

2. **Track handoff content tokens explicitly.** This is the key comparison metric. If handoff prompts are cheaper than discovery-based prompts, the experiment succeeds. If they're more expensive, we know the handoff is too large.

3. **Per-phase success rates over aggregate.** A bead can succeed overall but have a 50% red-phase failure rate. Per-phase rates reveal which phases need tuning.

4. **Stats command over separate tool.** `gromit stats --tdd` follows the existing pattern of extending the stats command with flags rather than creating new subcommands.
