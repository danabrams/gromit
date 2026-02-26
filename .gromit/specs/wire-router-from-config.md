---
id: wire-router-from-config
source_ideas: []
created: 2026-02-12
epic: provider-ecosystem
---

# Wire Router from Providers Config

## Specification

The `NewRunner()` constructor in `internal/runner/runner.go` has a TODO at line 103: when `cfg.HasProviders()` is true, it should build a multi-provider `Router` from the `providers` and `routing` config sections instead of falling back to the single-provider Claude path. Without this wiring, even a fully functional `CodexProvider` cannot be used in `gromit run`.

### Why

This is the last piece of plumbing that connects config-defined providers to the running loop. Everything else is in place: `Provider` interface, `Router`, `ClaudeProvider`, `CodexProvider` (skeleton), `state.File` with `StateFile` methods, and `RoutingConfig` parsing. But the constructor ignores the `providers` section entirely — the `if cfg.HasProviders()` branch is empty.

### What To Build

Fill in the `if cfg.HasProviders()` branch in `NewRunner()` to:

1. **Iterate `cfg.Providers`** and instantiate the correct provider for each entry:
   - If the provider name is `"claude"` (or the binary is `"claude"`), create a `claude.Client` from the `ProviderDef` fields and wrap it in `provider.NewClaudeProvider()` with the entry's `Models` map as the tier mapping.
   - If the provider name is `"codex"` (or `"openai"`, or the binary is `"codex"`), create a `provider.NewCodexProvider()` from the `ProviderDef` fields.
   - For unrecognized providers, return an error with the provider name and a hint about supported values.

2. **Parse the cooldown duration** from `cfg.Routing.Fallback.Cooldown` (a string like `"30m"`) using `time.ParseDuration`. Default to 30 minutes if empty or unparseable. Only apply cooldown if `cfg.Routing.Fallback.Enabled` is true; otherwise use 0.

3. **Load the state file** to pass as the `StateFile` argument to `NewRouter`. The state file is currently created locally in the `Run()` method at line 335, but the router needs it at construction time for initial provider counts. Either:
   - Create and load the state file earlier in `NewRunner()` and pass it to `NewRouter`, then store it on the Runner struct so `Run()` can reuse it instead of creating a second instance, OR
   - Accept that the router starts with nil `StateFile` and the counts start at zero (simpler, but loses cross-session ratio balancing).

   Preferred approach: create the state file in `NewRunner()`, store on Runner, and pass to router. This preserves cross-session state and avoids creating duplicate `state.File` instances.

4. **Construct the Router** via `provider.NewRouter(providers, preferences, ratio, cooldown, stateFile)` using the maps built in steps 1-3.

5. **Set `claudeProviderForLearnings`** to the Claude provider from the providers map (if one exists). The learnings filter and analyzer both need a provider — they should use whichever Claude provider was configured. If no Claude provider exists in the multi-provider config, use the first available provider as a fallback for learnings/analyzer (they just need an LLM, not specifically Claude).

6. **Remove the fallback `claudeClient` creation** that currently happens unconditionally at line 73. In the multi-provider path, Claude clients should be created from `ProviderDef` config, not from the legacy `cfg.Claude` section. The legacy path (`else` branch) continues to use `cfg.Claude` as today.

### Config Shape

The provider config already parses correctly (tested in config_test.go). Here's what the YAML looks like when active:

```yaml
providers:
  claude:
    binary: claude
    flags: ["--no-input"]
    models:
      high: opus
      medium: sonnet
      low: haiku
  codex:
    binary: codex
    flags: []
    models:
      high: gpt-5.3-codex
      medium: gpt-5.3-codex
      low: gpt-5.3-codex

routing:
  phase_preferences:
    build: claude
    validate: any
    review: any
  ratio:
    claude: 60
    codex: 40
  fallback:
    enabled: true
    cooldown: 30m
```

### Edge Cases

- **Single provider in `providers` section**: Should work identically to the legacy path but through the multi-provider code path. A `providers` section with only `claude` defined is valid.
- **No routing section**: When `providers` exists but `routing` is empty/missing, default to: all phases `"any"`, equal ratio split, fallback enabled with 30m cooldown.
- **Provider binary not found**: The provider constructors don't validate binary existence at construction time (they fail at invocation). This is consistent with the existing Claude client behavior — don't add binary checks here.
- **Empty models map**: If a provider's `models` map is nil or empty, use a default tier mapping. For Claude: `{high: opus, medium: sonnet, low: haiku}`. For Codex: `{high: gpt-5.3-codex, medium: gpt-5.3-codex, low: gpt-5.3-codex}`.

### Files to Touch

- `internal/runner/runner.go` — Fill in the `HasProviders()` branch, potentially refactor state file creation
- `internal/runner/runner.go` — Add Runner struct field for `stateFile *state.File` if promoting it
- `internal/config/config.go` — Add `SetDefaults()` entries for `Routing` defaults (empty preferences → "any", empty ratio → equal split, empty cooldown → "30m")
- `internal/config/config_test.go` — Test routing defaults
- `internal/runner/runner_test.go` — Test that NewRunner with providers config produces a working router

### What NOT To Do

- Do not fix the CodexProvider's invocation pattern (stdin vs `--prompt`) — that's `gromit-st3a`
- Do not add new provider types beyond claude and codex
- Do not change the Router or Provider interfaces
- Do not change how the legacy (`else`) branch works

## Acceptance Criteria

- When `gromit.yaml` has a `providers` section with at least one provider, `NewRunner` constructs a multi-provider `Router` instead of a single-provider router
- Each provider in the config map is instantiated as the correct type (`ClaudeProvider` or `CodexProvider`) with tier-to-model mapping from the config's `models` field
- Routing preferences, ratio, and cooldown from `cfg.Routing` are passed to `NewRouter`
- The state file is wired as the `StateFile` for the router, enabling cross-session provider count persistence
- `claudeProviderForLearnings` is set to the Claude provider from the config (or first available provider if no Claude)
- When `providers` exists but `routing` is empty, sensible defaults are applied (all `"any"`, equal ratio, 30m cooldown)
- Unrecognized provider names return a clear error
- All existing tests pass (backward-compatible — the `else` branch is unchanged)
- A new test verifies that `NewRunner` with a multi-provider config creates a router that selects the expected provider for a given phase

## Decisions

1. **State file promotion to Runner struct**: Creating the state file in `NewRunner` and storing it on the Runner avoids duplicate instances and enables the router to start with persisted provider counts. The `Run()` method's existing state file creation (line 335) should reuse the Runner's field.

2. **Provider name matching**: Use the provider map key name (e.g., `"claude"`, `"codex"`) as the primary discriminator, not the binary path. This is simpler and matches how `NewRouter` uses provider names for routing.

3. **Default tier mappings**: Provide hardcoded defaults per provider type rather than requiring the `models` map. Users who only want default model names shouldn't need to specify them.
