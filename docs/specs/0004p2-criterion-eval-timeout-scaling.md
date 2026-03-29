# Spec 0004p2 — Criterion Evaluation Timeout Scaling

## spec_id
0004p2-criterion-eval-timeout-scaling

## Depends on
0004p (structured output must land first so timeouts don't mask format issues)

## Vision

The criterion evaluator sends the full cumulative diff (500KB-880KB) to an LLM call per criterion with no per-criterion deadline — the accept adapter's `llmadapter.Config.Timeout` is zero, so LLM calls run unbounded. Simple criteria ("type X exists in package Y") need seconds; complex criteria ("end-to-end pipeline behavior under failure") may legitimately need several minutes with a large diff. Without per-criterion timeouts, a single slow evaluation can block the entire accept phase indefinitely, and there is no way to distinguish a legitimately slow evaluation from a hung LLM call. Remediation adds code, the diff grows monotonically, evaluation takes longer, and runs fail or hang. This was the terminal failure in 7/10 analyzed production runs.

## Summary

Add per-criterion timeout scaling for criterion evaluation based on diff size and criterion complexity. The `Evaluator.Evaluate` loop wraps each `AcceptAgent.EvaluateCriterion` call in its own `context.WithTimeout` whose duration is computed from a formula: `base + (diff_bytes / rate_constant) + complexity_bonus`. Criterion complexity is classified by keyword matching on the criterion text. Config fields control all formula constants and a hard maximum prevents runaway timeouts. The per-criterion timeout is the sole timeout mechanism for the accept phase (the adapter-level timeout is already zero).

## Goals

### Primary
- Each criterion evaluation gets its own timeout scaled to diff size and criterion complexity
- Simple criteria (existence/presence checks) get shorter timeouts than complex criteria (behavioral/end-to-end)
- Config fields control all timeout formula constants with sensible defaults

### Secondary
- A `criterion_timeout_computed` log line records the computed timeout for each criterion, enabling post-run analysis
- Per-criterion timeouts are the sole timeout mechanism for evaluation (the accept adapter's `Timeout` is already zero)

## Non-goals
- Delta-diff evaluation to reduce diff size (deferred to 0004r)
- Incremental evaluation during bead execution
- Changing the evaluation prompt template in `acceptor/prompt.go`
- Changing the `AcceptAgent` interface signature
- Timeout scaling for non-accept phases (plan, review, execute)

## Architecture

### Timeout formula

```
timeout = base + (diff_size_bytes / rate_constant) + complexity_bonus
timeout = min(timeout, hard_maximum)
timeout = max(timeout, base)
```

Default constants:
- `base`: 60s — minimum time for any criterion, regardless of diff size
- `rate_constant`: 5000 bytes/second — how many bytes of diff the LLM processes per second (conservative estimate; actual throughput varies by model)
- `complexity_bonus`: 120s — added for complex criteria
- `hard_maximum`: 600s (10 minutes) — absolute cap to prevent runaway single-criterion evaluation

Example: 500KB diff + simple criterion = 60 + (500000/5000) + 0 = 160s (~2.7 min).
Example: 800KB diff + complex criterion = 60 + (800000/5000) + 120 = 340s (~5.7 min).

### `TimeoutConfig` in `internal/next/acceptor/timeout.go` (new file)

```go
// TimeoutConfig holds the constants for per-criterion timeout scaling.
type TimeoutConfig struct {
    BaseSeconds         int `json:"base_seconds" yaml:"base_seconds"`
    RateConstant        int `json:"rate_constant" yaml:"rate_constant"`
    ComplexityBonusSecs int `json:"complexity_bonus_seconds" yaml:"complexity_bonus_seconds"`
    HardMaximumSecs     int `json:"hard_maximum_seconds" yaml:"hard_maximum_seconds"`
}

// DefaultTimeoutConfig returns production defaults.
func DefaultTimeoutConfig() TimeoutConfig {
    return TimeoutConfig{
        BaseSeconds:         60,
        RateConstant:        5000,
        ComplexityBonusSecs: 120,
        HardMaximumSecs:     600,
    }
}
```

### Complexity classifier in `internal/next/acceptor/timeout.go`

```go
// ClassifyCriterionComplexity returns "complex" or "simple" based on
// keyword analysis of the criterion text.
func ClassifyCriterionComplexity(criterion string) string
```

Classification rules:
- **Complex**: criterion text contains any of: "end-to-end", "pipeline", "integration", "behavior", "scenario", "workflow", "sequence", "across", "survive", "resume"
- **Simple**: everything else (existence checks, field presence, event emission, string containment)

The classifier is a pure function with no side effects, making it trivially testable.

### `ComputeCriterionTimeout` in `internal/next/acceptor/timeout.go`

```go
// ComputeCriterionTimeout returns the duration for evaluating a single
// criterion given the diff size in bytes and the criterion text.
func ComputeCriterionTimeout(cfg TimeoutConfig, diffSizeBytes int, criterion string) time.Duration
```

Applies the formula above. Returns a `time.Duration`.

### Changes to `Evaluator.Evaluate` in `internal/next/acceptor/evaluator.go`

The `Evaluator` gains a `TimeoutConfig` field set at construction:

```go
type Evaluator struct {
    agent      AcceptAgent
    timeoutCfg TimeoutConfig
}

func NewEvaluator(agent AcceptAgent, timeoutCfg TimeoutConfig) *Evaluator {
    return &Evaluator{agent: agent, timeoutCfg: timeoutCfg}
}
```

Inside the `for _, criterion := range input.Criteria` loop, before calling `e.agent.EvaluateCriterion`:

```go
diffSize := len(input.DiffSummary)
deadline := ComputeCriterionTimeout(e.timeoutCfg, diffSize, criterion)
criterionCtx, cancel := context.WithTimeout(ctx, deadline)
cr, err := e.agent.EvaluateCriterion(criterionCtx, prompt)
cancel()
```

The `len(input.DiffSummary)` serves as the diff size proxy. This is the same string already passed to the prompt, so its byte length is an accurate measure of the context the LLM must process.

### Accept adapter timeout status

In `cmd/gromit-next/stage_provider.go` (line ~201), the accept adapter's `llmadapter.Config` currently has no explicit `Timeout` field (zero value = no adapter-level timeout). The `LLMAdapter.Invoke` method skips `context.WithTimeout` when `Timeout == 0`, so no adapter-level deadline exists today. The per-criterion `context.WithTimeout` in `Evaluator.Evaluate` will be the sole timeout mechanism for accept-phase LLM calls. No change needed in `stage_provider.go` for timeout removal — Timeout is already zero.

### Wiring in `cmd/gromit-next/stage_provider.go`

The `NewEvaluator` call (line ~207) gains the `TimeoutConfig` argument. The config values can be hardcoded as `DefaultTimeoutConfig()` initially, with YAML config wiring deferred to a future spec if needed.

**Files in scope:**
- `internal/next/acceptor/timeout.go` (new — formula, classifier, config)
- `internal/next/acceptor/timeout_test.go` (new — unit tests)
- `internal/next/acceptor/evaluator.go` (modified — per-criterion timeout wrapping)
- `internal/next/acceptor/evaluator_test.go` (modified — updated constructor calls)
- `cmd/gromit-next/stage_provider.go` (modified — pass TimeoutConfig to NewEvaluator)

All other files are out of scope.

## Acceptance Criteria

1. `ComputeCriterionTimeout` with a 500KB diff and a simple criterion (e.g., "type X exists") returns a duration between 100s and 200s (base + diff/rate, no complexity bonus).

2. `ComputeCriterionTimeout` with a 500KB diff and a complex criterion (containing "end-to-end") returns a duration greater than the simple-criterion duration by exactly `ComplexityBonusSecs`.

3. `ComputeCriterionTimeout` never returns a duration exceeding `HardMaximumSecs`, even with a 3MB diff and a complex criterion (uncapped value: 780s, capped to 600s).

4. `ComputeCriterionTimeout` never returns a duration below `BaseSeconds`, even with an empty diff and a simple criterion.

5. `ClassifyCriterionComplexity` returns `"complex"` for criteria containing behavioral keywords ("end-to-end", "pipeline", "integration", "behavior", "scenario") and `"simple"` for criteria like "type X exists in package Y".

6. `Evaluator.Evaluate` wraps each `EvaluateCriterion` call in a per-criterion `context.WithTimeout` whose deadline is computed by `ComputeCriterionTimeout` using `len(input.DiffSummary)` as the diff size.

7. When a per-criterion timeout fires, `Evaluator.Evaluate` returns an error that includes the criterion text and the word "timeout" or "deadline exceeded".

8. All existing tests in `internal/next/acceptor/...` and `internal/next/specloop/stages/...` continue to pass (constructor signature change is propagated).

## Scenarios

### Scenario: Simple criterion with moderate diff finishes within scaled timeout
**Given:** An `EvaluateInput` with a 500KB `DiffSummary` and one criterion "RunState.ReviewThrashCounts field exists"
**When:** `Evaluate` is called with `DefaultTimeoutConfig()`
**Then:** The per-criterion context has a timeout of 160s (60 + 500000/5000 + 0); the evaluation completes before the deadline; the result contains one `CriterionResult` with status "pass"

### Scenario: Complex criterion with large diff gets complexity bonus
**Given:** An `EvaluateInput` with an 800KB `DiffSummary` and one criterion "end-to-end pipeline survives resume after crash"
**When:** `Evaluate` is called with `DefaultTimeoutConfig()`
**Then:** The per-criterion context has a timeout of 340s (60 + 800000/5000 + 120); `ClassifyCriterionComplexity` returns "complex" for this criterion

### Scenario: Hard maximum caps runaway timeout
**Given:** A 3MB `DiffSummary` and one criterion "integration test passes across all scenarios"
**When:** `ComputeCriterionTimeout` is called with `DefaultTimeoutConfig()`
**Then:** The uncapped value would be 60 + 3000000/5000 + 120 = 780s; the computed timeout is capped at 600s (the hard maximum)

### Scenario: Per-criterion timeout fires on slow criterion
**Given:** An `EvaluateInput` with two criteria: one simple (fast) and one complex (slow, exceeds its per-criterion deadline)
**When:** `Evaluate` is called and the complex criterion's LLM call exceeds its computed deadline
**Then:** The first criterion completes successfully; the second criterion's `EvaluateCriterion` returns a context deadline exceeded error; `Evaluate` returns an error containing the timed-out criterion's text and "deadline exceeded"; the first criterion's timeout does not affect the second criterion (each gets its own `context.WithTimeout`)

## Validation

```bash
go test ./internal/next/acceptor/... -run TestComputeCriterionTimeout
go test ./internal/next/acceptor/... -run TestClassifyCriterion
go test ./internal/next/acceptor/... -run TestEvaluate
go test ./internal/next/specloop/stages/... -run TestAccept
go vet ./internal/next/acceptor/...
go vet ./cmd/gromit-next/...
```
