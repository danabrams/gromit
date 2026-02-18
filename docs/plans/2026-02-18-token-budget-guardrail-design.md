# Token Budget Guardrail Design

**Problem:** Per-bead timeouts caused by high-token retry churn. On `gromit-evne`, a sonnet-tier bead consumed ~964k input tokens across retries, hitting the 40-minute bead timeout. The bead was closeable — retries burned the budget before it could finish.

**Root cause:** No token budget tracking across retries within a bead. The system tracks wall-clock time and attempt counts, but nothing prevents retries from snowballing token usage through increasingly large failure contexts.

## Three-Layer Defense

### Layer 1: Prompt Compression (Preventive)

Cap failure context injected into retry prompts at a configurable character limit (default 2000). Applied on every retry, unconditionally.

**Where:** `escalation/handler.go` in `AnalyzeAndHandleFailure`, before setting `bc.PromptCtx.FailureContext`.

Truncation preserves the tail (most recent, most relevant) and prepends a `[truncated]` marker.

**Config:**
```yaml
claude:
  max_failure_context_chars: 2000
```

### Layer 2: Token Budget (Pre-Invocation Gate)

Track cumulative input tokens per bead. Before each invocation, check against a per-model cap. When exceeded, skip retry and decompose immediately.

**Where:** `escalation/handler.go` — accumulate tokens in `ExecuteWithRetry` after each invocation, check in `checkRetryBudgetBeforeAttempt`.

**New fields on `BeadContext`:**
```go
CumulativeInputTokens  int
CumulativeOutputTokens int
```

**Budget check:**
```go
if bc.CumulativeInputTokens >= tokenBudget {
    return fmt.Errorf("token budget exceeded: %d/%d input tokens across %d attempts",
        bc.CumulativeInputTokens, tokenBudget, bc.AttemptsThisBead)
}
```

**When exceeded:** Error flows through `ExecuteWithRetry` to `AttemptDecomposition`. No escalation — token churn is a scope signal, not a capability signal.

**Config (extends existing `ModelTimeoutOverrides`):**
```yaml
claude:
  max_input_tokens_per_bead: 400000
  model_timeouts:
    sonnet:
      max_input_tokens_per_bead: 500000
    haiku:
      max_input_tokens_per_bead: 200000
```

### Layer 3: Adaptive Per-Invocation Timeout (Safety Net)

Clamp the invocation timeout to the remaining bead budget minus a validation reserve (2 minutes). Prevents starting doomed invocations near the bead deadline.

**Where:** `execution/invoker.go` in `Execute`, when constructing the invocation context.

```
remainingBead = beadTimeout - elapsed
validationReserve = 2 minutes
effectiveTimeout = min(configuredTimeout, remainingBead - validationReserve)

if effectiveTimeout <= 0:
    return error("insufficient time remaining for another invocation")
```

No new config. Derived from existing `BeadStartTime` and `BeadTimeout` on `BeadContext`.

## How the Layers Interact

```
Attempt 1: runs, consumes ~200k tokens, fails
  → Layer 1: truncate failure context to 2000 chars
Attempt 2: runs, cumulative tokens ~400k, fails
  → Layer 2: token budget check fires (400k >= 400k cap)
  → AttemptDecomposition → bead split into sub-tasks
Total elapsed: ~25 minutes instead of 40. No timeout.
```

If tokens stay under budget but time runs low, Layer 3 refuses to start an invocation that cannot finish.

## Changes Summary

| File | Change |
|------|--------|
| `runtypes/types.go` | Add `CumulativeInputTokens`, `CumulativeOutputTokens` to `BeadContext` |
| `config/config.go` | Add `MaxInputTokensPerBead` to `ClaudeConfig` and `ModelTimeoutOverrides`; add `MaxFailureContextChars` to `ClaudeConfig` |
| `escalation/handler.go` | Accumulate tokens in `ExecuteWithRetry`; check token budget in `checkRetryBudgetBeforeAttempt`; truncate failure context in `AnalyzeAndHandleFailure`; route token budget errors to decomposition |
| `execution/invoker.go` | Clamp invocation timeout to remaining bead budget minus validation reserve |

## Design Decisions

- **Decompose immediately on token budget exhaustion.** Token churn signals scope problems, not model capability. Escalation already has its own path through `AnalyzeAndHandleFailure`.
- **Absolute token cap, not velocity-based.** Simpler to configure, reason about, and test. Per-model overrides follow the existing `model_timeouts` pattern.
- **Truncate, don't summarize.** Deterministic, zero-cost, no extra LLM call. The tail of failure output is the most actionable part.
- **Validation reserve is a constant.** 2 minutes matches `analysis_timeout` and is enough for the fast validation gate.
