---
created: 2026-03-02T00:00:00Z
decomposed: true
decomposed_at: "2026-03-02T23:32:59-05:00"
id: unstick-beads
source_spec: unstick-beads
---

# Unstick Beads Implementation Plan

**Goal:** Add manual and automatic mechanisms to unstick beads that have exceeded the failure threshold, using a restart-point timestamp that preserves full history while giving beads a fresh slate.

**Architecture:** A new `internal/unstick` package owns restart-point persistence and auto-signal detection. Restart points filter JSONL log stats at read time so neither the gate nor the queue display incorrectly treats previously-failed-but-now-unstuck beads as stuck. A thin `internal/pipeline/unstick.go` exposes the feature through the standard pipeline interface, and `cmd/gromit/unstick.go` wraps it as a first-class CLI command.

**Tech Stack:** Go, existing `internal/logger`, `internal/queue`, `internal/pipeline`, `internal/events`, `internal/runner/policy`, bubbletea (already a dep, not needed for v1 picker)

**Spec:** `.gromit/specs/unstick-beads.md`

---

## Architecture

### Key Components

1. **`internal/unstick/` package** — New package owning all restart point logic:
   - `store.go`: `RestartPointStore` — loads/saves `.gromit/restart-points.json`; `RestartPoint` struct with `RestartAt time.Time` and `Reason string`
   - `auto.go`: `AutoChecker` — evaluates three signals (dependency closed, metadata changed, new commits) for each stuck bead and records restart points when triggered

2. **Modified `internal/logger/logger.go`** — New `ReadPerBeadStatsAfter(logsDir string, after map[string]time.Time)` function that filters log entries by per-bead timestamp, returning stats for only post-restart failures

3. **Modified `internal/queue/snapshot.go`** — `FindStuckBeadIDs` gains a `restartAfter map[string]time.Time` parameter to filter failures correctly for queue display

4. **Modified `internal/runner/constructor.go`** — Wire `StuckDetector` into gate (currently missing); create `stuckDetectorAdapter` that loads restart points and per-bead stats filtered by restart point, then calls `ThresholdStuckPolicy`

5. **`BeadUnstickedEvent` in `internal/events/types_lifecycle.go`** — Emitted on both manual and automatic unstick

6. **`internal/pipeline/unstick.go`** — `Pipeline.ListStuck()` and `Pipeline.Unstick()` methods; same thin-pipeline pattern as board/queue

7. **`cmd/gromit/unstick.go`** — Thin CLI wrapper + simple stdin interactive picker (numbered list); defines `unstickExecutor` interface for TUI wiring

### Integration Points

- `internal/pipeline/queue.go` — `Queue()` loads restart points and passes them to `PartitionQueueBeads`
- Gate's `stuck StuckDetector` interface — Currently unwired in constructor; this plan wires it
- `internal/runner/constructor.go` — Auto-unstick check runs inside `stuckDetectorAdapter.IsStuck` before threshold evaluation, or as a pre-gate hook; keeps orchestrator.go untouched

### Data Flow

**Manual unstick:**
1. `gromit unstick <id>` → `Pipeline.Unstick()` → validates bead exists → writes restart point to `.gromit/restart-points.json` → emits `BeadUnstickedEvent`
2. Next `gromit run` → `stuckDetectorAdapter.IsStuck()` → `ReadPerBeadStatsAfter` excludes pre-restart failures → `ThresholdStuckPolicy.IsStuck` returns false → bead proceeds through gate

**Auto unstick:**
1. Each gate call → `stuckDetectorAdapter.IsStuck()` runs `AutoChecker.Check()` for stuck beads → if signal triggered, writes restart point → bead is no longer considered stuck

**Queue display:**
1. `gromit queue` → `Pipeline.Queue()` → loads restart points → `ReadPerBeadStatsAfter` → `PartitionQueueBeads` → stuck beads with recent restart points appear in Ready

### Tradeoffs

- **Filter at read time vs. store filtered count**: Filter-at-read from JSONL logs. Slightly slower for large log sets but ensures accuracy and avoids double-bookkeeping.
- **New `internal/unstick` package vs. extending `logger`**: New package keeps restart point ownership clean; `logger` stays focused on JSONL I/O.
- **Simple stdin picker vs. bubbletea TUI**: Numbered-list stdin picker for v1. `unstickExecutor` interface makes future TUI wiring trivial.
- **Auto-check location**: Inside `stuckDetectorAdapter.IsStuck` rather than a separate orchestrator hook. Gate remains self-contained; orchestrator.go untouched.

---

## Test Strategy

### Test Levels

1. **Unit tests**: Core logic in isolation (store, logger filter, auto-checker signals)
2. **Integration tests**: Pipeline-level and CLI-level with mock deps
3. **Manual**: Interactive picker and auto-unstick signals verified by hand

### Key Test Cases

**`internal/unstick/store_test.go`:**
- Load from missing file returns empty store (no error)
- Set + Save + Load round-trips `RestartAt` and `Reason` correctly
- `All()` returns correct `map[string]time.Time`

**`internal/unstick/auto_test.go`:**
- Closed dependency → restart point with reason `"dep_closed"`
- Open dependency → no restart point
- Description changed since last failure → restart with reason `"metadata_changed"`
- Comments added since last failure → restart with reason `"metadata_changed"`
- New git commits since last failure → restart with reason `"new_commits"`
- No new commits → no restart
- Bead with restart point newer than last failure → skipped (no duplicate)

**`internal/logger/logger_test.go`:**
- `ReadPerBeadStatsAfter` with empty map = same as `ReadPerBeadStats`
- Failures before restart point excluded from counts
- Failures after restart point included
- Bead not in `after` map uses all failures

**`internal/queue/snapshot_test.go`:**
- `FindStuckBeadIDs` with no restart points: existing behavior unchanged
- Restart point set, post-restart failures below threshold: not stuck
- Restart point set, post-restart failures at threshold: still stuck

**`internal/pipeline/unstick_test.go`:**
- `ListStuck` returns only stuck beads (respects restart points)
- `Unstick` with valid ID writes restart point and emits `BeadUnstickedEvent`
- `Unstick` with unknown ID returns error
- `Unstick` is idempotent (updates timestamp)

**`cmd/gromit/unstick_test.go`:**
- `gromit unstick <id>` calls `Unstick` and prints confirmation
- `gromit unstick` with no stuck beads prints "No stuck beads"
- `gromit unstick <unknown-id>` surfaces error

### Mocking Strategy

- `unstick/auto_test.go`: Mock `bead.Client` and git runner; real `Store` with temp dir
- `pipeline/unstick_test.go`: Mock `QueueClient` and `RestartPointStore`; mock emitter for event assertion
- `logger_test.go`: Real JSONL files in temp dir (existing pattern)
- `cmd/gromit/unstick_test.go`: Mock `unstickExecutor` interface

### Coverage Goals

- Critical: manual unstick writes restart point → gate allows bead through on next run
- Critical: bead re-sticks after unstick (new failures hit threshold again) → gate blocks correctly
- Edge: auto-unstick doesn't re-fire when restart point is newer than last failure

---

## Implementation Tasks

### Task 1: Restart Point Store

**Files:**
- Create: `internal/unstick/store.go`
- Test: `internal/unstick/store_test.go`

**What to Do:**
Create the `internal/unstick` package. Define:
```go
type RestartPoint struct {
    RestartAt time.Time `json:"restart_at"`
    Reason    string    `json:"reason"` // "manual", "dep_closed", "metadata_changed", "new_commits"
}

type Store struct {
    path   string
    points map[string]RestartPoint
}
```
Implement `NewStore(gromitDir string) (*Store, error)`, `Load() error`, `Save() error`, `Set(beadID string, point RestartPoint)`, `Get(beadID string) (RestartPoint, bool)`, and `All() map[string]time.Time` (returns just the timestamps).

**Acceptance Criteria:**
- Load from missing file returns empty store without error
- Set + Save + Load round-trips all fields correctly
- `All()` returns `map[string]time.Time` mapping bead IDs to `RestartAt`

**Dependencies:** None

---

### Task 2: Restart-Aware Stats Filtering in Logger

**Files:**
- Modify: `internal/logger/logger.go`
- Test: `internal/logger/logger_test.go`

**What to Do:**
Add `ReadPerBeadStatsAfter(logsDir string, after map[string]time.Time) (map[string]BeadStats, error)`. Reads all JSONL logs; for each bead in `after`, skips log entries with `Timestamp.Before(after[beadID])`. Reuses `ReadPerBeadStatsFiltered` internals. Beads not in `after` are included in full.

**Acceptance Criteria:**
- Failures before restart point are excluded from `Failures` and `TotalRuns`
- Beads not in `after` use all historical failures
- Empty `after` map produces same result as `ReadPerBeadStats`

**Dependencies:** Task 1 (to understand the `map[string]time.Time` input shape)

---

### Task 3: Queue Display Respects Restart Points

**Files:**
- Modify: `internal/queue/snapshot.go`
- Modify: `internal/pipeline/queue.go`
- Test: `internal/queue/snapshot_test.go`

**What to Do:**
Change `FindStuckBeadIDs(beadStats map[string]logger.BeadStats, threshold int)` to `FindStuckBeadIDs(beadStats map[string]logger.BeadStats, threshold int, restartAfter map[string]time.Time)`. The function already receives pre-filtered `beadStats`, so the `restartAfter` parameter is used to validate that the stats were filtered — or simply pass filtered stats from the call site.

In `pipeline/queue.go`'s `Queue()`: load `Store` using `logsDir` to derive `gromitDir`, call `ReadPerBeadStatsAfter` with `store.All()`, and pass filtered stats to `PartitionQueueBeads`. Add `GromitDir string` to `QueueInput` so the pipeline can locate the store.

**Acceptance Criteria:**
- Bead with restart point and post-restart failures below threshold appears Ready, not Stuck
- Bead with restart point and post-restart failures at threshold still appears Stuck
- Existing behavior unchanged when no restart points exist

**Dependencies:** Tasks 1, 2

---

### Task 4: BeadUnstickedEvent + Gate Wiring

**Files:**
- Modify: `internal/events/types_lifecycle.go`
- Modify: `internal/runner/constructor.go`

**What to Do:**
Add to `types_lifecycle.go`:
```go
type BeadUnstickedEvent struct {
    BeadID string
    Reason string // "manual", "dep_closed", "metadata_changed", "new_commits"
    Time   time.Time
}
func (e *BeadUnstickedEvent) EventType() string { return "bead_unsticked" }
func (e *BeadUnstickedEvent) EventTime() time.Time { ... }
```

In `constructor.go`, create a `stuckDetectorAdapter` struct that implements `prepare.StuckDetector`:
```go
type stuckDetectorAdapter struct {
    logsDir     string
    gromitDir   string
    policy      policy.StuckPolicy
}
func (a *stuckDetectorAdapter) IsStuck(ctx context.Context, b *bead.Bead) (bool, error) {
    store := unstick.NewStore(a.gromitDir); store.Load()
    stats, _ := logger.ReadPerBeadStatsAfter(a.logsDir, store.All())
    return a.policy.IsStuck(b, stats), nil
}
```
Wire into gate: `gateStage.WithStuckDetector(&stuckDetectorAdapter{...})`.

**Acceptance Criteria:**
- `BeadUnstickedEvent.EventType()` returns `"bead_unsticked"`
- Gate blocks a bead with failures >= threshold and 100% failure rate (existing behavior now actually enforced)
- Gate allows a previously-stuck bead through after its restart point is set

**Dependencies:** Tasks 1, 2

---

### Task 5: Auto-Unstick Signal Detection

**Files:**
- Create: `internal/unstick/auto.go`
- Test: `internal/unstick/auto_test.go`

**What to Do:**
Create `AutoChecker` with configurable `BeadClient` (for `Show`/`GetComments`) and `GitLogFn func(since time.Time) (bool, error)` (returns true if any commits exist after `since`).

Method: `Check(ctx context.Context, stuckBeads []*bead.Bead, stats map[string]logger.BeadStats, store *Store, emitter events.EventEmitter) error`

For each stuck bead:
1. Skip if `store.Get(beadID)` returns a restart point newer than `stats[beadID].LastAttempt` (already unstuck, hasn't failed again)
2. Check deps: call `beadClient.Show(dep.ID)` for each dep; if any status is closed → record restart, emit, continue to next bead
3. Check metadata: call `beadClient.Show(beadID)` + `GetComments`; compare updated-at timestamps to `stats.LastAttempt` → if newer, record restart, emit, continue
4. Check commits: call `GitLogFn(stats.LastAttempt)` → if true, record restart, emit

**Acceptance Criteria:**
- Closed dependency triggers restart point with reason `"dep_closed"`
- Changed metadata (description or new comments) triggers reason `"metadata_changed"`
- New commits trigger reason `"new_commits"`
- Bead with restart point newer than last failure is skipped (no duplicate restart)

**Dependencies:** Tasks 1, 4

---

### Task 6: Pipeline Unstick Methods

**Files:**
- Create: `internal/pipeline/unstick.go`
- Test: `internal/pipeline/unstick_test.go`

**What to Do:**
Add to `Pipeline`:

```go
func (p *Pipeline) ListStuck(ctx context.Context, input QueueInput) ([]*bead.Bead, error)
func (p *Pipeline) Unstick(ctx context.Context, beadID, gromitDir string) error
```

`ListStuck` reuses queue logic and returns only the `Stuck` slice.

`Unstick` calls `beadClient.Show(beadID)` to validate existence, loads the restart point store from `gromitDir`, sets `RestartPoint{RestartAt: time.Now(), Reason: "manual"}`, saves, and emits `BeadUnstickedEvent` if an emitter is available.

**Acceptance Criteria:**
- `ListStuck` returns only currently-stuck beads (respects restart points via filtered stats)
- `Unstick` writes restart point and emits `BeadUnstickedEvent`
- `Unstick` with unknown bead ID returns an error

**Dependencies:** Tasks 1, 3, 4

---

### Task 7: CLI Command

**Files:**
- Create: `cmd/gromit/unstick.go`
- Test: `cmd/gromit/unstick_test.go`

**What to Do:**
Define:
```go
type unstickExecutor interface {
    ListStuck(ctx context.Context, input pipeline.QueueInput) ([]*bead.Bead, error)
    Unstick(ctx context.Context, beadID, gromitDir string) error
}
```

Register `unstickCmd` with cobra. With arg: call `Unstick(ctx, args[0], gromitDir)`, print `"Unsticked <id>"`. Without arg: call `ListStuck`; if empty, print `"No stuck beads"`; otherwise print numbered list (`"  1. [P0] <id>  <title>"`), read an integer from stdin, call `Unstick` on the selected bead.

Wire via `createUnstickPipelineFn = createUnstickPipeline` (same pattern as board/queue).

**Acceptance Criteria:**
- `gromit unstick <id>` unsticks bead and prints confirmation
- `gromit unstick` with no stuck beads prints "No stuck beads"
- `gromit unstick` with stuck beads shows numbered list and accepts stdin selection

**Dependencies:** Task 6

---

## Notes

- The gate's `WithStuckDetector` has been in the API since `prepare.Gate` was designed but was never wired in `constructor.go`. Task 4 finally connects it; this may surface previously-invisible stuck-bead blocking behavior. Regression-test `gromit run` with a fresh queue to confirm nothing unexpected is blocked.
- Auto-unstick signals are evaluated lazily (inside `IsStuck`) rather than eagerly at loop start. This means auto-unstick fires the same iteration the gate is evaluating the bead. The bead will be allowed through on that same iteration — it does not need to wait for the next run.
- The git commit check uses `git log --after=<timestamp> --oneline -1` (just needs to know if any commit exists). Keep this simple; file-scoped refinement is out of scope for v1.
- `BeadUnstickedEvent` reason values: `"manual"`, `"dep_closed"`, `"metadata_changed"`, `"new_commits"`.
- `QueueInput` gains a `GromitDir string` field in Task 3; update all callers (`cmd/gromit/queue.go`, any tests).
