---
id: tdd-fresh-context-per-cycle
source_spec: tdd-fresh-context-per-cycle
created: 2026-02-19
decomposed: true
---

# Fresh Context Per TDD Cycle Implementation Plan

**Goal:** Replace the single-invocation TDD build with a multi-cycle orchestrator where each red-green-refactor phase is a separate Claude invocation with structured handoffs.

**Architecture:** New `internal/runner/tdd/` package with a `CycleOrchestrator` that manages the red→green→refactor loop. Between phases, Go code assembles handoff structs carrying actual file contents and test output. New per-phase templates (`PROMPT_tdd_red.md`, `PROMPT_tdd_green.md`) replace the monolithic `PROMPT_tdd_build.md`. Existing `RunRefactorPhase` and `PROMPT_refactor.md` are reused for the refactor step.

**Tech Stack:** Go, text/template, existing runner dependency injection patterns

**Spec:** `.gromit/specs/tdd-fresh-context-per-cycle.md`

---

## Architecture

### Overview

The current TDD path renders a single `PROMPT_tdd_build.md` prompt and sends it to one Claude invocation that does all red-green-refactor cycles internally. This causes context pollution in late cycles and quadratic token scaling.

The new approach:
- Each TDD cycle consists of three separate Claude invocations (red, green, refactor)
- Between invocations, the runner assembles a structured handoff with actual file contents
- Each phase prompt is minimal — only what that phase needs to act
- The runner manages cycle termination based on spec coverage, max cycles, or failure

### Key Components

1. **`internal/runner/tdd/handoff.go`** — Data types for phase handoffs and cycle state
2. **`internal/runner/tdd/assembly.go`** — Go functions that read files, capture test output, build handoffs
3. **`internal/runner/tdd/orchestrator.go`** — `CycleOrchestrator` managing the multi-cycle loop
4. **`internal/prompt/prompt.go`** — `TDDRedContext`, `TDDGreenContext` types and render methods
5. **`.gromit/templates/PROMPT_tdd_red.md`** — Focused red phase template
6. **`.gromit/templates/PROMPT_tdd_green.md`** — Focused green phase template
7. **`internal/runner/process_methodology.go`** — New TDD cycle path replacing single-invocation render

### Integration Points

- `process_methodology.go:prepareMethodologyForBead` — when TDD active, delegates to orchestrator instead of rendering single prompt
- `callbacks.go` — wires orchestrator dependencies using narrow function types
- `runner.go` / `constructor.go` — adds `tddOrchestrator` field
- `interfaces.go` — adds `RenderTDDRed`, `RenderTDDGreen` to `PromptRenderer`
- `methodology/executor.go` — existing `RunRefactorPhase` reused directly for TDD refactor phase

### Data Flow Per Cycle

```
1. Runner assembles RedHandoff (reads existing files, extracts spec excerpt)
2. RenderTDDRed(RedHandoff) → Claude invocation → new test committed
3. Runner runs `go test` on touched packages → captures failure output
4. Runner assembles GreenHandoff (failing test content, failure output, impl files)
5. RenderTDDGreen(GreenHandoff) → Claude invocation → implementation committed
6. Runner runs `go test` on touched packages → verifies pass
7. Runner assembles RefactorHandoff (impl + test file contents)
8. Existing RunRefactorPhase → Claude invocation → cleanup committed
9. Runner runs `go test` on touched packages → verifies still passes
10. Update cycle state → if requirements remain, next cycle; else final validation
```

### Tradeoffs

- **New `tdd/` package vs extending `methodology/`**: New package because TDD cycles are fundamentally different from ATDD's single Red-Green. Cycle management, multi-cycle termination, and per-cycle handoff assembly would bloat the executor.
- **Reuse `RunRefactorPhase`**: Refactor logic (render, invoke, validate, revert-on-failure) is identical. Only the handoff assembly before calling it differs.
- **Handoff carries content, not paths**: Eliminates the "discovery tax" where each invocation spends tokens reading files.
- **Phase-level escalation**: If green fails, escalate that phase's model. Don't restart the whole cycle.

---

## Test Strategy

### Unit Tests

**Handoff assembly** (`tdd/assembly_test.go`):
- First-cycle red handoff with no existing files
- Subsequent-cycle red handoff reads existing test + impl files
- Green handoff captures failing test and failure output
- Refactor handoff reads current impl and test files
- Cycle-to-cycle handoff updates remaining requirements

**Cycle orchestrator** (`tdd/orchestrator_test.go`):
- Single cycle, all phases succeed → terminates
- Multiple cycles before coverage complete
- Terminates on max cycles
- Terminates when red phase signals all requirements tested
- Terminates on unrecoverable failure
- Red phase escalation (retry once, then escalate model)
- Green phase escalation (retry once, then escalate model)
- Refactor failure reverts and continues to next cycle
- Lightweight validation between phases (touched packages only)
- Full validation after all cycles

**Prompt rendering** (`prompt/prompt_test.go`):
- TDD red renders only spec excerpt, cycle summary, test files — no learnings, no ClaudeMD
- TDD green includes failing test and failure output — no spec, no history

**Config** (`config/config_test.go`):
- MaxTDDCycles defaults to 10
- YAML override works

### Mocking Strategy

- Claude invocations: `TDDInvokeFn` function type (same pattern as `InvokeFn`, `RefactorInvokeFn`)
- Validation: existing `ValidateDirectFn`
- File reads: `ReadFileFn` function type injected into assembly
- Git diff: existing `GetDiffFn` pattern
- Git head: existing `GetGitHeadFn` pattern

### Test Organization

- `internal/runner/tdd/handoff_test.go`
- `internal/runner/tdd/assembly_test.go`
- `internal/runner/tdd/orchestrator_test.go`
- `internal/prompt/prompt_test.go` (additions)
- `internal/runner/process_methodology_test.go` (additions)
- `internal/config/config_test.go` (additions)

---

## Implementation Tasks

### Task 1: Add TDD handoff data types

**Files:**
- Create: `internal/runner/tdd/handoff.go`
- Test: `internal/runner/tdd/handoff_test.go`

**What to Do:**
Define the data types that carry context between TDD phases. `RedHandoff` carries spec excerpt, existing test/impl file contents, API surface summary, and cycle summary. `GreenHandoff` carries the failing test content, test failure output, and current impl file contents. `RefactorHandoff` carries current impl and test file contents. `CycleState` tracks cycle number, covered requirements, remaining requirements, and touched file paths.

Types:
```go
type RedHandoff struct {
    SpecExcerpt      string            // current requirement to test
    TestFiles        map[string]string // path → content
    ImplFiles        map[string]string // path → content
    APISurface       string            // public signatures of code under test
    CycleSummary     string            // "Cycle N. Tests for X pass. Next: test Z."
}

type GreenHandoff struct {
    FailingTest      string            // verbatim test that failed
    TestFailureOutput string           // error messages, stack trace
    ImplFiles        map[string]string // path → content
}

type RefactorHandoff struct {
    ImplFiles        map[string]string // path → content
    TestFiles        map[string]string // path → content
}

type CycleState struct {
    CycleNumber      int
    MaxCycles        int
    CoveredSoFar     []string          // requirements already tested
    Remaining        []string          // requirements not yet tested
    TouchedFiles     []string          // all files changed across cycles
    Done             bool              // set by red phase when all requirements covered
}
```

**Acceptance Criteria:**
- All four handoff types defined with exported fields and doc comments
- `CycleState` has an `IsComplete()` method returning true when `Done || CycleNumber >= MaxCycles`
- Nil map fields in handoff types are safe (no panics on read)

**Dependencies:** None

---

### Task 2: Add MaxTDDCycles config field and FreshContextPerCycle feature flag

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add `MaxTDDCycles int` field with YAML tag `max_tdd_cycles` to `MethodologyConfig`. Default to 10 in `SetDefaults()` when zero. *(Done — commit c29a465.)*

Add `FreshContextPerCycle bool` field with YAML tag `fresh_context_per_cycle` to `MethodologyConfig`. Default is `false` (Go zero value — no `SetDefaults` change needed). When `false`, TDD uses existing single-invocation behavior. When `true`, TDD delegates to the new `CycleOrchestrator`. Add test for YAML deserialization of the flag. Document in `gromit.yaml`.

**Acceptance Criteria:**
- `MaxTDDCycles` defaults to 10 when unset *(done)*
- `FreshContextPerCycle` defaults to `false` when unset
- YAML deserialization populates both fields correctly
- Documented in gromit.yaml with comments explaining purpose

**Dependencies:** None

---

### Task 3: Add TDDRedContext and TDDGreenContext with render methods

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/interfaces.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Add `TDDRedContext` struct with fields: `BeadID`, `BeadTitle`, `SpecExcerpt`, `TestFileContents` (map), `APISurface`, `CycleSummary`, `Rules`, `WorkDir`, `ScopedTestCommand`, `IsRetry`, `FailureContext`, `PrevFailure`. Add `TDDGreenContext` struct with fields: `BeadID`, `BeadTitle`, `FailingTest`, `TestFailureOutput`, `ImplFileContents` (map), `Rules`, `WorkDir`, `ScopedTestCommand`, `IsRetry`, `FailureContext`, `PrevFailure`.

Add `RenderTDDRed(ctx *TDDRedContext) (string, error)` and `RenderTDDGreen(ctx *TDDGreenContext) (string, error)` methods to `Renderer`. Add both to `PromptRenderer` interface in `interfaces.go`. Follow the existing `RenderAcceptanceTests` pattern for template loading.

**Acceptance Criteria:**
- `TDDRedContext` contains no learnings, no ClaudeMD, no full spec — only phase-relevant fields
- `TDDGreenContext` contains no spec, no learnings, no history — only failing test + impl files
- Both render methods added to `PromptRenderer` interface with compile-time check

**Dependencies:** None

---

### Task 4: Create TDD red and green phase templates

**Files:**
- Create: `.gromit/templates/PROMPT_tdd_red.md`
- Create: `.gromit/templates/PROMPT_tdd_green.md`

**What to Do:**
Create `PROMPT_tdd_red.md` following the spec's prompt structure: Role (one sentence), Rules (short constraints), Context (handoff content: spec excerpt, test files, API surface, cycle summary), Task (write one failing test for the next requirement). Template uses `TDDRedContext` fields. Include retry section gated on `{{if .IsRetry}}`.

Create `PROMPT_tdd_green.md`: Role, Rules, Context (failing test verbatim, failure output, impl files), Task (make this test pass with minimal code). Template uses `TDDGreenContext` fields. Include retry section.

Both templates must be minimal — no learnings sections, no ClaudeMD blocks, no full spec. Each template should render file contents in fenced code blocks with file paths as labels.

**Acceptance Criteria:**
- Red template renders spec excerpt, cycle summary, and test/impl file contents only
- Green template renders failing test, failure output, and impl file contents only
- Both templates include self-check command via `{{.ScopedTestCommand}}`

**Dependencies:** Task 3 (context types define template fields)

---

### Task 5: Implement handoff assembly functions

**Files:**
- Create: `internal/runner/tdd/assembly.go`
- Test: `internal/runner/tdd/assembly_test.go`

**What to Do:**
Implement handoff assembly functions that build handoff structs between phases using filesystem reads and git operations. All external dependencies injected via function types.

Function types:
```go
type ReadFileFn func(path string) (string, error)
type GetDiffFn func(fromCommit string) (string, error)
type RunTestsFn func(ctx context.Context, commands []string, workDir string) (output string, passed bool, err error)
```

Functions:
- `AssembleRedHandoff(state CycleState, readFile ReadFileFn, getDiff GetDiffFn) (*RedHandoff, error)` — reads current test and impl files from `state.TouchedFiles`, extracts spec excerpt from `state.Remaining[0]`
- `AssembleGreenHandoff(testOutput string, readFile ReadFileFn, touchedFiles []string) (*GreenHandoff, error)` — reads the test file that was just written (from git diff), reads current impl files
- `AssembleRefactorHandoff(readFile ReadFileFn, touchedFiles []string) (*RefactorHandoff, error)` — reads all touched test and impl files
- `AssembleCycleState(prevState CycleState, redOutput string) CycleState` — increments cycle, moves covered requirements based on red phase's signal
- `ClassifyTouchedFiles(paths []string) (testFiles, implFiles []string)` — separates `_test.go` from non-test `.go` files

**Acceptance Criteria:**
- `AssembleRedHandoff` on first cycle (empty `TouchedFiles`) returns handoff with empty file maps and spec excerpt
- `AssembleGreenHandoff` captures test file contents and failure output
- `ClassifyTouchedFiles` correctly separates test files from implementation files

**Dependencies:** Task 1 (handoff types)

---

### Task 6: Implement TDD cycle orchestrator

**Files:**
- Create: `internal/runner/tdd/orchestrator.go`
- Test: `internal/runner/tdd/orchestrator_test.go`

**What to Do:**
Implement `CycleOrchestrator` struct with injected dependencies:

```go
type CycleOrchestrator struct {
    renderRedFn      func(*RedHandoff) (string, error)
    renderGreenFn    func(*GreenHandoff) (string, error)
    invokeFn         func(ctx context.Context, prompt string, tier string) error
    validateFn       func(ctx context.Context, commands []string, workDir string) (output string, passed bool, err error)
    runRefactorFn    func(ctx context.Context, bc *runtypes.BeadContext) error
    escalateTierFn   func(bc *runtypes.BeadContext, nextTier string) error
    getDiffFn        GetDiffFn
    readFileFn       ReadFileFn
    getGitHeadFn     func() (string, error)
    gitResetFn       func(commit string) error
    output           io.Writer
    cfg              *config.Config
}
```

Implement `RunCycles(ctx context.Context, bc *runtypes.BeadContext) error`:
```
state = initial CycleState from bc (spec requirements, max cycles from config)
for !state.IsComplete():
    1. AssembleRedHandoff → renderRedFn → invokeFn (red phase)
       - On failure: retry once with analysis, then escalateTierFn
    2. validateFn (touched packages) → capture failure output
       - If tests pass unexpectedly: signal "all requirements tested", set state.Done
    3. AssembleGreenHandoff → renderGreenFn → invokeFn (green phase)
       - On failure: retry once with analysis, then escalateTierFn
    4. validateFn (touched packages) → verify pass
       - On failure: retry green once with failure context
    5. runRefactorFn (delegates to existing RunRefactorPhase)
       - On failure: already handled internally (revert + skip)
    6. validateFn (touched packages) → verify still passes
    7. AssembleCycleState → update state
return nil (cycles complete, caller runs full validation)
```

Phase-level escalation: each phase maintains its own retry count. Red and green get `cfg.Escalation.MaxRetriesPerModel` retries before escalating. Refactor failures are non-blocking (existing behavior).

**Acceptance Criteria:**
- Single cycle with all phases succeeding produces correct sequence of calls (red invoke → validate → green invoke → validate → refactor → validate)
- Cycle terminates when `state.IsComplete()` returns true (max cycles or coverage done)
- Red phase failure retries once then escalates model; green phase likewise
- Refactor failure does not fail the cycle (continues to next cycle)

**Dependencies:** Task 1 (handoff types), Task 5 (assembly functions)

---

### Task 7: Wire TDD orchestrator into runner

**Files:**
- Modify: `internal/runner/runner.go` (or `constructor.go`)
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/runner/process_methodology.go`

**What to Do:**
Add `tddOrchestrator *tdd.CycleOrchestrator` field to `Runner`. In `makeMethodologyExec()` (or a new `makeTDDOrchestrator()` function in `callbacks.go`), construct the orchestrator wiring:
- `renderRedFn` → calls `r.renderer.RenderTDDRed(ctx)`
- `renderGreenFn` → calls `r.renderer.RenderTDDGreen(ctx)`
- `invokeFn` → calls `r.router.Select` + `p.StreamRun` (same pattern as ATDD's invokeFn in callbacks.go)
- `validateFn` → wraps `r.runDirectValidationCheck` to return (output, passed, error)
- `runRefactorFn` → calls `r.methodologyExec.RunRefactorPhase`
- `escalateTierFn` → calls `r.escalationHandler.EscalateTier`
- Other deps from existing runner fields

In `prepareMethodologyForBead`, when `tddActive`:
- **Check `cfg.Methodology.FreshContextPerCycle`** — if `false`, fall through to existing single-invocation TDD build prompt (preserving current behavior)
- If `true`, call `r.tddOrchestrator.RunCycles(ctx, bc)` instead of rendering a single TDD build prompt
- If `RunCycles` returns nil (success), set a flag so `executeBuildAndMethodologyLoop` knows cycles already ran
- If `RunCycles` returns error, set `bc.Result.Error` and return `done = true`

In `executeBuildAndMethodologyLoop`, when TDD cycles already ran:
- Skip the `executeWithRetry()` call (build already happened in cycles)
- Still run final full validation
- Skip refactor (already happened per-cycle)

**Acceptance Criteria:**
- When `tdd:true` AND `fresh_context_per_cycle: true`, orchestrator `RunCycles` is called instead of single TDD build prompt render
- When `tdd:true` but `fresh_context_per_cycle: false` (default), existing single-invocation behavior is preserved
- When `tdd:false` or TDD not active, existing behavior unchanged
- After cycles complete, full validation still runs

**Dependencies:** Task 6 (orchestrator), Task 3 (render methods)

---

### Task 8: Add TDD cycle integration tests

**Files:**
- Modify: `internal/runner/process_methodology_test.go`
- Test: new tests in existing file

**What to Do:**
Add integration-level tests that verify the TDD path through `prepareMethodologyForBead` and `executeBuildAndMethodologyLoop` with mock orchestrator:
- `TestTDD_FreshContext_DelegatesToOrchestrator` — TDD active + `fresh_context_per_cycle: true`, verify orchestrator called
- `TestTDD_FreshContext_FallsBackOnOrchestratorError` — orchestrator returns error, bead fails
- `TestTDD_FreshContext_FullValidationAfterCycles` — after cycles, full validation runs
- `TestTDD_FreshContext_FlagFalse_UsesSingleInvocation` — TDD active + `fresh_context_per_cycle: false` (default), verify existing single-invocation path used
- `TestTDD_LabelOverride_TDDFalse_SkipsOrchestrator` — `tdd:false` label bypasses orchestrator
- `TestTDD_ConfigToggle_PreservesExistingBehavior` — `methodology.tdd: false` in config, no orchestrator

**Acceptance Criteria:**
- TDD activation via labels and config toggles still works correctly
- Orchestrator is called when TDD active, skipped when not
- Full validation runs after successful cycle completion

**Dependencies:** Task 7 (wiring)

---

### Task 9: Final verification and cleanup

**Files:**
- No new files — verification only

**What to Do:**
Run `go test ./...`, `go vet ./...`, and `go build ./...` to confirm all quality gates pass. Verify:
- Existing TDD tests still pass (label overrides, config toggles)
- New TDD cycle tests pass
- No regressions in ATDD path
- `PROMPT_tdd_build.md` still exists (used as fallback or can be deprecated later)

**Acceptance Criteria:**
- `go test ./...` passes
- `go vet ./...` passes
- `go build ./...` passes

**Dependencies:** All previous tasks

---

## Notes

- **`PROMPT_tdd_build.md` is not deleted** — it remains as documentation of the old approach and potential fallback. A separate cleanup bead can deprecate it later.
- **ATDD path is untouched** — this spec only changes TDD. ATDD already has separate invocations.
- **The orchestrator's `invokeFn` pattern** mirrors the ATDD invokeFn in `callbacks.go` (lines ~200-280). Copy and adapt that pattern, including heartbeat/stall detection and codex transport-disconnect fallback.
- **Spec excerpt extraction** — the red phase needs a "current requirement to test" from the spec. The simplest approach: pass the full bead description as the spec excerpt for cycle 1, then let the red phase's output signal what's covered. The runner doesn't need to parse spec structure — it delegates coverage assessment to Claude's red phase.
- **"All requirements tested" signal** — the red phase should output a marker (e.g., `ALL_REQUIREMENTS_TESTED`) when it determines no more tests are needed. The orchestrator checks for this in the red phase output to set `state.Done = true`.
