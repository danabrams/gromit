---
id: integration-queue-coordinator-wiring
source_ideas: []
created: 2026-03-01
epic: codebase-health
---

# Integration Queue Coordinator Wiring

## Specification

Session commands already enqueue branch work into `.gromit/integration-queue.json` and rely on the run-loop coordinator to drain queue entries into terminal states (`merged`, `conflict`, `failed_gates`, `lane_violation`).

This spec replaces stub coordinator wiring in runner construction with a real `internal/integrationqueue.Coordinator` integration path so queued `ready` entries are actually processed.

## Problem

Queue entries accumulate in `ready` because the runner currently wires a no-op coordinator. The orchestrator calls `Coordinate(...)`, but the active implementation does not mutate queue state or execute integration operations.

## Goals

1. Wire a real coordinator implementation into production orchestrator construction.
2. Ensure one `ready` queue entry is processed per successful iteration, with deterministic state transitions.
3. Preserve crash recovery behavior by invoking real `RecoverFromCrash(...)` logic at startup.
4. Add tests that fail if coordinator wiring regresses to no-op behavior.

## Non-Goals

- Redesigning queue schema or transition model.
- Adding priority scheduling beyond existing FIFO behavior.
- Introducing background daemons or out-of-process integration workers.

## Design

### 1. Constructor wiring

- Replace `runner.NewIntegrationCoordinator()` stub usage in orchestrator wiring with:
  - `integrationqueue.NewStore(gromitDir)`
  - production `integrationqueue.GitOps` adapter
  - production `integrationqueue.ScopedGate` adapter
  - `integrationqueue.NewCoordinator(store, gitops, gate)`

### 2. Runner adapters

- Add/extend runner adapter types that satisfy:
  - `integrationqueue.GitOps`
  - `integrationqueue.ScopedGate`
- Keep subprocess execution argv-safe and consistent with project safety rules.

### 3. Orchestrator behavior

- Keep existing orchestration point: call `Coordinator.Coordinate(ctx)` after successful iterations.
- Keep startup crash-recovery call: `Coordinator.RecoverFromCrash(ctx)`.
- Ensure queue transitions reflect actual integration outcomes.

### 4. Regression protection

- Add tests at constructor/orchestrator seams proving that real coordination occurs and `ready` entries can drain.
- Verify startup recovery transitions stranded `integrating` entries to recoverable state.

## Acceptance Criteria

- Production constructor no longer injects the no-op `runner.IntegrationCoordinator` stub.
- At least one end-to-end runner/coordinator test demonstrates a `ready` entry transitioning out of `ready` during orchestrator execution.
- Crash-recovery path executes real queue recovery logic at startup.
- Existing queue status display continues to reflect persisted queue states without schema regression.

## Decisions

1. Keep single-writer integration ownership in the run loop; do not reintroduce direct session merge ownership.
2. Treat stub coordinator wiring as a bug fix in construction/adapters, not as a queue-model redesign.
3. Prefer adapter-based integration with existing runner dependencies over duplicating git/gate logic.

## Research & Context

- Investigation report: `.gromit/reports/debug-20260301-011049.md`
- Existing plan (superseded by this spec-driven flow): `.gromit/plans/fix-integration-queue-coordinator-wiring.md`
- Tracking issue: `gromit-ejm0`
