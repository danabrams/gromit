---
id: tui-foundation
source_spec: tui-foundation
created: 2026-02-27
decomposed: false
---

# TUI Foundation Implementation Plan

**Goal:** Deliver a production-ready, read-only terminal UI shell (`gromit tui`) with live dashboard and queue views backed by existing status, queue, and typed event primitives.

**Architecture:** Add a Bubble Tea frontend adapter that hydrates from current status/queue snapshots and applies live typed-event updates through an event-stream bridge, without adding mutation/orchestration logic to `cmd/gromit/`.

**Tech Stack:** Go, Cobra, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lip Gloss (`github.com/charmbracelet/lipgloss`), existing `internal/events`, `internal/runner`, `internal/pipeline`, and tracker/bead clients.

**Spec:** `.gromit/specs/tui-foundation.md`

---

## Architecture

**Overview:**
A new `gromit tui` command hosts a Bubble Tea app that reads initial run/queue state and then updates from the existing typed event model. Queue semantics are shared with the existing `gromit queue` command via extracted snapshot logic to prevent behavior drift.

**Key Components:**
1. **`cmd/gromit/tui.go`**: CLI entrypoint that resolves config, starts the TUI program, and wires read-only data adapters.
2. **`internal/tui`**: Bubble Tea model, keymap, layout, focus management, and view renderers (`Dashboard`, `Queue`) plus status bar.
3. **`internal/tui/store`**: In-memory view-model state for run details, queue breakdown, completion history, and subscription health.
4. **`internal/tui/hydration`**: Initial snapshot loader using `runner.ReadStatus`, `pipeline.ReadStatus`, and shared queue snapshot APIs.
5. **`internal/tui/events`**: Typed-event mapping layer translating lifecycle/phase/heartbeat events into store updates.
6. **`internal/queue` shared snapshot package**: Reused ready/blocked/stuck ordering, reason, and partition logic currently embedded in `cmd/gromit/queue.go`.
7. **`internal/events/stream` bridge**: File-backed structured event stream subscriber for cross-process TUI consumption while preserving typed emitter source-of-truth in runner.

**Integration Points:**
- Register a new `tui` subcommand in `cmd/gromit/main.go` without altering existing command behavior.
- Keep run lifecycle and tracker mutation authority in existing runner/pipeline code.
- Move queue state derivation into reusable internal APIs consumed by both CLI queue output and TUI queue view.
- Attach event-stream writer subscriber where orchestrator subscribers are started so TUI can follow live updates.

**Data Flow:**
1. `gromit tui` starts and loads initial state from status + pipeline + queue snapshot readers.
2. TUI renders immediately with non-empty dashboard/queue data where available.
3. TUI tails/subscribes to structured typed-event stream and applies incremental store updates (iteration/phase/heartbeat/completions).
4. Queue section refreshes from shared queue snapshot logic to reflect ready/in-progress/blocked/stuck transitions.
5. User key actions only affect UI state (view switch, panel focus, scroll, quit).

**Files to Modify:**
- `go.mod` / `go.sum` - add Bubble Tea/Lip Gloss dependencies.
- `cmd/gromit/main.go` - register `tui` command.
- `cmd/gromit/queue.go` - consume extracted shared queue snapshot helpers.
- `internal/runner/orchestrator.go` - wire optional event-stream subscriber startup alongside existing subscribers.
- `internal/runner/emitter_wiring_test.go` - extend subscriber startup assertions.

**Files to Create:**
- `cmd/gromit/tui.go`
- `internal/tui/model.go`
- `internal/tui/store.go`
- `internal/tui/keymap.go`
- `internal/tui/view_dashboard.go`
- `internal/tui/view_queue.go`
- `internal/tui/hydration.go`
- `internal/tui/events.go`
- `internal/queue/snapshot.go`
- `internal/events/stream/subscriber.go`
- `internal/events/stream/subscriber_test.go`

**Tradeoffs:**
- **Extract shared queue logic** over duplicating queue computation in TUI to keep output semantics consistent across interfaces.
- **File-backed event bridge** over direct cross-process emitter sharing to preserve existing in-process emitter design and keep mutation boundaries unchanged.
- **Read-only TUI first** over immediate run controls to reduce risk and stabilize event-driven rendering and layout ergonomics.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: TUI state reducer/update logic, key handling, typed-event mapping, queue partition/reason functions.
2. **Integration Tests**: command wiring for `gromit tui`, hydration of startup state, event stream subscription lifecycle.
3. **Manual Testing**: terminal layout behavior on desktop/mobile terminal sizes, key navigation, view switching, clean quit during active run.

**Key Test Cases:**
- Initial render includes hydrated run + queue state before live events arrive.
- Dashboard switches between running/not-running states from lifecycle events.
- Queue view shows ready order and blocked/stuck sections with reason text parity.
- Heartbeat events update activity/last-event indicators without mutating tracker/run.
- Completed-bead list is bounded and retains newest-first ordering.
- Keyboard controls (`tab`/focus move, view switch keys, quit) are shown in status bar and function.
- Event stream disconnect/EOF marks subscription health and continues in stable read-only mode.
- Existing `gromit queue` and `gromit status` command behavior remains unchanged.

**Mocking Strategy:**
- Mock hydration providers for deterministic startup-state tests.
- Use synthetic typed events for reducer-level tests.
- Use temp file-backed event stream in subscriber integration tests.
- Keep queue snapshot tests mostly pure with bead fixtures to lock semantics.

**Coverage Goals:**
- Critical paths: startup hydration, event ingestion, UI navigation, graceful shutdown.
- Edge cases: empty repos/no status, unknown events, malformed stream entries, long queue lists with scroll bounds.

**Test Organization:**
- `cmd/gromit/tui_test.go`
- `internal/tui/model_test.go`
- `internal/tui/events_test.go`
- `internal/tui/hydration_test.go`
- `internal/tui/view_dashboard_test.go`
- `internal/tui/view_queue_test.go`
- `internal/queue/snapshot_test.go`
- `internal/events/stream/subscriber_test.go`
- targeted updates in `cmd/gromit/queue_test.go` and `internal/runner/emitter_wiring_test.go`

## Implementation Tasks

### Task 1: Add TUI Command Surface and Dependencies

**Files:**
- Modify: `go.mod`
- Modify: `cmd/gromit/main.go`
- Create: `cmd/gromit/tui.go`
- Test: `cmd/gromit/tui_test.go`

**What to Do:**
Add Bubble Tea dependencies and a new `gromit tui` command that launches a read-only TUI program entrypoint. Ensure command wiring does not alter existing `run`, `status`, or `queue` behavior.

**Acceptance Criteria:**
- `gromit tui --help` is present and stable under root command output.
- TUI command launches and exits cleanly on quit key without invoking run/queue mutations.
- Existing CLI command help/tests remain green.

**Dependencies:**
- None

**Notes:**
Keep startup wiring thin; core state/render logic belongs under `internal/tui`.

### Task 2: Extract Shared Queue Snapshot Logic

**Files:**
- Create: `internal/queue/snapshot.go`
- Modify: `cmd/gromit/queue.go`
- Test: `internal/queue/snapshot_test.go`
- Test: `cmd/gromit/queue_test.go`

**What to Do:**
Move queue partitioning/order/reason logic into `internal/queue` so CLI and TUI share one implementation for ready/blocked/stuck semantics and spec grouping metadata.

**Acceptance Criteria:**
- Shared snapshot API returns ready/blocked/stuck sections consistent with current `queue` output semantics.
- `cmd/gromit/queue.go` uses shared snapshot package without output regression.
- Blocked reason text and stuck detection behavior match current expectations.

**Dependencies:**
- Task 1

**Notes:**
Preserve output formatting in `cmd/gromit/queue.go`; only move derivation logic.

### Task 3: Add Structured Event Stream Bridge for Cross-Process TUI

**Files:**
- Create: `internal/events/stream/subscriber.go`
- Test: `internal/events/stream/subscriber_test.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/emitter_wiring_test.go`

**What to Do:**
Add a subscriber that serializes typed events to a structured stream file and wire it into orchestrator subscriber startup so other processes (TUI) can consume live events without changing pipeline orchestration logic.

**Acceptance Criteria:**
- Typed events emitted by orchestrator are appended to structured stream entries.
- Subscriber startup/shutdown follows context cancellation and emitter close semantics.
- Existing CLI/status subscriber behavior remains intact.

**Dependencies:**
- Task 1

**Notes:**
Use additive wiring; no breaking changes to in-process emitter contracts.

### Task 4: Implement TUI Hydration Adapters

**Files:**
- Create: `internal/tui/hydration.go`
- Modify: `internal/tui/store.go`
- Test: `internal/tui/hydration_test.go`

**What to Do:**
Implement startup snapshot loading from `runner.ReadStatus`, `pipeline.ReadStatus`, and shared queue snapshot functions. Populate dashboard and queue store fields before starting event consumption.

**Acceptance Criteria:**
- Initial UI state reflects current run and queue state when available.
- Empty-state rendering is graceful when status/queue data is absent.
- Hydration errors degrade to warnings/state flags rather than crashing UI.

**Dependencies:**
- Task 2

**Notes:**
Track hydration timestamp to support staleness indicators in status bar.

### Task 5: Build TUI Store and Event Reducer

**Files:**
- Create: `internal/tui/store.go`
- Create: `internal/tui/events.go`
- Test: `internal/tui/events_test.go`

**What to Do:**
Define UI state model and reducer functions that map typed lifecycle/phase/heartbeat events into run progress, active phase/bead, recent completions, and last-event health indicators.

**Acceptance Criteria:**
- Event application updates dashboard fields deterministically.
- Recent completed beads list is bounded for terminal readability.
- Unknown events are ignored safely without panics.

**Dependencies:**
- Task 3
- Task 4

**Notes:**
Keep reducer pure where possible to simplify tests.

### Task 6: Implement Bubble Tea Model, Layout, and Keyboard Navigation

**Files:**
- Create: `internal/tui/model.go`
- Create: `internal/tui/keymap.go`
- Test: `internal/tui/model_test.go`

**What to Do:**
Build the Bubble Tea model/update loop with multi-panel layout, focus movement, view switching (`Dashboard`, `Queue`), scroll handling, and persistent status bar key hints.

**Acceptance Criteria:**
- Keyboard navigation supports at minimum focus move, view switch, scroll, and quit.
- Status bar documents active keybindings and connection health.
- Quit exits cleanly without side effects on active runs.

**Dependencies:**
- Task 5

**Notes:**
Use fixed minimum widths and responsive fallback for narrow terminals.

### Task 7: Implement Dashboard and Queue Views

**Files:**
- Create: `internal/tui/view_dashboard.go`
- Create: `internal/tui/view_queue.go`
- Test: `internal/tui/view_dashboard_test.go`
- Test: `internal/tui/view_queue_test.go`

**What to Do:**
Render the two primary views using store state. Dashboard must show run progress, queue depth summary, recent completions, and event/subscription health. Queue view must show ready order and blocked/stuck sections with reason text.

**Acceptance Criteria:**
- Dashboard includes all required sections from spec acceptance criteria.
- Queue view displays ready order and blocked/stuck content when present.
- Rendering stays readable under bounded terminal widths.

**Dependencies:**
- Task 2
- Task 6

**Notes:**
Prefer stable line ordering for deterministic golden/unit assertions.

### Task 8: Final Validation and Compatibility Regression Pass

**Files:**
- Modify: `cmd/gromit/queue_test.go`
- Modify: `cmd/gromit/status_test.go`
- Modify: `cmd/gromit/cli_contract_test.go`
- Test: targeted `internal/tui` and `internal/events/stream` tests

**What to Do:**
Run focused and package-level quality gates, add explicit regression assertions that non-TUI command paths are unchanged, and validate no run/tracker mutation APIs are invoked by TUI code.

**Acceptance Criteria:**
- TUI tests pass with deterministic behavior for hydration, events, and navigation.
- Existing status/queue command tests pass unchanged semantically.
- No tracker mutation or run lifecycle control is triggered by TUI path.

**Dependencies:**
- Tasks 1-7

**Notes:**
Document any unrelated pre-existing failures separately from TUI regressions.

---

## Notes

- This plan intentionally preserves the thin-wrapper architecture: TUI is a frontend adapter only.
- Follow-on phases can add mutating controls after the read-only event/render foundation proves stable.
- Keep queue semantics centralized to avoid divergence between text CLI and TUI surfaces.
