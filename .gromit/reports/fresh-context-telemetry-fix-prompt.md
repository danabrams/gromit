# Continuation Prompt — Fix TDD Fresh Context Telemetry

Read `.gromit/reports/benchmark-session-progress-2026-02-24.md` for full context on the session so far. We've fixed 8 bugs and all 3 benchmark modes now pass on gromit-kkst. One issue remains:

## The Bug

TDD fresh context iteration logs show `cost_usd: 0` and `duration_ms: 0` even on successful runs that consume 181K+ tokens. The telemetry accumulation works (tokens are recorded) but cost and duration aren't flowing to the iteration log.

## Where to Look

The data flow is:

1. **Accumulation**: `internal/runner/callbacks_tdd.go` — `buildInvokeFnWithTelemetry()` accumulates `bc.Result.CostUSD`, `bc.Result.InputTokens`, `bc.Result.OutputTokens` after each TDD phase invocation.

2. **Build stage**: `internal/pipeline/execute/build.go` — the Build stage runs the TDD cycle runner. Check how it returns the result and whether cost/duration from `bc.Result` are captured.

3. **Orchestrator**: `internal/runner/orchestrator.go` — creates `IterationLog` entries. Check the success path (~line 230-250) for how `CostUSD` and `DurationMs` are populated. The issue is likely that the orchestrator reads cost/duration from a different source than where TDD accumulates it (e.g., from the Claude invocation result rather than `bc.Result`).

4. **Logger**: `internal/logger/logger.go` — `IterationLog` struct definition. Check what fields exist for cost/duration.

## How to Verify

After fixing, run:
```
go build ./... && go test ./internal/runner/... ./internal/pipeline/execute/...
```

Then run the benchmark:
```
bd update gromit-kkst --status open --json
go run ./cmd/gromit benchmark run --manifest .gromit/benchmarks/tdd-fresh-only-low.yaml --single-bead gromit-kkst
```

Check `.gromit/benchmarks/logs/tdd_fresh_context.jsonl` — `cost_usd` and `duration_ms` should be non-zero.

## Files Modified This Session (uncommitted)

All changes are uncommitted on HEAD. Don't commit yet — just fix the telemetry bug, verify it works, then we'll commit everything together.
