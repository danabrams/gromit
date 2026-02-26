---
id: learnings-provider-selection
source_ideas: []
created: 2026-02-26
epic: provider-ecosystem
---

# Learnings Provider Selection Without Claude Hardcoding

## Problem

`selectLearningsProvider()` in `constructor.go` hardcodes a lookup for `providers["claude"]`, biasing learnings filtering toward Claude even when another provider is configured as primary. Users running Codex or other providers get Claude-based learnings filtering if Claude is also registered, or broken behavior if it is not.

## Approach

- Remove the hardcoded `providers["claude"]` lookup from `selectLearningsProvider()`
- Add an optional config field (e.g., `learnings.provider` in the gromit YAML config) that lets users explicitly designate which provider handles learnings filtering
- If `learnings.provider` is set and that provider exists in the registered providers map, use it
- Otherwise, fall back to the first available provider in the map (deterministic ordering: sort keys alphabetically to avoid map iteration nondeterminism)
- Update `selectLearningsProvider()` signature or logic to accept the config value
- Add unit tests: config field set to existing provider, config field set to missing provider (falls back), config field unset (falls back to first available), no providers registered (returns nil or error)

## Files to Change

- `internal/runner/constructor.go` — update `selectLearningsProvider()` to remove Claude hardcoding and implement config-based + fallback selection
- `internal/config/config.go` — add `Learnings.Provider` string field to the config struct
- `internal/config/config_test.go` or equivalent — add YAML parsing test for new field
- `internal/runner/constructor_test.go` — add unit tests for the new selection logic

## Acceptance Criteria

- `selectLearningsProvider()` contains no reference to the string literal `"claude"`
- When `learnings.provider` config field is set to a registered provider name, that provider is used for learnings filtering
- When config field is unset or names a missing provider, the first alphabetically-ordered registered provider is used
- Unit tests cover all four cases: explicit match, explicit miss (fallback), unset (fallback), empty providers map
- Existing learnings filtering behavior is preserved for single-provider setups
