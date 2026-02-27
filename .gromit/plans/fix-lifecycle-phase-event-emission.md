---
created: 2026-02-27T02:17:49Z
decomposed: true
decomposed_at: "2026-02-27T02:32:30Z"
id: fix-lifecycle-phase-event-emission
spec: event-system
---

# Fix Lifecycle + Phase Event Emission

**Goal:** Complete event-system wiring so orchestrator and pipeline stage boundaries emit typed events consistently, with `LogEvent` used only for unmapped transitional output.

**Research & Context:** `.gromit/reports/debug-20260227-021749.md`

**Scope:**
- `internal/runner/orchestrator.go`
- `internal/pipeline/stage.go`
- `internal/pipeline/prepare/gate.go`
- `internal/pipeline/execute/build.go`
- `internal/pipeline/review/review.go`
- `internal/pipeline/epilogue/epilogue.go`
- related unit/acceptance tests in `internal/runner` and `internal/pipeline/*`

## Architecture

- Add a stage-level event emitter contract through `pipeline.Input` (minimal interface, no sibling-package coupling).
- Keep typed boundary events as primary output path for lifecycle/progress.
- Keep direct terminal text out of stage boundary code except subscriber rendering.
- For messages with no dedicated typed event yet, emit `events.LogEvent` (level + message + timestamp).

## Tasks

### 1. Add event sink plumbing to stage contracts
- Update `internal/pipeline/stage.go` input shape to carry an emitter/event sink.
- Thread the field from orchestrator input construction.
- Preserve existing behavior when emitter is nil (no-op safe).

### 2. Emit orchestrator lifecycle events
- Emit run-level events at run start/complete.
- Emit iteration-level events at iteration start/complete.
- Emit bead-level success/failure/skip/stuck events based on stage decisions/outcomes.
- Keep any remaining `o.logf` paths as `LogEvent` fallback where no typed event exists.

### 3. Emit gate/build/review/epilogue boundary events
- Gate: emit scope/stuck/block/skip/proceed-adjacent events and warnings as `LogEvent` where needed.
- Build: emit build start/complete with model/tier/cost/token/duration metadata; emit escalation events on retry tier hops.
- Review: emit review start/complete (verdict + created bead count summary).
- Epilogue: emit bead completion/failure plus key cleanup outcomes.

### 4. Strengthen tests for event contracts
- Extend orchestrator tests to assert event ordering and payloads across success/failure/skip paths.
- Add stage-level tests verifying event emission and `LogEvent` fallback behavior.
- Ensure acceptance parity with existing subscriber/event-order tests.

### 5. Validation and hardening
- Run targeted tests for changed packages.
- Run full validation:
  - `go test -vet=off -p 4 -parallel 4 ./...`
  - `go vet ./...`
  - `go build ./...`
- If unrelated pre-existing failures persist, document them separately from this bead’s behavior.

## Dependencies

1. Task 1 before Tasks 2 and 3 (stage event sink required).
2. Task 2 before broad ordering assertions in Task 4.
3. Task 3 and Task 4 before Task 5 full validation.

## Testing Strategy

- Unit tests first for deterministic boundary behavior.
- Orchestrator sequencing tests to validate lifecycle and phase transitions.
- Acceptance-level event ordering checks to ensure subscriber-visible behavior reflects typed emission path.
- Explicit regression test: pipeline path should not rely on direct stage `fmt.Fprintf` text for core lifecycle observability.
