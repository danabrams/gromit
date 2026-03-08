---
id: fix-v2-invocation-metadata-events-test-plan
spec: debug-investigation
created: 2026-03-08
parent: fix-v2-invocation-metadata-events
---

# Test & Verification Plan: v2 Invocation Metadata Events

## Overview

This plan covers all tests required to verify the fix for missing invocation metadata display in the v2 loop. Tests are organized by layer (typed events, bridge, bead loop, CLI subscriber, end-to-end) and written to follow existing test patterns in the codebase.

## Existing Test Infrastructure

Tests follow established patterns found in:
- `internal/v2/event/event_test.go` — JSON round-trip, snake_case field validation, embedded Event checks
- `internal/v2/event/bridge_test.go` — table-driven bridge conversion with type assertion checks
- `internal/v2/loop/bead_loop_routing_test.go` — `spyStage` / `fakeProvider` pattern for capturing request fields
- `internal/v2/loop/bead_loop_stage_events_test.go` — typed emitter subscriber pattern for capturing events
- `internal/events/cli/subscriber_test.go` — emitter + subscriber + `eventtest.WaitForCondition` pattern

---

## Layer 1: Typed Events (`internal/v2/event/event_test.go`)

### Test 1.1: New event types embed Event base
**File:** `internal/v2/event/event_test.go` (extend `TestEventDefinitions`)
**Pattern:** Add to `LifecycleEventsEmbedBase` sub-test
```
expectEmbeddedEvent(t, reflect.TypeOf(BuildInvocationStartEvent{}))
expectEmbeddedEvent(t, reflect.TypeOf(BuildInvocationCompleteEvent{}))
expectEmbeddedEvent(t, reflect.TypeOf(ModelSelectedEvent{}))
```
**Verifies:** Struct embedding is correct.

### Test 1.2: JSON round-trip for new event types
**File:** `internal/v2/event/event_test.go` (extend `TestEventJSONRoundTrip`)
**Pattern:** Add table entries for each new event type.

**BuildInvocationStartEvent:**
```go
{
    name: "BuildInvocationStartEvent",
    event: BuildInvocationStartEvent{
        Event:       Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBuildInvocationStart},
        BeadID:      "gromit-abc12",
        Model:       "sonnet",
        Provider:    "claude",
        Tier:        "medium",
        Attempt:     1,
        MaxAttempts: 3,
        PromptSize:  24500,
    },
    expectedFields: []string{"schema_version", "bead_id", "model", "provider", "tier", "attempt", "max_attempts", "prompt_size"},
}
```

**BuildInvocationCompleteEvent:**
```go
{
    name: "BuildInvocationCompleteEvent",
    event: BuildInvocationCompleteEvent{
        Event:        Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBuildInvocationComplete},
        BeadID:       "gromit-abc12",
        Model:        "sonnet",
        Provider:     "claude",
        Success:      true,
        Duration:     2 * time.Minute,
        CostUSD:      0.042,
        InputTokens:  15000,
        OutputTokens: 3200,
    },
    expectedFields: []string{"schema_version", "bead_id", "model", "provider", "success", "duration", "cost_usd", "input_tokens", "output_tokens"},
}
```

**ModelSelectedEvent:**
```go
{
    name: "ModelSelectedEvent",
    event: ModelSelectedEvent{
        Event:    Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeModelSelected},
        BeadID:   "gromit-abc12",
        Model:    "sonnet",
        Provider: "claude",
        Tier:     "medium",
        Reason:   "phase preference",
    },
    expectedFields: []string{"schema_version", "bead_id", "model", "provider", "tier", "reason"},
}
```

**Verifies:** JSON serialization produces correct snake_case keys and round-trips losslessly.

### Test 1.3: snake_case field naming for new events
**File:** `internal/v2/event/event_test.go` (extend `TestEventJSONFieldNames`)
**Pattern:** Add table entries matching existing structure.
**Verifies:** No camelCase keys leak into JSON. Fields like `max_attempts`, `prompt_size`, `cost_usd`, `input_tokens`, `output_tokens` are all snake_case.

### Test 1.4: EventType() returns correct constants
**File:** `internal/v2/event/event_test.go` (new sub-test or extend existing)
```go
func TestNewEventTypeConstants(t *testing.T) {
    tests := []struct{
        event TypedEvent
        want  string
    }{
        {BuildInvocationStartEvent{}, EventTypeBuildInvocationStart},
        {BuildInvocationCompleteEvent{}, EventTypeBuildInvocationComplete},
        {ModelSelectedEvent{}, EventTypeModelSelected},
    }
    for _, tt := range tests {
        if got := tt.event.EventType(); got != tt.want {
            t.Errorf("%T.EventType() = %q, want %q", tt.event, got, tt.want)
        }
    }
}
```
**Verifies:** EventType() method wiring is correct.

### Test 1.5: BeadCompletedEvent new fields serialize correctly
**File:** `internal/v2/event/event_test.go` (update existing `BeadCompletedEvent` test entry)
**Add fields:** `Model`, `Provider`, `CostUSD`, `InputTokens`, `OutputTokens`, `Duration`
**Add expected fields:** `"model"`, `"provider"`, `"cost_usd"`, `"input_tokens"`, `"output_tokens"`, `"duration"`
**Verifies:** New fields on BeadCompletedEvent serialize and round-trip correctly.

---

## Layer 2: Event Bridge (`internal/v2/event/bridge_test.go`)

### Test 2.1: BuildInvocationStartEvent bridges to legacy BuildStartEvent
**File:** `internal/v2/event/bridge_test.go` (extend `TestLegacyEventsFromTyped`)
```go
{
    name: "build invocation start",
    typed: &BuildInvocationStartEvent{
        Event:       Event{Timestamp: now},
        BeadID:      "bead-1",
        Model:       "sonnet",
        Provider:    "claude",
        Attempt:     2,
        MaxAttempts: 3,
    },
    wantType: "*events.BuildStartEvent",
    check: func(evt events.Event) bool {
        e, ok := evt.(*events.BuildStartEvent)
        return ok && e.Model == "sonnet" && e.Attempt == 2 && e.MaxAttempts == 3
    },
}
```
**Verifies:** Typed event converts to correct legacy type with field mapping.

### Test 2.2: BuildInvocationCompleteEvent bridges to legacy BuildCompleteEvent
```go
{
    name: "build invocation complete",
    typed: &BuildInvocationCompleteEvent{
        Event:        Event{Timestamp: now},
        BeadID:       "bead-1",
        Success:      true,
        Duration:     3 * time.Minute,
        CostUSD:      0.05,
        InputTokens:  20000,
        OutputTokens: 4000,
    },
    wantType: "*events.BuildCompleteEvent",
    check: func(evt events.Event) bool {
        e, ok := evt.(*events.BuildCompleteEvent)
        return ok && e.Success && e.Cost == 0.05 && e.TokensIn == 20000 && e.TokensOut == 4000
    },
}
```

### Test 2.3: ModelSelectedEvent bridges to legacy ModelSelectedEvent
```go
{
    name: "model selected",
    typed: &ModelSelectedEvent{
        Event:    Event{Timestamp: now},
        Model:    "opus",
        Provider: "claude",
        Reason:   "phase preference",
    },
    wantType: "*events.ModelSelectedEvent",
    check: func(evt events.Event) bool {
        e, ok := evt.(*events.ModelSelectedEvent)
        return ok && e.Model == "opus" && e.Reason == "phase preference"
    },
}
```

### Test 2.4: Updated BeadCompletedEvent bridges with populated fields
```go
{
    name: "bead completed with build metadata",
    typed: &BeadCompletedEvent{
        Event:        Event{Timestamp: now},
        BeadID:       "bead-1",
        BeadTitle:    "test bead",
        Success:      true,
        Model:        "sonnet",
        CostUSD:      0.03,
        InputTokens:  12000,
        OutputTokens: 2500,
        Duration:     2 * time.Minute,
    },
    wantType: "*events.IterationCompleteEvent",
    // Also verify second legacy event (BeadCompleteEvent) has populated fields
}
```
**Note:** Need separate check for the second event in the list — `convertBeadCompleted` returns multiple events. Add a multi-event check:
```go
checkAll: func(evts []events.Event) bool {
    if len(evts) < 2 { return false }
    bce, ok := evts[1].(*events.BeadCompleteEvent)
    return ok && bce.Model == "sonnet" && bce.CostUSD == 0.03 && bce.InputTokens == 12000
}
```

### Test 2.5: Bridge end-to-end via BridgeTypedToLegacy
**Pattern:** Same as `TestBridgeTypedEventsToLegacyEmitter`
```go
func TestBridgeBuildInvocationEvents(t *testing.T) {
    typed := NewEmitter()
    legacy := events.NewEmitter()
    ch := legacy.Subscribe()
    defer legacy.Unsubscribe(ch)

    BridgeTypedToLegacy(typed, legacy)

    typed.Emit(&BuildInvocationStartEvent{
        Event:   Event{Timestamp: time.Now().UTC()},
        Model:   "sonnet",
        Attempt: 1, MaxAttempts: 2,
    })

    select {
    case evt := <-ch:
        if _, ok := evt.(*events.BuildStartEvent); !ok {
            t.Fatalf("got %T, want *events.BuildStartEvent", evt)
        }
    case <-time.After(time.Second):
        t.Fatal("timed out")
    }
}
```
**Verifies:** Full bridge pipeline works (emit typed -> bridge converts -> legacy subscriber receives).

---

## Layer 3: Router (`internal/v2/routing/router_test.go`)

### Test 3.1: Select returns provider name
**File:** `internal/v2/routing/router_test.go`
```go
func TestRouterSelectReturnsProviderName(t *testing.T) {
    r := NewRouter(RouterConfig{
        Providers: map[string]llmtypes.LLMProvider{
            "claude": &fakeProvider{},
            "openai": &fakeProvider{},
        },
        Ratio: map[string]int{"claude": 1, "openai": 1},
    })
    provider, model, name, err := r.Select("build", "medium")
    // ... assert name is "claude" or "openai" (deterministic via sort order)
    // ... assert provider is non-nil
    // ... assert err is nil
}
```

### Test 3.2: Select with phase preference returns correct provider name
```go
func TestRouterSelectPhasePreferenceReturnsName(t *testing.T) {
    r := NewRouter(RouterConfig{
        Providers: map[string]llmtypes.LLMProvider{
            "claude": &fakeProvider{},
            "openai": &fakeProvider{},
        },
        PhasePreferences: map[string]string{"review": "claude"},
        Ratio: map[string]int{"claude": 1, "openai": 1},
    })
    _, _, name, err := r.Select("review", "medium")
    if err != nil { t.Fatal(err) }
    if name != "claude" {
        t.Fatalf("provider name = %q, want %q", name, "claude")
    }
}
```

### Test 3.3: Select with unavailable provider returns fallback name
Verify that when preferred provider is unavailable, the returned name matches the fallback provider.

### Test 3.4: Existing router tests still pass
Run `go test ./internal/v2/routing/...` to confirm the `Select()` signature change doesn't break anything.

---

## Layer 4: Bead Loop Event Emission (`internal/v2/loop/`)

### Test 4.1: Build invocation events emitted during successful build
**File:** `internal/v2/loop/bead_loop_stage_events_test.go` (new test)
**Pattern:** Follow `TestBeadLoopEmitsStageLifecycleEvents` — use typed emitter subscriber to capture events.

```go
func TestBeadLoopEmitsBuildInvocationEvents(t *testing.T) {
    emitter := event.NewEmitter()
    ch := make(chan event.TypedEvent, 32)
    emitter.Subscribe(func(evt event.TypedEvent) { ch <- evt })

    // Use a mock build stage that returns BuildArtifacts
    buildStage := &artifactBuildStage{
        name: "build",
        artifacts: &build.BuildArtifacts{
            Model:    "sonnet",
            Tokens:   18000,
            CostUSD:  0.042,
            Duration: 2 * time.Minute,
            Success:  true,
        },
    }

    r := routing.NewRouter(routing.RouterConfig{
        Providers: map[string]llmtypes.LLMProvider{"claude": &fakeProvider{}},
        Ratio:     map[string]int{"claude": 1},
    })

    loop := // ... construct with emitter, router, buildStage
    loop.Run(ctx, []*bead.Bead{{ID: "bead-1"}}, nil)

    events := collectAllEvents(ch)
    // Assert: contains ModelSelectedEvent with provider="claude"
    // Assert: contains BuildInvocationStartEvent with model, tier, attempt
    // Assert: contains BuildInvocationCompleteEvent with success=true, tokens, cost
}
```

**Verifies:** Events are emitted at the right points in the build pipeline.

### Test 4.2: Build artifacts propagated to BeadCompletedEvent
**Pattern:** Same as 4.1, but assert on `BeadCompletedEvent` fields.
```go
// Find the BeadCompletedEvent and verify it has build metadata
for _, evt := range events {
    if bce, ok := evt.(event.BeadCompletedEvent); ok {
        if bce.Model != "sonnet" { t.Errorf(...) }
        if bce.CostUSD != 0.042 { t.Errorf(...) }
        if bce.InputTokens + bce.OutputTokens != 18000 { t.Errorf(...) }
    }
}
```
**Verifies:** Build artifacts flow through to the bead_complete event.

### Test 4.3: Escalation emits multiple invocation events
**Pattern:** Use `spyStage` that fails first call, succeeds second.
```go
// buildSpy fails first attempt, succeeds second
// Assert: two BuildInvocationStartEvent (attempt 1, attempt 2)
// Assert: first BuildInvocationCompleteEvent has Success=false
// Assert: second BuildInvocationCompleteEvent has Success=true
// Assert: ModelSelectedEvent shows escalated tier
```
**Verifies:** Escalation retry produces correct event sequence.

### Test 4.4: Events emitted when router is nil (no routing)
```go
// No router configured — model resolved via defaultTierToModel
// Assert: ModelSelectedEvent still emitted with provider="" and model="sonnet" (default medium)
// Assert: BuildInvocationStartEvent has model="sonnet"
```
**Verifies:** Events work in the no-router fallback path.

### Test 4.5: Events emitted when routing fails
```go
// Router with empty providers (ErrNoProviders)
// Assert: no ModelSelectedEvent (routing failed)
// Assert: BuildInvocationStartEvent still emitted with whatever model was resolved
```
**Verifies:** Graceful degradation — events don't crash when routing fails.

### Test 4.6: Prompt size in BuildInvocationStartEvent
```go
// Use a build stage that captures the prompt text length
// Assert: BuildInvocationStartEvent.PromptSize matches len(assembledPrompt)
```
**Verifies:** Prompt size metric is correctly reported.

---

## Layer 5: CLI Subscriber Display (`internal/events/cli/subscriber_test.go`)

### Test 5.1: BuildStartEvent formatting
**File:** `internal/events/cli/subscriber_test.go`
```go
func TestCLISubscriber_BuildStartEventFormat(t *testing.T) {
    emitter := events.NewEmitter()
    output := &bytes.Buffer{}
    subscriber := NewCLISubscriber(BasicWriter(output), emitter)
    // ... start subscriber, wait for ready
    emitter.Emit(&events.BuildStartEvent{
        Model: "sonnet", Attempt: 1, MaxAttempts: 2,
    })
    // ... wait for output
    want := "    Building (sonnet, attempt 1/2)...\n"
    if !strings.Contains(output.String(), want) {
        t.Errorf("output = %q, want to contain %q", output.String(), want)
    }
}
```
**Verifies:** Terminal output matches expected format.

### Test 5.2: BuildCompleteEvent formatting
```go
emitter.Emit(&events.BuildCompleteEvent{
    Success: true, Cost: 0.0420, TokensIn: 15000, TokensOut: 3200,
    Duration: 2 * time.Minute,
})
// Assert contains: "Build SUCCESS: 0.0420 USD, 15000 in, 3200 out, 2m0s"
```

### Test 5.3: ModelSelectedEvent formatting
```go
emitter.Emit(&events.ModelSelectedEvent{
    Model: "opus", Reason: "phase preference",
})
// Assert contains: "    Model: opus (phase preference)\n"
```

### Test 5.4: Failed build formatting
```go
emitter.Emit(&events.BuildCompleteEvent{
    Success: false, Cost: 0.01, TokensIn: 5000, TokensOut: 200,
    Duration: 4 * time.Second,
})
// Assert contains: "Build FAILED:"
```

---

## Layer 6: JSONL Log Verification

### Test 6.1: File subscriber writes new event types
**File:** `internal/v2/event/file_subscriber_test.go` (extend existing)
```go
// Emit BuildInvocationStartEvent, BuildInvocationCompleteEvent, ModelSelectedEvent
// Read JSONL output
// Assert each event type appears with correct fields
```
**Verifies:** New events are persisted to JSONL logs with correct structure.

### Test 6.2: Stream subscriber writes new legacy events
**File:** `internal/events/stream/subscriber_test.go` (extend existing if applicable)
**Verifies:** Legacy BuildStartEvent/BuildCompleteEvent appear in event JSONL logs.

---

## Layer 7: Regression Tests

### Test 7.1: All existing bead loop tests pass
```bash
go test ./internal/v2/loop/...
```
No behavioral changes to stage pipeline — existing tests must not break.

### Test 7.2: All existing event tests pass
```bash
go test ./internal/v2/event/...
```

### Test 7.3: All existing bridge tests pass
```bash
go test ./internal/v2/event/... -run Bridge
```

### Test 7.4: All existing CLI subscriber tests pass
```bash
go test ./internal/events/cli/...
```

### Test 7.5: All existing routing tests pass
```bash
go test ./internal/v2/routing/...
```

### Test 7.6: Full project compilation gate
```bash
go build ./...
go vet ./...
```

---

## Verification Checklist

After implementation, verify each of these manually with a single `gromit run2` execution:

- [ ] Terminal shows model and provider for each build invocation (e.g., "Building (sonnet, attempt 1/2)...")
- [ ] Terminal shows model selection reason (e.g., "Model: sonnet (ratio balancing)")
- [ ] When codex (openai) provider is selected, terminal shows the codex model name (e.g., "gpt-5.3-codex")
- [ ] When build fails and escalates, terminal shows escalation with model change
- [ ] JSONL log `bead_complete` events have non-empty `Model` field
- [ ] JSONL log `bead_complete` events have non-zero `CostUSD`, `InputTokens`, `OutputTokens`
- [ ] JSONL log contains `build_invocation_start` events with model, provider, tier, prompt_size
- [ ] JSONL log contains `build_invocation_complete` events with success, duration, cost, tokens
- [ ] JSONL log contains `model_selected` events with provider name and selection reason
- [ ] No regressions: `go test ./...` passes

---

## Test Count Summary

| Layer | Tests | Files Modified |
|-------|-------|----------------|
| Typed events | 5 | `event_test.go` |
| Bridge | 5 | `bridge_test.go` |
| Router | 4 | `router_test.go` |
| Bead loop | 6 | `bead_loop_stage_events_test.go` |
| CLI subscriber | 4 | `subscriber_test.go` |
| JSONL logs | 2 | `file_subscriber_test.go` |
| Regression | 6 | (existing files, run only) |
| **Total** | **32** | |
