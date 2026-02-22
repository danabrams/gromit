---
id: methodology-benchmark-harness
source_ideas: []
created: 2026-02-22
depends_on:
  - phase-cost-optimization
---

# Methodology Benchmark Harness (Controlled 3-Method Experiment)

## Specification

Add a reproducible benchmark command/workflow that runs the same bead cohort across three methodology modes and emits a comparable report.

Target comparison modes:

1. `single_pass` (no TDD)
2. `tdd` with `fresh_context_per_cycle=false`
3. `tdd` with `fresh_context_per_cycle=true`

The harness must run the same 5 beads in each mode, with at least one bead from each complexity tier (`low`, `medium`, `high`).

## Why

Today we can run each mode manually, but we cannot reliably guarantee:

- identical bead set and starting code state across runs
- identical provider/model-family constraints
- consistent metric extraction and rollup by tier/mode
- single-command report output for decision-making

Manual execution introduces drift and invalidates comparisons.

## Scope

### 1. Benchmark Definition

Add a benchmark manifest file under `.gromit/benchmarks/`:

```yaml
id: tdd-vs-single-pass
beads:
  - gromit-aaaa # low
  - gromit-bbbb # medium
  - gromit-cccc # high
  - gromit-dddd
  - gromit-eeee
modes:
  - single_pass
  - tdd_shared_context
  - tdd_fresh_context
provider: openai
model_family: gpt-5
low_tier_model: gpt-5-mini
medium_tier_model: gpt-5.3-codex
high_tier_model: gpt-5.3-codex
build_tier_default: low
final_review:
  enabled: true
  tier: high
  apply_fixes: true
```

Validation rules:

- exactly 5 beads
- includes at least one `complexity:low`, one `complexity:medium` (or unlabeled default), one `complexity:high`
- all beads exist and are open

### 2. Deterministic Run Setup

For each mode run:

- create isolated git worktree/session branch from same base commit
- seed same bead state (same 5 target beads, same order)
- apply mode-specific config overlay only (methodology + fresh-context)
- enforce provider/model-family pinning from manifest
- enforce low tier for build/validation phases
- run final high-tier non-interactive review and apply fixes
- run final validation

### 3. Metrics Extraction

Emit per-mode metrics from run logs:

- total `input_tokens`, `output_tokens`, `cost_usd` grouped by `actual_tier` (`low`, `medium`, `high`)
- total tokens and total cost overall
- wall-clock duration per mode (`run_started_at`, `run_finished_at`, elapsed seconds)
- code quality signals:
  - average `quality_score`
  - first-pass success rate
  - review findings/fixes applied
  - post-review validation pass/fail

### 4. Report Artifact

Write benchmark report to:

- `.gromit/benchmarks/results/<benchmark-id>/<timestamp>.json`
- `.gromit/benchmarks/results/<benchmark-id>/<timestamp>.md`

Report structure:

- manifest metadata (base commit, bead IDs, provider/models)
- per-mode summary table
- by-tier token/cost table per mode
- quality metrics table per mode
- normalized winner hints (fastest, cheapest, best quality, best cost/quality ratio)

## Acceptance Criteria

- A single command runs all three modes end-to-end from one manifest.
- Each mode uses the exact same 5 beads and same starting commit.
- Mode differences are limited to methodology settings (`build_strategy`, `fresh_context_per_cycle`).
- Provider/model family is identical across all modes.
- Build/validation phases run at low tier by default; final review runs at high tier.
- Output includes token and cost totals by tier (`low|medium|high`) and overall totals.
- Output includes wall-clock elapsed time for each mode.
- Output includes quality metrics (quality score, first-pass rate, review fixes, final validation).
- JSON and Markdown reports are written and deterministic for repeated runs.

## Decisions

1. **Manifest-driven benchmark**
A checked-in manifest makes runs auditable and repeatable.

2. **Worktree isolation per mode**
Hard isolation prevents cross-mode contamination of code and bead state.

3. **Tier-based aggregation, not model-name aggregation**
Tier answers the experiment question directly even when model names differ.

4. **High-tier review outside build loop**
This matches the intended policy: cheap build path, expensive final quality gate.

## Out of Scope

- Automatic statistical significance testing
- Multi-armed bandit allocation across modes
- Cross-project benchmark federation

## Related Specs

- `phase-cost-optimization`
- `tdd-fresh-context-per-cycle`
- `tdd-phase-metrics`
- `cost-per-spec-metric`
- `prompt-ab-framework`
