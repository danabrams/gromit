---
id: tui-foundation
source_ideas: []
created: 2026-02-27
epic: multi-interface-architecture
deprecated: 2026-03-05
deprecated_reason: "TUI interface removed — internal/tui/ deleted per docs/plans/2026-03-04-deprecate-tui.md"
---

# TUI Foundation

## Specification

Build the first production-ready terminal UI shell for Gromit using Bubble Tea. This phase delivers a read-only interface that sits on top of the existing pipeline/event architecture and thin command wrappers, without introducing new orchestration logic in `cmd/gromit/`.

The TUI provides:
- A multi-panel layout with a persistent status bar.
- Keyboard navigation between panels and switchable primary views.
- A live dashboard view showing run progress and recently completed work.
- A live queue view showing queue depth and queue composition.

The TUI is a frontend adapter over existing backend primitives:
- It subscribes to the structured event stream defined by the event-system spec.
- It reads current status/queue snapshots from existing runner and bead state sources for initial hydration.
- It does not run pipeline commands directly in this phase and does not mutate tracker state.

### User Workflows

1. User starts the TUI and immediately sees current run state and queue state.
2. User switches between dashboard and queue views using keyboard shortcuts.
3. While a run is active, the dashboard updates live from emitted events (iteration, phase, heartbeat, completion/failure).
4. Queue depth and breakdown update as beads move between ready/in-progress/blocked/completed states.
5. User can quit cleanly at any time without affecting active runs.

### Dashboard View Scope

The dashboard includes at minimum:
- Current run status (running/not running, iteration progress, active bead, active phase).
- Queue depth summary (ready, in-progress, blocked, stuck, completed counts where available).
- Recently completed beads list (bounded list for terminal readability).
- Last event timestamp and connection/subscription health indicator.

### Queue View Scope

The queue view includes at minimum:
- Ready beads in processing order.
- Grouping or labels needed to identify spec affinity.
- Blocked and stuck sections with reason text when available.
- Model assignment display consistent with current queue logic.

### Interaction Model

This phase is read-only:
- Allowed actions: navigate panels, switch views, scroll lists, quit.
- Not allowed in this phase: start/stop runs, close/reprioritize beads, trigger decomposition/review/refine from the TUI.

## Acceptance Criteria

- A Bubble Tea-based TUI entrypoint exists and launches successfully from a CLI command without replacing existing non-TUI flows.
- The TUI subscribes to the existing typed event emitter and updates dashboard state from lifecycle, phase, and heartbeat events.
- The initial TUI render hydrates from current status/queue state so users do not see an empty dashboard before the next event.
- The UI exposes at least two switchable views: `Dashboard` and `Queue`.
- Keyboard navigation is documented in the status bar and supports at minimum: view switching, focus movement, and quit.
- Dashboard view visibly shows: run progress, queue depth, and a completed-beads section.
- Queue view visibly shows: ready queue order plus blocked/stuck sections when present.
- The TUI performs no tracker mutations and no run lifecycle mutations in this phase.
- Existing CLI status/queue commands continue to function unchanged (thin-wrapper compatibility preserved).

## Decisions

1. **Read-only first cut**  
   Foundation prioritizes reliable event-driven rendering and layout ergonomics before adding control actions. This reduces risk and keeps phase boundaries clear.

2. **Event-system-first data flow**  
   Live state is driven by typed events instead of parsing terminal output. This directly leverages the event-system spec and keeps UI behavior deterministic and testable.

3. **Thin command wrappers remain authoritative for workflow actions**  
   The TUI in this phase does not bypass command/pipeline boundaries. Mutation workflows stay in existing wrappers until the follow-on TUI workflow phase.

4. **Dashboard + queue together in phase scope**  
   Shipping both views in foundation validates the core layout/navigation model and immediately proves multi-panel utility versus static status output.

## Research & Context

### Current State

- The multi-interface epic defines this as Phase 3 and expects a Bubble Tea frontend adapter:  
  `/home/dabrams/gromit/.-gromit-refine-1772160579158729395/.gromit/epics/multi-interface-architecture.md`
- Typed runtime events already exist and cover lifecycle, phase progress, heartbeat, and decomposition milestones:  
  `/home/dabrams/gromit/.-gromit-refine-1772160579158729395/internal/events/types_phase_progress.go`
- Existing subscribers prove event fan-out patterns for rendering and status persistence:  
  `/home/dabrams/gromit/.-gromit-refine-1772160579158729395/internal/events/cli/subscriber.go`  
  `/home/dabrams/gromit/.-gromit-refine-1772160579158729395/internal/events/status/subscriber.go`
- Current status/queue user-visible semantics are already defined in CLI paths and should be preserved as baseline behavior:  
  `/home/dabrams/gromit/.-gromit-refine-1772160579158729395/internal/runner/display/display.go`  
  `/home/dabrams/gromit/.-gromit-refine-1772160579158729395/internal/runner/print_status.go`  
  `/home/dabrams/gromit/.-gromit-refine-1772160579158729395/cmd/gromit/queue.go`

### Upstream Specs This Builds On

- `event-system`
- `pipeline-extraction`
- `runner-pipeline` (thin command wrapper direction and pipeline staging boundaries)

### Out of Scope for This Spec

- Conversation panels for explore/refine.
- TUI-initiated pipeline mutations (run control, bead edits, decomposition actions).
- API server or web/mobile clients.
