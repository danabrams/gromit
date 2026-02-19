---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T09:39:16Z"
id: pipeline-renderer-segregation
source_spec: pipeline-renderer-segregation
---

# Segregate pipeline.PromptRenderer into Per-Workflow Interfaces

**Goal:** Replace the monolithic 5-method `PromptRenderer` interface with five single-method interfaces so each workflow and adapter depends only on the renderer it uses.

**Architecture:** Define `RefineRenderer`, `PlanRenderer`, `DecomposeRenderer`, `ReviewRenderer`, `ExploreRenderer` as single-method interfaces. Replace the single `Deps.PromptRenderer` field with five optional fields. Each CLI adapter and test mock implements only the interface its workflow requires.

**Tech Stack:** Go interfaces, no new dependencies

**Spec:** `.gromit/specs/pipeline-renderer-segregation.md`

---

## Architecture

**Overview:**
The current `PromptRenderer` interface has 5 methods but every implementer uses exactly 1. CLI adapters stub 4 methods with "not implemented" errors. This violates Go's interface segregation convention. Split into five single-method interfaces — one per workflow.

**Key Components:**
1. **Five single-method interfaces** in `internal/pipeline/pipeline.go` — `RefineRenderer`, `PlanRenderer`, `DecomposeRenderer`, `ReviewRenderer`, `ExploreRenderer`
2. **Updated `Deps` struct** — five optional renderer fields replacing the single `PromptRenderer` field
3. **Simplified CLI adapters** — each implements only its workflow's interface (1 method, no stubs)
4. **Simplified test mocks** — per-workflow mock types with 1 method each

**Integration Points:**
- ReviewInteractive/ReviewNonInteractive use `deps.ReviewRenderer`
- Explore uses `deps.ExploreRenderer`
- `cliPromptRenderer` (review.go) implements only `ReviewRenderer`
- `explorePromptRenderer` (explore.go) implements only `ExploreRenderer`
- RefineRenderer, PlanRenderer, DecomposeRenderer defined but not yet wired (future pipeline extraction)

**Current State:**
- `PromptRenderer` defined at pipeline.go:146-153 with 5 methods
- `Deps.PromptRenderer` at pipeline.go:79
- Production call sites: `deps.PromptRenderer.RenderThoroughReview()` (pipeline.go:200, 261) and `deps.PromptRenderer.RenderExplore()` (explore.go:42)
- Validation checks: pipeline.go:189, 354 and explore.go:137
- CLI adapters: `cliPromptRenderer` (review.go:517-557, 4 stubs), `explorePromptRenderer` (explore.go:177-293, 4 stubs)
- Test mocks: 4 types across mocks_test.go, explore_test.go, review_test.go, typed_interfaces_test.go — all implement 5 methods

**Tradeoffs:**
- Five Deps fields over single composite: more fields but each command wires only what it needs
- No composite alias: YAGNI — embedding is trivial if needed later
- All five interfaces defined now: avoids second refactor when pipeline extraction wires refine/plan/decompose

## Test Strategy

**Approach:** This is a pure interface segregation refactor with zero behavioral change. Existing tests are the primary correctness guard — if wiring is wrong, they fail. No new behavioral tests needed.

**Compile-time checks:** Each interface gets `var _ Interface = (*ConcreteType)(nil)` for all production and test implementations.

**Verification:** `go build ./...` compiles, `go test ./...` passes.

**Mock simplification:**
- `testPromptRenderer` (5 methods) → `testReviewRenderer` + `testExploreRenderer` (1 method each)
- `mockPromptRenderer` in explore_test.go → explore-only mock
- `reviewAcceptanceMockPromptRenderer` → review-only mock
- `typedInterfacesPromptRenderer` → review-only mock

## Implementation Tasks

### Task 1: Define new interfaces and update Deps struct

**Files:**
- Modify: `internal/pipeline/pipeline.go`

**What to Do:**
Delete the `PromptRenderer` interface (lines 146-153). Add five single-method interfaces in its place:
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

Update the `Deps` struct (lines 73-83) to replace the single `PromptRenderer` field with five fields:
```go
RefineRenderer    RefineRenderer
PlanRenderer      PlanRenderer
DecomposeRenderer DecomposeRenderer
ReviewRenderer    ReviewRenderer
ExploreRenderer   ExploreRenderer
```

**Acceptance Criteria:**
- Five single-method interfaces defined in pipeline.go
- `PromptRenderer` interface deleted
- `Deps` struct has five renderer fields

**Dependencies:** None

### Task 2: Update production call sites and CLI adapters

**Files:**
- Modify: `internal/pipeline/pipeline.go` (ReviewInteractive, ReviewNonInteractive, validateReviewDeps)
- Modify: `internal/pipeline/explore.go` (Explore, validateExploreDeps)
- Modify: `cmd/gromit/review.go` (cliPromptRenderer, Deps construction at lines 341, 424)
- Modify: `cmd/gromit/explore.go` (explorePromptRenderer, Deps construction at line 153)

**What to Do:**

In pipeline.go:
- Line 189-191: Change nil check from `p.deps.PromptRenderer` to `p.deps.ReviewRenderer`
- Line 200: Change `p.deps.PromptRenderer.RenderThoroughReview` to `p.deps.ReviewRenderer.RenderThoroughReview`
- Line 261: Same change
- Line 354: Change `validateReviewDeps` map entry from `"PromptRenderer": p.deps.PromptRenderer` to `"ReviewRenderer": p.deps.ReviewRenderer`

In explore.go:
- Line 42: Change `p.deps.PromptRenderer.RenderExplore` to `p.deps.ExploreRenderer.RenderExplore`
- Line 137: Change validation map entry to `"ExploreRenderer": p.deps.ExploreRenderer`

In cmd/gromit/review.go:
- Delete 4 stub methods from `cliPromptRenderer` (RenderRefine, RenderPlan, RenderDecompose, RenderExplore — lines 523-529, 555-557)
- Keep only `RenderThoroughReview` (lines 535-553)
- Update Deps construction at line 341: change `PromptRenderer: promptRendererAdapter` to `ReviewRenderer: promptRendererAdapter`
- Same at line 424
- Add compile-time check: `var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)`

In cmd/gromit/explore.go:
- Delete 4 stub methods from `explorePromptRenderer` (RenderRefine, RenderPlan, RenderDecompose, RenderThoroughReview — lines 182-196)
- Keep only `RenderExplore` (lines 198-293)
- Update Deps construction at line 153: change `PromptRenderer: promptRenderer` to `ExploreRenderer: promptRenderer`
- Add compile-time check: `var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)`

**Acceptance Criteria:**
- `ReviewInteractive` and `ReviewNonInteractive` use `deps.ReviewRenderer`
- `Explore` uses `deps.ExploreRenderer`
- `cliPromptRenderer` implements only `ReviewRenderer` (1 method, no stubs)
- `explorePromptRenderer` implements only `ExploreRenderer` (1 method, no stubs)
- Compile-time interface checks present for both adapters

**Dependencies:** Task 1

### Task 3: Update test mocks and Deps wiring in tests

**Files:**
- Modify: `internal/pipeline/mocks_test.go`
- Modify: `internal/pipeline/explore_test.go`
- Modify: `internal/pipeline/review_test.go`
- Modify: `internal/pipeline/typed_interfaces_test.go`
- Modify: `internal/pipeline/pipeline_test.go`
- Modify: `internal/pipeline/explore_agent_input_test.go`

**What to Do:**

In mocks_test.go:
- Replace `testPromptRenderer` (5 methods) with two mocks:
  - `testReviewRenderer` with `RenderThoroughReview` returning `("", nil)`
  - `testExploreRenderer` with `RenderExplore` returning `("", nil)`
- Update compile checks: `var _ ReviewRenderer = (*testReviewRenderer)(nil)` and `var _ ExploreRenderer = (*testExploreRenderer)(nil)`
- Delete old `var _ PromptRenderer = (*testPromptRenderer)(nil)` check

In explore_test.go:
- Replace `mockPromptRenderer` (5 fn fields, 5 methods) with explore-only mock (1 fn field, 1 method)
- Update all ~10 Deps constructions: change `PromptRenderer: ...` to `ExploreRenderer: ...`
- For sites using `&testPromptRenderer{}`, change to `&testExploreRenderer{}`

In review_test.go:
- Replace `reviewAcceptanceMockPromptRenderer` (1 fn field, 5 methods) with review-only mock (1 fn field, 1 method)
- Update all ~5 Deps constructions: change `PromptRenderer: ...` to `ReviewRenderer: ...`

In typed_interfaces_test.go:
- Replace `typedInterfacesPromptRenderer` (1 fn field, 5 methods) with review-only mock (1 fn field, 1 method)
- Update compile check and Deps construction (line 439)

In pipeline_test.go:
- Update Deps construction (line 47): change `PromptRenderer: &testPromptRenderer{}` to `ReviewRenderer: &testReviewRenderer{}`
- Update nil check (line 65): change `deps.PromptRenderer` to `deps.ReviewRenderer`

In explore_agent_input_test.go:
- Update Deps construction (line 54): change `PromptRenderer: mockRenderer` to `ExploreRenderer: mockRenderer`

**Acceptance Criteria:**
- All test mocks implement only the interface their workflow requires (no stub methods)
- All Deps constructions in tests use the correct per-workflow field names
- Compile-time interface checks updated for all mock types
- `go build ./...` compiles without errors
- `go test ./...` passes

**Dependencies:** Task 1

---

## Notes

- Tasks 2 and 3 are independent of each other but both depend on Task 1. They can be parallelized during decompose.
- This is a purely mechanical refactor. Every change is a rename or deletion. No new logic is introduced.
- The five interfaces are defined now (even though only ReviewRenderer and ExploreRenderer have production callers) because the pipeline-extraction spec will wire the other three.
