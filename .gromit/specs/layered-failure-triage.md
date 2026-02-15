---
id: layered-failure-triage
source_ideas: []
created: 2026-02-15
---

# Layered Failure Triage

## Specification

When a build invocation fails, Gromit currently sends every failure through an LLM analyzer call — even transport disconnects, missing binaries, and rate limits. This wastes tokens on failures that a simple pattern match could classify. It also fragments the failure taxonomy: provider-level errors (`transport_disconnect`, `rate_limited`, `auth`) live in `provider.Result.FailureCategory`, while code-level errors (`syntax`, `logic`, `environment`, etc.) live in `analyzer.Analysis.Category`. Downstream decision-making (retry, escalate, decompose, stop) must consult both systems.

This feature adds a fast, programmatic triage step that classifies every failure into one of four layers before deciding the response path. Only failures in the `code` layer proceed to the LLM analyzer. The other three layers are handled directly — no tokens spent.

### Failure Layers

| Layer | What broke | Detection | Response |
|---|---|---|---|
| `provider_transport` | The connection to the LLM provider failed | `provider.Result.FailureCategory` field | Retry with backoff (same model) |
| `environment` | The local environment is misconfigured | Stderr pattern matching against known error strings | Fail fast — human must fix |
| `orchestration` | Gromit sent bad instructions or the bead is malformed | Structural checks on prompt and bead metadata | Skip bead or decompose |
| `code` | The generated code has bugs | Default (nothing above matched) | Existing LLM analyzer + retry/escalate/decompose |

Triage is a waterfall: check `provider_transport` first (cheapest signal), then `environment`, then `orchestration`, then fall through to `code`.

### Detection Rules

**Provider Transport** — read directly from `provider.Result.FailureCategory`:

- `transport_disconnect` → sub-category `disconnect`, retryable
- `rate_limited` → sub-category `rate_limit`, retryable
- `auth` → sub-category `auth`, not retryable

**Environment** — regex patterns checked against `provider.Result.Stderr` (falling back to `Output` when stderr is empty):

- `exec: .+: executable file not found` → `missing_tool`
- `go: go\.mod requires go >=` → `version_mismatch`
- `no space left on device` → `resource_exhausted`
- `permission denied` → `permission`

Start with these four patterns. Log unclassified failures so coverage can expand over time.

**Orchestration** — structural checks on existing data:

- `provider.IsScopeTooLarge()` returns true → `scope_too_large`
- Build prompt is empty → `bad_prompt`
- Bead has no description and no acceptance criteria → `bad_bead`

**Code** — the default when nothing above matches. Falls through to the existing `AnalyzeAndHandleFailure()` path.

### Response Strategies

**Provider Transport:**

- `disconnect` and `rate_limit`: retry, counting against the bead's retry budget. The existing retry/escalation counters apply — transport retries are not special-cased.
- `auth`: fail immediately with a clear message ("Authentication failed — check API key / credentials").

**Environment:**

- All sub-categories: fail immediately. Set `bc.Result.Error` with an actionable message (e.g., "Environment error: `go` not found in PATH"). No retry, no escalation — a stronger model will not install Go. The loop skips this bead and continues.

**Orchestration:**

- `scope_too_large`: attempt decomposition via the existing `AttemptDecomposition()` path.
- `bad_prompt` / `bad_bead`: fail immediately with a descriptive error. These indicate a Gromit bug or bad input data.

**Code:**

- Unchanged. The existing `AnalyzeAndHandleFailure()` → LLM analysis → retry/escalate/decompose chain handles it.

### Where Triage Fits

The triage step inserts into `ExecuteWithRetry` (in `escalation/handler.go`) between the partial-progress display and the LLM analyzer call:

```
invokeFn() returns failure
  → timeout handling (stall/invocation/bead)       [unchanged]
  → checkRetryBudgetAfterFailure()                 [unchanged]
  → showPartialProgress()                          [unchanged]
  → Triage(invResult, providerResult)               [NEW]
    → provider_transport? → handleProviderTransport()
    → environment?        → fail fast, return false
    → orchestration?      → skip/decompose, return false
    → code?               → fall through
  → AnalyzeAndHandleFailure()                      [only for code layer]
```

### Types

Add to the `escalation` package:

```go
type FailureLayer string

const (
    LayerProviderTransport FailureLayer = "provider_transport"
    LayerEnvironment       FailureLayer = "environment"
    LayerOrchestration     FailureLayer = "orchestration"
    LayerCode              FailureLayer = "code"
)

type TriageResult struct {
    Layer       FailureLayer
    SubCategory string   // e.g. "rate_limit", "missing_tool", "scope_too_large"
    Detail      string   // human-readable explanation
    Retryable   bool     // whether this failure can be retried
}
```

### Observability

Add two fields to `runtypes.IterationResult`:

```go
FailureLayer  string // "provider_transport", "environment", "orchestration", "code", ""
FailureSubCat string // e.g. "rate_limit", "missing_tool", "scope_too_large", "syntax"
```

When `FailureLayer` is anything other than `code` or empty, the LLM analyzer was not invoked. The JSONL iteration logs gain clear signal about when triage saved an LLM call.

## Acceptance Criteria

- A `Triage()` function in the `escalation` package classifies failures into one of four layers using only programmatic signals — no LLM calls
- `ExecuteWithRetry` calls `Triage()` before `AnalyzeAndHandleFailure()` and short-circuits for non-code layers
- Provider transport failures (`transport_disconnect`, `rate_limited`) retry using the existing retry budget; `auth` failures fail immediately
- Environment failures (missing tool, version mismatch, disk full, permission denied) fail fast with actionable error messages
- Orchestration failures (`scope_too_large`) trigger decomposition; `bad_prompt` / `bad_bead` fail immediately
- Only `code` layer failures invoke the LLM analyzer
- `IterationResult` includes `FailureLayer` and `FailureSubCat` fields for logging and diagnostics
- Table-driven unit tests cover each layer with representative `provider.Result` inputs
- Integration tests in `escalation/handler_test.go` verify that the LLM analyzer is not called for non-code failures

## Decisions

1. **Programmatic triage, not LLM triage.** The layer classification uses exit codes, `FailureCategory` fields, and stderr pattern matching — never an LLM call. This makes triage free, fast, and deterministic. The LLM analyzer is reserved for the one case where semantic understanding matters: classifying code-level failures.

2. **Triage lives in the `escalation` package.** It's a control-flow decision (what to do next), not a standalone classification service. Adding it to the escalation handler keeps the retry/escalate/triage logic co-located. A separate package would add indirection without adding clarity.

3. **Conservative environment patterns.** Start with four high-confidence patterns (missing binary, Go version mismatch, disk full, permission denied). Log unclassified failures to stderr so we can expand coverage based on real data. False positives (misclassifying a code error as environment) are worse than false negatives (sending an environment error to the LLM analyzer), because false positives skip the LLM entirely.

4. **Transport retries share the bead retry budget.** Transport retries count against `RetriesThisModel` and `TotalRetriesThisBead`, the same as code-level retries. A separate budget would risk unbounded retries. If transport issues persist, the bead fails and the loop moves on.

5. **Existing analyzer categories become code sub-categories.** The seven existing categories (`syntax`, `logic`, `environment`, `missing_context`, `unclear_spec`, `test_flake`, `task_too_complex`) remain unchanged. They become the sub-categories of the `code` layer. The analyzer's `environment` category (for cases where the LLM detects an environment issue in the code output) is distinct from the triage-level `environment` layer (detected programmatically before the LLM runs).

## Research & Context

### Current Failure Handling

- `escalation/handler.go` — `ExecuteWithRetry()` runs the retry loop. On failure: timeout handling → budget check → partial progress → `AnalyzeAndHandleFailure()`. The analyzer LLM call happens inside `AnalyzeAndHandleFailure()` for every non-timeout failure.
- `internal/analyzer/analyzer.go` — `Analyze()` invokes a high-tier model with the failure output and parses structured JSON. Seven categories. Falls back to heuristic parsing if JSON extraction fails.
- `internal/provider/provider.go` — `Result.FailureCategory` holds provider-level classification (`transport_disconnect`, `rate_limited`, `auth`, `other`). This field is set by the provider but currently unused by the escalation handler.

### What Changes

The `provider.Result.FailureCategory` field, which is already populated but ignored by the escalation handler, becomes the primary signal for the `provider_transport` layer. The triage function reads it before deciding whether to invoke the LLM analyzer.

### Related Specs

- `usage-limit-detection` — Defines `IsUsageLimitError()` for detecting rate/usage limits. The triage layer subsumes this for the `provider_transport` case: if `FailureCategory == "rate_limited"`, triage handles it directly rather than routing through the analyzer.
- `multi-provider-routing` — Defines the `Provider` interface and router. Triage operates downstream of the router — after a provider has been selected and invoked. The router's `MarkUnavailable()` mechanism complements triage's `rate_limit` handling.
