---
id: tui-pipeline-view
source_ideas: []
created: 2026-03-02
deprecated: 2026-03-05
deprecated_reason: "TUI interface removed — internal/tui/ deleted per docs/plans/2026-03-04-deprecate-tui.md"
accepted: true
---

# TUI Pipeline Management View

## Specification

The existing `gromit tui` command is extended with a tabbed pipeline management interface. The TUI provides five top-level tabs navigated with left/right arrow keys:

1. **Backlog** — unrefined ideas from `backlog.jsonl`
2. **Specs** — refined specs that have no corresponding plan
3. **Plans** — plans that have not been decomposed into beads
4. **Queue** — decomposed beads (ready/blocked/stuck)
5. **Run Loop** — active run monitoring (existing Dashboard, Queue, and Conversation sub-views)

### Tab Navigation

Left/right arrow keys switch between the five tabs. A tab bar at the top shows all tabs with the active tab visually highlighted. The existing numeric key shortcuts (1/2/3) are repurposed or removed in favor of the unified tab system.

### Item Lists (Backlog, Specs, Plans, Queue tabs)

Each pipeline stage tab displays a scrollable list of items in that stage. Up/down arrow keys move a highlight cursor through the list. Each item shows a one-line summary:

- **Backlog**: Idea text (truncated), type (feature/bug/chore)
- **Specs**: Spec name (filename without `.md`)
- **Plans**: Plan name (filename without `.md`)
- **Queue**: Bead ID, title, status (ready/blocked/stuck)

### Actions via Keystroke Hints

A persistent hint bar at the bottom of the screen shows available actions for the current tab and highlighted item. Actions are triggered by single keystrokes:

**Backlog tab:**
- `r` — Refine: launch interactive refine session for the highlighted idea (takes over the TUI inline, returns to pipeline view when done)
- `v` — View: display the full idea text in a detail pane
- `x` — Delete: remove the idea from the backlog (with confirmation)

**Specs tab:**
- `p` — Plan: launch interactive plan session for the highlighted spec
- `v` — View: display the spec content in a detail pane
- `x` — Delete: remove the spec file (with confirmation)

**Plans tab:**
- `d` — Decompose: launch interactive decompose session for the highlighted plan
- `v` — View: display the plan content in a detail pane
- `x` — Delete: remove the plan file (with confirmation)

**Queue tab:**
- `v` — View: display bead details
- (No transition action — beads are executed via the Run Loop)

**All tabs:**
- `q` — Quit
- `Ctrl+C` — Quit

### Inline Session Takeover

When a transition action is triggered (refine, plan, decompose), the TUI exits the Bubble Tea program, runs the corresponding interactive CLI command (e.g., `gromit refine`, `gromit plan`, `gromit decompose`) in the foreground, and relaunches the TUI when the session completes. This avoids the complexity of embedding interactive Claude sessions inside Bubble Tea.

### Run Loop Tab

The Run Loop tab preserves the existing three sub-views (Dashboard, Queue, Conversation) as they work today. Within the Run Loop tab, sub-view navigation uses Tab/Shift+Tab or numeric keys (1/2/3) to switch between Dashboard, Queue, and Conversation panels.

### Data Loading

On startup and when returning from an inline session, the TUI reads pipeline state:
- Backlog items via `backlog.File.List()`
- Unplanned specs via `pipeline.ListUnplannedSpecs()`
- Undecomposed plans via `pipeline.ListUndecomposedPlans()`
- Bead queue via existing Store hydration

A manual refresh keystroke (`R`) reloads pipeline data without restarting the TUI.

## Acceptance Criteria

- Left/right arrow keys navigate between five tabs: Backlog, Specs, Plans, Queue, Run Loop
- Tab bar renders at the top showing all tab names with the active tab visually distinct
- Each pipeline stage tab shows a scrollable list of items with up/down cursor navigation
- Backlog tab lists unrefined ideas with text and type
- Specs tab lists specs that have no corresponding plan
- Plans tab lists plans that have not been decomposed
- Queue tab lists beads with ID, title, and status
- Hint bar at the bottom shows available keystroke actions for the current tab
- `r` on a Backlog item exits TUI, runs refine, and relaunches TUI on completion
- `p` on a Spec exits TUI, runs plan, and relaunches TUI on completion
- `d` on a Plan exits TUI, runs decompose, and relaunches TUI on completion
- `v` on any item shows a detail view of that item's content
- `x` on Backlog/Specs/Plans items deletes with a confirmation prompt
- `R` refreshes pipeline data from disk
- Run Loop tab preserves existing Dashboard, Queue, and Conversation sub-views unchanged
- `q` and `Ctrl+C` quit the TUI from any tab

## Decisions

1. **Left/right tab navigation instead of numeric keys** — Arrow keys scale naturally to five tabs and leave letter keys free for actions. Numeric keys are repurposed for Run Loop sub-view navigation within that tab only.

2. **Inline session via TUI exit/relaunch** — Rather than embedding interactive Claude sessions inside Bubble Tea (which would require complex terminal multiplexing), the TUI exits cleanly, the interactive command runs in the foreground with full terminal control, and the TUI relaunches afterward. This is simpler, more reliable, and gives Claude sessions the full terminal they expect.

3. **Keystroke hints instead of popup menus** — Single-key actions with a persistent hint bar are faster and more discoverable than popup menus. Follows terminal UI conventions (vim, htop, lazygit).

4. **Five tabs with Queue separate from Run Loop** — Queue shows the static bead list for browsing. Run Loop shows the live execution monitoring with Dashboard/Queue/Conversation sub-views. This separates "what's waiting" from "what's happening."

5. **Manual refresh with `R`** — Pipeline data is loaded on startup and after inline sessions. An explicit refresh avoids polling overhead and filesystem watches while keeping the view current on demand.

## Research & Context

### Current State

The existing TUI lives in `internal/tui/` with:
- `model.go` — Bubble Tea Model with three views (Dashboard, Queue, Conversation) switched by numeric keys
- `store.go` — Thread-safe Store with DashboardState, QueueState, ConversationState, fed by pipeline events
- `view_dashboard.go`, `view_queue.go`, `view_conversation.go` — View renderers
- `events.go` — Conversation event mapping to Bubble Tea messages

The `cmd/gromit/tui.go` command creates a Store, Model, and tea.Program.

### Pipeline Data Sources

Pipeline state is already queryable:
- `pipeline.ReadStatus()` returns `PipelineStatus` with unrefined counts, unplanned specs, undecomposed plans, and bead counts
- `pipeline.ListUnplannedSpecs()` and `pipeline.ListUndecomposedPlans()` return filename lists
- `backlog.File.List()` returns all backlog ideas with status
- Bead data comes through `bead.Client` methods

### Existing Patterns

- CLI commands follow the thin wrapper pattern: flag parsing in `cmd/gromit/`, business logic in `internal/pipeline/`
- The Store uses `sync.RWMutex` with snapshot methods for thread-safe reads
- Bubble Tea's `tea.Quit` command cleanly exits the program, making the exit/relaunch pattern straightforward
