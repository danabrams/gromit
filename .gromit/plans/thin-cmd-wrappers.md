---
id: thin-cmd-wrappers
source_spec: thin-cmd-wrappers
created: 2026-02-27
decomposed: false
---

# Thin CMD Wrappers Implementation Plan

**Goal:** Make every `cmd/gromit` command a thin interface layer while moving reusable business logic into `internal/pipeline` as the product API.

**Architecture:** Expand `internal/pipeline` with workflow and query methods that own prompt construction, scope resolution, filtering, and operation orchestration. Keep `cmd/gromit` responsible only for flags, interactive UX, and terminal formatting.

**Tech Stack:** Go, Cobra CLI, existing `internal/pipeline` deps interfaces, `internal/bead`, `internal/backlog`, `internal/state`, prompt renderer packages.

**Spec:** `.gromit/specs/thin-cmd-wrappers.md`

---

## Architecture

### Overview
Adopt a strict interface boundary: `cmd/gromit` collects user intent and displays output, while `internal/pipeline` executes all workflow logic and data-selection logic. The pipeline package becomes the stable API surface for CLI, TUI, and future API server frontends.

### Key Components
1. **Pipeline API Expansion**
   - Implement missing workflow entrypoints (`Plan`) and add workflow methods for command behaviors still implemented directly in `cmd/gromit` (board, queue, stats, add, triage/resolve paths as needed).
   - Define typed inputs/outputs in `internal/pipeline/types.go` for each workflow/query.

2. **Pipeline Query Service**
   - Add query methods for listing specs/plans/ideas/beads and selection-oriented filtering in pipeline, not command files.
   - Move picker candidate shaping logic out of command files (for example undecomposed plan filtering and unplanned spec filtering).

3. **Pipeline Scope Resolution**
   - Move review scope determination (`--since`, `--spec`, `--epic`, state/tag fallback) from `cmd/gromit/review.go` to pipeline.
   - Keep CLI-specific flag mutual exclusivity validation at command layer; move repository/state/bead lookup logic into pipeline.

4. **Prompt Construction in Pipeline**
   - Ensure plan and review prompt construction is pipeline-owned.
   - Move `plan` prompt assembly from `cmd/gromit/plan.go` and keep command layer to input collection and launch/result rendering.
   - For renderer-backed prompts, keep interface adapters in CLI package but keep composition/orchestration in pipeline.

5. **Interface-Specific Adapters**
   - Keep adapters in interface packages (`cmd/gromit`) satisfying `pipeline.Deps` interfaces.
   - Avoid any `internal/pipeline` imports from `cmd` packages and any reverse dependency from pipeline into interface code.

6. **Boundary Guardrails**
   - Add structural tests that assert command files call pipeline methods and avoid direct business-layer calls.
   - Add import guard tests ensuring `internal/pipeline` does not import `cmd/` or interface-specific packages.

### Integration Points
- `cmd/gromit/plan.go` currently builds prompt/context and launches agent directly; this becomes a thin wrapper around `pipeline.Plan`.
- `cmd/gromit/review.go` currently owns scope-resolution and bead lookup logic; this moves to pipeline review API.
- `cmd/gromit/decompose.go` has plan picker/filter logic that should be served by pipeline queries.
- Simpler command files (`board.go`, `queue.go`, `stats.go`, `add.go`, and related helpers) should call pipeline methods rather than direct internal package orchestration.
- Existing pipeline methods (`Refine`, `Explore`, `Decompose`, `Review*`) remain and are normalized to consistent boundary rules.

### Data Flow
1. Command parses flags/args and gathers interactive input.
2. Command passes a typed input struct to pipeline workflow/query method.
3. Pipeline resolves scope, filters candidates, builds prompt/context, and performs state/bead/backlog operations.
4. Pipeline returns typed outputs describing results and displayable metadata.
5. Command renders text output and optional confirmations/pickers.

### Files to Modify
- `internal/pipeline/pipeline.go` - implement `Plan`, add new method signatures, and dependency validation.
- `internal/pipeline/types.go` - define typed inputs/outputs for new workflow and query methods.
- `internal/pipeline/helpers.go` - shared file scanning and filtering helpers for query methods.
- `internal/pipeline/review.go` and/or `internal/pipeline/review/*` - move review scope resolution and prompt orchestration boundaries.
- `internal/pipeline/refine.go` - align with shared workflow conventions where needed.
- `internal/pipeline/explore.go` - keep workflow orchestration only; ensure command-specific behaviors stay out.
- `cmd/gromit/plan.go` - replace embedded business logic with pipeline call + CLI UX.
- `cmd/gromit/review.go` - remove scope and orchestration logic, delegate to pipeline.
- `cmd/gromit/decompose.go` - delegate candidate listing/filtering queries to pipeline.
- `cmd/gromit/board.go` - delegate board data computation to pipeline.
- `cmd/gromit/queue.go` - delegate queue ordering/blocking computations to pipeline.
- `cmd/gromit/stats.go` - delegate stats data aggregation to pipeline.
- `cmd/gromit/add.go` - delegate backlog idea creation/categorization workflow to pipeline.
- `cmd/gromit/cli_adapters.go` - keep adapter implementations and remove business orchestration leakage where present.
- `cmd/gromit/main.go` - dependency wiring updates for expanded pipeline methods.

### Files to Create
- `internal/pipeline/plan.go` - full plan workflow implementation (if separated from `pipeline.go`).
- `internal/pipeline/query.go` - specs/plans/ideas/beads listing and filtering methods.
- `internal/pipeline/review_scope.go` - reusable review scope resolution.
- `internal/pipeline/board.go` - board query/workflow output assembly.
- `internal/pipeline/queue.go` - queue ordering and partition logic.
- `internal/pipeline/stats.go` - stats aggregation workflow.
- `internal/pipeline/add.go` - backlog add workflow (categorization, optional context handling).
- `internal/pipeline/*_test.go` siblings for each new workflow/query unit.

### Tradeoffs
- **Single pipeline product API vs additional domain packages**
  - Chose expanding `internal/pipeline` to complete the existing migration and avoid introducing parallel abstractions.
- **Centralized query/filter logic vs per-interface implementation**
  - Chose centralized pipeline queries to prevent drift between CLI/TUI/API implementations.
- **Command-layer pickers retained**
  - Chose to keep interactive picker UX in command layer while moving candidate derivation into pipeline, preserving UX flexibility.
- **Incremental command migration vs one-shot rewrite**
  - Chose staged migration with boundary tests to reduce risk across large command surface.

## Test Strategy

### Test Levels
1. **Pipeline Unit Tests**
   - New workflow/query methods: happy paths, edge cases, and dependency validation.
   - Scope resolution precedence and fallback behavior for review.
   - Plan workflow end-to-end prompt generation and agent launch orchestration via mocks.

2. **Command Delegation Tests**
   - Command tests assert flag parsing and output remain intact while business calls route through pipeline.
   - Ensure interactive command branches (pickers/confirmations) still work with pipeline-returned candidates.

3. **Boundary and Contract Tests**
   - Import boundary tests guaranteeing `internal/pipeline` has no `cmd/` imports.
   - Structural tests preventing reintroduction of business logic patterns in command files.

4. **Regression/Integration Tests**
   - Existing refine/explore/review/decompose command behavior stays functionally equivalent.
   - Non-interactive review and decomposition output semantics unchanged.

### Key Test Cases
- `Pipeline.Plan` succeeds with valid deps, writes prompt temp file, resolves agent, and returns session/output correctly.
- Review scope precedence: `--since` > `--spec` > `--epic` > repo state fallback.
- Review scope fallback errors remain user-actionable when no prior state exists.
- Plan picker and spec picker candidate filtering matches prior behavior.
- Queue partitioning (ready/blocked/stuck) and ordering remains unchanged after moving logic to pipeline.
- Stats JSON/text payload data is unchanged for representative log fixtures.
- Add workflow preserves categorization behavior and unknown-type interactive handoff contract.
- Command tests verify each migrated command uses pipeline entrypoints rather than direct calls into bead/backlog/logger internals.

### Mocking Strategy
- Reuse typed `pipeline.Deps` fakes in pipeline tests.
- Use command-level injected functions/fakes for picker input and terminal output capture.
- Keep file-backed integration tests where current behavior already depends on frontmatter/backlog/log files.

### Coverage Goals
- Full coverage on new pipeline entrypoints and query methods.
- Explicit tests for acceptance-criteria-critical moves:
  - plan workflow implementation
  - review scope resolution in pipeline
  - prompt construction in pipeline
  - delegation of agent launching through pipeline.
- Regression coverage for command output shape and non-interactive behavior.

### Test Organization
- New tests colocated with each new pipeline file: `internal/pipeline/<feature>_test.go`.
- Command delegation tests remain in `cmd/gromit/*_test.go` and focus on interface-layer responsibilities.
- Keep existing smoke/contract tests and extend them with boundary assertions.

## Implementation Tasks

### Task 1: Define Pipeline API Surfaces for Workflows and Queries

**Files:**
- Modify: `internal/pipeline/types.go`
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/pipeline_test.go`

**What to Do:**
Introduce typed inputs/outputs and method contracts for plan, review scope resolution, and query/list operations needed by command pickers and displays.

**Acceptance Criteria:**
- Pipeline exposes typed method signatures for required workflows and queries.
- Dependency validation is explicit and nil-safe for each method.
- New API compiles without interface-specific imports.

**Dependencies:**
- None.

**Notes:**
- Keep method signatures frontend-agnostic so TUI/API can reuse them unchanged.

### Task 2: Implement `Pipeline.Plan()` as the Canonical Plan Workflow

**Files:**
- Create: `internal/pipeline/plan.go`
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/plan_test.go`

**What to Do:**
Implement plan workflow currently stubbed in `pipeline.go`: spec loading, open-bead context assembly, prompt construction with plan skill, temp prompt writing, agent resolution/launch orchestration, and typed result/session output.

**Acceptance Criteria:**
- `Pipeline.Plan` no longer returns "not yet implemented".
- Prompt construction and launch flow happen inside pipeline.
- Error messages for missing spec/duplicate plan are preserved or improved without regressions.

**Dependencies:**
- Task 1.

**Notes:**
- Keep output content deterministic enough for existing/golden command tests.

### Task 3: Add Pipeline Query Methods for Specs, Plans, Ideas, and Beads

**Files:**
- Create: `internal/pipeline/query.go`
- Modify: `internal/pipeline/helpers.go`
- Test: `internal/pipeline/query_test.go`

**What to Do:**
Implement centralized query/filter methods used by interface selection UIs: unplanned specs, undecomposed plans, active bead sets, and other list/filter combinations currently embedded in command files.

**Acceptance Criteria:**
- Pipeline provides query methods returning candidates with filtering logic included.
- Decompose/plan picker logic can consume pipeline queries without reimplementing filters.
- Edge cases (empty dirs, malformed frontmatter, missing files) are covered by tests.

**Dependencies:**
- Task 1.

**Notes:**
- Keep return shapes suited for both picker and non-interactive API usage.

### Task 4: Move Review Scope Resolution into Pipeline

**Files:**
- Create: `internal/pipeline/review_scope.go`
- Modify: `internal/pipeline/review.go`
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/review_scope_test.go`

**What to Do:**
Extract and centralize scope resolution logic from `cmd/gromit/review.go`, including spec/epic bead lookup and state/tag fallback precedence.

**Acceptance Criteria:**
- Scope resolution logic lives in pipeline and is reusable across interfaces.
- Precedence and error semantics match current command behavior.
- Command layer only provides user-supplied scope flags and displays results/errors.

**Dependencies:**
- Task 1.

**Notes:**
- Keep flag mutual-exclusion checks in CLI; move everything else.

### Task 5: Migrate Prompt Construction Boundaries into Pipeline

**Files:**
- Modify: `internal/pipeline/plan.go`
- Modify: `internal/pipeline/review.go`
- Modify: `cmd/gromit/cli_adapters.go`
- Test: `internal/pipeline/plan_test.go`
- Test: `internal/pipeline/review_test.go`

**What to Do:**
Ensure pipeline owns prompt assembly and context embedding for plan/review workflows; adapters remain thin interface bridges to rendering dependencies.

**Acceptance Criteria:**
- Plan prompt assembly no longer happens in `cmd/gromit/plan.go`.
- Review prompt construction path remains pipeline-owned and tested.
- No pipeline code depends on command-layer packages.

**Dependencies:**
- Task 2.
- Task 4.

**Notes:**
- Preserve warning behavior for optional context loads.

### Task 6: Refactor `plan` and `review` Commands into Thin Wrappers

**Files:**
- Modify: `cmd/gromit/plan.go`
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/plan_test.go`
- Test: `cmd/gromit/review_test.go`

**What to Do:**
Reduce command files to flag parsing, picker/confirmation UX, and output formatting. Delegate all business logic and agent orchestration to pipeline methods.

**Acceptance Criteria:**
- `plan.go` and `review.go` contain no scope/prompt/business orchestration logic.
- Existing command UX and output remain compatible.
- Tests verify delegation boundaries and behavior parity.

**Dependencies:**
- Task 2.
- Task 4.
- Task 5.

**Notes:**
- Retain command-level injected seams used by current tests.

### Task 7: Refactor `decompose` Picker/Selection and Remaining Core Commands to Pipeline

**Files:**
- Modify: `cmd/gromit/decompose.go`
- Modify: `cmd/gromit/board.go`
- Modify: `cmd/gromit/queue.go`
- Modify: `cmd/gromit/stats.go`
- Modify: `cmd/gromit/add.go`
- Create: `internal/pipeline/board.go`
- Create: `internal/pipeline/queue.go`
- Create: `internal/pipeline/stats.go`
- Create: `internal/pipeline/add.go`
- Test: `cmd/gromit/decompose_test.go`
- Test: `cmd/gromit/queue_test.go`
- Test: `cmd/gromit/stats_test.go`
- Test: `cmd/gromit/add_test.go`
- Test: `internal/pipeline/board_test.go`
- Test: `internal/pipeline/queue_test.go`
- Test: `internal/pipeline/stats_test.go`
- Test: `internal/pipeline/add_test.go`

**What to Do:**
Move business computations and selection/filter logic for decompose and simpler commands into pipeline methods; keep commands as interface wrappers.

**Acceptance Criteria:**
- Command files retain only flags, interactive user prompts, and output formatting.
- Queue/board/stats/add functional behavior matches current results.
- Decompose candidate listing/filtering comes from pipeline query methods.

**Dependencies:**
- Task 3.

**Notes:**
- Migrate in small slices to keep regression surface manageable.

### Task 8: Keep Adapters Interface-Local and Formalize Wiring

**Files:**
- Modify: `cmd/gromit/cli_adapters.go`
- Modify: `cmd/gromit/adapters.go`
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/adapter_*_test.go`

**What to Do:**
Align adapter locations and dependency wiring so each interface owns implementations of pipeline dependency interfaces.

**Acceptance Criteria:**
- Adapters remain in `cmd/gromit` and satisfy expanded `pipeline.Deps` contracts.
- Pipeline package has no interface-specific implementation imports.
- Adapter tests validate typed signature compatibility.

**Dependencies:**
- Task 1.
- Task 2.
- Task 3.

**Notes:**
- Keep shared adapters reusable for future TUI/API entrypoints.

### Task 9: Add Boundary Guard Tests and Anti-Regression Contracts

**Files:**
- Modify: `cmd/gromit/cli_contract_test.go`
- Modify: `cmd/gromit/cmd_smoke_*_test.go`
- Modify: `internal/pipeline/typed_interfaces_behavioral_test.go`
- Create: `internal/pipeline/import_boundary_test.go`

**What to Do:**
Add tests that fail when business logic leaks back into commands or when pipeline takes interface-specific dependencies.

**Acceptance Criteria:**
- Tests assert `internal/pipeline` does not import `cmd/` or interface packages.
- Command smoke/contract tests enforce delegation and output parity.
- Migration acceptance criteria are directly encoded in tests where possible.

**Dependencies:**
- Task 6.
- Task 7.
- Task 8.

**Notes:**
- Keep assertions stable and focused on structural invariants, not line counts.

### Task 10: Full Validation and Migration Cleanup

**Files:**
- Modify: touched files above
- Test: full impacted package suites

**What to Do:**
Run targeted and then broad quality gates, remove dead command-layer business helpers, and ensure migrated command surfaces remain coherent.

**Acceptance Criteria:**
- `go test ./internal/pipeline/... ./cmd/gromit/...` passes.
- `go test ./...`, `go vet ./...`, and `go build ./...` pass or pre-existing failures are documented.
- No TODO stubs remain for migrated acceptance-criteria paths (including `Pipeline.Plan`).

**Dependencies:**
- Task 2 through Task 9.

**Notes:**
- Keep cleanup scoped to migration-related dead code only.

---

## Notes

- This migration should be implemented incrementally with behavior parity checks after each command conversion to avoid broad regressions.
- Treat `internal/pipeline` method signatures as long-lived interface contracts; prioritize clarity and frontend neutrality.
- If command-specific UX requires derived display metadata, return typed metadata from pipeline rather than recomputing in command files.
