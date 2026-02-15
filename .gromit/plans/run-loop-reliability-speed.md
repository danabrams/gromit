---
created: 2026-02-15T00:00:00Z
decomposed: true
decomposed_at: "2026-02-15T19:56:46Z"
id: run-loop-reliability-speed
source_spec: run-loop-reliability-speed
---

# Run Loop Reliability and Speed Implementation Plan

**Goal:** Increase iteration success rate under transient provider failures, reduce p50/p90 iteration wall-clock time, and improve failure diagnosability from logs.

**Architecture:** Five focused changes across the provider, preflight, validation, and logging layers — each independently testable. StreamRun gets retry parity with Run. Preflight catches CODEX_HOME problems before the loop starts. Fast-gate validation scopes to touched packages. Stream logs get lifecycle markers for every invocation.

**Tech Stack:** Go, existing provider/runner/logger/validation packages

**Spec:** `.gromit/specs/run-loop-reliability-speed.md`

---

## Architecture

**Key Components:**

1. **Codex StreamRun retry parity** — Add transient retry loop to `StreamRun()` matching `Run()`'s pattern: 3 attempts max, bounded backoff, context cancellation respected.

2. **CODEX_HOME preflight hardening** — Resolve effective `CODEX_HOME` (mirroring `prepareCodexEnv()` logic), verify writability, log resolved path. Fail fast with remediation.

3. **Transport-aware ATDD fallback** — Extend existing codex-transport-fallback in `callbacks.go` to also handle startup errors. Add structured fallback log fields. Classify failure as `transport_disconnect`, `startup_error`, or `other`.

4. **Fast-gate package scoping** — `scopeValidationCommands()` replaces `./...` with targeted packages in go test commands. Per-command timeout wrapper prevents hung commands. Full gate unchanged.

5. **Stream-log lifecycle markers** — Emit `INVOCATION_START` and `INVOCATION_END` from `makeInvokeFn()` via `StreamLogger.LogEvent()`. Every failed iteration gets a non-empty stream log.

**Data Flow:**
```
Bead processing:
  -> preflight: CODEX_HOME check (C) <- runs once before loop
  -> ATDD invocation: StreamRun with retry (B) -> transport fallback (A)
  -> build invocation: StreamRun with retry (B) -> lifecycle markers (E)
  -> fast-gate validation: scoped commands (D) -> per-command timeout (D)
  -> full validation gate: unscoped (unchanged)
```

**Integration Points:**
- `internal/provider/codex.go` — StreamRun retry loop (B)
- `internal/runner/codex_preflight.go` — CODEX_HOME check (C)
- `internal/runner/callbacks.go` — ATDD fallback extension (A), stream lifecycle markers (E)
- `internal/runner/process.go` — Scoped validation helper + wiring (D)
- `internal/runner/validation/runner.go` — Per-command timeout (D)

**Tradeoffs:**
- StreamRun retry in provider (matching Run) vs. caller: Provider-level is simpler, callers don't change. Small budget (3 attempts) bounds cost.
- Package scoping in fast-gate only: Full gate keeps `./...` for comprehensive coverage. Fast gates trade breadth for speed.
- CODEX_HOME check in preflight vs. provider: Preflight is correct — fail-fast before the loop, not mid-bead.

## Test Strategy

**Unit Tests:** Each section has isolated tests — pure function tests for scoping helper, mock-based tests for retry/fallback, filesystem-based tests for preflight.

**Integration Tests:** Verify wiring: retry loop with context cancellation, scoped commands flowing through validation runner, lifecycle markers on success and failure paths.

**Mocking:** Mock router/providers for fallback tests. Mock cmdRunner for timeout tests. Temp directories for preflight writability. Buffer-backed StreamLogger for lifecycle marker assertions.

**Key Test Cases:**
- StreamRun: transient retry (transport, rate limit), non-transient stop (auth), context cancellation, usage from final attempt
- Preflight: writable, unwritable, creatable, unsafe temp path
- ATDD fallback: transport disconnect, startup error, auth (no fallback), no alternate provider
- Package scoping: 0/1/many packages, non-go-test passthrough, full gate unscoped
- Lifecycle markers: start+end on success, start+end on failure, no events between

## Implementation Tasks

### Task 1: Add retry loop to CodexProvider.StreamRun

**Files:**
- Modify: `internal/provider/codex.go`
- Test: `internal/provider/codex_test.go`

**What to Do:**
Wrap the core of `StreamRun()` (lines 92-154+) in a retry loop matching `Run()`'s pattern. Extract the single-attempt streaming execution into a helper (e.g., `streamOnce()`) that returns `(*Result, error)`. Retry on transient failures (`transport_disconnect`, `rate_limited`) up to `codexTransientRetryMax` attempts with `codexRetryBackoff()`. Respect context cancellation via `sleepWithContext()`. On retry, create a fresh `exec.Command` and pipes. Return the final attempt's result and usage data. Keep the non-handler path (plain text capture) simple — it can share the retry wrapper or remain unretried since it's less common.

**Acceptance Criteria:**
- StreamRun retries transient failures (transport_disconnect, rate_limited) up to 3 times with bounded backoff
- Context cancellation stops retry loop immediately and returns last result
- Non-transient failures (auth, other) return immediately without retry

**Dependencies:** None

**Notes:**
The existing `Run()` retry loop (lines 70-87) is the exact pattern to follow. The key difference is that StreamRun creates `exec.Command` and pipes per attempt, so the helper must encapsulate all of that. The `processCodexStream` call and `cmd.Wait()` must both be inside the per-attempt scope.

### Task 2: Add CODEX_HOME writability check to Codex preflight

**Files:**
- Modify: `internal/runner/codex_preflight.go`
- Modify: `internal/provider/codex.go` (export `ResolveCodexHome`)
- Test: `internal/runner/codex_preflight_test.go`

**What to Do:**
After the existing login status check in `preflightCodex()`, add a CODEX_HOME writability verification step. Call a new exported `ResolveCodexHome() (string, error)` helper (extracted from `prepareCodexEnv()` in `codex.go`) to get the effective CODEX_HOME path — the same path the provider will actually use at runtime. Verify: (1) path exists or can be created via `os.MkdirAll`, (2) path is writable by creating and removing a temp file, (3) path is not a bare `/tmp` or similar unsafe location that Codex helper setup rejects. On failure, return an actionable error with the resolved path and remediation steps. On success, log the resolved path once: `"Codex preflight: CODEX_HOME=%s"`.

**Acceptance Criteria:**
- Preflight fails with actionable error when CODEX_HOME is not writable
- Preflight passes when CODEX_HOME exists and is writable (or can be created)
- Resolved CODEX_HOME path is logged once per run

**Dependencies:** None

**Notes:**
`prepareCodexEnv()` is currently unexported. Extract the CODEX_HOME resolution logic into an exported `ResolveCodexHome() (string, error)` helper. The preflight only needs the path, not the full env slice. `prepareCodexEnv()` can then call `ResolveCodexHome()` internally.

### Task 3: Extend ATDD fallback for startup errors and structured logging

**Files:**
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/provider/codex.go` (add `FailureCategoryStartupError` classification)
- Test: `internal/runner/callbacks_test.go`

**What to Do:**
Extend the existing codex-transport-fallback block (callbacks.go lines 279-312) to also trigger on startup errors. Add a `FailureCategoryStartupError = "startup_error"` constant to the provider package. Update `classifyCodexFailure()` to detect startup patterns (`"failed to start"`, `"exec: not found"`, `"no such file or directory"`). Broaden the fallback condition from `result.FailureCategory == FailureCategoryTransportDisconnect` to a helper like `isATDDFallbackEligible(result)` that checks for transport_disconnect OR startup_error. Add structured log lines for fallback decisions including: primary provider/model, fallback provider/model, and failure class. Mark provider unavailable with cooldown for both transport and startup failures.

**Acceptance Criteria:**
- ATDD fallback triggers for startup errors (not just transport disconnect)
- Fallback decision logs include primary provider, fallback provider, and failure class
- Provider marked unavailable with cooldown for both transport and startup failures

**Dependencies:** None

**Notes:**
The existing fallback (lines 279-312) is well-structured. The main change is broadening the condition and adding structured logging. Consider adding the logging inside the existing `streamInvoke` wrapper so both success and failure paths are covered.

### Task 4: Add fast-gate package scoping helper

**Files:**
- Modify: `internal/runner/process.go`
- Test: `internal/runner/process_test.go`

**What to Do:**
Add a `scopeValidationCommands(commands []string, touchedPackages []string) []string` function that replaces `./...` with targeted package paths in `go test` commands when `touchedPackages` is non-empty. Pattern: for each command starting with `go test`, if it contains `./...` and touchedPackages is non-empty, replace `./...` with space-joined `./pkg/...` entries. Leave non-go-test commands unchanged (e.g., `golangci-lint`). Return original commands unchanged when touchedPackages is empty. This follows the same pattern as `scopeAcceptanceGoTestCommand()` in `methodology/executor.go`.

**Acceptance Criteria:**
- `go test ./...` scoped to `./internal/runner/... ./internal/provider/...` when touched packages `["internal/runner", "internal/provider"]`
- Non-go-test commands pass through unchanged
- Empty touchedPackages returns commands unchanged

**Dependencies:** None

**Notes:**
`detectTouchedPackages()` already exists in `process.go` and returns package paths from git diff. The scoping helper is a pure function. The existing `scopeAcceptanceGoTestCommand()` in `methodology/executor.go` is the exact pattern — consider extracting a shared helper or following the same approach.

### Task 5: Wire scoped validation into fast gate and add per-command timeout

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/validation/runner.go`
- Modify: `internal/config/config.go`
- Test: `internal/runner/process_test.go`
- Test: `internal/runner/validation/runner_test.go`

**What to Do:**
Two changes:

**(a) Wire scoped commands into fast gate:** In `runValidation()` and `runValidationWithRecovery()` (process.go), after getting `commands := r.cfg.Validation.FastCommandsOrDefault()`, apply `scopeValidationCommands(commands, bc.TouchedPackages)` when `bc.TouchedPackages` is non-empty. Full validation gate (`runFullValidationGate`) must NOT scope — it continues using `FullCommandsOrDefault()` as-is.

**(b) Per-command timeout:** Add `CommandTimeout` duration field to `ValidationConfig` in `config.go` (default: 0 = no timeout, for backward compat). In `validation/runner.go`'s `runValidationWithCommands()`, when `CommandTimeout > 0`, create a child context with the timeout for each command execution. If a command times out, treat it as a validation failure with a clear message indicating which command timed out.

**Acceptance Criteria:**
- Fast-gate validation uses scoped packages when `bc.TouchedPackages` is populated
- Full validation gate remains unscoped
- Per-command timeout kills hung validation command without blocking the bead timeout

**Dependencies:** Task 4 (scoping helper)

**Notes:**
`bc.TouchedPackages` is populated in `processBead()` after the build phase from `detectTouchedPackages(diff)`. The fast gate runs after the build, so touchedPackages will be available. For the per-command timeout, use `context.WithTimeout(ctx, r.cfg.Validation.CommandTimeout)` wrapping the `cmdRunner` call.

### Task 6: Add stream-log lifecycle markers for invocations

**Files:**
- Modify: `internal/runner/callbacks.go`
- Test: `internal/runner/callbacks_test.go`

**What to Do:**
In `makeInvokeFn()` (callbacks.go), emit lifecycle markers via the runner's stream logger:
- Before `r.executeClaudeInvocation()`: log `INVOCATION_START provider=%s model=%s tier=%s bead=%s`
- After `r.executeClaudeInvocation()` (in all return paths): log `INVOCATION_END provider=%s model=%s tier=%s success=%t duration=%s failure_category=%s`

The runner needs access to its `StreamLogger` — it's created in `Run()` (runner.go) and stored as a local. Add a `streamLogger` field to `Runner` and set it when created in `Run()`. `StreamLogger.LogEvent()` is nil-safe (no-op when logger is nil).

For ATDD invocations (the `invokeFn` inside `makeMethodologyExec`), add similar markers: `ATDD_INVOCATION_START` and `ATDD_INVOCATION_END` with the same fields.

**Acceptance Criteria:**
- Every invocation (build and ATDD) has INVOCATION_START and INVOCATION_END markers in stream log
- Failed invocations with no provider events still have both markers
- Markers include provider name, model, tier, and outcome

**Dependencies:** None

**Notes:**
`StreamLogger.LogEvent()` is nil-safe — it returns immediately when logger is nil. This means the markers are safe to emit unconditionally. The simplest path is storing the stream logger on the Runner struct when created in `Run()`.

---

## Notes

- **Relationship to layered-failure-triage plan:** The triage layer (beads gromit-pbv6, gromit-sd8s, gromit-p4qt) handles failure classification in `ExecuteWithRetry` — the build retry loop. This plan's Task 3 handles ATDD-phase fallback in `callbacks.go` — a different code path. They're complementary, not overlapping.
- **Measurement:** After implementation, run `gromit run` over 30+ iterations tracking success rate, median duration, and percentage of failed runs with non-empty stream logs. Compare against baseline.
- **Rollout order:** Tasks 1-3 (reliability) can ship first. Task 4-5 (speed) second. Task 6 (observability) can ship independently at any time.
- **Backward compatibility:** All changes are additive. Package scoping falls back to `./...` when touchedPackages is empty. Per-command timeout defaults to 0 (disabled). StreamRun retry uses the same budget as Run. Preflight check only runs when Codex is configured.
