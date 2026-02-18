---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T18:21:40Z"
id: spec-level-atdd-gates
source_spec: spec-level-atdd-gates
---

# Spec-Level ATDD Gates Implementation Plan

**Goal:** Add a `methodology.granularity: "spec"` mode that shifts ATDD from per-bead to per-spec, authoring acceptance tests once before the first spec bead, suppressing per-bead ATDD, and running a spec gate after the last bead closes with fix-bead synthesis on failure.

**Architecture:** Function-type-dependency Gate in a new `internal/specgate/` package, config-driven granularity switch, runner-level orchestration with in-memory spec tracking, and a standalone `verify-spec` CLI command.

**Tech Stack:** Go, Cobra CLI, JSON parsing, Go templates

**Spec:** `.gromit/specs/spec-level-atdd-gates.md`

---

## Architecture

**Key Components:**

1. **`internal/specgate/` package (NEW)** — Self-contained spec gate logic:
   - `verdict.go` — `CriterionResult` and `GateVerdict` types with JSON parsing
   - `gate.go` — `Gate` struct with function-type deps (`RunTestsFn`, `InvokeLLMFn`, `RenderPromptFn`, `GetDiffFn`) and `Run()` method
   - `synthesize.go` — `BeadCreator` interface and `SynthesizeFixBeads()` function

2. **Config layer** (`internal/config/config.go`):
   - `Granularity string` on `MethodologyConfig` (default `"bead"`, validates `"bead"` or `"spec"`)
   - `SpecGateConfig` struct with `Enabled`, `MaxCycles`, `Model`, `AutoTrigger`
   - `SpecGate SpecGateConfig` on top-level `Config`

3. **Runner integration** (`internal/runner/`):
   - `spec_orchestrator.go` — `SpecOrchestrator` with `AuthorAcceptanceTests(ctx, specName)`
   - Modify `run_iteration.go` with `maybeRunSpecGate()` in `handleSuccessfulIteration()`
   - Update `MethodologyPolicy.IsActive()` for spec-aware ATDD suppression
   - Track spec authoring state and gate cycle counts in `runLoopState`

4. **CLI** (`cmd/gromit/`):
   - `verify_spec.go` — `gromit verify-spec <name>` with `--create-beads` flag
   - `--spec <name>` flag on `gromit run` for scoped execution

**Integration Points:**
- Config → Runner: granularity and SpecGateConfig read during NewRunner
- Runner → specgate: Gate.Run() called from maybeRunSpecGate() after last spec bead closes
- Runner → SpecOrchestrator: AuthorAcceptanceTests() called before first spec bead
- specgate → beads: SynthesizeFixBeads() creates fix beads via BeadClient
- CLI → specgate: verify-spec constructs Gate with real deps, runs standalone

**Data Flow:**
```
Run() → getNextBead() → bead has spec:foo label
  → First bead for spec:foo? → specOrchestrator.AuthorAcceptanceTests("foo")
  → processBead() with atddActive=false (suppressed by granularity)
  → handleSuccessfulIteration() → last bead for spec:foo?
    → maybeRunSpecGate("foo") → Gate.Run()
      → Run acceptance tests → Get cumulative diff → Render prompt → LLM
      → Parse GateVerdict → PASS: done / FAIL: SynthesizeFixBeads() → loop
```

## Test Strategy

**Unit Tests (per package, table-driven):**
- `internal/specgate/verdict_test.go` — ParseVerdict valid/invalid/malformed, FailedCriteria
- `internal/specgate/gate_test.go` — Gate.Run() with stubs: all-pass, some-fail, test error, LLM error, max-cycles
- `internal/specgate/synthesize_test.go` — 0/1/5/6+ failures, creator errors, label propagation, title truncation
- `internal/config/config_test.go` — Granularity defaults/validation, SpecGateConfig YAML round-trip
- `internal/runner/policy/methodology_test.go` — Spec-aware suppression in spec mode, passthrough in bead mode

**Integration Tests (runner-level with mocks):**
- `internal/runner/run_iteration_test.go` — maybeRunSpecGate decision matrix
- `internal/runner/spec_orchestrator_test.go` — Authoring, idempotent skip, file-not-found
- `cmd/gromit/verify_spec_test.go` — Arg parsing, output, exit codes

**Mocking:**
- Gate uses function-type deps — inline stubs in tests
- BeadCreator interface — mock for synthesis tests
- Runner tests use existing mock patterns (mockBeadClient, mockPromptRenderer, mock router)

**Coverage Goals:**
- Every Gate.Run() branch, every maybeRunSpecGate() branch
- Config validation rejects invalid granularity
- Fix bead cap enforced, backward compat with `granularity: "bead"`

---

## Implementation Tasks

### Task 1: Add Granularity config field to MethodologyConfig

**Files:**
- Modify: `internal/config/config.go`
- Modify: `gromit.yaml`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `Granularity string` field with YAML tag `granularity` to `MethodologyConfig`. In `SetDefaults()`, default to `"bead"` when empty. In `Validate()`, reject values other than `"bead"` and `"spec"`. Add commented `granularity` field to `gromit.yaml` methodology section.

**Acceptance Criteria:**
- `Granularity` defaults to `"bead"` when unset
- `Validate()` rejects `"invalid"` but accepts `"bead"` and `"spec"`
- YAML deserialization round-trips correctly

**Dependencies:** None

**Existing beads:** gromit-b56j

---

### Task 2: Add SpecGateConfig to Config with defaults

**Files:**
- Modify: `internal/config/config.go`
- Modify: `gromit.yaml`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `SpecGateConfig` struct with `Enabled bool`, `MaxCycles int`, `Model string`, `AutoTrigger *bool` fields (YAML tags: `enabled`, `max_cycles`, `model`, `auto_trigger`). Add `SpecGate SpecGateConfig` to top-level `Config`. In `SetDefaults()`: default MaxCycles to 3 when zero, Model to `"sonnet"` when empty, AutoTrigger to true when nil. Add `IsEnabled()` and `IsAutoTrigger()` helper methods. Add commented `spec_gate:` section to `gromit.yaml`.

**Acceptance Criteria:**
- Defaults applied correctly (MaxCycles=3, Model="sonnet", AutoTrigger=true)
- YAML deserialization populates all fields
- Helper methods return correct values

**Dependencies:** None

**Existing beads:** gromit-lem3h (preferred), gromit-gtrb (duplicate — close as superseded)

---

### Task 3: Add CriterionResult and GateVerdict types with JSON parsing

**Files:**
- Create: `internal/specgate/verdict.go`
- Create: `internal/specgate/verdict_test.go`

**What to Do:**
Create `internal/specgate/` package. Define `CriterionResult` struct with `Criterion string`, `Passed bool`, `Evidence string` (JSON tags). Define `GateVerdict` struct with `Passed bool`, `Results []CriterionResult`. Implement `ParseVerdict([]byte) (*GateVerdict, error)` and `FailedCriteria() []CriterionResult` method.

**Acceptance Criteria:**
- ParseVerdict handles valid mixed pass/fail JSON, malformed JSON, missing fields, empty input
- FailedCriteria returns only failed results
- Compile-time interface checks where applicable

**Dependencies:** None

**Existing beads:** gromit-8jo0i (preferred), gromit-5k3v (duplicate — close as superseded)

---

### Task 4: Implement Gate orchestration in specgate package

**Files:**
- Create: `internal/specgate/gate.go`
- Create: `internal/specgate/gate_test.go`

**What to Do:**
Define function types `RunTestsFn`, `InvokeLLMFn`, `RenderPromptFn`, `GetDiffFn`. Create `Gate` struct with these plus `Model string` and `MaxCycles int`. Implement `Gate.Run(ctx, specName, acceptanceCriteria) (*GateVerdict, error)` that: runs tests, gets diff, renders prompt, invokes LLM, parses verdict. Single cycle — caller manages retry loop.

**Acceptance Criteria:**
- All-pass verdict returns Passed=true
- Some-fail verdict returns Passed=false with correct failures
- Test failure returns error
- LLM invocation error returns error

**Dependencies:** Task 3 (verdict types)

**Existing beads:** gromit-h94d (preferred), gromit-43638 (duplicate — close as superseded)

---

### Task 5: Implement SynthesizeFixBeads in specgate package

**Files:**
- Create: `internal/specgate/synthesize.go`
- Create: `internal/specgate/synthesize_test.go`

**What to Do:**
Define `BeadCreator` interface with `Create(ctx, title, description, priority string, labels []string) (string, error)`. Implement `SynthesizeFixBeads(ctx, specName string, failures []CriterionResult, priority string, creator BeadCreator) ([]string, error)` that creates one bead per failure, capped at 5. Title derived from criterion (truncated to 80 chars), description includes criterion + evidence, labels include `spec:<specName>`.

**Acceptance Criteria:**
- 0 failures produces 0 beads
- 5 failures produces 5 beads with correct labels/priority
- 6+ failures capped at 5
- Creator errors propagated

**Dependencies:** Task 3 (verdict types)

**Existing beads:** gromit-qsn1k (preferred), gromit-uwag (duplicate — close as superseded), gromit-1d0u (related runner-level wiring)

---

### Task 6: Enhance PROMPT_spec_gate.md template

**Files:**
- Modify: `.gromit/templates/PROMPT_spec_gate.md`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Enhance the template with: labeled sections for TestOutput, CumulativeDiff, and AcceptanceCriteria; explicit GateVerdict JSON schema mandate with field-level required structure rules; example output showing the expected JSON format. Verify rendering works with SpecGateContext fields populated and empty.

**Acceptance Criteria:**
- Template renders cleanly with all SpecGateContext fields populated
- Template renders cleanly with optional fields empty (zero values)
- GateVerdict contract is explicitly named and structurally required

**Dependencies:** None (SpecGateContext already exists)

**Existing beads:** gromit-tv7a, gromit-lrbgk series (consolidate — close superseded sub-beads)

---

### Task 7: Update MethodologyPolicy for spec-aware ATDD suppression

**Files:**
- Modify: `internal/runner/policy/methodology.go`
- Test: `internal/runner/policy/methodology_test.go`

**What to Do:**
Update `ConfigMethodologyPolicy.IsActive()` so that when `cfg.Methodology.Granularity == "spec"` and the methodology is `"atdd"` and the bead has a `spec:` label, it returns `false`. Beads without `spec:` labels and TDD are unaffected. Need to pass granularity info through — either via config reference already on the policy, or by extending the method signature.

**Acceptance Criteria:**
- `IsActive("atdd", labels=["spec:foo"])` returns false when granularity is "spec"
- `IsActive("atdd", labels=["spec:foo"])` returns true when granularity is "bead"
- `IsActive("atdd", labels=["other"])` returns true regardless of granularity
- `IsActive("tdd", ...)` unaffected by granularity

**Dependencies:** Task 1 (Granularity config field)

---

### Task 8: Skip per-bead ATDD phases in spec granularity mode

**Files:**
- Modify: `internal/runner/runner.go` (or `process_methodology.go`)
- Test: existing runner test files

**What to Do:**
In `processBead()`, before the ATDD phase block, check if granularity is `"spec"` and the bead has a `spec:` label. If so, set `atddActive = false`, skipping all per-bead ATDD. Log when ATDD is suppressed. TDD remains independently controlled.

**Acceptance Criteria:**
- Per-bead ATDD suppressed for spec-labeled beads in spec granularity
- Per-bead ATDD active for non-spec-labeled beads regardless of granularity
- TDD unaffected
- Suppression logged

**Dependencies:** Task 7 (MethodologyPolicy update)

**Existing beads:** gromit-titn

---

### Task 9: Implement spec acceptance authoring phase

**Files:**
- Create: `internal/runner/spec_orchestrator.go`
- Create: `internal/runner/spec_orchestrator_test.go`

**What to Do:**
Create `SpecOrchestrator` struct with dependencies: prompt renderer, provider router, bead client, config. Implement `AuthorAcceptanceTests(ctx, specName) error` that: loads `.gromit/specs/<name>.md`, renders spec acceptance prompt (using existing `PROMPT_acceptance_tests.md` or spec-scoped variant), invokes provider at build tier, and commits test files. Track authored specs in-memory to skip subsequent calls for the same spec.

**Acceptance Criteria:**
- Loads spec content and invokes provider
- Second call for same spec is a no-op (idempotent)
- Missing spec file returns descriptive error

**Dependencies:** Task 1 (Granularity config), Task 2 (SpecGateConfig)

**Existing beads:** gromit-0zuz

---

### Task 10: Wire spec gate auto-trigger into runner

**Files:**
- Modify: `internal/runner/run_iteration.go`
- Modify: `internal/runner/run_init.go`
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/run_iteration_test.go`

**What to Do:**
Add `specGateCycles map[string]int` to `runLoopState` in `run_init.go`. Add `specGate *specgate.Gate` field to Runner, constructed in `NewRunner` when SpecGate is enabled. Add `maybeRunSpecGate(ctx, specName)` method called from `handleSuccessfulIteration()` after `beads.Sync()`. Logic: check enabled + auto_trigger, extract spec label from bead, query remaining open beads for spec, check cycle limit, run gate, synthesize fix beads on failure, increment cycle counter.

**Acceptance Criteria:**
- Gate not triggered when `auto_trigger: false`
- Gate not triggered when bead is not the last for its spec
- Gate triggers on last bead, passes → no fix beads
- Gate triggers on last bead, fails → fix beads created, cycle incremented
- Gate not triggered when `max_cycles` reached

**Dependencies:** Task 2 (SpecGateConfig), Task 4 (Gate), Task 5 (SynthesizeFixBeads), Task 8 (ATDD suppression)

**Existing beads:** gromit-tt7a (preferred), gromit-y9p0g (duplicate — close as superseded)

---

### Task 11: Wire spec orchestrator into runner.Run()

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/callbacks.go` (if needed for construction)
- Test: `internal/runner/runner_test.go`

**What to Do:**
Add `specOrchestrator *SpecOrchestrator` field to Runner. In `NewRunner`, initialize when `cfg.Methodology.Granularity == "spec"`. In the bead processing loop, before `processBead()`, check if orchestrator is active and this is the first bead for its spec — call `specOrchestrator.AuthorAcceptanceTests()`. Track "tests authored" per spec in `runLoopState`.

**Acceptance Criteria:**
- AuthorAcceptanceTests called before first bead of each spec
- Not called for second+ beads of same spec
- Not called when granularity is "bead"

**Dependencies:** Task 9 (SpecOrchestrator)

**Existing beads:** gromit-di6n

---

### Task 12: Add gromit verify-spec CLI command

**Files:**
- Create: `cmd/gromit/verify_spec.go`
- Create: `cmd/gromit/verify_spec_test.go`

**What to Do:**
Create `verifySpecCmd` cobra command registered in `init()`. Takes one positional arg (spec name), flag `--create-beads` (bool, default false). Load config, read spec file from `.gromit/specs/<name>.md`, construct Gate with real deps (provider router, validation runner, prompt renderer, git diff helper), run `Gate.Run()` once, print per-criterion verdict table (criterion | status | evidence), exit 0 on pass / 1 on failure. When `--create-beads` and gate fails, call `SynthesizeFixBeads()`.

**Acceptance Criteria:**
- Arg parsing validates spec name provided
- Pass verdict exits 0 with table output
- Fail verdict exits 1 with table output
- `--create-beads` creates fix beads on failure

**Dependencies:** Task 4 (Gate), Task 5 (SynthesizeFixBeads)

**Existing beads:** gromit-qrvz (preferred), gromit-8ku7u (duplicate — close as superseded)

---

### Task 13: Add --spec flag to gromit run

**Files:**
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/main_test.go` or relevant test file

**What to Do:**
Add `--spec` string flag to the run command. In `runLoop()`, when set: validate the spec file exists at `.gromit/specs/<name>.md`, resolve to label filter `spec:<name>`, call `r.SetLabelFilters()` before `r.Run()`. This scopes bead selection to the named spec.

**Acceptance Criteria:**
- `--spec foo` filters beads to `spec:foo` label
- Missing spec file produces error before run starts
- Without `--spec`, all beads processed as before

**Dependencies:** Task 10 (runner wiring, to be useful end-to-end)

**Existing beads:** gromit-su87 (covers both --spec and --epic)

---

### Task 14: Final verification

**Files:** None (verification only)

**What to Do:**
Run `go test ./...`, `go vet ./...`, `go build ./...`. Verify spec-granularity mode end-to-end: ATDD suppressed for spec beads, gate fires after last bead, fix beads created on failure, cycle limit respected. Verify `gromit verify-spec` works standalone. Verify backward compat: `granularity: "bead"` changes nothing.

**Acceptance Criteria:**
- All tests pass
- Build succeeds
- No regressions in existing bead-granularity behavior

**Dependencies:** All previous tasks

---

## Notes

**Duplicate bead cleanup:** Several tasks have duplicate beads from earlier decomposition passes. The preferred bead for each task is noted above. Duplicates should be closed as superseded during decompose.

**Beads to close as superseded:** gromit-gtrb, gromit-5k3v, gromit-43638, gromit-uwag, gromit-y9p0g, gromit-8ku7u, and the gromit-lrbgk sub-beads that are covered by Task 6.

**In-memory state:** Per the spec, "tests authored" tracking and gate cycle counts reset each run. No persistent state changes needed — `runLoopState` maps suffice.

**Existing SpecGateContext:** The `SpecGateContext` type and `RenderSpecGate()` method already exist in `internal/prompt/prompt.go` with `TestOutput`, `CumulativeDiff`, and `AcceptanceCriteria` fields. No changes needed there.

**Template already started:** `PROMPT_spec_gate.md` exists with basic structure. Task 6 enhances it rather than creating from scratch.
