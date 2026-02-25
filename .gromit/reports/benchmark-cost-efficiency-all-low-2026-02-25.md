# Benchmark Report: cost-efficiency-all-low (2026-02-25)

## Run Details

- **Manifest:** `.gromit/benchmarks/cost-efficiency-all-low.yaml`
- **Base commit:** ae0f672c
- **Results file:** `.gromit/benchmarks/results/cost-efficiency-all-low/20260225T003213Z.json`
- **Previous run (reference):** `.gromit/benchmarks/results/cost-efficiency-all-low/20260224T223030Z.json`
- **Provider:** OpenAI
- **Model (all tiers):** gpt-5.1-codex-mini
- **Pricing:** $0.25/M input, $1.50/M output

## Beads Under Test

| Bead | Description | Complexity |
|------|-------------|-----------|
| gromit-7rj2f | Add compile-time check for retro.ProviderRunner | low |
| gromit-980iu | Swap json.Unmarshal -> jsonutil.ExtractArray | low |
| gromit-dh34r | Add --spc flag + guarded path to status cmd | medium |
| gromit-cflw8 | Repair provider-family labeling in retro rendering | high |
| gromit-50ggy | Unify benchmark report writing under single schema owner | high |

## Results Summary (corrected pricing)

| Mode | Success | Elapsed | Input Tokens | Output Tokens | Real Cost | Avg Cost/Bead | Quality |
|------|---------|---------|-------------|--------------|-----------|--------------|---------|
| **single_pass** | **5/5** | 22m30s | 9.94M | 132K | **$2.69** | $0.54 | 1.00 |
| **tdd_shared_context** | **5/5** | 19m37s | 7.59M | 111K | **$2.06** | $0.41 | 1.00 |
| tdd_fresh_context | 2/5 | 42m13s | 5.42M | 70K | $1.46 | $0.73* | 0.40 |

> **WARNING: tdd_fresh_context data is NOT valid.** Two bugs caused incorrect behavior:
> 1. `BuildTierDefault` hardcoded to `"medium"` in `internal/benchmark/harness.go:212` (should be `"low"`)
> 2. 3-minute timeout in `resolveTDDPhaseInvocationTimeout()` killed red phases before completion
>
> See "Fresh Context Bugs" section below. This mode must be re-run after fixes.

## Per-Bead Breakdown

### Single Pass (5/5, all first-pass success)

| Bead | Complexity | Input | Output | Cost | Duration |
|------|-----------|-------|--------|------|----------|
| gromit-7rj2f | low | 1.49M | 22K | $0.41 | 3m54s |
| gromit-980iu | low | 593K | 7K | $0.16 | 1m28s |
| gromit-dh34r | medium | 550K | 8K | $0.15 | 1m36s |
| gromit-cflw8 | high | 2.81M | 40K | $0.76 | 6m20s |
| gromit-50ggy | high | 4.50M | 56K | $1.21 | 9m02s |

### TDD Shared Context (5/5, all first-pass success)

| Bead | Complexity | Input | Output | Cost | Duration |
|------|-----------|-------|--------|------|----------|
| gromit-7rj2f | low | 1.18M | 15K | $0.32 | 2m56s |
| gromit-980iu | low | 1.28M | 13K | $0.34 | 3m24s |
| gromit-dh34r | medium | 1.70M | 17K | $0.45 | 3m04s |
| gromit-cflw8 | high | 1.98M | 38K | $0.55 | 5m45s |
| gromit-50ggy | high | 1.44M | 28K | $0.40 | 4m19s |

### TDD Fresh Context (2/5 -- INVALID DATA, see bugs below)

| Bead | Complexity | Model | Result | Error |
|------|-----------|-------|--------|-------|
| gromit-7rj2f | low | sonnet | FAIL | red phase timed out after 3m0s |
| gromit-980iu | low | gpt-5.1-codex-mini | PASS | -- |
| gromit-dh34r | medium | gpt-5.1-codex-mini | PASS | -- |
| gromit-cflw8 | high | gpt-5.1-codex-mini | FAIL | red phase timed out after 3m0s |
| gromit-50ggy | high | sonnet | FAIL | red phase timed out after 3m0s |

## Key Observations (valid modes only)

1. **TDD shared context wins on efficiency**: 23% cheaper ($2.06 vs $2.69) and 13% faster than single pass, with identical 100% success rate and quality scores
2. **gpt-5.1-codex-mini handles all complexity levels**: Even "high" complexity beads succeed first-pass at the lowest tier
3. **High-complexity beads drive cost**: gromit-50ggy and gromit-cflw8 account for ~73% of single_pass cost but only 46% of shared_context cost -- TDD shared context amortizes context better for complex beads
4. **Token efficiency**: Shared context uses 24% fewer input tokens than single pass (7.59M vs 9.94M), suggesting context reuse reduces redundant file reads

## Fresh Context Bugs (must fix before re-running)

### Bug 1: Wrong tier assignment
- **File:** `internal/benchmark/harness.go:212`
- **Problem:** `BuildModeOverlay` for `tdd_fresh_context` hardcodes `BuildTierDefault: "medium"`, while single_pass and tdd_shared_context use `"low"`
- **Effect:** Fresh context resolves through medium tier, sometimes falling back to Sonnet instead of gpt-5.1-codex-mini
- **Fix:** Change `"medium"` to `"low"` on line 212

### Bug 2: Aggressive red phase timeout
- **File:** `internal/runner/callbacks_tdd.go:211-221`
- **Problem:** `resolveTDDPhaseInvocationTimeout()` gives models containing "mini" a 3-minute timeout, which is too short for complex beads
- **Effect:** 3/5 beads fail with "red phase: phase invocation timed out after 3m0s" before reaching the green phase
- **Fix:** Increase mini model timeout (suggest 6 minutes to match codex/sonnet)

### Bug 3: No green phase execution
- **Observation:** Zero green prompt dumps across all 5 beads
- **Cause 1:** Timed-out beads never reach green phase
- **Cause 2:** Successful beads (980iu, dh34r) had all criteria satisfied during red phase (red-skip), so green was never needed
- **Implication:** The green phase prompt has still not been validated in a real benchmark run

## Changes Applied Before This Run

Fixes from previous session (working tree, uncommitted):
1. Quality metrics wiring in `internal/runner/orchestrator.go`
2. TDD fresh-context tier wiring via `PhaseModelTier("build", ...)` in `callbacks_tdd.go`
3. `bc.Result.Model` assignment when derived from tier
4. `ScopedTestCommand` added to green context
5. Temporary prompt logging to `/tmp/gromit-{red,green}-prompt-*.md`
6. gromit-50ggy renamed to avoid proactive decomposition keyword trigger

## Next Steps

1. Fix Bug 1 (harness.go tier default) and Bug 2 (timeout)
2. Re-run benchmark to get valid fresh-context data
3. Examine green prompt dumps once they actually fire
4. If satisfied, remove temporary prompt logging and commit real fixes
