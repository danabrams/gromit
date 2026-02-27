---
id: spec-pr-review-gate
source_spec: spec-pr-review-gate
created: 2026-02-27
decomposed: false
---

# Spec PR Review Gate Implementation Plan

**Goal:** Implement spec-completion via GitHub PRs with CI/review/fix-loop gates, non-blocking approval handling, and a terminal `gromit prs` dashboard.

**Architecture:** Wire branch-per-spec routing into orchestrator execution, inject a real spec merge controller, and route all GitHub operations through a `PRClient` interface (with `gh` CLI implementation first) so pipeline logic is decoupled from transport details.

**Tech Stack:** Go, Cobra CLI, existing runner/pipeline framework, `gh` CLI via command execution, bd tracker integration.

**Spec:** `.gromit/specs/spec-pr-review-gate.md`

---

## Architecture

## Architecture Proposal (approved)

**Overview:**  
Implement a transport-abstracted PR pipeline: orchestrator/spec wiring stays the same, and all GitHub operations go through a `PRClient` interface with a `GhCLIClient` implementation first. This keeps auth simple now and enables a later REST client swap without touching orchestration logic.

**Key Components:**
1. **Orchestrator routing/wiring**: branch resolution + checkout per bead, `SpecMergeController` injection, and non-blocking PR poll call.
2. **`PRClient` interface (`internal/runner/specmerge`)**: typed methods for create PR, checks, reviews, comments, merge, and status fetch.
3. **`GhCLIClient` implementation**: wraps `gh` commands and parses structured output (`--json` where possible).
4. **Spec merge pipeline**: uses `PRClient` only (never shelling `gh` directly outside client).
5. **Spec PR state store**: persists PR number, gate status, fix cycles, and awaiting-approval state.
6. **`gromit prs` command**: reads state + `PRClient` status; merge action calls `PRClient.Merge`.

**Interface boundary (core methods):**
- `CreatePR(ctx, specName, head, base, title, body) (PRRef, error)`
- `GetPR(ctx, prNumber) (PRStatus, error)`
- `ListChecks(ctx, prNumber) ([]CheckStatus, error)`
- `PostReview(ctx, prNumber, review ReviewPayload) error`
- `PostComment(ctx, prNumber, body string) error`
- `RequestReviewers(ctx, prNumber, reviewers []string) error`
- `MergePR(ctx, prNumber, method string) error`

**Why this is the right split:**
- Pipeline code remains deterministic and testable with fake `PRClient`.
- Auth remains easiest now via `gh` credential flow.
- Future REST migration is isolated to a new `RESTClient` implementing the same interface.

---

## Test Strategy

## Test Strategy (approved)

**Test Levels:**
1. **Unit Tests**
- Branch routing decisions (`spec:<name>` -> `gromit/spec-<name>`, non-spec -> base branch).
- Orchestrator branch checkout sequencing (checkout occurs after Gate proceed, before Build).
- `PRClient` contract tests against a fake client for pipeline behavior.
- `GhCLIClient` command construction and JSON parsing (success/failure/partial data).
- Spec merge state transitions (created, fixing, ready_for_review, awaiting_approval, merged, closed, manual_intervention).
- Fix-loop logic (failure aggregation, bead creation, cycle cap behavior).

2. **Integration Tests**
- `specmerge.Pipeline.Trigger()` end-to-end with fakes for tracker/git/pr client/renderer.
- Constructor wiring test proving `SpecMergeController` is non-nil in spec mode.
- Orchestrator test proving branch switches correctly across bead sequence: same spec, different spec, non-spec.
- Poller integration tests for PR status handling (`merged`, `approved`, `changes_requested`, `closed`, open with pending CI).

3. **Acceptance / CLI Tests**
- `gromit prs` list output columns/status mapping.
- `gromit prs <spec>` detail rendering includes summary/checks/review stages/fix beads/cycle.
- `gromit prs <spec> --merge` enabled/disabled gating rules.
- Non-blocking behavior: run loop continues processing other beads while PRs await approval.

**Key Test Cases:**
- Spec bead runs on spec branch; next non-spec bead returns to main.
- Last bead closure triggers exactly one PR creation and state record.
- CI failure creates fix beads and reruns full validation/review pipeline.
- Review stage fail in stage 3 still restarts from CI after fixes.
- Retry cap reached posts summary + marks manual intervention.
- Approved-but-unmerged PR auto-merges only when `auto_merge_on_approval=true`.
- `max_open_prs=1` queues new spec PR creation.
- CLI merge action blocked when any gate condition unmet.

**Mocking Strategy:**
- Mock `PRClient`, tracker bead query/creator, git ops, diff provider, review renderer/provider router.
- Real filesystem-backed state store in temp dir (to test persistence/rehydration).
- No live GitHub network calls in test suite.

**Coverage Goals:**
- All branch-routing paths and PR state transitions.
- All failure-entry points into fix loop (CI, conformance, code-quality, architecture).
- CLI gating correctness and user-facing status determinism.

**Test Organization:**
- `internal/runner/specmerge/*_test.go`: pipeline, poller, state store, gh client parse tests.
- `internal/runner/orchestrator_test.go`: branch checkout + trigger/poll orchestration.
- `internal/runner/acceptance/*`: constructor wiring and non-blocking run-loop behavior.
- `cmd/gromit/prs_test.go`: list/detail/merge UX and guardrails.

---

## Implementation Tasks

### Task 1: Add `spec_pr` configuration surface

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `SpecPRConfig` to top-level config with all spec-defined fields (`enabled`, `reviewers`, `merge_method`, `fix_cycle_cap`, `auto_fix_human_comments`, `auto_merge_on_approval`, `ci_poll_interval`, `ci_timeout`, `max_open_prs`). Add defaults and validation-friendly normalization behavior consistent with existing config patterns.

**Acceptance Criteria:**
- Config parser accepts `spec_pr` block with all fields.
- Missing values resolve to documented defaults.
- Backward compatibility remains intact for configs without `spec_pr`.

**Dependencies:**
- None.

**Notes:**
- Keep merge-method values as constrained strings at validation points.

### Task 2: Wire branch routing into orchestrator

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/runner/specbranch/git_ops.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Add branch routing support to `OrchestratorConfig` (router + git checkout dependency). Resolve branch from bead labels after Gate `Proceed` and before Build. Ensure transitions across spec/non-spec/different-spec beads check out correct branches every iteration.

**Acceptance Criteria:**
- `spec:<name>` beads run on `gromit/spec-<name>`.
- Non-spec beads run on base branch (default `main`).
- Checkout is executed before Build on every proceeded iteration.

**Dependencies:**
- Task 1 (for base branch and spec_pr gating consistency in wiring code paths).

**Notes:**
- Keep behavior no-op-safe when router/git dependency is absent in non-spec runs.

### Task 3: Inject `SpecMergeController` in constructor

**Files:**
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/constructor_adapters.go`
- Test: `internal/runner/acceptance/constructor_spec_merge_wiring_test.go`

**What to Do:**
Construct and inject a real `specmerge.Pipeline` into `OrchestratorConfig.SpecMergeController` when spec granularity + `spec_pr.enabled` are active. Reuse existing adapter patterns for router/render/git/cmd dependencies.

**Acceptance Criteria:**
- Constructor sets non-nil `SpecMergeController` in enabled spec mode.
- Existing non-spec and disabled paths remain unchanged.
- Acceptance test proves wiring occurs and legacy spec gate remains unwired.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- Keep constructor composition readable; avoid putting full pipeline logic in constructor.

### Task 4: Define transport abstraction for PR operations

**Files:**
- Create: `internal/runner/specmerge/pr_client.go`
- Create: `internal/runner/specmerge/pr_client_test.go`

**What to Do:**
Define `PRClient` interface and typed DTOs (`PRRef`, `PRStatus`, `CheckStatus`, `ReviewPayload`, merge precondition signals). Include small contract tests for fake client behavior to anchor pipeline expectations.

**Acceptance Criteria:**
- Pipeline can depend exclusively on `PRClient` interface types.
- Types are sufficient to express create/check/review/comment/request-review/merge flows.
- Contract tests cover expected state transitions and error propagation semantics.

**Dependencies:**
- Task 1

**Notes:**
- Keep interface minimal but complete for current spec requirements.

### Task 5: Implement `gh`-backed PR client

**Files:**
- Create: `internal/runner/specmerge/gh_client.go`
- Create: `internal/runner/specmerge/gh_client_test.go`

**What to Do:**
Implement `PRClient` via `gh` CLI command execution and structured JSON parsing (`gh pr create`, `gh pr checks`, `gh pr view`, `gh pr review`, `gh api`, `gh pr merge`). Normalize command outputs into typed statuses used by pipeline and CLI.

**Acceptance Criteria:**
- `GhCLIClient` satisfies `PRClient`.
- Parsing handles pass/fail/running/pending checks and review states.
- Error messages include command context and preserve stderr details.

**Dependencies:**
- Task 4

**Notes:**
- Prefer `--json` output to reduce fragile text parsing.

### Task 6: Add spec PR state persistence and polling model

**Files:**
- Create: `internal/runner/specmerge/pr_state_store.go`
- Create: `internal/runner/specmerge/poller.go`
- Test: `internal/runner/specmerge/poller_test.go`

**What to Do:**
Create persistent spec PR state store and poller logic that updates state between iterations. Track PR number, gate stage results, fix cycle count, awaiting approval, and terminal states (merged/closed/manual intervention).

**Acceptance Criteria:**
- State survives process restarts and can rehydrate open spec PR tracking.
- Poller handles merged/approved/changes-requested/closed/still-open outcomes.
- `max_open_prs` policy is enforceable from stored/live state.

**Dependencies:**
- Task 1
- Task 4
- Task 5

**Notes:**
- State schema should be forward-compatible for future REST transport metadata.

### Task 7: Implement `specmerge.Trigger()` full PR pipeline

**Files:**
- Modify: `internal/runner/specmerge/pipeline.go`
- Create: `internal/runner/specmerge/pr_summary.go`
- Test: `internal/runner/specmerge/pipeline_test.go`

**What to Do:**
Replace `Trigger` stub with real orchestration: push spec branch, create PR, generate factual PR summary from diff+spec context, run CI gate + 3 review gates, create fix beads on failures, rerun full gates after fixes, enforce cycle cap, and post status comments.

**Acceptance Criteria:**
- Trigger creates PR when spec completion is detected and records PR number.
- CI and all 3 review stages run in hard-gate order.
- Failures create spec-labeled fix beads and restart validation from CI.
- Retry cap posts remaining-failures summary and sets manual intervention state.

**Dependencies:**
- Task 3
- Task 4
- Task 5
- Task 6

**Notes:**
- Keep stage logic composable so future gate additions are straightforward.

### Task 8: Hook non-blocking PR polling into orchestrator loop

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/orchestrator_test.go`
- Test: `internal/runner/acceptance/spec_mixed_queue_acceptance_test.go`

**What to Do:**
Add between-iteration polling call for open spec PRs while preserving run-loop progress on other beads. Ensure pending approvals do not block unrelated work.

**Acceptance Criteria:**
- Run loop continues processing available beads while spec PRs are awaiting review/approval.
- Polling executes between iterations without preventing bead execution.
- Polling failures are logged as warnings and do not crash the run loop.

**Dependencies:**
- Task 6
- Task 7

**Notes:**
- Keep poll interval/config checks centralized to avoid drift.

### Task 9: Add `gromit prs` command (list/detail)

**Files:**
- Create: `cmd/gromit/prs.go`
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/prs_test.go`

**What to Do:**
Implement `gromit prs` and `gromit prs <spec-name>` views using stored state + live `PRClient` status. Render required columns and detail sections per spec.

**Acceptance Criteria:**
- List view shows spec, PR, CI, review gate status, fix bead count, and human-readable status.
- Detail view includes PR summary, checks, per-stage review status, fix beads, cycle info, and PR link.
- Output remains deterministic in tests.

**Dependencies:**
- Task 6
- Task 7

**Notes:**
- Reuse existing CLI formatting conventions from `status`/`board` where practical.

### Task 10: Add merge action and guardrails to `gromit prs`

**Files:**
- Modify: `cmd/gromit/prs.go`
- Test: `cmd/gromit/prs_test.go`

**What to Do:**
Implement `gromit prs <spec-name> --merge` using configured merge method via `PRClient.MergePR`, with strict enablement rules (zero in-progress fix beads, CI pass, 3/3 reviews pass).

**Acceptance Criteria:**
- Merge command runs only when all merge preconditions are met.
- Blocked merge explains which condition is unmet.
- Successful merge updates local spec PR state and branch-cleanup status.

**Dependencies:**
- Task 9

**Notes:**
- Keep no-force behavior explicit in UX.

### Task 11: End-to-end acceptance and regression hardening

**Files:**
- Create: `internal/runner/acceptance/spec_pr_review_gate_acceptance_test.go`
- Modify: `internal/runner/acceptance/constructor_spec_merge_wiring_test.go`
- Modify: `cmd/gromit/help_goldens_test.go`

**What to Do:**
Add acceptance coverage for the full PR gate lifecycle and CLI command surfacing, including non-blocking loop behavior and sequential PR policy when `max_open_prs: 1`.

**Acceptance Criteria:**
- Acceptance suite verifies core lifecycle from spec completion to ready-for-human-review state.
- Command help/golden outputs include `prs` command.
- Existing behavior for non-spec workflows remains unchanged.

**Dependencies:**
- Tasks 2 through 10

**Notes:**
- Keep tests fake-driven; avoid requiring live GitHub connectivity in CI.

---

## Notes

- `PRClient` must remain the only GitHub transport boundary in spec merge code; do not leak `gh` command details into orchestration logic.
- Prefer incremental rollout flags: gate full behavior on `spec_pr.enabled` and preserve current defaults.
- Ensure branch operations are idempotent and tolerant of already-checked-out state.
- Keep plan decomposition-friendly: most tasks map to 1-3 beads each.
