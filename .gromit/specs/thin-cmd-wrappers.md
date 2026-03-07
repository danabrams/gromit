---
id: thin-cmd-wrappers
source_ideas: []
created: 2026-02-27
accepted: true
---

# Thin CMD Wrappers

## Specification

All CLI commands in `cmd/gromit/` become thin wrappers that handle only:

1. **Flag parsing** — Cobra flag definitions and argument validation
2. **User interaction** — interactive pickers, confirmations, progress display
3. **Output formatting** — rendering results as text for the terminal

All business logic moves to `internal/pipeline/`, which becomes the product API that any interface (CLI, TUI, API server) can call. The pipeline layer exposes:

- **Workflow methods** — Refine, Plan, Decompose, Review, Explore, and any other operations that currently embed logic in cmd/
- **Query methods** — listing available specs, plans, ideas, beads, and other data needed for selection UIs, with filtering logic included
- **Scope resolution** — determining review scope from various inputs (commit range, spec label, epic label, state fallback)

Each interface provides its own implementations of the `Deps` interfaces and its own presentation layer. The pipeline package never imports anything from `cmd/` or any interface-specific package.

The adapter pattern already used by some commands (typed wrappers that satisfy pipeline interfaces) remains, but adapters live in each interface's package, not in pipeline/.

## Acceptance Criteria

- Every cmd/gromit/ command file contains only flag parsing, user interaction (pickers/confirmations), and text output formatting — no business logic
- `internal/pipeline/` exposes query methods for listing available specs, plans, ideas, and beads with filtering, so interfaces don't need to implement selection logic
- Review scope resolution (--since, --spec, --epic, state fallback) lives in pipeline/, not cmd/
- Prompt construction (system prompt building with context embedding) lives in pipeline/, not cmd/
- Agent launching for all workflows goes through pipeline methods, not direct agent calls from cmd/
- `Plan()` pipeline method is fully implemented (currently stubbed)
- A new interface (TUI or API server) could be built by importing `internal/pipeline/` without duplicating any logic from `cmd/gromit/`
- `internal/pipeline/` has no imports from `cmd/` or any interface-specific package

## Decisions

1. **Pipeline as product API** The `internal/pipeline/` package becomes the single entry point for all business logic. This was chosen over alternatives (e.g., per-domain packages like `internal/review/`, `internal/planning/`) because the pipeline package and Deps pattern already exist and are partially adopted — this refactoring finishes the migration rather than creating a new abstraction.

2. **Query methods on pipeline** The pipeline exposes data-listing methods (available specs, plans, ideas) with filtering built in. Interfaces call these to get candidates, then present them however they want (CLI picker, TUI list, API JSON response). This keeps selection logic centralized while leaving presentation to each frontend.

3. **All commands, not just major ones** Every command gets refactored, including simpler ones like board, queue, add, and stats. This ensures consistency — any interface can offer the full feature set without cherry-picking which commands were migrated.

4. **Adapters stay in interface packages** Each interface (CLI, TUI, API) defines its own adapter types that satisfy pipeline's Deps interfaces. This keeps pipeline clean and lets each interface wire dependencies its own way.

## Research & Context

### Current State

The migration is partially underway:

- **`internal/pipeline/`** exists with a `Pipeline` struct, `Deps` struct with typed interfaces, and methods for Refine, Decompose, Review (interactive + non-interactive), and Explore. `Plan()` is stubbed.
- **Well-migrated commands:** `refine.go`, `explore.go`, and `review.go` (non-interactive path) already delegate to pipeline methods, though review.go still has scope resolution logic in cmd/.
- **Partially migrated:** `review.go` (917 lines) has complex 4-tier scope resolution and bead lookup logic in cmd/. `decompose.go` (640 lines) has plan picker filtering. `plan.go` (296 lines) builds prompts and launches agents directly.
- **Not yet migrated:** Simpler commands (`board.go`, `queue.go`, `add.go`, `stats.go`, `triage.go`, `resolve.go`, etc.) call internal packages directly rather than going through pipeline.
- **Adapter pattern** is established — commands create typed wrappers satisfying pipeline interfaces — but adapters are defined inline in cmd/ files.

### Key Files

- `cmd/gromit/` — 35 Go source files, one per subcommand
- `internal/pipeline/pipeline.go` — Core Pipeline struct and interface definitions
- `internal/pipeline/refine.go`, `decompose.go`, `explore.go` — Implemented workflow methods
- `internal/pipeline/review/` — Review workflow subdirectory
- `cmd/gromit/review.go` — Largest cmd file (917 lines), most business logic to extract
- `cmd/gromit/plan.go` — Prompt construction and direct agent launching to migrate
- `cmd/gromit/cli_adapters.go` — Shared adapter implementations
