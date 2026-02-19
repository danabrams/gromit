---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T18:33:01Z"
id: retro-review-worktree-launch-dir
source_spec: retro-review-worktree-launch-dir
---

# Retro/Review Worktree Launch Directory Implementation Plan

**Goal:** Make `gromit retro` and interactive `gromit review` launch agents in the interactive worktree only when the run loop is actively running.

**Architecture:** Resolve launch directory at the command launch boundary using existing run-loop detection (`worktree.IsRunLoopActive`) and switch interactive launch calls from `Launch` to `LaunchInDir` with either a worktree path or empty override.

**Tech Stack:** Go, Cobra CLI, existing `internal/agent`, `internal/pipeline`, and `internal/worktree` packages.

**Spec:** `.gromit/specs/retro-review-worktree-launch-dir.md`

---

## Architecture

**Overview:**
Implement a narrow wiring change for two interactive paths (`retro` and interactive `review`) without altering worktree lifecycle or non-interactive behavior.

**Key Components:**
1. **Launch dir resolver (`cmd/gromit`)**: Shared helper to decide launch dir from run-loop activity.
2. **Retro launch wiring (`cmd/gromit/main.go`)**: Use resolved dir for interactive retro agent launch.
3. **Interactive review wiring (`cmd/gromit/review.go` + `internal/pipeline/pipeline.go`)**: Thread launch dir through review input and launch with `LaunchInDir`.
4. **Existing detection (`internal/worktree/detect.go`)**: Reuse `IsRunLoopActive(gromitDir)` semantics as-is.

**Integration Points:**
- `runRetro` currently launches with `selectedAgent.Launch(promptPath)` and will move to `LaunchInDir(promptPath, launchDir)`.
- `pipeline.ReviewInteractive` currently launches with `agent.Launch(promptPath)` and will move to `LaunchInDir(promptPath, input.LaunchDir)`.
- Pipeline `Agent` interface and `ReviewInput` will be expanded to support directory-aware launch.

**Data Flow:**
1. Command prepares prompt exactly as today.
2. Command resolves `launchDir`:
   - run loop active: interactive worktree directory (non-empty)
   - run loop inactive/stale/missing/invalid: empty string
3. Interactive launch uses `LaunchInDir(promptPath, launchDir)`.

**Files to Modify:**
- `cmd/gromit/main.go` - retro interactive launch call and launch-dir resolution hook.
- `cmd/gromit/review.go` - compute launch dir and pass into pipeline review input.
- `internal/pipeline/pipeline.go` - add `LaunchInDir` to pipeline agent interface, add `LaunchDir` to `ReviewInput`, and launch using directory-aware method.
- `cmd/gromit/retro_agent_test.go` - update launch call expectation.
- `cmd/gromit/review_agent_test.go` - update interactive launch expectation.
- `internal/pipeline/review_test.go` - assert `LaunchInDir` path is used.

**Files to Create:**
- `cmd/gromit/interactive_launch_dir.go` - helper that maps run-loop activity to launch dir and keeps retro/review behavior consistent.
- `cmd/gromit/interactive_launch_dir_test.go` - helper behavior tests.

**Tradeoffs:**
- **Resolver in command layer vs pipeline layer:** command layer chosen to keep pipeline generic and avoid embedding run-loop/worktree policy into pipeline internals.
- **Deterministic worktree path vs worktree manager lifecycle calls:** deterministic path chosen to stay within spec scope (no create/cleanup policy changes).
- **Shared helper vs duplicated checks:** shared helper reduces future drift between retro and review semantics.

## Test Strategy

**Test Levels:**
1. **Unit tests (helper):** validate launch-dir selection for active/inactive/stale/missing status.
2. **Workflow tests (pipeline):** verify interactive review uses `LaunchInDir` and receives passed launch dir.
3. **Command integration-style tests:** verify retro/review interactive paths invoke directory-aware launch and non-interactive paths remain unaffected.

**Key Test Cases:**
- Active run loop (`running: true` + live PID) returns non-empty launch dir.
- Inactive or invalid status (missing file, malformed JSON, dead PID, `running: false`) returns empty launch dir.
- Retro interactive uses `LaunchInDir` with resolved dir.
- Interactive review passes `LaunchDir` into pipeline and pipeline launches with `LaunchInDir`.
- `retro --non-interactive` remains unchanged and bypasses interactive launch.
- Review non-interactive flow remains unchanged.

**Mocking Strategy:**
- Keep `worktree.IsRunLoopActive` as canonical detection implementation; helper tests can inject an activity function seam if needed.
- Extend existing pipeline agent mocks to record `LaunchInDir` arguments.
- Prefer existing command acceptance/source-assertion tests for lightweight wiring checks.

**Coverage Goals:**
- Explicitly cover all acceptance criteria for retro/review active vs inactive launch-dir behavior.
- Ensure no behavioral changes are introduced for non-interactive review and non-interactive retro mode.

**Test Organization:**
- `cmd/gromit/interactive_launch_dir_test.go` for launch-dir resolution.
- `internal/pipeline/review_test.go` for `LaunchInDir` behavior in `ReviewInteractive`.
- Existing `cmd/gromit/*_agent_test.go` files for launch-path wiring assertions.

## Implementation Tasks

### Task 1: Add shared interactive launch-dir resolver

**Files:**
- Create: `cmd/gromit/interactive_launch_dir.go`
- Test: `cmd/gromit/interactive_launch_dir_test.go`

**What to Do:**
Implement a helper that takes gromit/main directory context and returns launch directory based on run-loop activity:
- active run loop: interactive worktree path
- inactive/stale/missing/invalid: `""`
Use `worktree.IsRunLoopActive(gromitDir)` for activity checks.

**Acceptance Criteria:**
- Helper returns non-empty worktree dir only when run loop is active.
- Helper returns empty dir for all inactive/error-like statuses.
- Helper does not change any worktree lifecycle behavior (no create/cleanup).

**Dependencies:**
- None

### Task 2: Wire retro interactive launch to use LaunchInDir

**Files:**
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/retro_agent_test.go`

**What to Do:**
In `runRetro`, keep analysis and prompt generation unchanged. For interactive path only, resolve launch dir using new helper and replace `selectedAgent.Launch(promptPath)` with `selectedAgent.LaunchInDir(promptPath, launchDir)`.

**Acceptance Criteria:**
- Interactive retro uses non-empty launch dir only when run loop is active.
- Interactive retro uses empty launch dir when run loop inactive.
- `--non-interactive` retro path remains unchanged.
- Existing retro state recording (`RecordRetro`) remains unchanged.

**Dependencies:**
- Task 1

### Task 3: Add directory-aware launch to interactive review pipeline

**Files:**
- Modify: `cmd/gromit/review.go`
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/review_test.go`
- Test: `cmd/gromit/review_agent_test.go`

**What to Do:**
Add `LaunchDir` to `pipeline.ReviewInput`. Extend pipeline `Agent` interface with `LaunchInDir(promptPath, dir string) error`. In `ReviewInteractive`, call `agent.LaunchInDir(promptPath, input.LaunchDir)`. In CLI review path, resolve launch dir (helper from Task 1) and pass it in input.

**Acceptance Criteria:**
- Interactive review launches with `LaunchInDir`.
- Active run loop -> non-empty launch dir passed through to pipeline.
- Inactive/stale/missing run loop -> empty launch dir passed through.
- Non-interactive review path remains unchanged.

**Dependencies:**
- Task 1

### Task 4: Regression validation and scope lock

**Files:**
- Modify: impacted tests only as needed across `cmd/gromit` and `internal/pipeline`

**What to Do:**
Run targeted tests for retro/review interactive paths, pipeline review workflow, and helper resolution behavior. Confirm no functional change to non-interactive review and non-interactive retro.

**Acceptance Criteria:**
- All touched tests pass.
- Acceptance criteria from spec are covered by automated tests.
- No command behavior changes outside `retro` and interactive `review`.

**Dependencies:**
- Task 2
- Task 3

---

## Notes

- Keep this plan intentionally narrow: no changes to worktree merge policy, session creation/cleanup policy, or additional commands (`refine`, `plan`, `debug`, `explore`, `decompose`).
- Preserve existing prompt rendering and analysis behavior; only launch directory selection and launch method invocation should change.
