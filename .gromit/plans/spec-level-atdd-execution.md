---
created: 2026-02-16T00:00:00Z
decomposed: true
decomposed_at: "2026-02-16T01:30:40Z"
id: spec-level-atdd-execution
source_spec: spec-level-atdd-execution
---

# Spec-Level ATDD With Scoped Execution — Implementation Plan

**Goal:** Shift ATDD responsibility from individual beads to the spec lifecycle, with scoped execution via `--spec`/`--epic` and a spec-level acceptance gate that synthesizes fix beads on failure.

**Architecture:** Extend the runner with a spec-level orchestration layer. When `methodology.granularity` is `"spec"`, the runner authors acceptance tests once per spec before bead execution, skips per-bead ATDD phases, and runs a spec acceptance gate after the batch — synthesizing fix beads on failure and re-entering the scoped loop.

**Tech Stack:** Go, existing runner/config/prompt/bead/scope packages

**Spec:** `.gromit/specs/spec-level-atdd-execution.md`

---

## Architecture

**Key Components:**

1. **CLI Scope Flags** (`cmd/gromit/main.go`): `--spec` and `--epic` flags on `run` command. Resolve to label filters using existing `scope.ResolveSpec()` / `scope.ResolveEpic()`. Validate mutual exclusivity with `scope.ValidateFlags()`.

2. **Granularity Config** (`internal/config/config.go`): `Granularity string` on `MethodologyConfig` — `"bead"` (default) or `"spec"`. Config-level because it's a project decision, not per-invocation.

3. **Spec Orchestrator** (`internal/runner/spec_orchestrator.go`): Manages spec-level lifecycle when granularity is `"spec"` and scope is set. Wraps existing bead loop with pre-batch acceptance authoring and post-batch gate verification.

4. **Spec Acceptance Gate** (`internal/runner/spec_gate.go`): Post-batch verification running spec-level acceptance tests. Produces structured failure output (test name, message, file location).

5. **Fix Bead Synthesis** (`internal/runner/spec_fix.go`): Analyzes gate failures and creates targeted follow-up beads with the same `spec:<name>` label. Retry policy limits gate→fix→rerun cycles.

6. **Templates**: `PROMPT_spec_acceptance.md` (spec-level acceptance authoring) and `PROMPT_spec_gate.md` (gate failure analysis and fix bead generation).

**Integration Points:**
- `runner.Run()` gains spec-level outer loop when `granularity == "spec"` + label filters set
- `processBead()` skips ATDD phases when `granularity == "spec"`
- Existing `getNextBead()` label filtering handles scoped selection unchanged
- Existing escalation, retry, and provider routing untouched

**Data Flow (spec granularity mode):**
```
gromit run --spec foo
  → resolve labels: ["spec:foo"]
  → spec orchestrator:
      → load spec from .gromit/specs/foo.md
      → author spec acceptance tests (provider invocation)
      → commit tests
      → bead loop (processBead with ATDD phases skipped)
      → all beads done → spec gate
        → pass → done
        → fail → synthesize fix beads → re-enter bead loop
        → max retries exhausted → report and stop
```

**Tradeoffs:**
- Spec orchestrator in runner (not separate package) — needs provider routing, bead client, validation access; avoids excessive coupling
- Config-driven granularity (not flag-driven) — project-level decision, not per-invocation
- Reuses existing validation infrastructure for gate — avoids duplicating validation runner

## Test Strategy

**Unit Tests:**
- Spec orchestrator lifecycle (pre-batch → beads → gate → fix cycle)
- Granularity config parsing and defaults
- Gate pass/fail detection and structured failure output
- Fix bead synthesis from gate failure output
- ATDD phase skip logic when granularity is "spec"

**Integration Tests:**
- CLI flag parsing → scope resolution → label filter wiring
- Template rendering with real spec content and failure output
- End-to-end orchestrator with mock provider
- Fix bead creation through bead client (acceptance-tagged)

**Key Test Cases:**
- `--spec` resolves to correct label filter; only matching beads processed
- `--epic` resolves to multiple spec labels
- `--spec` and `--epic` mutually exclusive (error)
- `granularity: bead` (default) preserves all current behavior
- `granularity: spec` skips per-bead ATDD phases
- Spec authoring invokes provider and commits test files
- Gate detects pass/fail and produces structured output
- Gate failure triggers fix bead synthesis with `spec:<name>` label
- Retry policy limits gate→fix→rerun cycles
- Non-scoped `gromit run` works identically to current behavior

**Mocking:** Mock BeadClient, provider (via router), PromptRenderer. Real config loading and template files for integration tests.

**Test Organization:** Tests colocated with implementation — `*_test.go` alongside each new file, extending existing test files for config and prompt changes.

---

## Implementation Tasks

### Task 1: Add --spec and --epic flags to gromit run

**Files:**
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/main_test.go`

**What to Do:**
Add `--spec` and `--epic` string flags to the `run` command. In `runLoop()`, validate flags with `scope.ValidateFlags()`, resolve to label lists with `scope.ResolveSpec()` / `scope.ResolveEpic()`, and call `r.SetLabelFilters(labels)` before `r.Run()`. For `--spec`, also validate the spec file exists with `scope.ValidateSpec()`. Follow the same pattern as the retro command's flag handling.

**Acceptance Criteria:**
- `gromit run --spec foo` filters beads to `spec:foo` label only
- `gromit run --epic bar` resolves epic specs and filters to their labels
- `gromit run --spec x --epic y` returns a mutual exclusivity error

**Dependencies:** None

**Notes:** The scope package and runner label filtering already exist. This is pure wiring.

---

### Task 2: Add Granularity config field to MethodologyConfig

**Files:**
- Modify: `internal/config/config.go`
- Modify: `gromit.yaml`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `Granularity string` field with YAML tag `granularity` to `MethodologyConfig`. In `SetDefaults()`, default to `"bead"` when empty. Add validation in `Validate()` that rejects values other than `"bead"` and `"spec"`. Document the field in `gromit.yaml` with a comment explaining both modes.

**Acceptance Criteria:**
- `methodology.granularity` defaults to `"bead"` when unset
- YAML override to `"spec"` loads correctly
- Invalid values (e.g., `"foo"`) produce a validation error

**Dependencies:** None

---

### Task 3: Skip per-bead ATDD phases in spec granularity mode

**Files:**
- Modify: `internal/runner/runner.go` (`processBead` function)
- Test: `internal/runner/runner_test.go`

**What to Do:**
In `processBead()`, add a check before the ATDD phase block (~line 850). When `r.cfg.Methodology.Granularity == "spec"`, set `atddActive = false` regardless of label/config, effectively skipping all per-bead ATDD phases (write acceptance tests, verify fail, verify pass). TDD phases remain controlled independently. Log a message when ATDD is skipped due to spec granularity.

**Acceptance Criteria:**
- When `granularity: spec`, per-bead ATDD phases (write/verify-fail/verify-pass) are skipped
- When `granularity: bead` (default), all ATDD phases run as before
- TDD and refactor phases are unaffected by granularity setting

**Dependencies:** Task 2

---

### Task 4: Add spec acceptance and gate templates with render methods

**Files:**
- Create: `.gromit/templates/PROMPT_spec_acceptance.md`
- Create: `.gromit/templates/PROMPT_spec_gate.md`
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/interfaces.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Create `SpecAcceptanceContext` struct with fields: SpecName, SpecContent, Rules, Learnings, ExistingTests (string of any pre-existing acceptance tests). Create `SpecGateContext` struct with fields: SpecName, SpecContent, FailureOutput, AcceptanceCriteria. Add `RenderSpecAcceptance(*SpecAcceptanceContext) (string, error)` and `RenderSpecGate(*SpecGateContext) (string, error)` to `Renderer` and `PromptRenderer` interface. Write both templates: spec acceptance instructs the provider to write behavioral acceptance tests for the full spec contract; spec gate instructs the provider to analyze failures and output structured JSON with failing scenarios and suggested fix descriptions.

**Acceptance Criteria:**
- `RenderSpecAcceptance` renders template with spec content and rules
- `RenderSpecGate` renders template with failure output and spec criteria
- Both methods are on the `PromptRenderer` interface and mock implementations

**Dependencies:** None

**Notes:** Follow existing template patterns (PROMPT_acceptance_tests.md for acceptance authoring, PROMPT_analyze.md for structured failure output). Spec gate output should be JSON-parseable for fix bead synthesis.

---

### Task 5: Implement spec acceptance authoring phase

**Files:**
- Create: `internal/runner/spec_orchestrator.go`
- Test: `internal/runner/spec_orchestrator_test.go`

**What to Do:**
Create `SpecOrchestrator` struct with dependencies: renderer, provider router, bead client, spec content loader, config. Implement `AuthorAcceptanceTests(ctx, specName) error` method that: (1) loads spec content from `.gromit/specs/<name>.md`, (2) loads any existing spec-level tests, (3) renders spec acceptance prompt via renderer, (4) invokes provider at build tier, (5) commits resulting test files. Use the same provider invocation pattern as ATDD acceptance authoring in `methodology.Executor.RunAcceptanceTests`.

**Acceptance Criteria:**
- Spec content is loaded and passed to template rendering
- Provider is invoked with rendered prompt and result is applied
- Authoring is skipped if spec file doesn't exist (with warning)

**Dependencies:** Task 4

---

### Task 6: Implement spec acceptance gate verification

**Files:**
- Create: `internal/runner/spec_gate.go`
- Test: `internal/runner/spec_gate_test.go`

**What to Do:**
Create `SpecGate` struct with dependencies: validation runner, renderer, provider router, config. Implement `Verify(ctx, specName, specContent) (*GateResult, error)` that: (1) runs validation commands (reusing existing validation infrastructure), (2) if all pass returns `GateResult{Passed: true}`, (3) if failures exist, renders spec gate prompt with failure output, invokes provider to analyze, parses structured JSON response into `GateResult{Passed: false, Failures: []GateFailure}`. `GateFailure` has fields: TestName, Message, SuggestedFix.

**Acceptance Criteria:**
- Gate returns passed=true when all validation commands succeed
- Gate returns structured failures with test names and suggested fixes on failure
- Gate handles provider invocation errors gracefully (returns error, not panic)

**Dependencies:** Task 4

---

### Task 7: Wire spec orchestrator into runner.Run()

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/callbacks.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
Add `specOrchestrator *SpecOrchestrator` and `specGate *SpecGate` fields to `Runner`. In `NewRunner` (or runner construction), initialize them when `cfg.Methodology.Granularity == "spec"`. In `Run()`, when spec orchestrator is active and label filters are set: (1) before the bead loop, call `specOrchestrator.AuthorAcceptanceTests()` for each unique spec in the label set, (2) run the normal bead loop (ATDD already skipped per Task 3), (3) after the bead loop exhausts all ready beads, call `specGate.Verify()` for each spec. Wire orchestrator and gate construction in `makeMethodologyExec` or a new `makeSpecOrchestrator` helper in callbacks.go.

**Acceptance Criteria:**
- Spec acceptance authoring runs before bead execution when granularity=spec + scope set
- Spec gate runs after all scoped beads complete
- Non-scoped runs (no label filters) skip orchestrator entirely
- Backward compatible: granularity=bead runs identical to current behavior

**Dependencies:** Tasks 1, 2, 3, 5, 6

**Notes:** This is the central wiring task. Keep the orchestration logic in `Run()` thin — delegate to orchestrator and gate methods.

---

### Task 8: Implement fix bead synthesis from gate failures

**Files:**
- Create: `internal/runner/spec_fix.go`
- Test: `internal/runner/spec_fix_test.go`

**What to Do:**
Create `SynthesizeFixBeads(ctx, specName, failures []GateFailure, beadClient BeadClient) ([]string, error)` function that: for each `GateFailure`, creates a new bead via `beadClient.Create()` with title derived from the failure, description including the suggested fix, priority P0, and labels `["spec:<name>"]`. Returns created bead IDs. Apply a maximum of 5 fix beads per gate cycle to prevent runaway creation.

**Acceptance Criteria:**
- Each gate failure produces a bead with correct `spec:<name>` label
- Bead titles and descriptions reflect the specific failure
- Creation is capped at 5 beads per synthesis call

**Dependencies:** Task 6

---

### Task 9: Wire fix synthesis into orchestrator with retry policy

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/spec_orchestrator.go`
- Test: `internal/runner/spec_orchestrator_test.go`

**What to Do:**
Add `SpecGateMaxRetries int` config field to `MethodologyConfig` (default 3). In the spec orchestrator's post-batch flow in `Run()`: when gate fails, call `SynthesizeFixBeads()`, then re-enter the bead loop for the same spec scope. Track gate retry count. When max retries exhausted, log structured failure summary and stop the spec (don't block other specs in epic scope). Each retry cycle: gate → synthesize → beads → gate.

**Acceptance Criteria:**
- Gate failure triggers fix bead synthesis and re-enters bead loop
- Retry count is tracked and enforced (default max 3)
- Exhausted retries produce a structured failure report and stop cleanly

**Dependencies:** Tasks 7, 8

**Notes:** The retry loop wraps the existing bead loop. The bead loop will pick up newly created fix beads via `getNextBead()` since they have the matching spec label.

---

## Notes

- **Phase alignment**: Tasks 1 maps to spec Phase 1, Tasks 2–3 to Phase 2, Tasks 4–7 to Phase 3, Tasks 8–9 to Phase 4. Each phase is independently deployable.
- **Existing bead backlog**: There are ~80 open beads. This plan's tasks should be decomposed with `spec:spec-level-atdd-execution` labels to avoid cross-contamination.
- **Related specs**: `run-scope-flags` (Task 1 partially implements), `atdd-simplification` (Task 3 aligns with), `epic-scoped-execution` (Task 1 `--epic` flag).
- **Risk**: Task 7 (wiring) is the most complex integration point. The orchestrator's interaction with the existing bead loop needs careful testing to avoid regression.
- **Backward compatibility**: Every task preserves the default `granularity: bead` path unchanged. Non-scoped `gromit run` is never affected.
