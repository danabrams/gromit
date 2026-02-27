---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-27T01:51:11Z"
id: event-system
source_spec: event-system
---

# Structured Event System Implementation Plan

**Goal:** Introduce a typed, channel-based event system that decouples pipeline execution from terminal/status/tmux side effects while preserving current user-visible behavior.

**Architecture:** Add `internal/events` as a concurrency-safe fan-out bus with typed events, then migrate orchestration and pipeline stages to emit events consumed by CLI, status, and tmux subscribers.

**Tech Stack:** Go, channels/goroutines, existing runner/pipeline architecture, existing sync writer/status/tmux packages.

**Spec:** `.gromit/specs/event-system.md`

---

## Architecture

## Architecture Proposal

**Overview:**  
Add a new `internal/events` typed event bus and wire event emission at orchestration/pipeline boundaries, then move CLI rendering, status writes, and tmux title updates into subscribers.

**Key Components:**
1. **`internal/events`**: `Event` interface, `Emitter`, event type structs.
2. **`internal/events/cli`**: `CLISubscriber` that maps events to current terminal output format, including heartbeat overwrite behavior.
3. **`internal/events/status`**: subscriber that updates `status.json` from lifecycle/progress events.
4. **`internal/events/tmux`**: subscriber that updates tmux titles from lifecycle events.
5. **`internal/events/emit` (small adapters/helpers)**: stage-local helper functions to emit typed events with minimal boilerplate.

**Integration Points:**
- `Orchestrator.Run` emits run/iteration lifecycle events.
- `prepare/execute/validate/review/epilogue` emit phase start/complete/fail events.
- `execution/heartbeat` emits heartbeat/stall events (instead of direct print).
- direct status/tmux side effects in constructor/epilogue get replaced by subscribers.
- temporary fallback `LogEvent` covers unmigrated `fmt.Fprintf` paths.

**Data Flow:**
- Orchestrator creates `Emitter`.
- Subscribers register (`CLI`, `Status`, optional `TMUX`) and run goroutines.
- Pipeline/orchestration code emits typed events.
- Subscribers consume all events and ignore unneeded ones by type switch.
- `Emitter.Emit` stays non-blocking (drop on full subscriber channel).

**Files to Modify:**
- `internal/runner/constructor.go`
- `internal/runner/orchestrator.go`
- `internal/runner/execution/invoker.go`
- `internal/runner/execution/heartbeat.go`
- `internal/pipeline/prepare/gate.go`
- `internal/pipeline/execute/build.go`
- `internal/pipeline/review/review.go`
- `internal/pipeline/epilogue/epilogue.go`
- plus small interface wiring files in runner/pipeline packages.

**Files to Create:**
- `internal/events/event.go`
- `internal/events/emitter.go`
- `internal/events/types_*.go` (lifecycle, phase, progress, decompose, diagnostic)
- `internal/events/cli/subscriber.go`
- `internal/events/status/subscriber.go`
- `internal/events/tmux/subscriber.go`
- corresponding `_test.go` files.

**Tradeoffs:**
- **Event-at-boundaries first** over “replace every print immediately”: lower migration risk while preserving behavior via `LogEvent`.
- **Subscriber-owned rendering** over inline formatting: cleaner separation, but requires careful parity tests for current CLI output.
- **Non-blocking fan-out with drop** over blocking delivery: protects loop throughput; requires drop-aware tests/observability.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit tests (`internal/events`)**: emitter subscribe/unsubscribe/close semantics, concurrent emit safety, slow consumer drop behavior.
2. **Subscriber unit tests (`internal/events/cli`, `status`, `tmux`)**: event-to-output/status/title mapping, unknown-event ignore, context cancel and channel close shutdown behavior.
3. **Integration tests (runner/pipeline wiring)**: orchestrator + stage flow emits expected event sequences; status/tmux are driven by subscribers (not direct calls); CLI output parity for representative run scenarios.

**Key Test Cases:**
- `Emitter` fan-out sends an event to all active subscribers.
- `Emitter.Emit` does not block when one subscriber channel is full (event dropped only for that subscriber).
- `Subscribe`/`Unsubscribe`/`Close` are safe under concurrent `Emit` calls.
- `CLISubscriber` formats each defined event class correctly, including heartbeat carriage-return overwrite behavior.
- `CLISubscriber` ignores unknown event types without error.
- `StatusSubscriber` writes expected `status.json` updates for iteration start/progress/complete and final run completion.
- `TMUXSubscriber` updates title on iteration transitions and restores/sets terminal states as designed.
- Orchestrator/stage integration emits lifecycle + phase events in expected order for success, validation failure, escalation, and skip/block flows.
- Transitional `LogEvent` still preserves unmigrated user-visible messages.

**Mocking Strategy:**
- Mock/wrap `io.Writer` for CLI formatting assertions.
- Use fake tmux manager and fake status writer/file sink for deterministic subscriber assertions.
- Use real `Emitter` in most tests (important concurrency behavior), with controlled buffered channels and synchronization helpers.
- In runner/pipeline integration tests, use existing stage fakes and add event-capture subscriber to assert emitted event sequences.

**Coverage Goals:**
- Critical paths: run start/end, iteration start/end, build/validation/review transitions, heartbeat/stall, epilogue outcomes.
- Edge cases: no subscribers, unsubscribe unknown channel, emit after close (defined no-op/behavioral expectation), full channel drops, context cancellation races.
- Migration safety: assert there are no remaining direct status/tmux calls in execution paths after conversion.

**Test Organization:**
- `internal/events/emitter_test.go`
- `internal/events/types_test.go` (interface/time/type contracts)
- `internal/events/cli/subscriber_test.go`
- `internal/events/status/subscriber_test.go`
- `internal/events/tmux/subscriber_test.go`
- targeted wiring tests near existing runner/pipeline tests (`internal/runner/..._test.go`, `internal/pipeline/..._test.go`)

## Implementation Tasks

### Task 1: Build Event Core Types and Emitter

**Files:**
- Create: `internal/events/event.go`
- Create: `internal/events/emitter.go`
- Test: `internal/events/emitter_test.go`

**What to Do:**
Implement the `Event` interface and the concurrency-safe `Emitter` with buffered subscription channels, non-blocking fan-out, unsubscribe/close lifecycle, and defensive behavior for concurrent callers.

**Acceptance Criteria:**
- `Event` interface includes `EventType() string` and `EventTime() time.Time`.
- `Emitter.Subscribe()` returns a buffered receive channel and tracks subscriber membership.
- `Emit()` fan-outs to all subscribers with per-subscriber drop-on-full behavior and no emitter-level blocking.

**Dependencies:**
- None

**Notes:**
Define internal buffer size constant in the emitter package and cover it indirectly with slow-consumer tests.

### Task 2: Define Typed Event Catalog

**Files:**
- Create: `internal/events/types_lifecycle.go`
- Create: `internal/events/types_phase_progress.go`
- Create: `internal/events/types_decompose_diagnostic.go`
- Test: `internal/events/types_test.go`

**What to Do:**
Create concrete event structs for all spec-defined lifecycle, phase, progress, decomposition, and diagnostic events, each implementing `Event` and carrying strongly typed fields.

**Acceptance Criteria:**
- All listed event types exist as concrete structs in `internal/events/`.
- Every event type satisfies `Event` and carries timestamp/type data.
- `LogEvent` exists as a transitional catch-all type with level/message payload.

**Dependencies:**
- Task 1

**Notes:**
Use consistent naming and constructor helpers only when they reduce repetitive timestamp/type initialization.

### Task 3: Add CLI Event Subscriber

**Files:**
- Create: `internal/events/cli/subscriber.go`
- Test: `internal/events/cli/subscriber_test.go`
- Modify: `internal/runner/syncwriter.go` (only if required for reuse)

**What to Do:**
Implement `CLISubscriber` that consumes `Event` values, maps them to terminal output matching current user-visible format, and supports heartbeat overwrite rendering equivalent to current behavior.

**Acceptance Criteria:**
- `Start(ctx)` consumes events until context cancel or emitter channel close.
- Known events render expected lines; unknown event types are ignored.
- `HeartbeatEvent` uses carriage-return overwrite semantics compatible with existing sync writer behavior.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Prefer reusing existing thread-safe write/overwrite mechanics to avoid output race regressions.

### Task 4: Add Status and TMUX Subscribers

**Files:**
- Create: `internal/events/status/subscriber.go`
- Create: `internal/events/tmux/subscriber.go`
- Test: `internal/events/status/subscriber_test.go`
- Test: `internal/events/tmux/subscriber_test.go`

**What to Do:**
Implement subscribers that translate run/iteration/progress events into `status.json` writes and tmux title updates, replacing direct side effects in orchestration paths.

**Acceptance Criteria:**
- Status subscriber updates status data from emitted lifecycle/progress events and final run completion.
- TMUX subscriber updates titles from emitted lifecycle events and no-ops safely when tmux is unavailable.
- Both subscribers tolerate irrelevant/unknown events without failure.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Define small interfaces around status/tmux dependencies to keep tests deterministic and avoid shell-level effects.

### Task 5: Wire Emitter and Subscribers in Runner Construction

**Files:**
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/constructor_adapters_test.go` (or new focused wiring test)

**What to Do:**
Create and own emitter lifecycle in runner construction, register subscribers (CLI always; status/tmux based on environment/availability), and pass event emission capability into orchestrator/stages.

**Acceptance Criteria:**
- Runner wiring initializes emitter and subscriber goroutines before loop execution.
- Subscriber teardown occurs cleanly on completion/cancel (no goroutine leak behavior in tests).
- Orchestrator gains access to event emission primitives without tight coupling to subscriber internals.

**Dependencies:**
- Task 1
- Task 3
- Task 4

**Notes:**
Keep subscriber registration explicit in constructor wiring so future TUI/API subscribers can be added without stage changes.

### Task 6: Emit Lifecycle and Phase Events Across Pipeline

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/pipeline/prepare/gate.go`
- Modify: `internal/pipeline/execute/build.go`
- Modify: `internal/pipeline/review/review.go`
- Modify: `internal/pipeline/epilogue/epilogue.go`

**What to Do:**
Replace direct textual lifecycle/phase output paths with typed emissions at stage boundaries, including run start/complete, iteration start/complete, scope checks, build/validation/review events, and completion/failure/stuck/skip outcomes.

**Acceptance Criteria:**
- Core lifecycle transitions emit corresponding typed events.
- Stage boundaries emit start/complete/fail events with required payload fields.
- Remaining unmapped messages are emitted via `LogEvent` instead of direct user-facing writes where practical.

**Dependencies:**
- Task 2
- Task 5

**Notes:**
This is the largest migration step; prioritize deterministic event order for downstream subscribers and tests.

### Task 7: Convert Heartbeat/Stall Flow to Progress Events

**Files:**
- Modify: `internal/runner/execution/heartbeat.go`
- Modify: `internal/runner/execution/invoker.go`
- Test: `internal/runner/execution/heartbeat_test.go`

**What to Do:**
Emit `HeartbeatEvent`, `StallDetectedEvent`, and related progress/escalation events from execution/heartbeat flow and route rendering responsibility to subscribers rather than direct heartbeat printing.

**Acceptance Criteria:**
- Heartbeat data is emitted as typed events at existing cadence with equivalent metrics payload.
- Stall detection still cancels invocation and surfaces stall signal behavior unchanged.
- No regression in tool-call update responsiveness under load.

**Dependencies:**
- Task 2
- Task 5

**Notes:**
Retain current timing defaults and cancellation semantics; only transport/rendering path should change.

### Task 8: End-to-End Validation and Parity Hardening

**Files:**
- Modify: `internal/runner/acceptance/*` (targeted additions)
- Modify: `internal/pipeline/*_test.go` (targeted additions)
- Modify: `internal/events/*_test.go` (gap-filling)

**What to Do:**
Add integration and acceptance coverage proving event-driven architecture works end-to-end and preserves observable behavior (CLI formatting, status updates, and tmux updates).

**Acceptance Criteria:**
- Event ordering and payload assertions cover success/failure/escalation/skip paths.
- CLI output parity tests pass for representative iteration sequences, including heartbeat overwrite behavior.
- Status/tmux behavior is confirmed to be subscriber-driven (no direct side-effect coupling in run flow).

**Dependencies:**
- Task 3
- Task 4
- Task 6
- Task 7

**Notes:**
Focus parity on high-signal outputs users rely on; avoid brittle exact-string assertions when semantic assertions suffice.

---

## Notes

- The spec narrative references legacy `r.log()` concentration, but current code has already split concerns across `Orchestrator` and pipeline stages; migration should target those boundaries directly.
- `LogEvent` should be treated as migration debt with follow-up cleanup beads to convert remaining generic messages into typed events.
- Keep subscriber registration centralized in runner construction so new interfaces (TUI/API) can attach without altering stage code.
