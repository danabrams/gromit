---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T05:48:26-05:00"
id: atdd-methodology
source_spec: atdd-methodology
---

# ATDD Methodology Implementation Plan

**Goal:** Add a configurable ATDD methodology system that splits the build workflow into acceptance-test-writing, verify-tests-fail, build, validate, refactor, and review phases when active for a bead.

**Architecture:** New `MethodologyConfig` in config with independent boolean toggles. Methodology-aware branching in `processBead()` inserts acceptance test and refactor phases. Three new prompt templates. Methodology labels inherited during decomposition.

**Tech Stack:** Go, Go templates, YAML config

**Spec:** `.gromit/specs/atdd-methodology.md`

---

## Architecture

### Overview

When ATDD is active for a bead (via global config or per-bead label override), the processing pipeline becomes:

```
processBead()
  → setupBeadContext()           [existing]
  → buildPromptForBead()         [existing: scope check + build context]
  → isATDDActive()?
    YES → runAcceptanceTests()   [NEW: render PROMPT_acceptance_tests.md → StreamRun]
        → verifyTestsFail()      [NEW: RunValidation → expect failure]
        → re-render build prompt with RenderATDDBuild()
  → executeWithRetry()           [existing: runs build]
  → runValidation()              [existing]
  → isRefactorActive()?          [true when atdd OR tdd active]
    YES → runRefactorPhase()     [NEW: render PROMPT_refactor.md → Run → re-validate]
  → review                       [existing]
```

### Key Components

1. **MethodologyConfig** — `ATDD bool`, `TDD bool` fields in config, default `false`
2. **Methodology resolution** — `IsMethodologyActive()` helper checks bead labels then global default
3. **Acceptance test phase** — Separate `StreamRun` invocation with same model as build
4. **Verify-tests-fail** — Runs validation, expects failure. Retry once with analysis if tests pass.
5. **ATDD-aware build** — Separate template telling Claude to make acceptance tests pass
6. **Refactor phase** — Separate invocation after validation, re-validates after
7. **Methodology inheritance** — Sub-beads get methodology labels during decomposition

### Tradeoffs

- Reuse `prompt.Context` for acceptance tests (same fields needed, different template)
- Separate `PROMPT_atdd_build.md` rather than conditionals in existing build template
- `StreamRun` for acceptance tests (heartbeat/stall detection, matches build pattern)

## Test Strategy

### Unit Tests
- Config: `MethodologyConfig` parsing, defaults, label resolution
- Bead: `IsMethodologyActive()` with label combinations
- Prompt: New render methods with template execution
- Runner/process: Each new method via `NewRunnerWithDeps` with mocks

### Integration Tests
- Full ATDD flow through `processBead()`
- Tests-pass-before-implementation retry and fail behavior
- Build failure preserves acceptance tests
- Refactor failure revert-and-retry, then skip
- Methodology inheritance through `CreateSubBeads()`

### Mocking Strategy
- Mock `ClaudeClient` for phase-specific return values
- Mock `PromptRenderer` to verify correct render methods called
- Mock `BeadClient` for sub-bead label verification
- Real templates from temp dir for rendering tests

---

## Implementation Tasks

### Task 1: Add MethodologyConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add a `MethodologyConfig` struct with `ATDD bool` and `TDD bool` fields (yaml tags `atdd` and `tdd`). Add a `Methodology MethodologyConfig` field to `Config` with yaml tag `methodology`. No defaults needed in `setDefaults()` since `false` is the correct zero-value default.

**Acceptance Criteria:**
- `methodology.atdd` and `methodology.tdd` booleans parse from YAML correctly
- Both default to `false` when absent from config
- Table-driven tests cover: present true, present false, absent (defaults to false)

**Dependencies:** None

### Task 2: Add methodology label helper to bead package

**Files:**
- Modify: `internal/bead/bead.go`
- Test: `internal/bead/bead_test.go`

**What to Do:**
Add `IsMethodologyActive(labels []string, methodologyName string, globalDefault bool) bool` that checks for `<methodologyName>:true` and `<methodologyName>:false` labels. If found, return the label's value. If not found, return `globalDefault`. This follows the existing `HasLabel` and `FindSpecLabel` patterns.

**Acceptance Criteria:**
- `atdd:true` label returns `true` regardless of global default
- `atdd:false` label returns `false` regardless of global default
- No matching label falls back to global default
- Table-driven tests cover all combinations

**Dependencies:** None

### Task 3: Add prompt render methods and templates for acceptance tests, ATDD build, and refactor

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/interfaces.go`
- Create: `.gromit/templates/PROMPT_acceptance_tests.md`
- Create: `.gromit/templates/PROMPT_atdd_build.md`
- Create: `.gromit/templates/PROMPT_refactor.md`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Add three new render methods to `Renderer`: `RenderAcceptanceTests(ctx *Context)`, `RenderATDDBuild(ctx *Context)`, and `RenderRefactor(ctx *Context)`. All reuse the existing `prompt.Context` type. Each calls `r.render()` with its template name. Add corresponding methods to the `PromptRenderer` interface in `interfaces.go`.

Create three prompt templates:

`PROMPT_acceptance_tests.md` — Same context sections as build (rules, learnings, CLAUDE.md, task details, spec, parent, retry) but with different instructions:
- Explore codebase to understand test patterns, frameworks, conventions
- Write acceptance/integration tests based on acceptance criteria
- Each criterion maps to at least one test
- Follow existing test conventions
- Only create/modify test files — no implementation code
- Commit test files with clear message

`PROMPT_atdd_build.md` — Same as standard build template but instructions say:
- Acceptance tests have been written and committed
- Your job is to make the acceptance tests pass
- Study the failing tests to understand the behavioral contract
- Implement code to satisfy the tests
- Do not modify the acceptance test files

`PROMPT_refactor.md` — Same context sections plus instructions:
- Review implementation for code quality, naming, structure, duplication
- Refactor to improve clarity and maintainability
- Do not change behavior — all tests must continue to pass
- Commit refactoring changes separately

**Acceptance Criteria:**
- All three render methods work and produce non-empty output with valid context
- `PromptRenderer` interface includes all three new methods
- Templates render without errors using representative context data

**Dependencies:** None

**Notes:** Update mock implementations in `interfaces_test.go` to include stubs for new methods. This is needed for tasks 4-7 to compile.

### Task 4: Implement runAcceptanceTests method

**Files:**
- Modify: `internal/runner/process.go`
- Test: `internal/runner/process_test.go`

**What to Do:**
Add `runAcceptanceTests(ctx context.Context, bc *beadContext) error` method to Runner. This method:
1. Renders the acceptance test prompt via `r.renderer.RenderAcceptanceTests(bc.promptCtx)`
2. Calls `r.claude.StreamRun()` with the rendered prompt and `bc.model` (same model as build)
3. Uses the same heartbeat/stall detection pattern as `executeClaudeInvocation()`
4. On success: returns nil (tests written and committed)
5. On failure: returns error with context about what went wrong

The method should support retry with analysis context (for the case where the acceptance test phase itself fails). Add `acceptanceTestRetries` tracking to `beadContext` or pass retry state as parameters.

**Acceptance Criteria:**
- Calls `StreamRun` with the correct model and rendered acceptance test prompt
- Returns nil on successful Claude invocation
- Returns error on Claude failure with descriptive message

**Dependencies:** Task 3 (needs render methods and interface updates)

**Notes:** Error handling and escalation for the acceptance test phase itself is wired in Task 6.

### Task 5: Implement verifyTestsFail method

**Files:**
- Modify: `internal/runner/process.go`
- Test: `internal/runner/process_test.go`

**What to Do:**
Add `verifyTestsFail(ctx context.Context, bc *beadContext) error` method to Runner. This method:
1. Runs validation via `r.claude.RunValidation()` with the configured validation commands
2. If validation **fails** (tests fail): return nil — this is the expected/good outcome
3. If validation **passes** (tests pass before implementation): this is bad — return a sentinel error or specific error type indicating "tests passed before implementation"

The caller (`processBead`) handles the retry logic: on first "tests passed" error, re-run acceptance tests with analysis context, then verify again. On second "tests passed", fail the bead with the specified message.

**Acceptance Criteria:**
- Returns nil when validation fails (tests fail as expected)
- Returns specific error when validation passes (tests pass unexpectedly)
- Uses configured validation commands and validation model

**Dependencies:** Task 3

### Task 6: Implement runRefactorPhase method

**Files:**
- Modify: `internal/runner/process.go`
- Test: `internal/runner/process_test.go`

**What to Do:**
Add `runRefactorPhase(ctx context.Context, bc *beadContext) error` method to Runner. This method:
1. Gets git diff from `bc.startCommit` to current state via `getGitDiff()`
2. If no diff, skip refactoring (return nil)
3. Capture current git HEAD as `preRefactorCommit`
4. Renders refactor prompt via `r.renderer.RenderRefactor(bc.promptCtx)`
5. Calls `r.claude.Run()` (not StreamRun — refactoring is typically simpler) with same model as build
6. Re-validates via `r.claude.RunValidation()`
7. If re-validation fails: revert via `git reset --hard preRefactorCommit`, retry once with analysis context
8. If retry refactor also fails validation: skip refactoring (log warning, return nil — working code without refactoring is better than broken)
9. On success: return nil

**Acceptance Criteria:**
- Gets diff, renders refactor prompt, calls Claude with build model
- Re-validates after refactoring
- On validation failure: reverts to pre-refactor state and retries once
- On second failure: skips refactoring with warning (returns nil, not error)

**Dependencies:** Task 3

**Notes:** The git revert uses `git reset --hard <commit>` which is a destructive operation, but it's reverting to a known-good state (post-build, pre-refactor). This is safe because refactoring should not have changed behavior.

### Task 7: Wire ATDD phases into processBead

**Files:**
- Modify: `internal/runner/process.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
Modify `processBead()` to check methodology and insert ATDD phases:

1. After `buildPromptForBead()`, check if ATDD is active: `bead.IsMethodologyActive(b.Labels, "atdd", r.cfg.Methodology.ATDD)`
2. If ATDD active:
   a. Call `runAcceptanceTests()` — if fails, handle with retry/escalation (same pattern as build failure)
   b. Call `verifyTestsFail()` — if tests pass, retry acceptance tests once with analysis, verify again, fail bead if still passing
   c. Re-render build prompt using `RenderATDDBuild()` instead of `RenderBuild()`
3. Proceed with `executeWithRetry()` as normal (with ATDD-aware build prompt)
4. After `runValidation()` succeeds, check if refactor is active: ATDD or TDD active
5. If refactor active: call `runRefactorPhase()`
6. Continue to review

Also handle ATDD-specific escalation: when build fails under ATDD, preserve acceptance tests. The existing `executeWithRetry` already handles retry/escalation — the key difference is that the build prompt is the ATDD variant. No special preservation logic needed since acceptance tests are already committed.

For acceptance test phase failure escalation: implement a retry loop similar to `executeWithRetry` but simpler (retry with analysis context on same model → escalate model → fail bead).

**Acceptance Criteria:**
- ATDD-active beads run acceptance tests before build
- Verify-tests-fail check runs after acceptance tests
- Tests passing before implementation retries once then fails bead with correct message
- Build uses ATDD build prompt when ATDD is active
- Refactor phase runs after validation when ATDD (or TDD) is active
- Non-ATDD beads follow existing flow unchanged

**Dependencies:** Tasks 1, 2, 4, 5, 6

**Notes:** This is the main integration task. Most of the logic lives in the individual method implementations; this task wires them together in the correct order with the correct control flow.

### Task 8: Add methodology inheritance to CreateSubBeads

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
Modify `CreateSubBeads()` to inject methodology labels when the methodology is globally active but not already present in the parent's labels. Before creating each sub-bead:

1. Start with `b.Labels` (parent's labels — already inherited)
2. If `r.cfg.Methodology.ATDD` is true and no `atdd:true` or `atdd:false` label exists in labels, add `atdd:true`
3. If `r.cfg.Methodology.TDD` is true and no `tdd:true` or `tdd:false` label exists in labels, add `tdd:true`
4. Pass the augmented labels to `CreateWithParentAndDescription`

This ensures sub-beads inherit methodology even when it's set globally rather than via labels. When the parent already has `atdd:true` or `atdd:false`, it passes through unchanged.

**Acceptance Criteria:**
- Sub-beads get `atdd:true` label when global ATDD is true and parent has no atdd label
- Sub-beads preserve parent's `atdd:false` label (not overridden by global true)
- No duplicate labels when parent already has `atdd:true`

**Dependencies:** Tasks 1, 2

### Task 9: Register new templates in gromit init

**Files:**
- Modify: `cmd/gromit/init.go` (or wherever init template registration lives)

**What to Do:**
Ensure `gromit init` creates the three new template files (`PROMPT_acceptance_tests.md`, `PROMPT_atdd_build.md`, `PROMPT_refactor.md`) in the `.gromit/templates/` directory. Follow the existing pattern for how init creates template files. Also add `methodology` section to the generated `gromit.yaml` with commented defaults.

**Acceptance Criteria:**
- `gromit init` creates all three new template files
- Generated `gromit.yaml` includes commented `methodology` section

**Dependencies:** Task 3 (templates must exist)

**Notes:** Check how existing templates are embedded/registered in init. May use `embed.FS` or file copies.

---

## Notes

- The refactor phase is shared between ATDD and TDD specs. This plan implements it fully; the TDD spec will reuse it.
- The `MethodologyConfig` struct accommodates both ATDD and TDD fields. The TDD plan will add `RenderTDDBuild` and `PROMPT_tdd_build.md` — no config changes needed.
- Build failures under ATDD naturally preserve acceptance tests because they're already committed to git before the build phase starts. No special git preservation logic is needed.
- The acceptance test phase uses `StreamRun` (not `Run`) to get heartbeat/stall detection, matching the build phase pattern.
- Template mock stubs in `interfaces_test.go` need updating in Task 3 — all subsequent runner tests depend on this.
