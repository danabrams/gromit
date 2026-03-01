---
created: 2026-02-28T00:00:00Z
decomposed: true
decomposed_at: "2026-03-01T02:15:57Z"
id: branch-per-spec
source_spec: branch-per-spec
---

# Branch Per Spec ATDD Workflow Implementation Plan

**Goal:** Implement a resumable, stage-driven ATDD workflow for `gromit run --spec <name>` that isolates work on `gromit/spec-<name>` branches and enforces local/review/global gates before merge.

**Architecture:** Introduce a spec workflow state machine persisted in spec frontmatter, wire it into `run --spec` orchestration, and expand the existing spec merge pipeline into staged blocking gates with fix-bead loops and finalization.

**Tech Stack:** Go, Cobra CLI, existing Gromit runner/orchestrator, `internal/frontmatter`, `internal/specgate`, `internal/runner/specbranch`, `internal/runner/specmerge`, tracker/bead adapters.

**Spec:** `.gromit/specs/branch-per-spec.md`

---

## Architecture

**Overview:**
Add a dedicated spec workflow manager that persists stage in `.gromit/specs/<name>.md` frontmatter and drives `run --spec` through acceptance authoring, implementation, local gate, review, global gate, and merge completion.

**Key Components:**
1. **`internal/specflow` (new)**: Stage enum, transition validation, frontmatter persistence, and resume semantics.
2. **Run-stage orchestration**: `cmd/gromit/main.go` and runner wiring invoke specflow when `--spec` is set.
3. **Acceptance-tests stage execution**: Runner-level path creates/executes acceptance-test-focused beads before implementation.
4. **Local spec gate**: Reuse `verify-spec`/`specgate` logic to validate only spec acceptance behavior and synthesize fix beads on failure.
5. **Review and global gate execution**: Extend `internal/runner/specmerge` from placeholder trigger to staged hard gates with retries.
6. **Finalize + merge**: Use `specbranch` + `specmerge.FinalizeSpecBranch` to rebase, ff-merge, delete branch, and set stage `done`.

**Integration Points:**
- `gromit run --spec` currently resolves scoped labels and starts runner; add spec stage bootstrap and transition persistence.
- Existing branch routing (`specbranch.Router`) already maps `spec:<name>` to `gromit/spec-<name>`.
- Existing acceptance verification and fix-bead synthesis (`verify-spec`, `specgate`) provide local gate primitives.
- Existing spec merge primitives (`specmerge` review stage helpers + merge ops) become the enforced post-implementation pipeline.

**Data Flow:**
1. `run --spec <name>` loads spec frontmatter stage.
2. Fresh or `planning` stage creates/checks out `gromit/spec-<name>`, advances to `acceptance-tests`.
3. Acceptance test beads are generated/executed and committed, then stage advances to `implementation`.
4. Implementation beads execute on the spec branch until scoped work is complete.
5. `local-gate` runs spec-only acceptance checks; failure creates fix beads and loops.
6. `review` runs staged blocking review checks; findings create fix beads, then local gate is re-run.
7. `global-gate` runs full regression suite.
8. On pass (or explicit override), finalize merge to `main` and mark stage `done`.

**Files to Modify:**
- `cmd/gromit/main.go` - Run command stage bootstrap and resume entry for `--spec`.
- `internal/runner/orchestrator.go` - Stage-aware completion/transition hooks for spec-scoped runs.
- `internal/runner/constructor.go` - Wire specflow-aware controller/dependencies into orchestrator.
- `internal/runner/constructor_adapters.go` - Extend spec merge controller wiring and stage dependencies.
- `internal/runner/specmerge/pipeline.go` - Implement staged trigger flow and retry loop.
- `internal/runner/specmerge/review_stages.go` - Integrate as hard gates in trigger flow.
- `internal/runner/specbranch/git_ops.go` - Ensure finalize path supports workflow completion semantics.
- `cmd/gromit/verify_spec.go` - Reuseable local gate helpers for spec stage execution.

**Files to Create:**
- `internal/specflow/stage.go` - Stage constants and transition matrix.
- `internal/specflow/store.go` - Frontmatter load/store/update for spec stage.
- `internal/specflow/manager.go` - Resume, advance, and guard APIs.
- `internal/specflow/stage_test.go` - Transition correctness tests.
- `internal/specflow/store_test.go` - Frontmatter persistence tests.
- `internal/specflow/manager_test.go` - Resume/idempotency tests.
- `internal/runner/specmerge/trigger_flow.go` - Staged review/global-gate orchestration.

**Tradeoffs:**
- **Frontmatter persistence vs separate state file:** frontmatter is explicit and source-controlled, but requires strict transition validation.
- **In-process stage orchestration vs shelling out commands:** in-process is more testable/reliable, but increases runner complexity.
- **Hard stage machine vs loose flags:** hard machine improves resumability and correctness, with added implementation overhead.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** Spec stage parsing, transition guards, resume behavior, and invalid frontmatter handling.
2. **Integration Tests:** Runner/specmerge flow for branch creation, stage progression, fix-bead loops, and merge gating.
3. **CLI/Acceptance Tests:** `run --spec` behavior for fresh start, interruption, and resume from each stage.

**Key Test Cases:**
- Fresh spec run creates/checks out `gromit/spec-<name>` and sets stage to `acceptance-tests`.
- Resuming from each intermediate stage continues correctly without regressing prior stage state.
- Local gate failures create fix beads and block transition to `review`.
- Review findings create fix beads and require local gate pass before `global-gate`.
- Global gate failures halt merge and preserve branch/stage for human intervention.
- Successful pipeline sets stage to `done` and finalizes merge/deletes spec branch.
- Invalid or missing stage/frontmatter fails with actionable errors.
- Non-spec run behavior remains unchanged.

**Mocking Strategy:**
- Mock git operations for branch/rebase/merge/conflict scenarios.
- Mock tracker/bead interactions for open/closed/fix-bead creation paths.
- Mock review providers and command runner for deterministic gate pass/fail.
- Use temp real files for frontmatter persistence behavior.

**Coverage Goals:**
- Full legal/illegal transition matrix coverage.
- Retry-cap behavior for review/global gate loops.
- Human-stop path coverage for global regression and merge conflicts.
- Backward compatibility for existing non-`--spec` flows.

**Test Organization:**
- `internal/specflow/*_test.go` for state machine and persistence.
- `internal/runner/*_test.go` for stage wiring into run loop.
- `internal/runner/specmerge/*_test.go` for hard-gate pipeline behavior.
- `cmd/gromit/*_test.go` for CLI-level `run --spec` contracts.

## Implementation Tasks

### Task 1: Build Spec Workflow State Core

**Files:**
- Create: `internal/specflow/stage.go`
- Create: `internal/specflow/store.go`
- Create: `internal/specflow/manager.go`
- Test: `internal/specflow/stage_test.go`
- Test: `internal/specflow/store_test.go`
- Test: `internal/specflow/manager_test.go`

**What to Do:**
Implement stage definitions (`planning`, `acceptance-tests`, `implementation`, `local-gate`, `review`, `global-gate`, `done`), transition rules, and frontmatter-backed read/update APIs with idempotent resume behavior.

**Acceptance Criteria:**
- Stage values and transition validation are centralized and unit-tested.
- Spec frontmatter stage can be read/updated safely with clear errors on malformed data.
- Resume API returns deterministic next action for each stage.

**Dependencies:**
- None.

**Notes:**
Keep parsing/writing isolated from runner code so future stage additions remain local.

### Task 2: Wire `run --spec` Into Stage Bootstrap/Resume

**Files:**
- Modify: `cmd/gromit/main.go`
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/constructor_adapters.go`
- Test: `cmd/gromit/run_spec_flag_test.go` (or adjacent run tests)

**What to Do:**
When `--spec` is provided, initialize specflow manager, ensure branch exists/is checked out on fresh start, and pass stage context into runner/orchestrator dependencies for stage-aware execution.

**Acceptance Criteria:**
- Fresh `run --spec` starts workflow at `acceptance-tests` and on `gromit/spec-<name>`.
- Existing stage is loaded and reused on resumed runs.
- Non-spec runs are unaffected.

**Dependencies:**
- Task 1.

**Notes:**
Use existing scope resolution and label infrastructure; avoid duplicate spec name parsing.

### Task 3: Implement Acceptance-Tests Stage Execution

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/runner/constructor_adapters.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Add pre-implementation stage behavior that generates/executes acceptance-test authoring work for the spec (using existing decomposition/prompt infrastructure) and advances stage to `implementation` on success.

**Acceptance Criteria:**
- Acceptance-test stage runs before implementation beads for fresh/resumed spec flows.
- Stage only advances to `implementation` after acceptance-test authoring completes successfully.
- Failures do not incorrectly advance stage.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Keep acceptance generation entrypoints mockable for deterministic tests.

### Task 4: Add Local Spec Gate Stage and Failure Loop

**Files:**
- Modify: `cmd/gromit/verify_spec.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `cmd/gromit/verify_spec_test.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Run local spec acceptance gate at `local-gate`, synthesize fix beads on failure, keep stage from advancing until pass, and transition to `review` only on successful local gate verdict.

**Acceptance Criteria:**
- Local gate runs spec-focused acceptance checks only.
- Gate failures produce spec-labeled fix beads and preserve retryable stage state.
- Gate pass advances stage to `review`.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
Prefer reusing existing `specgate.SynthesizeFixBeads` behavior.

### Task 5: Expand Spec Merge Trigger to Full Review Pipeline

**Files:**
- Modify: `internal/runner/specmerge/pipeline.go`
- Create: `internal/runner/specmerge/trigger_flow.go`
- Modify: `internal/runner/specmerge/review_stages.go`
- Test: `internal/runner/specmerge/pipeline_test.go`

**What to Do:**
Replace placeholder `Trigger` behavior with staged hard gates (spec conformance, code quality, architecture), fix-bead synthesis on failure, retry cap enforcement, and re-entry from stage 1 after fixes.

**Acceptance Criteria:**
- Trigger executes blocking review stages in order.
- Any failed stage creates fix beads and stops later stages.
- Retry cap is enforced with terminal alert behavior.

**Dependencies:**
- Task 4.

**Notes:**
Keep stage orchestration explicit so future gate additions are straightforward.

### Task 6: Implement Global Regression Gate + Human Halt Semantics

**Files:**
- Modify: `internal/runner/specmerge/pipeline.go`
- Modify: `internal/runner/specmerge/pipeline_test.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Add `global-gate` full-suite execution after review stage success; on failure, halt workflow progression, keep spec branch checked out, and persist resumable state requiring human action/override path.

**Acceptance Criteria:**
- Full regression suite runs only after review gates pass.
- Global failures prevent merge/finalization and persist halt state.
- Resume behavior from halted global gate is deterministic.

**Dependencies:**
- Task 5.

**Notes:**
Use existing full validation command conventions from config.

### Task 7: Finalize Merge and Mark Stage Done

**Files:**
- Modify: `internal/runner/specmerge/merge_ops.go`
- Modify: `internal/runner/specbranch/git_ops.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/specbranch/git_ops_test.go`
- Test: `internal/runner/specmerge/pipeline_test.go`

**What to Do:**
After global gate pass (or explicit override), finalize spec branch via rebase + ff merge + delete branch, then persist stage `done`.

**Acceptance Criteria:**
- Successful finalize merges spec branch into main and deletes spec branch.
- Stage updates to `done` only after finalize success.
- Conflict/error paths leave workflow resumable and do not falsely mark done.

**Dependencies:**
- Task 6.

**Notes:**
Reuse `FinalizeSpecBranch` conflict handling flow; do not duplicate git orchestration.

### Task 8: End-to-End and Compatibility Coverage

**Files:**
- Modify: `cmd/gromit/*run*_test.go` (relevant acceptance/smoke tests)
- Modify: `internal/runner/*_test.go`
- Modify: `internal/runner/specmerge/*_test.go`

**What to Do:**
Add end-to-end style tests that cover fresh spec run, interrupted resume, local-fail/fix loops, review/global gate progression, and non-spec backward compatibility.

**Acceptance Criteria:**
- End-to-end tests validate full stage progression from `planning` to `done`.
- Resume from interruption at intermediate stages is validated.
- Existing non-spec run behavior remains green.

**Dependencies:**
- Task 1
- Task 2
- Task 3
- Task 4
- Task 5
- Task 6
- Task 7

**Notes:**
Prioritize deterministic fakes for provider and git behavior to keep runtime stable.

---

## Notes

- This plan intentionally aligns with existing implemented specs (`spec-level-atdd-gates`, `spec-level-atdd-execution`, `spec-branch-merge-pipeline`) and fills the remaining gap: explicit frontmatter-based stage persistence/resume and full hard-gated trigger execution.
- Keep stage writes atomic and minimal to avoid corrupting spec files during interruptions.
- Prefer extending existing adapters and pipelines over introducing parallel orchestration paths.
