---
id: event-system
source_ideas: [idea-1770820549600, idea-1770820550926]
created: 2026-02-11
epic: multi-interface-architecture
---

# Structured Event System

## Specification

Gromit's long-running operations (run loop, decompose, retro, etc.) emit structured, typed events through a channel-based `EventEmitter` rather than writing directly to an `io.Writer`. Each event is a concrete Go struct implementing a common `Event` interface. Subscribers receive events on a dedicated `chan Event` and render or process them however they want.

The event system lives in `internal/events/`. It is the abstraction layer between backend pipeline operations (which emit events) and frontend clients (which consume them). The CLI, TUI, and future API server are all event subscribers — none get privileged access to pipeline internals.

### Event Interface

All events implement:

```go
type Event interface {
    EventType() string
    EventTime() time.Time
}
```

Each concrete event struct carries a type-specific payload with strongly-typed fields. Subscribers use type-switch dispatch to handle events they care about and ignore the rest.

### EventEmitter

The `EventEmitter` manages subscriber registration and event dispatch:

```go
type Emitter struct { ... }

func NewEmitter() *Emitter
func (e *Emitter) Subscribe() <-chan Event      // Returns a new buffered channel
func (e *Emitter) Unsubscribe(ch <-chan Event)   // Removes and closes the channel
func (e *Emitter) Emit(event Event)              // Sends to all subscriber channels
func (e *Emitter) Close()                        // Closes all subscriber channels
```

- `Subscribe()` returns a buffered channel. Buffer size is an internal constant (not configurable by callers) chosen to prevent blocking during bursts.
- `Emit()` is non-blocking — if a subscriber's channel is full, the event is dropped for that subscriber (slow consumer protection). This prevents a stalled TUI panel from blocking the run loop.
- `Close()` closes all subscriber channels, signaling consumers to exit their receive loops.
- Thread-safe: multiple goroutines may call `Emit()` concurrently; `Subscribe()`/`Unsubscribe()` are safe to call during emission.

### No Filtering

Every subscriber receives every event. With ~22 event types and a small number of concurrent subscribers (CLI, TUI, logger), filtering adds complexity without meaningful benefit. Subscribers ignore events they don't care about via type-switch default cases.

### Event Types

#### Lifecycle Events

- **`RunStartEvent`** — Emitted when `gromit run` begins. Fields: `MaxIterations int`, `TimeBudget time.Duration`, `DryRun bool`.
- **`RunCompleteEvent`** — Emitted when the run loop exits. Fields: `IterationsCompleted int`, `Reason string` (e.g., "no work", "budget exhausted", "max iterations").
- **`IterationStartEvent`** — Emitted at the top of each loop iteration. Fields: `Iteration int`, `BeadID string`, `BeadTitle string`.
- **`IterationCompleteEvent`** — Emitted when an iteration finishes (success or failure). Fields: `Iteration int`, `BeadID string`, `Success bool`, `Duration time.Duration`.
- **`BeadCompleteEvent`** — Emitted when a bead is closed as done. Fields: `BeadID string`, `BeadTitle string`, `Duration time.Duration`.
- **`BeadFailedEvent`** — Emitted when a bead fails after exhausting retries/escalation. Fields: `BeadID string`, `BeadTitle string`, `Error string`.
- **`BeadStuckEvent`** — Emitted when a bead is marked as stuck. Fields: `BeadID string`, `BeadTitle string`, `Reason string`.
- **`BeadSkippedEvent`** — Emitted when a bead is skipped (e.g., precheck pass, scope gate block). Fields: `BeadID string`, `Reason string`.

#### Phase Events

- **`BuildStartEvent`** — Emitted before invoking Claude for the build phase. Fields: `BeadID string`, `Model string`, `Attempt int`, `MaxAttempts int`.
- **`BuildCompleteEvent`** — Emitted after Claude returns from the build phase. Fields: `BeadID string`, `Success bool`, `Duration time.Duration`, `Cost float64`, `TokensIn int`, `TokensOut int`.
- **`ValidationStartEvent`** — Emitted before running validation. Fields: `BeadID string`, `Model string`, `Commands []string`.
- **`ValidationPassEvent`** — Emitted when validation succeeds. Fields: `BeadID string`, `Duration time.Duration`.
- **`ValidationFailEvent`** — Emitted when validation fails. Fields: `BeadID string`, `Output string`, `Duration time.Duration`.
- **`ReviewStartEvent`** — Emitted before running post-iteration review. Fields: `BeadID string`, `Model string`, `Thorough bool`.
- **`ReviewCompleteEvent`** — Emitted after review finishes. Fields: `BeadID string`, `Verdict string`, `Issues []string`.
- **`AnalysisStartEvent`** — Emitted before failure analysis. Fields: `BeadID string`.
- **`AnalysisCompleteEvent`** — Emitted after failure analysis. Fields: `BeadID string`, `Category string`, `Recoverable bool`, `Suggestion string`.
- **`RetroStartEvent`** — Emitted before retrospective. Fields: `BeadID string`.
- **`RetroCompleteEvent`** — Emitted after retrospective. Fields: `BeadID string`, `ProvisionalLearnings int`, `RulesUpdated bool`.

#### Progress Events

- **`HeartbeatEvent`** — Emitted periodically during Claude invocations. Fields: `Elapsed time.Duration`, `ToolCalls int`, `FilesModified int`, `RateLimitHits int`, `WaitingForResponse bool`.
- **`ModelSelectedEvent`** — Emitted when a model is chosen. Fields: `Model string`, `Reason string` (e.g., "P0 priority", "complexity:high label", "escalation").
- **`EscalationEvent`** — Emitted when retrying with a higher-tier model. Fields: `FromModel string`, `ToModel string`, `Attempt int`, `Reason string`.
- **`StallDetectedEvent`** — Emitted when a stall timeout fires. Fields: `Elapsed time.Duration`, `Threshold time.Duration`.
- **`ScopeCheckEvent`** — Emitted after scope check completes. Fields: `BeadID string`, `Complexity string`, `Approved bool`, `Reason string`.

#### Decomposition Events

- **`DecomposeStartEvent`** — Emitted when bead decomposition begins. Fields: `BeadID string`, `BeadTitle string`.
- **`SubBeadCreatedEvent`** — Emitted for each sub-bead created. Fields: `ParentBeadID string`, `SubBeadID string`, `SubBeadTitle string`, `Index int`, `Total int`.
- **`DecomposeCompleteEvent`** — Emitted when decomposition finishes. Fields: `BeadID string`, `SubBeadsCreated int`.

#### Diagnostic Events

- **`LogEvent`** — Catch-all for messages that don't map to a specific event type (transitional — should shrink to zero over time). Fields: `Message string`, `Level string` (info, warn, error).

### CLISubscriber

A subscriber that reproduces the current terminal output. It reads from its event channel and writes formatted text to an `io.Writer`, matching the existing output format users see today.

```go
type CLISubscriber struct { ... }

func NewCLISubscriber(output io.Writer, emitter *events.Emitter) *CLISubscriber
func (c *CLISubscriber) Start(ctx context.Context)  // Blocks, receiving events until channel closes or ctx cancels
```

Behavior:
- Maps each event type to formatted text output matching current `r.log()` format (e.g., `IterationStartEvent` produces `"=== Iteration 3 ==="`)
- Handles `HeartbeatEvent` with carriage-return overwrite for in-place updates (same as current `syncWriter.WriteOverwrite` behavior)
- Uses a `syncWriter` (or equivalent) for thread-safe writes
- Unknown event types are silently ignored (forward-compatible)
- The `LogEvent` type is printed as-is, preserving current behavior for any messages not yet migrated to specific event types

The CLISubscriber lives in `internal/events/cli/` (or `internal/cli/events/`, see Decisions).

### TMUX and Status.json as Subscribers

The current hardcoded TMUX title updates and `status.json` writes become event subscribers rather than direct side effects. These are included in this spec's scope:

- **TMUXSubscriber** — Listens for lifecycle events and updates the tmux pane title accordingly. Only active when tmux is detected. Lives alongside or near the existing `internal/tmux/` package.
- **StatusSubscriber** — Listens for lifecycle and progress events and writes `status.json`. Lives alongside or near the existing `internal/state/` package.

### Migration Path

The runner (or post-Phase-1, the pipeline operations) creates an `Emitter`, registers subscribers (CLI, TMUX, status), and replaces `r.log()` calls with `emitter.Emit(SomeEvent{...})`. The `LogEvent` catch-all handles any messages that aren't worth creating a dedicated event type for during initial migration — the goal is to shrink `LogEvent` usage to zero over time.

## Acceptance Criteria

- `internal/events/` package exists with `Event` interface, `Emitter` struct, and all ~28 concrete event types listed above
- `Emitter.Subscribe()` returns a buffered channel; `Emit()` sends to all subscribers non-blocking; `Close()` closes all channels
- `Emitter` is safe for concurrent use (multiple goroutines calling `Emit`, `Subscribe`, `Unsubscribe`)
- `CLISubscriber` consumes events and produces text output matching current runner terminal output format
- `HeartbeatEvent` renders with carriage-return overwrite (in-place update) in `CLISubscriber`
- TMUX title updates and status.json writes are driven by event subscription, not hardcoded calls
- `LogEvent` exists as a transitional catch-all for unmigrated messages
- All existing `r.log()` calls in the runner (or pipeline, post-Phase-1) are replaced with typed event emissions
- Unit tests for `Emitter` (subscribe, emit, unsubscribe, close, concurrent safety, slow consumer drop)
- Unit tests for `CLISubscriber` (event-to-text formatting for each event type)

## Decisions

1. **Channel-based, not callback-based** — The primary consumers (TUI bubbletea update loop, API server WebSocket broadcast) are naturally concurrent. Channels are idiomatic Go for fan-out to concurrent consumers and avoid callback ordering/blocking concerns. The existing `claude.EventHandler` callback pattern is specific to stream processing and doesn't set a precedent for the broader event system.

2. **Typed structs, not generic payloads** — Each event is a concrete struct with typed fields rather than a `map[string]any` payload. This gives compile-time safety, self-documenting payloads, and type-switch dispatch in subscribers. The cost is more boilerplate per event type, but with ~28 types this is manageable.

3. **No filtering at the emitter level** — Every subscriber gets every event. With ~28 event types and a small subscriber count (CLI + TMUX + status + eventually TUI), filtering adds API complexity without meaningful benefit. Subscribers use type-switch default cases to ignore irrelevant events.

4. **Non-blocking emit with drop** — If a subscriber's channel buffer is full, the event is dropped for that subscriber. This prevents a slow or stalled consumer (e.g., a frozen TUI) from blocking pipeline operations. The buffer size is tuned to handle normal bursts; persistent drops indicate a consumer bug, not a system design problem.

5. **`LogEvent` as transitional catch-all** — Rather than requiring every `r.log()` call to become a dedicated event type on day one, `LogEvent` provides a migration path. The goal is to eliminate `LogEvent` usage over time as all messages get proper event types.

6. **CLISubscriber included in this spec** — The event system needs at least one consumer to validate the design and maintain user-visible behavior. The CLISubscriber ensures the transition is invisible to users — same output, different plumbing — and serves as the reference implementation for future subscribers.

7. **Assumes Phase 1 (pipeline extraction) is complete** — Events are emitted from `internal/pipeline/` operations, not from the current `cmd/gromit/` command handlers or `internal/runner/` directly. This avoids designing an event layer that gets immediately refactored when pipeline extraction lands.

## Research & Context

### Current State

All runner output flows through `r.log(format, args...)` in `internal/runner/runner.go` (~211 calls across `runner.go` and `process.go`). This method does `fmt.Sprintf` + `fmt.Fprint` to an `io.Writer` wrapped in a `syncWriter` for thread safety. The heartbeat goroutine uses `syncWriter.WriteOverwrite()` for carriage-return-based in-place terminal updates.

TMUX title updates are hardcoded in the runner via `tmux.Manager.SetTitle()`. Status.json writes are hardcoded via `StatusWriter`. JSONL iteration logging goes through the `IterationLogger` interface — this is already cleanly abstracted and can coexist with the event system (or become a subscriber itself).

The only existing callback patterns are `claude.EventHandler` (raw JSON stream lines) and `claude.ToolCallHandler` (tool call events), both specific to Claude stream processing in `internal/claude/claude.go`.

### Relevant Files

- `internal/runner/runner.go` — `r.log()` method, `syncWriter`, heartbeat goroutine, all current output
- `internal/runner/syncwriter.go` — thread-safe writer with overwrite mode
- `internal/runner/interfaces.go` — existing interfaces (`IterationLogger`, `ClaudeClient`, etc.)
- `internal/claude/claude.go` — `EventHandler`, `ToolCallHandler`, `ToolEvent` (existing callback patterns)
- `internal/logger/` — JSONL and stream logging (may become an event subscriber)
- `internal/tmux/` — TMUX integration (becomes a subscriber)
- `internal/state/` — status.json writing (becomes a subscriber)

### Epic Context

This is Phase 2 of the multi-interface-architecture epic (`.gromit/epics/multi-interface-architecture.md`). It follows Phase 1 (pipeline extraction) and precedes Phase 3 (TUI foundation). The event system is the architectural bridge that enables the TUI, API server, and any future frontend to observe pipeline operations without coupling to terminal I/O.

Related backlog items absorbed by this spec:
- `idea-1770820549600` — structured event system (this item)
- `idea-1770820550926` — make heartbeat and TMUX pluggable (covered by subscriber model)
