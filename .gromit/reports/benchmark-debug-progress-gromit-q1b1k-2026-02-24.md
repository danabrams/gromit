# Benchmark Debug Progress Report: gromit-q1b1k (2026-02-24)

## Reproduction
Command reproduced:

```bash
go run ./cmd/gromit benchmark run --manifest .gromit/benchmarks/tdd-vs-single-pass.yaml --single-bead gromit-q1b1k
```

Observed:
- `tdd_fresh_context` failure with `success:false`, `model:""`, `duration_ms:0`
- multiple runs/modes with `input_tokens:0`, `output_tokens:0`, `cost_usd:0`

## Root causes identified

### 1) Fresh-context mode cleanup failure (hard failure)
- Benchmark temp worktrees are created under names like:
  - `/home/dabrams/gromit/.-gromit-debug-1771940174023091731-gromit-benchmark-tdd_fresh_context-<timestamp>`
- Cleanup path (`git worktree remove --force`) fails with permission denied due to read-only files in:
  - `.gromit/tmp/go-mod-cache/...`
- Direct `rm -rf` also failed for the same reason.

Impact:
- Benchmark command exits non-zero at cleanup step.
- Leaves leaked temp worktree directories.

### 2) Build stage success handling bug (telemetry-quality bug)
File:
- `internal/pipeline/execute/build.go`

Issue:
- Build currently treats invocation as success when `err == nil`.
- It does not require `result.Success == true`.
- So provider failures that return a non-nil result with `Success:false` can be treated as successful build outcomes.

Impact:
- Can produce misleading "successful" iterations with empty/zero telemetry.
- Prevents expected escalation/failure handling path.

### 3) Same pattern likely in fresh-context TDD callbacks
File:
- `internal/runner/callbacks_tdd.go`

Issue:
- Invoke helpers call `StreamRun` and check only `err`, likely ignoring `result.Success`.

Impact:
- Fresh-context cycles may continue or terminate incorrectly on provider-level failures.
- Contributes to pre-invocation/failure-record noise and zeroed metrics.

## Additional findings
- In failing fresh-context mode worktrees, `.gromit/logs/` was not present after failed run attempts.
- Main repo had historical logs with mixed data quality: some entries had valid non-zero tokens/cost; others had zero-token “success” rows and pre-invocation zero rows.

## State at handoff
- Bead `gromit-q1b1k` is `open`.
- No code patches committed yet.
- No fix benchmark rerun completed yet.

## Required next actions
1. Patch build and fresh-context invoke paths to treat `result.Success=false` as failure.
2. Make benchmark cleanup robust against read-only cache files in temp worktrees.
3. Rerun benchmark for `gromit-q1b1k` and report:
   - winners
   - per-model breakdown
   - data-quality checks
4. Commit and push to `main`.
5. Update bd issue status.

## Completion update (2026-02-24)

### Implemented fixes
1. `internal/pipeline/execute/build.go`
   - Build now treats `result.Success=false` as invocation failure (not success).
   - Escalation retry path now also retries on unsuccessful provider results.
2. `internal/runner/callbacks_tdd.go`
   - Fresh-context TDD invoke/refactor callbacks now fail when `result.Success=false`.
3. `internal/benchmark/worktree_run.go`
   - Session cleanup now retries `git worktree remove --force` after normalizing permissions.
   - Added fallback cleanup (`os.RemoveAll` + `git worktree prune`) to avoid teardown dead-ends from read-only artifacts.

### Verification
- Focused tests pass for all patched areas (`internal/pipeline/execute`, `internal/runner`, `internal/benchmark`).
- Full `go test ./...` still has unrelated pre-existing failures in:
  - `internal/prompt` (`TestRulesPhaseCharBudgets`)
  - `internal/provider` (`TestProcessCodexStreamMapsMessageCreatedToSystem`)
  - `test/testutil` (`TestRunGromitWithStdin` ETXTBSY)

### Benchmark rerun
Command:

```bash
go run ./cmd/gromit benchmark run --manifest .gromit/benchmarks/tdd-vs-single-pass.yaml --single-bead gromit-q1b1k
```

Artifacts:
- `.gromit/benchmarks/results/tdd-vs-single-pass/20260224T141832Z.json`
- `.gromit/benchmarks/results/tdd-vs-single-pass/20260224T141832Z.md`

Result summary:
- `single_pass`: 295s, 3,072,972 in / 13,098 out, $5.561073
- `tdd_shared_context`: 170s, 1,334,567 in / 7,524 out, $2.440828
- `tdd_fresh_context`: 199s, 0 in / 0 out, $0

Data-quality outcome:
- Fresh-context row now records `success:false` (instead of previous misleading success behavior), with model empty and zero telemetry, indicating the failure is now surfaced correctly.
- Worktree cleanup no longer left a new leaked benchmark temp worktree in this run.

Follow-up issue filed:
- `gromit-xxfbs` (discovered-from `gromit-q1b1k`) to resolve the remaining fresh-context provider failure and improve error telemetry.
