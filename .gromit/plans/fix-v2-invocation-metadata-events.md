---
id: fix-v2-invocation-metadata-events
spec: debug-investigation
created: 2026-03-08
decomposed: false
---

# Fix v2 Loop Invocation Metadata Events

## Research & Context

Investigation report: `.gromit/reports/debug-20260308-040000.md`

The v2 loop never emits invocation metadata (model, provider, tier, reasoning level, prompt size, cost, tokens) during LLM invocations. The legacy runner had this via `BuildStartEvent`/`BuildCompleteEvent`/`ModelSelectedEvent`. The infrastructure (CLI subscriber handlers, legacy event types) already exists — the v2 code path just never produces these events.

## Architecture

The event flow is: v2 typed event -> bridge -> legacy event -> CLI subscriber -> terminal output.

Changes touch four layers:
1. **Typed events** (`internal/v2/event/event.go`) — add new event types
2. **Emitters** (`internal/v2/loop/bead_loop.go`, `internal/v2/stage/build/build.go`) — emit the events
3. **Bridge** (`internal/v2/event/bridge.go`) — convert typed -> legacy
4. **Bead completion** (`internal/v2/loop/bead_loop.go`) — propagate build artifacts to bead_complete

## Tasks

### Task 1: Add typed event types for build invocation metadata
**Files:** `internal/v2/event/event.go`
**Size:** Small (15-20 lines)

Add three new typed events:
- `BuildInvocationStartEvent` — emitted before LLM invoke: model, provider name, tier, attempt, max attempts, prompt size (len), reasoning effort
- `BuildInvocationCompleteEvent` — emitted after LLM invoke: model, provider name, success, duration, cost, tokens in, tokens out
- `ModelSelectedEvent` — emitted when router selects a provider/model: model, provider, tier, reason (phase preference / ratio / fallback)

Add corresponding `EventType` constants.

### Task 2: Emit build invocation events from bead loop
**Files:** `internal/v2/loop/bead_loop.go`
**Size:** Medium (30-40 lines)

In `runStageEntry()`, after router selection (lines 442-460):
- Emit `ModelSelectedEvent` with the resolved provider name, model, tier, and selection reason (phase preference, ratio balancing, or default)
- Before calling `runStage()` on build stage: emit `BuildInvocationStartEvent`
- After `runStage()` returns for build stage: emit `BuildInvocationCompleteEvent` with data from `BuildArtifacts`

The provider name needs to be passed through — currently `router.Select()` returns a provider interface but not its name. Either:
- Add a `SelectWithName()` method to Router that returns (provider, model, providerName, error), OR
- Track provider name in a wrapper, OR
- Add provider name to stage.Request so the loop can pass it along

Recommended: extend `Router.Select()` to also return the provider name.

### Task 3: Propagate build artifacts to bead_complete events
**Files:** `internal/v2/loop/bead_loop.go`, `internal/v2/event/event.go`
**Size:** Medium (20-30 lines)

- Add fields to `BeadCompletedEvent`: `Model string`, `Provider string`, `CostUSD float64`, `InputTokens int`, `OutputTokens int`, `Duration time.Duration`
- In `processBead()` / `runStageEntry()`, capture the build stage result's `BuildArtifacts` and thread it back to `emitBeadCompleted()`
- Update `emitBeadCompleted()` to accept and populate these fields

### Task 4: Bridge new typed events to legacy events
**Files:** `internal/v2/event/bridge.go`
**Size:** Small (20-30 lines)

Add bridge cases for:
- `BuildInvocationStartEvent` -> legacy `BuildStartEvent` (model, attempt, max attempts)
- `BuildInvocationCompleteEvent` -> legacy `BuildCompleteEvent` (success, duration, cost, tokens)
- `ModelSelectedEvent` -> legacy `ModelSelectedEvent` (model, reason)
- Updated `BeadCompletedEvent` -> legacy `BeadCompleteEvent` with populated Model/CostUSD/InputTokens/OutputTokens fields

### Task 5: Extend Router.Select to return provider name
**Files:** `internal/v2/routing/router.go`, `internal/v2/routing/router_test.go`
**Size:** Small (10-15 lines)

Change `Select()` signature to return `(LLMProvider, string, string, error)` where the third string is the provider name. Update all callers (bead_loop.go `runStageEntry`, `runTriage`, `decomposeAndRunSubBeads`).

Alternative: add a `SelectNamed()` method to avoid breaking existing callers.

## Dependencies

- Task 1 must complete before Tasks 2, 3, 4
- Task 5 must complete before Task 2 (provider name needed for events)
- Tasks 2, 3, 4 can be done in parallel after their dependencies

## Testing Strategy

### Unit tests for new typed events (Task 1)
- Verify `EventType()` returns correct constants
- Verify JSON serialization includes all fields with correct tags

### Unit tests for Router.Select provider name (Task 5)
- Existing router tests updated to verify provider name is returned
- Test phase preference returns preferred provider name
- Test ratio balancing returns selected provider name

### Unit tests for event bridge (Task 4)
- `BuildInvocationStartEvent` bridges to `BuildStartEvent` with correct fields
- `BuildInvocationCompleteEvent` bridges to `BuildCompleteEvent` with correct fields
- `ModelSelectedEvent` bridges to legacy `ModelSelectedEvent`
- Updated `BeadCompletedEvent` bridges to `BeadCompleteEvent` with model/cost/token fields populated

### Integration test for bead loop event emission (Tasks 2, 3)
- Mock provider that returns known model/tokens/cost
- Run a single bead through the loop
- Capture emitted typed events
- Verify `BuildInvocationStartEvent` emitted with correct model/tier/prompt size
- Verify `BuildInvocationCompleteEvent` emitted with correct duration/cost/tokens
- Verify `ModelSelectedEvent` emitted with provider name and reason
- Verify `BeadCompletedEvent` has populated model/cost/token fields

### CLI subscriber display test
- Feed `BuildStartEvent` through CLI subscriber, verify output format: `"    Building (sonnet, attempt 1/2)..."`
- Feed `BuildCompleteEvent` through CLI subscriber, verify output format
- Feed `ModelSelectedEvent` through CLI subscriber, verify output format: `"    Model: sonnet (ratio balancing)"`

### End-to-end log verification
- After fix, run a spec and verify JSONL log `bead_complete` events have non-empty Model and non-zero cost/tokens
- Verify terminal output shows model/provider info during builds

### Escalation event test
- When build fails on provider A and escalates to provider B:
  - Verify two `BuildInvocationStartEvent` events (one per attempt)
  - Verify `BuildInvocationCompleteEvent` with `Success: false` for first attempt
  - Verify `ModelSelectedEvent` reflects the escalation

### Regression tests
- Existing bead_loop_test.go tests still pass (no behavioral changes to stage pipeline)
- Existing bridge_test.go tests still pass
- Existing CLI subscriber tests still pass
