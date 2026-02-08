---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T22:39:04-05:00"
id: state-staleness-protection
source_spec: state-staleness-protection
---

# State File Staleness Protection Implementation Plan

**Goal:** Detect when `state.json` was left behind by a crashed run and auto-heal by resetting unreliable counter fields while preserving valid reference points.

**Architecture:** Dual staleness signals (`clean_exit` flag + `updated_at` timestamp threshold) with selective field reset. Staleness logic lives in the `state` package; the runner wires it in at run start/exit.

**Tech Stack:** Go, YAML config

**Spec:** `.gromit/specs/state-staleness-protection.md`

---

## Architecture

**Overview:**
Add `CleanExit` (bool) and `UpdatedAt` (time.Time) fields to the `State` struct. Every `Save()` auto-stamps `UpdatedAt`. The runner sets `CleanExit = false` at run start and `CleanExit = true` at clean exit. A `CheckStaleness()` method on `File` detects crash or time-based staleness and returns what was detected. An `AutoHeal()` method resets `IterationsSinceReview` and `LastReviewIteration` to 0 while preserving `LastReviewCommit` and `LastRetro`.

**Key Components:**
1. **`internal/state/state.go`**: Extended `State` struct, `CheckStaleness()` method, `AutoHeal()` method, `Save()` auto-stamps `UpdatedAt`
2. **`internal/config/config.go`**: `StateConfig` struct with `StaleThreshold` int (minutes), default 60
3. **`internal/runner/runner.go`**: Clean-exit flag management at run start/exit, staleness check after load

**Integration Points:**
- `runner.go` after `sf.Load()` (~line 246): check staleness, auto-heal, log warning
- `runner.go` after load: set `CleanExit = false`, save immediately
- `runner.go` at clean exit (~line 450): set `CleanExit = true`, save
- `config.go`: new `State StateConfig` field on `Config`
- `gromit.yaml`: new `state` section

**Data Flow:**
1. Run starts → load state → set `clean_exit: false` → save
2. Check staleness (crash flag or timestamp threshold) → auto-heal if stale → log warning
3. Normal loop proceeds, `Save()` stamps `updated_at` on each write
4. Run completes → set `clean_exit: true` → save

**Tradeoffs:**
- Staleness logic in `state` package (not runner): keeps state self-contained, testable without runner dependencies
- Threshold as `int` minutes in config: matches existing config style (all timeouts are ints)

## Test Strategy

**Unit Tests (`internal/state/state_test.go`):**
- Clean exit flag crash detection (clean_exit: false → stale)
- Clean exit flag normal (clean_exit: true + fresh timestamp → not stale)
- Updated-at threshold exceeded (clean_exit: true but old timestamp → stale)
- Updated-at within threshold (→ not stale)
- Legacy state file without new fields (→ treated as stale, upgraded on save)
- Save stamps updated_at
- Save persists clean_exit through round-trip
- Auto-heal preserves LastReviewCommit and LastRetro
- Auto-heal resets IterationsSinceReview and LastReviewIteration to 0
- Staleness reason reporting (crash vs stale timestamp)

**Unit Tests (`internal/config/config_test.go`):**
- Default threshold = 60
- Custom threshold from YAML
- Zero threshold gets default

**Mocking Strategy:**
- No mocks — uses real files via `t.TempDir()` (matches existing pattern)
- Stale timestamps constructed by setting `UpdatedAt` to past time

**Test Organization:**
- Appended to existing test files, following existing naming conventions

## Implementation Tasks

### Task 1: Add StateConfig to config package

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add a `StateConfig` struct with `StaleThreshold int` (yaml: `stale_threshold`). Add `State StateConfig` field to the `Config` struct (yaml: `state`). Add default of 60 in `setDefaults()`. Add a commented `state` section to `gromit.yaml`.

**Acceptance Criteria:**
- `StateConfig` struct exists with `StaleThreshold int` field
- Default threshold is 60 when not specified in YAML
- Custom threshold loads correctly from YAML
- Zero threshold gets default of 60

**Dependencies:** None

**Notes:** Follow the exact pattern of other config sections (e.g., `LoopConfig`). The `stale_threshold: 0` case should default to 60, matching how other zero-value fields are handled.

### Task 2: Add staleness fields and detection to state package

**Files:**
- Modify: `internal/state/state.go`
- Test: `internal/state/state_test.go`

**What to Do:**
Add `CleanExit bool` and `UpdatedAt time.Time` fields to the `State` struct (json: `clean_exit` and `updated_at`). Update `Save()` to auto-stamp `UpdatedAt = time.Now()` before marshalling. Add `SetCleanExit(bool)` method on `File`. Add `CheckStaleness(thresholdMinutes int) (stale bool, reason string)` method that returns true with a reason string if `CleanExit` is false OR if `UpdatedAt` is non-zero and older than the threshold. Add `AutoHeal()` method that resets `IterationsSinceReview` and `LastReviewIteration` to 0 while preserving `LastReviewCommit` and `LastRetro`.

Note: do NOT use `omitempty` on `CleanExit` — a false value must be written to distinguish "explicitly false" from "field missing". `UpdatedAt` should also not use `omitempty` so it's always present after any save.

**Acceptance Criteria:**
- `Save()` always writes `updated_at` and `clean_exit` fields
- `CheckStaleness()` detects crash (clean_exit: false) and stale timestamp correctly
- `AutoHeal()` resets counters but preserves commit hash and retro timestamp
- Legacy state files without new fields are treated as stale on first load

**Dependencies:** None (does not depend on Task 1 — threshold is passed as a parameter)

**Notes:** `CheckStaleness` takes the threshold as a parameter rather than depending on config, keeping the state package independent. The `omitempty` tags on existing fields should remain unchanged — only the new fields need non-omitempty treatment. Handle the edge case where `UpdatedAt` is zero (legacy file): treat as stale regardless of threshold.

### Task 3: Integrate staleness protection into runner

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**
In the `Run()` method, after loading state (~line 246-248): (1) set `CleanExit = false` and save immediately, (2) call `CheckStaleness(cfg.State.StaleThreshold)`, (3) if stale, call `AutoHeal()`, log a warning with the reason. Before the function returns on clean exit (~line 450, after "Gromit loop complete" log): set `CleanExit = true` and save. The clean-exit save should happen regardless of whether the loop processed any iterations.

**Acceptance Criteria:**
- `clean_exit` is set to false and saved at run start
- Staleness is checked and auto-heal triggers with a logged warning when detected
- `clean_exit` is set to true and saved on clean exit

**Dependencies:** Task 1, Task 2

**Notes:** The clean-exit save at the end should not use `defer` — it should be explicit before the `return nil` to ensure it runs after the loop completes but the timing is deterministic. If `sf` is nil (state file creation failed), skip all staleness logic gracefully (already handled by existing nil checks). The `cfg.State.StaleThreshold` will have been defaulted to 60 by config loading.

---

## Notes

- The `omitempty` decision on `CleanExit` is critical: Go's JSON zero value for bool is `false`, so `omitempty` would omit `clean_exit: false` — making crash detection impossible. The field must always be serialized.
- The `status.json` staleness system (PID-based, for active-run detection) is complementary and unrelated. No changes needed there.
- The runner's `Status()` method (line ~956) loads state for display but doesn't need staleness checks — it's read-only for health display.
