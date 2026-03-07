---
id: remove-tdd-fresh-context
source_ideas: []
created: 2026-03-02
accepted: true
---

# Remove TDD Fresh Context System

## Specification

Delete the TDD fresh context per-cycle system (`FreshContextPerCycle=true`). This system runs separate Claude invocations per TDD phase (red, green, refactor) with context reset between phases. It adds deep complexity — a cycle orchestrator state machine, convergence detection, handoff types, special-case routing in build and escalation — without measurable improvement over the shared context TDD build, which handles red-green-refactor in a single Claude invocation.

The shared context TDD build (`PROMPT_tdd_build.md`, `RenderTDDBuild`) is completely independent of the fresh context infrastructure and is unaffected by this removal.

### What Gets Deleted

**Entire directory (8 files):**
- `internal/runner/tdd/` — orchestrator.go, convergence.go, handoff.go, assembly.go + their tests

**Dedicated files (4 files):**
- `internal/runner/callbacks_tdd.go` + test — entirely fresh context wiring
- `internal/runner/tdd_pipeline_adapter.go` + test — bridges fresh context to pipeline

**Prompt templates (2 files):**
- `.gromit/templates/PROMPT_tdd_red.md` — red phase prompt (fresh context only)
- `.gromit/templates/PROMPT_tdd_green.md` — green phase prompt (fresh context only)

**Code removed from existing files:**
- `FreshContextPerCycle` field from `config_types.go` + YAML tag + config test
- `TDDCycleRunner` interface from `internal/pipeline/execute/build.go`
- `WithTDDCycleRunner()` option from build stage
- Fresh context branch in `build.go` (the `if tddCycleRunner != nil` path)
- Fresh context bypass in `escalation_build_stage.go`
- `optionalTDDCycleRunner()` call from `constructor.go`
- Related test cases in build_test.go, escalation_build_stage_test.go, constructor_test.go

### What Stays Untouched

- `PROMPT_tdd_build.md` — shared context TDD prompt
- `RenderTDDBuild()` in `internal/prompt/` — renders the shared context prompt
- TDD methodology activation — `IsMethodologyActive`, labels, config toggles
- `BuildStrategy` field — still used for methodology selection
- The standard build path's TDD case in `build.go` (`case MethodologyTDD: prompt, err = b.renderer.RenderTDDBuild(...)`)

## Acceptance Criteria

- The `internal/runner/tdd/` directory does not exist
- `callbacks_tdd.go`, `callbacks_tdd_test.go`, `tdd_pipeline_adapter.go`, `tdd_pipeline_adapter_test.go` do not exist
- `PROMPT_tdd_red.md` and `PROMPT_tdd_green.md` do not exist
- `FreshContextPerCycle` does not appear in config types or YAML parsing
- `TDDCycleRunner` interface and `WithTDDCycleRunner` do not appear in build.go
- The build stage's TDD path uses only `RenderTDDBuild` (single invocation), with no cycle runner branch
- The escalation wrapper has no fresh-context special case
- `go test ./...` passes
- `go vet ./...` passes
- TDD shared context build continues to work (methodology=TDD with FreshContextPerCycle absent still renders `PROMPT_tdd_build.md` and runs single-invocation build)

## Decisions

1. **Delete, don't deprecate.** Single user, already disabled in config. No migration path needed.

2. **Delete callbacks_tdd.go entirely.** The file contains only fresh context wiring (`buildTDDCycleRunner`, render callbacks, `optionalTDDCycleRunner`). No shared context TDD logic lives there.

3. **Keep TDD methodology activation intact.** The `tdd: true` config and label-based activation are used by the shared context build path and are unrelated to fresh context.

## Research & Context

### Why It Doesn't Help

Fresh context was designed to prevent context bleed between TDD phases. In practice, Claude handles red-green-refactor well in a single invocation with the `PROMPT_tdd_build.md` prompt. The fresh context approach:
- Costs 3x more (separate invocations per phase)
- Bypasses escalation (special-cased out of the escalation handler)
- Adds a complex state machine for convergence detection and phase handoffs
- Introduces phase-specific prompt templates that duplicate guidance already in the build prompt

### Dependency Chain (All Fresh Context Only)

```
constructor.go::optionalTDDCycleRunner()
  -> callbacks_tdd.go::buildTDDCycleRunner()
    -> tdd.NewCycleOrchestrator()
      -> tdd_pipeline_adapter.go::TDDPipelineAdapter
        -> build.go::TDDCycleRunner interface
          -> build.go::Run() fresh context branch
escalation_build_stage.go::Run() fresh context bypass
```

### Key Files

- `internal/runner/tdd/orchestrator.go` — CycleOrchestrator (~500 lines)
- `internal/runner/tdd/convergence.go` — deadlock/oscillation detection
- `internal/runner/tdd/handoff.go` — RedHandoff, GreenHandoff, RefactorHandoff types
- `internal/runner/tdd/assembly.go` — handoff assembly helpers
- `internal/runner/callbacks_tdd.go` — wires orchestrator into runner
- `internal/runner/tdd_pipeline_adapter.go` — bridges to pipeline
- `internal/pipeline/execute/build.go:147` — decision point
- `internal/runner/escalation_build_stage.go:42` — bypass
- `internal/config/config_types.go:335` — FreshContextPerCycle field
