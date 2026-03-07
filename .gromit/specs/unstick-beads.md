---
id: unstick-beads
source_ideas: []
created: 2026-03-02
accepted: true
---

# Unstick Beads

## Specification

Beads that accumulate enough historical failures (default 3, 100% failure rate) are permanently blocked by the gate stage with no way to recover. This feature adds both a manual CLI command and automatic detection to unstick beads so they can be retried.

### Manual Unstick

A new `gromit unstick` subcommand provides a first-class way to unstick beads:

- `gromit unstick <bead-id>` — directly unsticks the specified bead
- `gromit unstick` (no args) — lists all currently stuck beads and presents an interactive picker to choose which one(s) to unstick

When a bead is unsticked, a **restart point** is recorded. Historical failure data is fully preserved for audit purposes, but only failures occurring *after* the restart point count toward the stuck threshold. The bead re-enters the ready pool and gets a fresh set of attempts.

### Automatic Unstick

The system automatically marks a restart point for stuck beads when it detects that conditions have changed. Three signals trigger automatic unstick, in order of signal strength:

1. **Dependency bead closed** — A bead that was blocking the stuck bead has been closed. Strongest signal; direct causal relationship.

2. **Bead metadata changed** — The stuck bead's description, comments, or other metadata have been updated since the last failure. Indicates active human intervention.

3. **New commits** — New commits have landed on the repo since the stuck bead's last failure. In v1, any new commit counts. Future refinement could scope this to commits touching files relevant to the bead.

For all automatic signals: the same restart-point mechanism is used as for manual unstick. If the bead fails again after the restart point and re-hits the threshold, it becomes stuck again — no infinite retry loops.

### Restart Point Mechanism

The restart point is a timestamp recorded per bead. When computing whether a bead is stuck, the `ThresholdStuckPolicy` only considers failures that occurred after the most recent restart point (if one exists). Failures before the restart point are preserved in logs but excluded from the stuck calculation.

## Acceptance Criteria

- `gromit unstick <bead-id>` unsticks a specific bead and it re-enters the ready pool on the next run
- `gromit unstick` with no args displays a list of stuck beads and allows the user to pick one to unstick
- After unsticking, historical failure logs are preserved (no data deleted)
- After unsticking, only failures after the restart point count toward the stuck threshold
- A bead that fails again after being unsticked can become stuck again (no infinite retries)
- Closing a dependency of a stuck bead automatically unsticks it on the next run
- Updating a stuck bead's metadata (description, comments) automatically unsticks it on the next run
- New commits on the repo since a stuck bead's last failure automatically unstick it on the next run
- Automatic unstick events are logged/emitted so the user can see what happened

## Decisions

1. **Restart point over reset** — Rather than wiping failure history or bumping thresholds, we record a restart-point timestamp. Only post-restart failures count toward stuck detection. This preserves the full audit trail while giving beads a clean slate for threshold purposes.

2. **Dedicated subcommand** — `gromit unstick` is a standalone subcommand rather than a flag on `gromit run`. This makes it explicit, discoverable, and scriptable. The no-args interactive picker makes it usable without memorizing bead IDs.

3. **All three automatic signals in v1** — Dependency closure, metadata changes, and new commits are all implemented in v1. Dependency closure and metadata changes are high-confidence signals. New commits are noisier but the worst case is one wasted retry before re-sticking, which is acceptable.

4. **Coarse commit detection in v1** — Any new commit counts as a signal, not just commits touching bead-relevant files. This keeps v1 simple. Scoped commit detection can be refined later if the noise proves problematic.

## Research & Context

### Current State

- **Stuck policy**: `internal/runner/policy/stuck.go` — `ThresholdStuckPolicy` computes stuckness from `logger.BeadStats` (failures >= threshold AND 100% failure rate)
- **Gate stage**: `internal/pipeline/prepare/gate.go` — blocks stuck beads with `pipeline.Block` decision
- **Failure tracking**: `internal/logger/logger.go` — `BeadStats` aggregated from JSONL logs in `.gromit/logs/`, `ReadPerBeadStats()` computes per-bead statistics
- **Events**: `internal/events/types_gate.go` (`GateStuckEvent`) and `internal/events/types_lifecycle.go` (`BeadStuckEvent`) — emitted when stuck beads are detected
- **Config**: `internal/config/config_types.go` — `LoopConfig.StuckBeadThreshold` (default 3)
- **Bead client**: `internal/bead/bead.go` — `Show`, `Close`, `GetComments`, `Ready`, `ReadyExcluding`, etc.
- **CLI commands**: `cmd/gromit/` — one file per subcommand

### Key Integration Points

- The restart point needs to be stored somewhere persistent — likely a new file in `.gromit/` (e.g., `.gromit/restart-points.json`) or as additional metadata in the existing log structure
- `ThresholdStuckPolicy.IsStuck()` needs access to restart points to filter which failures count
- The gate stage's `StuckDetector` interface may need to incorporate automatic unstick signal checking
- Automatic signals need to be evaluated before or during gate execution on each run
- A new event type (e.g., `BeadUnstickedEvent`) should be emitted when a bead is automatically or manually unsticked
