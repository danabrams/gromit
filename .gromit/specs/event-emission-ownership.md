---
id: event-emission-ownership
source_ideas: []
created: 2026-02-27
epic: multi-interface-architecture
---

# Event Emission Ownership: Stage-Only Convention

## Problem

Phase events (BuildStart/Complete, ReviewStart/Complete) are emitted from both the orchestrator and the individual pipeline stages, producing duplicate events with conflicting data. The orchestrator's ReviewCompleteEvent hardcodes `verdict="pending"` and `issues=nil`, masking the actual review result. The orchestrator's BuildStartEvent uses `baseIn.Complexity` for Model while the stage uses the actual resolved `tier`.

Additionally, 3 event types defined in the new gate/epilogue files (GateScopeEvent, BeadCloseEvent, BeadCleanupEvent) are never emitted anywhere, and 8 new event types are missing from `types_test.go` registry tests. A pre-existing prompt budget test failure (60 chars over) also needs fixing.

## Convention

**Orchestrator owns lifecycle events only:** RunStart/Complete, IterationStart/Complete, BeadComplete/Failed/Stuck/Skipped. These represent loop-level state transitions.

**Stages own their own phase events:** BuildStart/Complete, ValidationStart/Pass/Fail, ReviewStart/Complete, EpilogueStart/Complete, Gate events. Stages have the richest context (actual model after escalation, real verdict, cost/tokens from result).

The orchestrator wires the emitter into `pipeline.Input` (already done) and does not duplicate phase-level emissions.

## Beads

### Bead 1: Remove duplicate orchestrator phase emissions

Remove BuildStartEvent, BuildCompleteEvent, ReviewStartEvent, and ReviewCompleteEvent emissions from `orchestrator.go` (lines ~335-351 and ~459-472). The stage-level emissions in `build.go` and `review.go` already emit these with richer, correct data. Populate `Time` field in stage-level emissions for consistency.

**Files:** `internal/runner/orchestrator.go`

**Acceptance Criteria:**
- No BuildStartEvent/BuildCompleteEvent emitted from orchestrator.go
- No ReviewStartEvent/ReviewCompleteEvent emitted from orchestrator.go
- Stage-level emissions in build.go and review.go include `Time: time.Now()`
- Review stage emission includes `Thorough` field
- Existing orchestrator event contract tests still pass (update if they assert on orchestrator-emitted phase events)
- `go test ./internal/runner/... ./internal/pipeline/...` passes

### Bead 2: Add Validate stage event emission

The Validate stage currently has no self-emitted events — ValidationStart/Pass/Fail are emitted from the orchestrator. Move these emissions into the Validate stage itself to match the stage-ownership convention.

**Files:** `internal/pipeline/validate/validate.go`, `internal/pipeline/validate/validate_test.go`, `internal/runner/orchestrator.go`

**Acceptance Criteria:**
- ValidationStartEvent emitted from Validate.Run() before validation runs
- ValidationPassEvent or ValidationFailEvent emitted from Validate.Run() after validation
- Orchestrator no longer emits ValidationStart/Pass/Fail events
- Test added to validate_test.go confirming event emission
- `go test ./internal/pipeline/validate/... ./internal/runner/...` passes

### Bead 3: Wire or remove dead event types

GateScopeEvent, BeadCloseEvent, and BeadCleanupEvent are defined but never emitted. Either wire them into the appropriate emission points (gate.go scope check path for GateScopeEvent, epilogue.go close/cleanup paths for BeadCloseEvent/BeadCleanupEvent) or remove them.

Recommendation: wire them — the emission points exist and the events carry useful telemetry.

**Files:** `internal/pipeline/prepare/gate.go`, `internal/pipeline/epilogue/epilogue.go`, `internal/events/types_gate.go`, `internal/events/types_epilogue.go`

**Acceptance Criteria:**
- GateScopeEvent emitted when scope gate detects oversized bead (before block/decompose decision)
- BeadCloseEvent emitted when epilogue successfully closes a bead
- BeadCleanupEvent emitted for each cleanup action (sync, merge, worktree_cleanup)
- Tests added for each new emission point
- `go test ./internal/pipeline/...` passes

### Bead 4: Update types_test.go for new event types

8 new event types (EpilogueStartEvent, EpilogueCompleteEvent, BeadCloseEvent, BeadCleanupEvent, GateScopeEvent, GateStuckEvent, GateSkipEvent, GateBlockEvent) are missing from TestAllEventTypesImplementEvent, TestEventTypeStringsAreUnique (hardcoded count=27), and specEventCases.

**Files:** `internal/events/types_test.go`

**Acceptance Criteria:**
- All 8 new event types added to TestAllEventTypesImplementEvent instantiation list
- All 8 new event types added to TestEventTypeStringsAreUnique map (count updated from 27 to 35)
- All 8 new event types added to specEventCases with correct EventType strings and Time handling
- `go test ./internal/events/...` passes

### Bead 5: Fix prompt budget overage

TestRulesPhaseCharBudgets/build fails: 9260 chars exceeds 9200 budget (+60 chars). Caused by cumulative event type additions increasing rules content size. Either trim rules content or increase the budget.

**Files:** `internal/prompt/` (budget constant or rules template)

**Acceptance Criteria:**
- TestRulesPhaseCharBudgets/build passes
- No other budget tests regressed
- `go test ./internal/prompt/...` passes
