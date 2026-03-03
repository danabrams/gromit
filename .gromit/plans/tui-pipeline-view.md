---
created: 2026-03-03T00:00:00Z
decomposed: true
decomposed_at: "2026-03-03T16:55:27-05:00"
id: tui-pipeline-view
source_spec: tui-pipeline-view
---

# TUI Pipeline Management View Implementation Plan

**Goal:** Extend the existing `gromit tui` with a five-tab pipeline management interface for browsing and acting on backlog ideas, specs, plans, and beads.

**Architecture:** Add a top-level tab system wrapping the existing Run Loop views, plus four new pipeline stage tabs with list navigation and action keystrokes. Transition actions (refine/plan/decompose) exit the TUI, run the CLI command, and relaunch.

**Tech Stack:** Go, Bubble Tea (bubbletea), lipgloss (styling)

**Spec:** `.gromit/specs/tui-pipeline-view.md`

---

## Architecture

**Overview:**
Extend the existing TUI Model with a top-level tab system. The current three views (Dashboard, Queue, Conversation) become sub-views of a new "Run Loop" tab. Four new pipeline stage tabs (Backlog, Specs, Plans, Queue) provide browsable lists with action keystrokes. Transition actions use the exit/relaunch pattern via `tea.Quit` with a relaunch signal.

**Key Components:**

1. **Tab System (`internal/tui/tabs.go`)**: Constants for 5 top-level tabs, tab bar renderer, and tab navigation logic. Replaces the current view-switching with two-level navigation: top tabs via left/right arrows, sub-views within Run Loop via 1/2/3.

2. **Pipeline State in Store (`store.go` extension)**: `PipelineItems` struct holding backlog ideas, unplanned specs, undecomposed plans, and partitioned beads. New `HydratePipeline()` method. Existing state untouched.

3. **List Model (`internal/tui/list_model.go`)**: Reusable list component with cursor position, scroll offset, and item rendering. Used by all four pipeline tabs.

4. **Pipeline Tab Views (`internal/tui/view_pipeline.go`)**: Render functions for Backlog, Specs, Plans, and Queue list tabs. Each renders a scrollable list with item summaries.

5. **Hint Bar (`internal/tui/hint_bar.go`)**: Bottom-of-screen renderer showing available keystrokes for the active tab. Context-sensitive.

6. **Action Dispatcher (`internal/tui/actions.go`)**: Handles action keystrokes (r/p/d/v/x/R). For transition actions, sets a "relaunch command" on the Model and returns `tea.Quit`. For view/delete, manages local state.

7. **Exit/Relaunch Loop (`cmd/gromit/tui.go` modification)**: `runTui()` becomes a loop: run tea.Program → check if Model has a pending action → exec CLI command → relaunch TUI. Normal quit exits the loop.

**Integration Points:**
- `model.go` — Replace view-switching with tab system. `currentView` becomes `activeTab` + `runLoopSubView`
- `store.go` — Add PipelineItems struct and hydration
- `cmd/gromit/tui.go` — Relaunch loop and pipeline hydration
- `keymap.go` — Update keybindings for tab navigation and actions

**Files to Modify:**
- `internal/tui/model.go` — Tab system, updated Update/View methods
- `internal/tui/store.go` — PipelineItems state
- `internal/tui/keymap.go` — New keybindings
- `internal/tui/hydration.go` — Pipeline data hydration
- `cmd/gromit/tui.go` — Relaunch loop, pipeline hydration call

**Files to Create:**
- `internal/tui/tabs.go` — Tab constants, tab bar renderer
- `internal/tui/list_model.go` — Reusable list component
- `internal/tui/view_pipeline.go` — Pipeline tab renderers
- `internal/tui/hint_bar.go` — Context-sensitive hint bar
- `internal/tui/actions.go` — Action dispatch and relaunch signaling

**Tradeoffs:**
- **Tab state in Model vs. nested tea.Model**: Flat tab state in Model matches existing codebase style. Avoids over-engineering.
- **Exit/relaunch vs. subprocess**: Full TUI exit + exec gives interactive Claude sessions full terminal control. Matches existing `execGromit()` pattern.
- **Shared list component vs. per-tab lists**: Reusable list model avoids duplicating cursor/scroll logic across 4 tabs.
- **Startup hydration vs. lazy load**: Startup hydration matches existing `HydrateStore` pattern. Manual `R` refresh covers staleness.

---

## Test Strategy

**Test Levels:**

1. **Unit Tests**: Core logic components tested in isolation
   - Tab navigation state transitions (left/right wrapping, active tab tracking)
   - List model cursor movement (up/down, bounds clamping, scroll offset)
   - Hint bar content per tab
   - Action dispatch (correct pending action set per keystroke per tab)
   - Pipeline item rendering (backlog summary, spec name, plan name, bead summary)
   - Store pipeline hydration (PipelineItems populated correctly)

2. **Integration Tests**: Model Update/View cycle with real Store
   - Tab switching produces correct View output (tab bar highlights, correct list rendered)
   - Keystroke sequences: navigate to tab → select item → trigger action → verify pending action + tea.Quit
   - `R` refresh triggers re-hydration and updates displayed items
   - Run Loop tab preserves existing sub-view navigation (1/2/3 keys only active in Run Loop tab)
   - Detail view (`v`) toggles on/off and shows correct content

3. **Manual Testing**: Full interactive flow
   - Exit/relaunch cycle with real commands
   - Terminal rendering at various widths
   - Delete confirmation prompt

**Key Test Cases:**
- Tab navigation wraps: right from Run Loop → Backlog, left from Backlog → Run Loop
- Numeric keys 1/2/3 only switch sub-views in Run Loop tab
- Empty list renders gracefully
- List cursor clamps to bounds
- Action keystrokes ignored when list is empty
- `v` toggles detail view; `v` again or Esc returns to list
- `x` sets confirmation state; `y` confirms, `n`/Esc cancels
- Pending action contains correct command and item identifier
- Pipeline hydration errors produce warnings, not crashes

**Mocking Strategy:**
- Mock `HydrationProvider` for pipeline data (existing pattern)
- Mock backlog `File` with in-memory ideas
- No mocking needed for tab/list/hint logic — pure state machines
- Real Store with injected state for integration tests

**Test Organization:**
- `internal/tui/tabs_test.go` — Tab navigation logic
- `internal/tui/list_model_test.go` — List cursor/scroll logic
- `internal/tui/hint_bar_test.go` — Hint content per tab
- `internal/tui/actions_test.go` — Action dispatch
- `internal/tui/view_pipeline_test.go` — Pipeline view rendering
- `internal/tui/model_test.go` — Integration tests for full Update/View cycle with tabs

---

## Implementation Tasks

### Task 1: Tab System Foundation

**Files:**
- Create: `internal/tui/tabs.go`
- Create: `internal/tui/tabs_test.go`
- Modify: `internal/tui/model.go`

**What to Do:**
Define the five top-level tab constants (`TabBacklog`, `TabSpecs`, `TabPlans`, `TabQueue`, `TabRunLoop`) and a `TabBar` renderer that draws all tab names with the active tab visually highlighted using lipgloss. Update Model to replace `currentView` with `activeTab` (top-level) and `runLoopSubView` (for Dashboard/Queue/Conversation within the Run Loop tab). Add `NextTab()` and `PrevTab()` methods with wrapping. Wire left/right arrow keys in `Update()` to call these methods. Move the existing 1/2/3 key handling to only fire when `activeTab == TabRunLoop`.

**Acceptance Criteria:**
- Left/right arrow keys cycle through 5 tabs with wrapping
- Tab bar renders at top with active tab visually distinct
- 1/2/3 keys only switch sub-views when Run Loop tab is active

**Dependencies:** None (foundation task)

**Notes:** The existing `ViewDashboard`/`ViewQueue`/`ViewConversation` constants become sub-views under `TabRunLoop`. Keep them working exactly as before when in the Run Loop tab.

---

### Task 2: Reusable List Model

**Files:**
- Create: `internal/tui/list_model.go`
- Create: `internal/tui/list_model_test.go`

**What to Do:**
Implement a `ListModel` struct with: `items []ListItem` (interface with `Title() string` and `Summary() string`), `cursor int`, `scrollOffset int`, `viewHeight int`. Methods: `MoveUp()`, `MoveDown()`, `Selected() ListItem`, `SetItems(items)`, `Render(width int) string`. Cursor clamps to `[0, len(items)-1]`. Scroll offset adjusts to keep cursor visible within `viewHeight`. Render produces a line-per-item view with a highlight marker on the cursor row.

**Acceptance Criteria:**
- Cursor moves up/down and clamps at bounds
- Scroll offset keeps cursor visible within view height
- `Selected()` returns the item at cursor position, nil if empty

**Dependencies:** None

**Notes:** Keep this generic — pipeline tabs will wrap their data types in `ListItem` adapters.

---

### Task 3: Pipeline State in Store

**Files:**
- Modify: `internal/tui/store.go`
- Modify: `internal/tui/hydration.go`

**What to Do:**
Add a `PipelineItems` struct to Store with fields: `BacklogIdeas []backlog.Idea`, `UnplannedSpecs []string`, `UndecomposedPlans []string`, `Beads []bead.Bead` (all beads for the queue tab). Add a `SetPipelineItems(items PipelineItems)` method with write lock. Extend `HydrationProvider` interface with `PipelineItems(ctx, gromitDir, specsDir, plansDir) (PipelineItems, error)`. Update `HydrateStore()` to call this and populate the Store. Use the nil-field normalization pattern (empty slices, not nil).

**Acceptance Criteria:**
- `PipelineItems` stored thread-safely with RWMutex
- `HydrateStore()` populates pipeline items from provider
- Hydration errors produce warnings, not crashes

**Dependencies:** None

**Notes:** Follow the existing `NormalizeNilFields()` pattern for the new struct. The `PipelineItems` provider implementation will call `backlog.File.List()`, `pipeline.ListUnplannedSpecs()`, `pipeline.ListUndecomposedPlans()`, and bead listing.

---

### Task 4: Hint Bar

**Files:**
- Create: `internal/tui/hint_bar.go`
- Create: `internal/tui/hint_bar_test.go`

**What to Do:**
Implement `RenderHintBar(activeTab Tab, hasSelection bool, inDetailView bool, inConfirmation bool) string`. Returns a styled string of available keystrokes for the current context. Backlog: `r:Refine  v:View  x:Delete  R:Refresh  q:Quit`. Specs: `p:Plan  v:View  x:Delete  R:Refresh  q:Quit`. Plans: `d:Decompose  v:View  x:Delete  R:Refresh  q:Quit`. Queue: `v:View  R:Refresh  q:Quit`. Run Loop: `1:Dashboard  2:Queue  3:Conversation  R:Refresh  q:Quit`. In detail view: `Esc:Back  q:Quit`. In confirmation: `y:Confirm  n:Cancel`.

**Acceptance Criteria:**
- Correct hint text rendered for each of the 5 tabs
- Hints change when in detail view or confirmation state

**Dependencies:** Task 1 (tab constants)

---

### Task 5: Pipeline Tab View Renderers

**Files:**
- Create: `internal/tui/view_pipeline.go`
- Create: `internal/tui/view_pipeline_test.go`

**What to Do:**
Implement render functions for each pipeline tab: `RenderBacklogTab(store, listModel, width, inDetailView, selectedItem)`, `RenderSpecsTab(...)`, `RenderPlansTab(...)`, `RenderQueueTab(...)`. Each renders the list via `ListModel.Render()` in normal mode, or shows full item content in detail view mode. Backlog items show truncated text + type. Specs show filename without `.md`. Plans show filename without `.md`. Queue beads show ID + title + status. Implement `ListItem` adapters for each data type.

**Acceptance Criteria:**
- Each tab renders its item list with correct summary format
- Detail view shows full content for the selected item
- Empty lists render a "No items" message

**Dependencies:** Task 2 (list model), Task 3 (store pipeline items)

---

### Task 6: Action Dispatcher

**Files:**
- Create: `internal/tui/actions.go`
- Create: `internal/tui/actions_test.go`
- Modify: `internal/tui/model.go`

**What to Do:**
Add `PendingAction` struct to Model with fields: `Command string` (e.g., "refine"), `Args []string` (e.g., idea ID or spec name). Add `detailView bool` and `confirmDelete bool` state to Model. Implement `handleAction(key string, activeTab Tab, selectedItem ListItem) (tea.Model, tea.Cmd)` that: for `r`/`p`/`d` — sets PendingAction and returns `tea.Quit`; for `v` — toggles `detailView`; for `x` — sets `confirmDelete` (then `y` executes delete via Store, `n`/Esc cancels); for `R` — returns a `tea.Cmd` that re-hydrates pipeline items. Wire this into Model.Update() for the pipeline tabs.

**Acceptance Criteria:**
- Transition actions (r/p/d) set correct PendingAction and quit
- `v` toggles detail view on/off
- `x` requires confirmation before delete

**Dependencies:** Task 1 (tabs in model), Task 2 (list model for selected item), Task 5 (view renderers for detail view)

---

### Task 7: Model Integration — View and Update Wiring

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go` (or create if not exists)

**What to Do:**
Wire everything together in Model. `View()` renders: tab bar (from Task 1) + active tab content (pipeline renderers from Task 5 or existing Run Loop views) + hint bar (from Task 4). `Update()` routes: left/right to tab navigation, up/down to list model when in pipeline tabs, action keys to dispatcher (Task 6), 1/2/3 to Run Loop sub-views only when in Run Loop tab, Esc to exit detail view. Each pipeline tab gets its own `ListModel` instance on the Model. On tab switch, nothing resets — cursor positions preserved.

**Acceptance Criteria:**
- Full View renders tab bar + content + hint bar
- All keystrokes route correctly based on active tab
- Pipeline tab list models maintain state across tab switches

**Dependencies:** Task 1, Task 2, Task 4, Task 5, Task 6

**Notes:** This is the integration task that connects all components. Write integration tests that verify keystroke sequences end-to-end.

---

### Task 8: Exit/Relaunch Loop in CLI

**Files:**
- Modify: `cmd/gromit/tui.go`

**What to Do:**
Change `runTui()` to a loop: (1) hydrate Store with pipeline data, (2) create Model and tea.Program, (3) run Program, (4) check `model.PendingAction` — if set, exec the corresponding `gromit` CLI command (e.g., `gromit refine <id>`) using `os/exec` with stdin/stdout/stderr inherited, (5) after command completes, loop back to step 1 to relaunch TUI with fresh data. If no pending action (normal quit), exit the loop. Handle `R` refresh by sending a `tea.Cmd` that calls `HydrateStore()` and sends a `pipelineRefreshedMsg` to update the Model.

**Acceptance Criteria:**
- TUI relaunches after a transition action completes
- Pipeline data is fresh on each relaunch
- Normal `q` quit exits cleanly without relaunch

**Dependencies:** Task 3 (hydration), Task 6 (PendingAction), Task 7 (model integration)

**Notes:** The exec pattern should give the child process full terminal control. Use `cmd.Stdin = os.Stdin` etc. Match the existing `execGromit()` pattern if one exists.

---

## Notes

- The Run Loop tab should work identically to the current TUI when active. All existing Dashboard/Queue/Conversation views, event handling, and conversation controller logic remain untouched.
- The `PendingAction` pattern is the simplest way to signal the relaunch loop. No channels or callbacks needed — just check the Model field after `p.Run()` returns.
- For the delete confirmation (`x` key), the backlog delete uses `backlog.File.Delete(id)`. Spec/plan delete is `os.Remove(filepath)`. Both should be behind the `y` confirmation.
- The list model should handle the case where items change (e.g., after refresh) by clamping the cursor to the new length.
