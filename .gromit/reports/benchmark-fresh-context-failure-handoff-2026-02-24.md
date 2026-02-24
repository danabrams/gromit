# Benchmark Failure Handoff: `tdd_fresh_context` still failing (2026-02-24)

## Workspace and Scope
- Worktree: `/home/dabrams/gromit/.-gromit-debug-1771940174023091731`
- Bead: `gromit-q1b1k`
- Manifest: `.gromit/benchmarks/tdd-vs-single-pass.yaml`
- Latest result artifact: `.gromit/benchmarks/results/tdd-vs-single-pass/20260224T160530Z.json`

## Current Outcome
`single_pass` and `tdd_shared_context` succeed, but `tdd_fresh_context` still fails.

Latest `tdd_fresh_context` row (`.gromit/benchmarks/logs/tdd_fresh_context.jsonl`):
- `timestamp`: `2026-02-24T16:05:28.908642367Z`
- `success:false`
- `failure_phase:"green"`
- `model:""`
- `duration_ms:0`
- `input_tokens:0`, `output_tokens:0`, `cost_usd:0`
- `error:"build: TDD cycle runner: green validation failed: tests still failing after green phase"`

## What Was Fixed Already
1. **Quality metric population at source**
- Populates `quality_score` and `first_pass_success` before writing iteration logs.
- `single_pass` and `tdd_shared_context` now emit:
  - `validated:true`
  - `quality_score:1`
  - `first_pass_success:true`

2. **Benchmark review metric plumbing**
- Stage-4 review now emits `review` records into the same log stream used by benchmark aggregation.
- Aggregator also has fallback quality/first-pass derivation when fields are absent.

3. **Fresh-context resiliency changes**
- Added green-phase retry-on-validation-failure with tier escalation in TDD orchestrator (`internal/runner/tdd/orchestrator.go`).
- Set `tdd_fresh_context` benchmark overlay default tier from low -> medium in `internal/benchmark/harness.go`.
- Added invocation timeout guard in TDD invoke callback (`internal/runner/callbacks_tdd.go`) to avoid infinite provider hangs.
- Added benchmark overlay timeout caps in `internal/benchmark/worktree_run.go`:
  - invocation timeout cap: 360s
  - stall timeout cap: 120s
  - active stall timeout cap: 240s
  - bead timeout cap: 900s

## Why This Is Still Broken
Even after the above, fresh-context mode still records a terminal green-phase failure with no model/tokens/cost.

This indicates one of:
- failure happens before provider/model attribution is copied into iteration result for fresh-context failure path,
- green retry/escalation path is not actually being exercised in real run (despite tests),
- validation is failing immediately in a deterministic way after green attempts and then bailing before metrics are captured.

## High-Value Next Steps
1. Add explicit runtime tracing for fresh-context cycle:
- Log tier before each green invocation.
- Log whether green validation failed and whether escalation happened.
- Log final tier/model selected at failure.

2. Ensure failure path carries model/tokens when available:
- In the `build: TDD cycle runner` error path, plumb best-known model/tier into `IterationLog` before epilogue write.

3. Re-run just this benchmark and verify fresh row:
- Command:
  - `bd update gromit-q1b1k --status open --json`
  - `go run ./cmd/gromit benchmark run --manifest .gromit/benchmarks/tdd-vs-single-pass.yaml --single-bead gromit-q1b1k`
- Pass criteria for fresh row:
  - `success:true`
  - non-empty `model`
  - `input_tokens > 0`
  - `cost_usd > 0`

4. If still failing, isolate with focused test:
- Add integration-style test around `buildTDDCycleRunner` + `CycleOrchestrator` green retry path using real validation failure/success transitions.

## Quick Status Snapshot
- ✅ quality population fixed for successful modes
- ✅ benchmark report generated with newest artifact
- ❌ `tdd_fresh_context` still fails green and reports zero telemetry
