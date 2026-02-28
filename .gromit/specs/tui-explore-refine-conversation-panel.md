---
id: tui-explore-refine-conversation-panel
source_ideas: []
created: 2026-02-28
epic: multi-interface-architecture
---

# TUI Explore/Refine Conversation Panel

## Specification

Add a dedicated conversation panel in the TUI for `explore` and `refine` sessions so users can run and monitor these sessions without dropping into an embedded terminal. The panel presents Claude CLI streaming output as structured chat messages and lightweight session status.

The panel behavior is:
- Shows a chat transcript for the active session (user prompts and assistant responses).
- Streams assistant output incrementally from Claude CLI `stream-json` events.
- Displays tool activity as status indicators (for example: "Reading file", "Searching", "Editing"), not as raw terminal text or command transcripts.
- Shows session lifecycle state (`idle`, `starting`, `streaming`, `waiting_for_tool`, `completed`, `failed`, `cancelled`) in the panel header/status area.
- Supports starting an `explore` or `refine` conversation from the TUI, sending follow-up user messages, and canceling the active session.

### Scope Boundaries

- In scope: conversation UX for `explore` and `refine` only.
- Out of scope: conversation UI for `plan`, `decompose`, `run`, or `review`.
- Out of scope: embedded shell/PTY rendering inside the panel.
- Out of scope: changing the underlying refine/explore artifact contracts (spec creation, backlog updates, etc.).

### Conversation Model

- A conversation is tied to exactly one active refine/explore session in the TUI.
- The transcript is append-only for the duration of the session and remains visible after completion until the user starts a new session.
- Tool indicators are ephemeral status elements associated with message timestamps; they are not injected into message body text.
- Stream interruptions and provider errors are rendered as explicit system-status rows in the panel and do not corrupt prior transcript messages.

## Acceptance Criteria

- The TUI has an explore/refine conversation panel that can be focused via keyboard navigation.
- Starting `explore` or `refine` from the panel launches the existing pipeline flow and renders assistant output as streaming chat text.
- Claude tool use events are shown as status indicators (name + phase/status), and raw tool event JSON/terminal noise is not shown in transcript content.
- The panel visibly transitions through lifecycle states (`starting`, `streaming`, terminal state) based on real session events.
- Users can submit at least one follow-up message in an active session without leaving the TUI.
- Canceling a session from the panel stops streaming promptly and marks the session `cancelled` without crashing the TUI.
- On provider or process failure, the panel shows an actionable failure state and preserves prior transcript content.
- Existing non-TUI `gromit explore` and `gromit refine` command behavior remains unchanged.

## Decisions

1. **Use structured stream events as the rendering source**  
   The panel consumes Claude `stream-json` semantics instead of parsing terminal output, so UI state is deterministic and testable.

2. **Tool calls are represented as status, not transcript text**  
   Tool use is an execution signal, not conversational content. Keeping it out of message bodies improves readability and reduces terminal-noise leakage.

3. **Limit first version to explore/refine sessions**  
   These are the conversational phases in the multi-interface architecture and provide the highest value for validating chat UX before broader phase coverage.

4. **Keep pipeline contracts stable**  
   The panel is a presentation adapter over existing pipeline APIs; artifact creation and post-processing remain owned by pipeline code.

## Research & Context

### Current State

- `cmd/gromit/tui.go` currently launches a stub Bubble Tea app; it does not yet wire conversation workflows.
- `internal/tui/model.go`, `internal/tui/store.go`, and `internal/tui/hydration.go` already define a TUI state model and hydration path for dashboard/queue views.
- `internal/claude/claude.go` already parses `stream-json`, streams assistant text blocks, and emits tool call callbacks (`ToolCallHandler`), which is the right integration seam for conversation and tool-status rendering.
- `internal/pipeline/explore.go` and `internal/pipeline/refine.go` already encapsulate workflow orchestration, so the panel should call these pipeline paths rather than introducing direct CLI orchestration in the TUI.
- `.gromit/specs/tui-foundation.md` explicitly marks conversation panels as out of scope for foundation and identifies this as follow-on phase work under the same epic.

### Dependencies

- `tui-foundation`
- `event-system`
- `pipeline-extraction`

### Open Questions To Resolve In Planning

- Whether tool status indicators should be a transient feed, per-message badges, or both.
- Whether completed transcripts should be persisted between TUI launches or remain in-memory only for this phase.
