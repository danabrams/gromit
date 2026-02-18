---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T01:59:17Z"
id: suite-wide-test-runtime-reduction
source_spec: suite-wide-test-runtime-reduction
---

# Suite-Wide Test Runtime Reduction Implementation Plan

**Goal:** Reduce `go test ./... -count=1` from ~50s to <5s by plugging missing mock seams, eliminating unnecessary subprocesses, and adding `t.Parallel()`.

**Architecture:** Four independent layers — runner seam fixes (~22s), provider/testutil/bead fixes (~13s), cmd/gromit subprocess elimination (~7s), and parallelization (final squeeze). Each layer is independently shippable.

**Tech Stack:** Go, existing mock injection patterns (`runFn`, `gitDiffFn`, `CmdRunner`), `syscall.SysProcAttr`, `cmd.WaitDelay` (Go 1.20+)

**Spec:** `.gromit/specs/suite-wide-test-runtime-reduction.md`

---

## Architecture

Four layers target the five slowest packages. Each layer eliminates a category of accidental real I/O:

1. **Layer 1 — Runner seams (25s → ~2-3s):** Log file isolation, compile check bypass, injectable `gitHeadFn`, tmux suppression, default CmdRunner in test helpers.
2. **Layer 2 — Provider/testutil/bead (13.3s → ~0.5s):** Injectable `sleepFn` for retry backoff, `cmd.WaitDelay` for child cleanup, process group kill in testutil, bead validation test stubs.
3. **Layer 3 — cmd/gromit (9.4s → ~1-2s):** Direct cobra invocation for epic tests, mock bead client for spec validation.
4. **Layer 4 — Parallelization:** `t.Parallel()` across top 5 packages after shared state audit.

Production code changes in Layers 2 (WaitDelay, process group kill) fix real bugs — not test-only hacks.

## Test Strategy

- Each seam fix is validated by existing tests running faster (no new test functions for plumbing)
- Primary gate: `go test ./... -count=1` completes in <5s with all tests passing
- Flake check: 10 consecutive green runs
- Race check: `go test -race ./...` passes after parallelization
- No test logic deleted without equivalent replacement

## Implementation Tasks

### Task 1: Runner test helper log isolation and compile check bypass

**Files:**
- Modify: `internal/runner/interfaces_test.go`
- Modify: `internal/runner/runner_test.go`

**What to Do:**
In `newRunnerWithMocks`, set `cfg.Paths.Logs = t.TempDir()` before `NewRunnerWithDeps` is called (which triggers `SetDefaults`). Set `cfg.Preflight.CompileCheck = boolPtr(false)` so tests don't run real `go build ./...`. Apply the same fixes to `setupRunStopChTestRunner` and any other test helper constructors that create runners. Tests needing log data should write fixtures into the temp dir explicitly.

**Acceptance Criteria:**
- Runner tests no longer read real `.gromit/logs` directory (verify via temp dir path in config)
- Runner tests no longer invoke `go build ./...` unless explicitly opted in
- All existing runner tests pass

**Dependencies:** None

**Notes:** `SetDefaults()` is called inside `NewRunnerWithDeps`, so `Paths.Logs` and `CompileCheck` must be set on the config *before* passing it in. Check for any tests that rely on reading real log files — they'll need fixture files written to the temp dir.

---

### Task 2: Add injectable `gitHeadFn` to Runner

**Files:**
- Modify: `internal/runner/helpers.go`
- Modify: `internal/runner/runner.go` (field declaration)

**What to Do:**
Add `gitHeadFn func() (string, error)` field to the Runner struct, following the existing `gitDiffFn` pattern. Add a `getHead()` wrapper method that checks `r.gitHeadFn != nil` before falling back to `getGitHead()`. Replace all direct `getGitHead()` calls in `process.go` and `run_init.go` with `r.getHead()`. In test helpers (`newRunnerWithMocks`, `setupRunStopChTestRunner`), inject a stub returning a fixed hash like `"abc123"`.

**Acceptance Criteria:**
- No direct `getGitHead()` calls remain outside of the `getHead()` wrapper
- Test helpers inject a stub `gitHeadFn` — no real `git rev-parse HEAD` in tests
- All runner tests pass

**Dependencies:** Task 1 (test helpers are being modified in both)

---

### Task 3: Tmux suppression and default CmdRunner in test helpers

**Files:**
- Modify: `internal/runner/interfaces_test.go`
- Modify: `internal/runner/runner_test.go`

**What to Do:**
In test helpers, add `t.Setenv("TMUX", "")` to prevent `tmux.NewManager()` from spawning real subprocesses. Ensure all test helper constructors set a no-op or recording `CmdRunner` when none is provided in deps, so tests never fall through to real `sh -c` subprocess execution.

**Acceptance Criteria:**
- Tests with `$TMUX` set in the environment don't spawn tmux subprocesses
- All test helper constructors provide a default CmdRunner when deps.CmdRunner is nil
- All runner tests pass

**Dependencies:** Task 1 (same files being modified)

---

### Task 4: Injectable `sleepFn` for provider retry backoff

**Files:**
- Modify: `internal/provider/codex.go`
- Modify: `internal/provider/codex_test.go` (if test setup needs updating)

**What to Do:**
Add `sleepFn func(context.Context, time.Duration) error` field to `CodexProvider`. Default to `sleepWithContext` in the constructor. In `Run()` and `StreamRun()` retry loops (lines 89 and 118), replace direct `sleepWithContext()` calls with `p.sleepFn()`. In tests, inject `func(ctx context.Context, d time.Duration) error { return nil }` for instant return. Expose a `SetSleepFn` setter or constructor option.

**Acceptance Criteria:**
- Provider retry tests complete in <100ms instead of >1s
- Production behavior unchanged (defaults to real `sleepWithContext`)
- All provider tests pass

**Dependencies:** None

---

### Task 5: Set `cmd.WaitDelay` for child process cleanup

**Files:**
- Modify: `internal/provider/codex.go`
- Modify: `internal/provider/claude.go` (if it has similar cmd.Wait patterns)

**What to Do:**
In the production `run()` functions that create `exec.Cmd`, set `cmd.WaitDelay = 100 * time.Millisecond` (Go 1.20+). This prevents `cmd.Wait()` from blocking indefinitely on child I/O after the process is killed via context cancellation. This is a production correctness fix — child processes currently survive cancellation.

**Acceptance Criteria:**
- `cmd.WaitDelay` is set on all `exec.Cmd` instances in codex.go
- Context-cancelled provider tests don't block waiting for child I/O
- All provider tests pass

**Dependencies:** None (can parallel with Task 4)

---

### Task 6: Process group kill in testutil `runWithStdin`

**Files:**
- Modify: `test/testutil/runner.go`

**What to Do:**
In `runWithStdin()`, set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` so child processes get their own process group. Configure `cmd.Cancel` to kill the entire process group via `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`. This ensures `sleep 5` children are killed when context is cancelled, not just the parent bash process. This is a production correctness fix.

**Acceptance Criteria:**
- `TestRunGromitHelperProcessWithStdin_Timeout` completes in <1s instead of 5s
- Child processes are killed when context is cancelled (no orphan `sleep` processes)
- All testutil tests pass

**Dependencies:** None

---

### Task 7: Stub 3 bead validation tests

**Files:**
- Modify: `internal/bead/bead_test.go`

**What to Do:**
Replace `NewClient()` with `&Client{runFn: func(args ...string) (string, error) { return "", fmt.Errorf("...") }}` in `TestClientShowValidation`, `TestClientCloseValidation`, and `TestClientAddCommentValidation`. These tests verify input validation (empty/invalid IDs) — they should never reach the real `bd` CLI. Follow the pattern used by the other 92 tests.

**Acceptance Criteria:**
- All 3 validation tests use `runFn` mock instead of real `bd` CLI
- No real `bd` subprocess spawned by any bead unit test
- All bead tests pass

**Dependencies:** None

---

### Task 8: Direct cobra invocation for epic tests

**Files:**
- Modify: `cmd/gromit/epic_test.go`
- Modify: `cmd/gromit/test_binary_helpers_test.go`

**What to Do:**
Add a helper function that executes a cobra command directly instead of forking a subprocess. The helper should: create a `bytes.Buffer` for stdout/stderr, set `rootCmd.SetOut(buf)` and `rootCmd.SetErr(errBuf)`, set `rootCmd.SetArgs(args)`, call `rootCmd.Execute()`, and return the captured output. Replace all 24 `TestEpicStatusCommand_*` tests to use this helper instead of `runGromit()`. Use `t.Setenv()` for environment variable setup.

**Acceptance Criteria:**
- All 24 epic status tests pass without forking subprocesses
- Test output matches expected patterns (same assertions as before)
- Epic test total time drops from ~5s to <500ms

**Dependencies:** None

**Notes:** Tests are in `package main` so they have access to `rootCmd`. Be careful to reset cobra state between tests — `rootCmd` may accumulate args. Create a fresh command tree per test if needed.

---

### Task 9: Mock bead client for spec validation tests

**Files:**
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/review_spec_validation_reclassified_test.go`
- Modify: `cmd/gromit/retro_spec_validation_test.go`

**What to Do:**
Refactor `getSpecBaseCommit(specName, specsDir string)` to accept a `bead.Client` parameter: `getSpecBaseCommit(specName, specsDir string, client *bead.Client)`. Update the call site in `determineReviewScope()` to pass the real client. In tests, pass a mock client using `&bead.Client{runFn: mockFn}` that returns canned `bd list` output.

**Acceptance Criteria:**
- `getSpecBaseCommit` accepts injectable bead client
- Spec validation tests don't invoke real `bd` CLI
- All review and retro tests pass

**Dependencies:** None

---

### Task 10: Parallelization — audit and fix shared state in top 5 packages

**Files:**
- Modify: All test files in `internal/runner/`, `cmd/gromit/`, `internal/provider/`, `test/testutil/`, `internal/bead/`

**What to Do:**
Audit all test files in the top 5 packages for shared state that prevents `t.Parallel()`:
- `os.Chdir()` calls → replace with isolated temp dirs or pass working dir explicitly
- `os.Setenv()` calls → convert to `t.Setenv()` (which is parallel-safe per test)
- Package-level mutable vars → make per-test or protect with mutex
- Shared filesystem paths → redirect to `t.TempDir()`

Fix all identified shared state issues. Do NOT add `t.Parallel()` yet — that's Task 11.

**Acceptance Criteria:**
- No `os.Chdir()` calls remain in top 5 package tests (replaced with temp dirs)
- No `os.Setenv()` calls remain (replaced with `t.Setenv()`)
- No unprotected package-level mutable state in test files
- All tests still pass after shared state fixes

**Dependencies:** Tasks 1-9 (seam fixes must be in place first)

**Notes:** This is the highest-risk task. Some tests may have subtle dependencies on shared state. Run `go test -race ./...` after changes.

---

### Task 11: Add `t.Parallel()` to top 5 packages

**Files:**
- Modify: All test files in `internal/runner/`, `cmd/gromit/`, `internal/provider/`, `test/testutil/`, `internal/bead/`

**What to Do:**
Phased rollout:
1. Add `t.Parallel()` to pure/mocked tests (those using only injected mocks with no side effects)
2. Add `t.Parallel()` to tests using `t.TempDir()` for isolation
3. Audit remaining tests with real side effects — refactor or leave sequential

For each test, add `t.Parallel()` as the first line after `func TestXxx(t *testing.T) {`. For table-driven tests, add it in both the outer test and each `t.Run` subtest.

**Acceptance Criteria:**
- Majority of tests in top 5 packages have `t.Parallel()`
- `go test -race ./...` passes (no data races)
- All tests pass

**Dependencies:** Task 10 (shared state must be fixed first)

---

### Task 12: Validation — timing verification and flake check

**Files:**
- No file changes — verification only

**What to Do:**
Run `go test ./... -count=1` and verify total time is under 5s. Run 10 consecutive times to check for flakes. Run `go test -race ./...` to verify no data races. Compare per-package timing against baseline to confirm each layer delivered expected savings.

**Acceptance Criteria:**
- `go test ./... -count=1` completes in under 5 seconds
- 10 consecutive runs all pass (zero flakes)
- `go test -race ./...` passes
- All existing behavior coverage preserved

**Dependencies:** Tasks 1-11

---

## Notes

- **Layer independence:** Tasks 1-3 (runner), 4-6 (provider/testutil), 7 (bead), 8-9 (cmd/gromit) are independent layers that can be worked in parallel.
- **Task 10-11 ordering is critical:** Shared state must be fixed before adding `t.Parallel()`. Adding parallel without fixing state will cause intermittent failures.
- **Production code changes:** Tasks 5 (WaitDelay) and 6 (process group kill) are correctness fixes. Review them as production changes, not test plumbing.
- **Risk area:** Task 8 (direct cobra invocation) requires careful cobra state management. If `rootCmd` leaks state between tests, create a fresh command tree per test using a factory function.
- **Supersedes:** This plan supersedes prior specs `reduce-runner-test-runtime` and `runner-test-runtime-reduction`.
