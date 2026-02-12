---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T15:20:57-05:00"
id: pipeline-concrete-types
source_spec: pipeline-concrete-types
---

# Replace Pipeline interface{} Returns with Concrete Types — Implementation Plan

**Goal:** Replace all `interface{}` usages in pipeline dependency interfaces with pipeline-local concrete types, eliminating runtime type assertions and reflection.

**Architecture:** Define lightweight structs (`ClaudeRunResult`, `BeadInfo`, `ThoroughReviewInput`) in the pipeline package that carry only the fields workflows need. Update the 4 affected interfaces (`ClaudeClient`, `BeadClient`, `PromptRenderer`, `LogWriter`) to use these types. Simplify all adapters, workflows, and test mocks accordingly.

**Tech Stack:** Go

**Spec:** `.gromit/specs/pipeline-concrete-types.md`

---

## Architecture

### New Types in `internal/pipeline/pipeline.go`

```go
// ClaudeRunResult holds the fields the pipeline needs from a Claude invocation.
type ClaudeRunResult struct {
    Success  bool
    ExitCode int
    Output   string
}

// BeadInfo holds the fields the pipeline needs from a bead operation.
type BeadInfo struct {
    ID       string
    Title    string
    Priority int
    Labels   []string
}

// ThoroughReviewInput holds the fields needed to render a thorough review prompt.
type ThoroughReviewInput struct {
    Diff  string
    Model string
}
```

### Interface Changes

**ClaudeClient:**
```go
Run(prompt string, model string) (*ClaudeRunResult, error)
```

**BeadClient:**
```go
Ready() (*BeadInfo, error)
Show(id string) (*BeadInfo, error)
Create(title string, priority int, labels []string, outputs []string) (*BeadInfo, error)
CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error)
Close(id string) error  // unchanged
```

**PromptRenderer:**
```go
RenderRefine(input interface{}) (string, error)       // unchanged — no workflow calls this
RenderPlan(input interface{}) (string, error)          // unchanged — no workflow calls this
RenderDecompose(input interface{}) (string, error)     // unchanged — no workflow calls this
RenderThoroughReview(ctx *ThoroughReviewInput) (string, error)  // typed
```

**LogWriter:**
```go
Write(entry interface{}) error  // unchanged per Decision 3
```

### Data Flow

```
claude.Client.Run() -> *claude.Result
  -> claudeClientAdapter.Run() -> *pipeline.ClaudeRunResult
    -> Pipeline.Decompose() uses result.Success, result.Output directly

bead.Client.CreateWithDepsAndDescription() -> *bead.Bead
  -> beadClientAdapter.CreateWithDepsAndDescription() -> *pipeline.BeadInfo
    -> Pipeline.Decompose() uses result.ID directly (no extractBeadID)
```

### Key Decisions

1. **Pipeline-local types** — avoids importing `internal/bead` and `internal/claude` into pipeline
2. **Only type RenderThoroughReview** — the other 3 render methods are never called by any workflow
3. **Keep LogWriter.Write() as `any`** — write-only sink, never inspected
4. **Delete extractBeadID entirely** — reflection no longer needed with typed returns

---

## Test Strategy

### Test Levels

1. **Compile-time checks**: Existing `var _ Interface = (*mock)(nil)` assertions in `mocks_test.go` verify all mocks satisfy updated interfaces
2. **Acceptance tests**: Existing tests in `decompose_test.go` and `review_test.go` cover full workflows — mock signatures update, test logic stays the same
3. **Build verification**: `go build ./...` catches any missed adapter or caller

### Mocking Strategy

- Update existing mock func signatures from `(interface{}, error)` to `(*ClaudeRunResult, error)` / `(*BeadInfo, error)`
- Replace `map[string]interface{}` construction in mock return values with typed struct literals
- Delete `decomposeAcceptanceBeadDef` struct (superseded by `BeadInfo`)
- No new mocks or test files needed

### Coverage Goals

- All existing tests pass with identical assertions — this is a pure refactor
- `extractBeadID` code path tested via existing dependency mapping tests (now simpler)
- Adapter construction tested implicitly by full workflow tests

---

## Implementation Tasks

### Task 1: Define pipeline-local types and update interface signatures

**Files:**
- Modify: `internal/pipeline/pipeline.go`

**What to Do:**
Add `ClaudeRunResult`, `BeadInfo`, and `ThoroughReviewInput` structs. Update `ClaudeClient.Run()` to return `(*ClaudeRunResult, error)`. Update `BeadClient.Ready()`, `Show()`, `Create()`, `CreateWithDepsAndDescription()` to return `(*BeadInfo, error)`. Update `PromptRenderer.RenderThoroughReview()` to take `*ThoroughReviewInput`. Keep `LogWriter.Write(entry interface{})` unchanged. Keep `RenderRefine`, `RenderPlan`, `RenderDecompose` as `interface{}`.

**Acceptance Criteria:**
- `ClaudeRunResult`, `BeadInfo`, and `ThoroughReviewInput` structs exist in pipeline package
- `ClaudeClient`, `BeadClient`, and `PromptRenderer` interfaces use typed signatures
- `LogWriter.Write()` still takes `interface{}`

**Dependencies:** None

---

### Task 2: Update all test mocks to satisfy new interface signatures

**Files:**
- Modify: `internal/pipeline/mocks_test.go`
- Modify: `internal/pipeline/decompose_test.go`
- Modify: `internal/pipeline/review_test.go`

**What to Do:**
Update all mock types to match new interface signatures. Replace `map[string]interface{}` returns with typed struct construction. Update mock func types. Delete `decomposeAcceptanceBeadDef` (replaced by `BeadInfo`).

**Acceptance Criteria:**
- All mocks satisfy their respective interfaces (compile-time checks pass)
- All mock construction sites use typed structs
- `decomposeAcceptanceBeadDef` struct deleted

**Dependencies:** Task 1

---

### Task 3: Simplify decompose and review workflows

**Files:**
- Modify: `internal/pipeline/decompose.go`
- Modify: `internal/pipeline/pipeline.go` (ReviewNonInteractive method)

**What to Do:**
Remove `reflect` import from `decompose.go`. Replace `claudeResult.(map[string]interface{})` with direct field access in both Decompose and ReviewNonInteractive. Replace `extractBeadID(beadResult)` with `beadResult.ID`. Delete `extractBeadID` function. Update log entry construction in ReviewNonInteractive if needed.

**Acceptance Criteria:**
- No `reflect` import in `decompose.go`
- No `.(map[string]interface{})` type assertions in decompose.go or pipeline.go
- `extractBeadID` function deleted
- Direct field access used everywhere

**Dependencies:** Task 1

---

### Task 4: Update review helpers to use typed structs

**Files:**
- Modify: `internal/pipeline/review.go`

**What to Do:**
Change `buildThoroughReviewContext()` return type from `map[string]interface{}` to `*ThoroughReviewInput`. Update body to construct typed struct.

**Acceptance Criteria:**
- `buildThoroughReviewContext` returns `*ThoroughReviewInput`
- Callers in pipeline.go work with typed return

**Dependencies:** Task 1

---

### Task 5: Simplify CLI adapters

**Files:**
- Modify: `cmd/gromit/decompose.go`
- Modify: `cmd/gromit/review.go`

**What to Do:**
Update `claudeClientAdapter.Run()` and `cliClaudeClient.Run()` to return `(*pipeline.ClaudeRunResult, error)`. Update all `beadClientAdapter` and `cliBeadClient` methods to return `(*pipeline.BeadInfo, error)`. Update `cliPromptRenderer.RenderThoroughReview()` to accept `*pipeline.ThoroughReviewInput` instead of casting from `interface{}`.

**Acceptance Criteria:**
- No `map[string]interface{}` construction in any adapter
- No runtime type assertions in adapters
- `go build ./...` compiles

**Dependencies:** Task 1

---

### Task 6: Verify all tests pass

**Files:** None (verification only)

**What to Do:**
Run `go build ./...` and `go test ./...`. Fix any remaining issues.

**Acceptance Criteria:**
- `go build ./...` succeeds
- `go test ./...` passes all tests

**Dependencies:** Tasks 2, 3, 4, 5

---

## Notes

- Tasks 2-5 are independent of each other but all depend on Task 1. They can be done in any order after Task 1.
- The `PromptRenderer` methods `RenderRefine`, `RenderPlan`, `RenderDecompose` are left as `interface{}` because no pipeline workflow calls them. Typing them now would create dead prompt input types. They should be typed when the corresponding workflows are implemented.
- The `decomposeAcceptanceBeadDef` struct in `decompose_test.go` has the same fields as `BeadInfo` plus `Deps` — tests that use the `Deps` field should be checked to ensure they don't need it on the return type (they pass deps as a parameter, not read from the return).
