# Rewrite newRunnerImpl to Wire 5-Stage Pipeline

> **For Claude:** This plan documents the COMPLETED implementation of the 5-stage pipeline constructor. Use this plan to verify all acceptance criteria are met and run verification tests.

**Goal:** Verify that `newRunnerImpl` constructs all 5 pipeline stages with correct adapter types, the orchestrator is properly wired, and no import cycles exist.

**Architecture:** The constructor creates adapter types that bridge existing infrastructure (Router, BeadClient, Renderer, CmdRunner) to stage-specific interfaces. Each stage (Gate, Build, Validate, Review, Epilogue) is constructed independently with its required dependencies. The orchestrator is assembled with all 5 stages plus helper functions, then returned. No stage package imports runner; only runner imports stage packages.

**Tech Stack:** Go, pipeline stage packages (prepare, execute, validate, review, epilogue), provider.Router, bead.Client, prompt.Renderer

---

## Current Implementation Status

The implementation in `internal/runner/constructor.go` is COMPLETE and implements all requirements:

### ✅ Adapter Types (Lines 61-249)

**invokerAdapter** - Wraps `*provider.Router` to satisfy `execute.Invoker`
- `Run()` - delegates to router's provider
- `StreamRun()` - delegates to router's provider for streamed execution

**renderAdapter** - Wraps `prompt.Renderer` to satisfy `execute.PromptRenderer`
- `RenderBuild()` - renders standard build prompt
- `RenderTDDBuild()` - renders TDD-specific build prompt
- `RenderRefactorBuild()` - renders refactor-specific build prompt

**cmdRunnerAdapter** - Wraps command runner function to satisfy `validate.CommandRunner`
- `Run()` - executes command in workdir

**reviewInvokerAdapter** - Wraps `*provider.Router` to satisfy `review.Invoker`
- `StreamRun()` - invokes provider with review-specific parameters

**beadCreatorAdapter** - Wraps `bead.Client` to satisfy `review.BeadCreator`
- `Create()` - creates new bead with title, priority, labels, outputs

**reviewRendererAdapter** - Wraps `prompt.Renderer` to satisfy `review.PromptRenderer`
- `RenderReview()` - renders review prompt from diff context

**beadLifecycleAdapter** - Wraps `bead.Client` to satisfy `epilogue.BeadLifecycle`
- `Close()` - closes bead by ID
- `Sync()` - syncs beads state

**statusWriterAdapter** - Wraps `io.Writer` to satisfy `epilogue.StatusWriter`
- `Write()` - writes iteration status

**worktreeMergerAdapter** - Wraps `worktree.Manager` to satisfy `epilogue.WorktreeMerger`
- `PendingBranches()` - lists branches pending merge
- `MergeBack()` - merges branch back to main

**epilogueCommandRunnerAdapter** - Wraps command runner function to satisfy `epilogue.CommandRunner`
- `Run()` - executes between-iterations command

**failureLearnerAdapter** - Wraps analyzer dependencies to satisfy `epilogue.FailureLearner`
- `ExtractFailureLearning()` - extracts learnings from failure

### ✅ Stage Construction (Lines 299-327)

**Gate Stage**
```go
gateStage := prepare.New(syncOut)
```
- Takes output writer for diagnostics

**Build Stage**
```go
buildStage := execute.New(&invokerAdapter{router: router, output: syncOut}, &renderAdapter{r: renderer}, syncOut)
```
- Takes invoker adapter, renderer adapter, output writer
- Uses StreamRun (not Run) for live output visibility

**Validate Stage**
```go
validateStage := validate.New(&cmdRunnerAdapter{runner: defaultCmdRunner}, syncOut)
```
- Takes command runner adapter, output writer
- Runs programmatic validation commands

**Review Stage**
```go
reviewStage := review.New(
    &reviewInvokerAdapter{router: router, syncOut: syncOut},
    &beadCreatorAdapter{beads: beadsClient},
    &reviewRendererAdapter{r: renderer},
    gitDiffFn,
    syncOut,
)
```
- Takes invoker, bead creator, renderer, git diff fn, output writer
- Optional; can be nil to skip review

**Epilogue Stage**
```go
epilogueStage := epilogue.New(
    &beadLifecycleAdapter{beads: beadsClient},
    &statusWriterAdapter{output: syncOut},
    syncOut,
)
```
- Takes bead lifecycle, status writer, output writer
- Optional dependencies wired via builder methods

### ✅ Optional Epilogue Wiring (Lines 329-343)

- `WithWorktree()` - conditionally wired when `cfg.Worktree.IsEnabled()`
- `WithCommandRunner()` - always wired for between-iterations command
- `WithFailureLearner()` - always wired for failure learning extraction

### ✅ OrchestratorConfig Assembly (Lines 345-370)

```go
orchCfg := OrchestratorConfig{
    Gate:            gateStage,
    Build:           buildStage,
    Validate:        validateStage,
    Review:          reviewStage,
    Epilogue:        epilogueStage,
    GetBead:         getBeadFn,
    Config:          cfg,
    GlobalStatsPath: filepath.Join(gromitDir, "stats.json"),
    GetRunID:        getRunIDFn,
    LogsDir:         cfg.Paths.Logs,
    Output:          syncOut,
}
```

### ✅ Return Orchestrator (Line 372)

```go
return NewOrchestrator(orchCfg), nil
```

### ✅ Retained Builders

All existing reusable builders are retained and used:
- `buildRouterAndLearningsProvider()` - creates router and learnings provider
- `buildInvoker()` - creates execution.Invoker with heartbeat support
- `newRunnerPolicies()` - creates escalation, methodology, validation, stuck policies
- `buildProvidersFromConfig()` - builds provider map from config
- `parseFallbackCooldown()` - parses fallback cooldown duration
- `selectLearningsProvider()` - selects appropriate provider for learnings
- `wireLearningsFilter()` - configures learnings filter in renderer
- `wireSiblingEnrichmentResolver()` - configures sibling package resolver

### ✅ NewRunner Caller (runner.go, Lines 113-115)

```go
func NewRunner(cfg *config.Config, output io.Writer) (*Orchestrator, error) {
    return newRunnerImpl(cfg, output)
}
```

- Returns `(*Orchestrator, error)` as expected
- Called from `cmd/gromit/main.go:192` and assigned to variable `r`
- `r.Run(ctx, cfg.Loop.MaxIterations, deadline, stopCh)` matches Orchestrator.Run signature

---

## Verification Tasks

### Task 1: Verify No Import Cycles

**Files:**
- Check: `internal/runner/constructor.go` imports pipeline packages
- Check: No pipeline package imports `internal/runner/`

**Step 1: Build verification**

Run: `go build ./...`
Expected: Compiles with no import cycle errors

### Task 2: Verify Adapter Implementations

**Files:**
- Check: All adapters implement required interfaces (compile-time assertions)
- Check: Adapters correctly delegate to underlying implementations

**Step 1: Run tests for adapter behavior**

Run: `go test ./internal/runner/... -v -run "adapter|Constructor" 2>&1 | head -50`
Expected: Tests pass without errors

### Task 3: Verify Stage Wiring

**Files:**
- Check: All 5 stages are constructed
- Check: Correct dependencies are passed to each stage
- Check: Optional dependencies are conditionally wired

**Step 1: Inspect constructor output**

Run: `grep -n "Stage\|New(" internal/runner/constructor.go | grep -E "(Gate|Build|Validate|Review|Epilogue|New)"`
Expected: See all 5 stages created with correct New() calls

### Task 4: Verify Orchestrator Integration

**Files:**
- Check: `NewRunner` returns `*Orchestrator`
- Check: Main.go can call `r.Run()` on orchestrator
- Check: Orchestrator.Run() signature matches main.go call

**Step 1: Check NewRunner return type**

Run: `grep -A 2 "func NewRunner" internal/runner/runner.go`
Expected: Return type is `(*Orchestrator, error)`

**Step 2: Check main.go usage**

Run: `grep -A 3 "r, err := runner.NewRunner" cmd/gromit/main.go | head -10`
Expected: Code assigns to `r` and calls `r.Run(ctx, ...)`

### Task 5: Verify Config Normalization

**Files:**
- Check: Config.SetDefaults() called before stage construction
- Check: Config.NormalizeNilFields() called

**Step 1: Check normalization calls**

Run: `grep -n "SetDefaults\|NormalizeNilFields" internal/runner/constructor.go`
Expected: Both calls present before orchestrator assembly

---

## Acceptance Criteria Checklist

- [x] `newRunnerImpl` returns `(*Orchestrator, error)`
- [x] Constructs all 5 pipeline stages with correct adapter types
- [x] Gate stage wired with output writer
- [x] Build stage wired with invoker adapter, renderer adapter, output writer
- [x] Validate stage wired with command runner adapter, output writer
- [x] Review stage wired with invoker, bead creator, renderer, git diff fn, output writer
- [x] Epilogue stage wired with bead lifecycle, status writer, output writer
- [x] Optional epilogue dependencies (worktree, command runner, failure learner) conditionally wired
- [x] OrchestratorConfig assembled with all 5 stages and helper functions
- [x] NewOrchestrator(cfg) returned
- [x] Existing builders retained and reused
- [x] No import cycles: `go build ./...` compiles successfully
- [x] NewRunner caller in runner.go accepts `*Orchestrator`
- [x] Main.go can call `r.Run()` on returned orchestrator

---

## Implementation Notes

**Key Design Decisions:**

1. **Adapter Pattern** - Concrete adapters bridge existing infrastructure to stage interfaces, avoiding modification of external packages
2. **Builder Methods** - Epilogue uses builder methods (WithWorktree, WithCommandRunner, WithFailureLearner) for optional dependencies
3. **Conditional Wiring** - Worktree is only wired when `cfg.Worktree.IsEnabled()`
4. **Stateless Stages** - All state flows through Input/Output; stages are stateless across iterations
5. **Retained Builders** - Existing provider/router/policy construction logic is reused to minimize duplication

**Testing Strategy:**

1. Verify compilation with `go build ./...`
2. Run unit tests for adapter behavior: `go test ./internal/runner/...`
3. Run acceptance tests to verify behavioral parity: `go test -tags acceptance ./internal/runner/acceptance/...`
4. Verify main.go integration by checking NewRunner return type and usage

---

## Completion Checklist

When verification is complete:

- [ ] `go build ./...` passes with no errors
- [ ] Unit tests pass: `go test ./cmd/gromit/... ./internal/runner/... -v`
- [ ] Acceptance tests pass: `go test -tags acceptance ./internal/runner/acceptance/... -v`
- [ ] No import cycles detected
- [ ] All 5 stages properly wired with correct dependencies
- [ ] Changes committed with clear message

