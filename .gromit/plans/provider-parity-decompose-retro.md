---
created: 2026-02-22T00:00:00Z
decomposed: true
decomposed_at: "2026-02-22T19:25:32Z"
id: provider-parity-decompose-retro
source_spec: provider-parity-decompose-retro
---

# Provider Parity for Decompose and Retro — Implementation Plan

**Goal:** Wire `gromit decompose` and `gromit retro` into the existing provider routing system so any configured provider can handle them, with automatic fallback on usage-limit errors.

**Architecture:** Extract a shared `BuildRouterFromConfig()` helper into `internal/provider/build.go`. Wire decompose through the existing `providerRouterClientAdapter` (which already implements `pipeline.ClaudeClient` with retry logic). Wire retro through a new `retroRouterAdapter` implementing `retro.ProviderRunner`. Replace hardcoded `"sonnet"` in decompose with a configurable tier (default `medium`) read from `gromit.yaml`.

**Tech Stack:** Go, existing provider/router/adapter patterns

**Spec:** `.gromit/specs/provider-parity-decompose-retro.md`

---

## Architecture

### Shared Router Construction

A new `BuildRouterFromConfig()` in `internal/provider/build.go` eliminates 4 duplicated copies of provider construction logic. It:
1. Checks `cfg.HasProviders()` — if false, creates a single-provider Claude router (backward compat)
2. If true, iterates `cfg.Providers`, creates each provider, builds a `Router` with routing config
3. Accepts an optional `StateFile` for persistent provider counts (runner passes one; decompose/retro/review pass nil)

A lower-level `BuildProvidersFromConfig()` is also exported for callers that need the providers map directly (e.g., constructor.go needs it for learnings provider selection and cost defs).

### Decompose Wiring

A new `DecomposeConfig` struct in `config_types.go` adds a `Tier` field (default `"medium"`) configurable via `gromit.yaml`:
```yaml
decompose:
  tier: medium  # low, medium, or high
```

The tier flows through `DecomposeInput.Tier` to the pipeline. `Pipeline.Decompose()` uses `input.Tier` instead of the hardcoded `"sonnet"` constant.

`decomposeSinglePlanInCurrentDir()` gains a `cfg.HasProviders()` branch:
- **Providers configured:** Call `BuildRouterFromConfig(cfg, nil)`, wrap in existing `providerRouterClientAdapter{Router, Timeout, Phase: "decompose"}`, pass as `pipeline.Deps.ClaudeClient`
- **No providers:** Fall back to `claude.NewClient()` → `claudeClientAdapter` as today

The command layer reads `cfg.Decompose.Tier` and passes it as `DecomposeInput.Tier`. The adapter's `Run()` calls `TierFromLegacyModel(model)` which maps tier strings to themselves, so the router receives the correct tier.

### Retro Wiring

`runRetro()` gains a `cfg.HasProviders()` branch:
- **Providers configured:** Call `BuildRouterFromConfig(cfg, nil)`, wrap in new `retroRouterAdapter`, pass to `retro.NewRetroWithProviderAndBudget()`
- **No providers:** Fall back to `NewClaudeProvider()` as today

The `retroRouterAdapter` implements `retro.ProviderRunner` (Run + StreamRun). It:
1. Calls `router.Select("retro", tier)` to pick a provider
2. Delegates to that provider's `Run()` or `StreamRun()`
3. On usage-limit error: marks provider unavailable, retries with next provider once
4. Returns clear error when all providers exhausted

No Timeout field needed — retro already passes `context.Context` with its own timeout.

### Existing Command Refactoring

`constructor.go`, `review.go`, and `verify_spec.go` switch from duplicated inline provider construction to the shared helper. The constructor calls `BuildProvidersFromConfig()` directly since it also needs the providers map for learnings provider selection and cost defs. Review and verify-spec call `BuildRouterFromConfig()` and delete their local `buildVerifySpecProviders`, `buildReviewRouter`, etc.

## Test Strategy

### Unit Tests
- `BuildRouterFromConfig` with no providers → single-provider router
- `BuildRouterFromConfig` with claude-only → correct router
- `BuildRouterFromConfig` with codex-only → correct router
- `BuildRouterFromConfig` with multi-provider → router with both
- `BuildRouterFromConfig` with nil config → error
- `retroRouterAdapter.Run` delegates to selected provider
- `retroRouterAdapter.StreamRun` delegates to selected provider
- `retroRouterAdapter.Run` retries on usage-limit error
- `retroRouterAdapter` returns error when all providers exhausted

### Integration Tests
- Decompose passes `"decompose"` phase and configured tier through adapter
- Decompose defaults to `"medium"` when no tier configured
- Decompose uses `"high"` or `"low"` when configured
- Retro passes `"retro"` phase and correct tier through adapter
- Legacy paths unchanged when no `providers:` section

### Mocking Strategy
- Mock `routerSelector` interface for adapter tests (already defined)
- Mock `provider.Provider` for delegation and usage-limit behavior
- Real config structs for `BuildRouterFromConfig` (pure construction)

### Test Organization
- `internal/provider/build_test.go` — shared helper tests
- `cmd/gromit/adapters_test.go` — `retroRouterAdapter` tests
- Existing test files for decompose constant change

## Implementation Tasks

### Task 1: Create shared BuildRouterFromConfig helper

**Files:**
- Create: `internal/provider/build.go`
- Create: `internal/provider/build_test.go`

**What to Do:**
Create `BuildProvidersFromConfig(cfg *config.Config) (map[string]Provider, error)` — extracted from the duplicated logic in `constructor.go:236-265`, `verify_spec.go:347-374`. Handles claude, codex, and unknown provider types. Uses `cfg.Claude.Timeout` for Claude client creation, `DefaultTierToModelMap` when provider config has no models.

Create `BuildRouterFromConfig(cfg *config.Config, sf StateFile) (*Router, error)` — calls `BuildProvidersFromConfig` when `cfg.HasProviders()`, creates router with routing config. Falls back to single-provider Claude router otherwise. Accepts optional StateFile for persistent counts.

Export `ParseFallbackCooldown(cfg *config.Config) time.Duration` — extracted from the duplicated `parseFallbackCooldown` / `parseVerifySpecFallbackCooldown`.

**Acceptance Criteria:**
- `BuildRouterFromConfig` returns single-provider router when no providers configured
- `BuildRouterFromConfig` returns multi-provider router when providers configured
- `BuildProvidersFromConfig` creates correct provider types for claude and codex entries

**Dependencies:** None

**Notes:** `internal/provider` already imports `internal/claude`. `internal/config` does not import `internal/provider`, so no import cycle. The `defaultTierToModelMap` in constructor.go and `defaultVerifySpecTierToModelMap` in verify_spec.go are identical (high→opus, medium→sonnet, low→haiku) — export one `DefaultTierToModelMap` in build.go.

### Task 2: Add DecomposeConfig with configurable Tier

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`

**What to Do:**
Add a `DecomposeConfig` struct with a `Tier` string field (yaml tag `tier`). Add `Decompose DecomposeConfig` field to the top-level `Config` struct (yaml tag `decompose`). In `SetDefaults()`, default `Decompose.Tier` to `"medium"` when empty.

```go
type DecomposeConfig struct {
    Tier string `yaml:"tier"` // low, medium, or high — default medium
}
```

This enables `gromit.yaml` configuration:
```yaml
decompose:
  tier: medium  # or low, high
```

**Acceptance Criteria:**
- `DecomposeConfig` struct exists with `Tier` field
- Default tier is `"medium"` after `SetDefaults()`
- Config loads tier from YAML when specified

**Dependencies:** None

### Task 3: Make decompose tier configurable in pipeline

**Files:**
- Modify: `internal/pipeline/types.go`
- Modify: `internal/pipeline/decompose.go`

**What to Do:**
Add `Tier string` field to `DecomposeInput`. In `Pipeline.Decompose()`, use `input.Tier` instead of the hardcoded `decomposeModel` constant (`"sonnet"`). Remove or deprecate the `decomposeModel` constant.

If `input.Tier` is empty, default to `"medium"` as a safety fallback (defensive coding in case caller doesn't set it).

The `providerRouterClientAdapter.Run()` already calls `TierFromLegacyModel(model)` which maps tier strings to themselves. The `claudeClientAdapter.Run()` passes model directly to `claude.Client.Run()` — verify Claude CLI handles `"medium"` correctly in the legacy path. If not, the adapter can map tiers to legacy model names.

**Acceptance Criteria:**
- `DecomposeInput` has a `Tier` field
- `Pipeline.Decompose()` uses `input.Tier` instead of hardcoded constant
- Empty tier defaults to `"medium"` inside the pipeline

**Dependencies:** None

**Notes:** No need to import `internal/provider` in pipeline — just use the string literal `"medium"` as default. The tier constants are simple strings (`"low"`, `"medium"`, `"high"`).

### Task 4: Add retroRouterAdapter

**Files:**
- Modify: `cmd/gromit/adapters.go`
- Modify: `cmd/gromit/adapters_test.go`

**What to Do:**
Add `retroRouterAdapter` struct wrapping `routerSelector`. Implement `retro.ProviderRunner` interface (Run + StreamRun). Both methods:
1. Call `a.Router.Select("retro", tier)` to pick provider
2. Delegate to `provider.Run(ctx, prompt, tier)` or `provider.StreamRun(ctx, prompt, tier, output, handler, onToolCall)`
3. On usage-limit error (`provider.IsUsageLimitError(result, err)`): call `a.Router.MarkUnavailable(provider.Name())`, retry `Select` + invoke once
4. If no provider available after retry, return `fmt.Errorf("all providers exhausted (usage limits reached)")`

Add compile-time check: `var _ retro.ProviderRunner = (*retroRouterAdapter)(nil)`

No Timeout field — retro passes its own `context.Context`.

**Acceptance Criteria:**
- `retroRouterAdapter` implements `retro.ProviderRunner`
- `Run` delegates to selected provider and returns result
- `StreamRun` delegates to selected provider and returns result
- Usage-limit error triggers one retry with fallback provider

**Dependencies:** None

### Task 5: Wire decompose command to use provider routing

**Files:**
- Modify: `cmd/gromit/decompose.go`

**What to Do:**
In `decomposeSinglePlanInCurrentDir()`, add `cfg.HasProviders()` check before creating the Claude client:
- **If providers configured:** Call `provider.BuildRouterFromConfig(cfg, nil)`. Wrap router in `providerRouterClientAdapter{Router: router, Timeout: pipelineTimeout, Phase: "decompose"}`. Use as `pipeline.Deps.ClaudeClient`.
- **If no providers:** Keep existing `claude.NewClient()` → `claudeClientAdapter` path unchanged.

Read `cfg.Decompose.Tier` and pass it as `DecomposeInput.Tier`.

**Acceptance Criteria:**
- Decompose uses provider router when `providers:` config exists
- Decompose falls back to direct Claude client when no providers
- Phase `"decompose"` is passed to router for phase-preference selection
- Config tier is threaded through to `DecomposeInput.Tier`

**Dependencies:** Task 1 (shared helper), Task 2 (config field), Task 3 (pipeline input field)

### Task 6: Wire retro command to use provider routing

**Files:**
- Modify: `cmd/gromit/main.go`

**What to Do:**
In `runRetro()`, add `cfg.HasProviders()` check before creating the Claude provider:
- **If providers configured:** Call `provider.BuildRouterFromConfig(cfg, nil)`. Wrap router in `retroRouterAdapter{Router: router}`. Pass to `retro.NewRetroWithProviderAndBudget()`.
- **If no providers:** Keep existing `NewClaudeProvider()` path unchanged.

Remove the inline `tierToModel` map and `claude.NewClient()` from the providers path (the shared helper handles it).

**Acceptance Criteria:**
- Retro uses provider router when `providers:` config exists
- Retro falls back to direct Claude provider when no providers
- Phase `"retro"` is used for router selection

**Dependencies:** Task 1 (shared helper), Task 4 (retro adapter)

### Task 7: Refactor existing commands to use shared helper

**Files:**
- Modify: `internal/runner/constructor.go`
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/verify_spec.go`

**What to Do:**
**constructor.go:** Replace `buildProvidersFromConfig()` call with `provider.BuildProvidersFromConfig(cfg)`. Keep the learnings provider selection and state file handling locally (constructor needs the providers map for these). Delete the local `buildProvidersFromConfig` function. Replace `parseFallbackCooldown` with `provider.ParseFallbackCooldown`.

**review.go:** Replace `buildReviewRouter()` with `provider.BuildRouterFromConfig(cfg, nil)`. Delete `buildReviewRouter` function. The `buildReviewNonInteractiveClient` continues to wrap the router in `providerRouterClientAdapter`.

**verify_spec.go:** Replace `buildVerifySpecRouter()` with `provider.BuildRouterFromConfig(cfg, nil)`. Delete `buildVerifySpecRouter`, `buildVerifySpecProviders`, `defaultVerifySpecTierToModelMap`, `defaultVerifySpecCodexTierToModelMap`, `parseVerifySpecFallbackCooldown`. The `invokeSpecGateLLM` continues to use the router directly.

**Acceptance Criteria:**
- No duplicated provider construction logic remains across commands
- All existing tests pass after refactoring
- Behavior identical to before refactoring

**Dependencies:** Task 1

**Notes:** This task touches 3 files but each change is mechanical (replace local function call with shared helper call, delete dead local functions). Could decompose into 2 beads: one for constructor.go, one for review.go + verify_spec.go.

---

## Notes

- The `providerRouterClientAdapter` already exists in `cmd/gromit/adapters.go` with full usage-limit retry logic — decompose reuses it directly with `Phase: "decompose"`.
- The `routerSelector` interface (Select + MarkUnavailable) is already defined in `cmd/gromit/adapters.go` — both adapters use it.
- `internal/pipeline/decompose.go` may need a new import of `internal/provider` for `TierMedium`. If this causes an import cycle, use the string literal `"medium"` instead.
- The `defaultTierToModelMap` in constructor.go and `defaultVerifySpecTierToModelMap()` in verify_spec.go are identical — the shared helper exports one canonical version.
- Codex default tier maps differ per command (constructor uses `defaultCodexTierToModelMap`, verify_spec uses `defaultVerifySpecCodexTierToModelMap`). The shared helper should use the constructor's version as the canonical default, since it's the most commonly used.
