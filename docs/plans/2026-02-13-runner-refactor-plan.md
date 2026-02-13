# Runner Package Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Split `internal/runner/` into 5 focused sub-packages so agents read only the code relevant to each bead.

**Architecture:** Extract-and-Delegate. Each sub-package owns a struct with narrow interfaces. The `runner/` facade constructs these structs and delegates to them. A shared `runtypes/` package breaks circular imports.

**Tech Stack:** Go 1.23+, standard testing, existing mock patterns with callback functions.

**Design doc:** `docs/plans/2026-02-13-runner-refactor-design.md`

---

## Prerequisite: Verify green baseline

Before any changes, confirm existing tests pass.

### Task 0: Verify baseline

**Step 1:** Run all tests
```bash
go test ./internal/runner/... -count=1
```
Expected: All tests pass.

**Step 2:** Run linter
```bash
golangci-lint run ./internal/runner/...
```
Expected: No errors.

---

## Phase 1: Extract `runtypes/`

Move shared types that sub-packages need into `internal/runner/runtypes/` to prevent circular imports.

### Task 1: Create `runtypes/` with BeadContext

**Files:**
- Create: `internal/runner/runtypes/bead_context.go`
- Modify: `internal/runner/process.go` — remove `beadContext`, alias to `runtypes.BeadContext`

**Step 1: Create the runtypes package with BeadContext**

Create `internal/runner/runtypes/bead_context.go`:

```go
package runtypes

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
)

// BeadContext holds the shared state for processing a single bead.
// Passed between the methods that compose processBead.
type BeadContext struct {
	Bead          *bead.Bead
	Parent        *bead.Bead
	Result        *IterationResult
	Model         string // concrete model name for display/logging
	Tier          string // abstract tier (high/medium/low) for router selection
	BuildProvider string // name of provider that performed the build
	PromptCtx     *prompt.Context
	BuildPrompt   string
	StartCommit   string
	Iteration     int

	// Retry tracking
	RetriesThisModel     int
	TotalRetriesThisBead int
	MaxRetries           int
	MaxRetriesPerBead    int

	// Context management
	ParentCtx   context.Context
	BeadTimeout time.Duration
	RunDeadline time.Time

	// Scope estimate (cached from scope gate)
	ScopeEstimate *prompt.ScopeEstimate
}
```

All fields promoted from unexported to exported (capitalized). The struct was `beadContext` with lowercase fields; now `BeadContext` with uppercase fields.

**Step 2: Create IterationResult in runtypes**

Create `internal/runner/runtypes/iteration_result.go`:

```go
package runtypes

import "time"

// IterationResult captures the outcome of one loop iteration.
type IterationResult struct {
	BeadID                string
	BeadTitle             string
	Model                 string
	Success               bool
	Validated             bool
	Duration              time.Duration
	Error                 error
	Escalated             bool
	EscalatedTo           string
	Decomposed            bool
	Output                string
	CostUSD               float64
	InputTokens           int
	OutputTokens          int
	ReviewBrokeValidation bool
	AlreadyDone           bool
	ValidationRetried     bool
	TrivialAutoFixed      bool
	UsageLimited          bool
	ValidationMode        string

	// Diagnostic fields
	TimeoutType         string
	TimeToFirstEventMs  int64
	ToolCallCount       int
	StallCount          int
	StallTier           string
	RateLimitHits       int
	RateLimitRecoveryMs int64
}
```

**Step 3: Create common function types in runtypes**

Create `internal/runner/runtypes/types.go`:

```go
package runtypes

import "context"

// LogFn is the standard logging function signature used across sub-packages.
type LogFn func(format string, args ...any)

// CmdRunner executes a shell command and returns stdout, stderr, exit code.
type CmdRunner func(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error)

// DiffProvider returns a git diff from the given commit.
type DiffProvider func(fromCommit string) (string, error)
```

**Step 4: Run tests**
```bash
go test ./internal/runner/runtypes/... -count=1
```
Expected: Package compiles (no tests yet, but compilation verifies types).

**Step 5: Commit**
```bash
git add internal/runner/runtypes/
git commit -m "feat: add runner/runtypes package with shared types"
```

### Task 2: Migrate runner/ to use runtypes.BeadContext and runtypes.IterationResult

**Files:**
- Modify: `internal/runner/process.go` — replace `beadContext` with `runtypes.BeadContext`
- Modify: `internal/runner/runner.go` — replace `IterationResult` with `runtypes.IterationResult`, add type alias
- Modify: All `internal/runner/*_test.go` files referencing these types

This is the most invasive single task. Every reference to `beadContext` fields must change from `bc.model` to `bc.Model`, `bc.bead` to `bc.Bead`, etc.

**Step 1: Add import and type alias in runner.go**

In `runner.go`, add import for runtypes, then add type aliases:

```go
import "github.com/danabrams/gromit/internal/runner/runtypes"

// IterationResult is an alias to maintain backward compatibility.
type IterationResult = runtypes.IterationResult
```

Remove the `IterationResult` struct definition from `runner.go` (lines 268-298).

**Step 2: Replace beadContext with runtypes.BeadContext in process.go**

Remove the `beadContext` struct definition (lines 27-52). Replace all occurrences of `*beadContext` with `*runtypes.BeadContext`. Update all field accesses:

| Old | New |
|-----|-----|
| `bc.bead` | `bc.Bead` |
| `bc.parent` | `bc.Parent` |
| `bc.result` | `bc.Result` |
| `bc.model` | `bc.Model` |
| `bc.tier` | `bc.Tier` |
| `bc.buildProvider` | `bc.BuildProvider` |
| `bc.promptCtx` | `bc.PromptCtx` |
| `bc.buildPrompt` | `bc.BuildPrompt` |
| `bc.startCommit` | `bc.StartCommit` |
| `bc.iteration` | `bc.Iteration` |
| `bc.retriesThisModel` | `bc.RetriesThisModel` |
| `bc.totalRetriesThisBead` | `bc.TotalRetriesThisBead` |
| `bc.maxRetries` | `bc.MaxRetries` |
| `bc.maxRetriesPerBead` | `bc.MaxRetriesPerBead` |
| `bc.parentCtx` | `bc.ParentCtx` |
| `bc.beadTimeout` | `bc.BeadTimeout` |
| `bc.runDeadline` | `bc.RunDeadline` |
| `bc.scopeEstimate` | `bc.ScopeEstimate` |

**Step 3: Update runner.go references**

Update all methods in `runner.go` that reference `*beadContext` or `beadContext` fields. Same field renaming as above.

**Step 4: Update test files**

Update all test files that reference `beadContext`, `IterationResult`, or their fields. Use the same field renaming.

**Step 5: Run tests**
```bash
go test ./internal/runner/... -count=1
```
Expected: All tests pass.

**Step 6: Run linter**
```bash
golangci-lint run ./internal/runner/...
```
Expected: No errors.

**Step 7: Commit**
```bash
git add internal/runner/
git commit -m "refactor: migrate BeadContext and IterationResult to runtypes package"
```

---

## Phase 2: Extract `execution/`

Move Claude invocation and heartbeat logic into `internal/runner/execution/`.

### Task 3: Create execution package with Invoker

**Files:**
- Create: `internal/runner/execution/invoker.go`
- Create: `internal/runner/execution/heartbeat.go`
- Create: `internal/runner/execution/invoker_test.go`

**Step 1: Define narrow interfaces and Invoker struct**

Create `internal/runner/execution/invoker.go` with:
- `Router` interface (Select, MarkUnavailable)
- `Provider` interface (StreamRun, IsUsageLimitError, Name)
- `StreamLogger` interface (for event parsing)
- `Invoker` struct holding these deps + config
- `InvocationResult` struct
- `InvocationConfig` struct (stall/timeout settings extracted from config.Config)
- `Execute()` method — adapted from `executeClaudeInvocation()` in process.go

The `Execute` method takes `*runtypes.BeadContext` and a prompt string, returns `*InvocationResult, error`.

Key adaptation: instead of accessing `r.cfg.Claude.TimeoutsForModel(bc.Model)`, the Invoker receives an `InvocationConfig` or the facade passes timeout values through the BeadContext.

**Step 2: Move heartbeat logic**

Create `internal/runner/execution/heartbeat.go` with:
- `heartbeatConfig` struct
- `startHeartbeat()` function (adapted from Runner method to standalone function)
- `printHeartbeat()`, `overwriteHeartbeat()` functions
- `OverwriteWriter` interface (for syncWriter.WriteOverwrite)

These become standalone functions rather than methods on Invoker, since they need the sync writer.

**Step 3: Write test for Execute**

Create `internal/runner/execution/invoker_test.go`:
- Mock the narrow Router and Provider interfaces
- Test successful invocation
- Test stall detection fires
- Test usage limit fallback to second provider

Use the same mock callback pattern as existing runner tests:
```go
type mockRouter struct {
    selectFn func(phase, tier string) (Provider, string)
    // ...
}
```

**Step 4: Run tests**
```bash
go test ./internal/runner/execution/... -count=1
```
Expected: New tests pass.

**Step 5: Commit**
```bash
git add internal/runner/execution/
git commit -m "feat: add runner/execution package with Invoker"
```

### Task 4: Wire execution.Invoker into Runner facade

**Files:**
- Modify: `internal/runner/runner.go` — add `invoker` field, construct in NewRunner/NewRunnerWithDeps
- Modify: `internal/runner/process.go` — delegate `executeClaudeInvocation` to `r.invoker.Execute()`
- Delete: heartbeat methods from `runner.go` (moved to execution/)

**Step 1: Add invoker field and construct it**

In `runner.go`, add `invoker *execution.Invoker` to the Runner struct. In `NewRunner()` and `NewRunnerWithDeps()`, construct the Invoker with the appropriate deps (router, stream logger, sync writer, timeouts config).

**Step 2: Delegate executeClaudeInvocation**

Replace the body of `executeClaudeInvocation` with a call to `r.invoker.Execute()`. Map the return type back to what callers expect.

**Step 3: Remove heartbeat methods from runner.go**

Delete `startHeartbeat`, `startHeartbeatWithConfig`, `printHeartbeat`, `overwriteHeartbeat`, `heartbeatConfig`, `defaultHeartbeatConfig`, `heartbeatInterval` from runner.go. These now live in execution/.

**Step 4: Run all runner tests**
```bash
go test ./internal/runner/... -count=1
```
Expected: All tests pass (existing integration tests exercise the full path through the facade).

**Step 5: Run linter**
```bash
golangci-lint run ./internal/runner/...
```

**Step 6: Commit**
```bash
git add internal/runner/
git commit -m "refactor: wire execution.Invoker into runner facade"
```

---

## Phase 3: Extract `escalation/`

Move retry, escalation, failure analysis, and learning extraction into `internal/runner/escalation/`.

### Task 5: Create escalation package with Handler

**Files:**
- Create: `internal/runner/escalation/handler.go`
- Create: `internal/runner/escalation/learning.go`
- Create: `internal/runner/escalation/tier_selection.go`
- Create: `internal/runner/escalation/handler_test.go`

**Step 1: Define Handler struct and narrow interfaces**

Create `internal/runner/escalation/handler.go` with:
- `EscalationConfig` struct (MaxRetriesPerModel, MaxRetriesPerBead, NextEscalationTier func, AnalysisTimeout)
- `FailureAnalyzer` interface (Analyze)
- `LearningsWriter` interface (Add)
- `PromptRenderer` interface (RenderBuild, GetLearningsFile — narrow subset)
- `InvokeFn` type — `func(ctx, bc) (*execution.InvocationResult, error)`
- `DecomposeFn` type — `func(ctx, bead) ([]SubTask, error)` for decomposition callback
- `Handler` struct
- `ExecuteWithRetry()` — adapted from runner.executeWithRetry
- `AnalyzeAndHandleFailure()` — adapted from runner.analyzeAndHandleFailure
- `HandleStallTimeout()` — adapted from runner.handleStallTimeout
- `HandleEscalation()` — adapted from runner.handleEscalation
- `EscalateTier()` — adapted from runner.escalateTier
- `EscalateModel()` — adapted from runner.escalateModel

**Step 2: Move learning extraction**

Create `internal/runner/escalation/learning.go` with:
- `ExtractLearning()` — adapted from runner.extractLearning
- `ExtractSyntheticLearning()` — adapted from runner.extractSyntheticLearning
- `ExtractScopeTooLargeLearning()` — adapted from runner.extractScopeTooLargeLearning
- `ExtractTimeoutLearning()` — adapted from runner.extractTimeoutLearning
- `ExtractSuccessLearning()` — adapted from runner.extractSuccessLearning

**Step 3: Move tier/model selection**

Create `internal/runner/escalation/tier_selection.go` with:
- `SelectTier()` — adapted from runner.selectTier
- `SelectModel()` — adapted from runner.selectModel

**Step 4: Write tests for Handler**

Test `ExecuteWithRetry` with:
- Success on first try
- Retry after recoverable failure
- Escalation after exhausting retries
- Max retries per bead limit
- Stall timeout handling

Use mock InvokeFn that returns configurable results.

**Step 5: Run tests**
```bash
go test ./internal/runner/escalation/... -count=1
```

**Step 6: Commit**
```bash
git add internal/runner/escalation/
git commit -m "feat: add runner/escalation package with Handler"
```

### Task 6: Wire escalation.Handler into Runner facade

**Files:**
- Modify: `internal/runner/runner.go` — add `escalationHandler` field
- Modify: `internal/runner/process.go` — delegate escalation methods

**Step 1: Add handler field and construct it**

Add `escalationHandler *escalation.Handler` to Runner. Construct in NewRunner/NewRunnerWithDeps.

**Step 2: Delegate methods**

Replace bodies of: `executeWithRetry`, `analyzeAndHandleFailure`, `handleStallTimeout`, `handleEscalation`, `escalateTier`, `escalateModel`, `extractLearning`, `extractSyntheticLearning`, `extractScopeTooLargeLearning`, `extractTimeoutLearning`, `extractSuccessLearning`, `selectTier`, `selectModel`.

Each becomes a one-line delegation to the escalation handler.

**Step 3: Run all runner tests**
```bash
go test ./internal/runner/... -count=1
```

**Step 4: Commit**
```bash
git add internal/runner/
git commit -m "refactor: wire escalation.Handler into runner facade"
```

---

## Phase 4: Extract `validation/`

Move direct validation, recovery, and summary extraction into `internal/runner/validation/`.

### Task 7: Create validation package

**Files:**
- Create: `internal/runner/validation/runner.go`
- Create: `internal/runner/validation/summary.go`
- Create: `internal/runner/validation/runner_test.go`

**Step 1: Define Runner struct**

Create `internal/runner/validation/runner.go` with:
- `ValidationConfig` struct (Enabled, Commands, MaxValidationRetries)
- `PreflightChecker` interface
- `ExecuteFn` type — `func(ctx, bc) bool` for Claude-based fix attempts
- `Runner` struct
- `RunValidation()` — adapted from runner.runValidation
- `RunWithRecovery()` — adapted from runner.runValidationWithRecovery
- `RunDirect()` — adapted from runner.runDirectValidationCheck

**Step 2: Move summary extraction**

Create `internal/runner/validation/summary.go` with `ExtractValidationSummary()` — moved from validation_summary.go.

**Step 3: Write tests**

Test RunDirect with:
- All commands pass → VALIDATION_PASSED
- Command fails → failure output captured
- Context cancellation

Test RunWithRecovery with:
- Pass on first try
- Auto-fix resolves
- Claude fix resolves after auto-fix fails
- Max retries exhausted

**Step 4: Run tests**
```bash
go test ./internal/runner/validation/... -count=1
```

**Step 5: Commit**
```bash
git add internal/runner/validation/
git commit -m "feat: add runner/validation package"
```

### Task 8: Wire validation.Runner into facade

**Files:**
- Modify: `internal/runner/runner.go` — add `validationRunner` field
- Modify: `internal/runner/process.go` — delegate validation methods
- Delete: `internal/runner/validation_summary.go` (moved)

**Step 1: Wire and delegate**

Add `validationRunner *validation.Runner` to Runner. Delegate `runValidation`, `runValidationWithRecovery`, `runDirectValidationCheck`. Delete `validation_summary.go`.

**Step 2: Run all runner tests**
```bash
go test ./internal/runner/... -count=1
```

**Step 3: Commit**
```bash
git add internal/runner/
git commit -m "refactor: wire validation.Runner into runner facade"
```

---

## Phase 5: Extract `methodology/`

Move ATDD, TDD, and refactor phases into `internal/runner/methodology/`.

### Task 9: Create methodology package

**Files:**
- Create: `internal/runner/methodology/executor.go`
- Create: `internal/runner/methodology/refactor.go`
- Create: `internal/runner/methodology/executor_test.go`

**Step 1: Define Executor struct**

Create `internal/runner/methodology/executor.go` with:
- `MethodologyConfig` struct (ATDD, TDD settings, escalation config)
- `EscalateFn` type — callback to escalation.EscalateTier
- `DirectValidationFn` type — callback to validation.RunDirect
- `Executor` struct
- `RunAcceptanceTestsWithRetry()` — adapted from runner method
- `RunAcceptanceTests()` — adapted from runner method
- `VerifyTestsFailWithRetry()` — adapted from runner method
- `VerifyTestsFail()` — adapted from runner method

**Step 2: Move refactor logic**

Create `internal/runner/methodology/refactor.go` with:
- `RunRefactorPhase()` — adapted from runner method
- `HandleRefactorValidationFailure()` — adapted from runner method
- `RunRefactorWithRouter()` — adapted from runner method
- `ShouldRunRefactor()` — adapted from runner method
- `IsTestOnlyDiff()`, `CountChangedFiles()` — standalone utility functions

**Step 3: Write tests**

Test ATDD workflow:
- Acceptance tests succeed → verify tests fail → success
- Tests pass before implementation → errATDDAlreadyDone

Test refactor:
- Skipped for low tier
- Skipped when fewer files than threshold
- Revert on validation failure

**Step 4: Run tests**
```bash
go test ./internal/runner/methodology/... -count=1
```

**Step 5: Commit**
```bash
git add internal/runner/methodology/
git commit -m "feat: add runner/methodology package with ATDD/TDD/refactor"
```

### Task 10: Wire methodology.Executor into facade

**Files:**
- Modify: `internal/runner/runner.go` — add `methodologyExec` field
- Modify: `internal/runner/process.go` — delegate methodology methods

**Step 1: Wire and delegate**

Add `methodologyExec *methodology.Executor` to Runner. Delegate all ATDD, TDD, and refactor methods.

**Step 2: Run all runner tests**
```bash
go test ./internal/runner/... -count=1
```

**Step 3: Commit**
```bash
git add internal/runner/
git commit -m "refactor: wire methodology.Executor into runner facade"
```

---

## Phase 6: Extract `review/`

Move light review, thorough review, and result application into `internal/runner/review_pkg/` (using `review_pkg` to avoid conflict with the existing `internal/review/` package; the Go package name will be `reviewpkg`).

**Alternative:** If `internal/runner/review/` doesn't conflict (since it's a different import path), use that name. Test compilation first.

### Task 11: Create review sub-package

**Files:**
- Create: `internal/runner/reviewpkg/reviewer.go`
- Create: `internal/runner/reviewpkg/reviewer_test.go`

**Step 1: Define Reviewer struct**

Create `internal/runner/reviewpkg/reviewer.go` with:
- `ReviewConfig` struct (Enabled, Model, Timeout, MatchBuildModel, Thorough settings)
- `Router` interface (Select, SelectCross)
- `PromptRenderer` interface (RenderReview, RenderThoroughReview, LoadClaudeMD, LoadRulesForPhase, LoadSpec)
- `BeadCreator` interface (CreateWithParentAndDescription)
- `ReviewLogger` interface (LogReview)
- `Reviewer` struct
- `RunLight()` — adapted from runner.runLightReview
- `RunThorough()` — adapted from runner.runThoroughReview
- `ApplyResult()` — adapted from runner.applyReviewResult
- `RunPostSuccess()` — adapted from runner.runPostSuccessReview
- `SelectReviewModel()` — adapted from runner.selectReviewModel
- `SelectReviewTier()` — adapted from runner.selectReviewTier
- Helper functions: `BuildReviewBeadLabels()`, `BuildBacklogLabels()`
- `WriteReviewLog()` — adapted from runner.writeReviewLog

**Step 2: Write tests**

Test RunLight:
- Skips when no time remaining
- Skips when no diff
- Returns parsed ReviewResult

Test ApplyResult:
- Creates beads from proposals
- Creates backlog items as P2
- Handles nil result gracefully

**Step 3: Run tests**
```bash
go test ./internal/runner/reviewpkg/... -count=1
```

**Step 4: Commit**
```bash
git add internal/runner/reviewpkg/
git commit -m "feat: add runner/reviewpkg package with Reviewer"
```

### Task 12: Wire reviewpkg.Reviewer into facade

**Files:**
- Modify: `internal/runner/runner.go` — add `reviewer` field
- Modify: `internal/runner/runner.go` — delegate review methods

**Step 1: Wire and delegate**

Add `reviewer *reviewpkg.Reviewer` to Runner. Delegate `runLightReview`, `runThoroughReview`, `applyReviewResult`, `runPostSuccessReview`, `selectReviewModel`, `selectReviewTier`, `buildReviewBeadLabels`, `buildBacklogLabels`, `writeReviewLog`.

**Step 2: Run all runner tests**
```bash
go test ./internal/runner/... -count=1
```

**Step 3: Commit**
```bash
git add internal/runner/
git commit -m "refactor: wire reviewpkg.Reviewer into runner facade"
```

---

## Phase 7: Clean up

### Task 13: Remove dead code and delegation stubs

**Files:**
- Modify: `internal/runner/process.go` — should now be much smaller (setupBeadContext, buildPromptForBead, processBead orchestration only)
- Modify: `internal/runner/runner.go` — remove one-line delegation methods that are only called once; call sub-package directly

**Step 1: Inline single-use delegations**

Where a facade method just calls `r.escalationHandler.Foo()` and is only called from one place in processBead, replace the call site to call the handler directly. Remove the delegation method.

**Step 2: Verify process.go and runner.go sizes**

Target: runner.go ~800 lines, process.go ~200 lines (just orchestration in processBead, setupBeadContext, buildPromptForBead).

**Step 3: Run all tests**
```bash
go test ./internal/runner/... -count=1
go test ./... -count=1
```

**Step 4: Run linter**
```bash
golangci-lint run ./...
```

**Step 5: Commit**
```bash
git add internal/runner/
git commit -m "refactor: clean up runner facade after sub-package extraction"
```

### Task 14: Move test files to sub-packages where appropriate

**Files:**
- Move tests that only exercise sub-package logic into the sub-package
- Keep integration tests that exercise the full pipeline in `runner/`

**Step 1: Identify moveable tests**

Tests like `select_tier_test.go`, `select_tier_acceptance_test.go` → `escalation/`
Tests like `validation_sentinel_error_test.go`, `extract_validation_summary_test.go` → `validation/`
Tests like `refactor_conditional_test.go` → `methodology/`
Tests like `select_review_tier_test.go`, `cross_review_routing_test.go` → `reviewpkg/`

**Step 2: Move tests one file at a time**

For each test file:
1. Change `package runner` to `package escalation` (or whichever sub-package)
2. Update imports
3. Replace `NewRunnerWithDeps` setup with direct sub-package handler construction
4. Run tests to verify

**Step 3: Run all tests**
```bash
go test ./internal/runner/... -count=1
```

**Step 4: Commit**
```bash
git add internal/runner/
git commit -m "refactor: move sub-package-specific tests to their packages"
```

### Task 15: Final verification

**Step 1: Run full test suite**
```bash
go test ./... -count=1
```

**Step 2: Run linter**
```bash
golangci-lint run ./...
```

**Step 3: Build**
```bash
go build ./cmd/gromit
```

**Step 4: Verify package sizes**

```bash
wc -l internal/runner/*.go | sort -n
wc -l internal/runner/runtypes/*.go | sort -n
wc -l internal/runner/execution/*.go | sort -n
wc -l internal/runner/escalation/*.go | sort -n
wc -l internal/runner/methodology/*.go | sort -n
wc -l internal/runner/validation/*.go | sort -n
wc -l internal/runner/reviewpkg/*.go | sort -n
```

Verify no production file exceeds ~400 lines.

**Step 5: Commit**
```bash
git commit -m "chore: verify runner refactor complete — all tests pass"
```
