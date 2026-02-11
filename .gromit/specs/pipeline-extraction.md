---
id: pipeline-extraction
source_ideas: []
created: 2026-02-11
---

# Pipeline Extraction

## Specification

Extract workflow orchestration logic from `cmd/gromit/` command handlers into the `internal/pipeline/` package, creating a reusable business logic layer that is fully decoupled from the CLI. The pipeline package becomes the **model** in an MVC separation — it owns all orchestration, state mutations, and post-processing. CLI handlers (and future TUI, web, mobile interfaces) become thin adapters that provide input and render output.

### Workflows

Five workflows move into the pipeline package:

| Workflow | Current Handler | Modes |
|----------|----------------|-------|
| Refine | `cmd/gromit/refine.go` | Interactive + Non-interactive |
| Plan | `cmd/gromit/plan.go` | Interactive + Non-interactive |
| Decompose | `cmd/gromit/decompose.go` | Non-interactive only |
| Review | `cmd/gromit/review.go` | Interactive + Non-interactive |
| Explore | `cmd/gromit/explore.go` | Interactive + Non-interactive |

Each workflow that supports interactive mode returns a `Session` for the caller to drive. Non-interactive mode runs to completion and returns a structured result.

### Session Interface

Interactive workflows return a `Session` that uses event channels for communication:

```go
type Session interface {
    Events() <-chan Event
    SendInput(text string) error
    Cancel()
    Wait() error
}

type EventType int

const (
    EventOutput EventType = iota
    EventSessionStarted
    EventSessionEnded
    EventError
)

type Event struct {
    Type    EventType
    Content string
}
```

- **CLI adapter**: drains `Events()` to stdout, pipes stdin to `SendInput()`
- **TUI adapter**: routes events to a bubbletea model, sends input from a text widget
- **Web adapter**: serializes events as JSON over websockets/SSE

### Input/Output Structs

Each workflow gets clean input and output structs. Inputs contain everything the workflow needs — no flag parsing, no interactive picking, no terminal I/O. Outputs contain structured results — no printing, no prompting.

Examples:

```go
type RefineInput struct {
    IdeaText   string   // Ad-hoc idea text (mutually exclusive with IdeaID)
    IdeaID     string   // Backlog item ID (mutually exclusive with IdeaText)
    AgentName  string   // Optional agent override
}

type RefineResult struct {
    CreatedSpecs   []string  // Paths to new spec files
    RefinedItems   []string  // Backlog item IDs marked as refined
}

type DecomposeInput struct {
    PlanName string   // Name of plan to decompose
    Force    bool     // Re-decompose even if already done
    Review   bool     // Return proposed beads for review before creating
}

type DecomposeResult struct {
    CreatedBeads []CreatedBead  // Beads that were created
    PlanUpdated  bool           // Whether plan frontmatter was updated
}
```

### Execution Model

- All workflows accept `context.Context` as their first parameter for cancellation and timeouts
- Interactive workflows: the pipeline starts the agent session and returns a `Session` handle. The caller drives I/O through the session. Post-session processing (detecting new specs, creating beads, updating state) runs automatically when the session ends.
- Non-interactive workflows: the pipeline runs Claude, parses results, performs all post-processing (creating beads, updating frontmatter, persisting learnings), and returns a structured result.
- Explore is normalized to use `agent.Resolve()`/`Agent.Launch()` instead of direct `exec.Command`, making all interactive workflows consistent.

### Post-Processing Ownership

The pipeline package owns all post-processing and state mutations:

- **Refine**: detecting new spec files, marking backlog items as refined
- **Plan**: detecting new plan files
- **Decompose**: parsing JSON output, creating beads with dependencies and labels, updating plan frontmatter
- **Review (non-interactive)**: parsing review results, creating beads/backlog items, persisting learnings, logging, updating state
- **Explore**: detecting new artifacts (specs, epics, backlog items)

### Chaining

The current chaining system (`chain.go`) prompts the user to continue to the next pipeline stage. Since chaining involves user interaction ("Do you want to plan this spec?"), it stays in the CLI/interface layer, not in the pipeline package. The pipeline provides the data needed to offer chaining (e.g., `RefineResult.CreatedSpecs` tells the CLI which specs can be planned next).

## Acceptance Criteria

- Each of the five workflows (refine, plan, decompose, review, explore) is callable via `pipeline.<Workflow>(ctx, input)` with no dependency on cobra, os.Stdin/Stdout/Stderr, or terminal I/O
- Interactive workflows return a `Session` with event channel, input method, cancellation, and wait
- Non-interactive workflows return structured result types containing all outputs
- All post-processing (bead creation, frontmatter updates, backlog mutations, learning persistence) happens inside the pipeline package
- All workflows accept `context.Context` and respect cancellation
- Explore uses the agent abstraction instead of direct `exec.Command`
- Existing `cmd/gromit/` handlers are refactored to be thin adapters over `pipeline.<Workflow>()` calls
- Existing CLI behavior is preserved — no user-visible changes

## Decisions

1. **Event channels over callbacks** — Channels are more Go-idiomatic, compose with `select` and context cancellation, and map naturally to bubbletea's `Cmd` model for the planned TUI.

2. **Hybrid interactive/non-interactive support** — Each workflow supports both modes where it makes sense, letting the interface choose. This enables CLI and TUI to use interactive mode while API/web/mobile use non-interactive mode.

3. **Pipeline as model, not controller** — The pipeline owns business logic and state mutations. It does not own user interaction (confirmations, picking from lists, chaining prompts). Callers handle all user-facing decisions and pass resolved choices in the input structs.

4. **Normalize explore to use agent abstraction** — Explore currently bypasses `agent.Resolve()`/`Agent.Launch()` and directly runs `exec.Command`. This extraction normalizes it to match the other interactive workflows, giving one consistent execution model.

5. **Chaining stays in the interface layer** — Chaining involves user prompts ("Plan this spec?") which are interface-specific. The pipeline returns enough data in results for any interface to implement chaining however it wants.

## Research & Context

### Current State

The five workflow handlers live in `cmd/gromit/` as cobra `RunE` functions:
- `refine.go` (~200 lines) — 3 input modes, agent launch, post-session spec detection, backlog updates
- `plan.go` (~150 lines) — spec picker, agent launch, plan detection
- `decompose.go` (~250 lines) — plan picker, Claude `Run()`, JSON parsing, bead creation, frontmatter updates
- `review.go` (~300 lines) — scope resolution, dual-mode (interactive agent launch + non-interactive Claude `Run()` with result parsing)
- `explore.go` (~150 lines) — direct `exec.Command` (bypasses agent abstraction), topic injection

Supporting infrastructure:
- `chain.go` — pipeline chaining with `chainAfterRefine()`, `execGromit()` for subprocess re-invocation
- `resolve.go` — config path resolvers (`resolveGromitDir`, `resolveSpecsDir`, etc.)
- `internal/agent/` — agent abstraction with `Resolve()` and `Launch()`, 3 prompt delivery modes
- `internal/claude/` — Claude CLI wrapper with `Run()` (non-interactive) and `StreamRun()` (streaming)
- `internal/pipeline/status.go` — existing pipeline status tracking (will coexist with new orchestration code)

### Patterns to Preserve

- **Dependency injection via interfaces** — `runner/interfaces.go` defines `BeadClient`, `ClaudeClient` etc. with compile-time checks. The pipeline package should follow the same pattern.
- **Two constructor patterns** — production `New()` and test `NewWithDeps()` constructors for dependency injection in tests.
- **Nil-safe normalization** — slice fields initialized to empty slices, not nil.
- **Error wrapping** — all errors wrapped with `fmt.Errorf("context: %w", err)`.
