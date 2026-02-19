---
id: pipeline-renderer-segregation
source_ideas: []
created: 2026-02-19
---

# Segregate pipeline.PromptRenderer into Per-Workflow Interfaces

## Specification

Replace the monolithic `pipeline.PromptRenderer` interface with five single-method interfaces, one per workflow. Each CLI command and pipeline workflow depends only on the renderer it uses. No implementer stubs unused methods.

### New interfaces in `internal/pipeline/pipeline.go`

```go
type RefineRenderer interface {
    RenderRefine(input *RefinePromptInput) (string, error)
}

type PlanRenderer interface {
    RenderPlan(input *PlanPromptInput) (string, error)
}

type DecomposeRenderer interface {
    RenderDecompose(input *DecomposePromptInput) (string, error)
}

type ReviewRenderer interface {
    RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error)
}

type ExploreRenderer interface {
    RenderExplore(input *ExplorePromptInput) (string, error)
}
```

### Deps struct changes

The single `PromptRenderer` field becomes five optional fields:

```go
type Deps struct {
    AgentResolver     AgentResolver
    ClaudeClient      ClaudeClient
    BeadClient        BeadClient
    BacklogClient     BacklogClient
    RefineRenderer    RefineRenderer    // used by Refine
    PlanRenderer      PlanRenderer      // used by Plan
    DecomposeRenderer DecomposeRenderer // used by Decompose
    ReviewRenderer    ReviewRenderer    // used by Review
    ExploreRenderer   ExploreRenderer   // used by Explore
    LearningsManager  LearningsManager
    StateManager      StateManager
    LogWriter         LogWriter
}
```

### Workflow validation changes

Each workflow validates its specific renderer at entry:

```go
// Before
if p.deps.PromptRenderer == nil {
    return nil, fmt.Errorf("pipeline: nil PromptRenderer")
}
renderedPrompt, err := p.deps.PromptRenderer.RenderThoroughReview(reviewCtx)

// After
if p.deps.ReviewRenderer == nil {
    return nil, fmt.Errorf("pipeline: nil ReviewRenderer")
}
renderedPrompt, err := p.deps.ReviewRenderer.RenderThoroughReview(reviewCtx)
```

### CLI adapter simplification

`cliPromptRenderer` in `cmd/gromit/review.go` drops from 5 methods to 1 — it implements only `ReviewRenderer`. The four stub methods returning `fmt.Errorf("not implemented")` are deleted.

`explorePromptRenderer` in `cmd/gromit/explore.go` drops from 5 methods to 1 — it implements only `ExploreRenderer`. The four stub methods are deleted.

### Test mock simplification

Test mocks shrink to single-method implementations:

```go
// Before: 5 methods, 4 returning "not implemented"
type testPromptRenderer struct{}

// After: 1 method per mock
type testReviewRenderer struct{}
func (m *testReviewRenderer) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
    return "rendered", nil
}
```

The composite `PromptRenderer` interface is deleted entirely. No composite alias is added — if a future need arises, embedding the small interfaces is trivial.

## Acceptance Criteria

- The `PromptRenderer` interface is deleted from `internal/pipeline/pipeline.go`
- Five single-method interfaces exist: `RefineRenderer`, `PlanRenderer`, `DecomposeRenderer`, `ReviewRenderer`, `ExploreRenderer`
- `Deps` struct has five renderer fields replacing the single `PromptRenderer` field
- `ReviewInteractive` and `ReviewNonInteractive` use `deps.ReviewRenderer`
- `Explore` uses `deps.ExploreRenderer`
- `cliPromptRenderer` in `cmd/gromit/review.go` implements only `ReviewRenderer` (1 method, no stubs)
- `explorePromptRenderer` in `cmd/gromit/explore.go` implements only `ExploreRenderer` (1 method, no stubs)
- All test mocks updated — no mock implements more methods than its workflow requires
- `go build ./...` compiles without errors
- All existing tests pass (`go test ./...`)

## Decisions

1. **Single-method interfaces over grouped interfaces.** Each workflow uses exactly one render method. Go convention favors small interfaces — `io.Reader`, `fmt.Stringer`, `analyzer.PromptRenderer` (1 method) in this codebase. Grouping methods that are never called together defeats the purpose of interface segregation.

2. **Per-workflow Deps fields over a single composite field.** A composite field reintroduces the original problem: callers must provide an object satisfying all methods even when they use one. Separate fields let each CLI command wire only the renderer it needs. Nil fields for unused renderers are expected and validated per-workflow.

3. **No composite alias.** YAGNI. Nothing in the current or planned architecture needs all five renderers simultaneously. If that changes, composing via embedding is a one-line addition.

4. **Design for target state.** All five interfaces are defined now, even though only `ReviewRenderer` and `ExploreRenderer` have production callers today. The pipeline-extraction spec plans to wire refine, plan, and decompose through the pipeline. Defining the interfaces now avoids a second refactor.

## Research & Context

### Current State

- **`internal/pipeline/pipeline.go`** defines `PromptRenderer` with 5 methods (lines 146-153). Two production implementations exist: `cliPromptRenderer` (review) and `explorePromptRenderer` (explore). Each implements 1 method and stubs the other 4 with `fmt.Errorf("not implemented")`.

- **`internal/pipeline/explore.go`** calls `deps.PromptRenderer.RenderExplore()` (line 42). **`internal/pipeline/pipeline.go`** calls `deps.PromptRenderer.RenderThoroughReview()` (lines 200, 261). No other render methods are called in pipeline code.

- **Refine and decompose** build prompts directly via helper functions, bypassing `PromptRenderer` entirely. **Plan** is not yet implemented.

- **Test mocks** (`mocks_test.go`, `explore_test.go`, `review_test.go`, `typed_interfaces_test.go`) all implement the full 5-method interface. Most methods return empty strings or "not implemented" errors.

### Precedent in the Codebase

The codebase already follows the small-interface pattern:
- `analyzer.PromptRenderer` — 1 method (`RenderAnalyze`)
- `reviewpkg.PromptRenderer` — 4 methods scoped to review operations
- `runner.PromptRenderer` — 16+ methods for the full build loop (appropriate given the runner's breadth)

### Scale of Change

- **1 interface deleted** (`PromptRenderer`)
- **5 interfaces added** (single-method each)
- **1 struct updated** (`Deps` — replace 1 field with 5)
- **2 CLI adapters simplified** (delete 4 stub methods each)
- **~5 test files updated** (mock types shrink)
- **0 behavioral changes** — all existing callers use the same method signatures
