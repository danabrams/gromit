---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T17:21:48Z"
id: token-budget-guardrail
source_spec: token-budget-guardrail
---

# Token Budget Guardrail Implementation Plan

**Goal:** Prevent per-bead timeouts caused by high-token retry churn via a three-layer defense: prompt compression, token budget gating, and adaptive timeout clamping.

**Architecture:** Extend existing config/retry/invocation patterns — add cumulative token tracking to BeadContext, token budget checks alongside existing attempt/wall-clock checks in the escalation handler, failure context truncation before retry prompt injection, and invocation timeout clamping in the invoker. Token budget exhaustion routes to decomposition (same as bead timeout).

**Tech Stack:** Go, existing config/escalation/execution packages

**Spec:** `.gromit/specs/token-budget-guardrail.md`

---

## Architecture

**Overview:**
Three-layer defense against token-budget-induced bead timeouts, all following existing patterns.

**Key Components:**
1. **Config extensions** (`internal/config/config.go`): `MaxFailureContextChars` and `MaxInputTokensPerBead` on `ClaudeConfig` and `ModelTimeoutOverrides`. Resolution via existing `TimeoutsForModel` pattern.
2. **BeadContext token tracking** (`internal/runner/runtypes/types.go`): `CumulativeInputTokens` and `CumulativeOutputTokens` fields.
3. **Escalation handler** (`internal/runner/escalation/handler.go`): Token accumulation after each invocation, budget check in `checkRetryBudgetBeforeAttempt`, failure context truncation in `AnalyzeAndHandleFailure`.
4. **Invoker timeout clamping** (`internal/runner/execution/invoker.go`): Clamp invocation timeout to remaining bead budget minus 2-min validation reserve.

**Data Flow:**
```
StreamStats collects per-invocation tokens
  → InvocationResult carries tokens back to handler
  → Handler accumulates into BeadContext.CumulativeInputTokens
  → Next iteration: checkRetryBudgetBeforeAttempt() checks cumulative vs cap
  → If exceeded: route to AttemptDecomposition()
```

**Files to Modify:**
- `internal/config/config.go` — add config fields and resolution method
- `internal/runner/runtypes/types.go` — add cumulative token fields
- `internal/runner/escalation/handler.go` — token accumulation, budget check, truncation
- `internal/runner/execution/invoker.go` — adaptive timeout clamping

**Tradeoffs:**
- Absolute cap over velocity-based: simpler, follows existing timeout pattern
- Truncate over summarize: deterministic, zero-cost, no extra LLM call
- 2-min validation reserve as constant: matches `analysis_timeout`, avoids config proliferation
- Decomposition on token exhaustion, not escalation: token churn = scope signal, not capability signal

---

## Test Strategy

**Layer 1 — Failure Context Truncation (handler_test.go):**
- Short context passes through unchanged
- Long context truncated to tail with `[truncated]` prefix
- Zero/unset MaxFailureContextChars = no truncation (backward compat)

**Layer 2 — Token Budget (handler_test.go + config_test.go):**
- checkRetryBudgetBeforeAttempt returns error when cumulative exceeds cap
- Token accumulation across retries in ExecuteWithRetry
- Per-model override takes precedence over top-level default
- Zero/unset cap = no enforcement (backward compat)
- Token budget exhaustion routes to AttemptDecomposition

**Layer 3 — Adaptive Timeout (invoker_test.go):**
- Timeout clamped when remaining bead time < configured timeout
- Immediate failure when remaining <= 0
- 2-min validation reserve subtracted
- No clamping when BeadTimeout/BeadStartTime unset (backward compat)

---

## Implementation Tasks

### Task 1: Add config fields for token budget and failure context truncation

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `MaxFailureContextChars int` (yaml: `max_failure_context_chars`) and `MaxInputTokensPerBead int` (yaml: `max_input_tokens_per_bead`) to `ClaudeConfig`. Add `MaxInputTokensPerBead int` to `ModelTimeoutOverrides`. In `SetDefaults()`, set `MaxFailureContextChars` to 2000 and `MaxInputTokensPerBead` to 400000 when zero. Add `TokenBudgetForModel(model string) int` method on `ClaudeConfig` that returns the per-model override if non-zero, else the top-level default. Add commented config examples to `gromit.yaml`.

**Acceptance Criteria:**
- SetDefaults applies default values (2000 / 400000)
- YAML deserialization works for both top-level and per-model overrides
- TokenBudgetForModel resolves per-model override over top-level default

**Dependencies:**
- None

### Task 2: Add cumulative token fields to BeadContext

**Files:**
- Modify: `internal/runner/runtypes/types.go`

**What to Do:**
Add `CumulativeInputTokens int` and `CumulativeOutputTokens int` fields to `BeadContext` struct, after the existing retry tracking fields. Zero-value means no tokens tracked yet. No test file changes needed — these are plain data fields consumed by handler tests in Task 3.

**Acceptance Criteria:**
- Fields exist on BeadContext with correct types
- Existing tests pass unchanged (zero-value is safe)

**Dependencies:**
- None

### Task 3: Add token accumulation, budget check, and failure context truncation to escalation handler

**Files:**
- Modify: `internal/runner/escalation/handler.go`
- Test: `internal/runner/escalation/handler_test.go`

**What to Do:**

**Token accumulation:** In `ExecuteWithRetry`, after a successful or failed invocation (after line 408 where `claudeResult` is extracted), accumulate `invResult.Result.InputTokens` and `OutputTokens` from StreamStats/ProviderResult into `bc.CumulativeInputTokens` and `bc.CumulativeOutputTokens`. The tokens are available on `bc.Result.InputTokens` / `bc.Result.OutputTokens` which are populated by the invoker.

**Token budget check:** In `checkRetryBudgetBeforeAttempt`, add a check: if `h.cfg.Claude.TokenBudgetForModel(bc.Model) > 0` and `bc.CumulativeInputTokens >= budget`, return an error. In `ExecuteWithRetry`, when this specific error fires, route to `AttemptDecomposition` instead of just returning false (same pattern as bead timeout).

**Failure context truncation:** In `AnalyzeAndHandleFailure`, after setting `bc.PromptCtx.FailureContext = analysis.Suggestion` (line 297), truncate to `h.cfg.Claude.MaxFailureContextChars` if > 0. Keep the tail, prepend `[truncated]\n`.

**Acceptance Criteria:**
- Cumulative tokens accumulate across multiple invocations in retry loop
- Token budget check triggers decomposition when exceeded
- Failure context is truncated to configured limit preserving tail

**Dependencies:**
- Task 1 (config fields for TokenBudgetForModel and MaxFailureContextChars)
- Task 2 (BeadContext cumulative token fields)

### Task 4: Add adaptive invocation timeout clamping

**Files:**
- Modify: `internal/runner/execution/invoker.go`
- Test: `internal/runner/execution/invoker_test.go`

**What to Do:**
In `Execute()`, after provider selection and before creating the invocation context, compute the effective timeout. If `bc.BeadTimeout > 0` and `!bc.BeadStartTime.IsZero()`:
1. `remaining = bc.BeadTimeout - time.Since(bc.BeadStartTime)`
2. `effective = remaining - 2*time.Minute` (validation reserve)
3. If `effective <= 0`, return error "insufficient time remaining for invocation"
4. If `effective < configuredTimeout`, use `effective` as the invocation timeout

The 2-minute constant should be a package-level `const validationReserve = 2 * time.Minute`.

Currently the invoker uses `context.WithCancel(ctx)` (line 89) — the bead-level timeout is on `ctx` itself. The invocation timeout needs to be applied here as a `context.WithTimeout` wrapping the cancel context, clamped to the computed effective value.

**Acceptance Criteria:**
- Invocation timeout clamped when remaining bead time < configured timeout
- Immediate error when effective timeout <= 0
- No clamping when BeadTimeout or BeadStartTime are zero (backward compat)

**Dependencies:**
- Task 2 (BeadContext fields, specifically BeadStartTime/BeadTimeout already exist but the test setup pattern matters)

---

## Notes

- Token counts flow from `StreamStats` through `InvocationResult` into `bc.Result.InputTokens`/`OutputTokens`. The handler should accumulate from `bc.Result` after each invocation since that's already populated by the invoker's diagnostic snapshot.
- The `TruncateOutput` utility in `runtypes/truncate.go` follows a similar tail-preservation pattern — reference it for consistency but don't share code (different constants and markers).
- All three layers are independently valuable. Layer 1 (truncation) prevents prompt bloat. Layer 2 (budget) prevents runaway retries. Layer 3 (clamping) prevents starting doomed invocations. Failure of any task doesn't block the others at runtime — they're defense-in-depth.
- The validation reserve constant (2 min) should NOT be configurable. It matches `analysis_timeout` and adding config for it would be over-engineering.
