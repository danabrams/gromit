# Leaf-Bead File Count vs First-Pass Success Experiment

## Problem

The 2026-02-18 retro found that leaf beads touching 1-3 files achieved 70% first-pass success, while historical beads touching 6+ files averaged 3.3%. This signal appeared across all model tiers during the gromit-deid epic. The question: is this causal, or merely correlated with gromit-deid's task family?

## Hypothesis

Leaf beads that touch fewer files succeed on the first pass more often because the model can hold the full change context. The file count, not the task family, drives the improvement.

## Approach: Measure-Only Instrumentation

Add a `files_touched` count to the iteration metrics pipeline. After each bead completes (pass or fail), count distinct files from the git diff. Bucket iterations by file count (1-3, 4-6, 7+) and compare first-pass success rates at retro time.

No enforcement, no decomposition changes, no model routing changes.

## Design

### Instrumentation

**Structs to change:**

1. `IterationLog` in `internal/logger/logger.go` — add `FilesTouched int`
2. `IterationMetric` in `internal/logger/process_trend.go` — add `FilesTouched int`
3. `BuildContinuousMetrics()` — propagate the field from log to metric

**Capture point:** In the runner's iteration callback (`internal/runner/callbacks.go`), where git diff is already fetched, count files using the existing `ParseDiffFiles()` from `internal/runner/methodology/diff.go` and set the field before calling `LogIteration()`.

### Retro Template

Add a file-count bucketing section to the retro prompt template. The retro LLM:

1. Reads `files_touched` from iteration metrics
2. Buckets into 1-3, 4-6, 7+
3. Reports first-pass success rate per bucket
4. Flags buckets with n < 5
5. Notes which epics contributed iterations (diversity check)

### Experiment Protocol

- **Start:** After the prompt-budget experiment concludes
- **Target:** 30 leaf-bead iterations across 2-3 different epics (no single epic > 50%)
- **Baseline:** 3.3% historical first-pass mean

### Conclusion Criteria

- **Signal confirmed:** 1-3 bucket first-pass rate > 2x the 4-6 or 7+ bucket rate, with n >= 10 per compared bucket. Next step: Codex routing experiment.
- **Signal refuted:** Rates similar across buckets. The improvement was task-family-specific.
- **Inconclusive:** Higher buckets have n < 5 after 30 iterations. Extend, or note that natural decomposition already produces small beads.

## Scope

~4 files changed, ~30-50 lines of new code plus template additions. The experiment infrastructure already handles the PDSA lifecycle.

## What We Do NOT Build

- Real-time dashboard or control chart changes
- Enforcement in decomposition
- Changes to model routing
- New CLI commands
- Changes to the spec format
