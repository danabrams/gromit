# Runner Pipeline Refactor Design

**Date:** 2026-02-21
**Status:** Proposed

---

## Background

Gromit's `Runner` struct has accumulated roughly 40 fields spread across more than 20 files. The `callbacks.go` file alone runs to 756 lines and is exercised by every execution path through the system. An earlier round of sub-package splits — `escalation/`, `methodology/`, `validation/`, `execution/` — provided some organizational relief but did not decouple state. The `Runner` struct still owns everything. Each new feature found it psychologically easy to attach another field to `Runner` or another method to `callbacks.go`, and the accumulation continued.

The fix is a different object model. Stages receive explicit inputs and return explicit outputs. The orchestrator holds no mutable state beyond its wiring of stages together. State flows through data, not through a shared struct. To keep v1 running while the new architecture bootstraps, we implemented the v2 loop inside its own `internal/v2/` package tree and exposed it via the `gromit run2` command instead of mutating the existing runner.

---

## Decision: Build the v2 run loop in `internal/v2/` and keep v1 untouched

The 170 non-acceptance test files in `internal/runner/` encode implementation-specific wiring; rewriting them for the new model would have anchored the refactor to the old structure. Rather than evolve `internal/runner` in place, we let it continue as-is and built the cleaner loop adjacent to it.

Key consequences of the decision:

1. **Safe bootstrap.** Gromit v1 builds the project binaries and powers automation. Touching `internal/runner` directly while v2 is incomplete could break the tool that builds and tests v2 itself. The `internal/v2/` package tree avoids that dependency cycle.
2. **No translational adapters.** Instead of wrapping v1 stages in adapters and rewriting them one by one, v2 stages are natively implemented. The new stage interfaces and loop are their own code, not incremental mutations of runner internals.
3. **Controlled cutover.** `gromit run2` routes to the v2 loop while `gromit run` keeps executing the legacy runner. We switch the commands only after v2 acceptance passes and we can delete the old runner wholesale.

Acceptance tests remain the behavioral contract. They continue to live under `internal/runner/acceptance/` and assert the outcomes that v2 must satisfy before the cutover.

---

## Package Layout

```
internal/v2/
  stage/             # Stage contracts, retry policies, telemetry models
  stage/plan         # Plan stage that turns specs into work statements
  stage/decompose    # Spec-level bead decomposition
  stage/gate         # Relevance gate plus dependency guardrails
  stage/build        # LLM authoring, methodology selection, escalation
  stage/validate     # Programmatic verification of worktrees
  stage/review       # Optional LLM review and secondary bead creation
  stage/epilogue     # Bead lifecycle, status updates, artifact publishing
  stage/accept       # Spec-level acceptance gate and remediation kickoff
  stage/present      # Product-owner presentation generator
  adapter/           # Bridges to Git, LLM, task tracker, presenter, etc.
  adapter/git        # Executes git operations in isolated worktrees
  adapter/llm        # Provides Claude/LLM runs with model escalation
  adapter/tasktracker# bd client adapter for recording work
  adapter/presenter  # GitHub/presentation helpers
  loop/              # SpecLoop and BeadLoop orchestration
  loop/run2_components.go # Wiring for Run2 command (Gate/Build/Validate/Review/Epilogue)
  event/             # Typed events consumed by subscribers
  dep/               # Spec dependency gate and DAG traversal
  prompt/            # Prompt fragments, templates, and combinators
  presentation/      # Summary and remediation builders
  spec/              # Spec metadata loader and validation helpers
  testutil/          # Test doubles for adapters, loops, events
```

`cmd/gromit/run2.go` is the CLI entry point for the v2 loop. It builds the adapters, dependency gate, loop components, and event subscribers before invoking `loop.SpecLoop.Run` for each requested spec.

---

## Stage Interface

Stages live under `internal/v2/stage` and share a concise contract:

```go
func (s Stage) Name() string
func (s Stage) Run(context.Context, *StageRequest) (*StageResult, error)
```

`StageRequest` carries the bead metadata, iteration count, configuration, worktree path, remediation flag, retry context, and telemetry summary inherited from earlier stages. `StageResult` bundles a `Decision` (`Proceed`, `Skip`, `Block`, or `Fail`), optional typed `Artifacts`, and emitted `event.TypedEvent`s. The loop type-asserts `Artifacts` where needed (e.g., the `Decompose` stage returns a list of beads). Stages remain stateless; every invocation receives the context it needs and reports exactly what the loop should do next.

---

## Two-Level Loop

The run loop is split between `SpecLoop` and `BeadLoop`.

**SpecLoop** (`internal/v2/loop/spec_loop.go`)
1. Resolve spec dependencies via `dep.SpecDependencyGate` to ensure prerequisite specs are satisfied.
2. Check out an isolated worktree for the spec.
3. Emit the `plan` stage by calling the task-tracker-aware LLM adapter and persist the plan.
4. Run `Decompose` to emit beads ordered with dependency metadata.
5. Feed the produced beads into the inner `BeadLoop` for execution.
6. Invoke `Accept` to enforce spec-level acceptance criteria (and trigger remediation runs or escalation if needed).
7. Run `Present` to publish summaries and telemetry.
8. Clean up the worktree and emit completion events.

**BeadLoop** (`internal/v2/loop/bead_loop.go`)
1. Pick the next bead whose dependencies are satisfied.
2. Execute Gate, Build, Validate, Review, and Epilogue stages sequentially.
3. Gate may skip or block beads; Validate failures trigger retries that rerun Build before re-validating.
4. The loop tracks bead generations so review-created beads are labeled with `gen+1`, acceptance-created beads start a fresh generation, and a default cap prevents runaway retries.

Subscribing components observe typed events emitted by both loops and by individual stages.

---

## CLI & Workflow

`gromit run2 <spec>` is the sanctioned way to execute v2. The command:

1. Loads configuration and resolves spec files (single spec or `--epic`).
2. Builds the dependency gate (`dep.NewSpecDependencyGate`) to enforce spec ordering.
3. Assembles adapters for Git, LLM, TaskTracker, and Presenter.
4. Calls `loop.NewRun2LoopComponents` to construct the `Decompose` stage plus the inner `BeadLoop` (Gate, Build, Validate, Review, Epilogue) and the typed-event emitter funding the subscribers.
5. Starts the subscriber mesh via `startRun2Subscribers`, routing events to the CLI output and log files.
6. Instantiates `loop.SpecLoop` for each spec with the prepared adapters, gate, and bead loop, then runs it under the provided context.

Signals (`SIGINT`, `SIGTERM`) and stop channels ensure clean shutdown. `run2` also streams CLI output while collecting telemetry, keeping the legacy `gromit run` untouched until v2 acceptance is proven.

---

## Migration Sequence

1. Scaffold `internal/v2/` with stage interfaces, adapters, and an empty loop.
2. Implement the two-level loop (spec + bead) and wire typed events.
3. Implement the stage packages (Plan, Decompose, Gate, Build, Validate, Review, Epilogue, Accept, Present) under `internal/v2/stage` with TDD.
4. Add `cmd/gromit/run2` to wire adapters, dependency gate, loop components, and subscribers.
5. Run the acceptance suite under `internal/runner/acceptance/` against the v2 loop to validate behavioral parity.
6. Once v2 satisfies the acceptance contract, delete the legacy `internal/runner/` implementation and rename `run2` → `run` for the main CLI entry point.

---

## Testing Strategy

- Each `internal/v2/stage/*` package has focused unit tests that exercise the stage contract via fake adapters from `internal/v2/testutil/`.
- The two-level loop has integration and acceptance coverage (`internal/v2/loop/spec_loop_test.go`, `internal/v2/loop/bead_loop_test.go`, `internal/v2/acceptance_*.go`) to ensure ordering, retries, and event emission.
- The `internal/runner/acceptance` suite remains the behavioral contract for cutover; these tests now exercise the v2 loop through the CLI adapters.

---

## Structural Constraint

The new architecture enforces that no stage package imports `loop`. Stages only depend on `stage.Stage` and their own helpers; the orchestrator (`loop.SpecLoop`/`loop.BeadLoop`) wires everything together. This prevents the old god-object anti-pattern because the compiler prohibits attaching new state directly to the orchestrator — every new capability must live behind a stage contract.
