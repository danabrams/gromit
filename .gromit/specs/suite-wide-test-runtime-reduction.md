---
id: suite-wide-test-runtime-reduction
source_ideas: []
created: 2026-02-18
---

# Suite-Wide Test Runtime Reduction

## Specification

`go test ./... -count=1` takes ~50 seconds. Five packages account for ~48s of that. Most of the time is spent on accidental real I/O: reading production log files, spawning subprocesses for `bd`/`git`/`tmux`/`go build`, and waiting through real sleep durations — all in tests where mocks exist but aren't used, or where injectable seams are missing.

This spec reduces the full suite from ~50s to under 5s through three layers: plugging missing seams, eliminating unnecessary subprocesses, and adding `t.Parallel()`.

### Profiling Baseline (2026-02-18)

| Package | Time | Dominant Cost |
|---------|------|---------------|
| `internal/runner` | 25.1s | Reads 94 real log files per `r.Run()`; real `go build ./...`; real git/tmux subprocesses; no `t.Parallel()` |
| `cmd/gromit` | 9.4s | 24 subprocess forks in epic tests; real `bd` calls in spec validation |
| `internal/provider` | 5.4s | Real `sleepWithContext()` in retry backoff; context cancellation blocked by child processes |
| `test/testutil` | 5.1s | Single test: `sleep 5` child not killed by context cancellation |
| `internal/bead` | 2.8s | 3 tests call real `bd` CLI instead of using existing `runFn` mock |
| All others | <3s | Already fast |

### Layer 1: Runner Seam Fixes (25s to ~2-3s)

**1a. Log file isolation.** `config.SetDefaults()` sets `Paths.Logs` to `.gromit/logs`, which resolves to the real project directory (94 JSONL files, 1.9MB). Every `r.Run()` reads them twice — in `startTrendUpdater()` and `initRunLoopState()`. Fix: set `cfg.Paths.Logs = t.TempDir()` in test helpers. Tests needing log data write fixtures into the temp dir.

**1b. Compile check bypass.** `Preflight.CompileCheck` defaults to nil (enabled), so tests run real `go build ./...`. Fix: set `cfg.Preflight.CompileCheck = boolPtr(false)` in test helpers. Tests verifying compile-check behavior opt in explicitly.

**1c. Injectable `getGitHead()`.** Hardcoded at `internal/runner/helpers.go:17` as `exec.Command("git", "rev-parse", "HEAD")`. Fix: add `gitHeadFn func() (string, error)` to Runner, matching the existing `gitDiffFn` pattern. Tests inject a stub.

**1d. Tmux suppression.** `tmux.NewManager()` spawns real subprocesses when `$TMUX` is set. Fix: `t.Setenv("TMUX", "")` in test helpers. No production code changes.

**1e. Default CmdRunner in test helpers.** Some helpers omit `CmdRunner`, causing real `sh -c` subprocess calls. Fix: all test-helper constructors set a no-op or recording `CmdRunner` by default.

### Layer 2: Provider, Testutil, and Bead Fixes (13.3s to ~0.5s)

**2a. Injectable sleep for retry backoff (provider, ~2s).** The codex retry loop calls `sleepWithContext()` with real durations (250ms, 750ms). Fix: add `sleepFn func(context.Context, time.Duration) error` to `CodexProvider`. Default to `sleepWithContext`; tests inject instant-return.

**2b. Context cancellation cleanup (provider, ~2s).** Mock scripts use `sleep 1`; `cmd.Wait()` blocks for the full second after context cancellation. Fix: set `cmd.WaitDelay = 100 * time.Millisecond` (Go 1.20+) in the production `run()` function so Wait doesn't block indefinitely on child I/O after the process is killed. This also improves production behavior.

**2c. Process group kill (testutil, ~5s).** `TestRunGromitHelperProcessWithStdin_Timeout` spawns `sleep 5` but context cancellation sends SIGKILL only to the parent bash process, not the child. Fix: in `runWithStdin()`, set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and configure `cmd.Cancel` to kill the process group via `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`. Production correctness fix.

**2d. Bead stubs (~2.8s).** Three error-wrapping tests use `NewClient()` without `runFn`, hitting real `bd`. Fix: use `&Client{runFn: func(...) (string, error) { return "", fmt.Errorf("boom") }}` like the other 92 tests.

### Layer 3: cmd/gromit Subprocess Elimination (9.4s to ~1-2s)

**3a. Epic tests: direct invocation (~4-5s).** 24 `TestEpicStatusCommand_*` tests each fork a full Go subprocess via `runGromit()`. Fix: call the cobra command's `RunE` function directly. The tests are in `package main` and have access to the commands. A helper captures stdout/stderr and sets environment variables without forking.

**3b. Spec validation: mock bead client (~1.5-2s).** `getSpecBaseCommit()` calls real `bd list` when specs pass validation. Fix: refactor `getSpecBaseCommit()` to accept a `bead.Client` parameter so tests pass a stub using the existing `runFn` hook.

### Layer 4: Parallelization

Only 2 of ~750 runner tests use `t.Parallel()`. Scope: the top 5 packages only.

**Shared state to fix before parallelizing:**
- `os.Chdir()` calls — replace with isolated temp dirs
- `os.Setenv()` calls — convert to `t.Setenv()`
- Package-level mutable vars — make per-test or protect with mutex
- Shared filesystem paths — redirect to `t.TempDir()`

**Phased rollout:**
1. Parallelize pure/mocked tests — those using only injected mocks with no side effects
2. Parallelize tests using `t.TempDir()` for isolation — most tests after seam fixes
3. Audit remaining tests with real side effects — refactor or leave sequential

## Acceptance Criteria

- `go test ./... -count=1` completes in under 5 seconds
- All tests pass
- Zero new flaky tests, verified by 10 consecutive green runs
- All existing behavior coverage preserved — no test logic deleted without equivalent replacement
- `go test -tags acceptance ./...` still covers any moved end-to-end paths

## Decisions

1. **Fix seams before parallelizing.** The biggest wins come from eliminating accidental real I/O. Parallelization is the final squeeze, not the first step.

2. **Production code changes where they improve correctness.** `cmd.WaitDelay` and process-group kill aren't test-only hacks — they fix real bugs where child processes survive cancellation.

3. **Parallelize only the top 5 packages.** The other 28 packages total <3s. The audit cost outweighs the benefit.

4. **Layer the work sequentially.** Each layer is independently valuable and shippable. If parallelization proves too risky, layers 1-3 alone deliver ~50s to ~4-5s.

5. **Extend existing patterns.** The `runFn` injection on `bead.Client`, `gitDiffFn` on Runner, and `acceptance` build tags are established project conventions. This spec follows them rather than introducing new abstractions.

## Research & Context

### Existing Specs

Two prior specs address subsets of this work:
- `reduce-runner-test-runtime` — runner package only, achieved 33% reduction (24.5s to 16.5s) through acceptance tag splits and tighter fakes
- `runner-test-runtime-reduction` — similar scope, same runner-focused approach

This spec supersedes both by addressing the full suite and targeting a deeper reduction through seam fixes and parallelization.

### Runner Remaining Hotspots (post prior optimization)

The top 10 slow runner tests are all 0.4-0.75s each, dominated by `r.Run()` calls that still read real log files and spawn real subprocesses. Layer 1 fixes target the root causes those prior specs didn't address.

### Mock Infrastructure Already in Place

- `internal/bead`: `runFn` field on Client (used by 92 of 95 tests)
- `internal/runner`: `mockBeadClient`, `mockClaudeClient`, `mockFailureAnalyzer`, `mockPromptRenderer`, `mockIterationLogger` in `interfaces_test.go`; injectable `cmdRunnerFn`, `gitDiffFn`, `processChecker`, `lookupHostFn`, `lookPathFn` on Runner
- `cmd/gromit`: `mockClaudeClient` in `epic_gap_analysis_test.go`

### Affected Files

**Layer 1 (runner seams):**
- `internal/runner/helpers.go` — add `gitHeadFn`
- `internal/runner/runner_test.go`, `internal/runner/interfaces_test.go` — update test helpers
- All test files using `newRunnerWithMocks`, `setupRunStopChTestRunner`

**Layer 2 (provider/testutil/bead):**
- `internal/provider/codex.go` — add `sleepFn`, set `cmd.WaitDelay`
- `test/testutil/runner.go` — process group kill
- `internal/bead/bead_test.go`, `ready_with_label_test.go` — stub 3 tests

**Layer 3 (cmd/gromit):**
- `cmd/gromit/epic_test.go` — replace `runGromit()` with direct calls
- `cmd/gromit/review_spec_validation_reclassified_test.go`, `retro_spec_validation_test.go` — inject mock bead client

**Layer 4 (parallelization):**
- All test files in the top 5 packages — add `t.Parallel()`, audit shared state
