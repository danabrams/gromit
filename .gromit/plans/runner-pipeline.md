---
id: runner-pipeline
source_spec: runner-pipeline
created: 2026-02-21
decomposed: true
decomposed_at: "2026-02-23T01:34:11Z"
---

# Runner Pipeline Refactor Implementation Plan

**Goal:** Rebuild `internal/runner/` as a 5-stage pipeline in `internal/pipeline/<stage>/`, eliminating the God Object `Runner` struct by giving each stage a hard package boundary enforced by import cycles.

**Architecture:** Each stage (`prepare/`, `execute/`, `validate/`, `review/`, `epilogue/`) implements a `Stage` interface (`Run(ctx, Input) (Output, error)`). The orchestrator in `internal/runner/` holds no logic beyond sequencing stages. State flows through `Input`/`Output` structs, not through a shared struct.

**Tech Stack:** Go, `internal/pipeline/` (existing package), `internal/runner/` sub-packages, `bead`, `config`, `provider`, `runtypes`

**Spec:** `.gromit/specs/runner-pipeline.md`

---

## Architecture

The `internal/pipeline/` package already exists with `stage.go` (empty), `pipeline.go` (CLI workflows — untouched), and 5 stage sub-packages with placeholder `doc.go` files. The implementation populates these placeholders.

**Package layout after migration:**
```
internal/pipeline/
    stage.go          ← Stage interface, Input, Output, Decision types
    pipeline.go       ← existing CLI-workflow Pipeline (unchanged)
    prepare/          ← Gate stage
    execute/          ← Build stage
    validate/         ← Validate stage
    review/           ← Review stage (optional)
    epilogue/         ← Epilogue stage

internal/runner/
    orchestrator.go   ← loop sequencer only
    constructor.go    ← wires 5 stages from config
    acceptance/       ← behavioral tests (moved from runner package)
```

**Structural constraint:** `internal/runner/` imports `internal/pipeline/` only. No stage sub-package imports `internal/runner/`. The compiler enforces this — an import cycle is the mechanical guarantee.

**Coexistence:** `pipeline.go` (CLI Pipeline struct) and `stage.go` (Stage interface) live in the same `pipeline` package without conflict. The stage sub-packages (`prepare/`, `execute/`, etc.) are separate packages.

**Existing sub-packages as stage dependencies:** `escalation/`, `execution/`, `methodology/`, `validation/`, `runtypes/`, `policy/`, `reviewpkg/`, `tdd/` become dependencies of stage packages directly. No stage needs to go through `runner/` to reach them.

## Test Strategy

- **Stage unit tests**: Each stage has its own `*_test.go` with local fakes for its dependencies. No stage test imports `internal/runner/`. Run in default `go test ./...` lane.
- **Acceptance tests** (`internal/runner/acceptance/`): `//go:build acceptance` behavioral tests moved from `internal/runner/`. Test the assembled pipeline through the public `Runner` API. These are the migration gate.
- **Compile verification**: `go build ./...` enforces the import cycle constraint mechanically.
- **Acceptance gate command**: `go test -tags acceptance ./internal/runner/acceptance/...`

---

## Implementation Tasks

### Task 1: Stage interface and shared types

**Files:**
- Modify: `internal/pipeline/stage.go`
- Create: `internal/pipeline/stage_test.go`

**What to Do:**
Add to `internal/pipeline/stage.go`:
- `Stage` interface: `Run(ctx context.Context, in Input) (Output, error)`
- `Decision` type (string or int) with constants `Proceed`, `Skip`, `Block`
- `Input` struct: `Bead *bead.Bead`, `Config *config.Config`, `Iteration int`, `Deadline time.Time`, `ValidationFailures []string`, plus any accumulated context fields from prior stages
- `Output` struct: `Decision Decision` plus stage-specific result fields (build output, validation summaries, review bead IDs, etc.)

The `Output` struct carries a superset of what all stages need to return. Unused fields default to zero values. This avoids per-stage output types while keeping the `Stage` interface uniform.

Add `stage_test.go` with compile-time checks (`var _ Stage = ...`) and basic Decision constant tests.

**Acceptance Criteria:**
- `internal/pipeline/stage.go` compiles with `Stage` interface, `Decision` type, `Input` and `Output` structs
- `Decision` has exactly three constants: `Proceed`, `Skip`, `Block`
- `Input` contains at minimum: `Bead`, `Config`, `Iteration`, `Deadline`, `ValidationFailures []string`
- `go test ./internal/pipeline/...` passes

**Dependencies:** None — this is the foundation.

**Notes:** Keep imports minimal in `stage.go` — only `context`, `time`, and the packages needed for `Input` fields (`bead`, `config`). Avoid importing `runner/` or any stage sub-package.

---

### Task 2: Extract acceptance tests to internal/runner/acceptance/

**Files:**
- Create: `internal/runner/acceptance/` package (new directory)
- Create: `internal/runner/acceptance/helpers_test.go` (fake BeadClient, fake Provider, test helpers)
- Move + adapt: the 12 `//go:build acceptance` files from `internal/runner/` into `internal/runner/acceptance/`

**What to Do:**
1. Create `internal/runner/acceptance/` directory.
2. Identify the 12 `//go:build acceptance` files in `internal/runner/` (e.g., `loop_acceptance_test.go`, `runner_pipeline_acceptance_test.go`, `worktree_merge_acceptance_test.go`, etc.).
3. Move each file into `internal/runner/acceptance/`.
4. Change the package declaration from `package runner` to `package acceptance_test` (external test package).
5. Update imports: add `github.com/danabrams/gromit/internal/runner`; replace bare references to internal runner types with qualified names (e.g., `runner.NewRunner`, `runner.NewRunnerWithDeps`).
6. Create `internal/runner/acceptance/helpers_test.go` with `package acceptance_test` containing the fake implementations previously defined in runner's `*_test.go` files (mockBeadClient, mockClaudeClient, mockProvider, etc.) that the moved acceptance tests depend on.
7. Verify `go test -tags acceptance ./internal/runner/acceptance/...` passes against the **existing** runner (behavioral parity established before migration).

**Acceptance Criteria:**
- `internal/runner/acceptance/` exists as a package with `//go:build acceptance` files
- `go test -tags acceptance ./internal/runner/acceptance/...` passes against the existing runner implementation
- No acceptance test file remains in `internal/runner/` (they are all in `acceptance/`)
- The moved files compile as `package acceptance_test`

**Dependencies:** Task 1 (stage types available, though acceptance tests may not use them yet).

**Notes:** The acceptance tests use `mockBeadClient`, `mockClaudeClient`, etc. defined in `runner`'s `_test.go` files. These are test helpers — recreate them in `helpers_test.go` in the acceptance package. This is the behavioral contract. These tests must be green before deleting the old runner tests.

---

### Task 3: Delete non-acceptance runner test files

**Files:**
- Delete: all `internal/runner/*_test.go` files **except** those already moved to `acceptance/`

**What to Do:**
After Task 2 confirms acceptance tests pass from their new location, delete the ~160 non-acceptance `*_test.go` files from `internal/runner/`. This removes the implementation-coupled tests that would anchor the refactor to the old structure.

Run `go build ./...` to confirm nothing breaks (production code is unaffected by deleting test files).

**Acceptance Criteria:**
- `internal/runner/` contains no `*_test.go` files except those in `acceptance/`
- `go build ./...` passes
- `go test -tags acceptance ./internal/runner/acceptance/...` still passes

**Dependencies:** Task 2 (acceptance tests must be established and green before deletion).

---

### Task 4: Gate stage (prepare)

**Files:**
- Create: `internal/pipeline/prepare/gate.go`
- Create: `internal/pipeline/prepare/gate_test.go`

**What to Do:**
Implement `Gate` struct in `package prepare` that satisfies the `pipeline.Stage` interface.

`Gate.Run(ctx, Input)` executes in order:
1. **Precheck**: checks if bead's work is already done. Returns `Output{Decision: Skip}` if so.
2. **Stuck detection**: checks failure count against threshold. Returns `Output{Decision: Block}` if exceeded.
3. **Scope gate**: estimates file count from bead description. Returns `Output{Decision: Block}` (scope-blocked) if too large.
4. **Proactive decomposition**: if scope estimate triggers decomposition, creates sub-beads via BeadClient and returns `Output{Decision: Skip}`.
5. Returns `Output{Decision: Proceed}` when all checks pass.

Define `Gate`'s dependency interfaces locally in `prepare` (BeadClient subset, PromptRenderer subset, etc.). No imports from `internal/runner/`. Extract logic from `internal/runner/gates.go`, `internal/runner/codex_preflight.go`, and the gate-related sections of `internal/runner/process.go`.

`gate_test.go`: table-driven tests for each decision path using local fakes.

**Acceptance Criteria:**
- `Gate` implements `pipeline.Stage`
- `go test ./internal/pipeline/prepare/...` passes with tests for Proceed, Skip (precheck), Block (stuck), Block (scope), Skip (decomposed)
- No import of `internal/runner/` in any `prepare` file
- `go build ./...` passes

**Dependencies:** Task 1 (Stage interface).

**Notes:** The scope gate and precheck logic currently live in `runner/gates.go` and `runner/process.go`. Extract the logic, don't call into runner. The `prepare` package defines its own narrow interfaces for what it needs from bead client and renderer.

---

### Task 5: Build stage (execute)

**Files:**
- Create: `internal/pipeline/execute/build.go`
- Create: `internal/pipeline/execute/build_test.go`

**What to Do:**
Implement `Build` struct in `package execute` that satisfies `pipeline.Stage`.

`Build.Run(ctx, Input)`:
1. Selects methodology (TDD, refactor, standard) using `ShouldRunPostSuccess` policy and bead labels.
2. Constructs build prompt with bead context + `Input.ValidationFailures` injected.
3. Invokes provider via `StreamRun` (not `Run`).
4. If invocation fails and `Input.EscalationEnabled` is true, escalates `haiku→sonnet→opus` internally. Orchestrator only sees success or final failure.
5. Returns `Output` with build result, model used, provider used, cost/token fields.

**`ShouldRunPostSuccess(bead, config) bool`** — method on Build (or a policy type in execute) that determines whether a post-success methodology phase should run. Named exactly `ShouldRunPostSuccess`.

**Explicit escalation flag**: `Input` carries `EscalationEnabled bool`. When false, the first tier failure is final (used in validation callback path to avoid config mutation). This replaces the save-and-restore mutation of `r.cfg.Escalation.Enabled`.

`build_test.go`: tests for methodology selection, StreamRun invocation, escalation with/without flag, ValidationFailures injection into prompt.

**Acceptance Criteria:**
- `Build` implements `pipeline.Stage`
- `StreamRun` is used (not `Run`) for all Claude invocations — verified by fake that panics on `Run`
- `ShouldRunPostSuccess` method exists with correct signature
- `Input.EscalationEnabled=false` stops at first failure without escalating
- `go test ./internal/pipeline/execute/...` passes
- No import of `internal/runner/` in any `execute` file

**Dependencies:** Task 1 (Stage interface).

**Notes:** Escalation logic currently lives in `runner/escalation/handler.go`. The `execute` package calls into `runner/escalation` as a dependency — that's fine since `escalation/` is a sub-package of `runner/` by path but is an independent package (no circular import). Verify with `go build`.

---

### Task 6: Validate stage

**Files:**
- Create: `internal/pipeline/validate/validate.go`
- Create: `internal/pipeline/validate/validate_test.go`

**What to Do:**
Implement `Validate` struct in `package validate` satisfying `pipeline.Stage`.

`Validate.Run(ctx, Input)`:
1. Runs fast validation commands (`go test`, `golangci-lint`).
2. Applies auto-fix (`gofmt`, `goimports`) on changed files. If auto-fix resolves all failures, returns `Output{Decision: Proceed}`.
3. On remaining failure, returns `Output{Decision: Block, ValidationSummaries: [...]}`  with structured failure summaries. Orchestrator passes summaries into next Build's `Input.ValidationFailures`.
4. Handles periodic full validation (runs full suite at configured frequency, not every iteration).
5. Enforces mandatory command prefix policy.

`validate_test.go`: tests for clean pass, failure with summaries, auto-fix success path, auto-fix+still-failing path, periodic full validation gate.

**Acceptance Criteria:**
- `Validate` implements `pipeline.Stage`
- `Output.ValidationSummaries` is non-nil on failure
- Auto-fix applied before returning failure; resolving auto-fix returns Proceed not Block
- `go test ./internal/pipeline/validate/...` passes
- No import of `internal/runner/`

**Dependencies:** Task 1 (Stage interface).

**Notes:** The validation logic currently lives in `runner/validation/runner.go`. The `validate` stage can call `runner/validation` as a dependency (same as escalation — independent package by compiler). Check for import cycles.

---

### Task 7: Review stage

**Files:**
- Create: `internal/pipeline/review/review.go`
- Create: `internal/pipeline/review/review_test.go`

**What to Do:**
Implement `Review` struct in `package review` satisfying `pipeline.Stage`.

`Review.Run(ctx, Input)`:
1. If review is disabled in `Input.Config`, returns `Output{Decision: Proceed}` immediately (noop).
2. Invokes LLM review via `StreamRun`.
3. Parses review output for findings.
4. Creates new beads with `[from-review]` label for each finding.
5. Returns `Output{Decision: Proceed, ReviewBeadIDs: [...]}`.

`review_test.go`: tests for disabled config (noop), enabled with findings (creates beads), enabled with no findings (passes through).

**Acceptance Criteria:**
- `Review` implements `pipeline.Stage`
- Returns Proceed immediately when review disabled in config — no LLM call
- Creates `[from-review]`-labeled beads from findings
- `go test ./internal/pipeline/review/...` passes
- No import of `internal/runner/`

**Dependencies:** Task 1 (Stage interface).

**Notes:** The review stage in `pipeline/review/` is distinct from the existing `pipeline/review.go` (which handles `ReviewNonInteractive` for the CLI). The new `pipeline/review/` sub-package handles the per-iteration code review stage in the run loop. No conflict — different packages.

---

### Task 8: Epilogue stage — bead lifecycle

**Files:**
- Create: `internal/pipeline/epilogue/epilogue.go`
- Create: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
Implement `Epilogue` struct in `package epilogue` satisfying `pipeline.Stage`.

`Epilogue.Run(ctx, Input)` handles:
1. **Bead close + sync**: closes the bead on success path; syncs.
2. **Spec gate evaluation**: evaluates spec acceptance criteria when spec-level methodology is active.
3. **Worktree branch merge**: merges interactive worktree branches when applicable.
4. **`status.json` write**: updates the run status file.
5. **Thorough review trigger**: triggers by frequency counter or epic completion signal.
6. **Between-iterations command**: executes `cfg.Loop.BetweenIterationsCommand` if set.

`epilogue_test.go`: tests for successful bead close, worktree merge trigger, status write, between-iterations command execution.

**Acceptance Criteria:**
- `Epilogue` implements `pipeline.Stage`
- Bead close + sync called on success path
- Between-iterations command executed when configured
- `status.json` written after each iteration
- `go test ./internal/pipeline/epilogue/...` passes (for the lifecycle behaviors)
- No import of `internal/runner/`

**Dependencies:** Task 1 (Stage interface).

---

### Task 9: Epilogue stage — learning extraction and metrics

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Modify: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
Add the learning/metrics behaviors to the Epilogue stage:

1. **Failure-path learning extraction always runs**: regardless of `Input.Tier` and regardless of package novelty filters applied to success-path learning. This is unconditional. Extract from `runner/callbacks.go` failure learning path.

2. **Touched-packages tracking**: `Input` carries `TouchedPackages []string` accumulated across iterations. Epilogue updates the orchestrator's loop-level touched-packages state (via `Output.TouchedPackages`) after each iteration. Success-path learning gating uses this: skip if touched packages are not novel.

3. **`usage_limited` field persistence**: when `Input.Result.UsageLimited == true`, the epilogue's iteration log write includes `usage_limited: true` in the JSONL entry.

4. **Iteration log write**: epilogue owns writing the `IterationLog` to the logger. Consolidates what currently happens across `runner/logging.go` and `runner/writeiterationlog.go`.

`epilogue_test.go` additions: failure-path learning runs even when tier is low + package not novel; touched-packages returned in Output; usage_limited=true persisted to log.

**Acceptance Criteria:**
- Failure-path learning extraction runs on every failure, regardless of tier or package filter
- `Output.TouchedPackages` carries the updated package set for the orchestrator
- When `Input.Result.UsageLimited=true`, iteration log entry contains `usage_limited: true`
- `go test ./internal/pipeline/epilogue/...` passes including new tests
- No import of `internal/runner/`

**Dependencies:** Task 8 (Epilogue base implementation).

---

### Task 10: Orchestrator

**Files:**
- Create: `internal/runner/orchestrator.go`

**What to Do:**
Implement the `Run` method loop in `orchestrator.go`. The orchestrator imports `internal/pipeline` and the 5 stage sub-packages. It holds no business logic beyond sequencing.

Loop structure:
```
for each bead:
    gateOut = Gate.Run(ctx, Input{bead, ...})
    if gateOut.Decision == Skip || Block:
        iteration++  ← monotonically increases even for scope-blocked beads
        Epilogue.Run(ctx, Input{...})  ← failure-path epilogue
        continue
    buildOut = Build.Run(ctx, Input{..., ValidationFailures: accumulated})
    if buildOut failed:
        accumulate failure summaries
        retry Build (up to limit) or move to Epilogue
    validateOut = Validate.Run(ctx, Input{...})
    if validateOut failed:
        feed summaries into next Build Input.ValidationFailures
        retry
    if cfg.Review.Enabled:
        Review.Run(ctx, Input{...})
    Epilogue.Run(ctx, Input{...})
    iteration++
```

**Monotonically-increasing iteration numbers**: the iteration counter increments for every bead processed, including scope-blocked beads. No bead reuses the same iteration number.

**Global stats merge**: `Run()` reads the existing `global_stats.json` file (if present) and merges new per-bead/model stats and refreshes `Updated` timestamp. Does not overwrite from zero.

**Acceptance Criteria:**
- `orchestrator.go` imports `internal/pipeline` and stage packages only — no callbacks, no process.go logic
- Iteration counter monotonically increases for scope-blocked beads (test via acceptance suite)
- `Run()` merges existing global stats rather than overwriting
- `go build ./...` passes (import cycle enforcement)

**Dependencies:** Tasks 4–9 (all stages implemented).

---

### Task 11: Constructor

**Files:**
- Modify: `internal/runner/constructor.go` (replace existing implementation)

**What to Do:**
Rewrite `constructor.go` to wire the 5 stages and produce an orchestrator.

Key requirement: **Router-only construction** — `NewRunnerWithDeps` (or equivalent) must support a `Deps` struct that has only a `Router` field (no `Claude` field) and construct successfully. This is currently blocked by the old constructor requiring `claude.Client`.

`constructor.go` responsibilities:
1. Build `Gate` from deps (BeadClient, PromptRenderer, StuckPolicy, etc.)
2. Build `Build` from deps (Router, PromptRenderer, MethodologyPolicy, etc.)
3. Build `Validate` from deps (CmdRunner, AutoFixFn, ValidationPolicy, etc.)
4. Build `Review` from deps (Router, PromptRenderer, BeadClient, config)
5. Build `Epilogue` from deps (BeadClient, Logger, StateFile, WorktreeManager, etc.)
6. Return orchestrator with wired stages

Existing `constructor.go` logic for building providers, routers, and policies is retained where reusable; only the `Runner` struct assembly is replaced.

**Acceptance Criteria:**
- `NewRunnerWithDeps` accepts a `Deps` with only a `Router` (no `Claude` field) and constructs without error
- `NewRunner` (standard constructor) builds from config as before
- `go build ./...` passes
- `go test -tags acceptance ./internal/runner/acceptance/...` passes

**Dependencies:** Task 10 (orchestrator), Tasks 4–9 (all stages).

---

### Task 12: Integration, acceptance gate, and cleanup

**Files:**
- Delete: superseded `internal/runner/*.go` production files
- Verify: `go build ./...`, `go test ./...`, `go test -tags acceptance ./internal/runner/acceptance/...`

**What to Do:**
1. Run `go test -tags acceptance ./internal/runner/acceptance/...` against the new orchestrator + stages. Investigate and fix any behavioral divergence from the prior runner.

2. Delete the superseded runner production files now replaced by stage packages and orchestrator:
   - `callbacks.go`, `callbacks_tdd.go`, `callbacks_validation.go`, `callbacks_coverage.go`
   - `process.go`, `process_methodology.go`, `process_methodology_deadline.go`
   - `run_iteration.go`, `run_init.go`
   - `runner.go` (replaced by orchestrator.go)
   - `epilogue.go` (replaced by epilogue stage)
   - `gates.go`, `lifecycle.go`
   - `adapters.go` (if superseded), `helpers.go` (if superseded)
   - Other files that contained logic now owned by a stage

   Keep: `constructor.go` (rewritten), `orchestrator.go` (new), `interfaces.go`, `syncwriter.go`, `format.go`, `format_bead_breakdown.go`, `status.go`, `logging.go`, and any file not yet absorbed by a stage.

3. Verify `go build ./...` passes — import cycle constraint confirmed mechanically.
4. Verify `go test ./...` passes (unit tests).
5. Verify `go test -tags acceptance ./internal/runner/acceptance/...` passes — migration complete.

**Acceptance Criteria:**
- `go build ./...` passes
- `go test ./...` passes
- `go test -tags acceptance ./internal/runner/acceptance/...` passes
- `internal/runner/` contains only `orchestrator.go`, `constructor.go`, `acceptance/`, and support files (`interfaces.go`, `syncwriter.go`, format/status/logging files that are not stage logic)
- No stage sub-package imports `internal/runner/` (verified by `go build`)

**Dependencies:** Task 11 (constructor complete), Task 3 (old tests deleted).

---

## Notes

- **Migration is additive first**: stages are implemented alongside the existing runner. The old runner keeps working until Task 12 when it's replaced. This lets acceptance tests be green throughout.
- **Import cycle is the contract**: `go build ./...` is the authoritative check. Any shortcut that imports `runner/` from a stage package will fail the build — no convention needed.
- **`runtypes` is already extracted**: `runtypes.IterationResult`, `runtypes.BeadContext`, etc. are available to stages without needing to cross into `runner/`. Stage packages can import `internal/runner/runtypes` directly.
- **Existing `pipeline.go` is untouched**: the `Pipeline` struct for CLI workflows (Refine, Plan, Review, Decompose) stays as-is. New stage types coexist in the same package via `stage.go`.
- **`ShouldRunPostSuccess`**: the exact method name is required by the spec. Ensure it's on the Build stage (or its methodology policy type) with the correct signature.
- **`StreamRun` not `Run`**: every LLM invocation in Build and Review stages must use `StreamRun`. A fake that panics on `Run` in tests is the recommended way to enforce this.
- **Acceptance tests need fake helpers**: `internal/runner/acceptance/helpers_test.go` is the critical scaffolding for Task 2. If the fakes are incomplete, tests will fail to compile. Invest time here.
