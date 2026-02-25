# Benchmark Continuation Prompt

## What happened so far

We set up and ran a benchmark called `cost-efficiency-all-low` to test the hypothesis that TDD fresh context can succeed at lower cost than single_pass and TDD shared context across varying complexity levels, using the lowest tier model (gpt-5.1-codex-mini) for everything.

### Benchmark config: `.gromit/benchmarks/cost-efficiency-all-low.yaml`

5 real beads of varying complexity, all 3 modes, gpt-5.1-codex-mini in every tier slot.

| Bead | Description | Complexity Label |
|------|-------------|-----------------|
| gromit-7rj2f | Add compile-time check for retro.ProviderRunner | complexity:low |
| gromit-980iu | Swap json.Unmarshal → jsonutil.ExtractArray | complexity:low |
| gromit-dh34r | Add --spc flag + guarded path to status cmd | complexity:medium |
| gromit-cflw8 | Repair provider-family labeling in retro rendering | complexity:high |
| gromit-50ggy | Unify benchmark report writing under a single schema owner (renamed from "Consolidate..." to avoid proactive decomposition keyword trigger) | complexity:high |

### First run results (`.gromit/benchmarks/results/cost-efficiency-all-low/20260224T223030Z.json`)

- **single_pass**: 4/5 succeeded, $10.19, 19.3 min
- **tdd_shared_context**: 4/5 succeeded, $12.05, 16.1 min
- **tdd_fresh_context**: 0/5 succeeded, $0.00 (telemetry not captured), 23.3 min
- gromit-50ggy failed in all modes (0 tokens, proactive decomposition skipped it)

### Root cause investigation (3 subagents)

**1. Missing quality metrics**: `quality_score` always 0, `first_pass_success` always false. Root cause: orchestrator.go never called `ComputeQualityScore()` or set `FirstPassSuccess`.

**2. TDD fresh context failures**: All beads fail with "green validation failed: tests still failing after green phase". Root causes:
   - Tier not wired from benchmark overlay — `TDDPipelineAdapter` used bare `SelectTier()` instead of `PhaseModelTier("build", ...)`
   - `bc.Result.Model` never set (empty model in logs)
   - Green phase prompt lacked bead description and scoped test command

**3. gromit-50ggy**: Title contained "Consolidate" which triggered proactive decomposition keyword regex. Bead was decomposed and skipped before any LLM invocation. Fixed by renaming to "Unify benchmark report writing..."

### Fixes applied (in working tree, not committed)

**Fix 1 — Quality metrics** (`internal/runner/orchestrator.go` lines 234-262):
- Added `FirstPassSuccess: !validationRetried && !escalated`
- Added `QualityScore: logger.ComputeQualityScore(0, 0, validationRetried, false, escalated, 0)`

**Fix 2 — TDD fresh context** (`internal/runner/callbacks_tdd.go`):
- Changed tier init from `cfg.SelectTier(...)` to `cfg.PhaseModelTier("build", cfg.SelectTier(...))`
- Set `bc.Result.Model = bc.Model` when model derived from tier
- Added `ScopedTestCommand` to green context from `cfg.Validation.FastCommands[0]`
- `BeadDescription` added to `TDDGreenContext` struct in `context_types.go` but NOT rendered in template yet (user wants to examine prompts first)

**Fix 3 — Temporary prompt logging** (`internal/runner/callbacks_tdd.go`):
- Red phase prompt dumped to `/tmp/gromit-red-prompt-{beadID}-{timestamp}.md`
- Green phase prompt dumped to `/tmp/gromit-green-prompt-{beadID}-{timestamp}.md`

### Binary rebuilt and installed with all fixes

`go build && go install` passed. All tests pass.

## What to do next

1. Re-run the benchmark: `gromit benchmark run --manifest .gromit/benchmarks/cost-efficiency-all-low.yaml`
2. Monitor results — check `/tmp/gromit-{red,green}-prompt-*.md` files to examine what the green phase is receiving
3. Based on prompt examination, decide whether to add `BeadDescription` to the green template or make other prompt improvements
4. Compare results across all 3 modes with the fixes applied
5. If satisfied, remove the temporary prompt logging and commit the real fixes
