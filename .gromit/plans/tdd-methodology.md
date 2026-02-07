---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T05:54:36-05:00"
id: tdd-methodology
source_spec: tdd-methodology
---

# TDD Methodology Implementation Plan

**Goal:** Add TDD support with a prescriptive red-green-refactor build prompt, wired into the existing processBead flow alongside the shared refactor phase.

**Architecture:** A new `PROMPT_tdd_build.md` template and `RenderTDDBuild` method replace the standard build prompt when TDD is active. TDD prompt selection happens in `processBead()` after the standard prompt is built. All shared infrastructure (MethodologyConfig, IsMethodologyActive, refactor phase, methodology inheritance) is already being built by the ATDD beads.

**Tech Stack:** Go, Go templates, YAML config

**Spec:** `.gromit/specs/tdd-methodology.md`

---

## Architecture

### Overview

When TDD is active for a bead (via global config `methodology.tdd: true` or per-bead `tdd:true` label), the build prompt is swapped from the standard `PROMPT_build.md` to `PROMPT_tdd_build.md`, which instructs Claude to follow strict red-green-refactor cycles with one-test-at-a-time commits.

The refactor phase (shared with ATDD, already being built) fires after validation when either methodology is active.

### Flow

```
processBead()
  → setupBeadContext()         [existing]
  → buildPromptForBead()       [existing: renders standard build prompt]
  → isTDDActive()?
    YES → re-render with RenderTDDBuild()   [NEW]
  → executeWithRetry()         [existing]
  → runValidation()            [existing]
  → isRefactorActive()?        [shared: atdd OR tdd]
    YES → runRefactorPhase()   [shared: ATDD beads]
  → review                     [existing]
```

When both ATDD and TDD are active:
```
  → ATDD acceptance tests      [ATDD beads]
  → ATDD verify-tests-fail     [ATDD beads]
  → TDD build prompt           [THIS plan]
  → executeWithRetry()
  → runValidation()
  → runRefactorPhase()         [shared]
  → review
```

### Key Decision

The TDD build prompt reuses `prompt.Context` (same fields as standard build). Only the instructions differ. This avoids creating a new context type and keeps the change minimal.

## Test Strategy

### Unit Tests
- `RenderTDDBuild` produces non-empty output with valid context
- TDD prompt selection: when active, `RenderTDDBuild` called; when inactive, `RenderBuild` used
- Mock stubs compile and satisfy updated interface

### Integration Tests
- Full `processBead` with TDD active via `NewRunnerWithDeps`
- Refactor phase fires when only TDD active
- Combined ATDD+TDD ordering

### Mocking Strategy
- Mock `PromptRenderer` with `RenderTDDBuildFn` field
- Existing mock patterns for Claude, beads, analyzer

---

## Implementation Tasks

### Task 1: Create PROMPT_tdd_build.md template

**Files:**
- Create: `.gromit/templates/PROMPT_tdd_build.md`

**What to Do:**
Create the TDD build prompt template. It uses the same context sections as the standard build template (rules, learnings, CLAUDE.md, task details, spec, parent, retry) but with TDD-specific instructions:

- Follow the red-green-refactor cycle strictly
- Write ONE small failing unit test that addresses a piece of the requirements
- Write the minimum code to make that test pass
- Commit the test + implementation together with a descriptive message
- Do not write multiple tests before implementing — one at a time
- Focus each test on a single behavior or requirement from the bead
- After all requirements are covered, stop — refactoring will happen in a separate phase
- Repeat the cycle for each piece of the requirement

Copy the context sections (rules, learnings, CLAUDE.md, task, spec, parent, retry) directly from the existing `PROMPT_build.md` template in `cmd/gromit/init.go` (the `defaultBuildTemplate` constant), then replace the Instructions and Completion sections with TDD-specific guidance.

**Acceptance Criteria:**
- Template renders without errors using `prompt.Context` data
- Instructions clearly prescribe one-test-at-a-time red-green cycles
- Context sections match the standard build template structure

**Dependencies:** None

### Task 2: Add RenderTDDBuild method and update interface

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/interfaces.go`
- Modify: `internal/runner/interfaces_test.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Add `RenderTDDBuild(ctx *Context) (string, error)` method to `Renderer` in `prompt.go`. It calls `r.render("PROMPT_tdd_build.md", ctx)` — identical pattern to `RenderBuild`.

Add the method to the `PromptRenderer` interface in `interfaces.go`.

Add stub implementations to both `mockPromptRenderer` and `mockRenderer` in `interfaces_test.go`:
- `mockPromptRenderer` gets a `RenderTDDBuildFn` field and dispatches to it (or returns `"mock tdd build prompt", nil`)
- `mockRenderer` gets a simple stub returning `"mock tdd build prompt", nil`

Add a test in `prompt_test.go` that creates a temp dir with the TDD build template, creates a `Renderer`, and verifies `RenderTDDBuild` produces non-empty output with a valid `Context`.

**Acceptance Criteria:**
- `RenderTDDBuild` method works and produces non-empty output
- `PromptRenderer` interface includes the new method
- Both mock renderers satisfy the updated interface (compile check)

**Dependencies:** Task 1 (template must exist for rendering test)

### Task 3: Wire TDD build prompt selection into processBead

**Files:**
- Modify: `internal/runner/process.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
In `processBead()`, after `buildPromptForBead()` succeeds and before `executeWithRetry()`, add TDD prompt selection:

```go
// Select TDD build prompt if TDD is active
if bead.IsMethodologyActive(b.Labels, "tdd", r.cfg.Methodology.TDD) {
    tddPrompt, err := r.renderer.RenderTDDBuild(bc.promptCtx)
    if err != nil {
        bc.result.Error = fmt.Errorf("rendering TDD build prompt: %w", err)
        return bc.result
    }
    bc.buildPrompt = tddPrompt
    r.log("TDD active: using red-green-refactor build prompt")
}
```

This goes after the existing ATDD check (when ATDD beads are implemented) so that when both are active, the TDD build prompt is used for the build phase (ATDD acceptance tests happen earlier in a separate invocation).

Also add the refactor trigger check after `runValidation()` succeeds. If either `atdd` or `tdd` is active for the bead, call `runRefactorPhase()`. This may already be wired by ATDD beads — if so, just ensure the condition checks both flags:

```go
if bead.IsMethodologyActive(b.Labels, "tdd", r.cfg.Methodology.TDD) ||
   bead.IsMethodologyActive(b.Labels, "atdd", r.cfg.Methodology.ATDD) {
    if err := r.runRefactorPhase(ctx, bc); err != nil {
        // log warning but don't fail — refactor is best-effort
    }
}
```

Add tests via `NewRunnerWithDeps`:
- TDD-active bead: verify `RenderTDDBuildFn` is called (use mock renderer with tracking)
- TDD-inactive bead: verify standard `RenderBuildFn` is used, `RenderTDDBuildFn` is not called
- `tdd:true` label on bead overrides global `tdd: false`

**Acceptance Criteria:**
- TDD-active beads use the TDD build prompt
- TDD-inactive beads use the standard build prompt (no regression)
- Label override works: `tdd:true` on bead with global `tdd: false`

**Dependencies:** Task 2 (needs `RenderTDDBuild` method and interface)

**Notes:** This task depends on `IsMethodologyActive` and `MethodologyConfig` from the ATDD beads. If those aren't merged yet, the bead implementing this task will need to check and potentially implement them (they're trivial — a struct with two bools and a label-checking function). The ATDD beads for these are: `ralph-runner-qqby` (MethodologyConfig) and `ralph-runner-xyp1` (IsMethodologyActive).

### Task 4: Register PROMPT_tdd_build.md in gromit init

**Files:**
- Modify: `cmd/gromit/init.go`

**What to Do:**
Add the TDD build template to `gromit init`. Follow the existing pattern:

1. Add a `defaultTDDBuildTemplate` constant with the template content (same as `.gromit/templates/PROMPT_tdd_build.md`)
2. Add a `writeFileIfNotExists` call for `.gromit/templates/PROMPT_tdd_build.md` in `runInit`
3. Add `PROMPT_tdd_build.md` to the `Long` description of the init command

**Acceptance Criteria:**
- `gromit init` creates `PROMPT_tdd_build.md` in `.gromit/templates/`
- `gromit init` help text lists the new template

**Dependencies:** Task 1 (template content)

**Notes:** The ATDD beads (ralph-runner-i9vn) are also modifying init.go to add ATDD templates. This task should coordinate — if the ATDD init bead lands first, just add the TDD template alongside. If not, this task adds only the TDD template.

---

## Notes

- This plan is intentionally small. Most of the TDD infrastructure is shared with ATDD and already has open beads. The TDD-specific work is: one template, one render method, and prompt selection wiring.
- The refactor phase wiring in Task 3 may already exist when the ATDD beads land. If so, Task 3 only needs to ensure the condition checks `tdd` in addition to `atdd`.
- The `MethodologyConfig` struct (with `TDD bool` field) and `IsMethodologyActive` helper are prerequisites from the ATDD beads. If they haven't landed when these tasks are decomposed, the decomposer should include them as dependencies or inline them.
- Task 3 is the integration point. Tasks 1, 2, and 4 are independent of each other and can be parallelized.
