---
id: gemini-provider-adapter
source_ideas: []
created: 2026-02-20
---

# Gemini Provider Adapter

## Specification

Implement a `GeminiProvider` that satisfies the `provider.Provider` interface, enabling Gemini as a first-class provider in Gromit's multi-provider routing system alongside Claude and Codex.

This spec depends on `gemini-cli-spike` — the exact invocation format, JSONL schema, and error patterns come from spike findings.

### Provider Implementation

Add `internal/provider/gemini.go`:

```go
type GeminiProvider struct {
    binary     string            // "gemini" (from config)
    flags      []string          // global flags (permissions, etc.)
    tierModels map[string]string // tier → model mapping from config
    dir        string            // working directory
    costInput  float64           // cost per 1K input tokens
    costOutput float64           // cost per 1K output tokens
    runFn      func(...)         // injectable subprocess function
}
```

**Interface methods:**

| Method | Implementation |
|--------|---------------|
| `Name()` | Returns `"gemini"` |
| `ModelForTier(tier)` | Lookup from `tierModels` map; empty string if tier not configured |
| `Run(ctx, prompt, tier)` | Non-streaming: launch `gemini -m <model> --output-format json` with prompt delivered via stdin by default. Parse JSON result. |
| `StreamRun(ctx, prompt, tier, output, handler, onToolCall)` | Streaming: launch `gemini -m <model> --output-format stream-json` with prompt delivered via stdin by default. Parse JSONL events. |
| `RunValidation(ctx, commands, tier, workDir)` | Validation prompt with numbered command block (same pattern as Claude/Codex) |
| `IsUsageLimitError(result, err)` | Pattern match on error strings from spike findings |
| `IsValidationPassed(result)` | Check for `VALIDATION_PASSED` marker in output |
| `IsScopeTooLarge(result)` | Check for `SCOPE_TOO_LARGE:` marker in output |

**Note**: Spike findings (`.gromit/plans/gemini-cli-spike-findings.md`, updated 2026-02-23 with live runs) recommend stdin-first delivery for provider runs, with `-p` retained as fallback for short prompts/diagnostics. Live `json`, `stream-json`, and model-valid/invalid behavior were observed; auth/rate-limit/transport failure signatures remain partially unverified and should stay configurable.

### Helper Functions

Add `internal/provider/gemini_helpers.go`:

- `classifyGeminiError(stderr string) string` — returns `FailureCategoryAuth`, `FailureCategoryRateLimited`, `FailureCategoryTransportDisconnect`, or `FailureCategoryOther` based on patterns from spike findings (including setup failures like `command not found: gemini` mapped to `other` until a dedicated startup/setup category is introduced).
- `parseGeminiStreamEvent(line []byte) (eventType string, data map[string]interface{}, err error)` — parse a single JSONL line.
- `parseGeminiJSONResult(output []byte) (*Result, error)` — parse the single-JSON response format.
- `extractGeminiText(events)` — accumulate assistant text from message events.
- `extractGeminiUsage(resultEvent)` — extract token counts and calculate cost.

### Transient Failure Retry

Follow the Codex pattern: bounded retry for transient failures within `Run()` and `StreamRun()`:
- Max 2 retries for `transport_disconnect` or `rate_limited` categories.
- Backoff: 250ms → 750ms → 1500ms.
- Non-transient failures (auth, environment/setup, other) are not retried.

### Constructor Wiring

In `internal/runner/constructor.go`:
- Add `buildGeminiProvider(cfg)` that reads `providers.gemini` from config.
- Register in `buildProvidersFromConfig()` when the `gemini` key exists.
- No hardcoded tier→model defaults — require explicit configuration.

### Configuration

```yaml
providers:
  gemini:
    binary: gemini
    flags: []  # permission flags from spike findings
    models:
      high: gemini-3.1-pro
      medium: gemini-3-flash
      low: gemini-3-flash
    cost_per_1k_input: 0.0005   # Flash pricing; override per deployment
    cost_per_1k_output: 0.003
    reasoning_effort:           # if Gemini supports this
      high: high
      medium: medium
      low: low
```

Router config adds Gemini to the ratio:
```yaml
routing:
  ratio:
    claude: 1
    openai: 94
    gemini: 5
```

### Agent Preset Update

Update `internal/agent/resolve.go` `resolveByName` to align with spike findings: default Gemini preset to `Stdin`, keep prompt-flag delivery available through config override, and preserve compatibility for environments where local CLI invocation differs.

### Cost Tracking

- Configure `cost_per_1k_input` and `cost_per_1k_output` in the provider config section.
- If the spike shows Gemini reports cost directly in result events, prefer provider-reported cost over calculated cost.
- Token counts (`InputTokens`, `OutputTokens`, `CachedInputTokens`) populate from stream result events or JSON stats object.
- Cost flows into `Result.CostUSD` and propagates through iteration logging unchanged.

### Legacy Model Mapping

Add Gemini models to `legacyModelToTier` in `provider.go`:
```go
"gemini-3.1-pro": TierHigh,
"gemini-3-pro":   TierHigh,
"gemini-3-flash": TierMedium,  // or TierLow depending on usage
```

## Acceptance Criteria

- `GeminiProvider` implements `provider.Provider` with compile-time assertion.
- `Run()` executes Gemini CLI and parses JSON result with token/cost data.
- `StreamRun()` parses JSONL events, accumulates text, reports tokens/cost.
- `RunValidation()` sends validation commands and detects pass/fail.
- Error classification correctly categorizes environment/setup failures now, and supports auth/rate-limit/transport categories with conservative matching until live signatures are captured.
- Transient retry respects backoff bounds and max retries.
- Constructor creates `GeminiProvider` from `providers.gemini` config when present.
- Router selects Gemini based on configured ratio.
- Unit tests cover all Provider methods with injected `runFn`.
- Contract tests in `test/contracts/gemini_contract_test.go` verify invocation format.
- Fixtures in `test/fixtures/gemini_stream_success.jsonl`, `gemini_stream_failure.jsonl`, `gemini_success.txt` from spike captures.
- Agent preset in `resolve.go` matches verified invocation format.
- Existing Claude and Codex behavior is unchanged.

## Execution Order

- Sequence position: 2
- Dependencies: `gemini-cli-spike`
- Unblocks: Gemini routing experiments, three-provider load balancing

## Decisions

1. **YAML-configured tier→model mapping** — no hardcoded defaults. The Gemini model landscape moves fast; config-driven is future-proof.

2. **Follow CodexProvider patterns** — transient retry, error classification, FnField testing. Reduces cognitive overhead and maintains consistency.

3. **Config-driven cost** — `cost_per_1k_input`/`cost_per_1k_output` per provider. Prefer provider-reported cost if available.

4. **Start at routing ratio 0%** — add to config but don't route traffic until manually enabled. Safe rollout.

5. **Partial-verification rollout** — spike findings are sufficient to implement parser scaffolding and wiring, but production confidence requires a rerun with installed/authenticated Gemini CLI to lock final schemas and classifier signatures.

## Research & Context

### Provider Interface (8 methods)

```go
type Provider interface {
    Name() string
    ModelForTier(tier string) string
    Run(ctx, prompt, tier) (*Result, error)
    StreamRun(ctx, prompt, tier, output, handler, onToolCall) (*Result, error)
    RunValidation(ctx, commands, tier, workDir) (*Result, error)
    IsUsageLimitError(result, err) bool
    IsValidationPassed(result) bool
    IsScopeTooLarge(result) (bool, string)
}
```

### Existing Implementations

- `ClaudeProvider` (`claude.go`): wraps `internal/claude/Client`, stream-json events
- `CodexProvider` (`codex.go`): direct binary execution, `--json` JSONL events, transient retry

### Files to Change

| File | Change |
|------|--------|
| `internal/provider/gemini.go` | New: GeminiProvider struct + Provider methods |
| `internal/provider/gemini_helpers.go` | New: error classification, event parsing |
| `internal/provider/provider.go` | Add Gemini models to `legacyModelToTier` |
| `internal/runner/constructor.go` | Add `buildGeminiProvider()`, register in `buildProvidersFromConfig()` |
| `internal/agent/resolve.go` | Update gemini preset invocation format |
| `internal/config/config.go` | Ensure providers config supports gemini key (may already work) |
| `test/contracts/gemini_contract_test.go` | New: contract tests |
| `test/fixtures/gemini_*.jsonl` | New: streaming fixtures from spike captures |
| `test/fixtures/gemini_*.txt` | New: text fixtures from spike captures |

### Out of Scope

- Gemini-specific model selection heuristics (e.g., routing certain bead types to Gemini).
- Performance benchmarking Gemini vs Claude/Codex.
- Gemini-specific prompt optimization or template changes.
- Multi-turn conversation support (Gromit uses fresh context per iteration).
