---
id: multi-provider-routing
source_spec: multi-provider-routing
created: 2026-02-11
decomposed: false
---

# Multi-Provider Routing Implementation Plan

**Goal:** Enable the Gromit build loop to route LLM invocations across multiple CLI providers (Claude, Codex) with phase preferences, ratio balancing, and automatic usage-limit fallback.

**Architecture:** Introduce a `provider` package with a `Provider` interface and `Router` that replaces the runner's `ClaudeClient` dependency. Model selection shifts from concrete names to abstract tiers (high/medium/low), with each provider mapping tiers to its own models. The router handles phase preferences, ratio balancing, and usage-limit fallback via three-layer selection.

**Tech Stack:** Go, CLI subprocesses (claude, codex), YAML config, JSON state persistence

**Spec:** `.gromit/specs/multi-provider-routing.md`

---

## Architecture

### Overview

The current architecture has a single `ClaudeClient` interface through which all ~15 LLM invocation sites in the runner operate. This plan introduces a `Provider` interface (tier-based instead of model-name-based) and a `Router` that selects providers based on phase preferences, ratio balancing, and availability. `ClaudeProvider` wraps the existing `claude.Client`, and `CodexProvider` (stubbed initially) will handle Codex CLI invocations.

### Key Components

1. **`internal/provider/provider.go`** — Provider interface, Result type, Tier constants (High, Medium, Low)
2. **`internal/provider/claude.go`** — ClaudeProvider wrapping `claude.Client` with tier→model mapping
3. **`internal/provider/codex.go`** — CodexProvider stub (completed after Codex spike beads)
4. **`internal/provider/router.go`** — Three-layer selection: phase preferences → ratio balancing → fallback
5. **`internal/config/config.go`** — New `ProvidersConfig`, `RoutingConfig`, `SelectTier()`, backward compat
6. **`internal/state/state.go`** — Provider counts and unavailability timestamps

### Integration Points

- `Runner.claude ClaudeClient` replaced by `Runner.router *provider.Router`
- All invocation sites convert from `r.claude.Run(ctx, prompt, model)` to `p, _ := r.router.Select(phase, tier); p.Run(ctx, prompt, tier)`
- `learnings.ClaudeRunnerAdapter` and `analyzer.Analyzer` need adapting to work with Provider instead of ClaudeClient
- Backward compat: when no `providers` config section exists, a single ClaudeProvider is created automatically

### Model Selection Flow

```
bead priority/labels
  → config.SelectTier() → abstract tier (high/medium/low)
  → router.Select(phase, tier) → (Provider, modelName)
  → provider.Run(ctx, prompt, tier) → provider maps tier internally
  → CLI execution → provider.Result
  → on usage limit: router.MarkUnavailable() → retry with fallback
```

### Invocation Sites by Category

**Build (process.go):** executeClaudeInvocation (StreamRun), runAcceptanceTests (StreamRun), runRefactorPhase (Run), handleRefactorValidationFailure (Run)
**Validation (process.go):** runValidation (RunValidation), verifyTestsFail (RunValidation), runRefactorPhase validation, handleRefactorValidationFailure validation, runPostSuccessReview validation
**Auxiliary (runner.go):** runPrecheck (Run), checkScope (Run), runLightReview (Run), runThoroughReview (Run + RunValidation), DecomposeTask (Run), extractSuccessLearning (Run)

### Tradeoffs

- **Router replaces ClaudeClient** rather than coexisting — cleaner single path, larger refactor but no split-brain risk
- **Tier indirection** adds a mapping layer but makes adding providers trivial
- **ClaudeProvider wraps `claude.Client`** rather than reimplementing — preserves proven stream-json parsing
- **CodexProvider stubbed** initially — allows parallel progress on routing while spike beads validate Codex CLI behavior

---

## Test Strategy

### Unit Tests
- `provider/router_test.go` — Router.Select() with all three layers, cooldown expiry, error on all unavailable
- `provider/claude_test.go` — Tier→model mapping, IsUsageLimitError, delegation to claude.Client
- `provider/codex_test.go` — Tier→model mapping, command construction, IsUsageLimitError
- `config/config_test.go` — SelectTier(), backward compat detection, new config section parsing

### Integration Tests
- Router + mock providers — end-to-end routing with simulated usage limits
- Runner with Router — verify correct phase/tier passed to Select(), fallback retry works
- Config loading — full YAML with `providers`/`routing` sections; missing sections fall back to legacy

### Key Test Cases
- Phase preference pins to named provider → selected
- Phase preference `any` → ratio balancer picks furthest-below-target
- Preferred provider unavailable → fallback to next available
- All providers unavailable → error returned
- Cooldown expiry → provider becomes available again
- Legacy config (no `providers`) → identical behavior to current
- Legacy model names in escalation chain → mapped to tiers
- Normal failures NOT misidentified as usage limits

### Mocking Strategy
- Mock `Provider` for Router tests
- Mock `Router` for Runner tests (or use Router with mock Providers)
- Keep existing `ClaudeClient` mocks for backward compat verification

---

## Implementation Tasks

### Task 1: Create provider package foundation

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/provider_test.go`

**What to Do:**
Define the `Provider` interface with five methods: `Name() string`, `Run(ctx, prompt, tier) (*Result, error)`, `StreamRun(ctx, prompt, tier, output, handler, onToolCall) (*Result, error)`, `RunValidation(ctx, commands, tier, workDir) (*Result, error)`, `IsUsageLimitError(result, err) bool`. Define the `Result` struct mirroring `claude.Result` fields (Success, Output, ExitCode, Duration, Model). Define tier constants (`TierHigh`, `TierMedium`, `TierLow`). Define `EventHandler` and `ToolCallHandler` types (same signatures as `claude.EventHandler`/`claude.ToolCallHandler`). Add a `TierFromLegacyModel()` function that maps known model names (opus→high, sonnet→medium, haiku→high, o3→high, gpt-4o→medium, gpt-4o-mini→low) to tiers, returning the original string if not a known model name (for forward compat with tier names).

**Acceptance Criteria:**
- Provider interface defined with all five methods taking tier instead of model
- Result struct defined with Success, Output, ExitCode, Duration, Model fields
- TierFromLegacyModel correctly maps known model names to tiers and passes through tier names unchanged

**Dependencies:** None (foundational)

**Notes:** The EventHandler/ToolCallHandler types are defined here so providers can use them, but they're only meaningful for ClaudeProvider currently. CodexProvider will no-op these callbacks.

---

### Task 2: Add tier mapping and provider/routing config sections

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**What to Do:**
Add `SelectTier(priority int, labels []string) string` that works like `SelectModel` but returns tier names. Add `IsTierName(s string) bool` to detect whether a config value is a tier name vs. a legacy model name. Add `ProvidersConfig` struct (map of provider name → `ProviderDef` with Binary, Flags, PromptDelivery, PromptFlag, Models map[string]string). Add `RoutingConfig` struct with PhasePreferences (map[string]string), Ratio (map[string]int), Fallback (Enabled bool, Cooldown string). Add both to `Config`. Update `SetDefaults()` and `NormalizeNilFields()` for new fields. Add `HasProviders() bool` method that returns true when providers section is non-empty. Update escalation chain: `NextEscalationTier(currentTier string) string` alongside existing `NextEscalationModel`. For backward compat: when `Models.P0` contains a known model name (not a tier name), `SelectTier` auto-maps it via `TierFromLegacyModel`; when escalation chain contains model names, they're auto-mapped to tiers.

**Acceptance Criteria:**
- SelectTier returns tier names for priority-based and label-override selection
- ProvidersConfig and RoutingConfig parse correctly from YAML with appropriate defaults
- Backward compat: legacy model names in models/escalation config auto-detected and mapped to tiers

**Dependencies:** Task 1 (uses TierFromLegacyModel and tier constants)

**Notes:** Keep `SelectModel` and `NextEscalationModel` for backward compat during the transition. They can be deprecated later once all call sites are converted.

---

### Task 3: Implement ClaudeProvider

**Files:**
- Create: `internal/provider/claude.go`
- Create: `internal/provider/claude_test.go`

**What to Do:**
Create `ClaudeProvider` struct holding a `*claude.Client` and a tier→model map (e.g., `map[string]string{"high": "opus", "medium": "sonnet", "low": "haiku"}`). Implement all Provider interface methods. `Run()` resolves tier to model name via the map, then delegates to `claude.Client.Run()`, converting `claude.Result` to `provider.Result`. `StreamRun()` does the same, passing through EventHandler and ToolCallHandler (cast from `provider.EventHandler`/`provider.ToolCallHandler` to `claude.EventHandler`/`claude.ToolCallHandler`). `RunValidation()` delegates to `claude.Client.RunValidation()`. `IsUsageLimitError()` checks for Claude-specific patterns: exit code 2 with stderr containing "usage limit", "rate limit", or "quota exceeded" (case-insensitive). Add `NewClaudeProvider(client *claude.Client, tierMap map[string]string) *ClaudeProvider` constructor. Also add helper `IsValidationPassed(result) bool` and `IsScopeTooLarge(result) (bool, string)` that delegate to the underlying `claude` package functions (these are used by the runner but are Claude-specific).

**Acceptance Criteria:**
- Run/StreamRun/RunValidation correctly map tier to model and delegate to claude.Client
- IsUsageLimitError detects Claude-specific usage limit patterns and rejects normal failures
- Helper methods (IsValidationPassed, IsScopeTooLarge) work through the provider wrapper

**Dependencies:** Task 1 (Provider interface, Result type)

**Notes:** The EventHandler/ToolCallHandler type casting should be zero-cost since they have identical signatures. If the types differ, use adapter functions. Tests should use a mock claude.Client to verify correct delegation without requiring the real CLI.

---

### Task 4: Add provider routing state to state package

**Files:**
- Modify: `internal/state/state.go`
- Modify: `internal/state/state_test.go`

**What to Do:**
Add `ProviderCounts map[string]int` and `ProviderUnavailableUntil map[string]time.Time` fields to the `State` struct with appropriate JSON tags. Add helper methods: `IncrementProviderCount(name string)`, `GetProviderCounts() map[string]int`, `SetProviderUnavailable(name string, until time.Time)`, `IsProviderAvailable(name string) bool` (checks if current time is past the unavailable-until time), `ClearProviderUnavailable(name string)`. Update `NormalizeNilFields` equivalent (or add to existing nil-guard patterns) to ensure maps are initialized. Add `ResetProviderCounts()` for use at the start of each `gromit run` session.

**Acceptance Criteria:**
- ProviderCounts and ProviderUnavailableUntil serialize/deserialize correctly in state.json
- IsProviderAvailable returns false during cooldown and true after expiry
- Existing state fields are unaffected by the new fields

**Dependencies:** None (independent of provider package)

---

### Task 5: Implement Router with three-layer selection

**Files:**
- Create: `internal/provider/router.go`
- Create: `internal/provider/router_test.go`

**What to Do:**
Create `Router` struct holding: `providers map[string]Provider`, `preferences map[string]string` (phase→provider or "any"), `ratio map[string]int` (provider→target percentage), `counts map[string]int` (provider→invocation count), `unavailable map[string]time.Time` (provider→unavailable-until), `cooldown time.Duration`, `stateFn func() *state.File` (for persistence). Implement `Select(phase string, tier string) (Provider, string, error)` with three layers: (1) check phase preference — if named provider and available, use it; (2) for `any` or unavailable preferred, pick available provider furthest below target ratio; (3) if all unavailable, return error. Implement `MarkUnavailable(name string)` — records current time + cooldown. Implement `RecordInvocation(name string)` — increments count and persists to state. Add `NewRouter(providers, preferences, ratio, cooldown) *Router` constructor. Add `NewSingleProviderRouter(p Provider) *Router` convenience constructor for backward compat (one provider, all preferences "any", ratio 100%).

**Acceptance Criteria:**
- Select respects phase preferences, falls back to ratio balancing for "any", returns error when all unavailable
- MarkUnavailable prevents selection until cooldown expires
- NewSingleProviderRouter creates a minimal router that always returns the single provider

**Dependencies:** Task 1 (Provider interface), Task 4 (state persistence)

**Notes:** The router should check availability by comparing `time.Now()` against the unavailable-until time, not by maintaining a separate "available" flag. This ensures cooldown recovery is automatic. Provider counts can optionally be persisted to state.json via the stateFn, but the router should also work without persistence (in-memory only for tests).

---

### Task 6: Wire Router into Runner struct and constructors

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/interfaces.go`

**What to Do:**
Add `router *provider.Router` field to `Runner` struct (alongside existing `claude ClaudeClient` temporarily). Update `NewRunner()`: when `cfg.HasProviders()` is true, build providers from config, create Router; when false (backward compat), create a single ClaudeProvider and wrap in `NewSingleProviderRouter`. Update `Deps` struct to include an optional `Router *provider.Router` field. Update `NewRunnerWithDeps()` to use `deps.Router` when provided, falling back to creating a `NewSingleProviderRouter` from `deps.Claude` for backward compat with existing tests. Add a `selectTier(b *bead.Bead) string` method on Runner (parallel to existing `selectModel`) that uses `cfg.SelectTier`. Update the `Run()` nil checks to validate `r.router` instead of (or in addition to) `r.claude`.

**Acceptance Criteria:**
- NewRunner creates Router from providers config or falls back to single-provider Router
- NewRunnerWithDeps accepts Router in Deps, falls back to wrapping Claude for test compat
- Existing tests continue to pass with no changes (they pass ClaudeClient in Deps, which gets wrapped)

**Dependencies:** Task 2 (config), Task 3 (ClaudeProvider), Task 5 (Router)

**Notes:** This task intentionally keeps `r.claude` temporarily — it will be removed in Tasks 7-8 as call sites are converted. The `learnings.ClaudeRunnerAdapter` and `analyzer.Analyzer` still use `r.claude` until those call sites are converted. This is the riskiest task in terms of breaking existing tests — the backward compat path in NewRunnerWithDeps is critical.

---

### Task 7: Convert process.go invocation sites to use Router

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/process_test.go`

**What to Do:**
Convert all LLM invocation sites in process.go from `r.claude.Method()` to router-based calls. For each site: (1) determine the phase name (e.g., "build", "validate", "review"), (2) determine the tier (from `bc.model` → `selectTier`, or config field → tier mapping), (3) call `r.router.Select(phase, tier)` to get provider, (4) call provider method, (5) on failure, check `p.IsUsageLimitError()` and if true, call `r.router.MarkUnavailable()` then retry with new provider. Specific sites to convert:
- `executeClaudeInvocation`: phase="build", tier=selectTier(bead) — uses StreamRun
- `runAcceptanceTests`: phase="build", tier=selectTier(bead) — uses StreamRun
- `verifyTestsFail`: phase="validate", tier="low" — uses RunValidation
- `runRefactorPhase`: phase="build", tier=selectTier(bead) — uses Run; validation uses phase="validate", tier="low"
- `handleRefactorValidationFailure`: same as runRefactorPhase
- `runValidation`: phase="validate", tier="low" — uses RunValidation
- `runPostSuccessReview` validation: phase="validate", tier="low"

Update `beadContext` to store `tier string` alongside (or replacing) `model string`. Update `IterationResult.Model` to record the provider-resolved model name.

**Acceptance Criteria:**
- All process.go invocation sites use router.Select() with correct phase and tier
- Usage limit errors trigger MarkUnavailable + retry with fallback provider
- beadContext tracks tier, IterationResult.Model records the resolved model name

**Dependencies:** Task 6 (Router wired into Runner)

**Notes:** The `claude.IsValidationPassed()` and `claude.IsScopeTooLarge()` checks need to work through the provider. Either call them through `ClaudeProvider` helper methods, or keep using the `claude` package functions directly (they just do string matching on output, so they're not provider-specific in practice). The latter is simpler.

---

### Task 8: Convert runner.go invocation sites to use Router

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**
Convert all remaining LLM invocation sites in runner.go from `r.claude.Method()` to router-based calls. Sites to convert:
- `runPrecheck`: phase="precheck", tier="low" (currently hardcoded haiku)
- `checkScope`: phase="scope_check", tier="low" (currently cfg.ScopeCheck.Model)
- `runLightReview`: phase="review", tier=selectTier(bead) or "high" if matching opus build (current logic matches build model)
- `runThoroughReview`: phase="review", tier="high" (currently cfg.Review.Thorough.Model=opus); validation sub-call: phase="validate", tier="low"
- `DecomposeTask`: phase="decompose", tier="high" (currently hardcoded opus)
- `extractSuccessLearning`: phase="build", tier="low" (currently hardcoded haiku)

Also update the `learnings.ClaudeRunnerAdapter` — it currently wraps `ClaudeClient`. Either update it to accept a Provider, or create the adapter from the ClaudeProvider directly. Update `analyzer.Analyzer` similarly — it takes `ClaudeClient` in its constructor; switch to accepting a Provider or keep it using `r.claude` with the ClaudeProvider's underlying client.

After all sites are converted, remove the `claude ClaudeClient` field from `Runner` — all access now goes through `r.router`.

**Acceptance Criteria:**
- All runner.go invocation sites use router.Select() with correct phase and tier
- learnings adapter and analyzer work with the new provider-based flow
- `Runner.claude` field is removed — all LLM access goes through `r.router`

**Dependencies:** Task 7 (process.go converted first to reduce merge conflicts)

**Notes:** The review model matching logic (`selectReviewModel` checks if build used opus, and if so, uses opus for review too) needs to translate to tier-based logic: if build used "high" tier, review also uses "high" tier. This is simpler with tiers than with model names. The `FailureAnalyzer` currently takes `ClaudeClient` — it can be updated to take `Provider` or be given the `ClaudeProvider`'s underlying `*claude.Client` directly since analysis is always Claude-specific for now.

---

### Task 9: Create CodexProvider stub

**Files:**
- Create: `internal/provider/codex.go`
- Create: `internal/provider/codex_test.go`

**What to Do:**
Create `CodexProvider` struct holding binary path, flags, prompt delivery method, prompt flag, and tier→model map. Implement `Name() string` returning "codex". Implement `Run()`: build command with `--model <resolved-model>`, write prompt to temp file, invoke `codex --prompt <tempfile>` with flags, capture stdout/stderr, return Result. Implement `StreamRun()`: similar to Run but streams output to the writer; EventHandler/ToolCallHandler are no-ops (Codex doesn't emit Claude-style stream events). Implement `RunValidation()`: build prompt from commands list (same format as ClaudeProvider), invoke Codex. Implement `IsUsageLimitError()`: check for Codex-specific patterns ("rate limit", "quota", "too many requests", exit code patterns — exact patterns TBD from spike beads). Add `NewCodexProvider(binary string, flags []string, promptFlag string, tierMap map[string]string) *CodexProvider`.

**Acceptance Criteria:**
- CodexProvider satisfies Provider interface and compiles
- Run() constructs correct codex command with model flag and prompt file delivery
- Tier→model mapping works (high→o3, medium→gpt-4o, low→gpt-4o-mini from config)

**Dependencies:** Task 1 (Provider interface)

**Notes:** This is a stub — the exact Codex CLI behavior (exit codes, output format, approval mode flags) depends on the spike beads (gromit-zyc8, gromit-akah). The Run() implementation should be functional enough to invoke Codex and capture output, but StreamRun() event parsing and IsUsageLimitError() patterns may need refinement once spike results are in. Mark StreamRun event handling with TODO comments referencing the spike beads.

---

## Notes

### Relationship to Existing Beads

Several open beads overlap with this plan:
- **Usage limit detection** (gromit-tpzp, gromit-m5og, gromit-3eq6, gromit-t3pj): These implement `internal/usagelimit/` as a standalone package. This plan's `IsUsageLimitError()` on Provider serves a similar purpose but is provider-specific rather than centralized. The usagelimit beads can proceed in parallel — their detection logic can be called from within `ClaudeProvider.IsUsageLimitError()` and `CodexProvider.IsUsageLimitError()`.
- **Codex CLI spike** (gromit-zyc8, gromit-akah, gromit-w58v): Task 9 (CodexProvider) depends on these spike results for exact CLI flags, output format, and error patterns. The stub can be implemented now and refined after spikes complete.

### Migration Strategy

The backward compat path is critical. When no `providers` section exists in `gromit.yaml`:
1. `cfg.HasProviders()` returns false
2. `NewRunner` creates a single `ClaudeProvider` from the existing `claude` config
3. Wraps it in `NewSingleProviderRouter` — all phases route to Claude, no ratio balancing
4. Behavior is identical to current — same model names, same timeouts, same everything

This means users can upgrade Gromit without changing their config file.

### Phase Names for Routing

The following phase names are used in `router.Select()`:
- `build` — main code generation (executeClaudeInvocation, runAcceptanceTests, runRefactorPhase)
- `validate` — running tests/lint (runValidation, verifyTestsFail, all RunValidation calls)
- `analyze` — failure analysis (analyzer.Analyze)
- `scope_check` — complexity estimation (checkScope)
- `precheck` — pre-build acceptance check (runPrecheck)
- `decompose` — task decomposition (DecomposeTask)
- `review` — code review (runLightReview, runThoroughReview)

These match the phase names in the spec's `routing.phase_preferences` config.
