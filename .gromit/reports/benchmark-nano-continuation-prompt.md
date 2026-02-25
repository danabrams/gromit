# Benchmark Continuation: Nano vs Mini Cost Efficiency

## Context from previous session

We ran a `cost-efficiency-all-low` benchmark comparing 3 modes (single_pass, tdd_shared_context, tdd_fresh_context) across 5 beads of varying complexity, all using gpt-5.1-codex-mini.

### Previous results (gpt-5.1-codex-mini, $0.25/M in, $1.50/M out)

| Mode | Success | Elapsed | Real Cost | Quality |
|------|---------|---------|-----------|---------|
| single_pass | 5/5 | 22m30s | $2.69 | 1.00 |
| tdd_shared_context | 5/5 | 19m37s | $2.06 | 1.00 |
| tdd_fresh_context | 2/5 | 42m13s | $1.46 | 0.40 |

**Note:** Fresh-context results were invalid due to two bugs that have now been fixed:
1. `BuildTierDefault` was hardcoded to "medium" instead of "low" (fixed in harness.go:212)
2. Red phase timeout was 3 minutes for mini models (increased to 6 minutes in callbacks_tdd.go)

Full report: `.gromit/reports/benchmark-cost-efficiency-all-low-2026-02-25.md`

### Fixes applied and committed (commit 3d8dbf0e on main)

- Tier default fix for fresh context
- Timeout increase for mini models
- Quality metrics wiring
- Fresh-context telemetry wiring
- Temporary prompt dump logging (still active in /tmp/gromit-{red,green}-prompt-*.md)

### Key observation

The red phase in fresh context was writing BOTH tests and implementation code despite being told to only write tests. This is a model compliance issue — the prompt clearly says "MUST NOT" modify production code, but codex-mini does it anyway. This may affect nano even more.

## What to do now

### Step 1: Run the nano benchmark

```bash
gromit benchmark run --manifest .gromit/benchmarks/cost-efficiency-nano-vs-mini.yaml
```

This runs 5 beads x 3 modes = 15 worktree runs using `gpt-5-nano` ($0.05/M input, $0.40/M output) for all tiers.

### Step 2: Monitor results

- Check prompt dumps: `ls -lt /tmp/gromit-{red,green}-prompt-*.md`
- Check logs: `cat .gromit/benchmarks/logs/{single_pass,tdd_shared_context,tdd_fresh_context}.jsonl`
- The benchmark writes results to `.gromit/benchmarks/results/cost-efficiency-nano-vs-mini/`

### Step 3: Compare against mini results

Build a comparison table across both benchmarks:

**gpt-5.1-codex-mini pricing:** $0.25/M input, $1.50/M output
**gpt-5-nano pricing:** $0.05/M input, $0.40/M output

For each mode, compare:
- Success rate (X/5)
- Total real cost (using correct per-model pricing)
- Average cost per successful bead
- Time elapsed
- Quality score

### Step 4: Examine prompt dumps

Check `/tmp/gromit-{red,green}-prompt-*.md` to verify:
- Fresh context green prompts are now being generated (they weren't before due to bugs)
- The green phase is getting proper context (bead description, test failures, impl files)
- Whether nano follows TDD phase discipline (red = test only, green = impl only)

### Step 5: Save results report

Write a comparison report to `.gromit/reports/benchmark-nano-vs-mini-2026-02-25.md` with:
- Side-by-side cost comparison using correct pricing for each model
- Success rate comparison
- Analysis of whether nano is viable for the task complexity levels tested
- Recommendation on which model to use for each mode/complexity

## Beads under test

| Bead | Description | Complexity |
|------|-------------|-----------|
| gromit-7rj2f | Add compile-time check for retro.ProviderRunner | low |
| gromit-980iu | Swap json.Unmarshal -> jsonutil.ExtractArray | low |
| gromit-dh34r | Add --spc flag + guarded path to status cmd | medium |
| gromit-cflw8 | Repair provider-family labeling in retro rendering | high |
| gromit-50ggy | Unify benchmark report writing under single schema owner | high |

## Important notes

- The binary is already rebuilt and installed with all fixes
- gromit-50ggy may need to be reopened if it was closed: `bd reopen gromit-50ggy`
- The base_commit in the manifest must match an ancestor of HEAD
