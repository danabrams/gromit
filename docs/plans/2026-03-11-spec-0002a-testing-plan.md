# Spec 0002a -- Testing and Verification Plan

Core Execution Loop and Deterministic Validation for Gromit Next.

Seven new packages under `internal/next/`: `execpolicy`, `runstore`, `planner`, `executor`, `validator`, `evidence`, `specloop`. Plus CLI commands under `cmd/gromit/` and provider extension.

## Overview

This document defines every test required to satisfy the Spec 0002a evidence requirements. Each evidence item maps to at least one named test function. All tests are deterministic -- no network calls, no timing dependencies, no flaky assertions.

## Test Strategy

### Unit test approach

- Table-driven tests for all parsing, validation, and state-transition logic.
- Test files co-located with implementation: `foo.go` -> `foo_test.go`.
- Each test function name describes the behavior under test, not the implementation detail.
- Assertions use `t.Errorf` / `t.Fatalf` with descriptive messages; no assertion libraries.

### Integration test approach

- Build tag `//go:build integration` on all integration test files.
- Integration tests use real filesystem via `t.TempDir()`.
- Integration tests wire real packages together but stub the LLM provider.
- Run separately: `go test -tags integration ./internal/next/...`

### Fixture/fake strategy

All external boundaries are interfaces. Tests inject fakes:

| Boundary | Interface | Fake |
|----------|-----------|------|
| LLM provider | `AgentInvoker` | `FakeAgent` -- returns canned JSON, records calls |
| Git operations | `GitClient` | `FakeGit` -- operates on temp dirs, tracks commits |
| Filesystem | stdlib `os` | Real filesystem via `t.TempDir()` |
| Clock | `Clock` | `FakeClock` -- deterministic time control |
| Command runner | `CmdRunner` | `FakeCmdRunner` -- returns preconfigured exit codes and stdout |

All fakes live in `internal/next/testutil/`.

```go
// internal/next/testutil/fake_agent.go
type FakeAgent struct {
    Responses []string // returned in order per call
    Calls     []FakeAgentCall
}

type FakeAgentCall struct {
    Prompt  string
    Model   string
    Timeout time.Duration
}

// internal/next/testutil/fake_clock.go
type FakeClock struct {
    Now time.Time
}

func (c *FakeClock) CurrentTime() time.Time { return c.Now }
func (c *FakeClock) Advance(d time.Duration) { c.Now = c.Now.Add(d) }

// internal/next/testutil/fake_cmd_runner.go
type FakeCmdRunner struct {
    Results map[string]CmdResult // keyed by command string
}

type CmdResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
}
```

## Package-by-Package Test Coverage

### execpolicy/ tests

File: `internal/next/execpolicy/execpolicy_test.go`

**`TestLoadFromFile_ValidJSON`**
- Input: JSON file with all fields populated.
- Assert: All fields match. No error.

**`TestLoadFromFile_FileNotFound_ReturnsDefaults`**
- Input: Non-existent path.
- Assert: Returns default policy. `MaxTaskRetries == 1`, `MaxRedecompositionPasses == 1`, `MaxSpecCycles == 3`, `AlwaysRun` is non-empty default list (note: assumes Go project; see I5), `MaxRunDurationSeconds > 0`, `MaxRunCostUSD > 0`.

**`TestLoadFromFile_PartialConfig_MergesDefaults`**
- Input: JSON with only `max_task_retries: 5` set.
- Assert: `MaxTaskRetries == 5`, all other fields take defaults.

**`TestValidate_NegativeBudget_ReturnsError`**
- Input: Policy with `MaxRunCostUSD: -1`.
- Assert: Returns validation error mentioning "cost".

**`TestValidate_ZeroMaxRetries_Accepted`**
- Input: Policy with `MaxTaskRetries: 0`.
- Assert: No validation error. Zero is valid (means no repair attempts, initial attempt only).

**`TestValidate_EmptyAlwaysRun_ReturnsError`**
- Input: Policy with `AlwaysRun: []`.
- Assert: Returns validation error mentioning "always_run".
- Note (I5): Default `AlwaysRun` values in `TestLoadFromFile_FileNotFound_ReturnsDefaults` assume a Go project (e.g., `go test ./...`, `go vet ./...`). If the policy file is absent and the project language is unknown, defaults should either be empty (requiring explicit configuration) or clearly documented as Go-specific. Current test asserts non-empty defaults, which is Go-specific behavior.

**`TestValidate_ValidPolicy_NoError`**
- Table-driven with several valid configs.
- Assert: No error for each.

**`TestDefaults_NilFieldsNormalized`**
- Assert: `Defaults()` returns policy with no nil slices or maps.

### runstore/ tests

File: `internal/next/runstore/runstore_test.go`

**`TestCreateRunDir_CreatesExpectedStructure`**
- Assert: Creates `<runs-dir>/<run-id>/` with subdirs: `tasks/`, `evidence/`.
- Assert: `run.json` written with correct initial state.

**`TestCreateRunDir_RunIDIsUnique`**
- Create two runs in rapid succession.
- Assert: Different run IDs.

**`TestWriteReadRunJSON_RoundTrip`**
- Write a RunState, read it back.
- Assert: All fields match including `TerminalState`, `StartedAt`, `Tasks`.

**`TestAppendEvent_SingleEvent`**
- Append one event.
- Assert: `events.jsonl` contains exactly one line. Line parses to correct event.

**`TestAppendEvent_MultipleEvents_PreservesOrder`**
- Append 5 events of different types.
- Assert: File has 5 lines. Events read back in order.

**`TestAppendEvent_ConcurrentWrites`**
- Launch 20 goroutines each appending 10 events.
- Assert: File has exactly 200 lines. No corrupted JSON lines.

**`TestStoreTaskResult_CreatesTaskFile`**
- Store a task result with ID "task-003".
- Assert: `tasks/task-003.json` exists and round-trips correctly.

**`TestListRuns_EmptyDir_ReturnsEmpty`**
- Assert: Returns empty slice, no error.

**`TestListRuns_MultipleRuns_SortedByTime`**
- Create 3 runs with known timestamps.
- Assert: Returned in reverse chronological order.

**`TestListRuns_FiltersByProject`**
- Create runs for projects "alpha" and "beta".
- Assert: Listing for "alpha" returns only alpha runs.

**`TestUpdateTerminalState_WritesToRunJSON`**
- Create run, update terminal state to "needs_human" with blocker summary.
- Assert: Re-read `run.json` shows updated state and summary.

### validator/ tests

File: `internal/next/validator/validator_test.go`

**`TestRunCheck_PassingCommand`**
- FakeCmdRunner returns exit 0.
- Assert: `CheckResult.Passed == true`, `Output` captured.

**`TestRunCheck_FailingCommand`**
- FakeCmdRunner returns exit 1 with stderr.
- Assert: `CheckResult.Passed == false`, `FailureDetails` populated.

**`TestRunCheck_Timeout`**
- FakeCmdRunner configured to exceed timeout.
- Assert: Returns error indicating timeout. `CheckResult.Passed == false`.

**`TestRunAlwaysRunChecks_AllPass`**
- Policy has 3 always-run commands, all pass.
- Assert: `ValidationResult.Passed == true`, 3 check results.

**`TestRunAlwaysRunChecks_OneFailsDeterministic`**
- Second of 3 checks fails.
- Assert: `ValidationResult.Passed == false`. Failed check identified by name.

**`TestRunTargetedChecks_FromTaskProofPlan`**
- Task proof plan specifies `go test ./internal/refund/...`.
- Assert: Only that command runs. Result reflects pass/fail.

**`TestRunFinalValidation_CombinesAlwaysRunAndProjectCell`**
- Always-run has 2 checks, project cell validation has 1 check.
- Assert: All 3 run. Overall pass requires all 3 passing.

**`TestParseCheckOutput_ExtractsFailureLines`**
- Input: Multi-line test output with `FAIL` markers.
- Assert: `FailureDetails` contains file paths and line numbers from output.

**`TestDeterministicFailure_PreventsReadyForReview`**
- Validation fails on same check twice in a row (same output).
- Assert: Returns `DeterministicFailure == true`.
- **Evidence**: Requirement 9.

### planner/ tests

File: `internal/next/planner/planner_test.go`

**`TestParsePlannerOutput_ValidJSON`**
- Input: Well-formed JSON with tasks array.
- Assert: Parsed plan has correct task count, IDs, descriptions, affected packages.

**`TestParsePlannerOutput_InvalidJSON_ReturnsError`**
- Input: Malformed JSON string.
- Assert: Returns parse error.

**`TestParsePlannerOutput_EmptyTasks_ReturnsError`**
- Input: Valid JSON with `"tasks": []`.
- Assert: Returns validation error "plan must contain at least one task".

**`TestParsePlannerOutput_MissingRequiredFields`**
- Table-driven: missing `task_id`, missing `objective`, missing `expected_touched_area`, missing `proof_checks`.
- Assert: Each returns specific validation error.

**`TestParsePlannerOutput_DuplicateTaskIDs_ReturnsError`**
- Input: Two tasks with `id: "task-01"`.
- Assert: Returns error mentioning "duplicate".

**`TestRetryOnInvalidOutput_SucceedsOnSecondAttempt`**
- FakeAgent returns invalid JSON first, valid JSON second.
- Assert: Plan returned successfully. Agent called twice.

**`TestRetryOnInvalidOutput_ExhaustsRetries`**
- FakeAgent always returns invalid JSON. Max retries = 1 (spec says retry ONCE).
- Assert: Returns error after 2 attempts (1 initial + 1 retry).

**`TestFixPlanGeneration_IncludesFailureContext`**
- Input: Prior plan + validation failures for task-02.
- Assert: Fix plan prompt includes failure details. Fix plan only references task-02's scope.

**`TestFixPlanGeneration_DoesNotReplanCompletedTasks`**
- Input: tasks 01-03 completed, task-04 failed.
- Assert: Fix plan contains only remediation for task-04 scope.
- **Evidence**: Requirement 6.

**`TestPlanCreatedEvent_CarriesMetadata`**
- After planning completes, a `plan_created` event is emitted.
- Assert: Event metadata includes `cycle` number and `kind` ("initial" or "fix").

**`TestFixPlan_CarriesMetadata`**
- Input: Fix plan generated after cycle 1 validation failure.
- Assert: Fix plan output includes `cycle` (2), `kind` ("fix"), `parent_cycle` (1), and `failures_addressed` fields.

**`TestTaskIDSequencing_AcrossCycles`**
- First cycle produces task-01 through task-03. Second cycle (fix) should produce task-04+.
- Assert: No ID collisions. IDs are monotonically increasing.

**`TestPlanValidation_CrossCycleIDUniqueness`**
- Cycle 1 plan has tasks t-001 through t-004. Cycle 2 fix plan starts at t-005.
- If cycle 2 fix plan reuses t-001, validation fails.
- Assert: Duplicate ID across cycles returns validation error mentioning "duplicate".

**`TestPlanForRefundPackage_PacketExcludesUnrelated`**
- Input: Spec mentioning `internal/refund/`. Context packet compiled for this task.
- Assert: Packet sections do not include unrelated packages like `internal/billing/`.
- **Evidence**: Requirement 5.

### executor/ tests

File: `internal/next/executor/executor_test.go`

**`TestCompileTaskPacket_IncludesRequiredSections`**
- Assert: Packet contains doctrine, spec-text, proof-requirements, task-description sections.

**`TestCompileTaskPacket_ScopedToAffectedPackages`**
- Task affects `internal/refund/`.
- Assert: Packet context scoped to that subtree.
- **Evidence**: Requirement 5.

**`TestInvokeAgent_CapturesOutput`**
- FakeAgent returns implementation text.
- Assert: Output captured in task result. Agent call recorded with correct model tier.

**`TestInvokeAgent_Timeout`**
- FakeAgent configured to exceed timeout.
- Assert: Returns timeout error. Task marked as failed.
- **Evidence**: Requirement 13.

**`TestExtractResult_Success`**
- Agent output contains success markers and diff.
- Assert: `TaskResult.Status == "done"`, diff captured.

**`TestExtractResult_NeedsSplit`**
- Agent output indicates task too large.
- Assert: `TaskResult.Status == "needs_split"`.

**`TestNeedsSplitHeuristic_ThreePackages`**
- Git diff touches 3+ packages.
- Assert: `NeedsSplit() == true`.

**`TestNeedsSplitHeuristic_TwoTimesFileSpread`**
- Git diff touches 2x the expected file count for task scope.
- Assert: `NeedsSplit() == true`.

**`TestNeedsSplitHeuristic_SmallChange`**
- Git diff touches 1 package, 2 files.
- Assert: `NeedsSplit() == false`.

**`TestInspectWorktree_RunsGitDiffAndTargetedChecks`**
- FakeGit returns diff. FakeCmdRunner passes targeted checks.
- Assert: Inspection result includes diff summary and check results.

**`TestRepairLoop_SucceedsOnRetry`**
- First inspection fails (targeted check fails). Agent fixes. Second inspection passes.
- Assert: Task completes after one repair cycle.

**`TestRepairLoop_FailsAfterOneRetry`**
- Both inspections fail.
- Assert: Task marked as failed after exactly one repair attempt.

**`TestTimeoutEnforcement_StopsExecution`**
- FakeClock advances past task timeout mid-execution.
- Assert: Executor returns timeout error.
- **Evidence**: Requirement 13.

### evidence/ tests

File: `internal/next/evidence/evidence_test.go`

**`TestAssembleBundle_IncludesAllArtifacts`**
- Input: Completed run with 3 tasks, validation results.
- Assert: Bundle contains `review.md`, `metrics.json`, `diff-summary.md`.

**`TestGenerateReviewMD_ReadyForReview`**
- Input: Run with terminal state `ready_for_review`.
- Assert: `review.md` contains terminal state, per-task summaries, validation results, aggregate diff.

**`TestGenerateReviewMD_NeedsHuman_IncludesBlockerSummary`**
- Input: Run with terminal state `needs_human`, blocker summary.
- Assert: `review.md` contains blocker summary section.
- Assert: Blocker summary contains a "Recommended action:" substring per the spec's terminal state descriptions.
- **Evidence**: Requirement 11.

**`TestGenerateReviewMD_Blocked_IncludesBlockerSummary`**
- Input: Run with terminal state `blocked`, blocker summary.
- Assert: `review.md` contains blocker summary with reason.
- Assert: Blocker summary contains a "Recommended action:" substring per the spec's terminal state descriptions.
- **Evidence**: Requirement 11.

**`TestGenerateMetricsJSON_HasRequiredFields`**
- Input: Run with 2 tasks, each having 3 agent invocations.
- Assert: `metrics.json` contains `total_tokens`, `total_cost`, `total_duration`.
- Assert: Per-invocation entries have `tokens_in`, `tokens_out`, `duration_ms`, `model_tier`.
- **Evidence**: Requirement 12.

**`TestGenerateMetricsJSON_TimingsArePositive`**
- Assert: All `duration_ms` values are > 0.

**`TestGenerateDiffSummary_ListsChangedFiles`**
- Input: Run with known git diffs.
- Assert: `diff-summary.md` lists all changed files grouped by task.

**`TestBundleStoredOutsideTargetRepo`**
- Input: Target repo at `/tmp/test-repo`. Run artifacts dir at `/tmp/gromit-runs/`.
- Assert: No evidence files exist under `/tmp/test-repo/`.
- Assert: All evidence files exist under `/tmp/gromit-runs/<run-id>/evidence/`.
- **Evidence**: Requirement 4.

### specloop/ tests

File: `internal/next/specloop/specloop_test.go`

**`TestStagePipelineOrder`**
- Assert: Stages execute in order: Plan -> Execute -> Validate.
- Assert: Each stage receives output of previous stage.

**`TestSpecLoop_HappyPath_ReadyForReview`**
- FakeAgent produces valid plan. Executor completes all tasks. Validator passes.
- Assert: Terminal state is `ready_for_review`.
- Assert: Events log contains: `run_started`, `plan_created`, `task_created`, `task_started`, `task_completed` (per task), `final_validation_result`, `terminal_state`.
- **Evidence**: Requirements 1, 15.

**`TestSpecLoop_ValidationFailure_TriggersReplan`**
- First validation fails with specific failures. Fix plan generated. Second validation passes.
- Assert: Two planning cycles occurred. Second plan is a fix plan.
- Assert: Completed tasks from cycle 1 not replanned in cycle 2.
- **Evidence**: Requirement 6.

**`TestSpecLoop_BudgetExhausted_NeedsHuman`**
- Validation fails repeatedly until `max_spec_cycles` exhausted (validation fails, fix cycles exhaust).
- Assert: Terminal state is `needs_human`.
- Assert: Blocker summary describes validation failures.
- **Evidence**: Requirements 2, 11.

**`TestSpecLoop_InvalidPlannerOutput_Blocked`**
- FakeAgent returns invalid JSON for planning, exhausts retries.
- Assert: Terminal state is `blocked`.
- Assert: Blocker summary describes planner failure.
- **Evidence**: Requirements 3, 11.

**`TestSpecLoop_MaxTaskRetries_Respected`**
- Policy has `max_task_retries: 2`. Task fails on every attempt.
- Assert: Task attempted exactly 3 times (1 initial + 2 retries) then marked failed.
- **Evidence**: Requirement 7.

**`TestSpecLoop_MaxTaskRetries_Default`**
- Policy uses default `max_task_retries: 1`. Task fails on first attempt.
- Assert: Task attempted exactly 2 times (1 initial + 1 retry) then marked failed.
- **Evidence**: Requirement 7.

**`TestSpecLoop_MaxRedecompositionPasses_Respected`**
- Policy has `max_redecomposition_passes: 3`. Tasks return `needs_split` during ExecuteStage on each pass.
- Assert: Exactly 3 redecomposition passes occur. Fourth `needs_split` triggers task failure instead of redecomposition.
- **Evidence**: Requirement 8.

**`TestSpecLoop_MaxRedecompositionPasses_Default`**
- Policy uses default `max_redecomposition_passes: 1`. First task returns `needs_split` and is redecomposed. A second `needs_split` across any task triggers task failure.
- Assert: Exactly 1 redecomposition pass occurs. Second `needs_split` (from any task) marks that task as failed, not redecomposed.
- **Evidence**: Requirement 8.

**`TestSpecLoop_DeterministicValidationFailure_PreventsReadyForReview`**
- Validation returns identical failure on two consecutive cycles.
- Assert: Terminal state is NOT `ready_for_review`. State is `needs_human`.
- Assert: Blocker summary mentions deterministic failure.
- **Evidence**: Requirement 9.

**`TestSpecLoop_CostBudgetEnforced`**
- Policy has `max_run_cost_usd: 1.00`. FakeAgent tracks cumulative cost.
- Cost exceeds budget after second task.
- Assert: Execution stops. Terminal state is `blocked`. `budget_exceeded` event emitted. Blocker mentions cost.
- **Evidence**: Requirement 13.

**`TestSpecLoop_TimeoutEnforced`**
- Policy has `max_run_duration_seconds: 300`. FakeClock advances 6 minutes during execution.
- Assert: Execution stops. Terminal state is `blocked`. `budget_exceeded` event emitted. Blocker mentions timeout.
- **Evidence**: Requirement 13.

**`TestTimeout_TaskLevel_vs_RunLevel`**
- Setup: Policy with `max_run_duration_seconds: 1800` (run-level). Task-level timeout derived from remaining budget.
- Scenario A (task timeout): FakeClock advances past task timeout during a single task. Assert: That task is marked failed, but next task still executes.
- Scenario B (run timeout): FakeClock advances past run-level timeout between tasks. Assert: Execution stops entirely. Terminal state is `blocked`. No further tasks attempted.
- **Evidence**: Requirement 13.

**`TestConcurrentRuns_SameSpec_NoConflict`**
- Launch two specloop.Run calls concurrently for the same spec but different run IDs.
- Assert: Both runs get separate worktrees, separate run directories, and separate event logs.
- Assert: No file corruption or race conditions (run with `-race`).

**`TestExecution_ZeroRepoPollution`**
- Run a full execution (happy path) against a fixture project.
- After execution completes, run `git status` on the target repo.
- Assert: No new untracked files in the target repo's tracked tree. The only changes should be code modifications in the worktree, not in the main repo checkout.
- Assert: No run artifacts, evidence files, or event logs exist under the target repo directory.
- **Evidence**: Requirement 4.

**`TestInitStage_WorktreeBranchNaming`**
- Run InitStage with spec ID "feat-refund" and run ID "run-abc123".
- Assert: Worktree branch name matches format `gromit/spec-feat-refund-run-abc123`.

**`TestSpecLoop_CostEnforced_BetweenStages`**
- Policy has tight `max_run_cost_usd`. Cost exceeds budget after CompileStage.
- Assert: PlanStage does not run. Terminal state is `blocked`. `budget_exceeded` event emitted.

**`TestSpecLoop_StageError_Blocked_EvidenceStillRuns`**
- PlanStage returns an unexpected error (not a terminal state).
- Assert: Terminal state is `blocked`. Blocker summary includes the error.
- Assert: EvidenceStage still runs and produces `review.md` with the blocker summary.

**`TestFinalizeStage_WorktreePreservation`**
- Run finishes with `ready_for_review`: Assert worktree preserved for human review.
- Run finishes with `needs_human`: Assert worktree preserved for human intervention.
- Run finishes with `blocked`: Assert worktree preserved for inspection (design choice: safer to preserve for debugging; spec is silent on blocked-state cleanup).

**`TestSpecLoop_MaxSpecCycles_Exhausted_NeedsHuman`**
- Policy has `max_spec_cycles: 2`. Validation fails in cycle 1, fix cycle 2 also fails validation.
- Assert: Terminal state is `needs_human`.
- Assert: Exactly 2 Plan-Execute-Validate passes occurred.
- Assert: Blocker summary describes persistent validation failures.
- **Evidence**: Requirements 2, 11.

**`TestTaskLoop_ImplementInspectRepairDone`**
- Task implemented successfully. Inspection passes.
- Assert: Task result status is `done`. No repair cycle triggered.

**`TestTaskLoop_ImplementInspectRepairFailed`**
- Implementation done. Inspection fails. Repair fails.
- Assert: Task result status is `failed`. Exactly one repair attempt.

**`TestTaskLoop_NeedsSplit_TriggersRedecomposition`**
- Task returns `needs_split`.
- Assert: Task is split into sub-tasks. Sub-tasks added to queue.

**`TestRedecomposition_WithinExecuteStage`**
- One task fails with `needs_split`. Remaining tasks unaffected.
- Assert: Split task decomposed into 2+ sub-tasks. Original completed tasks untouched.

**`TestRedecomposition_GlobalBudget_AcrossTasks`**
- Policy has `max_redecomposition_passes: 1`. Task A returns `needs_split` and is redecomposed into sub-tasks.
- Task B also returns `needs_split`.
- Assert: Task B marked failed (global redecomposition budget already spent), not redecomposed.

**`TestRedecomposition_RevertsTaskChanges_BeforeSubTasks`**
- Task returns `needs_split` after touching files.
- Assert: `git checkout` called on the touched files before sub-tasks are queued.
- Assert: Sub-tasks start from clean worktree state.

**`TestRedecomposition_SubTaskCannotRetrigger`**
- Task A returns `needs_split`, redecomposed into sub-tasks A1, A2.
- Sub-task A1 returns `needs_split`.
- Assert: A1 marked failed, not re-decomposed. Sub-tasks of redecomposed tasks cannot trigger further redecomposition.

**`TestExecuteStage_AllTasksFailed_NeedsHuman`**
- All tasks in the plan fail execution.
- Assert: ExecuteStage returns `needs_human` terminal state.
- Assert: Blocker summary lists all failed tasks.

**`TestExecuteStage_PartialFailure_Continue`**
- 3 tasks in plan: task 1 succeeds, task 2 fails, task 3 succeeds.
- Assert: ExecuteStage continues past task 2 failure. Tasks 1 and 3 marked done, task 2 marked failed.
- Assert: Validation stage still runs on the partial results.

**`TestEventLog_ContainsAllSpecifiedTypes`**
- Run a complete loop that exercises all event paths (happy path with a fix cycle, a needs_split, and budget check).
- Assert: `events.jsonl` contains at least these event types:
  - `run_started`
  - `spec_packet_compiled`
  - `plan_created`
  - `plan_validation_result`
  - `task_created`
  - `task_started`
  - `task_completed`
  - `task_failed`
  - `task_validation_result`
  - `task_needs_split`
  - `redecomposition_triggered`
  - `final_validation_result`
  - `replan_triggered`
  - `budget_exceeded`
  - `terminal_state`
- **Evidence**: Requirement 15.

### CLI tests

File: `cmd/gromit/exec_spec_test.go`

**`TestExecSpecCommand_RequiresSpecFlag`**
- Run `exec spec` without `--spec` flag.
- Assert: Error message mentions required flag.

**`TestExecSpecCommand_DryRun_ProducesPlanOnly`**
- Run `exec spec --spec fixtures/sample.md --dry-run`.
- Assert: Run directory created with plan saved to `runs/<run-id>/plan.md`. No tasks executed. No agent invocations beyond planning.
- **Evidence**: Requirement 14.

**`TestExecSpecCommand_DryRun_NoSideEffects`**
- Run with `--dry-run`. Check target repo.
- Assert: No commits created. No files modified in target repo.
- **Evidence**: Requirement 14.

**`TestExecSpecCommand_WithPolicyFlag`**
- Run `exec spec --spec fixtures/sample.md --policy fixtures/policy.json`.
- Assert: Policy loaded from specified file.

**`TestExecSpecCommand_DefaultPolicy`**
- Run without `--policy`.
- Assert: Default policy used. No error.

File: `cmd/gromit/exec_show_test.go`

**`TestExecShowCommand_FormatsRunState`**
- Create a completed run. Run `exec show <run-id>`.
- Assert: Output includes terminal state, task summary, duration.

**`TestExecShowCommand_UnknownRunID`**
- Run `exec show nonexistent-run`.
- Assert: Error message mentions "not found".

File: `cmd/gromit/exec_list_test.go`

**`TestExecListCommand_EmptyRuns`**
- No runs exist.
- Assert: Output indicates no runs.

**`TestExecListCommand_ListsRuns`**
- Create 3 runs.
- Assert: All 3 listed with IDs, terminal states, timestamps.

File: `cmd/gromit/spec_list_test.go`

**`TestSpecListCommand_DerivesStatus`**
- Project has 5 specs exercising each status: one with all tasks done (`completed`), one mid-execution (`running`), one with `needs_human` terminal state (`needs_attention`), one approved with no run yet or whose prior run was rejected/abandoned (`ready`), one with no run history (`draft`).
- Assert: Status shows `completed`, `running`, `needs_attention`, `ready`, `draft` respectively.
- Note (I6): `completed` requires human acceptance per Spec 0002b. In 0002a scope, this status may map to `ready_for_review` instead, since there is no human acceptance workflow yet.

## Integration Test Scenarios

All integration tests in `internal/next/specloop/specloop_integration_test.go` with build tag `//go:build integration`.

### Scenario 1: Happy path -> ready_for_review

**`TestIntegration_HappyPath_ReadyForReview`**

Setup:
- Create two fixture projects (alpha, beta) in separate temp dirs, each with `go.mod`, a `main.go`, and a simple test file.
- Initialize git repos in both.
- Create project cells for both in a shared `projects/` temp dir.
- Write a spec file: "Add a helper function to `pkg/util/`".
- Wire FakeAgent to return valid plan (2 tasks), valid implementation, passing checks.

Execution:
- Run `specloop.Run(ctx, specPath, cell, policy)`.

Assertions:
- Terminal state == `ready_for_review`.
- `run.json` exists with completed state.
- `events.jsonl` has entries for every stage.
- `evidence/review.md` exists and mentions "ready_for_review".
- `evidence/metrics.json` has per-invocation token counts.
- All run artifacts stored under `projects/<cell>/runs/`, NOT under the target repo dir.

**Evidence**: Requirements 1, 4, 12, 15.

### Scenario 2: Fix cycle -> ready_for_review

**`TestIntegration_FixCycle_ReadyForReview`**

Setup:
- FakeAgent returns valid plan. First task completes. Validation fails on a targeted check.
- FakeAgent produces fix plan. Fix task completes. Validation passes.

Assertions:
- Terminal state == `ready_for_review`.
- Two planning cycles in events log.
- Fix plan only addresses the failed scope, not completed tasks.
- Completed tasks from cycle 1 not re-executed.

**Evidence**: Requirements 1, 6.

### Scenario 3: Budget exhausted -> needs_human

**`TestIntegration_BudgetExhausted_NeedsHuman`**

Setup:
- Policy: `max_spec_cycles: 2`.
- Validation fails on every cycle.

Assertions:
- Terminal state == `needs_human`.
- Exactly 2 Plan-Execute-Validate cycles occurred.
- Blocker summary in `run.json` describes the persistent failures.
- `evidence/review.md` includes blocker summary.

**Evidence**: Requirements 2, 11.

### Scenario 4: Invalid planner -> blocked

**`TestIntegration_InvalidPlanner_Blocked`**

Setup:
- FakeAgent always returns unparseable JSON for planning.
- Planner retries exhausted.

Assertions:
- Terminal state == `blocked`.
- Blocker summary mentions planner failure.
- No tasks executed.
- `evidence/review.md` includes blocker summary.

**Evidence**: Requirements 3, 11.

### Scenario 5: Multi-project isolation

**`TestIntegration_TwoProjects_Isolation`**

Setup:
- Two fixture projects: `alpha` (Go CLI app) and `beta` (Go library).
- Each has its own project cell.
- Run spec execution on `alpha`.

Assertions:
- Run artifacts stored under alpha's cell, not beta's.
- No files created or modified in beta's repo dir.
- No files created or modified in alpha's repo dir (all artifacts in cell).
- Alpha's validation commands do not reference beta's paths.

**Evidence**: Requirement 10.

### Scenario 6: Dry run

**`TestIntegration_DryRun_ProducesPlanOnly`**

Setup:
- Valid spec, valid project cell.
- Run with `DryRun: true`.

Assertions:
- Plan produced and returned.
- Run directory created with `plan.md` saved to `runs/<run-id>/plan.md`.
- No agent invocations for task execution.
- No git operations on target repo.
- FakeAgent called exactly once (for planning).

**Evidence**: Requirement 14.

### Scenario 7: Cost/timeout enforcement

**`TestIntegration_CostLimitStopsExecution`**

Setup:
- Policy: `max_run_cost_usd: 0.50`.
- FakeAgent reports `$0.20` cost per invocation. Plan has 5 tasks.

Assertions:
- Execution stops after 2-3 tasks (cumulative cost crosses $0.50).
- Terminal state == `blocked`.
- `budget_exceeded` event emitted.
- Blocker summary mentions cost limit.
- Remaining tasks not attempted.

**Evidence**: Requirement 13.

**`TestIntegration_TimeoutStopsExecution`**

Setup:
- Policy: `max_run_duration_seconds: 600`.
- FakeClock advances 3 minutes per task. Plan has 5 tasks.

Assertions:
- Execution stops after 3-4 tasks (cumulative time crosses 10 minutes).
- Terminal state == `blocked`.
- `budget_exceeded` event emitted.
- Blocker summary mentions timeout.

**Evidence**: Requirement 13.

## Evidence Checklist

Mapping each spec evidence requirement to the test(s) that satisfy it.

| # | Requirement | Test(s) |
|---|-------------|---------|
| 1 | Integration: end-to-end through `ready_for_review` | `TestIntegration_HappyPath_ReadyForReview`, `TestIntegration_FixCycle_ReadyForReview` |
| 2 | Integration: run ending in `needs_human` | `TestIntegration_BudgetExhausted_NeedsHuman`, `TestSpecLoop_BudgetExhausted_NeedsHuman`, `TestSpecLoop_MaxSpecCycles_Exhausted_NeedsHuman`, `TestExecuteStage_AllTasksFailed_NeedsHuman` |
| 3 | Integration: run ending in `blocked` | `TestIntegration_InvalidPlanner_Blocked`, `TestSpecLoop_InvalidPlannerOutput_Blocked`, `TestSpecLoop_CostBudgetEnforced`, `TestSpecLoop_TimeoutEnforced` |
| 4 | Run artifacts stored outside target repo | `TestBundleStoredOutsideTargetRepo`, `TestExecution_ZeroRepoPollution`, `TestIntegration_HappyPath_ReadyForReview` |
| 5 | Task touching `internal/refund/` gets scoped packet | `TestPlanForRefundPackage_PacketExcludesUnrelated`, `TestCompileTaskPacket_ScopedToAffectedPackages` |
| 6 | Fix-plan targets specific failures without replanning completed work | `TestFixPlanGeneration_DoesNotReplanCompletedTasks`, `TestFixPlan_CarriesMetadata`, `TestSpecLoop_ValidationFailure_TriggersReplan`, `TestIntegration_FixCycle_ReadyForReview` |
| 7 | Task retry count respects `max_task_retries` | `TestSpecLoop_MaxTaskRetries_Respected`, `TestSpecLoop_MaxTaskRetries_Default`, `TestRepairLoop_FailsAfterOneRetry` |
| 8 | Re-decomposition respects `max_redecomposition_passes` | `TestSpecLoop_MaxRedecompositionPasses_Respected`, `TestRedecomposition_GlobalBudget_AcrossTasks`, `TestRedecomposition_SubTaskCannotRetrigger` |
| 9 | Deterministic validation failure prevents `ready_for_review` | `TestDeterministicFailure_PreventsReadyForReview`, `TestSpecLoop_DeterministicValidationFailure_PreventsReadyForReview` |
| 10 | Two fixture projects demonstrating isolation | `TestIntegration_TwoProjects_Isolation` |
| 11 | `needs_human` and `blocked` include blocker summary | `TestGenerateReviewMD_NeedsHuman_IncludesBlockerSummary`, `TestGenerateReviewMD_Blocked_IncludesBlockerSummary`, `TestSpecLoop_BudgetExhausted_NeedsHuman`, `TestSpecLoop_InvalidPlannerOutput_Blocked`, `TestSpecLoop_StageError_Blocked_EvidenceStillRuns` |
| 12 | `metrics.json` has per-invocation token counts, timings, model tiers | `TestGenerateMetricsJSON_HasRequiredFields`, `TestIntegration_HappyPath_ReadyForReview` |
| 13 | Timeout/cost limits enforced | `TestSpecLoop_CostBudgetEnforced`, `TestSpecLoop_TimeoutEnforced`, `TestSpecLoop_CostEnforced_BetweenStages`, `TestInvokeAgent_Timeout`, `TestTimeoutEnforcement_StopsExecution`, `TestIntegration_CostLimitStopsExecution`, `TestIntegration_TimeoutStopsExecution` |
| 14 | `--dry-run` produces plan without executing | `TestExecSpecCommand_DryRun_ProducesPlanOnly`, `TestExecSpecCommand_DryRun_NoSideEffects`, `TestIntegration_DryRun_ProducesPlanOnly` |
| 15 | `events.jsonl` contains all specified event types | `TestEventLog_ContainsAllSpecifiedTypes`, `TestIntegration_HappyPath_ReadyForReview` |

## Test Fixtures

### Fixture projects

Located in `internal/next/testutil/fixtures/`. Committed to the repo so tests are reproducible.

```
testutil/fixtures/
  alpha/
    go.mod            # module example.com/alpha
    main.go           # minimal main package
    main_test.go      # one passing test
    project.json      # includes specs_dir: "specs/"
    specs/
      sample.md       # sample spec for testing
    internal/
      refund/
        refund.go     # simple refund logic
        refund_test.go
      billing/
        billing.go    # unrelated package (for scoping tests)
    .git/             # initialized via TestMain or test setup
  beta/
    go.mod            # module example.com/beta
    lib.go            # library package
    lib_test.go
    .git/
```

For tests that need fixture projects, use a helper that copies the fixture into `t.TempDir()` and initializes a git repo:

```go
// internal/next/testutil/fixtures.go
func SetupFixtureProject(t *testing.T, name string) string {
    t.Helper()
    dst := filepath.Join(t.TempDir(), name)
    copyDir(t, filepath.Join("testutil", "fixtures", name), dst)
    initGitRepo(t, dst)
    return dst
}
```

### Canned agent responses

Located in `internal/next/testutil/responses/`. JSON files with canned LLM outputs:

```
testutil/responses/
  valid_plan_2tasks.json        # Plan with 2 tasks
  valid_plan_5tasks.json        # Plan with 5 tasks (for budget tests)
  invalid_plan_malformed.json   # Unparseable JSON
  invalid_plan_empty_tasks.json # Valid JSON, empty tasks array
  valid_implementation.json     # Successful task implementation output
  valid_fix_plan.json           # Fix plan targeting specific failures
  needs_split_output.json       # Agent output indicating needs_split
```

### Execution policies

```
testutil/policies/
  default.json                  # All defaults
  tight_budget.json             # max_run_cost_usd: 0.50, max_run_duration_seconds: 600
  single_retry.json             # max_task_retries: 1, max_redecomposition_passes: 1
  custom_always_run.json        # Custom always_run commands
```

## Test Utilities

### `internal/next/testutil/assertions.go`

```go
// AssertFileExists fails the test if the file does not exist.
func AssertFileExists(t *testing.T, path string) {
    t.Helper()
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Fatalf("expected file to exist: %s", path)
    }
}

// AssertFileNotUnder fails if any file exists under the given directory
// that was not present before the test started.
func AssertFileNotUnder(t *testing.T, dir string, baseline []string) {
    t.Helper()
    // Walk dir, compare against baseline, fail if new files found.
}

// AssertJSONContainsKey fails if the JSON at path does not contain the key.
func AssertJSONContainsKey(t *testing.T, path string, key string) {
    t.Helper()
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("reading %s: %v", path, err)
    }
    var m map[string]any
    if err := json.Unmarshal(data, &m); err != nil {
        t.Fatalf("parsing %s: %v", path, err)
    }
    if _, ok := m[key]; !ok {
        t.Fatalf("expected key %q in %s", key, path)
    }
}

// AssertEventsContainTypes fails if events.jsonl does not contain
// at least one event of each specified type.
func AssertEventsContainTypes(t *testing.T, eventsPath string, types []string) {
    t.Helper()
    data, err := os.ReadFile(eventsPath)
    if err != nil {
        t.Fatalf("reading events: %v", err)
    }
    found := map[string]bool{}
    for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
        var evt struct{ Type string `json:"type"` }
        if err := json.Unmarshal([]byte(line), &evt); err != nil {
            continue
        }
        found[evt.Type] = true
    }
    for _, typ := range types {
        if !found[typ] {
            t.Errorf("events.jsonl missing event type %q", typ)
        }
    }
}

// AssertTerminalState reads run.json and asserts the terminal_state field.
func AssertTerminalState(t *testing.T, runDir string, expected string) {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
    if err != nil {
        t.Fatalf("reading run.json: %v", err)
    }
    var run struct{ TerminalState string `json:"terminal_state"` }
    if err := json.Unmarshal(data, &run); err != nil {
        t.Fatalf("parsing run.json: %v", err)
    }
    if run.TerminalState != expected {
        t.Fatalf("terminal_state: got %q, want %q", run.TerminalState, expected)
    }
}

// AssertBlockerSummaryPresent reads run.json and fails if blocker_summary is empty.
func AssertBlockerSummaryPresent(t *testing.T, runDir string) {
    t.Helper()
    data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
    if err != nil {
        t.Fatalf("reading run.json: %v", err)
    }
    var run struct{ BlockerSummary string `json:"blocker_summary"` }
    if err := json.Unmarshal(data, &run); err != nil {
        t.Fatalf("parsing run.json: %v", err)
    }
    if run.BlockerSummary == "" {
        t.Fatal("expected non-empty blocker_summary in run.json")
    }
}
```

### `internal/next/testutil/fake_agent.go`

```go
// FakeAgent implements AgentInvoker for tests.
type FakeAgent struct {
    mu        sync.Mutex
    responses []FakeResponse
    callIdx   int
    Calls     []FakeAgentCall
}

type FakeResponse struct {
    Output string
    Cost   float64
    Tokens int
    Err    error
}

type FakeAgentCall struct {
    Prompt  string
    Model   string
    Timeout time.Duration
}

func (a *FakeAgent) Invoke(ctx context.Context, prompt string, model string, timeout time.Duration) (AgentResult, error) {
    a.mu.Lock()
    defer a.mu.Unlock()

    a.Calls = append(a.Calls, FakeAgentCall{Prompt: prompt, Model: model, Timeout: timeout})

    if a.callIdx >= len(a.responses) {
        return AgentResult{}, fmt.Errorf("FakeAgent: no more responses (call %d)", a.callIdx)
    }
    resp := a.responses[a.callIdx]
    a.callIdx++

    if resp.Err != nil {
        return AgentResult{}, resp.Err
    }
    return AgentResult{
        Output: resp.Output,
        Cost:   resp.Cost,
        Tokens: resp.Tokens,
    }, nil
}
```

### Running tests

```bash
# Unit tests only (fast, no build tag needed)
go test ./internal/next/execpolicy/...
go test ./internal/next/runstore/...
go test ./internal/next/validator/...
go test ./internal/next/planner/...
go test ./internal/next/executor/...
go test ./internal/next/evidence/...
go test ./internal/next/specloop/...
go test ./cmd/gromit/...

# All unit tests
go test ./internal/next/...

# Integration tests (slower, real filesystem)
go test -tags integration ./internal/next/specloop/...

# All tests
go test -tags integration ./...
```
