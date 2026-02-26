---
id: runner-subpackage-split
source_ideas: []
created: 2026-02-13
epic: codebase-health
---

# Runner Sub-Package Split

## Specification

Split `internal/runner/` into focused sub-packages to reduce the amount of code an agent must read to orient on a bead. The package currently has 60 files, ~27K lines, and 15 internal dependencies. Two production files dominate: `runner.go` (2,287 lines) and `process.go` (1,328 lines). An agent working on validation logic must wade through escalation, ATDD, review, and heartbeat code to find what matters.

### Goal

When an agent works on a bead that touches one concern (say, escalation logic), it reads only that sub-package's ~400 lines rather than all 3,600 lines of production code.

### Approach: Extract-and-Delegate

Each sub-package owns a struct with narrow dependency interfaces. The `runner/` facade constructs these structs during initialization and delegates to them. A shared `runtypes/` package provides types that cross sub-package boundaries, breaking the circular import between `runner/` and its children.

Sub-packages do not import each other. The facade wires them together through callbacks and function types.

### Package Layout

```
internal/runner/
├── runtypes/          # BeadContext, IterationResult, common function types
├── execution/         # Claude invocation, heartbeat, stall detection
├── escalation/        # Retry loops, tier escalation, failure analysis, learning extraction
├── methodology/       # ATDD phases, TDD wiring, refactor phase
├── validation/        # Direct validation commands, recovery, auto-fix
├── reviewpkg/         # Light review, thorough review, result application
│
├── runner.go          # Runner struct, Run(), Status(), processBead(), constructors
├── process.go         # setupBeadContext, buildPromptForBead (orchestration only)
├── format.go          # Display formatting (unchanged)
├── format_bead_breakdown.go
├── status.go          # StatusWriter (unchanged)
├── syncwriter.go      # Thread-safe writer (unchanged)
├── interfaces.go      # BeadClient, PromptRenderer, IterationLogger, FailureAnalyzer
└── test_helpers.go
```

### `runtypes/` — Shared Types

Contains types that multiple sub-packages need, preventing circular imports.

- `BeadContext` — promoted from unexported `beadContext`. Shared state for processing a single bead: the bead itself, parent, iteration result, model/tier, prompt context, retry counters, timeout settings, scope estimate.
- `IterationResult` — outcome of one loop iteration. Runner exports a type alias for backward compatibility.
- Common function types: `LogFn`, `CmdRunner`, `DiffProvider`.

Does not contain package-specific interfaces. Each sub-package defines its own narrow deps.

### `execution/` — Claude Invocation

Encapsulates how Gromit talks to Claude: one invocation at a time, with heartbeat monitoring and stall detection.

**Moves from:** `process.go` (`executeClaudeInvocation`), `runner.go` (heartbeat functions, `heartbeatConfig`, `syncWriter`)

**Exports:**
- `Invoker` struct with an `Execute(ctx, bc, prompt)` method returning `*InvocationResult`
- `InvocationResult` struct: Claude result, stream stats, stall flag, model name, provider name

**Narrow interfaces:** `Router` (Select, MarkUnavailable), `Provider` (StreamRun, IsUsageLimitError, Name)

Does not decide whether to retry or escalate. Reports what happened; the caller decides.

### `escalation/` — Retry and Escalation

Decides what to do when things fail: retry with the same model, escalate to a higher tier, attempt decomposition, or stop. Also extracts learnings from failures and successes.

**Moves from:** `process.go` (`executeWithRetry`, `handleStallTimeout`, `handleEscalation`, `analyzeAndHandleFailure`, `escalateTier`, `escalateModel`, `attemptDecomposition`, all `extractLearning` variants), `runner.go` (`selectTier`, `selectModel`)

**Exports:**
- `Handler` struct with `ExecuteWithRetry`, `AnalyzeAndHandleFailure`, `HandleStallTimeout`, `EscalateTier`
- Learning extraction: `ExtractLearning`, `ExtractSuccessLearning`, `ExtractSyntheticLearning`
- Tier/model selection: `SelectTier`, `SelectModel`

The `ExecuteWithRetry` method accepts an `InvokeFn` callback — the facade passes `execution.Invoker.Execute` wrapped to match. This avoids a direct import between escalation and execution.

### `methodology/` — ATDD/TDD/Refactor

Handles methodology-specific workflow phases: writing acceptance tests, verifying they fail before implementation, and running the refactor phase after validation passes.

**Moves from:** `process.go` (`runAcceptanceTestsWithRetry`, `runAcceptanceTests`, `verifyTestsFailWithRetry`, `verifyTestsFail`, `runRefactorPhase`, `handleRefactorValidationFailure`, `runRefactorWithRouter`, `shouldRunRefactor`, `isTestOnlyDiff`, `countChangedFiles`)

**Exports:**
- `Executor` struct with `RunAcceptanceTestsWithRetry`, `VerifyTestsFailWithRetry`, `RunRefactorPhase`, `ShouldRunRefactor`

Uses callbacks for escalation (tier changes) and direct validation (running test commands) to avoid importing sibling sub-packages.

### `validation/` — Direct Validation

Runs test and lint commands directly via `exec.Command`, handles recovery (trivial auto-fix via gofmt/goimports, then Claude-based fix), and extracts validation summaries from failure output.

**Moves from:** `process.go` (`runValidation`, `runValidationWithRecovery`, `runDirectValidationCheck`), `validation_summary.go`

**Exports:**
- `Runner` struct with `RunWithRecovery`, `RunDirect`
- `ExtractValidationSummary` standalone function

The `RunWithRecovery` method accepts an `ExecuteFn` callback for Claude-based fix attempts, avoiding a direct import of the escalation package.

### `reviewpkg/` — Code Review

All review functionality: light reviews after each bead, thorough periodic reviews, creating beads and backlog items from review findings.

Named `reviewpkg` to avoid import path collision with the existing `internal/review/` package.

**Moves from:** `runner.go` (`runLightReview`, `runThoroughReview`, `applyReviewResult`, `runPostSuccessReview`, `selectReviewModel`, `selectReviewTier`, `buildReviewBeadLabels`, `buildBacklogLabels`, `writeReviewLog`)

**Exports:**
- `Reviewer` struct with `RunLight`, `RunThorough`, `ApplyResult`, `RunPostSuccess`
- `SelectReviewModel`, `SelectReviewTier` standalone functions

### `runner/` Facade — What Stays

After extraction, the facade retains the public API and orchestration:

- `Runner` struct gains handler fields: `invoker`, `escalationHandler`, `methodologyExec`, `validationRunner`, `reviewer`
- Constructors (`NewRunner`, `NewRunnerWithDeps`) build the sub-package handlers
- `Run()` — main loop, precheck, scope gate, stuck bead detection
- `processBead()`, `buildPromptForBead()`, `setupBeadContext()` — orchestrate the sub-packages in sequence
- `DecomposeTask()`, `CreateSubBeads()` — small enough to stay (~170 lines)
- `Status()`, `getNextBead()`, `SetLabelFilters()`
- Formatting files (`format.go`, `format_bead_breakdown.go`), `status.go`, `syncwriter.go` — already separate

Estimated facade size after extraction: ~800-900 lines.

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
  └── defines own narrow interfaces
```

No circular imports. Sub-packages do not import each other or the facade.

### Migration Order

Each sub-package is extracted one at a time. After each extraction: update the facade to delegate, run existing tests, commit.

1. `runtypes/` — shared types first (everything else depends on this)
2. `execution/` — self-contained
3. `escalation/` — depends on execution via callback
4. `validation/` — depends on escalation via callback
5. `methodology/` — depends on validation and escalation via callbacks
6. `reviewpkg/` — self-contained

### Field Promotion

The `beadContext` struct has unexported fields (`bead`, `model`, `tier`, etc.). Moving it to `runtypes/` requires promoting all fields to exported (`Bead`, `Model`, `Tier`, etc.). Every reference in `process.go`, `runner.go`, and test files must update. This is the most invasive single step but is mechanical.

## Acceptance Criteria

- `runner.go` and `process.go` each contain fewer than 1,000 lines of production code
- No sub-package production file exceeds 500 lines
- Sub-packages do not import each other or the `runner/` facade
- All existing tests pass without modification to test assertions (test setup may change to use sub-package types)
- External callers (`cmd/gromit/main.go`) require no changes
- `go test ./internal/runner/... -count=1` passes
- `golangci-lint run ./internal/runner/...` reports no errors
- Each sub-package has its own `_test.go` exercising its exported API with mock dependencies

## Decisions

1. **Extract-and-Delegate over Shared RunContext.** Each sub-package defines its own struct and narrow interfaces rather than sharing a god-object context. Narrow interfaces make dependencies explicit and catch accidental coupling at compile time. The tradeoff is some delegation boilerplate in the facade.

2. **`runtypes/` package for shared types.** Go forbids circular imports. Since sub-packages need `BeadContext` and the facade needs sub-packages, a small types-only package breaks the cycle. It contains only data types and function signatures — no logic.

3. **Callbacks over sibling imports.** Where one sub-package's logic needs another (e.g., validation recovery needs Claude retry), the facade passes a callback function rather than letting sub-packages import each other. This keeps the dependency graph a strict tree.

4. **Keep `DecomposeTask`/`CreateSubBeads` in the facade.** At ~170 lines total, decomposition is small enough that extracting it adds more interface boilerplate than it saves in orientation time. If it grows, it can be extracted later.

5. **Name `reviewpkg` to avoid collision.** `internal/runner/review/` would compile (different import path than `internal/review/`), but the similar names invite confusion. `reviewpkg` makes the distinction obvious.

6. **Promote `beadContext` fields to exported.** Moving to `runtypes/` requires exported fields. This is a mechanical but invasive change. The alternative — accessor methods on an unexported struct — adds hundreds of lines of boilerplate for no benefit.

7. **Medium granularity (5-6 sub-packages).** A finer split (one per concern) would create 10+ tiny packages with heavy interface overhead. A coarser split (2-3) wouldn't reduce orientation time enough. Five sub-packages grouped by the natural seams in the code hit the right balance.

## Research & Context

### Current State

`internal/runner/` is the core of Gromit. It imports 15 internal packages and is imported by 3 files outside itself (`cmd/gromit/main.go`, one logger test, one mock test).

Production code by file:

| File | Lines | Concern |
|------|-------|---------|
| `runner.go` | 2,287 | Loop, constructors, review, decompose, precheck, scope, git, formatting orchestration |
| `process.go` | 1,328 | Bead processing, invocation, retry, escalation, ATDD, validation, refactor, learning |
| `format.go` | 249 | Status display formatting |
| `status.go` | 151 | StatusWriter for status.json |
| `syncwriter.go` | 54 | Thread-safe writer |
| `format_bead_breakdown.go` | 47 | Bead breakdown formatting |
| `validation_summary.go` | 40 | Validation error extraction |
| `interfaces.go` | 71 | 4 interfaces + compile-time checks |

`runner.go` and `process.go` together hold 3,615 lines mixing 8+ concerns. The remaining files are small and focused — they don't need splitting.

### Test Structure

Tests use `NewRunnerWithDeps` for dependency injection with mock structs that have callback fields (e.g., `mockBeadClient.ReadyFn`). This pattern transfers directly to sub-package tests: each sub-package defines its own mock types for its narrow interfaces.

60 test files total, ~24K lines of test code. Integration tests that exercise the full pipeline through `Run()`/`processBead()` stay in `runner/`. Unit tests that exercise isolated concerns (tier selection, validation summary extraction, review model selection) move to the relevant sub-package.

### Related Work

- The `internal/provider/` package already demonstrates the extract-and-delegate pattern: `Router` holds multiple `Provider` implementations and selects between them.
- The `internal/pipeline/` package demonstrates clean input/output struct patterns.
