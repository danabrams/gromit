Read these files to understand the current state of the tdd_fresh_context benchmark work, then let's discuss next steps:

1. `.gromit/reports/benchmark-fresh-context-telemetry-fix-2026-02-24.md` — summary of all fixes landed today
2. `.gromit/benchmarks/results/tdd-vs-single-pass/20260224T173820Z.json` — latest benchmark result artifact
3. `.gromit/benchmarks/logs/tdd_fresh_context.jsonl` — raw iteration log for the fresh-context mode

Context: We fixed tdd_fresh_context telemetry (model, tokens, tiers now populate on failure) and improved green-phase robustness (TouchedFiles discovery after red phase, green retry re-renders prompt with validation failure context). Across 4 benchmark runs, tokens went from 0 to 5.6M input / 66K output, confirming the plumbing works and retries fire. Two issues remain:

- `success: false` — the green phase still fails for bead gromit-q1b1k ("Unify pipeline.Idea and backlog.Idea"). The LLM can't solve this cross-package task with only per-phase prompt context. single_pass and tdd_shared_context succeed on the same bead.
- `cost_usd: 0` — the codex provider streaming API doesn't report total_cost_usd for these invocations (tokens are populated; cost is not).

Key files changed: `internal/runner/callbacks_tdd.go`, `internal/runner/tdd/orchestrator.go`, `internal/runner/tdd_pipeline_adapter.go`, `internal/runner/process_methodology_atdd.go`, `internal/pipeline/execute/build.go`, `internal/runner/orchestrator.go`.
