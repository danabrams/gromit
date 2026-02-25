# Benchmark Session Progress — 2026-02-24

## Goal
Test whether TDD fresh-context mode can succeed on simple beads, and whether it's more cost-effective using lower-tier models.

## Fixes Landed This Session (all on HEAD, uncommitted)

### 1. Model mismatch fix — `internal/benchmark/worktree_run.go`
**Bug**: Shared-map mutation in `applyBenchmarkOverlayToConfig()`. When merging overlay with existing provider, `pinned = existing` shared the Models map reference. Writing to `pinned.Models` mutated the original config.
**Fix**: Replace entire Models map with a fresh one from the overlay instead of writing individual keys.
**Test added**: `worktree_run_test.go` — assertions that original config is not mutated.

### 2. Green phase infinite loop fix — `internal/runner/tdd/orchestrator.go`
**Bug**: `runGreenPhaseUntilValidated` re-renders the green prompt via `renderGreenFn`, which has a side effect resetting `bc.Tier` back to default via `PhaseModelTier()`. After escalation sets tier to "high", re-render resets it to "medium", causing infinite escalation loop.
**Fix**: Added `bc.Tier = nextTier` after the render block to restore the escalated tier.

### 3. Validation failures in iteration logs — `internal/logger/logger.go` + `internal/runner/orchestrator.go`
**Bug**: Iteration logs recorded `validated: false` but no details about what failed.
**Fix**: Added `ValidationFailures []string` field to `IterationLog` struct, populated from `validateOut.ValidationFailures` in the orchestrator's validation failure path.

### 4. Validated flag on success path — `internal/runner/orchestrator.go`
**Bug**: The success path IterationLog never set `Validated: true`, so benchmark reports always showed `final_validation_passed: false` even when validation passed.
**Fix**: Added `Validated: true` to the success path IterationLog (line ~239).

### 5. Green phase missing implementation files — `internal/runner/tdd/assembly.go`
**Bug**: `TouchedFiles` only contained files from `git diff --name-only HEAD~1` after the red phase. Red phase only creates test files, so the green phase received NO implementation file content — it couldn't see the code it needed to modify.
**Fix**: When `implPaths` is empty but `testPaths` exist, discover sibling `.go` files (non-test) from the same directories as the test files using `filepath.Glob`. Added `discoverSiblingImplFiles()` helper.
**Tests added**: 3 new tests in `assembly_test.go`.

### 6. Sibling impl file over-inclusion — `internal/runner/tdd/assembly.go`
**Bug**: `discoverSiblingImplFiles` included ALL `.go` files in the directory (9 files for `internal/pipeline/`), drowning the green phase model in irrelevant context.
**Fix**: Direct filename matching (`helpers_test.go` → `helpers.go`), falling back to all siblings only when no direct match exists.
**Tests updated/added**: Updated 2 existing tests, added `TestDiscoverSiblingImplFilesFallsBackWhenNoDirectMatch`.

### 7. Green phase test file guard — `internal/runner/tdd/orchestrator.go` + `callbacks_tdd.go`
**Bug**: Green phase model modified `_test.go` files (creating duplicate function declarations) despite prompt instruction not to.
**Fix**: Added `RestoreTestFilesFn` callback that runs `git checkout HEAD --` on test files after each green phase invocation. Strengthened prompt warning in `PROMPT_tdd_green.md`.

### 8. TDD validation scoping — `internal/runner/callbacks_tdd.go`
**Bug**: TDD per-phase validation ran `go test ./...` (entire project), causing false negatives from pre-existing failures in unrelated packages (`internal/prompt` char budget tests, `internal/provider` flaky codex stream test).
**Fix**: `buildValidateFn` now uses `FastCommands` (scoped `test_touched.sh`) when available, consistent with the main loop's per-bead validation.

## Benchmark Manifests Created

- `.gromit/benchmarks/tdd-vs-single-pass-low.yaml` — all tiers set to `gpt-5.1-codex-mini`
- `.gromit/benchmarks/tdd-vs-single-pass-medium.yaml` — all tiers set to `gpt-5.2-codex`
- `.gromit/benchmarks/tdd-vs-single-pass.yaml` — currently has low=gpt-5.1-codex-mini, medium=gpt-5.2-codex, high=gpt-5.3-codex
- `.gromit/benchmarks/tdd-fresh-only-low.yaml` — fresh context only, low tier (debug manifest)

## Benchmark Results

### POST-FIX: gromit-kkst ("Fix DiffFiles nil→empty slice") — TRIVIAL bead, low tier

All 3 modes now succeed:

| Mode | Success | Validated | Cost | Duration | Input Tokens | Output Tokens |
|------|---------|-----------|------|----------|-------------|---------------|
| single_pass | pass | true | $0.37 | 30s | 200K | 1.7K |
| tdd_shared_context | pass | true | $0.61 | 53s | 324K | 3.2K |
| tdd_fresh_context | pass | true | $0* | 0s* | 181K | 3.0K |

*Fresh context cost/duration are zero due to telemetry bug — next fix target.

### PRE-FIX results (tainted by bugs, for reference):

#### gromit-q1b1k ("Unify pipeline.Idea and backlog.Idea") — HARD bead
- single_pass: success, $4.70, 308s
- tdd_shared_context: success, $4.02, 246s
- tdd_fresh_context: FAILED (green phase), 1551s, 5.6M tokens

#### gromit-5tk11 ("Add JSON tags to claude.Result struct") — MEDIUM bead
- All runs tainted by model mismatch bug (used gpt-5.2-codex regardless of manifest)

## What Needs to Happen Next

1. **Fix fresh-context telemetry** — `cost_usd` and `duration_ms` are zero on the fresh context success path. The TDD cycle runner accumulates telemetry into `bc.Result` but these values aren't making it to the iteration log.
2. **Commit all fixes** once telemetry is resolved.
3. **Re-run on medium tier** to compare cost-efficiency across tiers.
4. **Test on a harder bead** (gromit-5tk11 or gromit-q1b1k) to see if fresh context can handle non-trivial work.

## Key Files Modified (uncommitted)
- `internal/benchmark/worktree_run.go` — model mismatch fix
- `internal/benchmark/worktree_run_test.go` — new test
- `internal/runner/tdd/orchestrator.go` — infinite loop fix, test file guard, RestoreTestFilesFn
- `internal/runner/tdd/assembly.go` — sibling impl file discovery with filename matching
- `internal/runner/tdd/assembly_test.go` — 4 new/updated tests
- `internal/runner/callbacks_tdd.go` — gitRestoreFiles, scoped TDD validation
- `internal/logger/logger.go` — ValidationFailures field
- `internal/runner/orchestrator.go` — ValidationFailures population + Validated flag
- `.gromit/templates/PROMPT_tdd_green.md` — stronger test-file-locked warning
