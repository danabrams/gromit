---
created: 2026-02-25T03:10:18Z
decomposed: false
id: reset-routing-state-on-stale
source_spec: debug-20260225-031018
---

# Reset Stale Provider Routing State — Implementation Plan

**Goal:** Ensure provider routing rebalances after crashes/stale state by auto-healing `provider_counts` and cooldowns.

**Architecture:** Use `state.File.CheckStaleness()` during router setup and extend `AutoHeal()` to clear provider routing counters and cooldowns. Save healed state so subsequent runs start from a neutral routing baseline.

**Spec:** Investigation report at `.gromit/reports/debug-20260225-031018.md`.

---

## Architecture

**Overview:**
Provider routing uses persisted counts (`state.json`) to hit ratio targets. If a run crashes (`clean_exit: false`) or state is stale, counts should be treated as unreliable. We already have `CheckStaleness()` and `AutoHeal()` in `internal/state/state.go`, but they are unused and incomplete. Wire them into router initialization and reset routing fields in `AutoHeal()`.

**Key Components:**
1. **`internal/runner/constructor.go`**: After loading `state.json`, call `CheckStaleness` with the configured threshold. If stale, call `AutoHeal()` and `Save()`, and emit a warning including the staleness reason.
2. **`internal/state/state.go`**: Extend `AutoHeal()` to reset `ProviderCounts` and `ProviderUnavailableUntil` (routing/cooldowns) in addition to `IterationsSinceReview`.
3. **`internal/state/state_test.go`**: Update `TestAutoHeal` (and add a new test if needed) to verify provider counts/cooldowns are reset when auto-heal runs.

**Files to Modify:**
- `internal/runner/constructor.go`
- `internal/state/state.go`
- `internal/state/state_test.go`

**Tradeoffs:**
- Clearing provider counts after stale detection may briefly reduce long-term ratio smoothing, but it restores expected multi-provider behavior after crashes.
- Persisting healed state ensures deterministic behavior across runs.

## Test Strategy

**Test Levels:**
1. **Unit tests** for `AutoHeal` behavior in `internal/state/state_test.go`.
2. **Unit test** (or update existing test) in `internal/runner/constructor_test.go` to confirm stale detection triggers auto-heal and save.

**Key Test Cases:**
- `AutoHeal` resets `ProviderCounts` and `ProviderUnavailableUntil` while preserving `LastRetro`.
- When `clean_exit` is false, constructor staleness path triggers `AutoHeal` and results in empty provider counts.

## Implementation Tasks

### Task 1: Extend AutoHeal to clear routing state

**Files:**
- Modify: `internal/state/state.go`

**What to Do:**
Update `AutoHeal()` to reset `ProviderCounts` (use `ResetProviderCounts()` or re-init to empty map) and clear `ProviderUnavailableUntil` to an empty map. Preserve `LastRetro` and other historical fields.

**Acceptance Criteria:**
- `AutoHeal()` clears provider counts and cooldowns.
- Existing behavior for `IterationsSinceReview` remains intact.

**Dependencies:** None

### Task 2: Apply staleness auto-heal during router initialization

**Files:**
- Modify: `internal/runner/constructor.go`

**What to Do:**
After loading the state file in `buildRouterAndLearningsProvider`, call `CheckStaleness` using the configured stale threshold (from config). If stale, call `AutoHeal()`, save the state, and print a warning to `output` with the staleness reason.

**Acceptance Criteria:**
- On stale state (e.g., `clean_exit: false`), provider counts reset and persisted.
- A warning is emitted with the staleness reason.

**Dependencies:** Task 1

### Task 3: Update tests for auto-heal behavior

**Files:**
- Modify: `internal/state/state_test.go`
- Modify: `internal/runner/constructor_test.go` (or add a new targeted test)

**What to Do:**
Update `TestAutoHeal` to include provider counts/cooldowns. Add a constructor test that writes a stale `state.json` (clean_exit false), loads via `buildRouterAndLearningsProvider`, and asserts provider counts are reset.

**Acceptance Criteria:**
- Tests pass and confirm routing state is cleared on auto-heal.

**Dependencies:** Tasks 1 and 2

---

## Notes

- Use the configured staleness threshold from `cfg.State.StaleThreshold` (default 60 minutes).
- Keep warnings structured and consistent with existing constructor warnings.
