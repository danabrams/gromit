# Spec 0002a Manual Test Plan

**Spec**: Core Execution Loop and Deterministic Validation
**Date**: 2026-03-11
**Scope**: Human QA walkthrough of all Spec 0002a acceptance criteria

---

## 1. Prerequisites

### 1.1 System Requirements

- Go 1.22+ installed (`go version`)
- Git 2.30+ installed (`git --version` -- needs worktree support)
- An LLM provider API key set in environment (e.g., `ANTHROPIC_API_KEY`)
- `jq` installed for JSON inspection (`jq --version`)
- ~500 MB free disk for worktrees and fixture repos

### 1.2 Build the Binary

```bash
cd /Users/dabrams/gromit
go build -o gromit-next ./cmd/gromit-next/
# Verify:
./gromit-next --help
```

Confirm output shows `exec` and `spec` subcommands (these are the Spec 0002a additions; if they are missing, the implementation is not yet wired up).

### 1.3 Create Fixture Repos

See Section 4 for full details. You need three fixture repos:
- `fixture-calc` -- a tiny Go calculator package (happy path, fix cycle, budget tests)
- `fixture-greeter` -- a tiny Go greeting package (multi-project isolation)
- `fixture-multipackage` -- a multi-package Go module with `internal/auth/`, `internal/refund/`, `internal/billing/` (scoped context verification, Scenario 1b)

### 1.4 Attach Fixture Projects

Use the existing Spec 0001 CLI to attach each fixture repo as a project:

```bash
./gromit-next project attach --name fixture-calc --repo /tmp/gromit-fixtures/fixture-calc
./gromit-next project attach --name fixture-greeter --repo /tmp/gromit-fixtures/fixture-greeter
./gromit-next project attach --name fixture-multipackage --repo /tmp/gromit-fixtures/fixture-multipackage
```

Verify project cells exist:

```bash
ls ~/.local/share/gromit/projects/fixture-calc/
ls ~/.local/share/gromit/projects/fixture-greeter/
ls ~/.local/share/gromit/projects/fixture-multipackage/
```

### 1.5 Install Execution Policies

Copy the execution policy files into each project cell. See Section 4.3 for the exact JSON content.

```bash
mkdir -p ~/.local/share/gromit/projects/fixture-calc/policy
cp /tmp/gromit-fixtures/policies/fixture-calc-execution.json \
   ~/.local/share/gromit/projects/fixture-calc/policy/execution.json

mkdir -p ~/.local/share/gromit/projects/fixture-greeter/policy
cp /tmp/gromit-fixtures/policies/fixture-greeter-execution.json \
   ~/.local/share/gromit/projects/fixture-greeter/policy/execution.json

mkdir -p ~/.local/share/gromit/projects/fixture-multipackage/policy
cp /tmp/gromit-fixtures/policies/fixture-multipackage-execution.json \
   ~/.local/share/gromit/projects/fixture-multipackage/policy/execution.json
```

### 1.6 Run Inspect/Enrich (Spec 0001 baseline)

Ensure each project cell has populated context artifacts (doctrine, architecture, validation, source map) before testing execution:

```bash
./gromit-next context inspect --project fixture-calc
./gromit-next context enrich --project fixture-calc
./gromit-next context inspect --project fixture-greeter
./gromit-next context enrich --project fixture-greeter
./gromit-next context inspect --project fixture-multipackage
./gromit-next context enrich --project fixture-multipackage
```

Confirm `validation.json`, `architecture.json`, etc. exist in each project cell.

---

## 2. Test Scenarios

### Notation

- `RUN_DIR` = `~/.local/share/gromit/projects/<project>/runs/<run-id>/`
- All `Verify` steps use the actual run-id printed by the CLI.
- Cleanup: each scenario notes whether to clean up or leave artifacts.

---

### Scenario 1: Happy Path -- Simple Spec to `ready_for_review`

**Purpose**: Verify that a straightforward spec executes end-to-end, all validation passes, and the evidence bundle is complete.

**Setup**:
```bash
# Ensure fixture-calc is attached and enriched (see Prerequisites)
# Place the happy-path spec in the fixture repo:
cp /tmp/gromit-fixtures/specs/add-subtract.md /tmp/gromit-fixtures/fixture-calc/specs/
cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add subtract spec"
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. CLI output shows stages: `[init]`, `[compile]`, `[plan]`, `[execute]`, `[validate]`, `[evidence]`.
2. Terminal state printed: `ready_for_review`.
3. Run directory exists:
   ```bash
   RUN_ID=<from output>
   ls ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/
   ```
4. Required files present:
   - `run.json` -- parse with `jq .status` -> `"ready_for_review"`
   - `spec.md` -- copy of the spec
   - `spec-packet.md` -- non-empty
   - `plan.md` -- non-empty, lists tasks
   - `tasks.json` -- valid JSON array, each task has `task_id`, `objective`, `expected_touched_area`, `proof_checks`
   - `events.jsonl` -- at least one line per event type: `run_started`, `spec_packet_compiled`, `plan_created`, `plan_validation_result`, `task_created`, `task_started`, `task_completed`, `final_validation_result`, `terminal_state`
   - `execution-policy.json` -- snapshot of the policy used
5. Task directories exist under `tasks/`:
   ```bash
   ls ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/tasks/
   ```
   Each task directory has `task-packet.md`, `result.json`, `agent-output.txt`.
6. Evidence directory:
   ```bash
   ls ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/
   ```
   Required files: `summary.md`, `diff-summary.md`, `task-results.json`, `validation.json`, `review.md`, `metrics.json`.
6b. **Scoped context verification (AC 3)**: Inspect each task's `task-packet.md` to confirm it excludes unrelated context:
   ```bash
   for d in ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/tasks/*/; do
     echo "=== $(basename $d) ==="
     # Verify task-packet.md references only the files in expected_touched_area
     grep -c "greet/" "$d/task-packet.md" && echo "FAIL: contains unrelated greet/ context" || echo "OK: no unrelated context"
   done
   ```
   Note: For a single-package fixture like `fixture-calc`, this check is limited. Use `fixture-multipackage` (see Section 4.5) for a stronger scoping test -- run Scenario 1b below.
7. `validation.json` shows all checks passed (zero failures).
8. `metrics.json` contains: `cycles`, `total_tokens`, `total_cost_usd`, `duration_ms`, and at least one per-invocation record with `phase`, `tier`, `model`, `tokens_in`, `tokens_out`, `duration_ms`.
9. `review.md` is human-readable and includes: terminal state, what changed, validation results.
10. Worktree exists at the path recorded in `run.json` and is a valid git worktree:
    ```bash
    WORKTREE=$(jq -r .worktree_path ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/run.json)
    # Verify the directory exists
    test -d "$WORKTREE" && echo "OK: worktree directory exists" || echo "FAIL: worktree directory missing"
    # Verify it is a git worktree (not just a directory)
    git -C "$WORKTREE" rev-parse --is-inside-work-tree  # Should print "true"
    git -C "$WORKTREE" rev-parse --git-common-dir        # Should point to the main repo's .git
    # Verify worktree_path in run.json is an absolute path
    echo "$WORKTREE" | grep -q '^/' && echo "OK: absolute path" || echo "FAIL: not absolute path"
    ```
11. The worktree branch name matches pattern `gromit/spec-*-<run-id>`.
12. The target repo (fixture-calc) has NO new Gromit files committed to its main branch.
13. **Execution policy snapshot verification**: After the run completes, modify the source policy file and verify the run's snapshot is unchanged:
    ```bash
    # Record the original snapshot value
    ORIG_CYCLES=$(jq .budgets.max_spec_cycles ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/execution-policy.json)
    # Modify the live policy
    jq '.budgets.max_spec_cycles = 99' \
      ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
      && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
    # Verify snapshot is unchanged
    SNAP_CYCLES=$(jq .budgets.max_spec_cycles ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/execution-policy.json)
    test "$ORIG_CYCLES" = "$SNAP_CYCLES" && echo "OK: snapshot is immutable" || echo "FAIL: snapshot was mutated"
    # Restore the policy
    jq '.budgets.max_spec_cycles = 3' \
      ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
      && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
    ```

**Expected**: Terminal state `ready_for_review`. All validation passed. Full evidence bundle present. Worktree preserved. Execution policy snapshot is immutable (not a live reference).

**Cleanup**: Leave artifacts for Scenario 12 (CLI Inspection).

---

### Scenario 1b: Scoped Context Verification -- Multi-Package Fixture (AC 3)

**Purpose**: Verify that task packets exclude unrelated package context. AC 3 requires task packets to contain only scoped context relevant to the task's `expected_touched_area`.

**Setup**:
```bash
# Ensure fixture-multipackage is attached and enriched (see Prerequisites 1.3-1.6)
# Copy the scoped spec
cp /tmp/gromit-fixtures/specs/add-refund-endpoint.md /tmp/gromit-fixtures/fixture-multipackage/specs/
cd /tmp/gromit-fixtures/fixture-multipackage && git add specs/ && git commit -m "add refund endpoint spec"
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-multipackage \
  --spec /tmp/gromit-fixtures/fixture-multipackage/specs/add-refund-endpoint.md
```

**Verify**:

1. Terminal state: `ready_for_review`.
2. Inspect each task's `task-packet.md`:
   ```bash
   RUN_ID=<from output>
   for d in ~/.local/share/gromit/projects/fixture-multipackage/runs/$RUN_ID/tasks/*/; do
     echo "=== $(basename $d) ==="
     # Tasks targeting internal/refund/ should NOT contain internal/auth/ or internal/billing/ details
     grep -c "internal/auth/" "$d/task-packet.md" && echo "FAIL: leaks auth context" || echo "OK"
     grep -c "internal/billing/" "$d/task-packet.md" && echo "FAIL: leaks billing context" || echo "OK"
   done
   ```
3. Task packets referencing `internal/refund/` should contain `internal/refund/` code and interfaces it depends on, but NOT full source of unrelated packages like `internal/auth/` or `internal/billing/`.

**Expected**: Task packets are scoped to relevant code. Unrelated package internals are excluded.

**Cleanup**: None needed.

---

### Scenario 2: Fix Cycle -- Type Mismatch Then Fix to `ready_for_review`

**Purpose**: Verify that a validation failure triggers a fix-cycle replan and the run succeeds after the fix.

**Setup**:
```bash
# Ensure the pre-committed TestDivide (expecting int return) exists in fixture-calc
# (see Section 4.1 -- calc/divide_test.go should already be committed)
# Copy the float64 divide spec (see Section 4.2)
cp /tmp/gromit-fixtures/specs/divide-float64.md /tmp/gromit-fixtures/fixture-calc/specs/
cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add divide-float64 spec"
```

This spec asks for `Divide(a, b int) float64` but the pre-committed `TestDivide` expects an `int` return type. This guarantees a compilation error in cycle 1, forcing a fix cycle.

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/divide-float64.md
```

**Verify**:

1. CLI output shows at least 2 cycles (initial + fix).
2. `events.jsonl` contains:
   - `plan_created` with `"kind": "initial"` (cycle 1)
   - `final_validation_result` with at least one failure (cycle 1)
   - `replan_triggered` with failure context
   - `plan_created` with `"kind": "fix"` (cycle 2)
   - `final_validation_result` with all passes (cycle 2)
   - `terminal_state` with `"ready_for_review"`
3. `tasks.json` contains tasks from both cycles. Fix-cycle tasks have higher task IDs (globally sequential, e.g., cycle 1 has t-001 through t-003, cycle 2 has t-004).
4. Fix-cycle plan metadata includes `"failures_addressed"` referencing specific validation errors.
5. **Fix-plan scoping (AC 10)**: Verify the fix plan does NOT replan completed work:
   ```bash
   RUN_ID=<from output>
   # Extract cycle-1 and cycle-2 tasks
   jq '[.[] | select(.cycle == 1)] | map(.task_id)' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/tasks.json
   jq '[.[] | select(.cycle == 2)] | map(.task_id)' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/tasks.json
   ```
   - Cycle-2 task IDs must NOT duplicate any cycle-1 task IDs.
   - Cycle-2 tasks must target only the type-mismatch failure (not re-implement already-passing work).
   - Verify fix-plan metadata fields: `cycle`, `kind` (should be `"fix"`), `parent_cycle`, `failures_addressed`.
   ```bash
   # Check fix plan event metadata
   grep 'plan_created' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | \
     jq 'select(.kind == "fix") | {cycle, kind, parent_cycle, failures_addressed}'
   ```
6. Terminal state: `ready_for_review`.
7. `metrics.json` shows `cycles >= 2`.

**Expected**: Run completes in 2+ cycles. Cycle 1 fails with a compilation error (type mismatch between `float64` return and `int` assertion in `TestDivide`). Fix plan targets the type-mismatch failure without replanning completed work. Cycle-2 tasks are distinct from cycle-1 tasks. Terminal state `ready_for_review`.

**Determinism Fallback**: The out-of-scope constraint ("Do NOT modify any existing test files") may cause the agent to loop to `needs_human` instead of `ready_for_review`, because the only real fix requires modifying the pre-committed `TestDivide`. This makes the fix-cycle outcome non-deterministic. If this happens:
1. Note the outcome as "agent correctly identified the conflict and escalated" -- this is valid behavior.
2. To verify the fix-cycle machinery itself, rely on the automated integration tests: `TestSpecLoop_FixCycle_*` and `TestReplan_*` in the testing plan cover the replan/fix-cycle path deterministically with mocked agent outputs.
3. Optionally, to force a fix cycle manually: after PlanStage completes in a separate run (e.g., re-run Scenario 1's add-subtract spec), introduce a compilation error in the worktree before validation completes, then let the cycle continue. Document the result.

**Cleanup**: Leave for inspection.

---

### Scenario 3: Budget Exhausted -- `needs_human` After `max_spec_cycles`

**Purpose**: Verify that an unfixable validation failure exhausts the spec-cycle budget and terminates as `needs_human`.

**Setup**:
```bash
# Use the unfixable spec (see Section 4.2)
# This spec asks for behavior that conflicts with an existing test that cannot be changed
cp /tmp/gromit-fixtures/specs/unfixable-conflict.md /tmp/gromit-fixtures/fixture-calc/specs/
cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add unfixable spec"
```

Ensure the execution policy has `max_spec_cycles: 2` for a faster test:
```bash
jq '.budgets.max_spec_cycles = 2' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/unfixable-conflict.md
```

**Verify**:

1. Terminal state: `needs_human`.
2. `run.json` -> `jq .status` -> `"needs_human"`.
3. **Cycle exhaustion event**: Check `events.jsonl` for how cycle exhaustion is signaled. Note: `budget_exceeded` is primarily for cost/time budgets. Cycle exhaustion may or may not emit `budget_exceeded` depending on implementation. The key verification is:
   - Terminal state is `needs_human` (not `blocked` -- cycle exhaustion is a task-progress failure, not an infrastructure issue).
   - `events.jsonl` contains `terminal_state` with `"needs_human"` and a reason referencing cycle exhaustion.
   ```bash
   grep 'terminal_state' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq '{state, reason}'
   ```
   - If `budget_exceeded` IS emitted, verify `budget_type` is `"cycles"`. Cross-reference: Scenario 10 should show `"cost"`, Scenario 11 should show `"time"`.
4. `run.json` or blocker summary includes: what failed, what was tried, recommended next action.
5. Evidence bundle is still emitted (review.md, metrics.json, validation.json all present).
6. `validation.json` shows at least one failing check.
7. Worktree preserved (path exists on disk).
8. `metrics.json` -> `cycles` equals `max_spec_cycles` (2).

**Expected**: Run exhausts 2 spec cycles. Terminates `needs_human` with blocker summary. Evidence bundle complete.

**Cleanup**: Restore `max_spec_cycles` to 3:
```bash
jq '.budgets.max_spec_cycles = 3' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 4: Blocked -- Bad Spec (Planner Failure)

**Purpose**: Verify that a vague/invalid spec causes the planner to fail and the run transitions to `blocked`.

**Setup**:
```bash
# Use the vague spec (see Section 4.2)
cp /tmp/gromit-fixtures/specs/vague-spec.md /tmp/gromit-fixtures/fixture-calc/specs/
cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add vague spec"
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/vague-spec.md
```

**Verify**:

1. Terminal state: `blocked`.
2. `run.json` -> `jq .status` -> `"blocked"`.
3. CLI output mentions planner failure (invalid output or empty task list).
4. `events.jsonl` contains `plan_validation_result` with `"pass": false` (at least twice -- initial + retry).
5. Blocker summary mentions: the spec may be too vague, recommended action is to revise the spec.
6. Evidence bundle is emitted with whatever was collected (review.md exists, may be partial).
7. No task directories exist (execution never started).

**Expected**: Planner fails after 1 retry. Terminal state `blocked`. Blocker summary legible.

**Note**: Plan validation (malformed JSON, missing fields) is best verified via automated tests. This scenario tests planner-level failure (empty/nonsensical plan), which is a different path. See the testing plan for comprehensive plan validation coverage.

**Cleanup**: None needed.

---

### Scenario 4b: Blocked -- Provider Unavailability

**Purpose**: Verify that provider errors (503, invalid API key, unreachable endpoint) cause `blocked` with a clear provider-error reason.

**Setup**:
```bash
# Save the real API key and set an invalid one
REAL_KEY="$ANTHROPIC_API_KEY"
export ANTHROPIC_API_KEY="sk-invalid-key-00000000"
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. Terminal state: `blocked`.
2. `run.json` -> `jq .status` -> `"blocked"`.
3. CLI output mentions provider error, authentication failure, or unreachable endpoint.
4. `events.jsonl` contains an event indicating provider failure (e.g., `stage_error` or `terminal_state` with provider context).
5. Blocker summary mentions the provider issue and recommends checking API key / endpoint configuration.
6. No task directories created (execution never started).

**Expected**: Immediate `blocked` with clear provider-unavailability error. The run does not retry indefinitely against a broken provider.

**Cleanup**:
```bash
export ANTHROPIC_API_KEY="$REAL_KEY"
```

---

### Scenario 5: Blocked -- Missing Project Cell (Infrastructure)

**Purpose**: Verify that a missing or corrupt project cell causes `blocked` at InitStage.

**Setup**:
```bash
# Temporarily rename the project cell to simulate it being missing
mv ~/.local/share/gromit/projects/fixture-calc ~/.local/share/gromit/projects/fixture-calc.bak
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. Terminal state: `blocked`.
2. CLI output mentions missing project cell or project not found.
3. Error is clear and actionable (mentions project name, expected location).
4. If a run directory was created, it contains `run.json` with state `blocked` and the blocker reason.

**Expected**: Immediate `blocked` with clear infrastructure error.

**Cleanup**:
```bash
mv ~/.local/share/gromit/projects/fixture-calc.bak ~/.local/share/gromit/projects/fixture-calc
```

---

### Scenario 6: Dry Run -- Plan Without Executing

**Purpose**: Verify `--dry-run` creates a plan but does not execute tasks.

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md \
  --dry-run
```

**Verify**:

1. CLI output shows `[init]`, `[compile]`, `[plan]` stages but NOT `[execute]` or `[validate]`.
2. CLI output says "Dry run complete" and shows the plan.
3. Run directory exists:
   ```bash
   RUN_ID=<from output>
   ls ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/
   ```
4. `run.json` exists and indicates dry-run mode.
5. `plan.md` exists and lists tasks with descriptions.
6. `tasks.json` exists and is valid.
7. `spec-packet.md` exists (compilation happened).
8. No `tasks/` subdirectories with `result.json` (no tasks were executed).
9. No `evidence/validation.json` (validation did not run).

10. **Dry-run is visible in exec show**: Verify `exec show` displays this run with a `dry-run` flag or indicator:
    ```bash
    ./gromit-next exec show $RUN_ID --project fixture-calc
    # The run should be visible and clearly marked as a dry-run
    ```

**Expected**: Plan is generated and saved. No task execution. No validation. Run is clearly marked as a dry-run in `exec show`.

**Cleanup**: None needed.

---

### Scenario 7: Task Repair -- Task Fails Then Repairs

**Purpose**: Verify that a task-level failure triggers one repair attempt before marking the task as done or failed.

**Setup**: Use a spec where one task is likely to have a minor error on first attempt (e.g., the add-subtract spec which is simple enough that most tasks should succeed, but you can verify the mechanism exists by inspecting task results).

If the happy path completes without any task needing repair, use an alternative approach: temporarily set `max_task_retries: 1` in the execution policy (should already be the default) and inspect the task result records.

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. Inspect each task's `result.json`:
   ```bash
   RUN_ID=<from output>
   for d in ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/tasks/*/; do
     echo "=== $(basename $d) ==="
     jq '{status, attempts, targeted_checks, always_run_checks}' "$d/result.json"
   done
   ```
2. If any task has `"attempts": 2`, verify:
   - `events.jsonl` has `task_validation_result` with failures for that task, followed by another `task_started` (the repair).
   - The final `task_completed` event shows success after repair.
3. If all tasks complete in 1 attempt, verify:
   - The mechanism is wired: set `max_task_retries: 0` in the policy, re-run, and confirm a task that would have been repaired now ends as `failed` (no repair attempt).
4. Verify that no task has `"attempts"` exceeding `max_task_retries + 1` (i.e., 2 total with default settings).

**Expected**: Task repair mechanism exists and respects `max_task_retries` budget. Tasks that fail once get exactly one repair attempt.

**Cleanup**: Restore `max_task_retries` to 1 if changed.

---

### Scenario 8: Task Split -- Re-decomposition

**Purpose**: Verify that a task that is too broad triggers `needs_split` and re-decomposition within ExecuteStage.

**Setup**:
```bash
# Use fixture-multipackage (which has auth/, refund/, billing/ directories)
# and the cross-package refactor spec (see Section 4.2)
cp /tmp/gromit-fixtures/specs/cross-package-refactor.md /tmp/gromit-fixtures/fixture-multipackage/specs/
cd /tmp/gromit-fixtures/fixture-multipackage && git add specs/ && git commit -m "add cross-package-refactor spec"
```

Ensure `max_redecomposition_passes: 1` (default) in fixture-multipackage policy.

**Execute**:
```bash
./gromit-next exec spec --project fixture-multipackage --spec /tmp/gromit-fixtures/fixture-multipackage/specs/cross-package-refactor.md
```

**Verify**:

1. `events.jsonl` contains `task_needs_split` event for the broad task.
2. `events.jsonl` contains `redecomposition_triggered` event with the original `task_id` and sub-task count.
3. Sub-task directories exist under `tasks/` with IDs appended to the ledger (e.g., t-001a, t-001b, t-001c or sequential t-004, t-005, t-006). Sub-tasks should target individual packages (`internal/auth/`, `internal/refund/`, `internal/billing/`).
4. The failed task's changes were reverted before sub-task execution (check git log in worktree for absence of the broad task's commit).
5. `max_redecomposition_passes` budget is consumed:
   - If a second task also needs splitting, it should be marked `failed` (not re-decomposed).
   - Verify via `events.jsonl` or task `result.json`.
6. Re-decomposition did NOT consume a `max_spec_cycles` budget unit (check `metrics.json` -> `cycles` is still 1 if no validation failures).

**Expected**: Broad task triggers `needs_split`. Re-decomposition produces sub-tasks. Budget is respected.

**Determinism Fallback**: If the planner decomposes the cross-package refactor into per-package tasks from the start, `needs_split` will never trigger. In that case:
1. Note the outcome as "planner correctly decomposed the spec upfront" -- this validates planning quality rather than `needs_split`.
2. The `needs_split` heuristic and redecomposition machinery are better verified via the automated tests in the testing plan: `TestNeedsSplitHeuristic_ThreePackages`, `TestNeedsSplitHeuristic_TwoTimesFileSpread`, `TestTaskLoop_NeedsSplit_TriggersRedecomposition`, `TestRedecomposition_WithinExecuteStage`, `TestRedecomposition_GlobalBudget_AcrossTasks`, and `TestSpecLoop_MaxRedecompositionPasses_Respected`.
3. Record whether `needs_split` was triggered or not. Both outcomes are acceptable for the manual test.

**Cleanup**: None needed.

---

### Scenario 9: Multi-Project Isolation -- Two Concurrent Runs

**Purpose**: Verify that two runs on different projects do not share state.

**Setup**: Ensure both `fixture-calc` and `fixture-greeter` are attached with policies and enriched (see Prerequisites).

**Execute**:
```bash
# Run both in parallel
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md &
PID1=$!

./gromit-next exec spec --project fixture-greeter \
  --spec /tmp/gromit-fixtures/fixture-greeter/specs/add-farewell.md &
PID2=$!

wait $PID1 $PID2
```

**Verify**:

1. Both runs complete (check exit codes, both should be 0 or at least produce terminal states).
2. Run directories are under their respective projects:
   ```bash
   ls ~/.local/share/gromit/projects/fixture-calc/runs/
   ls ~/.local/share/gromit/projects/fixture-greeter/runs/
   ```
3. Worktrees are separate (different paths, different branch names):
   ```bash
   # Get worktree paths from run.json for each
   jq -r .worktree_path ~/.local/share/gromit/projects/fixture-calc/runs/*/run.json
   jq -r .worktree_path ~/.local/share/gromit/projects/fixture-greeter/runs/*/run.json
   ```
4. Spec packets are different (compiled from different project cells):
   ```bash
   # Use the specific run-ids from the parallel execution output above
   CALC_RUN_ID=<from PID1 output>
   GREETER_RUN_ID=<from PID2 output>
   # Spot-check: fixture-calc packet mentions calculator, fixture-greeter mentions greeter
   head -20 ~/.local/share/gromit/projects/fixture-calc/runs/$CALC_RUN_ID/spec-packet.md
   head -20 ~/.local/share/gromit/projects/fixture-greeter/runs/$GREETER_RUN_ID/spec-packet.md
   ```
5. No cross-contamination: fixture-calc evidence does not reference fixture-greeter files, and vice versa.
6. Events logs are independent (each has its own `run_started` with the correct project).

**Expected**: Full isolation. Concurrent execution completes without conflicts.

**Cleanup**: None needed.

---

### Scenario 10: Cost Limit -- `blocked` on `budget_exceeded`

**Purpose**: Verify that `max_run_cost_usd` is enforced and the run terminates when exceeded.

**Setup**:
```bash
# Set an extremely low cost budget
jq '.budgets.max_run_cost_usd = 0.001' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. Terminal state: `blocked`.
2. `events.jsonl` contains `budget_exceeded` event with `"budget": "cost"`.
3. `run.json` -> `jq .status` -> `"blocked"`.
4. Evidence bundle is still emitted (partial).
5. `metrics.json` shows the accumulated cost near or above the limit.
6. **Cost enforcement timing**: Verify the stage that was running when the limit was hit completed (check its events are present), but no further stages executed after it.

**Expected**: Cost limit triggers `blocked`. The currently executing stage completes, but no further stages start. Evidence bundle preserved.

**Cleanup**:
```bash
jq '.budgets.max_run_cost_usd = 50.0' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 11: Timeout -- `blocked` on `max_run_duration_seconds`

**Purpose**: Verify that the run timeout is enforced.

**Setup**:
```bash
# Set a very short timeout (5 seconds -- should not be enough to complete)
jq '.budgets.max_run_duration_seconds = 5' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. Terminal state: `blocked`.
2. `events.jsonl` contains `budget_exceeded` event with `"budget": "time"`.
3. Run completes in roughly the timeout duration (not hanging indefinitely).
4. Evidence bundle emitted with whatever was collected.
5. CLI output mentions timeout.

**Expected**: Timeout enforced. Run terminates promptly. Partial evidence preserved.

**Cleanup**:
```bash
jq '.budgets.max_run_duration_seconds = 3600' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 11b: Per-Task Timeout -- `max_task_duration_seconds`

**Purpose**: Verify that individual task execution respects `max_task_duration_seconds` and the run handles a timed-out task correctly.

**Setup**:
```bash
# Set an extremely low per-task timeout (5 seconds)
jq '.budgets.max_task_duration_seconds = 5' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

Use a spec that requires non-trivial work (e.g., `broad-refactor.md` which has multiple files to create).

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/broad-refactor.md
```

**Verify**:

1. At least one task's `result.json` shows a timeout-related failure (`"status": "failed"` with timeout reason).
2. `events.jsonl` contains a task failure event mentioning timeout or duration exceeded.
3. The run continues to process remaining tasks (timeout of one task does not abort the entire run unless all tasks fail).
4. If all tasks time out, the run should reach `needs_human` (not hang indefinitely).
5. `metrics.json` per-invocation records show `duration_ms` near the 5-second limit for timed-out tasks.

**Expected**: Per-task timeout enforced. Timed-out tasks are marked failed. Run continues or terminates gracefully.

**Cleanup**:
```bash
jq '.budgets.max_task_duration_seconds = 300' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 11c: All Tasks Failed -- `needs_human` via ExecuteStage

**Purpose**: Verify that when every task in the plan fails (not due to budget exhaustion), the run transitions to `needs_human` via ExecuteStage's NeedsHuman path.

**Setup**:
```bash
# Use a spec where all tasks will fail. One approach: set max_task_duration_seconds very low
# and use a spec with multiple complex tasks, so every task times out.
jq '.budgets.max_task_duration_seconds = 2 | .budgets.max_task_retries = 0' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/broad-refactor.md
```

**Verify**:

1. Terminal state: `needs_human` (distinct from `blocked` which is for infrastructure/budget issues).
2. `run.json` -> `jq .status` -> `"needs_human"`.
3. Every task in `tasks/*/result.json` has `"status": "failed"`.
4. `events.jsonl` contains `task_failed` events for each failed task:
   ```bash
   grep 'task_failed' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq '{task_id, reason}'
   ```
5. `events.jsonl` does NOT contain `budget_exceeded` -- this is a task-failure path, not a budget path.
6. Blocker summary explains that all tasks failed and recommends human intervention.
7. Evidence bundle is emitted.

**Expected**: All-tasks-failed triggers `needs_human` via ExecuteStage, not `blocked`. Evidence bundle present.

**Cleanup**:
```bash
jq '.budgets.max_task_duration_seconds = 300 | .budgets.max_task_retries = 1' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 11d: Concurrent Same-Spec Runs

**Purpose**: Verify that two concurrent runs of the SAME spec on the SAME project get unique branch names and do not conflict.

**Setup**: Ensure `fixture-calc` is attached with the happy-path spec committed.

**Execute**:
```bash
# Run the same spec twice in parallel
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md &
PID1=$!

./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md &
PID2=$!

wait $PID1 $PID2
```

**Verify**:

1. Both runs complete (both produce terminal states, neither crashes).
2. Two distinct run directories exist:
   ```bash
   ls ~/.local/share/gromit/projects/fixture-calc/runs/ | tail -2
   ```
3. Run IDs are different.
4. Worktree paths are different:
   ```bash
   for d in ~/.local/share/gromit/projects/fixture-calc/runs/*/; do
     jq -r .worktree_path "$d/run.json"
   done | sort -u | wc -l
   # Should be >= 2 (one per run)
   ```
5. Branch names are different (each includes the unique run-id).
6. Neither run's evidence references the other run's artifacts.

**Expected**: Full isolation between concurrent same-spec runs. Unique run IDs, branch names, and worktrees.

**Cleanup**: None needed.

---

### Scenario 12: CLI Inspection -- `exec show`, `exec list`, `spec list`

**Purpose**: Verify the inspection CLI commands return correct data.

**Prerequisite**: Scenarios 1-4 should have completed, leaving multiple runs in fixture-calc.

**Execute and Verify**:

#### 12a: `exec list`
```bash
./gromit-next exec list --project fixture-calc
```
- Output is a table with columns: Run ID, Spec, State, When.
- All runs from previous scenarios appear.
- States match what was observed (at least one `ready_for_review`, one `needs_human`, one `blocked`).

#### 12b: `exec show <run-id>`
```bash
# Use the run-id from Scenario 1
./gromit-next exec show <run-id-from-scenario-1>
```
- Shows: Run ID, Spec name, State (`ready_for_review`), Cycles, Tasks completed count, Validation pass count, Duration, Cost.
- Shows worktree path and evidence path.

#### 12c: `exec show latest`
```bash
./gromit-next exec show latest --project fixture-calc
```
- Shows the most recent run for fixture-calc.
- Fields match the most recently executed scenario.

#### 12d: `exec show --full`
```bash
./gromit-next exec show <run-id-from-scenario-1> --full
```
- Shows the complete evidence bundle content (or paths to all evidence files).
- Must include paths or content for ALL expected evidence files: `summary.md`, `diff-summary.md`, `task-results.json`, `validation.json`, `review.md`, `metrics.json`.
- Must include task-level details: per-task status, attempts, files changed.
- Must include plan details: task count, cycle count.

#### 12e: `spec list`
```bash
./gromit-next spec list --project fixture-calc
```
- Output is a table with: ID, Title, Status.
- Verify which statuses are producible in 0002a scope:
  - `ready` -- a spec file exists but has no associated run yet. **Verifiable**: add a new spec file to the repo without running it.
  - `needs_attention` -- a spec with a `needs_human` run (e.g., `unfixable-conflict` from Scenario 3). **Verifiable**.
  - `running` -- a spec with an active run. **Verifiable**: run another spec concurrently and check `spec list` while it executes.
  - `completed` -- requires human acceptance (0002b scope, e.g., `./gromit-next exec accept <run-id>`). **Not verifiable in 0002a** unless the accept command is already wired.
  - `draft` -- requires an approval signal to distinguish draft from ready. **TBD**: document how draft is detected, or note as not yet implemented if the approval workflow is 0002b scope.
- Status derivation is consistent with run history:
  - add-subtract: should show `ready_for_review`-derived status (no accept command in 0002a)
  - unfixable-conflict: should show `needs_attention` (has a `needs_human` run)
  - vague-spec: may show `ready` or `blocked`-derived status

**Expected**: All CLI inspection commands return correct, formatted output. Status derivation is consistent with run history. At minimum, `ready`, `needs_attention`, and `running` statuses are demonstrated in 0002a scope. `completed` and `draft` may require 0002b features.

**Cleanup**: None needed.

---

## 3. Artifact Verification Checklist

Use this after any scenario to systematically verify artifacts.

### 3.1 Run Directory Structure

For any completed run at `RUN_DIR`:

| File | Must Exist | Check |
|------|-----------|-------|
| `run.json` | Always | Valid JSON. Has `state`, `run_id`, `project`, `spec_id`, `worktree_path`, `created_at`. |
| `spec.md` | Always | Non-empty. Content matches the input spec file. |
| `spec-packet.md` | Always | Non-empty. Contains project context + spec content. |
| `plan.md` | If past PlanStage | Non-empty. Lists tasks with descriptions. |
| `tasks.json` | If past PlanStage | Valid JSON array. Each task has `task_id`, `objective`, `expected_touched_area`, `proof_checks`. |
| `events.jsonl` | Always | Each line is valid JSON. Has `event_type` and `timestamp`. |
| `execution-policy.json` | Always | Snapshot of policy at run start. Matches what was in the project cell. |

### 3.2 Task Directories

For each `RUN_DIR/tasks/<task-id>/`:

| File | Must Exist | Check |
|------|-----------|-------|
| `task-packet.md` | Always | Non-empty. Scoped context (should NOT contain unrelated package info). |
| `result.json` | Always | Valid JSON. Has `task_id`, `status`, `attempts`, `targeted_checks`, `always_run_checks`, `files_changed`, `tokens_used`, `duration_ms`, `model_tier`. |
| `agent-output.txt` | Always | Non-empty. Raw stdout from executor agent. |

### 3.3 Evidence Directory

For `RUN_DIR/evidence/`:

| File | Must Exist | Check |
|------|-----------|-------|
| `summary.md` | Always | Human-readable summary of the run. |
| `diff-summary.md` | If code changed | Lists changed files and brief descriptions. |
| `task-results.json` | Always | Aggregated task outcomes. |
| `validation.json` | If validation ran | Check results per command. Each has name, command, type, pass/fail, output. |
| `review.md` | Always | Decision sheet. Contains: terminal state, what changed, validation results, known risks, recommended action. |
| `metrics.json` | Always | See metrics checks below. |

### 3.4 Metrics Spot-Checks

In `metrics.json`, verify:

```bash
jq '{
  cycles,
  total_tokens,
  total_cost_usd,
  duration_ms,
  invocation_count: (.invocations | length)
}' RUN_DIR/evidence/metrics.json
```

Each invocation record should have:
- `phase` (one of: planner, executor, etc.)
- `tier` (low, medium, high, xhigh)
- `model` (actual model name used)
- `tokens_in`, `tokens_out` (integers > 0)
- `duration_ms` (integer > 0)
- `success` (boolean)

### 3.5 Events Spot-Checks

```bash
# Count event types
cat RUN_DIR/events.jsonl | jq -r .event_type | sort | uniq -c | sort -rn
```

For a successful run, expect all of:
- `run_started` (1)
- `spec_packet_compiled` (1)
- `plan_created` (>= 1)
- `plan_validation_result` (>= 1)
- `task_created` (>= 1, one per task in the plan)
- `task_started` (>= 1)
- `task_validation_result` (>= 1)
- `task_completed` (>= 1)
- `final_validation_result` (>= 1)
- `terminal_state` (1)

Conditional events (only present when the corresponding path is taken):
- `redecomposition_triggered` -- only when a task triggers `needs_split` (Scenario 8); NOT expected in happy-path runs
- `replan_triggered` -- only when a fix cycle occurs (Scenario 2)
- `budget_exceeded` -- only when a budget limit is hit (Scenarios 3, 10, 11)

### 3.6 Worktree Isolation

```bash
# Verify worktree branch name
WORKTREE=$(jq -r .worktree_path RUN_DIR/run.json)
cd "$WORKTREE" && git branch --show-current
# Should match: gromit/spec-<spec-id>-<run-id>

# Verify target repo main branch is clean
cd /tmp/gromit-fixtures/fixture-calc
git log --oneline -5
# Should NOT contain any Gromit-generated commits
```

---

## 4. Fixture Repos

### 4.1 fixture-calc

A minimal Go module with a calculator package.

```bash
mkdir -p /tmp/gromit-fixtures/fixture-calc
cd /tmp/gromit-fixtures/fixture-calc
git init
go mod init github.com/test/fixture-calc
mkdir -p calc specs
```

**calc/calc.go**:
```go
package calc

// Add returns the sum of two integers.
func Add(a, b int) int {
    return a + b
}
```

**calc/calc_test.go**:
```go
package calc

import "testing"

func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 {
        t.Errorf("Add(2, 3) = %d, want 5", Add(2, 3))
    }
}
```

```bash
cd /tmp/gromit-fixtures/fixture-calc
git add -A && git commit -m "initial: calculator with Add"
```

**calc/divide_test.go** (pre-committed test for Scenario 2 -- expects `int` return type):
```go
package calc

import "testing"

func TestDivide(t *testing.T) {
    result := Divide(10, 3)
    if result != 3 {
        t.Fatalf("got %d", result)
    }
}
```

```bash
cd /tmp/gromit-fixtures/fixture-calc
git add calc/divide_test.go && git commit -m "add Divide test expecting int return"
```

### 4.2 Spec Files

Place these in `/tmp/gromit-fixtures/specs/`.

**add-subtract.md** (Happy Path - Scenario 1):
```markdown
# Add Subtract Function

## spec_id
add-subtract

## Title
Add a Subtract function to the calculator

## Problem
The calculator package only has Add. We need Subtract.

## In-Scope
- Add a `Subtract(a, b int) int` function to `calc/calc.go`
- Add tests for Subtract in `calc/calc_test.go`

## Out-of-Scope
- No changes to the Add function
- No new packages

## Acceptance Criteria
1. `calc.Subtract(5, 3)` returns `2`
2. `calc.Subtract(0, 0)` returns `0`
3. `calc.Subtract(3, 5)` returns `-2`
4. All existing tests continue to pass
5. `go vet ./...` passes
6. `gofmt -l .` produces no output

## Architectural Constraints
- All code stays in the `calc` package

## Validation
- `go test ./calc/...`
- `go vet ./...`
```

**divide-float64.md** (Fix Cycle - Scenario 2):
```markdown
# Add Divide Function

## spec_id
divide-float64

## Title
Add a Divide function returning float64

## Problem
The calculator needs a division function that returns float64 for precision.

## In-Scope
- Add a `Divide(a, b int) float64` function to `calc/calc.go`
- The function must return `float64(a) / float64(b)`

## Out-of-Scope
- No changes to existing functions
- Do NOT modify any existing test files

## Acceptance Criteria
1. `calc.Divide(10, 2)` returns `5.0`
2. `calc.Divide(10, 3)` returns approximately `3.333...`
3. All existing tests continue to pass
4. `go vet ./...` passes

## Architectural Constraints
- All code stays in the `calc` package
- Existing tests must not be modified

## Validation
- `go test ./calc/...`
- `go vet ./...`
```

**Why this guarantees a fix cycle**: The pre-committed `TestDivide` in `calc/divide_test.go` expects `Divide(10, 3)` to return an `int` value of `3`. This spec asks for a `float64` return type. Cycle 1 will produce a compilation error (`cannot use Divide(10, 3) (value of type float64) as int`). The fix-cycle planner must then produce a task to reconcile the type mismatch (e.g., change the test to expect `float64`, or change the return type).

**unfixable-conflict.md** (Budget Exhausted - Scenario 3):
```markdown
# Unfixable Conflict

## spec_id
unfixable-conflict

## Title
Make Add return the product instead of the sum

## Problem
We want Add to multiply instead of adding, but without changing the existing test.

## In-Scope
- Change `Add(a, b int) int` to return `a * b` instead of `a + b`
- Do NOT modify any existing test files

## Out-of-Scope
- Must not change calc_test.go

## Acceptance Criteria
1. `calc.Add(2, 3)` returns `6` (product)
2. The existing TestAdd test must still pass (it expects Add(2,3) == 5)
3. `go vet ./...` passes

## Architectural Constraints
- All code stays in the `calc` package
- Existing tests must not be modified

## Validation
- `go test ./calc/...`
```

Note: Acceptance criteria 1 and 2 are contradictory. The agent cannot satisfy both. This guarantees validation failure on every cycle.

**vague-spec.md** (Blocked - Scenario 4):
```markdown
# Vague Spec

## spec_id
vague-spec

## Title
Make it better

## Problem
Things could be improved.

## In-Scope
- Improvements

## Out-of-Scope
- Nothing specific

## Acceptance Criteria
(none provided)
```

**broad-refactor.md** (used by Scenarios 11b, 11c as a multi-file spec on fixture-calc):

```markdown
# Broad Refactor

## spec_id
broad-refactor

## Title
Add Division, Modulo, Power, and Absolute Value with full test coverage

## Problem
The calculator needs many more operations, each in its own file with comprehensive tests.

## In-Scope
- Add `calc/division.go` with `Divide(a, b int) (int, error)` and `SafeDivide`
- Add `calc/modulo.go` with `Mod(a, b int) (int, error)`
- Add `calc/power.go` with `Power(base, exp int) int` and `PowerFloat(base float64, exp int) float64`
- Add `calc/abs.go` with `Abs(a int) int` and `AbsFloat(a float64) float64`
- Add comprehensive test files for each: `calc/division_test.go`, `calc/modulo_test.go`, `calc/power_test.go`, `calc/abs_test.go`
- Add `calc/doc.go` with package documentation

## Out-of-Scope
- No changes to existing Add function

## Acceptance Criteria
1. All new functions work correctly with positive, negative, and zero inputs
2. Division and Modulo return errors for zero divisors
3. All tests pass
4. `go vet ./...` passes
5. `gofmt -l .` produces no output

## Architectural Constraints
- Each operation in its own file
- All code stays in the `calc` package

## Validation
- `go test ./calc/...`
- `go vet ./...`
```

**cross-package-refactor.md** (Task Split - Scenario 8, targets fixture-multipackage):

This spec is intentionally broad: a single task spanning all three packages (`internal/auth/`, `internal/refund/`, `internal/billing/`) to trigger the `needs_split` heuristic via file-spread across directories.

```markdown
# Cross-Package Logging Refactor

## spec_id
cross-package-refactor

## Title
Add structured logging to all internal packages

## Problem
None of the internal packages have any logging. We need structured logging in auth, refund, and billing for observability.

## In-Scope
- Add a `internal/logging/logger.go` package with a `Log(pkg, action, detail string)` function
- Add `internal/logging/logger_test.go` with tests
- Modify `internal/auth/auth.go`: add logging to `ValidateToken` (log token validation attempts)
- Modify `internal/refund/refund.go`: add logging to `Process` (log refund processing)
- Modify `internal/billing/billing.go`: add logging to `CreateInvoice` (log invoice creation)
- Update all three packages' test files to verify logging calls occur
- This is ONE task: add logging across all packages in a single pass

## Out-of-Scope
- No external logging libraries
- No changes to function signatures

## Acceptance Criteria
1. `internal/logging` package exists with `Log` function
2. All three packages (`auth`, `refund`, `billing`) call `Log` in their functions
3. All existing tests continue to pass
4. New tests verify logging behavior
5. `go vet ./...` passes
6. `gofmt -l .` produces no output

## Architectural Constraints
- All packages import the new `internal/logging` package
- No external dependencies

## Validation
- `go test ./...`
- `go vet ./...`
```

### 4.3 fixture-greeter

A minimal Go module for multi-project isolation testing.

```bash
mkdir -p /tmp/gromit-fixtures/fixture-greeter
cd /tmp/gromit-fixtures/fixture-greeter
git init
go mod init github.com/test/fixture-greeter
mkdir -p greet specs
```

**greet/greet.go**:
```go
package greet

import "fmt"

// Hello returns a hello greeting.
func Hello(name string) string {
    return fmt.Sprintf("Hello, %s!", name)
}
```

**greet/greet_test.go**:
```go
package greet

import "testing"

func TestHello(t *testing.T) {
    got := Hello("World")
    want := "Hello, World!"
    if got != want {
        t.Errorf("Hello(World) = %q, want %q", got, want)
    }
}
```

```bash
cd /tmp/gromit-fixtures/fixture-greeter
git add -A && git commit -m "initial: greeter with Hello"
```

**specs/add-farewell.md**:
```markdown
# Add Farewell Function

## spec_id
add-farewell

## Title
Add a Farewell function to the greeter

## Problem
The greeter only says hello. We need goodbyes.

## In-Scope
- Add a `Farewell(name string) string` function to `greet/greet.go`
- Returns "Goodbye, <name>!"
- Add tests in `greet/greet_test.go`

## Out-of-Scope
- No changes to Hello function

## Acceptance Criteria
1. `greet.Farewell("World")` returns `"Goodbye, World!"`
2. All existing tests continue to pass
3. `go vet ./...` passes

## Architectural Constraints
- All code stays in the `greet` package

## Validation
- `go test ./greet/...`
```

### 4.4 Execution Policy Files

**fixture-calc-execution.json** (save to `/tmp/gromit-fixtures/policies/`):
```json
{
  "always_run": [
    {"name": "unit-tests", "command": "go test ./...", "type": "test"},
    {"name": "format", "command": "gofmt -l .", "type": "lint"},
    {"name": "vet", "command": "go vet ./...", "type": "lint"}
  ],
  "budgets": {
    "max_spec_cycles": 3,
    "max_task_retries": 1,
    "max_redecomposition_passes": 1,
    "max_task_duration_seconds": 300,
    "max_run_duration_seconds": 3600,
    "max_run_cost_usd": 50.0
  },
  "models": {
    "planner": "high",
    "executor": "medium"
  }
}
```

**fixture-greeter-execution.json**:
```json
{
  "always_run": [
    {"name": "unit-tests", "command": "go test ./...", "type": "test"},
    {"name": "vet", "command": "go vet ./...", "type": "lint"}
  ],
  "budgets": {
    "max_spec_cycles": 3,
    "max_task_retries": 1,
    "max_redecomposition_passes": 1,
    "max_task_duration_seconds": 300,
    "max_run_duration_seconds": 3600,
    "max_run_cost_usd": 50.0
  },
  "models": {
    "planner": "high",
    "executor": "medium"
  }
}
```

### 4.5 fixture-multipackage

A Go module with multiple internal packages, used for scoped context verification (Scenario 1b, AC 3) and as an alternative broad-refactor target (Scenario 8).

```bash
mkdir -p /tmp/gromit-fixtures/fixture-multipackage
cd /tmp/gromit-fixtures/fixture-multipackage
git init
go mod init github.com/test/fixture-multipackage
mkdir -p internal/auth internal/refund internal/billing specs
```

**internal/auth/auth.go**:
```go
package auth

// ValidateToken checks if a token is valid.
func ValidateToken(token string) bool {
    return token != ""
}
```

**internal/auth/auth_test.go**:
```go
package auth

import "testing"

func TestValidateToken(t *testing.T) {
    if !ValidateToken("abc") {
        t.Error("expected valid token")
    }
    if ValidateToken("") {
        t.Error("expected invalid token")
    }
}
```

**internal/refund/refund.go**:
```go
package refund

// Refund represents a refund request.
type Refund struct {
    OrderID string
    Amount  int
    Reason  string
}

// Process processes a refund request. Returns true if approved.
func Process(r Refund) bool {
    return r.Amount > 0 && r.OrderID != ""
}
```

**internal/refund/refund_test.go**:
```go
package refund

import "testing"

func TestProcess(t *testing.T) {
    r := Refund{OrderID: "order-1", Amount: 100, Reason: "defective"}
    if !Process(r) {
        t.Error("expected refund to be approved")
    }
}
```

**internal/billing/billing.go**:
```go
package billing

// Invoice represents a billing invoice.
type Invoice struct {
    CustomerID string
    Total      int
}

// CreateInvoice creates a new invoice.
func CreateInvoice(customerID string, total int) Invoice {
    return Invoice{CustomerID: customerID, Total: total}
}
```

**internal/billing/billing_test.go**:
```go
package billing

import "testing"

func TestCreateInvoice(t *testing.T) {
    inv := CreateInvoice("cust-1", 500)
    if inv.Total != 500 {
        t.Errorf("expected total 500, got %d", inv.Total)
    }
}
```

```bash
cd /tmp/gromit-fixtures/fixture-multipackage
git add -A && git commit -m "initial: multi-package with auth, refund, billing"
```

**specs/add-refund-endpoint.md** (Scoped Context - Scenario 1b):
```markdown
# Add Refund Endpoint

## spec_id
add-refund-endpoint

## Title
Add a partial refund capability to the refund package

## Problem
The refund package only supports full refunds. We need partial refunds.

## In-Scope
- Add a `ProcessPartial(r Refund, percentage int) bool` function to `internal/refund/refund.go`
- Add tests for ProcessPartial in `internal/refund/refund_test.go`

## Out-of-Scope
- No changes to auth or billing packages
- No changes to the existing Process function

## Acceptance Criteria
1. `ProcessPartial(Refund{OrderID: "o1", Amount: 100}, 50)` returns true
2. `ProcessPartial(Refund{OrderID: "", Amount: 100}, 50)` returns false
3. `ProcessPartial(Refund{OrderID: "o1", Amount: 100}, 0)` returns false
4. All existing tests continue to pass
5. `go vet ./...` passes

## Architectural Constraints
- All changes stay in the `internal/refund` package

## Validation
- `go test ./internal/refund/...`
- `go vet ./...`
```

**Execution policy** (`fixture-multipackage-execution.json`):
```json
{
  "always_run": [
    {"name": "unit-tests", "command": "go test ./...", "type": "test"},
    {"name": "vet", "command": "go vet ./...", "type": "lint"}
  ],
  "budgets": {
    "max_spec_cycles": 3,
    "max_task_retries": 1,
    "max_redecomposition_passes": 1,
    "max_task_duration_seconds": 300,
    "max_run_duration_seconds": 3600,
    "max_run_cost_usd": 50.0
  },
  "models": {
    "planner": "high",
    "executor": "medium"
  }
}
```

---

## 5. Regression Checks

### 5.1 Smoke Test Sequence

After any code change to Spec 0002a packages, run this minimal sequence:

```bash
# 1. Build
cd /Users/dabrams/gromit
go build -o gromit-next ./cmd/gromit-next/

# 2. Unit tests for all Spec 0002a packages
go test ./internal/next/execpolicy/...
go test ./internal/next/runstore/...
go test ./internal/next/planner/...
go test ./internal/next/executor/...
go test ./internal/next/validator/...
go test ./internal/next/evidence/...
go test ./internal/next/specloop/...

# 3. Quick happy-path end-to-end
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
# Confirm: terminal state is ready_for_review

# 4. Quick dry-run
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md --dry-run
# Confirm: plan output, no execution

# 5. CLI inspection
./gromit-next exec list --project fixture-calc
./gromit-next exec show latest --project fixture-calc
./gromit-next spec list --project fixture-calc
# Confirm: commands return without error, output is formatted
```

### 5.2 What to Re-test After Specific Changes

| Changed Package | Re-run Scenarios |
|----------------|-----------------|
| `specloop` | All scenarios (1-12) including 11d |
| `planner` | 1, 2, 4, 6, 8 |
| `executor` | 1, 2, 7, 8, 11b, 11c, 11d |
| `validator` | 1, 2, 3, 7, 10, 11 |
| `evidence` | 1, 3, 12 (check artifact completeness and CLI evidence display) |
| `runstore` | 1, 11d, 12 (check storage, concurrency, and retrieval) |
| `execpolicy` | 3, 10, 11, 11b (budget enforcement) |
| CLI wiring | 6, 12 (flags and subcommands) |

### 5.3 Pre-merge Checklist

Before merging Spec 0002a implementation:

- [ ] All unit tests pass: `go test ./internal/next/...`
- [ ] Scenario 1 (Happy Path) passes
- [ ] Scenario 1b (Scoped Context) verifies task-packet scoping
- [ ] Scenario 2 (Fix Cycle) produces fix plan without replanning completed work
- [ ] Scenario 3 (Budget Exhausted) produces `needs_human`
- [ ] Scenario 4 (Blocked -- Bad Spec) produces `blocked`
- [ ] Scenario 4b (Blocked -- Provider) produces `blocked` with provider error
- [ ] Scenario 5 (Missing Project Cell) produces `blocked`
- [ ] Scenario 6 (Dry Run) produces plan without execution, is not resumable
- [ ] Scenario 7 (Task Repair) respects `max_task_retries` budget
- [ ] Scenario 8 (Task Split) triggers `needs_split` or validates upfront decomposition (see fallback)
- [ ] Scenario 9 (Multi-Project Isolation) shows no cross-contamination
- [ ] Scenario 10 (Cost Limit) produces `blocked` (not `needs_human`)
- [ ] Scenario 11 (Run Timeout) produces `blocked` on `max_run_duration_seconds`
- [ ] Scenario 11b (Per-Task Timeout) enforces per-task timeout
- [ ] Scenario 11c (All Tasks Failed) produces `needs_human` via ExecuteStage
- [ ] Scenario 11d (Concurrent Same-Spec) shows isolation
- [ ] Scenario 12 (CLI Inspection) commands all work, including `--full` and all 5 statuses
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` produces no output
- [ ] No Gromit files committed to fixture repos' main branches
