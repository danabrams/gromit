---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T16:45:00-05:00"
id: wire-router-from-config
source_spec: wire-router-from-config
---

# Wire Router from Providers Config — Implementation Plan

**Goal:** Fill in the empty `HasProviders()` branch in `NewRunner()` so that config-defined providers are instantiated and wired into a multi-provider `Router`.

**Architecture:** Iterate `cfg.Providers` in `NewRunner()` to create `ClaudeProvider` or `CodexProvider` per entry, promote state file creation from `Run()` to `NewRunner()`, parse routing config (with defaults from `SetDefaults()`), and construct a `Router` via `NewRouter()`. Move the unconditional `claudeClient` creation into the legacy `else` branch.

**Tech Stack:** Go

**Spec:** `.gromit/specs/wire-router-from-config.md`

---

## Architecture

**Overview:** The `NewRunner()` constructor has two branches: the existing legacy path (single Claude client wrapped in `NewSingleProviderRouter`) and the new multi-provider path (when `cfg.HasProviders()` is true). The multi-provider path creates the correct `Provider` for each entry in `cfg.Providers`, promotes the state file to the Runner struct, and constructs a full `Router`.

**Integration Points:**
- `NewRunner()` in `internal/runner/runner.go` — main change site (fill HasProviders branch, promote state file)
- `SetDefaults()` in `internal/config/config.go` — add routing defaults when providers are present
- `Run()` in `internal/runner/runner.go` — reuse pre-created state file

**Data Flow:**
1. `SetDefaults()` fills missing routing fields when `providers` is present
2. `NewRunner()` iterates `cfg.Providers`, creates providers with tier-to-model maps
3. State file created in `NewRunner()`, stored on `Runner.stateFile`
4. `NewRouter()` receives providers, preferences, ratio, cooldown, state file
5. `Run()` reuses `r.stateFile` instead of creating a second `state.File`

**Key Decisions:**
- Provider type determined by map key name (`"claude"` or `"codex"`), not binary path
- State file promoted to Runner struct to avoid duplicate instances and enable cross-session ratio balancing
- Default tier mappings hardcoded per provider type (Claude: opus/sonnet/haiku, Codex: gpt-5.3-codex for all tiers)
- `claudeClient` creation moved inside `else` branch — multi-provider path creates Claude clients from `ProviderDef`

## Test Strategy

**Config defaults** (config_test.go):
- `SetDefaults()` fills routing defaults when providers present and routing empty
- `SetDefaults()` doesn't overwrite user-specified routing values
- `SetDefaults()` leaves routing alone when no providers present

**Runner wiring** (runner_test.go):
- Multi-provider config creates router that selects expected provider per phase
- Single claude in providers section works through multi-provider path
- Unrecognized provider name returns clear error
- Empty models map uses default tier mappings
- State file created on Runner struct and reused in Run()
- Legacy path unchanged

**Mocking:** Use `NewRunnerWithDeps` for router behavior tests. Verify behavior (which provider is selected) rather than inspecting internal state.

## Implementation Tasks

### Task 1: Add routing defaults to SetDefaults() with tests

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
In `SetDefaults()`, when `len(c.Providers) > 0`, fill in missing routing defaults:
- If `Routing.PhasePreferences` is nil, initialize to empty map (all phases default to `"any"` via Router logic)
- If `Routing.Ratio` is nil, build equal-split ratio from provider names (e.g., 2 providers → 50/50)
- If `Routing.Fallback.Cooldown` is empty, set to `"30m"`
- If `Routing.Fallback` has zero-value Enabled field and providers > 1, default Enabled to true

Do NOT overwrite non-zero user values.

**Acceptance Criteria:**
- When providers are present and routing is empty, defaults are applied (equal ratio, 30m cooldown, fallback enabled for multi-provider)
- When providers are present and routing has user values, those values are preserved
- When no providers are present, routing section is untouched

**Dependencies:** None

### Task 2: Wire multi-provider Router from config in NewRunner with tests

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**

**Runner struct changes:**
- Add `stateFile *state.File` field to Runner struct

**NewRunner() changes:**
- Move `claudeClient` creation (currently at line 73) into the `else` branch so it only runs on the legacy path
- Fill the `if cfg.HasProviders()` branch:
  1. Create state file via `state.NewFile(gromitDir)`, load it, store on Runner
  2. Define default tier maps: `defaultClaudeTierMap` (opus/sonnet/haiku) and `defaultCodexTierMap` (gpt-5.3-codex for all)
  3. Iterate `cfg.Providers`: for `"claude"` entries, create `claude.NewClient` from `ProviderDef` fields and wrap in `provider.NewClaudeProvider`; for `"codex"`/`"openai"` entries, create `provider.NewCodexProvider`; for unknown names, return error
  4. Use provider's `Models` map if non-nil/non-empty, otherwise use the default tier map for that type
  5. Parse `cfg.Routing.Fallback.Cooldown` via `time.ParseDuration`, default to 30m
  6. Set cooldown to 0 if `cfg.Routing.Fallback.Enabled` is false
  7. Call `provider.NewRouter(providers, cfg.Routing.PhasePreferences, cfg.Routing.Ratio, cooldown, stateFile)`
  8. Set `claudeProviderForLearnings` to the claude provider from the map (or first available if no claude)

**Run() changes:**
- If `r.stateFile` is already set (from NewRunner), reuse it instead of calling `state.NewFile()` again
- Keep existing state file creation as fallback for legacy path (where stateFile is nil)

**Acceptance Criteria:**
- `NewRunner` with multi-provider config creates a Router that selects the expected provider for a given phase
- Unrecognized provider name returns error like `unsupported provider "gemini": supported providers are claude, codex`
- Empty models map on a provider uses default tier mapping for that type
- State file is created in NewRunner and reused in Run()
- Legacy path (no providers section) continues to work identically
- A test verifies phase preference routing: config with `build: claude` → router selects claude for build

**Dependencies:** Task 1

---

## Notes

- The `defaultTierToModelMap` var already exists at runner.go line 33 — reuse it for the claude default tier map rather than creating a duplicate
- The codex default tier map (`gpt-5.3-codex` for all tiers) is new and specific to this wiring
- `claude.NewClient` takes `(binary, flags, timeoutSecs)` — for the multi-provider path, use `ProviderDef.Binary` and `ProviderDef.Flags`, and fall back to `cfg.Claude.Timeout` for the timeout since `ProviderDef` doesn't have a timeout field
- The `ProviderDef` has `PromptDelivery` and `PromptFlag` fields — these are only used by `NewCodexProvider`, not by Claude
- Do NOT change the Router or Provider interfaces, do NOT fix CodexProvider invocation patterns, do NOT add new provider types
