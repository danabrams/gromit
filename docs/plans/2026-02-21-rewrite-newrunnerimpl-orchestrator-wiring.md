# Rewrite newRunnerImpl to Wire 5-Stage Pipeline

> **For Claude:** This plan notes the COMPLETED verification of the v2 run loop constructor exposed through `gromit run2`. Use it to confirm that the new package tree wires adapters, stages, and subscribers correctly.

**Goal:** Demonstrate that `cmd/gromit/run2.go` wires the `internal/v2` adapters into the SpecLoop and BeadLoop so the Gate → Build → Validate → Review → Epilogue sequence runs under the new architecture without touching the legacy runner.

**Architecture:** `run2` is the CLI entry point for the new loop. It builds the Git, LLM, TaskTracker, and Presenter adapters, creates the dependency gate (`dep.SpecDependencyGate`), and calls `loop.NewRun2LoopComponents` to produce the `Decompose` stage plus the loaded BeadLoop (Gate/Build/Validate/Review/Epilogue). The components feed into `loop.SpecLoop`, which orchestrates spec-level stages (`plan`, `decompose`, `accept`, `present`) around the bead execution. Typed events stream through `events.NewEmitter()` and the CLI subscriber mesh (`startRun2Subscribers`) for realtime output and logging.

---

## Current Implementation Status

### Adapter-layer infrastructure
* `cmd/gromit/run2.go` wires `adapter.AdapterSet` with `gitadapter.NewExecGitAdapter`, `llm.NewPlanLLMAdapter`, `tasktracker.NewBDAdapter`, and a GitHub presenter adapter.
* A `bead.Client` powers the bd-backed task tracker adapter and is reused by stages that mutate beads (Plan, Review, Epilogue).
* Signals (`SIGINT`, `SIGTERM`) are forwarded to the loop via `stopCh` so both `SpecLoop` and `BeadLoop` can terminate gracefully.

### Stage construction via `loop.NewRun2LoopComponents` (`internal/v2/loop/run2_components.go`)
* Constructs `Decompose`, `Gate`, `Build`, `Validate`, `Review`, and `Epilogue` stages with the adapters they need (provider, task tracker, git, emitter) plus the `noopValidationRunner` for testing.
* Wraps the stages into a `BeadLoop` while wiring the typed event emitter used by subscribers.
* Returns the `Decompose` stage separately so `SpecLoop` can insert it before the bead execution.

### Orchestrator assembly
* `loop.SpecLoop` (see `spec_loop.go`) enforces dependency ordering, checks out spec worktrees, and orchestrates plan, decompose, bead, accept, and present stages.
* `run2` stores the typed emitter, subscribers, and logs directory so that `startRun2Subscribers` can stream events to stderr and record them for offline diagnostics.
* Specs and epics are resolved via `v2spec.Load`, and each spec runs inside a contextual loop invocation.

---

## Verification Tasks

### Task 1: Verify the `run2` CLI entry point
* **Check:** `cmd/gromit/run2.go` instantiates adapters, dependency gate, emitter, and subscribers.
* **Step:** `go test ./cmd/gromit -run TestRun2` and inspect `run2_test.go` to ensure the command covers the dependency gate, spec resolution, and subscriber wiring.

### Task 2: Verify stage and bead loop wiring
* **Check:** `loop.NewRun2LoopComponents` builds every stage and composes them into `BeadLoop` and exposes `Decompose`.
* **Step:** `grep -n "New(" internal/v2/loop/run2_components.go | grep -E "Decompose|Gate|Build|Validate|Review|Epilogue"` to confirm each `New` call is present, then run `go test ./internal/v2/loop -run TestBeadLoop` to exercise the wiring.

### Task 3: Verify spec-level sequencing
* **Check:** `loop.SpecLoop` iterates through stages `plan`, `decompose`, `gate`, `build`, `validate`, `review`, `epilogue`, `accept`, `present` and handles retries/acceptance appropriately.
* **Step:** `go test ./internal/v2/loop -run TestSpecLoop` and inspect `StageSequence` in `spec_loop.go` to show the canon.

### Task 4: Verify RTL acceptance contract
* **Check:** The existing `internal/runner/acceptance/` suite now targets the new run2 wiring through the CLI adapters.
* **Step:** `go test -tags acceptance ./internal/runner/acceptance/...` after ensuring `gromit run2` is invoked by the acceptance helpers, then `go test ./internal/runner/...` to keep behavioral parity.

---

## Optional Observability

Typed events emit from both loops, their stages, and the `review` stage when it emits new beads. `startRun2Subscribers` routes these through `events/stream` and `events/cli`, so verifying telemetry in `cmd/gromit/run2_test.go` ensures the CLI subscribers stay connected.
