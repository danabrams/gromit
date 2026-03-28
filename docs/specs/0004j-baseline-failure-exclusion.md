DONE 2026-03-27
# Spec 0004j — Baseline Failure Exclusion and Test Fixture Standard

## spec_id
baseline-failure-exclusion

## Vision

Test infrastructure failures look identical to implementation failures at the always-run gate. When `go test ./...` was already broken before the executor touched anything — because test helpers don't scaffold required files like project config or policy — the run replans indefinitely, generating fix tasks that can't clear a gate they didn't break. Two gaps compound each other: the missing baseline means pre-existing failures aren't distinguished from new ones, and inconsistent test setup means those failures keep appearing in new packages as the feature grows.

## Summary

Two changes. First, the init stage runs `always_run` checks in the fresh worktree immediately after creation and records any failures as a baseline. Subsequent validate cycles exclude baseline failures from replan triggers — only regressions introduced by the executor count. Second, test packages that require project-level file fixtures (config, policy) must scaffold that structure as part of test setup; no test should fail solely because expected project-level files are absent from the test working directory.

## Goals

### Primary
- Init stage captures a pre-execution baseline of always-run check failures
- Validate stage excludes baseline failures from replan triggers
- Test packages scaffold required project file fixtures before invoking components

### Secondary
- Baseline is persisted in run state and survives resume
- Baseline capture failure is non-fatal (run proceeds with empty baseline)

## Non-goals

- Fixing pre-existing failures automatically
- Per-check granularity beyond check name matching
- Cross-run baseline sharing
- Modifying existing passing tests

## Architecture

### Baseline capture (init stage)

After the worktree is created, the init stage runs each `always_run` check in the worktree and records failures:

```
RunState.BaselineFailures: map of check name → failure output
```

The init stage accepts an optional baseline runner (same interface already used by validate). If absent or if the baseline run itself errors, the run proceeds with an empty baseline — baseline capture is non-fatal.

### Validate stage exclusion

Before triggering a replan, validate filters out any check whose name appears in `BaselineFailures`. A check excluded this way is logged as `baseline_excluded` but does not count toward blocking. If all blocking failures are baseline-excluded, validate returns Continue.

### Test fixture standard (language-agnostic)

Test packages that invoke stages or components requiring a project file structure must scaffold that structure before running. The spec requires:

- A minimal project config file at the expected path
- A minimal execution policy file at the expected path

The implementation (a shared helper, a fixture factory, inline setup — whatever fits the language) is left to the executor. The behavioral requirement is: no test in this codebase should fail solely because expected project-level files are absent from the test working directory.

### New RunState field

```
baseline_failures: map<check_name, failure_output>
```

Persisted in run state, restored on resume.

### Event log additions

- `baseline_captured` — emitted by init with count of baseline failures
- `baseline_failure_excluded` — emitted by validate when a check is excluded

## Acceptance Criteria

1. When `always_run` checks are configured, the init stage runs them in the worktree after creation and stores any failures in `RunState.BaselineFailures` keyed by check name.

2. When the baseline runner is absent or returns an error, `RunState.BaselineFailures` is empty and the run continues without blocking.

3. `RunState.BaselineFailures` is persisted to run state storage and restored on resume.

4. When validate would trigger a replan, any blocking check failure whose check name appears in `RunState.BaselineFailures` is excluded from the replan context.

5. When all blocking failures are baseline-excluded, validate returns Continue rather than triggering a replan.

6. A `baseline_captured` event is emitted by init with the count of baseline failures found.

7. A `baseline_failure_excluded` event is emitted by validate each time a check is excluded due to baseline.

8. Test packages that invoke components requiring project-level file fixtures (config, policy) scaffold those fixtures as part of test setup; no test fails solely because expected project-level files are absent from the test working directory.

9. All existing tests continue to pass.

## Scenarios

### Scenario: Pre-existing failure excluded from replan

**Given:** the repo has a failing test in a package the spec does not touch
**When:** init runs the always-run checks in the fresh worktree
**Then:** that check name is recorded in `BaselineFailures`
**And:** when validate runs after execution and that same check fails, it is excluded from the replan context
**And:** a `baseline_failure_excluded` event is emitted
**And:** if it is the only blocking failure, validate returns Continue

### Scenario: Executor-introduced failure still triggers replan

**Given:** init captures a clean baseline (no failures)
**And:** the executor introduces a bug that breaks `unit-tests`
**When:** validate runs
**Then:** the failure is not in `BaselineFailures` and is not excluded
**And:** validate triggers a replan with the failure in context

### Scenario: Baseline runner absent — run proceeds normally

**Given:** no baseline runner is configured
**When:** init runs
**Then:** `RunState.BaselineFailures` is empty
**And:** validate treats all check failures as executor-introduced
**And:** no `baseline_captured` event is emitted

### Scenario: Baseline capture error — run proceeds with empty baseline

**Given:** the baseline runner returns an error (e.g. worktree not yet ready)
**When:** init attempts baseline capture
**Then:** the error is recorded in the event log
**And:** `RunState.BaselineFailures` is empty
**And:** the run continues to planning without blocking

### Scenario: Baseline survives resume

**Given:** a run captured baseline failures at init and was then paused mid-execution
**When:** the run is resumed
**Then:** `RunState.BaselineFailures` is restored from persisted state
**And:** subsequent validate cycles continue to exclude the same baseline failures

### Scenario: Test fixtures scaffolded in test setup

**Given:** a test that invokes a stage or component requiring project config and policy files
**When:** the test runs in an isolated temp directory
**Then:** the test sets up the required project file structure before invoking the component
**And:** the test does not fail due to missing project-level files

## Validation

```
go test ./internal/next/specloop/stages/...
go test ./internal/next/runstore/...
go vet ./...
go build ./...
```
