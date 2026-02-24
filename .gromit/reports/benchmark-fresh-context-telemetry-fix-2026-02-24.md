# Benchmark Fix Report: `tdd_fresh_context` telemetry and robustness (2026-02-24)

## Newest Artifact
`.gromit/benchmarks/results/tdd-vs-single-pass/20260224T173820Z.json`

## What Was Fixed (8 changes across 5 files)

### Telemetry plumbing

1. **`internal/runner/callbacks_tdd.go`** — Added `buildInvokeFnWithTelemetry()` that accumulates `provider.Result` (model, tokens, cost) into `bc.Result` after each StreamRun call. Added `gitListChangedFiles()` and wired `ListChangedFilesFn` to populate `state.TouchedFiles` after each phase so the green handoff includes test and implementation file contents.

2. **`internal/runner/tdd/orchestrator.go`** — Added `SetInvokeFn()` so the bc-aware invoke function can be swapped in at RunCycles time. Added `ListChangedFilesFn` callback type and `updateTouchedFiles()` helper to merge newly changed files into cycle state.

3. **`internal/runner/tdd_pipeline_adapter.go`** — Returns partial telemetry (model, tokens, tiers) even on cycle error instead of empty `TDDCycleResult{}`. Falls back to `bc.Tier` when `PhaseMetrics` is empty.

4. **`internal/pipeline/execute/build.go`** — Returns partial `pipeline.Output` (with model, tokens, tiers) on TDD cycle error instead of `Output{}`. Added `Model`, `CostUSD`, `InputTokens`, `OutputTokens` fields to `TDDCycleResult`.

5. **`internal/runner/orchestrator.go`** — Populates `Model`, `CostUSD`, `InputTokens`, `OutputTokens`, `OriginalTier`, `ActualTier` in the failure `IterationLog` from `buildOut`, so benchmark logs capture telemetry even on build failure.

6. **`internal/runner/process_methodology_atdd.go`** — `aggregateTDDPhaseMetricsToResult` now returns early when `PhaseMetrics` is empty, preserving values already accumulated directly into `bc.Result` by the telemetry-aware InvokeFn.

### Green phase robustness

7. **`internal/runner/tdd/orchestrator.go`** — `runGreenPhaseUntilValidated` now re-assembles the green handoff with the latest validation failure output and current implementation files, then re-renders the prompt on retry. Previously it reused the identical original prompt, making fresh-context retries ineffective.

8. **`internal/runner/tdd/orchestrator.go`** — After the red phase invocation, `runOneCycle` calls `updateTouchedFiles` to discover which files were created/modified, ensuring the green handoff includes actual test and implementation file contents instead of empty maps.

## Results Across 4 Benchmark Runs

| Run | Fix Applied | `model` | `input_tokens` | `output_tokens` | `success` |
|-----|------------|---------|----------------|-----------------|-----------|
| 1 (baseline) | none | `""` | `0` | `0` | `false` |
| 2 | telemetry accumulation | `gpt-5.3-codex` | `962,261` | `9,477` | `false` |
| 3 | + TouchedFiles discovery | `gpt-5.3-codex` | `1,113,974` | `19,230` | `false` |
| 4 | + green retry with context | `gpt-5.3-codex` | `5,614,451` | `65,968` | `false` |

## Latest Run Comparison (20260224T173820Z)

| Mode | Elapsed (s) | Input Tokens | Output Tokens | Cost USD | Validation |
|------|-------------|-------------|---------------|----------|------------|
| single_pass | 308 | 2,569,028 | 14,620 | $4.70 | success=true |
| tdd_shared_context | 246 | 2,204,041 | 11,522 | $4.02 | success=true |
| tdd_fresh_context | 1,551 | 5,614,451 | 65,968 | $0.00 | success=false |

## What Remains

### `success: false` — green phase still fails
The fresh-context green phase fails because each LLM invocation is a brand-new process with only per-phase prompt context (test failure output + implementation files). The bead task ("Unify pipeline.Idea and backlog.Idea types to prevent schema drift") requires cross-package type system understanding that the LLM cannot achieve without full conversation history.

Potential next steps:
- Try with a simpler bead that requires fewer cross-package changes
- Enrich the green prompt with more codebase context (e.g., package-level type summaries)
- Add a "context assembly" phase that gathers relevant code before invoking the green phase

### `cost_usd: 0` — provider doesn't report cost
The codex provider's streaming API populates `input_tokens` and `output_tokens` but does not include `total_cost_usd` in the streaming events for these invocations. This is a provider-level issue, not a plumbing problem. The single-pass and shared-context modes report cost because they go through a different invocation path that receives cost data.

Potential next steps:
- Compute cost from token counts using known per-model pricing
- Investigate whether the codex streaming format omits cost for short-lived sessions

## Files Modified
- `internal/runner/callbacks_tdd.go`
- `internal/runner/tdd/orchestrator.go`
- `internal/runner/tdd_pipeline_adapter.go`
- `internal/runner/process_methodology_atdd.go`
- `internal/pipeline/execute/build.go`
- `internal/runner/orchestrator.go`
