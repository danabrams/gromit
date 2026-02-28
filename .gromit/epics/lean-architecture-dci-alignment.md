---
epic_id: lean-architecture-dci-alignment
created: 2026-02-28
---

# Lean Architecture & DCI Alignment

## Problem

Gromit's core orchestration logic is beginning to suffer from "monolithic context" and "interface bloat." The `Orchestrator` manages multiple execution methodologies (Simple, TDD, Experimentation) through complex internal conditionals, while the `PromptRenderer` and `pipeline.Input/Output` have become "God Interfaces" and "Wide DTOs." This coupling makes it difficult to add new methodologies or evolve specific stages without impacting the entire pipeline.

## Vision

Align Gromit's internal structure with James Coplien's **Lean Architecture** and **DCI (Data, Context, and Interaction)** principles. The codebase should explicitly separate stable Data (Beads), specialized Contexts (Methodologies), and meaningful Roles (Interactions). This will reduce cognitive load, improve testability, and ensure the system's "Function" (what it does for the user) remains more prominent than its "Form" (technical implementation details).

## Scope

- **Interface Segregation:** Split the monolithic `PromptRenderer` into specialized, role-based interfaces (e.g., `AuthoringPrompts`, `PlanningPrompts`, `ReviewPrompts`).
- **DCI Context Extraction:** Extract methodology-specific logic (e.g., the TDD cycle, simple run-loop) from the `Orchestrator` into standalone "Use Case Context" objects.
- **Role-Based Interaction:** Introduce Role interfaces for `bead.Bead` (e.g., `Authorable`, `Validatable`) so pipeline stages interact only with the specific behaviors they require.
- **Input/Output Narrowing:** Reduce the width of `pipeline.Input` and `pipeline.Output` by using composition or stage-specific subsets, ensuring "Lean" data transfer across package boundaries.
- **Mental Model Alignment:** Audit the naming and structure of the `internal/runner` and `internal/pipeline` packages to ensure they reflect the user's strategic mental model of engineering.

## Tasks

### Phase 1: Interface Segregation (Lean-out)
- [ ] Decompose `PromptRenderer` in `internal/runner/interfaces.go` into focused interfaces.
- [ ] Update `internal/prompt/renderer.go` and its consumers to use the segregated interfaces.
- [ ] Identify and prune "Header Interfaces" that are only used by a single implementation.

### Phase 2: DCI & Role Extraction
- [ ] Define `Authorable` and `Validatable` Roles for the `Bead` data object.
- [ ] Refactor the `Build` and `Validate` stages to accept these Roles instead of the raw `Bead` struct.
- [ ] Create a `Methodology` interface and extract the TDD logic from `orchestrator.go` into a `TDDContext`.

### Phase 3: Narrowing the DTOs
- [ ] Audit `pipeline.Input` usage; identify fields used by only a subset of stages.
- [ ] Refactor `Stage.Run` signature to use more targeted input structures (e.g., through role-casting or composition).
- [ ] Clean up `pipeline.Output` to prevent "telemetry leakage" (technical details like cache keys being carried through high-level business logic).

### Phase 4: Validation & Alignment
- [ ] Verify that the `Orchestrator` is reduced to a high-level "Script" of the run-loop use case.
- [ ] Ensure unit tests for stages no longer require mocking the entire `PromptRenderer`.
- [ ] Review `LEARNINGS.md` and `retro` logic for alignment with Coplien's organizational learning patterns.
