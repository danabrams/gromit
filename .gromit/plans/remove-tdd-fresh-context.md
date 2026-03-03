---
created: 2026-03-02T00:00:00Z
decomposed: true
decomposed_at: "2026-03-02T23:24:02-05:00"
id: remove-tdd-fresh-context
source_spec: remove-tdd-fresh-context
---

# Remove TDD Fresh Context Implementation Plan

**Goal:** Delete the TDD fresh-context-per-cycle subsystem — 12 files deleted outright, 4 existing files surgically trimmed — leaving the shared-context TDD build path completely intact.

**Architecture:** Pure deletion. No new code. Remove the cycle orchestrator, its adapters, phase-specific templates, and all wiring from config, build stage, escalation wrapper, and constructor.

**Tech Stack:** Go

**Spec:** `.gromit/specs/remove-tdd-fresh-context.md`

---

## Architecture

Delete the fresh-context TDD subsystem while leaving the shared-context TDD path (`RenderTDDBuild` + single `StreamRun` invocation) fully intact.

**Files to Delete (15):**
- `internal/runner/tdd/orchestrator.go`, `orchestrator_test.go`
- `internal/runner/tdd/convergence.go`, `convergence_test.go`
- `internal/runner/tdd/handoff.go`, `handoff_test.go`
- `internal/runner/tdd/assembly.go`, `assembly_test.go`
- `internal/runner/callbacks_tdd.go`, `callbacks_tdd_test.go`, `callbacks_tdd_missing_outputs_test.go`
- `internal/runner/tdd_pipeline_adapter.go`, `tdd_pipeline_adapter_test.go`
- `.gromit/templates/PROMPT_tdd_red.md`
- `.gromit/templates/PROMPT_tdd_green.md`

**Files to Modify (4):**
- `internal/config/config_types.go` — drop `FreshContextPerCycle bool` field from `MethodologyConfig`
- `internal/pipeline/execute/build.go` — drop `TDDCycleResult`, `TDDCycleRunner` interface, `tddCycleRunner` field, `WithTDDCycleRunner()`, and the `if … FreshContextPerCycle && b.tddCycleRunner != nil` branch in `Run()`
- `internal/runner/escalation_build_stage.go` — drop `fallback pipeline.Stage` field, its `newEscalationBuildStage` parameter, and the `if methodology == MethodologyTDD && … FreshContextPerCycle` bypass in `Run()`
- `internal/runner/constructor.go` — remove `optionalTDDCycleRunner()` call and `buildStage.WithTDDCycleRunner(runner)` block

**Dependency chain being severed:**
```
constructor.go::optionalTDDCycleRunner()
  -> callbacks_tdd.go::buildTDDCycleRunner()
    -> tdd.NewCycleOrchestrator()
      -> tdd_pipeline_adapter.go::TDDPipelineAdapter
        -> build.go::TDDCycleRunner interface
          -> build.go::Run() fresh context branch
escalation_build_stage.go::Run() fresh context bypass
```

## Test Strategy

**Test Levels:**
1. **Deleted test files** — entire test files in `internal/runner/tdd/` and the dedicated runner test files are deleted wholesale
2. **Trimmed test files** — fresh-context test functions are removed from `build_test.go`, `constructor_test.go`, and `escalation_build_stage_test.go`; shared-context tests remain
3. **Verification** — `go test ./...` and `go vet ./...` confirm no dangling references or broken imports

**Test Cases to Remove:**

From `build_test.go`:
- `TestTDDCycleRunner_InterfaceSatisfied` — interface gone
- `TestBuildRun_TDD_FreshContext_DelegatesToTDDCycleRunner` and all variants
- `TestBuildRun_BuildStrategyTDDLabel_DelegatesToTDDCycleRunner`
- `TestBuildRun_BuildStrategySinglePassLabel_SkipsTDDCycleRunner`
- `fakeTDDCycleRunner` and `trackingTDDCycleRunner` helper types

From `constructor_test.go`:
- `TestBuildTDDCycleRunner_ReturnsTDDPipelineAdapter`
- `TestBuildTDDCycleRunner_RunnerHasConfiguredOrchestrator`
- `TestOptionalTDDCycleRunner_ReturnsNilWhenFreshContextDisabled`
- `TestOptionalTDDCycleRunner_ReturnsAdapterWhenFreshContextEnabled`
- `TestOptionalTDDCycleRunner_ReturnsNilWhenMethodologyAdapterIsNonGo`
- `TestNewRunnerImpl_BuildStageUsesTDDCycleRunner_WhenFreshContextPerCycle`

From `escalation_build_stage_test.go`:
- `stubFallbackStage` type and the fresh-context fallback delegation test

**Coverage Goals:**
- All existing shared-context TDD tests continue to pass
- `go test ./...` passes with zero fresh-context references
- `go vet ./...` passes with no unused imports

---

## Implementation Tasks

### Task 1: Delete `internal/runner/tdd/` package

**Files:**
- Delete: `internal/runner/tdd/orchestrator.go`, `orchestrator_test.go`
- Delete: `internal/runner/tdd/convergence.go`, `convergence_test.go`
- Delete: `internal/runner/tdd/handoff.go`, `handoff_test.go`
- Delete: `internal/runner/tdd/assembly.go`, `assembly_test.go`

**What to Do:**
Delete all 8 files. The entire directory goes away.

**Acceptance Criteria:**
- `internal/runner/tdd/` directory does not exist
- No file in the repo imports `github.com/danabrams/gromit/internal/runner/tdd`

**Dependencies:** None

---

### Task 2: Delete dedicated runner fresh-context files

**Files:**
- Delete: `internal/runner/callbacks_tdd.go`
- Delete: `internal/runner/callbacks_tdd_test.go`
- Delete: `internal/runner/callbacks_tdd_missing_outputs_test.go`
- Delete: `internal/runner/tdd_pipeline_adapter.go`
- Delete: `internal/runner/tdd_pipeline_adapter_test.go`

**What to Do:**
Delete all 5 files wholesale. They contain only fresh-context wiring and adapters.

**Acceptance Criteria:**
- None of these files exist in the repo
- `callbacks_tdd.go`, `tdd_pipeline_adapter.go` do not appear in any import or reference

**Dependencies:** Task 1 (tdd package must be gone before its users can compile)

---

### Task 3: Delete phase-specific prompt templates

**Files:**
- Delete: `.gromit/templates/PROMPT_tdd_red.md`
- Delete: `.gromit/templates/PROMPT_tdd_green.md`

**What to Do:**
Delete both template files. `PROMPT_tdd_build.md` stays untouched.

**Acceptance Criteria:**
- `PROMPT_tdd_red.md` and `PROMPT_tdd_green.md` do not exist
- `PROMPT_tdd_build.md` still exists and is unmodified

**Dependencies:** None

---

### Task 4: Remove `FreshContextPerCycle` from config

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: relevant config test file(s) that reference `FreshContextPerCycle`

**What to Do:**
Remove the `FreshContextPerCycle bool \`yaml:"fresh_context_per_cycle"\`` field from `MethodologyConfig`. Find and remove any test that sets or asserts on this field.

**Acceptance Criteria:**
- `FreshContextPerCycle` does not appear in any `.go` file
- `fresh_context_per_cycle` does not appear in any `.go` file
- `go build ./internal/config/...` passes

**Dependencies:** None

---

### Task 5: Remove `TDDCycleRunner` from `build.go` and clean `build_test.go`

**Files:**
- Modify: `internal/pipeline/execute/build.go`
- Modify: `internal/pipeline/execute/build_test.go`

**What to Do:**
From `build.go`:
- Delete `TDDCycleResult` struct
- Delete `TDDCycleRunner` interface
- Remove `tddCycleRunner TDDCycleRunner` field from `Build` struct
- Delete `WithTDDCycleRunner()` method
- Remove the entire `if methodology == MethodologyTDD && in.Config != nil && in.Config.Methodology.FreshContextPerCycle && b.tddCycleRunner != nil { … }` block from `Run()`

From `build_test.go`:
- Delete `fakeTDDCycleRunner`, `trackingTDDCycleRunner` types and all test functions that reference `TDDCycleRunner`, `WithTDDCycleRunner`, or `FreshContextPerCycle`

**Acceptance Criteria:**
- `TDDCycleRunner`, `TDDCycleResult`, `WithTDDCycleRunner` do not appear in `build.go`
- `FreshContextPerCycle` does not appear in `build.go` or `build_test.go`
- `go test ./internal/pipeline/execute/...` passes

**Dependencies:** Tasks 1, 2, 4

---

### Task 6: Remove fresh-context bypass from `escalation_build_stage.go` and its test

**Files:**
- Modify: `internal/runner/escalation_build_stage.go`
- Modify: `internal/runner/escalation_build_stage_test.go`

**What to Do:**
From `escalation_build_stage.go`:
- Remove `fallback pipeline.Stage` field from the struct
- Remove the `fallback pipeline.Stage` parameter from `newEscalationBuildStage()`
- Remove the `fallback: fallback` assignment in the constructor body
- Delete the bypass block at the top of `Run()`: `if methodology == execute.MethodologyTDD && in.Config != nil && in.Config.Methodology.FreshContextPerCycle { return s.fallback.Run(ctx, in) }`
- Update the comment on `escalationBuildStage` and `Run()` to remove all fresh-context mentions

From `escalation_build_stage_test.go`:
- Delete `stubFallbackStage` type
- Delete the test that sets `FreshContextPerCycle: true` and asserts fallback delegation

**Acceptance Criteria:**
- `fallback`, `FreshContextPerCycle`, and "fresh-context" references are gone from both files
- `newEscalationBuildStage` signature no longer has a `fallback` parameter
- `go build ./internal/runner/...` passes after also applying Task 7

**Dependencies:** Tasks 1, 2, 4, 5

---

### Task 7: Remove fresh-context wiring from `constructor.go` and clean `constructor_test.go`

**Files:**
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/constructor_test.go`

**What to Do:**
From `constructor.go`:
- Delete the `if runner := optionalTDDCycleRunner(…); runner != nil { buildStage.WithTDDCycleRunner(runner) }` block
- Update the `newEscalationBuildStage(…)` call to remove the `fallback` argument (previously `buildStage`)
- Remove any now-unused imports (the `tdd` package import and/or `callbacks_tdd` references if they were inline)

From `constructor_test.go`:
- Delete `TestBuildTDDCycleRunner_*` test functions
- Delete `TestOptionalTDDCycleRunner_*` test functions
- Delete `TestNewRunnerImpl_BuildStageUsesTDDCycleRunner_WhenFreshContextPerCycle`

**Acceptance Criteria:**
- `optionalTDDCycleRunner`, `WithTDDCycleRunner`, and `FreshContextPerCycle` do not appear in `constructor.go` or `constructor_test.go`
- `newEscalationBuildStage` call in `constructor.go` passes no `fallback` argument
- `go test ./internal/runner/...` passes

**Dependencies:** Tasks 1, 2, 4, 5, 6

---

### Task 8: Final verification

**Files:** None (read-only verification)

**What to Do:**
Run `go test ./...` and `go vet ./...`. Confirm:
- All acceptance criteria from the spec are met
- No references to `FreshContextPerCycle`, `TDDCycleRunner`, `TDDCycleResult`, `WithTDDCycleRunner`, `optionalTDDCycleRunner`, `buildTDDCycleRunner` remain in any `.go` file
- The shared-context TDD path still works: a TDD-methodology bead should route through `RenderTDDBuild` → `StreamRun`

**Acceptance Criteria:**
- `go test ./...` passes
- `go vet ./...` passes
- Grep for `FreshContextPerCycle` across the repo returns no `.go` matches

**Dependencies:** Tasks 1–7

---

## Notes

- `callbacks_tdd_missing_outputs_test.go` is an extra test file not called out explicitly in the spec but clearly belongs to the fresh-context system; delete it with Task 2.
- The `PhaseModelsConfig` fields `Red string` and `Green string` are not mentioned in the spec for removal. Verify after deletion whether anything still references them; if not, they can be removed as a follow-up.
- `newEscalationBuildStage` currently takes `fallback pipeline.Stage` as an argument. After Task 6 removes that field and parameter, Task 7 must update the call site in `constructor.go` to match — do Task 6 before Task 7.
- Check for any lingering import of `"github.com/danabrams/gromit/internal/runner/tdd"` after Tasks 1–2; `go vet` will catch it.
