---
created: 2026-02-28T00:00:00Z
decomposed: true
decomposed_at: "2026-02-28T04:09:44Z"
id: tui-explore-refine-conversation-panel
source_spec: tui-explore-refine-conversation-panel
---

# TUI Explore/Refine Conversation Panel Implementation Plan

**Goal:** Add an in-TUI conversation panel for `explore` and `refine` that streams assistant output as chat, surfaces tool activity as status indicators, and supports follow-up input and cancel without dropping to an embedded terminal.

**Architecture:** Extend the existing Bubble Tea TUI with a conversation state machine/view and introduce session-oriented explore/refine pipeline adapters that expose structured streaming events from Claude `stream-json`, while keeping existing non-TUI command behavior unchanged.

**Tech Stack:** Go, Cobra, Bubble Tea, existing `internal/tui` store/view model, `internal/pipeline` orchestration, `internal/claude` stream parsing callbacks.

**Spec:** `.gromit/specs/tui-explore-refine-conversation-panel.md`

---

## Architecture

**Overview:**
Add a conversation-focused TUI adapter that consumes structured streaming events from new explore/refine session APIs in the pipeline layer. The UI renders transcript and lifecycle state directly from typed events, and tool activity remains out-of-band status metadata.

**Key Components:**
1. **`internal/tui` conversation state and rendering**: transcript rows, tool-status feed, lifecycle status, input/edit state, and panel focus integration.
2. **TUI conversation controller**: starts sessions, forwards follow-up messages, handles cancellation, maps async session events into Bubble Tea messages.
3. **Pipeline conversation session APIs**: new session-oriented entrypoints for `explore` and `refine` that expose streaming lifecycle/events (start, output, tool wait, completion, failure, cancel).
4. **Claude stream event mapping seam**: reuse `StreamRun` event/tool callbacks to produce deterministic conversation events (assistant chunks + tool activity + terminal result/failure).
5. **Command wiring**: `gromit tui` launches the production model/store stack instead of the current stub, with additive wiring only.

**Integration Points:**
- Extend `internal/tui/model.go`, `internal/tui/store.go`, and key/view renderers to include a conversation panel accessible by keyboard navigation.
- Add pipeline-level conversation primitives for TUI usage without replacing existing synchronous `Explore`/`Refine` command flows.
- Keep artifact creation and post-session post-processing in pipeline code; TUI remains presentation/control adapter.
- Keep existing non-TUI `gromit explore` and `gromit refine` behavior unchanged.

**Data Flow:**
1. User selects `explore` or `refine` from the conversation panel and submits an initial prompt.
2. TUI calls pipeline conversation start; lifecycle transitions `idle -> starting`.
3. Pipeline launches a stream-backed session and emits typed events.
4. TUI appends user/assistant transcript entries and updates lifecycle `streaming` / `waiting_for_tool` based on events.
5. Tool-use events are recorded as ephemeral status indicators associated with timestamps, not transcript text.
6. User sends follow-up prompt via the panel; same active session receives new input.
7. User cancel triggers session cancellation; lifecycle becomes `cancelled` and transcript remains visible.
8. Completion/failure produces terminal status row while preserving prior conversation transcript.

**Files to Modify:**
- `cmd/gromit/tui.go` - replace stub entrypoint with wired Bubble Tea model using real store/controller dependencies.
- `internal/tui/model.go` - add conversation view selection, key handling for send/cancel/start, and async message handling.
- `internal/tui/store.go` - add conversation state models, lifecycle enum, transcript append/update logic, tool-status tracking.
- `internal/tui/keymap.go` - add explicit bindings for conversation actions and panel/view focus docs.
- `internal/tui/events.go` - map pipeline conversation events into UI-focused state update structures.
- `internal/pipeline/types.go` - add typed conversation event/session interfaces used by TUI explore/refine sessions.
- `internal/pipeline/explore.go` - add conversation-oriented explore session entrypoint.
- `internal/pipeline/refine.go` - add conversation-oriented refine session entrypoint.
- `internal/claude/claude.go` - expose/refactor stream-json mapping helpers needed for robust typed conversation events.

**Files to Create:**
- `internal/tui/view_conversation.go` - transcript, lifecycle header, tool indicators, and input area renderer.
- `internal/tui/conversation_controller.go` - orchestrates session lifecycle and bridges async events to Bubble Tea messages.
- `internal/pipeline/conversation.go` - shared session scaffolding/event channels for explore/refine interactive streaming.
- `internal/tui/conversation_test_helpers.go` (optional) - reusable fake sessions/event fixtures for integration-style model tests.

**Tradeoffs:**
- **Session-oriented pipeline API over direct agent launch in TUI**: required for follow-up input, cancellation, and deterministic streaming state.
- **Structured event rendering over terminal passthrough**: satisfies readability and testability requirements; avoids terminal-noise leakage.
- **In-memory transcript for first version over cross-launch persistence**: minimizes scope while delivering core UX and preserves future extension path.
- **Ephemeral tool-status feed over transcript injection**: keeps conversational content clean and aligns with spec decisions.

## Test Strategy

**Approach:**
Adopt an automated-first validation strategy that replaces most manual checks with deterministic state-sequence tests, race/cancellation coverage, and one process-level stream integration test.

**Test Levels:**
1. **Reducer/Unit Tests**: lifecycle transitions, transcript chunking/append logic, tool-status mapping and expiry behavior.
2. **Bubble Tea Interaction Tests**: scripted key/input flows through `tea.NewProgram(...WithInput/WithOutput...)` asserting render snapshots across state transitions.
3. **Pipeline/Claude Integration Tests**: fixture-driven stream-json event sequences and one subprocess-backed fake Claude binary test for end-to-end parsing/wiring.
4. **Concurrency/Race Tests**: cancel-during-stream, follow-up-during-tool-wait, late events after cancellation.

**Key Test Cases:**
- Starting a panel session transitions `idle -> starting -> streaming` with first assistant chunk.
- Assistant content streams incrementally into a single evolving assistant message for the active turn.
- Tool-use events produce status indicators (`name + phase/status`) without appearing in transcript body text.
- `waiting_for_tool` transitions are emitted and then return to `streaming` when assistant text resumes.
- Follow-up message submission appends a new user entry and triggers subsequent assistant streaming in same session.
- Cancel while streaming halts event consumption promptly and transitions to `cancelled` without panic/deadlock.
- Provider/process failures surface actionable failure state and preserve all prior transcript rows.
- Completed transcript remains visible until user starts a new session.
- Existing `gromit explore` and `gromit refine` command tests remain unchanged and passing.

**Mocking Strategy:**
- Use a fake `ConversationSession` implementation in TUI tests that emits deterministic event timelines over channels.
- Use stream-json fixture lines for Claude mapping tests (assistant text blocks, tool_use blocks, result/error).
- Add a fake CLI subprocess test harness that emits real stdout stream-json timing to validate parser/controller wiring end-to-end.

**Coverage Goals:**
- Critical paths: start, stream, tool-status display, follow-up send, cancellation, failure handling, completion retention.
- Robustness paths: event ordering anomalies, empty chunks, duplicate tool events, cancel-after-terminal-state no-op behavior.
- Concurrency safety: no goroutine leaks, no data races under `go test -race` for touched packages.

**Test Organization:**
- `internal/tui/model_test.go` - conversation key routing and view/panel focus behavior.
- `internal/tui/view_conversation_test.go` - rendering contract and transcript/tool separation.
- `internal/tui/store_test.go` or expanded `store` tests - lifecycle/transcript/tool status reducers.
- `internal/tui/conversation_controller_test.go` - async command/event bridge behavior.
- `internal/pipeline/conversation_test.go` - explore/refine session API behavior and cancellation semantics.
- `internal/claude/claude_test.go` additions - stream-json mapping coverage for conversation event extraction.
- A targeted integration test for `cmd/gromit/tui.go` wiring with fakes.

**Minimal Manual Residual:**
Keep only a short terminal ergonomics smoke pass (resize, scroll feel, focus visibility) since those are difficult to fully assert headlessly.

## Implementation Tasks

### Task 1: Add Conversation Domain Types and Session Contracts

**Files:**
- Modify: `internal/pipeline/types.go`
- Create: `internal/pipeline/conversation.go`
- Test: `internal/pipeline/conversation_test.go`

**What to Do:**
Define typed conversation lifecycle/events and a session interface that supports streaming output, follow-up input, cancel, and wait semantics for TUI usage. Implement shared session scaffolding for explore/refine orchestration.

**Acceptance Criteria:**
- Pipeline exposes typed conversation events covering lifecycle, assistant output chunks, tool status, and terminal states.
- Session contract supports send-input and cancel safely during active streaming.
- Unit tests validate event ordering and terminal-state guarantees.

**Dependencies:**
- None

**Notes:**
Design this as additive API surface so existing non-TUI calls remain untouched.

### Task 2: Implement Explore/Refine Conversation Session Adapters

**Files:**
- Modify: `internal/pipeline/explore.go`
- Modify: `internal/pipeline/refine.go`
- Modify: `internal/claude/claude.go`
- Test: `internal/pipeline/explore_test.go`
- Test: `internal/pipeline/refine_test.go`
- Test: `internal/claude/claude_test.go`

**What to Do:**
Add TUI-oriented conversation start APIs for explore/refine that use Claude stream-json callbacks and emit typed conversation events, including tool-use status and failure/cancel classification.

**Acceptance Criteria:**
- Explore/refine conversation sessions stream assistant text and tool status via typed events.
- Cancel interrupts streaming promptly and yields `cancelled` terminal state.
- Failure events preserve prior transcript output and include actionable error status content.

**Dependencies:**
- Task 1

**Notes:**
Avoid behavior changes to existing synchronous `Explore`/`Refine` command paths.

### Task 3: Extend TUI Store and Event Mapping for Conversation State

**Files:**
- Modify: `internal/tui/store.go`
- Modify: `internal/tui/events.go`
- Test: `internal/tui/events_test.go`
- Test: `internal/tui/store_mutex_test.go`

**What to Do:**
Introduce conversation state in the store (transcript rows, lifecycle status, tool indicators, active mode/session metadata) and event reducers/mappers that apply pipeline conversation events deterministically.

**Acceptance Criteria:**
- Store represents lifecycle states: `idle`, `starting`, `streaming`, `waiting_for_tool`, `completed`, `failed`, `cancelled`.
- Transcript append/update logic is stable under chunked assistant streaming.
- Tool indicators are stored separately from message text and can be rendered independently.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Preserve existing dashboard/queue state behavior and locking guarantees.

### Task 4: Build Conversation Panel View and Input UX

**Files:**
- Create: `internal/tui/view_conversation.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/view_conversation_test.go`
- Test: `internal/tui/model_test.go`

**What to Do:**
Add conversation panel rendering and keyboard/input interactions: start explore/refine session, submit follow-up prompts, cancel session, and focus navigation with visible status and transcript.

**Acceptance Criteria:**
- Conversation panel is focusable via keyboard navigation and integrated with existing views/panels.
- Assistant text streams visibly in chat form while lifecycle and tool-status indicators update live.
- Users can submit at least one follow-up message and cancel an active session from within TUI.

**Dependencies:**
- Task 3

**Notes:**
Keep rendering deterministic and testable with textual output assertions.

### Task 5: Wire TUI Command to Production Conversation-Capable Model

**Files:**
- Modify: `cmd/gromit/tui.go`
- Test: `cmd/gromit/tui_test.go`

**What to Do:**
Replace stub model bootstrapping with the real TUI model/store/controller wiring, including dependencies required to launch explore/refine conversation sessions.

**Acceptance Criteria:**
- `gromit tui` launches the conversation-capable app instead of the placeholder stub.
- Command wiring remains additive and does not regress other root command behavior.
- Basic command-level tests confirm registration and launch wiring.

**Dependencies:**
- Task 2
- Task 3
- Task 4

**Notes:**
Keep dependency construction thin in command layer; place behavior in internal packages.

### Task 6: Automated Interaction and Concurrency Harness

**Files:**
- Create: `internal/tui/conversation_controller_test.go`
- Create: `internal/tui/conversation_test_helpers.go` (if needed)
- Modify: `internal/tui/model_test.go`
- Modify: `internal/pipeline/conversation_test.go`

**What to Do:**
Implement scripted Bubble Tea interaction tests and race-focused concurrency tests that validate behavior over time, including cancel and follow-up edge cases.

**Acceptance Criteria:**
- Scripted integration tests verify render/state sequences across start, stream, tool wait, follow-up, completion, failure, and cancel.
- Race-targeted tests cover cancellation and concurrent input paths without data races.
- Test harness is deterministic and suitable for CI.

**Dependencies:**
- Task 2
- Task 3
- Task 4

**Notes:**
Prefer fixture-driven event timelines over sleeps where possible.

### Task 7: Regression Gate for Non-TUI Explore/Refine Behavior

**Files:**
- Modify: `cmd/gromit/explore_test.go`
- Modify: `cmd/gromit/refine_test.go`
- Modify: `internal/pipeline/explore_test.go`
- Modify: `internal/pipeline/refine_test.go`

**What to Do:**
Add or update regression tests ensuring new session APIs do not change existing non-TUI command behavior or artifact contracts.

**Acceptance Criteria:**
- Existing explore/refine CLI behavior remains unchanged from user perspective.
- Artifact creation/post-processing assertions for non-TUI paths remain green.
- New conversation plumbing is confirmed additive by tests.

**Dependencies:**
- Task 2

**Notes:**
This task protects scope boundary and avoids accidental contract drift.

---

## Notes

- Open question resolved for this phase: use an in-memory transient tool-status feed associated with timestamps; do not persist status entries into transcript body text.
- Transcript persistence across TUI launches is intentionally deferred; keep in-memory only for this implementation.
- Run `go test ./...` and `go test -race ./internal/tui ./internal/pipeline` as quality gates before decomposition/implementation handoff.
