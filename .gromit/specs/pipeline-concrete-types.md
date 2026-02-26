---
id: pipeline-concrete-types
source_ideas: []
created: 2026-02-12
epic: codebase-health
---

# Replace Pipeline interface{} Returns with Concrete Types

## Specification

Pipeline dependency interfaces (`ClaudeClient`, `BeadClient`, `PromptRenderer`, `LogWriter`) use `interface{}` in return types and parameters. This loses compile-time type safety and forces runtime type assertions, adapter boilerplate, and reflection-based extraction logic. Replace all `interface{}` usages with pipeline-local concrete types.

The approach defines lightweight structs within the pipeline package that carry only the fields the pipeline actually needs. This avoids importing `internal/bead` and `internal/claude` into the pipeline package (preserving the pipeline's role as an abstraction layer independent of specific implementations) while still providing compile-time type safety.

### New types in `internal/pipeline/pipeline.go`

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
```

### Interface changes

**ClaudeClient:**
```go
// Before: Run(prompt string, model string) (interface{}, error)
// After:
Run(prompt string, model string) (*ClaudeRunResult, error)
```

**BeadClient:**
```go
// Before: all methods return (interface{}, error)
// After:
Ready() (*BeadInfo, error)
Show(id string) (*BeadInfo, error)
Create(title string, priority int, labels []string, outputs []string) (*BeadInfo, error)
CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error)
Close(id string) error  // unchanged
```

**PromptRenderer:**
```go
// Before: all methods take interface{}
// After:
RenderRefine(input *RefinePromptInput) (string, error)
RenderPlan(input *PlanPromptInput) (string, error)
RenderDecompose(input *DecomposePromptInput) (string, error)
RenderThoroughReview(ctx *ThoroughReviewPromptInput) (string, error)
```

Where each prompt input type is a pipeline-local struct carrying the fields needed by that template.

**LogWriter:**
```go
// Before: Write(entry interface{}) error
// After:
Write(entry *LogEntry) error
```

Where `LogEntry` is a pipeline-local struct (or kept as `any` if log entries are truly polymorphic and only serialized, not inspected).

### Adapter simplification

The adapters in `cmd/gromit/decompose.go` change from constructing `map[string]interface{}` to constructing typed pipeline structs:

```go
func (a *claudeClientAdapter) Run(prompt string, model string) (*pipeline.ClaudeRunResult, error) {
    result, err := a.Client.Run(context.Background(), prompt, model)
    if err != nil { return nil, err }
    return &pipeline.ClaudeRunResult{
        Success:  result.Success,
        ExitCode: result.ExitCode,
        Output:   result.Output,
    }, nil
}

func (a *beadClientAdapter) CreateWithDepsAndDescription(...) (*pipeline.BeadInfo, error) {
    b, err := a.Client.CreateWithDepsAndDescription(...)
    if err != nil { return nil, err }
    return &pipeline.BeadInfo{ID: b.ID, Title: b.Title, Priority: b.Priority, Labels: b.Labels}, nil
}
```

### Decompose workflow simplification

In `internal/pipeline/decompose.go`, all runtime type assertions and the `extractBeadID` function are removed:

```go
// Before:
resultMap, ok := claudeResult.(map[string]interface{})
success, _ := resultMap["Success"].(bool)
output, _ := resultMap["Output"].(string)
beadID, err := extractBeadID(beadResult)

// After:
if !claudeResult.Success { ... }
output := claudeResult.Output
createdIDs = append(createdIDs, beadResult.ID)
```

The `extractBeadID` function (lines 207-239) and its reflection-based logic are deleted entirely.

## Acceptance Criteria

- `ClaudeClient.Run()` returns `(*ClaudeRunResult, error)` instead of `(interface{}, error)`
- `BeadClient.Ready()`, `Show()`, `Create()`, and `CreateWithDepsAndDescription()` return `(*BeadInfo, error)` instead of `(interface{}, error)`
- `PromptRenderer` methods take typed pipeline-local input structs instead of `interface{}`
- `LogWriter.Write()` takes a typed parameter instead of `interface{}`
- `extractBeadID` function is deleted from `decompose.go`
- No `reflect` import in `decompose.go`
- No `.(map[string]interface{})` type assertions in `decompose.go`
- Adapters in `cmd/gromit/decompose.go` construct typed pipeline structs instead of `map[string]interface{}`
- All test mocks in `decompose_test.go` updated to return typed structs
- `go build ./...` compiles without errors
- All existing tests pass (`go test ./...`)

## Decisions

1. **Pipeline-local types rather than importing concrete packages** — The pipeline package is an abstraction layer. Importing `internal/bead` and `internal/claude` would couple it to specific implementations and partially defeat the purpose of having interfaces. The runner package imports concrete types because the runner *is* the implementation; the pipeline defines the contracts. Pipeline-local types (`ClaudeRunResult`, `BeadInfo`) carry only the fields the pipeline workflows actually use, which is a strict subset of the full concrete types. Adapters map between the two at the boundary.

2. **Structs, not generics** — Go generics don't help here. The problem isn't parametric polymorphism; it's that `interface{}` erases structure. Named structs with typed fields solve the problem directly.

3. **Keep `LogWriter.Write()` using `any` for now** — `LogWriter` is used for fire-and-forget serialization. The pipeline never inspects log entry fields — it passes them through to a serializer. If future workflows need to construct log entries, this should be revisited, but currently `any` is appropriate for a write-only sink. This is a deliberate exception to the "no interface{}" goal.

4. **PromptRenderer gets prompt-specific input types** — Rather than passing `interface{}` and hoping the template can handle it, each render method takes a typed struct. This matches how the runner's `PromptRenderer` interface already works (typed `*prompt.Context`, `*prompt.DecomposeContext`, etc.), but using pipeline-local types to avoid importing `internal/prompt`.

## Research & Context

### Current State

- **`internal/pipeline/pipeline.go`** — Defines 8 dependency interfaces. Four interfaces use `interface{}`: `ClaudeClient` (return), `BeadClient` (returns), `PromptRenderer` (params), `LogWriter` (param). The other four (`AgentResolver`, `BacklogClient`, `LearningsManager`, `StateManager`) already use concrete types.

- **`internal/pipeline/decompose.go`** — The only implemented workflow. Lines 59-65 cast `interface{}` to `map[string]interface{}` to extract Success/Output. Lines 139-142 call `extractBeadID()`. Lines 207-239 define `extractBeadID()` which tries three strategies (map, method, reflection) to extract an ID from `interface{}`.

- **`cmd/gromit/decompose.go`** — Contains `claudeClientAdapter` (lines 224-238) that wraps `*claude.Client.Run()` return into `map[string]interface{}`, and `beadClientAdapter` (lines 241-263) that wraps `*bead.Client` methods. Both exist solely to bridge typed concrete methods to `interface{}`-based pipeline interfaces.

- **`internal/runner/interfaces.go`** — The runner's interfaces already use concrete types (`*bead.Bead`, `*claude.Result`, `*prompt.Context`). This is the proven pattern to follow.

- **`internal/pipeline/decompose_test.go`** — Test mocks return `map[string]interface{}` for Claude results and `decomposeAcceptanceBeadDef` structs for bead results. The `decomposeAcceptanceBeadDef` struct (line 977) already has typed fields — it just gets wrapped in `interface{}`.

### Import Graph Safety

`internal/bead` and `internal/claude` do not import `internal/pipeline`. `internal/runner` imports `internal/pipeline` (for types like `Idea`). No cycle risk exists if pipeline imported bead/claude, but the abstraction argument (Decision 1) is the reason to avoid it.

### Scale of Change

- **4 interfaces** gain typed signatures
- **~3 new types** defined in pipeline package (`ClaudeRunResult`, `BeadInfo`, prompt input types)
- **2 adapter files** simplified (cmd/gromit/decompose.go)
- **1 function deleted** (`extractBeadID`)
- **1 import removed** (`reflect` from decompose.go)
- **~10 test mock methods** updated to return typed structs
