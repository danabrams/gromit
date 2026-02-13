---
id: runner-subpackage-split
source_spec: runner-subpackage-split
created: 2026-02-13
decomposed: true
decomposed_at: 2026-02-13
---

# Runner Sub-Package Split Implementation Plan

**Goal:** Split `internal/runner/` into focused sub-packages so an agent working on one concern reads ~400 lines instead of 3,600.

**Architecture:** Extract-and-delegate with 5 sub-packages (`execution/`, `escalation/`, `validation/`, `methodology/`, `reviewpkg/`) plus a shared `runtypes/` types package. The `runner/` facade constructs sub-package handlers and delegates. Callbacks wire cross-cutting concerns — no sibling imports.

**Tech Stack:** Go, existing test infrastructure (callback-based mocks via `NewRunnerWithDeps`)

**Spec:** `.gromit/specs/runner-subpackage-split.md`

---

## Architecture

### Package Layout

```
internal/runner/
├── runtypes/          # BeadContext, IterationResult, SubTask, function types
├── execution/         # Claude invocation, heartbeat, stall detection
├── escalation/        # Retry loops, tier escalation, failure analysis, learning extraction
├── methodology/       # ATDD phases, TDD wiring, refactor phase
├── validation/        # Direct validation commands, recovery, auto-fix
├── reviewpkg/         # Light review, thorough review, result application
│
├── runner.go          # Runner struct, Run(), Status(), processBead(), constructors (~800-900 lines)
├── process.go         # setupBeadContext, buildPromptForBead (orchestration only, ~300-400 lines)
├── format.go          # Display formatting (unchanged)
├── format_bead_breakdown.go  # (unchanged)
├── status.go          # StatusWriter (unchanged)
├── syncwriter.go      # Thread-safe writer (unchanged)
├── interfaces.go      # BeadClient, PromptRenderer, IterationLogger, FailureAnalyzer
└── test_helpers.go
```

### Dependency Flow

```
runner/ (facade)
  ├── imports runtypes/
  ├── imports execution/
  ├── imports escalation/
  ├── imports methodology/
  ├── imports validation/
  └── imports reviewpkg/

Each sub-package:
  ├── imports runtypes/
  └── defines own narrow interfaces (no sibling imports)
```

No circular imports. Sub-packages do not import each other or the facade.

### Key Design Decisions

1. **Extract-and-Delegate** — each sub-package has its own struct with narrow interfaces, not a shared god-object context. Compile-time coupling detection.
2. **`runtypes/` for shared types** — breaks circular import cycle. Contains only data types and function signatures, no logic.
3. **Callbacks over sibling imports** — facade passes function values (e.g., `InvokeFn`, `ExecuteFn`) instead of letting sub-packages import each other.
4. **Type aliases for backward compat** — `type IterationResult = runtypes.IterationResult` in facade means existing test files compile unchanged.
5. **Keep DecomposeTask/CreateSubBeads in facade** — at ~170 lines, extraction adds more boilerplate than it saves.
6. **`reviewpkg` name** — avoids confusion with `internal/review/`.

## Test Strategy

### Existing Tests (Safety Net)

All ~60 existing test files in `runner/` remain and pass. Test assertions stay identical — only setup may change where types move (type aliases handle most cases). These serve as integration-level regression coverage.

### Sub-Package Unit Tests

Each sub-package gets `_test.go` files exercising its exported API with mock dependencies following the existing callback-mock pattern. Key cases:

- **`execution/`** — Invoker returns correct InvocationResult fields; heartbeat detects stalls; usage limit errors surface
- **`escalation/`** — SelectTier maps priority+labels; ExecuteWithRetry retries then escalates; learning extraction invokes renderer correctly; decomposition triggers at threshold
- **`validation/`** — RunDirect executes commands; RunWithRecovery tries auto-fix then Claude fix; ExtractValidationSummary caps at 500 chars
- **`methodology/`** — AcceptanceTests escalates on failure; VerifyTestsFail detects passing-tests-as-failure; ShouldRunRefactor gates on tier+file count
- **`reviewpkg/`** — RunLight delegates with correct tier; ApplyResult creates beads/backlog; SelectReviewTier maps correctly

### Verification

After each extraction: `go build ./internal/runner/...`, `go test ./internal/runner/... -count=1`, `golangci-lint run ./internal/runner/...`.

## Implementation Tasks

### Task 1: Create runtypes/ Package with Shared Types

**Files:**
- Create: `internal/runner/runtypes/types.go`
- Modify: `internal/runner/runner.go` (add type aliases)

**What to Do:**
Create the `runtypes/` package containing all types that cross sub-package boundaries:

- `BeadContext` struct — promoted from unexported `beadContext` with all 22 fields exported (`Bead`, `Parent`, `Result`, `Model`, `Tier`, `BuildProvider`, `PromptCtx`, `BuildPrompt`, `StartCommit`, `Iteration`, `RetriesThisModel`, `TotalRetriesThisBead`, `MaxRetries`, `MaxRetriesPerBead`, `ParentCtx`, `BeadTimeout`, `RunDeadline`, `ScopeEstimate`)
- `IterationResult` struct — moved from runner.go:268-298 with all fields
- `SubTask` struct — moved from runner.go:301-306
- Function types: `GitDiffFn`, `CmdRunnerFn`, `AutoFixFn`

Add type aliases in `runner.go` for backward compatibility:
```go
type IterationResult = runtypes.IterationResult
type SubTask = runtypes.SubTask
```

Do NOT migrate references yet — that's Task 2.

**Acceptance Criteria:**
- `runtypes/` package compiles with all types exported
- Type aliases in runner/ mean existing code referencing `IterationResult` and `SubTask` still compiles
- `go test ./internal/runner/... -count=1` passes

**Dependencies:** None (foundational)

**Notes:** The `beadContext` type is currently only used within the runner package (unexported), so creating the exported version in runtypes/ doesn't break anything yet. The actual migration happens in Task 2.

---

### Task 2: Migrate beadContext to runtypes.BeadContext

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Replace all uses of the local `beadContext` struct with `runtypes.BeadContext`:

- Delete the `beadContext` struct definition from `process.go:27-52`
- Update `setupBeadContext()` to return `*runtypes.BeadContext`
- Promote all field references: `bc.bead` → `bc.Bead`, `bc.model` → `bc.Model`, `bc.tier` → `bc.Tier`, `bc.result` → `bc.Result`, `bc.parent` → `bc.Parent`, `bc.promptCtx` → `bc.PromptCtx`, `bc.buildPrompt` → `bc.BuildPrompt`, `bc.startCommit` → `bc.StartCommit`, `bc.iteration` → `bc.Iteration`, `bc.retriesThisModel` → `bc.RetriesThisModel`, `bc.totalRetriesThisBead` → `bc.TotalRetriesThisBead`, `bc.maxRetries` → `bc.MaxRetries`, `bc.maxRetriesPerBead` → `bc.MaxRetriesPerBead`, `bc.parentCtx` → `bc.ParentCtx`, `bc.beadTimeout` → `bc.BeadTimeout`, `bc.runDeadline` → `bc.RunDeadline`, `bc.scopeEstimate` → `bc.ScopeEstimate`, `bc.buildProvider` → `bc.BuildProvider`
- Update all function signatures that accept/return `*beadContext` to use `*runtypes.BeadContext`

This is mechanical find-and-replace across ~150 reference sites in process.go and runner.go.

**Acceptance Criteria:**
- No remaining references to the old `beadContext` type
- `go build ./internal/runner/...` compiles cleanly
- `go test ./internal/runner/... -count=1` passes with no assertion changes

**Dependencies:** Task 1

**Notes:** Most invasive single step but purely mechanical. Test files reference `beadContext` only indirectly through the facade (it was unexported), so they should need minimal changes. If any test files directly construct `beadContext`, update them to `runtypes.BeadContext`.

---

### Task 3: Extract execution/ Package

**Files:**
- Create: `internal/runner/execution/invoker.go`
- Create: `internal/runner/execution/heartbeat.go`
- Create: `internal/runner/execution/invoker_test.go`
- Create: `internal/runner/execution/heartbeat_test.go`
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Extract Claude invocation and heartbeat monitoring into `execution/`:

**Invoker** (`invoker.go`):
- `Invoker` struct holding narrow interfaces: `Router` (Select, MarkUnavailable), `Provider` (StreamRun, IsUsageLimitError, Name), plus config fields (stall timeouts, usage limit config)
- `InvocationResult` struct: provider.Result, stream stats, stall flag, model name, provider name
- `Execute(ctx, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error)` — extracted from `executeClaudeInvocation` (process.go:161-275)

**Heartbeat** (`heartbeat.go`):
- `StartHeartbeat` / `StartHeartbeatWithConfig` — extracted from runner.go:1070-1161
- `PrintHeartbeat` — extracted from runner.go:1162-1181
- Narrow interface for output: `OverwriteWriter` with `WriteOverwrite([]byte) (int, error)`

Update facade:
- Add `invoker *execution.Invoker` field to Runner struct
- Wire in `NewRunner`/`NewRunnerWithDeps` constructors
- Replace `executeClaudeInvocation` call in process.go with `r.invoker.Execute()`
- Replace heartbeat calls with `execution.StartHeartbeat()`

**Acceptance Criteria:**
- `execution/` compiles independently (imports only runtypes/ and external packages)
- Facade delegates to execution.Invoker for Claude invocation
- All existing runner tests pass unchanged

**Dependencies:** Task 2

---

### Task 4: Extract escalation/ Package

**Files:**
- Create: `internal/runner/escalation/handler.go`
- Create: `internal/runner/escalation/learning.go`
- Create: `internal/runner/escalation/tierselect.go`
- Create: `internal/runner/escalation/handler_test.go`
- Create: `internal/runner/escalation/learning_test.go`
- Create: `internal/runner/escalation/tierselect_test.go`
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Extract retry/escalation logic, learning extraction, and tier selection:

**Handler** (`handler.go`):
- `Handler` struct with narrow interfaces: `FailureAnalyzer`, `BeadClient` (for decomposition), config fields
- `InvokeFn` callback type — facade passes `execution.Invoker.Execute` wrapped to match
- `ExecuteWithRetry(ctx, bc) error` — from process.go:874-973 (the main retry loop)
- `HandleStallTimeout(ctx, bc) error` — from process.go:279-311
- `AnalyzeAndHandleFailure(ctx, bc, output) error` — from process.go:467-532
- `HandleEscalation(ctx, bc) error` — from process.go:536-566
- `AttemptDecomposition(ctx, bc) error` — from process.go:571-594
- `EscalateTier(bc) error` — from process.go:598-610

**Learning** (`learning.go`):
- `ExtractLearning(ctx, bc, analysis)` — from process.go:332-350
- `ExtractSuccessLearning(ctx, bc)` — from process.go:385-463
- `ExtractSyntheticLearning(ctx, bc, message)` — from process.go:353-369
- `ExtractScopeTooLargeLearning(ctx, bc)` — from process.go:372-375
- `ExtractTimeoutLearning(ctx, bc)` — from process.go:378-381

**Tier Selection** (`tierselect.go`):
- `SelectTier(bead, cfg) string` — from runner.go:976-984
- `SelectModel(bead, cfg) string` — from runner.go:995-1012

Update facade:
- Add `escalationHandler *escalation.Handler` to Runner
- Wire InvokeFn callback in constructor
- Replace `executeWithRetry` in processBead() with `r.escalationHandler.ExecuteWithRetry()`
- Replace `selectTier`/`selectModel` calls with `escalation.SelectTier()`/`escalation.SelectModel()`

**Acceptance Criteria:**
- `escalation/` defines own narrow interfaces, does not import `execution/` or facade
- InvokeFn callback wires execution through facade without direct coupling
- All existing runner tests pass unchanged

**Dependencies:** Task 3

**Notes:** This is the largest extraction (~600 lines of logic). During decompose, this task will likely become 2-3 beads: one for handler/retry core, one for learning extraction, one for tier selection.

---

### Task 5: Extract validation/ Package

**Files:**
- Create: `internal/runner/validation/runner.go`
- Create: `internal/runner/validation/summary.go`
- Create: `internal/runner/validation/runner_test.go`
- Create: `internal/runner/validation/summary_test.go`
- Modify: `internal/runner/process.go`
- Delete: `internal/runner/validation_summary.go` (moved to sub-package)

**What to Do:**
Extract direct validation execution and recovery:

**Runner** (`runner.go`):
- `Runner` struct with narrow interfaces: `CmdRunnerFn`, `AutoFixFn`, config fields (validation commands, max retries)
- `ExecuteFn` callback type — for Claude-based fix attempts (facade passes escalation handler's invoke)
- `RunDirect(ctx, bc, commands) (*Result, error)` — from process.go `runDirectValidationCheck` (922-949)
- `Run(ctx, bc) error` — from process.go `runValidation` (1149-1234)
- `RunWithRecovery(ctx, bc) error` — from process.go `runValidationWithRecovery` (1239-1311)

**Summary** (`summary.go`):
- `ExtractValidationSummary(output string) string` — moved from validation_summary.go:1-39

Update facade:
- Add `validationRunner *validation.Runner` to Runner
- Wire ExecuteFn callback in constructor
- Replace `runDirectValidationCheck`/`runValidation`/`runValidationWithRecovery` calls with delegation
- Delete `validation_summary.go` from runner/ root

**Acceptance Criteria:**
- `validation/` compiles independently, does not import escalation/ or execution/
- ExecuteFn callback wires Claude fix attempts without direct coupling
- `extractValidationSummary` tests move to or are duplicated in sub-package

**Dependencies:** Task 4

---

### Task 6: Extract methodology/ Package

**Files:**
- Create: `internal/runner/methodology/executor.go`
- Create: `internal/runner/methodology/executor_test.go`
- Modify: `internal/runner/process.go`

**What to Do:**
Extract ATDD/TDD/refactor workflow phases:

**Executor** (`executor.go`):
- `Executor` struct with narrow interfaces: config fields, plus callbacks for escalation (`EscalateTierFn`), validation (`RunDirectFn`), and Claude invocation (`InvokeFn`)
- `RunAcceptanceTestsWithRetry(ctx, bc) error` — from process.go:627-660
- `RunAcceptanceTests(ctx, bc) (*InvocationResult, error)` — from process.go:665-794
- `VerifyTestsFailWithRetry(ctx, bc) error` — from process.go:798-855
- `VerifyTestsFail(ctx, bc) error` — from process.go:860-893
- `RunRefactorPhase(ctx, bc) error` — from process.go:1027-1089
- `HandleRefactorValidationFailure(ctx, bc) error` — from process.go:1093-1143
- `RunRefactorWithRouter(ctx, bc) error` — from process.go:954-996
- `ShouldRunRefactor(bc, cfg) bool` — from process.go:1000-1021
- `IsTestOnlyDiff(diff string) bool` — from process.go:898-916
- `CountChangedFiles(diff string) int` — from process.go:1315-1326

Update facade:
- Add `methodologyExec *methodology.Executor` to Runner
- Wire callbacks in constructor
- Replace ATDD/refactor calls in `processBead()` with delegation

**Acceptance Criteria:**
- `methodology/` does not import any sibling sub-packages
- Uses callbacks for escalation and validation
- All existing ATDD/refactor tests pass through facade

**Dependencies:** Task 5

---

### Task 7: Extract reviewpkg/ Package

**Files:**
- Create: `internal/runner/reviewpkg/reviewer.go`
- Create: `internal/runner/reviewpkg/reviewer_test.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Extract all review functionality:

**Reviewer** (`reviewer.go`):
- `Reviewer` struct with narrow interfaces: `Router`, `BeadClient`, `PromptRenderer`, config fields, plus callbacks for validation
- `RunLight(ctx, bc, iteration) error` — from runner.go:1819-1948
- `RunThorough(ctx, iteration, beadID) error` — from runner.go:2081-2214
- `ApplyResult(ctx, result, beadID) error` — from runner.go:1949-2006
- `RunPostSuccess(ctx, bc) error` — from process.go `runPostSuccessReview` (1329-1373)
- `SelectReviewModel(buildModel string, cfg) string` — from runner.go
- `SelectReviewTier(buildTier string) string` — from runner.go:988-993
- Helper functions: `buildReviewBeadLabels`, `buildBacklogLabels`, `writeReviewLog`

Update facade:
- Add `reviewer *reviewpkg.Reviewer` to Runner
- Wire in constructors
- Replace review calls in `Run()` and `processBead()` with delegation

**Acceptance Criteria:**
- `reviewpkg/` does not import sibling sub-packages or facade
- Named `reviewpkg` to avoid collision with `internal/review/`
- All existing review tests pass through facade

**Dependencies:** Task 2 (only needs runtypes/, independent of other extractions — but ordered last to reduce merge conflicts since it modifies runner.go's main loop)

---

### Task 8: Final Verification and Facade Cleanup

**Files:**
- Modify: `internal/runner/runner.go` (cleanup)
- Modify: `internal/runner/process.go` (cleanup)

**What to Do:**
Final verification pass:
- Confirm `runner.go` is under 1,000 lines
- Confirm `process.go` is under 1,000 lines
- Confirm no sub-package production file exceeds 500 lines
- Verify no sub-package imports another sub-package or the facade
- Run full test suite: `go test ./internal/runner/... -count=1`
- Run linter: `golangci-lint run ./internal/runner/...`
- Verify `cmd/gromit/main.go` requires no changes (public API unchanged)
- Clean up any dead imports, unused type aliases, or orphaned helper functions in the facade

**Acceptance Criteria:**
- `runner.go` < 1,000 lines, `process.go` < 1,000 lines
- No sub-package file > 500 lines
- `go test ./internal/runner/... -count=1` passes
- `golangci-lint run ./internal/runner/...` clean
- `go build ./cmd/gromit` succeeds with no changes to main.go

**Dependencies:** Tasks 3, 4, 5, 6, 7

---

## Notes

- **Migration order matters.** runtypes first, then each extraction one at a time. After each: build, test, commit. Don't batch extractions.
- **beadContext promotion is the riskiest step.** It's mechanical but touches ~150 sites. A systematic find-and-replace with compilation checks after each field is the safest approach.
- **Type aliases prevent cascade.** `type IterationResult = runtypes.IterationResult` means the ~24K lines of test code referencing `IterationResult` need zero changes.
- **Narrow interfaces per sub-package are key.** Don't use the facade's interfaces (BeadClient, PromptRenderer) — each sub-package defines only what it needs. This catches accidental coupling at compile time.
- **Callback wiring happens in constructors.** `NewRunner` and `NewRunnerWithDeps` both construct all sub-package handlers. Test injection works by constructing sub-package handlers with test-specific callbacks.
- **The facade's `processBead()` becomes a sequencer.** After extraction, it calls: setupBeadContext → methodology (if ATDD) → escalation.ExecuteWithRetry → validation.RunWithRecovery → methodology.RunRefactorPhase → reviewpkg.RunPostSuccess. Each call is one line of delegation.
