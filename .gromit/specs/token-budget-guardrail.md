---
id: token-budget-guardrail
epic: run-loop-reliability
---

# Spec: Token Budget Guardrail

**Problem:** Per-bead timeouts caused by high-token retry churn. On `gromit-evne`, a sonnet-tier bead consumed ~964k input tokens across retries, hitting the 40-minute bead timeout. The bead was closeable — retries burned the budget before it could finish.

**Root cause:** No token budget tracking across retries within a bead. Wall-clock and attempt-count limits exist, but nothing prevents retries from snowballing token usage through increasingly large failure contexts.

**Investigation report:** `.gromit/reports/debug-20260216-014254.md`

## Three-Layer Defense

### Layer 1: Prompt Compression (Preventive)

Truncate failure context injected into retry prompts to a configurable character limit (default 2000). Preserve the tail (most recent, most actionable). Prepend `[truncated]` marker when trimming occurs. Applied on every retry, unconditionally.

Config: `claude.max_failure_context_chars: 2000`

### Layer 2: Token Budget (Pre-Invocation Gate)

Track cumulative input tokens per bead across all invocations. Before each invocation, check against a per-model cap. When exceeded, decompose immediately — token churn is a scope signal, not a capability signal.

New fields on `BeadContext`: `CumulativeInputTokens`, `CumulativeOutputTokens`.

Config (extends existing `model_timeouts` pattern):
```yaml
claude:
  max_input_tokens_per_bead: 400000
  model_timeouts:
    sonnet:
      max_input_tokens_per_bead: 500000
    haiku:
      max_input_tokens_per_bead: 200000
```

When exceeded, error flows to `AttemptDecomposition` — same terminal recovery path as bead timeout.

### Layer 3: Adaptive Per-Invocation Timeout (Safety Net)

Clamp invocation timeout to remaining bead budget minus a 2-minute validation reserve. Prevents starting doomed invocations near the bead deadline. No new config — derived from existing `BeadStartTime` and `BeadTimeout`.

If effective timeout <= 0, fail with "insufficient time remaining" error, which flows to decomposition.

## Files Changed

- `runtypes/types.go` — add `CumulativeInputTokens`, `CumulativeOutputTokens` to `BeadContext`
- `config/config.go` — add `MaxInputTokensPerBead` to `ClaudeConfig` and `ModelTimeoutOverrides`; add `MaxFailureContextChars` to `ClaudeConfig`
- `escalation/handler.go` — accumulate tokens after each invocation in `ExecuteWithRetry`; check token budget in `checkRetryBudgetBeforeAttempt`; truncate failure context in `AnalyzeAndHandleFailure`; route token budget errors to decomposition
- `execution/invoker.go` — clamp invocation timeout to remaining bead budget minus validation reserve

## Design Decisions

- **Decompose immediately on token budget exhaustion.** Escalation has its own path through `AnalyzeAndHandleFailure`. Token churn means the task is too large, not that the model is too weak.
- **Absolute token cap, not velocity-based.** Simpler to configure and test. Per-model overrides follow existing `model_timeouts` pattern.
- **Truncate, don't summarize.** Deterministic, zero-cost, no extra LLM call.
- **Validation reserve is a constant (2 min).** Matches `analysis_timeout` and covers the fast validation gate.
