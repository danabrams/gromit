---
id: spec-branch-merge-pipeline
source_spec: spec-branch-merge-pipeline
created: 2026-02-26
decomposed: false
---

# Spec Branch Merge Pipeline Implementation Plan

**Goal:** Execute spec-labeled beads on isolated per-spec branches and land them to `main` only through a strict four-stage merge pipeline with fix-loop retries.

**Architecture:** Add branch-aware bead routing in the run loop plus a dedicated merge-pipeline subsystem that runs full validation and three blocking review stages, synthesizes fix beads on failure, and performs rebase/fast-forward merge with conflict-resolution safeguards.

**Tech Stack:** Go, existing runner pipeline (`internal/runner`), worktree/git abstractions (`internal/worktree`), review infrastructure (`internal/runner/reviewpkg`), config system (`internal/config`), bead client (`internal/bead`).

**Spec:** `.gromit/specs/spec-branch-merge-pipeline.md`

---

## Architecture

**Overview:**
The run loop becomes branch-aware. For each bead, it resolves execution branch from labels: `spec:<name>` beads run on `gromit/spec-<name>`, and non-spec beads run on `main`. When a spec completes (no remaining open/ready beads for that spec), a new merge pipeline runs as a hard-gated sequence (Stage 1-4). Failures create spec-labeled fix beads and retry from Stage 1 up to a configurable cap.

**Key Components:**
1. **Spec Branch Router (`internal/runner/specbranch`)**: Resolves bead → branch, ensures branch lifecycle rules, and performs safe branch switching before iteration execution.
2. **Spec Branch Git Ops (`internal/worktree` extension or focused helper in `specbranch`)**: Creates spec branches from `main`, rebases onto `main`, performs fast-forward merge, and deletes merged branch.
3. **Spec Merge Pipeline (`internal/runner/specmerge`)**: Executes Stage 1 Full Validation, Stage 2 Spec Conformance Review, Stage 3 Code Quality Review, Stage 4 Architectural Review with hard stop semantics.
4. **Fix Loop Controller (`specmerge`)**: On stage failure, synthesizes fix beads via existing synthesis path and re-triggers the pipeline from Stage 1 after fix beads close.
5. **Constructor Wiring (`internal/runner/constructor*.go`)**: Replaces old spec-gate epilogue trigger wiring with merge-pipeline wiring and dependencies.

**Integration Points:**
- `internal/runner/orchestrator.go`: per-iteration branch resolution and checkout before Build.
- `internal/pipeline/epilogue/epilogue.go`: remove/retire old spec-gate auto-trigger path for spec completion.
- `internal/runner/constructor.go` and `internal/runner/constructor_adapters.go`: add merge-pipeline adapter wiring.
- `internal/config/config_types.go` + defaults/accessors: add pipeline retry cap and stage controls.
- `internal/runner/reviewpkg/*`: reuse review invocation/parsing patterns for blocking merge stages.

**Data Flow:**
1. Runner fetches next bead.
2. Router resolves execution branch (`main` or `gromit/spec-<name>`).
3. Runner checks out resolved branch and executes normal stages.
4. On successful close of a spec bead, completion detector checks if spec has remaining work.
5. If spec is complete, merge pipeline runs Stage 1→4.
6. Any failed stage synthesizes spec-labeled fix beads and records attempt.
7. On fix completion, pipeline restarts at Stage 1.
8. On all-pass, pipeline rebases spec branch on latest `main`, fast-forward merges, handles conflicts via LLM resolver fallback, and deletes spec branch.

**Files to Modify:**
- `internal/runner/orchestrator.go`
- `internal/runner/constructor.go`
- `internal/runner/constructor_adapters.go`
- `internal/pipeline/epilogue/epilogue.go`
- `internal/worktree/worktree.go`
- `internal/config/config_types.go`
- `internal/config/config_defaults.go`
- `internal/config/config_accessors.go`
- `cmd/gromit/verify_spec.go` (if reused shared merge-stage helpers are extracted)

**Files to Create:**
- `internal/runner/specbranch/router.go`
- `internal/runner/specbranch/router_test.go`
- `internal/runner/specmerge/pipeline.go`
- `internal/runner/specmerge/pipeline_test.go`
- `internal/runner/specmerge/review_stages.go`
- `internal/runner/specmerge/review_stages_test.go`
- `internal/runner/specmerge/merge_ops.go`
- `internal/runner/specmerge/merge_ops_test.go`

**Tradeoffs:**
- **Runner-owned routing over epilogue-owned trigger:** Chosen because branch isolation must happen before build, not after completion.
- **Strict full restart from Stage 1 after fixes:** Chosen for regression safety over runtime savings.
- **Reuse existing review + synthesis primitives:** Chosen to reduce risk and keep behavior consistent with current review/fix semantics.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** branch routing decisions, gate sequencing, retry accounting, conflict outcome classification.
2. **Integration Tests:** orchestrator + temp git repository fixtures validating branch creation/switch/rebase/merge/delete behavior.
3. **Acceptance Tests:** mixed queue scenarios, hard-gate failures, fix-loop retries, and successful merge-to-main lifecycle.

**Risk-Driven Test Focus:**
- Wrong-branch execution prevention.
- No premature merge trigger while spec work remains.
- Strict hard-gate sequencing (no later stage runs after failure).
- Retry restarts from Stage 1 every time.
- Conflict resolution fallback correctness and safe blocking when unresolved.
- Non-spec beads remain on `main` with no branch side effects.

**Key Test Cases:**
- Spec bead executes on `gromit/spec-<name>` branch.
- Non-spec bead executes on `main`.
- Branch switching works when queue alternates across specs and non-spec.
- Pipeline triggers only when the last bead of the spec closes.
- Stage 1 uses full commands and includes acceptance/integration scope.
- Stage 2/3/4 pass/fail handling produces deterministic gate outcomes.
- Failed review stages create correctly labeled fix beads.
- Retry cap default (3) enforced with terminal user alert path.
- Successful pipeline rebases, fast-forwards, and deletes spec branch.
- Conflict unresolved path blocks merge and preserves branch for intervention.

**Mocking Strategy:**
- Mock git runner for deterministic branch/merge conflict cases.
- Mock review providers for stage pass/fail outcomes.
- Mock bead creation for fix synthesis assertions.
- Use real git in integration tests where branch graph correctness matters.

**Coverage Goals:**
- All stage-transition branches in merge pipeline.
- Full routing matrix (spec, non-spec, cross-spec switching).
- Retry-cap terminal behavior and alerting path.
- Conflict-resolution success/failure branches.

## Implementation Tasks

### Task 1: Add Config Surface for Spec Merge Pipeline

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add config for the new merge pipeline retry cap and optional stage toggles where needed, preserving safe defaults. Keep default retry cap at 3. Ensure config normalization and accessors are stable for nil/zero values.

**Acceptance Criteria:**
- Config exposes merge retry cap defaulting to 3.
- Zero/nil inputs resolve to safe defaults.
- YAML round-trip tests cover new fields.

**Dependencies:** None

### Task 2: Implement Spec Branch Router (Branch Resolution + Safety Guards)

**Files:**
- Create: `internal/runner/specbranch/router.go`
- Test: `internal/runner/specbranch/router_test.go`

**What to Do:**
Implement a small router that maps bead labels to execution branch names, validates/sanitizes spec names for branch naming, and returns `main` for non-spec beads. Include explicit safety checks to prevent accidental checkout to malformed branch names.

**Acceptance Criteria:**
- `spec:<name>` maps to `gromit/spec-<name>` deterministically.
- Non-spec beads map to `main`.
- Invalid/malformed spec labels are rejected safely.

**Dependencies:** Task 1

### Task 3: Add Spec Branch Lifecycle Git Operations

**Files:**
- Modify: `internal/worktree/worktree.go` (or create helper under `internal/runner/specbranch`)
- Test: `internal/worktree/worktree_test.go` and/or `internal/runner/specbranch/*_test.go`

**What to Do:**
Add operations for creating/checking out spec branches from current `main`, rebasing spec branches onto `main`, fast-forward merging to `main`, and deleting merged spec branches. Add careful conflict-path handling hooks.

**Acceptance Criteria:**
- First spec bead creates or checks out the spec branch from `main`.
- Rebase + FF merge APIs return clear typed errors for conflict vs generic failure.
- Branch deletion runs only after successful merge.

**Dependencies:** Task 2

### Task 4: Wire Branch Routing into Runner Iteration Flow

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/runner/constructor.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Before Build stage execution, resolve and checkout the correct branch per bead. Ensure branch switches occur when moving across specs or between spec/non-spec beads.

**Acceptance Criteria:**
- Every bead executes on its routed branch.
- Switching between `main` and spec branches is deterministic and tested.
- Runner fails safely if branch checkout cannot be completed.

**Dependencies:** Task 2, Task 3

### Task 5: Implement Spec Completion Detection and Trigger Control

**Files:**
- Create: `internal/runner/specmerge/pipeline.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/specmerge/pipeline_test.go`

**What to Do:**
Implement completion detection for specs (no remaining open/ready spec-labeled beads) and trigger merge pipeline exactly once per completion event.

**Acceptance Criteria:**
- Pipeline triggers only after the last bead of a spec closes.
- No trigger while any spec-labeled work remains.
- Duplicate trigger on same completion event is prevented.

**Dependencies:** Task 4

### Task 6: Implement Stage 1 Full Validation Gate

**Files:**
- Create/Modify: `internal/runner/specmerge/pipeline.go`
- Reuse/Modify: `internal/runner/validation/runner.go` as needed
- Test: `internal/runner/specmerge/pipeline_test.go`

**What to Do:**
Stage 1 must run full validation commands (no build-tag filtering), include linting, and include acceptance tests when ATDD is enabled. This stage is a hard gate for all later stages.

**Acceptance Criteria:**
- Stage 1 uses `FullCommandsOrDefault()` and blocks on any failure.
- Stage 2+ are skipped when Stage 1 fails.
- Stage output includes actionable failure context for fix-bead synthesis.

**Dependencies:** Task 5

### Task 7: Implement Stages 2-4 Blocking Reviews

**Files:**
- Create: `internal/runner/specmerge/review_stages.go`
- Test: `internal/runner/specmerge/review_stages_test.go`
- Modify wiring: `internal/runner/constructor_adapters.go`

**What to Do:**
Implement three blocking review stages on full diff (`main...spec-branch`):
- Stage 2 spec conformance against acceptance criteria
- Stage 3 code quality
- Stage 4 architecture
Each stage produces pass/fail with findings.

**Acceptance Criteria:**
- Review stages run in strict order.
- Failure in any stage prevents subsequent stages.
- Findings are structured for fix-bead creation.

**Dependencies:** Task 6

### Task 8: Implement Fix-Bead Loop and Retry Cap

**Files:**
- Modify: `internal/runner/specmerge/pipeline.go`
- Reuse: `internal/specgate/synthesize.go` (no behavior regressions)
- Test: `internal/runner/specmerge/pipeline_test.go`

**What to Do:**
On stage failure, synthesize spec-labeled fix beads and return control to run loop. When fixes complete, restart merge pipeline from Stage 1. Enforce retry cap default 3 with clear terminal alert.

**Acceptance Criteria:**
- Failures create fix beads with matching `spec:<name>` label.
- Retry always restarts at Stage 1.
- Retry cap is enforced and emits explicit stop/alert behavior.

**Dependencies:** Task 7

### Task 9: Implement Rebase, Conflict Resolution, and Merge Finalization

**Files:**
- Create: `internal/runner/specmerge/merge_ops.go`
- Test: `internal/runner/specmerge/merge_ops_test.go`
- Modify wiring: `internal/runner/constructor_adapters.go`

**What to Do:**
After all stages pass: rebase spec branch onto latest `main`, attempt fast-forward merge, invoke LLM-assisted conflict resolver when needed, and block with clear alert if unresolvable. Delete spec branch after successful merge only.

**Acceptance Criteria:**
- Clean rebase+FF merge path succeeds and deletes branch.
- Conflict path invokes resolver and retries merge when resolved.
- Unresolvable conflicts block merge and preserve branch for intervention.

**Dependencies:** Task 8

### Task 10: Replace Legacy Spec Gate Trigger and Add End-to-End Tests

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Modify: `internal/runner/constructor.go`
- Test: `internal/runner/acceptance/*` (new/updated)

**What to Do:**
Remove/disable old epilogue spec-gate trigger path in spec-mode runs and ensure new merge pipeline is the only completion mechanism for spec work. Add acceptance coverage for mixed queues and full success/failure loops.

**Acceptance Criteria:**
- Legacy spec gate no longer controls spec completion in this flow.
- Acceptance suite covers spec/non-spec routing and merge lifecycle.
- No regression for non-spec bead behavior on `main`.

**Dependencies:** Task 9

### Task 11: Quality Gates and Regression Verification

**Files:**
- No source changes expected (unless fixes required)

**What to Do:**
Run targeted and broad tests for runner, specmerge/specbranch, worktree, reviewpkg, and config to verify branch safety and gate correctness.

**Acceptance Criteria:**
- New unit/integration/acceptance tests pass.
- Existing runner/worktree/specgate/review suites remain green.
- Any discovered follow-up work is filed as linked beads.

**Dependencies:** Task 10

---

## Notes

- Highest-risk areas (branch safety, gate ordering, conflict handling) are intentionally front-loaded in design and deeply covered by tests.
- Keep merge operations conservative: no destructive git operations and no branch deletion unless merge is confirmed.
- If conflict auto-resolution fails, prefer explicit block + user alert over unsafe heuristics.
- During implementation, any newly discovered gaps should be tracked as `bd` issues with `discovered-from` links.
