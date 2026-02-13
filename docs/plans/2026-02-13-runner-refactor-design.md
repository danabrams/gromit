# Runner Package Refactor Design

**Date:** 2026-02-13
**Goal:** Reduce agent orientation time per bead by splitting `internal/runner/` into focused sub-packages.

## Problem

The runner package contains 60 files and ~27K lines. Two production files dominate: `runner.go` (2,287 lines) and `process.go` (1,328 lines). When an agent works on a bead that touches, say, validation logic, it must read thousands of lines of unrelated code to orient.

## Constraints

- `runner/` remains the public facade. External callers (`cmd/gromit/main.go`) do not change.
- Sub-packages live under `internal/runner/` (e.g., `internal/runner/escalation/`).
- Medium split: 5-6 sub-packages, not a fine-grained one-per-concern split.
- Approach: Extract-and-Delegate. Each sub-package defines its own type with narrow interfaces. The facade constructs handlers and delegates to them.

## Package Layout

```
internal/runner/
├── runtypes/          # Shared types: BeadContext, IterationResult, common interfaces
├── execution/         # Claude invocation, heartbeat, stall detection
├── escalation/        # Retry loops, tier escalation, failure analysis, learning extraction
├── methodology/       # ATDD phases, TDD wiring, refactor phase
├── validation/        # Direct validation commands, recovery, auto-fix
├── review/            # Light review, thorough review, result application
│
├── runner.go          # Runner struct, Run(), Status(), processBead(), constructors
├── format.go          # Display formatting (unchanged)
├── format_bead_breakdown.go
├── status.go          # StatusWriter (unchanged)
├── syncwriter.go      # Thread-safe writer (unchanged)
├── interfaces.go      # BeadClient, PromptRenderer, IterationLogger, FailureAnalyzer
└── test_helpers.go
```

## Package Details

### `runner/runtypes/` — Shared Types

Breaks the circular import between `runner/` and its sub-packages. Contains types that multiple packages need.

**Contains:**
- `BeadContext` — shared state for processing a single bead (promoted from unexported `beadContext`)
- `IterationResult` — outcome of one loop iteration
- Common narrow interfaces: `Logger func(string, ...any)`, `DiffProvider`, `CmdRunner`

**Does not contain:** Package-specific interfaces. Each sub-package defines its own narrow deps.

### `runner/execution/` — Claude Invocation (~350 lines)

Encapsulates how we talk to Claude: one invocation at a time, with heartbeat monitoring and stall detection.

**Moves from:** `process.go` (executeClaudeInvocation), `runner.go` (heartbeat functions)

**Exports:**
```go
type Invoker struct { ... }
type InvocationResult struct {
    ClaudeResult *claude.Result
    Stats        *logger.StreamStats
    StallFired   bool
    ModelName    string
    ProviderName string
}
func (inv *Invoker) Execute(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error)
```

**Narrow deps:**
```go
type Router interface {
    Select(phase, tier string) (Provider, string)
    MarkUnavailable(name string)
}
type Provider interface {
    StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler, toolHandler) (*provider.Result, error)
    IsUsageLimitError(*provider.Result, error) bool
    Name() string
}
```

**Does not decide** whether to retry or escalate. Reports what happened; the caller decides.

### `runner/escalation/` — Retry and Escalation (~400 lines)

Decides what to do when things fail: retry, escalate tier, decompose, or stop. Also extracts learnings from failures and successes.

**Moves from:** `process.go` (handleStallTimeout, handleEscalation, analyzeAndHandleFailure, escalateTier, escalateModel, all extractLearning variants, attemptDecomposition), `runner.go` (selectTier, selectModel)

**Exports:**
```go
type Handler struct { ... }
func (h *Handler) ExecuteWithRetry(ctx context.Context, bc *runtypes.BeadContext, invoke InvokeFn) bool
func (h *Handler) AnalyzeAndHandleFailure(ctx context.Context, bc *runtypes.BeadContext, result *claude.Result) bool
func (h *Handler) HandleStallTimeout(ctx context.Context, bc *runtypes.BeadContext) bool
func (h *Handler) EscalateTier(bc *runtypes.BeadContext, nextTier string)
func (h *Handler) ExtractLearning(bc *runtypes.BeadContext, analysis *analyzer.Analysis)
func (h *Handler) ExtractSuccessLearning(ctx context.Context, bc *runtypes.BeadContext)
func SelectTier(cfg TierConfig, priority int, labels []string) string
func SelectModel(cfg ModelConfig, priority int, labels []string) string
```

`InvokeFn = func(ctx context.Context, bc *runtypes.BeadContext) (*execution.InvocationResult, error)` — the facade passes `execution.Invoker.Execute` wrapped to match.

`ExecuteWithRetry` moves here because it is fundamentally about retry/escalation decisions.

### `runner/methodology/` — ATDD/TDD/Refactor (~500 lines)

Handles methodology-specific workflow phases: writing acceptance tests, verifying they fail before implementation, running the refactor phase.

**Moves from:** `process.go` (runAcceptanceTestsWithRetry, runAcceptanceTests, verifyTestsFailWithRetry, verifyTestsFail, runRefactorPhase, handleRefactorValidationFailure, runRefactorWithRouter, shouldRunRefactor, isTestOnlyDiff, countChangedFiles)

**Exports:**
```go
type Executor struct { ... }
func (e *Executor) RunAcceptanceTestsWithRetry(ctx context.Context, bc *runtypes.BeadContext) error
func (e *Executor) VerifyTestsFailWithRetry(ctx context.Context, bc *runtypes.BeadContext) error
func (e *Executor) RunRefactorPhase(ctx context.Context, bc *runtypes.BeadContext) error
func (e *Executor) ShouldRunRefactor(bc *runtypes.BeadContext, diff string) bool
```

ATDD retry/escalation reuses `escalation.Handler.EscalateTier()` through a callback or by accepting the handler.

### `runner/validation/` — Direct Validation (~300 lines)

Runs test/lint commands directly via `exec.Command`, handles recovery (auto-fix then Claude fix), and extracts validation summaries.

**Moves from:** `process.go` (runValidation, runValidationWithRecovery, runDirectValidationCheck), `validation_summary.go`

**Exports:**
```go
type Runner struct { ... }
func (v *Runner) RunWithRecovery(ctx context.Context, bc *runtypes.BeadContext, executeFn ExecuteFn) error
func (v *Runner) RunDirect(ctx context.Context, commands []string, workDir string) (*claude.Result, error)
func ExtractValidationSummary(output string) string
```

`ExecuteFn` lets validation recovery re-invoke Claude for fixes without importing the escalation package directly.

### `runner/review/` — Code Review (~450 lines)

All review functionality: light reviews after each bead, thorough periodic reviews, creating beads/backlog from findings.

**Moves from:** `runner.go` (runLightReview, runThoroughReview, applyReviewResult, runPostSuccessReview, selectReviewModel, selectReviewTier, buildReviewBeadLabels, buildBacklogLabels, writeReviewLog)

**Exports:**
```go
type Reviewer struct { ... }
func (rv *Reviewer) RunLight(ctx context.Context, b *bead.Bead, parent *bead.Bead, startCommit, buildModel string, ...) (*review.ReviewResult, error)
func (rv *Reviewer) RunThorough(ctx context.Context, sf *state.File, iteration int, deadline time.Time)
func (rv *Reviewer) ApplyResult(result *review.ReviewResult) (beadsCreated, backlogCreated int)
func (rv *Reviewer) RunPostSuccess(ctx context.Context, bc *runtypes.BeadContext) error
func SelectReviewModel(cfg ReviewConfig, buildModel string) string
```

### `runner/` — Facade (~800-900 lines after extraction)

Retains the public API and orchestration logic.

**Keeps:**
- `Runner` struct, `NewRunner()`, `NewRunnerWithDeps()`, `Deps`
- `Run()` — main loop with precheck, scope gate, stuck detection
- `processBead()`, `buildPromptForBead()`, `setupBeadContext()` — orchestrates sub-packages
- `DecomposeTask()`, `CreateSubBeads()` — decomposition (~170 lines, small enough to stay)
- `Status()`, `getNextBead()`, `SetLabelFilters()`
- `format.go`, `format_bead_breakdown.go`, `status.go`, `syncwriter.go` — already separate
- Git utilities, logging helpers

**Runner struct gains handler fields:**
```go
type Runner struct {
    // ...existing fields...
    invoker           *execution.Invoker
    escalationHandler *escalation.Handler
    methodologyExec   *methodology.Executor
    validationRunner  *validation.Runner
    reviewer          *review.Reviewer
}
```

`NewRunner()` and `NewRunnerWithDeps()` construct these handlers during initialization.

## Dependency Flow

```
runner/ (facade)
  ├── imports runtypes/      (shared types)
  ├── imports execution/     (invoke Claude)
  ├── imports escalation/    (retry decisions)
  ├── imports methodology/   (ATDD/TDD/refactor)
  ├── imports validation/    (test/lint)
  └── imports review/        (code review)

Each sub-package:
  ├── imports runtypes/      (BeadContext, IterationResult)
  └── defines own narrow interfaces (no import of runner/ or sibling sub-packages)
```

No circular imports. Sub-packages do not import each other. The facade wires them together through callbacks and function types.

## Migration Strategy

Each sub-package can be extracted one at a time. The order should be:

1. `runtypes/` — extract shared types first (everything else depends on this)
2. `execution/` — self-contained, no deps on other sub-packages
3. `escalation/` — depends on execution via InvokeFn callback
4. `validation/` — depends on escalation via ExecuteFn callback
5. `methodology/` — depends on validation.RunDirect and escalation.EscalateTier via callbacks
6. `review/` — self-contained, only needs router and renderer

Each step: extract, update facade to delegate, run tests, commit. The public API never changes.

## Test Strategy

- Each sub-package gets its own `_test.go` files testing the exported API with mock deps
- Existing integration tests in `runner/` continue to test the full pipeline through the facade
- Existing acceptance tests remain unchanged (they test through `cmd/gromit/`)
- Move test helpers into sub-packages where they test sub-package logic; keep shared test helpers in `runner/test_helpers.go`

## Size Estimates (Post-Refactor)

| Package | Production Lines | Test Lines (est.) |
|---|---|---|
| `runtypes/` | ~80 | ~20 |
| `execution/` | ~350 | ~300 |
| `escalation/` | ~400 | ~500 |
| `methodology/` | ~500 | ~600 |
| `validation/` | ~300 | ~400 |
| `review/` | ~450 | ~500 |
| `runner/` (facade) | ~800 | ~700 |
| **Total** | ~2,880 | ~3,020 |

Current production total: ~3,600 lines. The difference (~720 lines) accounts for narrow interface definitions replacing direct field access, plus some delegation boilerplate. The key win is that no single file exceeds ~400 lines of production code.
