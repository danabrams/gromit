---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T12:36:50Z"
id: learnings-provider-selection
source_spec: learnings-provider-selection
---

# Learnings Provider Selection Implementation Plan

**Goal:** Make learnings filtering provider selection configurable and deterministic without hardcoding Claude.

**Architecture:** Extend config with `learnings.provider`, pass it into runner provider selection, and use deterministic alphabetical fallback when unset or invalid.

**Tech Stack:** Go, YAML config loading (`gopkg.in/yaml.v3`), existing runner/provider abstractions.

**Spec:** `.gromit/specs/learnings-provider-selection.md`

---

## Architecture

## Architecture Proposal

**Overview:**  
Add a config-driven learnings-provider selector that first honors `learnings.provider` when valid, otherwise deterministically falls back to the alphabetically first registered provider.

**Key Components:**
1. **Config schema (`internal/config`)**: Extend `LearningsConfig` with `Provider string` (`yaml:"provider"`).
2. **Normalization path (`config.Load` flow)**: Normalize `learnings.provider` (trim + lowercase) during post-load normalization so matching is consistent.
3. **Runner selection (`internal/runner`)**: Update `selectLearningsProvider` to accept the configured provider name and remove Claude-specific behavior.
4. **Deterministic fallback helper**: Sort provider map keys before fallback selection to avoid map iteration nondeterminism.

**Integration Points:**
- `buildRouterAndLearningsProvider` passes `cfg.Learnings.Provider` into the selector.
- Single-provider setups remain unchanged: fallback picks the only provider.
- No router strategy changes; this affects only learnings filtering provider choice.

**Data Flow:**
1. YAML `learnings.provider` loads into `Config.Learnings.Provider`.
2. Normalization standardizes the configured name.
3. Runner builds providers map from config.
4. Selector resolves provider:
   - explicit configured match if present
   - else alphabetically first available provider
   - else `nil` for empty map
5. Resolved provider is wired into learnings filter.

**Files to Modify:**
- `internal/config/config_types.go` - add `Learnings.Provider`.
- `internal/config/config.go` - normalize new field in post-load normalization.
- `internal/config/learnings_config_test.go` - add YAML parsing/normalization tests for `learnings.provider`.
- `internal/runner/constructor.go` - remove Claude hardcoding; add config-aware deterministic selector.
- `internal/runner/constructor_test.go` - add explicit match/miss/unset/empty-map test cases.

**Files to Create:**
- None expected.

**Tradeoffs:**
- Sorting keys on each selection is slightly more work than direct map iteration, but guarantees deterministic behavior.
- Lowercasing config values improves UX consistency but assumes provider names are case-insensitive, which aligns with existing normalization behavior.

## Test Strategy

**Test Levels:**
1. **Unit Tests (config):** verify YAML parsing + normalization for `learnings.provider`.
2. **Unit Tests (runner selection):** verify selection logic for configured match, configured miss fallback, unset fallback, and empty map.
3. **Integration-adjacent constructor test:** keep existing `buildRouterAndLearningsProvider` behavior checks and add assertions for the new config-driven selection path.

**Key Test Cases:**
- `learnings.provider: codex` with providers `{codex, claude}` selects `codex`.
- `learnings.provider: missing` with providers `{codex, claude}` falls back to alphabetically first (`claude`).
- `learnings.provider` unset with providers `{codex, claude}` falls back to alphabetically first (`claude`).
- Empty providers map returns `nil`.
- YAML with mixed case/whitespace (for example `"  CoDeX  "`) normalizes to `codex`.

**Mocking Strategy:**
- Reuse existing lightweight stub provider in `constructor_test.go`.
- No heavy mocking needed; pure in-memory maps and config structs are enough.

**Coverage Goals:**
- Remove all dependence on `"claude"` literal in selection path.
- Cover deterministic fallback behavior.
- Cover config parse + normalize path so runtime behavior matches YAML intent.

**Test Organization:**
- Add focused tests to:
  - `internal/config/learnings_config_test.go`
  - `internal/runner/constructor_test.go`
- Use existing naming style: `TestXxx_Yyy`.

## Implementation Tasks

### Task 1: Extend Learnings Config Schema and Normalization

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config.go`
- Test: `internal/config/learnings_config_test.go`

**What to Do:**
Add `Provider string` to `LearningsConfig` with YAML tag `provider`. Update post-load normalization to trim and lowercase this value so provider matching is stable regardless of YAML casing/spacing. Add tests verifying parse and normalization behavior.

**Acceptance Criteria:**
- `LearningsConfig` includes `Provider string` mapped to `learnings.provider`.
- Loading YAML with `learnings.provider` sets config correctly after normalization.
- Omitted `learnings.provider` remains empty and does not change existing defaults.

**Dependencies:**
- None.

**Notes:**
- Follow existing normalization conventions used for other selector fields.

### Task 2: Refactor Learnings Provider Selection in Runner

**Files:**
- Modify: `internal/runner/constructor.go`
- Test: `internal/runner/constructor_test.go`

**What to Do:**
Update `selectLearningsProvider` to accept configured provider name and registered providers map. Remove hardcoded Claude lookup. Implement resolution order: configured match, else first provider by alphabetical key ordering, else nil. Update call site in `buildRouterAndLearningsProvider` to pass `cfg.Learnings.Provider`.

**Acceptance Criteria:**
- `selectLearningsProvider` contains no reference to string literal `"claude"`.
- Configured provider is used when present in providers map.
- Unset or missing configured provider falls back to alphabetically first provider.
- Empty providers map returns `nil`.

**Dependencies:**
- Task 1 (for config field usage in call path).

**Notes:**
- Keep fallback deterministic with sorted keys to avoid map iteration nondeterminism.

### Task 3: Add Selection Matrix Tests for Runner Behavior

**Files:**
- Modify: `internal/runner/constructor_test.go`

**What to Do:**
Add explicit tests for all selection branches: config match, config miss fallback, config unset fallback, and no providers. Ensure tests assert exact selected provider names to lock deterministic behavior.

**Acceptance Criteria:**
- Unit tests cover all four required scenarios.
- Tests fail if fallback becomes non-deterministic.
- Existing single-provider behavior remains validated.

**Dependencies:**
- Task 2.

**Notes:**
- Use existing test helper patterns in `constructor_test.go` for provider setup.

### Task 4: Validate End-to-End Config-to-Selection Wiring

**Files:**
- Modify: `internal/runner/constructor_test.go`
- Modify: `internal/config/learnings_config_test.go` (if needed for additional integration confidence)

**What to Do:**
Add or extend a constructor-level test that uses full config object path (including `Learnings.Provider`) and verifies `buildRouterAndLearningsProvider` returns the expected learnings provider. This ensures schema, normalization, and selection logic work together.

**Acceptance Criteria:**
- Constructor-level test verifies configured `learnings.provider` influences returned learnings provider.
- Constructor-level test verifies fallback path when configured provider is missing.
- All updated config and runner tests pass together.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
- Prefer extending existing constructor tests to keep test surface compact.

---

## Notes

- This plan intentionally keeps changes localized to `internal/config` and `internal/runner`.
- Existing behavior for single-provider setups should remain unchanged by deterministic fallback.
- If template documentation updates are desired, add a follow-up task to document `learnings.provider` in `gromit.yaml` and corresponding doc tests.
