# Runner Pipeline Refactor Design

**Date:** 2026-02-21
**Status:** Proposed

---

## Background

Gromit's `Runner` struct has accumulated roughly 40 fields spread across more than 20 files. The `callbacks.go` file alone runs to 756 lines and is exercised by every execution path through the system. An earlier round of sub-package splits — `escalation/`, `methodology/`, `validation/`, `execution/` — provided some organizational relief but did not decouple state. The `Runner` struct still owns everything. Each new feature found it psychologically easy to attach another field to `Runner` or another method to `callbacks.go`, and the accumulation continued.

The fix is a different object model. Stages receive explicit inputs and return explicit outputs. The orchestrator holds no mutable state beyond its wiring of stages together. State flows through data, not through a shared struct.

---

## Decision: Full Test Reset with Acceptance Preservation

The 170 non-acceptance test files in `internal/runner/` test internal wiring rather than observable behavior. They encode the current implementation's shape — field names, method signatures, callback sequencing — and will need to be rewritten regardless of how carefully a refactor is approached. Carrying them forward would anchor the new design to the old structure.

The `//go:build acceptance` tagged files are different. They represent behavioral contracts: given this environment and configuration, the pipeline must produce this outcome. These files are preserved by extracting them to `internal/runner/acceptance/` and become the behavioral contract the new pipeline must satisfy. The migration is complete when the acceptance suite passes against the new implementation.

---

## Package Layout

```
internal/pipeline/          ← stage interfaces and shared types only
    stage.go                ← Stage interface, Input, Output types
    prepare/                ← stage 1: gate decisions
    execute/                ← stage 2: LLM code authoring
    validate/               ← stage 3: programmatic test/lint
    review/                 ← stage 4: LLM code review (optional)
    epilogue/               ← stage 5: bead lifecycle and cleanup

internal/runner/
    orchestrator.go         ← wires stages, runs the loop
    constructor.go          ← builds pipeline from config
    acceptance/             ← behavioral tests extracted from runner_test.go
```

The core structural constraint is that `internal/runner/` imports `internal/pipeline/` only. No stage package imports `runner/`. This import cycle enforcement is the mechanical guarantee against re-accumulation. When a developer wants to add a new capability, they cannot attach it to the orchestrator — they must define a stage interface, implement it, and register it in the constructor. The compiler prevents shortcuts.

---

## Stage Interface

```go
type Stage interface {
    Run(ctx context.Context, in Input) (Output, error)
}

type Input struct {
    Bead             *bead.Bead
    Config           *config.Config
    Iteration        int
    Deadline         time.Time
    ValidationFails  []string  // fed back from prior Validate failures
    // accumulated context from prior stages
}

type Output struct {
    Decision         Decision  // for Gate: Proceed | Skip | Block
    // stage-specific fields
}
```

`Input` carries everything a stage needs to execute. `Output` carries everything subsequent stages or the orchestrator need to proceed. Neither type is mutable once constructed. The orchestrator merges `Output` from one stage into the `Input` for the next.

---

## The Five Stages

**Stage 1: Gate (programmatic)**

Gate is the only place early exit is permitted. It runs precheck, stuck check, scope gate, and proactive decomposition. It returns one of three decisions: `Proceed`, `Skip`, or `Block`. If the decision is `Skip` or `Block`, the orchestrator exits the current iteration without running any further stages. Gate contains no LLM calls — it is entirely programmatic and must execute quickly and deterministically.

**Stage 2: Build (LLM)**

Build handles prompt construction, methodology selection (TDD, refactor, standard), and Claude invocation. The escalation chain (haiku → sonnet → opus) is internal to Build. The orchestrator sees only success or failure, not the individual escalation attempts. Build returns the result of the LLM authoring pass, the model that produced the successful output, and the diff applied to the working tree.

**Stage 3: Validate (programmatic)**

Validate runs `go test`, `golangci-lint`, and auto-fix passes (gofmt, goimports). It also handles periodic full validation when configured. On failure, Validate returns a `ValidationFailed` result with structured failure summaries. The orchestrator passes these summaries into the next iteration's Build `Input`. No stage holds failure state between iterations — the summaries travel through the data flow.

**Stage 4: Review (LLM, optional)**

Review invokes a code review pass and generates new beads from the findings. It only runs when configured. Review returns the review output and the IDs of any new beads created. Because it is optional, the orchestrator checks the configuration before including it in the stage sequence for a given iteration.

**Stage 5: Epilogue (programmatic)**

Epilogue closes and syncs beads, evaluates the spec gate, merges the worktree if appropriate, writes status, triggers a thorough review when conditions warrant, and runs the between-iterations command. Epilogue marks the iteration complete. It is always the last stage when it runs.

---

## Data Flow

The pipeline runs sequentially: Gate → Build → Validate → Review → Epilogue.

Each stage's `Output` is merged into the next stage's `Input`. Validate failure feeds back into Build `Input` on the next iteration attempt. No stage holds mutable state between iterations — all state flows through `Input`/`Output` structs or loop-level state owned by the orchestrator. The orchestrator's loop-level state is limited to what is needed to construct the next iteration's `Input`: the current bead, the iteration counter, the deadline, and the accumulated validation failure summaries.

---

## Migration Sequence

The migration proceeds in six steps, each leaving the codebase in a working state.

First, extract the `//go:build acceptance` tagged files from `internal/runner/` to `internal/runner/acceptance/`. These files must pass before the migration begins and must continue to pass at every subsequent step.

Second, delete all other `internal/runner/*_test.go` files. This removes the implementation-coupled tests that would otherwise anchor the refactor to the old structure.

Third, create the `internal/pipeline/` skeleton with the five stage subdirectories and placeholder files. No logic is written at this step — only the directory and interface structure.

Fourth, specify the `Stage` interface and the `Input`/`Output` types in `internal/pipeline/stage.go`. This is a design decision point; the types should be reviewed before implementation proceeds.

Fifth, implement each stage TDD against its interface. No stage package imports `internal/runner/`. Each stage package contains its own tests using fakes for dependencies. The five stages can be implemented in sequence or in parallel by different contributors.

Sixth, wire the orchestrator in `orchestrator.go` and `constructor.go`, then run the acceptance suite. The migration is complete when the acceptance suite passes.

---

## Testing Strategy

Each stage package contains its own unit tests. Dependencies are provided through fakes that satisfy the relevant interfaces — no stage test imports `internal/runner/`. This keeps stage tests fast and isolated from the orchestrator's wiring.

The `internal/runner/acceptance/` package runs end-to-end against the assembled pipeline. These tests are the behavioral contract. They are the authoritative answer to whether the new pipeline is equivalent to the old one. They are not a supplement to unit tests — they are the migration gate.

---

## Structural Constraint

The God Object reformed because the `Runner` namespace made it easy to attach new methods without confronting the question of where they belonged. The `internal/pipeline/` top-level stage layout enforces a hard boundary. A new feature cannot attach to the orchestrator. It must define its stage interface, implement it in a stage package, and register it in the constructor. The compiler rejects any attempt to shortcut this by importing `runner/` from a stage package. The mechanical enforcement is what makes the constraint durable across contributors and time.
