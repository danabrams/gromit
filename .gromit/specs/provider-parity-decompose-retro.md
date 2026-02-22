---
id: provider-parity-decompose-retro
source_ideas: []
created: 2026-02-22
---

# Provider Parity for Decompose and Retro Commands

## Specification

`gromit decompose` and `gromit retro` bypass the provider routing system entirely — they always create a `claude.NewClient()` directly, ignoring the `providers:` config in `gromit.yaml`. This spec wires both commands into the existing `provider.Router` so any configured provider can handle decomposition and retrospective work, with automatic fallback on usage-limit errors.

### Why

When a user's Claude quota is exhausted, `gromit decompose` and `gromit retro` halt completely even if another provider (e.g., Codex) is available and configured. This contradicts the multi-provider-routing spec's goal: any provider can do any work. These are the last two fully-hardcoded commands.

### Decompose Command

**Current state** (`cmd/gromit/decompose.go`, `decomposeSinglePlanInCurrentDir()`):
- Creates `claude.NewClient()` directly
- Wraps it in a `claudeClientAdapter` implementing `pipeline.ClaudeClient`
- Passes to `pipeline.Deps{ClaudeClient: ...}`
- Model hardcoded to `"sonnet"` in `internal/pipeline/decompose.go`

**Target state:**
- Check `cfg.HasProviders()` first
- If providers configured: build a `provider.Router` from config (reuse `buildProvidersFromConfig()` or extract a shared helper), wrap it in an adapter implementing `pipeline.ClaudeClient`
- If no providers: fall back to `claude.NewClient()` as today (backward compat)
- Replace hardcoded `"sonnet"` model with `provider.TierMedium` tier selection — the router resolves to the appropriate model for whichever provider is selected
- On usage-limit error from the selected provider: mark unavailable, retry with fallback provider

The adapter wrapping the router needs to implement `pipeline.ClaudeClient.Run(prompt, model) (*ClaudeRunResult, error)`. It should:
1. Call `router.Select("decompose", tier)` to pick a provider
2. Call `provider.Run(ctx, prompt, tier)`
3. Convert `provider.Result` → `pipeline.ClaudeRunResult`
4. If `IsUsageLimitError`, mark unavailable and retry once with next provider

### Retro Command

**Current state** (`cmd/gromit/main.go`, retro subcommand ~line 277):
- Creates `claude.NewClient()` with opus timeout
- Builds a `map[string]string` tier-to-model map
- Creates `provider.NewClaudeProvider()` directly
- Passes to `retro.NewRetroWithProviderAndBudget()`

**Target state:**
- Check `cfg.HasProviders()` first
- If providers configured: build a `provider.Router` from config, wrap it in an adapter implementing `retro.ProviderRunner`
- If no providers: fall back to `NewClaudeProvider()` as today
- The retro package itself is already provider-agnostic (takes `ProviderRunner` interface) — only the command wiring needs to change
- On usage-limit error: mark unavailable, retry with fallback provider

The adapter wrapping the router needs to implement `retro.ProviderRunner`. It should:
1. Call `router.Select("retro", tier)` to pick a provider
2. Delegate to that provider's `Run()` or `StreamRun()`
3. Handle usage-limit fallback

### Shared Router Construction

Both commands need to build a `provider.Router` from config. The `gromit run` path does this in `internal/runner/constructor.go` via `buildRouterAndLearningsProvider()`. Rather than duplicating that logic, extract a shared helper:

```go
// internal/provider/build.go or similar
func BuildRouterFromConfig(cfg *config.Config) (*Router, error)
```

This helper:
1. Checks `cfg.HasProviders()` — if false, returns a single-provider router wrapping a Claude provider from `cfg.Claude`
2. If true, iterates `cfg.Providers`, creates each provider, builds a `Router` with routing config
3. Used by `gromit run`, `gromit decompose`, `gromit retro`, `gromit review --non-interactive`, and `gromit verify-spec`

This also fixes the partially-hardcoded commands (`review --non-interactive`, `verify-spec`) since they already have the right if/else structure — they just need to call the shared helper instead of duplicating provider construction.

### Fallback Behavior

The fallback pattern is the same across all commands:

1. `router.Select(phase, tier)` picks the best available provider
2. Invoke the provider
3. If `provider.IsUsageLimitError(result, err)`: call `router.MarkUnavailable(provider.Name())`, then `router.Select(phase, tier)` again for the fallback
4. If all providers are unavailable, return a clear error: `"all providers exhausted (usage limits reached)"`

This pattern already exists in the `gromit run` path. The adapters for `decompose` and `retro` should implement the same retry-once-on-limit logic.

## Acceptance Criteria

- `gromit decompose` uses the `providers:` config from `gromit.yaml` when present
- `gromit decompose` falls back to `claude.NewClient()` when no `providers:` section exists
- `gromit decompose` automatically retries with a fallback provider on usage-limit errors
- `gromit retro` uses the `providers:` config from `gromit.yaml` when present
- `gromit retro` falls back to `NewClaudeProvider()` when no `providers:` section exists
- `gromit retro` automatically retries with a fallback provider on usage-limit errors
- A shared `BuildRouterFromConfig()` helper eliminates duplicated provider construction across commands
- Existing behavior is identical when no `providers:` section is configured (full backward compatibility)
- The hardcoded `"sonnet"` model in decompose is replaced with tier-based selection (`TierMedium`)

## Decisions

1. **Adapter pattern, not interface change.** The `pipeline.ClaudeClient` and `retro.ProviderRunner` interfaces stay as-is for now. Adapters wrap the router to implement these interfaces. The separate rename spec will clean up the naming later.

2. **Shared helper, not duplicated construction.** One `BuildRouterFromConfig()` function used by all commands. This prevents drift where one command handles providers correctly and another doesn't.

3. **Retry once, not indefinitely.** On usage-limit error, try the next available provider once. If that also fails, surface the error. This prevents infinite retry loops when all providers are down.

4. **Decompose uses TierMedium.** The current hardcoded `"sonnet"` maps to medium tier. This preserves the intent (decomposition doesn't need the most expensive model) while making it provider-agnostic.

5. **Retro keeps TierHigh default.** Retro currently uses opus, which maps to high tier. Retrospectives benefit from the most capable model for synthesis.

## Research & Context

### Current Adapter Patterns

Several adapter types already exist in `cmd/gromit/` and `internal/runner/constructor_adapters.go`:
- `claudeClientAdapter` — wraps `claude.Client` as `pipeline.ClaudeClient`
- `providerRouterClientAdapter` — wraps `Router` as `pipeline.ReviewInvoker` (in review.go)
- `reviewInvokerAdapter` — wraps `Router` as `review.Invoker`

The decompose adapter follows the same pattern as `providerRouterClientAdapter` but implements `pipeline.ClaudeClient` instead of `pipeline.ReviewInvoker`.

### Related Specs

- `multi-provider-routing` — the parent spec that established the provider/router system. This spec fills gaps it left in decompose and retro.
- `codex-streaming-parity` — ensures CodexProvider has feature parity for streaming. Decompose and retro use `Run()` (not `StreamRun()`), so streaming parity is not a blocker.
