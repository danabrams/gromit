---
id: state-staleness-protection
source_ideas: [idea-1770462181900]
created: 2026-02-07
---

# State File Staleness Protection

## Specification

When `gromit run` starts, it loads `.gromit/state.json` to track review lifecycle state (iteration counters, last review commit, retro timestamps). If a previous run crashed or was killed, the state file is left behind with potentially unreliable counter values. The next run blindly consumes these values, which can cause incorrect thorough review trigger timing.

This feature adds two staleness detection mechanisms and an auto-heal behavior:

### 1. Clean-Exit Flag (Primary Signal)

The runner sets a `clean_exit` boolean in `state.json`:
- On run start: set `clean_exit: false` and save immediately
- On clean exit (all iterations complete or graceful stop): set `clean_exit: true` and save

When the next run loads state and finds `clean_exit: false`, it knows the previous run crashed.

### 2. Updated-At Timestamp (Secondary Signal)

Every `Save()` call writes an `updated_at` timestamp to `state.json`. When loading, if `time.Since(updated_at)` exceeds a configurable threshold, the state is considered stale regardless of the `clean_exit` flag. This catches edge cases like manual state edits, very old leftover state from before the clean-exit flag existed, or state from a process that set `clean_exit: true` but whose counters are from a much earlier context.

### 3. Auto-Heal on Stale/Crash Detection

When staleness is detected (either `clean_exit: false` or `updated_at` exceeds threshold), the runner auto-heals by selectively resetting fields:

| Field | Action | Rationale |
|-------|--------|-----------|
| `IterationsSinceReview` | Reset to 0 | Counter is unreliable after crash |
| `LastReviewIteration` | Reset to 0 | Tied to iteration counter |
| `LastReviewCommit` | Keep | Git commit hash is still valid |
| `LastRetro` | Keep | Historical timestamp, still useful |

The runner logs a warning when auto-heal triggers, explaining what was detected and what was reset.

### 4. Configuration

A new `state` section in `gromit.yaml`:

```yaml
state:
  stale_threshold: 60  # minutes; default 60 (1 hour)
```

The threshold only applies to the `updated_at` check. The `clean_exit` flag always triggers auto-heal regardless of time elapsed.

## Acceptance Criteria

- When a run crashes (simulated by not setting `clean_exit: true`), the next run resets `IterationsSinceReview` and `LastReviewIteration` to 0 while preserving `LastReviewCommit` and `LastRetro`
- When `updated_at` is older than the configured threshold, the same auto-heal behavior triggers
- A warning message is logged when auto-heal activates, including the reason (crash vs. stale timestamp)
- The `stale_threshold` is configurable in `gromit.yaml` with a default of 60 minutes
- `state.json` always contains `updated_at` and `clean_exit` fields after any save
- Existing `state.json` files without the new fields are handled gracefully (treated as stale on first load, then upgraded on save)

## Decisions

1. **Auto-heal over warn-and-continue or full-reset.** Selectively resetting counters while preserving git anchors gives the best tradeoff: unreliable data is discarded, but valid reference points (commit hashes, timestamps of past events) are kept. A full reset would lose the review baseline unnecessarily.

2. **Dual staleness signals.** The `clean_exit` flag is the primary crash detector — it's definitive. The `updated_at` threshold is a secondary safety net for edge cases the flag can't catch (pre-existing state files, manual edits, very old state). Using both provides defense in depth.

3. **Config under `state` section, not `loop` or `review`.** Staleness protection is a property of the state system itself, not specific to the loop or review features. A dedicated `state` section keeps concerns separated and allows future state-related config to live nearby.

4. **Default threshold of 1 hour.** Most gromit runs complete within minutes. An hour-old state file almost certainly represents a previous session, not an active one. This is conservative enough to avoid false positives during long-running builds while catching genuinely stale state.

## Research & Context

### Current State

The state system lives in `internal/state/state.go`. The `State` struct currently has four fields:

```go
type State struct {
    LastRetro             time.Time `json:"last_retro,omitempty"`
    LastReviewCommit      string    `json:"last_review_commit,omitempty"`
    LastReviewIteration   int       `json:"last_review_iteration,omitempty"`
    IterationsSinceReview int       `json:"iterations_since_review,omitempty"`
}
```

The `File` type wraps this with `Load()` and `Save()` methods. State is consumed in three places:
- `internal/runner/runner.go` line 242: loaded at run start, used for review trigger logic
- `internal/runner/runner.go` line 956: loaded by `Status()` for health display
- `cmd/gromit/main.go`: `runRetro()` calls `RecordRetro()` to update `LastRetro`

### Related Work

The `status-json-staleness` spec/plan (already implemented) added PID-based stale detection for `status.json` — a separate file that tracks whether a run is actively in progress. That work is complementary: `status.json` staleness answers "is a run currently happening?", while this spec's `state.json` staleness answers "is the accumulated review state reliable?".

### Key Files

- `internal/state/state.go` — State struct, Load/Save, field accessors
- `internal/state/state_test.go` — existing tests
- `internal/runner/runner.go` — primary consumer (lines 242-248, 434-445)
- `internal/config/config.go` — Config struct (add StateConfig)
- `gromit.yaml` — reference config
