---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T17:22:05Z"
id: spec-acceptance-verification-loop
source_spec: spec-acceptance-verification-loop
---

# Spec Acceptance Verification Loop Implementation Plan

**Goal:** After all beads for a spec close, run a hybrid acceptance gate (tests + LLM review) that creates targeted fix beads for failures, looping until the gate passes or a retry limit is exhausted.

**Architecture:** New `internal/specgate/` package with function-injected dependencies for testability. Runner auto-triggers the gate in `handleSuccessfulIteration()` when the last spec bead closes. Standalone `gromit verify-spec` CLI command for on-demand use.

**Tech Stack:** Go, cobra CLI, existing prompt/template system, provider/router for LLM invocation

**Spec:** `.gromit/specs/spec-acceptance-verification-loop.md`

---

## Architecture

**Overview:**
`internal/specgate/` encapsulates all gate logic with three files: verdict types, gate orchestration, and fix bead synthesis. All external dependencies (test runner, LLM invoker, prompt renderer, diff getter) are injected as function types, making every code path unit-testable with stubs.

**Key Components:**

1. **`internal/specgate/verdict.go`** — `CriterionResult` (Criterion, Passed, Evidence) and `GateVerdict` (Passed, Results) types with `ParseVerdict([]byte)` JSON parser and `FailedCriteria()` filter.

2. **`internal/specgate/gate.go`** — `Gate` struct accepting `RunTestsFn`, `InvokeLLMFn`, `RenderPromptFn`, `GetDiffFn`, `Model`, `MaxCycles`. `Run(ctx, specName, acceptanceCriteria)` returns `(*GateVerdict, error)`. Orchestration: run tests → get diff → render prompt → invoke LLM → parse verdict.

3. **`internal/specgate/synthesize.go`** — `BeadCreator` interface with `Create(title, priority, labels, description)` method. `SynthesizeFixBeads(ctx, specName, failures, priority, creator)` creates one bead per failure, capped at 5.

4. **`internal/config/config.go`** — `SpecGateConfig{Enabled *bool, MaxCycles int, Model string, AutoTrigger *bool}` on `Config`. Defaults: enabled=true, max_cycles=3, model="sonnet", auto_trigger=true.

5. **`internal/prompt/prompt.go`** — Extend `SpecGateContext` with `TestOutput`, `CumulativeDiff`, `AcceptanceCriteria` fields.

6. **`.gromit/templates/PROMPT_spec_gate.md`** — Enhanced to show all three context fields and instruct the LLM to emit structured JSON: `{"passed": bool, "results": [{"criterion": str, "passed": bool, "evidence": str}]}`.

7. **`internal/runner/run_iteration.go`** — `maybeRunSpecGate()` called from `handleSuccessfulIteration()` after bead sync. Checks: scoped run + auto_trigger enabled + no remaining open beads for spec label → runs gate → synthesizes fix beads on failure. Tracks cycles in `runLoopState.specGateCycles`.

8. **`cmd/gromit/verify_spec.go`** — `gromit verify-spec <name>` with `--create-beads` flag. Constructs Gate with real deps, runs once, prints per-criterion table, exits 0/1.

**Integration Points:**
- Runner hook point: `handleSuccessfulIteration()` line 240 (after `beads.Sync()`, before status write)
- Bead client: uses existing `ReadyWithLabel()`, `ListWithLabel()`, `CreateWithParentAndDescription()`
- Provider: `r.router.Select("spec_gate", provider.TierMedium)` for sonnet-tier invocation
- Prompt: existing `RenderSpecGate()` method with extended context
- Config: new `SpecGate` field on top-level `Config` struct

**Data Flow:**
1. Last spec bead closes → runner detects no remaining open beads for `spec:<name>` label
2. Gate runs acceptance tests scoped to touched packages → captures exit code + output
3. Gate collects cumulative diff via git
4. Gate loads spec acceptance criteria from `.gromit/specs/<name>.md`
5. Gate renders prompt with test output + diff + criteria → sends to sonnet LLM
6. LLM returns JSON verdict with per-criterion pass/fail + evidence
7. If failures → `SynthesizeFixBeads()` creates fix beads with `spec:<name>` label
8. Runner loop picks up fix beads naturally via `getNextBead()` label filter
9. After fix beads close, gate re-triggers (up to `max_cycles`)

**Tradeoffs:**
- Function injection over concrete dependencies: enables unit testing without complex mocks
- Cap at 5 fix beads per cycle: prevents runaway creation; remaining failures noted in logs
- Cycle tracking in runLoopState (not persistent): resets per `gromit run` invocation — fresh attempts on new runs
- Extending existing SpecGateContext rather than new type: minimizes interface changes

## Test Strategy

**Unit Tests:**
- `internal/specgate/verdict_test.go` — JSON parsing: valid mixed pass/fail, malformed JSON, missing fields, empty input
- `internal/specgate/gate_test.go` — Gate.Run() with stubs: all pass, some fail, test failure, LLM error, max cycles
- `internal/specgate/synthesize_test.go` — 0, 1, 5, 6+ failures, bead creator error, label/priority propagation
- `internal/config/config_test.go` — SpecGateConfig defaults, YAML deserialization, pointer bool behavior
- `internal/prompt/prompt_test.go` — extended SpecGateContext renders through template
- `cmd/gromit/verify_spec_test.go` — CLI arg parsing, output table, exit codes, --create-beads flag

**Integration Tests:**
- `internal/runner/run_iteration_test.go` — auto-trigger decision matrix: fires when last bead closes in scoped run, skips when beads remain, skips when not scoped, skips when disabled, respects max_cycles

**Mocking:**
- Gate deps: function stubs returning canned values
- BeadCreator: mock tracking Create() calls and returning fake IDs
- Runner: existing mockBeadClient and mockPromptRenderer patterns
- CLI: Gate wired with stubs, no real LLM/bead calls

**Coverage Goals:**
- Every branch in Gate.Run() (pass, fail, error, max cycles)
- Every branch in auto-trigger decision (scoped/not, beads remain/empty, enabled/disabled, cycles exhausted)
- Verdict JSON parsing edge cases
- Fix bead cap at 5

## Implementation Tasks

### Task 1: Add SpecGateConfig to Config with defaults

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add `SpecGateConfig` struct with `Enabled *bool` (yaml: enabled), `MaxCycles int` (yaml: max_cycles), `Model string` (yaml: model), `AutoTrigger *bool` (yaml: auto_trigger). Add `SpecGate SpecGateConfig` field to the top-level `Config` struct. In `SetDefaults()`, default MaxCycles to 3 when zero, Model to "sonnet" when empty, Enabled to true when nil, AutoTrigger to true when nil. Follow existing `PrecheckConfig` pattern for `*bool` fields with `IsEnabled()` and `IsAutoTrigger()` helper methods. Add commented `spec_gate:` section to `gromit.yaml`.

**Acceptance Criteria:**
- `SpecGateConfig` struct exists with all four fields and correct YAML tags
- `SetDefaults()` applies defaults for zero/nil values
- Config test verifies defaults and YAML override round-trip

**Dependencies:** None

### Task 2: Add CriterionResult and GateVerdict types with JSON parsing

**Files:**
- Create: `internal/specgate/verdict.go`
- Create: `internal/specgate/verdict_test.go`

**What to Do:**
Create the `internal/specgate/` package. Define `CriterionResult` struct with `Criterion string`, `Passed bool`, `Evidence string` (all with JSON tags). Define `GateVerdict` struct with `Passed bool` and `Results []CriterionResult`. Implement `ParseVerdict(data []byte) (*GateVerdict, error)` that unmarshals JSON. Implement `FailedCriteria() []CriterionResult` method on GateVerdict that filters to failed results.

**Acceptance Criteria:**
- `ParseVerdict` handles valid JSON with mixed pass/fail criteria
- `ParseVerdict` returns error for malformed JSON
- `FailedCriteria()` returns only results where `Passed == false`

**Dependencies:** None

### Task 3: Extend SpecGateContext and update spec gate template

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/prompt/prompt_test.go`
- Modify: `.gromit/templates/PROMPT_spec_gate.md`
- Modify: `internal/runner/interfaces.go` (if PromptRenderer signature changes)

**What to Do:**
Add `TestOutput string`, `CumulativeDiff string`, and `AcceptanceCriteria string` fields to `SpecGateContext` in prompt.go. Update `PROMPT_spec_gate.md` to display all fields and instruct the LLM to output structured JSON: `{"passed": bool, "results": [{"criterion": "...", "passed": bool, "evidence": "..."}]}`. The template should explain: compare test results + diff against acceptance criteria, emit one result per criterion, set `passed` to true only when all results pass.

**Acceptance Criteria:**
- `SpecGateContext` has `TestOutput`, `CumulativeDiff`, and `AcceptanceCriteria` fields
- Template renders cleanly with all fields populated and with optional fields empty
- Template instructs LLM to emit JSON matching `GateVerdict` structure

**Dependencies:** None

### Task 4: Implement Gate orchestration in specgate package

**Files:**
- Create: `internal/specgate/gate.go`
- Create: `internal/specgate/gate_test.go`

**What to Do:**
Define function types: `RunTestsFn func(ctx context.Context, packages []string) (output string, passed bool, err error)`, `InvokeLLMFn func(ctx context.Context, prompt string) (string, error)`, `RenderPromptFn func(ctx *SpecGatePromptInput) (string, error)`, `GetDiffFn func() (string, error)`. Define `SpecGatePromptInput` with TestOutput, CumulativeDiff, AcceptanceCriteria fields (or reuse prompt.SpecGateContext). Define `Gate` struct with these function fields plus `Model string` and `MaxCycles int`. Implement `Run(ctx context.Context, specName string, acceptanceCriteria string) (*GateVerdict, error)`: run tests via RunTestsFn, get diff via GetDiffFn, render prompt via RenderPromptFn, invoke LLM via InvokeLLMFn, parse verdict via ParseVerdict. Return the verdict. Gate.Run executes a single cycle — the caller (runner or CLI) manages retry looping.

**Acceptance Criteria:**
- `Gate.Run()` executes the full pipeline: tests → diff → render → invoke → parse
- Returns passing verdict when all criteria pass
- Returns failing verdict with failed criteria when some fail
- Returns error when LLM invocation fails or JSON parsing fails
- Test failure (passed=false from RunTestsFn) is reflected in the rendered prompt

**Dependencies:** Task 2, Task 3

**Notes:** Gate.Run() is a single-cycle operation. The runner handles the retry loop with max_cycles tracking. This keeps the Gate pure and testable.

### Task 5: Implement SynthesizeFixBeads in specgate package

**Files:**
- Create: `internal/specgate/synthesize.go`
- Create: `internal/specgate/synthesize_test.go`

**What to Do:**
Define `BeadCreator` interface with `Create(title, priority, labels string, description string) (string, error)` method. Implement `SynthesizeFixBeads(ctx context.Context, specName string, failures []CriterionResult, priority string, creator BeadCreator) ([]string, error)`. For each failure (up to 5): title = `"Fix: <criterion summary>"` (truncate criterion to 80 chars), labels = `"spec:<specName>"`, description = criterion text + evidence. Return slice of created bead IDs. If >5 failures, create 5 and log a warning about remaining.

**Acceptance Criteria:**
- Creates exactly one bead per failing criterion
- Caps at 5 beads, logs warning for remainder
- Beads have correct `spec:<name>` label and inherited priority
- Returns created bead IDs
- Handles bead creator errors gracefully (continues with remaining, returns partial results + error)

**Dependencies:** Task 2

### Task 6: Wire spec gate auto-trigger into runner

**Files:**
- Modify: `internal/runner/run_iteration.go`
- Modify: `internal/runner/run_init.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/run_iteration_test.go`

**What to Do:**
Add `specGateCycles map[string]int` to `runLoopState` in run_init.go (initialize in `initRunLoopState`). Add `specGate *specgate.Gate` field to `Runner` in runner.go (constructed in NewRunner when SpecGate config is enabled). Add `maybeRunSpecGate(ctx, b, st)` method to Runner in run_iteration.go. In `handleSuccessfulIteration()`, call `maybeRunSpecGate` after `beads.Sync()` (line 240) and before status write (line 242). Logic: if `cfg.SpecGate` not enabled or AutoTrigger not enabled → return. Extract spec label from bead via `bead.FindSpecLabel(b.Labels)` → if empty, return. Check `r.beads.ReadyWithLabel("spec:"+specName)` → if bead found, return (more work remains). Check `st.specGateCycles[specName] >= cfg.SpecGate.MaxCycles` → log exhaustion and return. Run `r.specGate.Run(ctx, specName, acceptanceCriteria)`. If verdict passes → log success, return. If verdict fails → call `SynthesizeFixBeads` → increment `st.specGateCycles[specName]`. Fix beads get picked up by subsequent `getNextBead()` iterations naturally.

**Acceptance Criteria:**
- Gate fires when last spec bead closes in a scoped run with auto_trigger enabled
- Gate does not fire when open beads remain for the spec label
- Gate does not fire when auto_trigger is disabled or SpecGate is disabled
- Gate does not fire when not in a scoped run (no spec label on bead)
- Cycle count increments per spec and stops at max_cycles
- Fix beads are created and picked up by subsequent loop iterations

**Dependencies:** Task 1, Task 4, Task 5

**Notes:** The acceptance criteria loading (reading spec file) should use `os.ReadFile` on `.gromit/specs/<specName>.md`. The test packages for `RunTestsFn` come from `r.touchedPackages`.

### Task 7: Add gromit verify-spec CLI command

**Files:**
- Create: `cmd/gromit/verify_spec.go`
- Create: `cmd/gromit/verify_spec_test.go`

**What to Do:**
Create `verifySpecCmd` cobra command registered in `init()`. Takes one positional arg: spec name. Flags: `--create-beads` (bool, default false). Implementation: load config, load spec file from `.gromit/specs/<name>.md`, construct `specgate.Gate` with real dependencies (provider router for LLM, validation runner for tests, prompt renderer, git diff helper), call `Gate.Run()` once, print per-criterion verdict table (criterion | status | evidence), exit 0 if passed / exit 1 if failed. When `--create-beads` is set and gate fails, call `SynthesizeFixBeads()` and print created bead IDs.

**Acceptance Criteria:**
- `gromit verify-spec <name>` runs gate once and prints verdict table
- Exit code 0 on pass, 1 on fail
- `--create-beads` creates fix beads when gate fails
- Error message when spec file not found

**Dependencies:** Task 4, Task 5

**Notes:** Wire real dependencies following patterns from other CLI commands (e.g., review.go, decompose.go). Use `config.Load()` for config, `provider.NewRouter()` for routing, `prompt.NewRenderer()` for templates.

---

## Notes

- Several beads already exist for pieces of this work (gromit-gtrb, gromit-5k3v, gromit-h94d, gromit-uwag, gromit-tv7a, gromit-tt7a, gromit-qrvz). During decompose, these should be checked — existing beads may cover individual tasks and can be reused or updated rather than creating duplicates.
- This plan implements phases 3 and 4 of the parent `spec-level-atdd-execution` plan. Prerequisites from that plan (--spec flag on run command, Granularity config field, skip per-bead ATDD in spec mode) are tracked in separate beads and not duplicated here.
- The Gate.Run() is deliberately single-cycle. The runner manages the retry loop. This separation keeps the gate testable and the runner in control of lifecycle decisions.
- The 5-bead cap per synthesis cycle prevents runaway creation. With max_cycles=3, the theoretical maximum is 15 fix beads before the system stops — a reasonable upper bound.
