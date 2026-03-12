# Spec 0002a — Core Execution Loop and Deterministic Validation

## Summary

Build the core execution kernel for Gromit Next. Given an attached project from Spec 0001 and an approved spec, Gromit creates an isolated worktree, compiles a spec packet, plans and decomposes the spec into small tasks, executes each task through a bounded TaskLoop, runs deterministic validation, and emits a validation evidence bundle. The run terminates in one of three machine states: `ready_for_review`, `needs_human`, or `blocked`.

This spec implements the inner execution loop with deterministic gates only. LLM-driven review, acceptance evaluation, and fix-cycle replanning from those stages are deferred to Spec 0002b.

---

## Problem

Spec 0001 gives Gromit a workspace-level project cell, context compiler, and agent guide. That solves project understanding and multi-project isolation, but does not turn approved intent into validated code.

The next step is a small, explicit execution kernel that consumes project/spec/task context, works inside a bounded loop, produces evidence instead of self-declared success, and keeps final product judgment with the human.

---

## Goals

### Primary

- Execute one approved spec against one attached project.
- Run execution in an isolated git worktree.
- Compile and use scoped context packets (project, spec, task).
- Generate a plan and decompose the spec into small, verifiable tasks.
- Validate planner output before execution.
- Execute each task through a tiny build/verify/fix loop.
- Run full-project deterministic validation before completion.
- Produce a validation evidence bundle suitable for human review.
- Preserve external run records and artifacts per project.
- Record raw metrics signal (timings, tokens, retries, models) for future vision metrics.

### Secondary

- Keep the automated loop architecture small and inspectable.
- Make every failure legible enough for a human to understand the next action.
- Produce run artifacts that can later feed vision metrics and auditability.

---

## Non-goals

- LLM-driven code review (Spec 0002b)
- Acceptance evaluation against spec criteria (Spec 0002b)
- Fix-cycle replanning from review/acceptance failures (Spec 0002b)
- Outer multi-spec planning loop
- Backlog management or queue scheduling
- Parallel spec execution
- PR creation or merge automation
- Cross-project learning promotion
- Automatic project doctrine rewriting
- Indefinite retries
- Resume/recovery beyond artifact preservation for inspection
- Vision metrics reporting/analysis layer (raw signal is recorded, formalization is later)
- Recording VISION.md review outcome labels (`accepted`, `rework_implementation_gap`, `rework_vision_change`) — deferred to Spec 0003 which formalizes human review capture

---

## Use Cases / Scenarios

### Happy path: clean spec execution

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

[init] Created run 20260311-143022-a1b2c3
[init] Worktree created at /tmp/gromit-worktrees/payments-api-20260311-a1b2c3
[compile] Spec packet compiled (doctrine: 3 rules, architecture: 2 modules, validation: 4 commands)
[plan] Planner produced 4 tasks
[execute] Task t-001: add refund handler ... done (1 attempt, 23s)
[execute] Task t-002: add refund tests ... done (1 attempt, 18s)
[execute] Task t-003: wire up route ... done (1 attempt, 8s)
[execute] Task t-004: update OpenAPI spec ... done (2 attempts, 31s)
[validate] Final validation: 6/6 checks passed
[evidence] Bundle written to runs/20260311-143022-a1b2c3/evidence/

Terminal state: ready_for_review
Review: runs/20260311-143022-a1b2c3/evidence/review.md
```

The human reads review.md, inspects the worktree, and decides whether to merge.

### Fix cycle: validation failure triggers replan

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

[init] Created run 20260311-150000-b2c3d4
...
[execute] Task t-001: add refund handler ... done
[execute] Task t-002: add refund tests ... done
[execute] Task t-003: wire up route ... done
[validate] Final validation: 5/6 checks passed
  FAIL: go vet ./... — unreachable code in refund_handler.go:45
[replan] Cycle 2 (fix): 1 task targeting validation failure
[execute] Task t-004: fix unreachable code in refund_handler.go ... done (1 attempt, 6s)
[validate] Final validation: 6/6 checks passed
[evidence] Bundle written

Terminal state: ready_for_review
```

The validation failure triggered a fix cycle. The planner produced one targeted task. Total: 2 cycles.

### Task repair: targeted check fails, agent fixes it

```
[execute] Task t-002: add refund tests
  Implement: agent writes test file
  Inspect: go test ./internal/refund/... FAIL (TestRefundNegativeAmount undefined)
  Repair: agent fixes test with correct function name
  Re-inspect: go test ./internal/refund/... PASS
  done (2 attempts, 31s)
```

The task-level repair handled a trivial error without consuming a spec cycle.

### Task needs_split: task too broad

```
[execute] Task t-001: implement refund system
  Implement: agent touches 8 files across 4 packages
  Inspect: failures in 3+ packages → needs_split
  Re-decompose: planner splits into 3 sub-tasks (budget: 1/1 used)
  [execute] Task t-001a: add refund model ... done
  [execute] Task t-001b: add refund handler ... done
  [execute] Task t-001c: add refund tests ... done
```

### Budget exhausted: needs_human

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

...
[validate] Cycle 1: 2/6 checks failed (test failures in integration suite)
[replan] Cycle 2 (fix): 2 tasks targeting failures
[execute] ...
[validate] Cycle 2: 1/6 checks failed (flaky integration test)
[replan] Cycle 3 (fix): 1 task targeting failure
[execute] ...
[validate] Cycle 3: 1/6 checks failed (same flaky test)
[budget] max_spec_cycles (3) exhausted

Terminal state: needs_human
Blocker: Integration test TestRefundConcurrency fails intermittently.
  Tried: 3 fix cycles, each targeting the test. Failure appears non-deterministic.
  Recommended action: Investigate test flakiness manually. The refund endpoint
  implementation itself passes all other checks.

Review: runs/20260311-160000-c3d4e5/evidence/review.md
```

### Blocked: planner fails

```
$ gromit-next exec spec --project payments-api --spec ./specs/vague-spec.md

[init] Created run 20260311-170000-d4e5f6
[compile] Spec packet compiled
[plan] Planner produced invalid output (empty task list)
[plan] Retry: planner produced invalid output again (missing required fields)

Terminal state: blocked
Blocker: Planner failed to produce valid task list after 2 attempts.
  The spec may be too vague or missing concrete acceptance criteria.
  Recommended action: Revise the spec with clearer scope and acceptance criteria.
```

### Blocked: infrastructure failure

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

[init] Creating worktree...
  ERROR: git worktree add failed — lock held by another process

Terminal state: blocked
Blocker: Could not create git worktree. Git lock contention.
  Recommended action: Check for other gromit runs or git processes on this repo.
```

### Blocked: provider unavailability

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

[init] Created run 20260311-190000-f6g7h8
[compile] Spec packet compiled
[plan] Planner invocation failed: provider returned 503 Service Unavailable
[plan] Retry: provider returned 503 Service Unavailable

Terminal state: blocked
Blocker: LLM provider unavailable after 2 attempts.
  Recommended action: Check provider status and retry later.
```

### Dry run: review plan before committing budget

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md --dry-run

[init] Created run 20260311-180000-e5f6g7 (dry-run)
[compile] Spec packet compiled
[plan] Planner produced 4 tasks:
  t-001: Add refund data model and repository layer
  t-002: Implement refund HTTP handler with validation
  t-003: Add unit and integration tests for refund flow
  t-004: Update OpenAPI spec and route registration

Dry run complete. Plan saved to runs/20260311-180000-e5f6g7/plan.md
Run `gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md` to execute.
```

Note: Dry-run runs are not resumable. Executing the spec creates a fresh run.

### Listing specs and runs

```
$ gromit-next spec list --project payments-api

ID                     Title                          Status
add-refund-endpoint    Add refund endpoint            ready
update-auth-flow       Update authentication flow     completed
migrate-to-postgres    Migrate to PostgreSQL          needs_attention

$ gromit-next exec list --project payments-api

Run ID                      Spec                    State             When
20260311-143022-a1b2c3      add-refund-endpoint     ready_for_review  2m ago
20260310-091500-x1y2z3      update-auth-flow        ready_for_review  1d ago
20260311-150000-b2c3d4      migrate-to-postgres     needs_human       15m ago

$ gromit-next exec show 20260311-143022-a1b2c3

Run: 20260311-143022-a1b2c3
Spec: add-refund-endpoint
State: ready_for_review
Cycles: 1
Tasks: 4/4 completed
Validation: 6/6 passed
Duration: 1m 22s
Cost: $0.42

Worktree: /tmp/gromit-worktrees/payments-api-20260311-a1b2c3
Evidence: ~/.local/share/gromit/projects/payments-api/runs/20260311-143022-a1b2c3/evidence/review.md
```

### Multi-project isolation

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md &
$ gromit-next exec spec --project user-service --spec ./specs/add-profile-api.md &

# Both runs execute independently:
# - Separate worktrees (gromit/spec-add-refund-..., gromit/spec-add-profile-...)
# - Separate run directories under their respective project cells
# - Separate context packets compiled from their own project cells
# - No shared state
```

---

## Design Principles

### 1. Same control law at two grains

Both automated loops use PDSA. At the spec level: plan/decompose, execute task loops, study validation evidence, act by stopping or re-planning. At the task level: plan one small change, implement it, study targeted checks, keep/retry/escalate.

### 2. Human controls product; loop controls process

The human decides whether the outcome belongs in the product. The loop owns only the mechanics of turning an approved spec into code and evidence.

### 3. Compile to architecture, not prompt sprawl

The loop consumes structured project/spec/task packets compiled from Spec 0001. No giant implicit prompt, no repo-local Gromit state.

### 4. Success requires evidence

The system does not declare success because the model says the work is done. Success requires deterministic validation results and a presentable evidence bundle.

---

## Spec 0001 Extensions

This spec extends the project cell layout from Spec 0001 with:

- `policy/execution.json` — execution policy config (new directory)
- `runs/` — run records and artifacts (new directory)
- `specs_dir` field in `project.json` — configurable path to specs directory in target repo

These additions do not modify any existing Spec 0001 artifacts or directories.

---

## Package Architecture

```
internal/next/
  execpolicy/     # Execution policy config (always-run checks, budgets, model tiers)
  runstore/       # Run record CRUD, artifact layout, events log
  planner/        # Agent-driven plan generation, task decomposition, plan validation
  executor/       # Agent invocation, worktree inspection, result extraction
  validator/      # Runs validation commands (always-run, targeted, final)
  evidence/       # Assembles evidence bundle from run artifacts
  specloop/       # SpecLoop + TaskLoop + stage pipeline orchestration
```

Packages deferred to Spec 0002b:
```
  review/         # Multi-facet LLM code review
  acceptor/       # Evaluates acceptance criteria against evidence
```

Dependency flow (top = leaf, bottom = root):

```
execpolicy   runstore                (leaf packages)
    \          |
     \    planner  executor  validator  evidence
      \      \        |         |        /
       -------\-------+---------+-------/
               \                       /
                -----  specloop  -----
```

All packages consume from Spec 0001 as needed (contextpkt, projectcell, artifact, provider, etc).

---

## Provider Model

### Tiers via existing provider abstraction

The execution policy references abstract tiers (low, medium, high, xhigh), not concrete models. Tier-to-model resolution uses the existing `internal/provider` package's `Provider.ModelForTier()` interface, extended as needed to support `xhigh` and reasoning level settings.

No separate `providers.json` file. The existing provider abstraction is the single source of truth for tier resolution.

Extending the existing provider to support the `xhigh` tier is a deliverable of this spec. The scope is small: add one tier constant and its model mapping.

**Execution policy** references tiers only:

```json
{
  "models": {
    "planner": "high",
    "executor": "medium"
  }
}
```

---

## Execution Policy

Lives in the project cell at `policy/execution.json`. Optional; sensible defaults if absent.

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

- **always_run**: Deterministic checks that fire after every task and during final validation. These are human-declared checks, separate from Spec 0001's auto-extracted `validation.json`. Both sources run during final validation with no deduplication — redundancy is cheap, missing a check is expensive.
- **budgets**:
  - `max_spec_cycles` — caps total Plan-Execute-Validate passes (default 3; cycle 1 is initial, cycles 2-3 are fix cycles)
  - `max_task_retries` — repair attempts per task after initial failure (default 1; so 2 total attempts per task: initial + 1 retry; 0 means no repair attempts)
  - `max_redecomposition_passes` — total re-decomposition passes per run, not per task (default 1; if 3 tasks need splitting and budget is 1, only the first gets re-decomposed, others are marked `failed`)
  - `max_task_duration_seconds` — per executor agent invocation timeout (default 300)
  - `max_run_duration_seconds` — total wall-clock timeout for the entire run (default 3600)
  - `max_run_cost_usd` — total LLM cost cap for the run (default 50.0)

**Cost enforcement**: `max_run_cost_usd` is enforced by accumulating cost from token counts per invocation using the provider's published pricing. The cost check runs between task executions and between stages. When exceeded, the current stage completes but no further stages or tasks execute, and the run transitions to `blocked` with a `budget_exceeded` event.

- **models**: Tier selection per execution phase.

---

## Stage Pipeline

The SpecLoop is a bounded PDSA cycle implemented as a stage pipeline.

### Stages

```go
type Stage interface {
    Name() string
    Run(ctx context.Context, run *RunState) (NextAction, error)
}

type NextAction struct {
    Kind    ActionKind      // Continue, ReplanFrom, NeedsHuman, Blocked
    Context *FailureContext // populated on ReplanFrom
}
```

A non-nil `error` return from `Stage.Run()` is always treated as `blocked`. The EvidenceStage still runs to capture whatever was collected before the infrastructure failure.

### Stage-to-terminal-state mapping

| Stage | Can produce `Blocked`? | Can produce `ReplanFrom`? | Can produce `NeedsHuman`? |
|-------|----------------------|-------------------------|-------------------------|
| InitStage | Yes (worktree, project cell) | No | No |
| CompileStage | Yes (missing artifacts) | No | No |
| PlanStage | Yes (invalid output after retry) | No | No |
| ExecuteStage | No | No | Yes (all tasks failed); partial failure produces `Continue` |
| ValidateStage | No | Yes (failures to fix) | No |
| EvidenceStage | No | No | No |
| FinalizeStage | No | No | No |

In Spec 0002a, only ValidateStage can trigger `ReplanFrom`. Spec 0002b adds ReviewStage and AcceptStage which also produce `ReplanFrom`.

### Pipeline (Spec 0002a)

```
InitStage       -> create run record, worktree, snapshot configs, copy spec
CompileStage    -> build spec packet from project cell
PlanStage       -> invoke planner agent, validate output, produce plan.md + tasks.json
ExecuteStage    -> run TaskLoop for each task; partial failure -> Continue (failed tasks recorded in ledger, ValidateStage catches resulting failures); all tasks failed -> NeedsHuman
ValidateStage   -> run final validation (always-run + project cell checks)
EvidenceStage   -> assemble bundle, write review.md, metrics.json
FinalizeStage   -> log terminal state, return result
```

### Pipeline (Spec 0002b extends with)

```
... -> ValidateStage -> ReviewStage -> AcceptStage -> EvidenceStage -> FinalizeStage
```

### Bounded loop (Spec 0002a)

```
Init -> Compile -> Plan -> Execute -> Validate -> Evidence -> Finalize
                    ^                    |
                    |____________________|
                     validation failures loop back to Plan
```

The SpecLoop runner iterates stages. When ValidateStage returns `ReplanFrom`, the runner jumps back to PlanStage with failure context. The budget (`max_spec_cycles`) caps the total number of full cycles. When budget is exhausted with remaining failures, the run terminates as `needs_human`.

### Fix plans, not fresh replans

When looping back to Plan, the planner receives:

- The original plan and completed tasks (with their results)
- What specifically failed (validation errors with full output)
- The current worktree state (diff summary)

Its job is to produce only the new tasks needed to fix the failures. It must not re-plan completed work or change the approach.

Fix-cycle plans carry metadata:

```json
{
  "cycle": 2,
  "kind": "fix",
  "parent_cycle": 1,
  "failures_addressed": ["validation: TestFoo failed -- expected 3, got 2"],
  "tasks": [...]
}
```

Fix-cycle tasks are appended to the task ledger, not replacing it.

Note: fix-plan scoping is prompt-enforced. Structural enforcement (e.g., flagging fix tasks that overlap with completed work) may be added as a validation step in a later spec. This is a known limitation.

---

## Plan Validation

Between PlanStage output and ExecuteStage consumption, the planner output must be validated:

1. **JSON schema validation** — `tasks.json` must parse as valid JSON matching the expected task schema.
2. **Non-empty task list** — A plan with zero tasks is invalid.
3. **Required fields** — Each task must have: task_id, objective, expected_touched_area, proof_checks.
4. **No duplicate task IDs** — Within a cycle or across cycles. Task IDs are globally sequential across cycles within a run (t-001, t-002, ... t-005, t-006 for fix cycle tasks). No cycle prefix needed.

If validation fails, PlanStage retries the planner once with the validation errors as context. If the retry also fails, the run transitions to `blocked` with the planner failure details.

---

## TaskLoop

Inside ExecuteStage. One task at a time. Tasks execute sequentially within a cycle — this is a deliberate design decision, not just an implementation detail, because budget semantics (e.g., `max_redecomposition_passes` is a global budget consumed in execution order) depend on deterministic task ordering.

```
Implement -> targeted checks + always-run checks -> pass? -> done
                                                 -> fail? -> one repair attempt -> recheck -> done/failed/needs_split
```

### Steps

1. **Implement** — Invoke executor agent with task packet in the worktree. Agent runs with full autonomy (no permission prompts). Gromit captures stdout.
2. **Inspect** — Gromit inspects the worktree: git diff, targeted checks from the task's proof plan, always-run checks from execution policy. Gromit determines success from deterministic signals, not agent self-report.
3. **If all pass** -> `done`, record task result + metrics.
4. **If failures** -> invoke executor agent again with: original task objective, specific failure output, current diff. One shot to fix.
5. **Re-inspect** — same checks.
6. **If pass** -> `done`.
7. **If fail** -> evaluate scope: if the task's failures span multiple unrelated areas or the touched files significantly exceed the expected_touched_area, it is `needs_split`; otherwise `failed`.

### `needs_split` decision heuristic

The `needs_split` vs `failed` distinction is determined by Gromit, not the agent:
- Failures in 3+ distinct packages/directories -> `needs_split`
- Files changed exceed expected_touched_area by 2x+ -> `needs_split`
- Otherwise -> `failed`

This avoids relying on agent self-report for the split decision.

### Re-decomposition

Re-decomposition is **internal to ExecuteStage**, not a full spec cycle. When a task returns `needs_split`:

1. Gromit reverts the task's changes (`git checkout` the files touched by the failed task) so that sub-tasks start from a clean baseline, not from a dirty partial implementation.
2. ExecuteStage invokes the planner with the failed task + failure context to produce replacement sub-tasks.
3. The sub-tasks are appended to the task ledger and executed in sequence.
4. This consumes one `max_redecomposition_passes` budget unit.
5. Sub-tasks that themselves fail cannot trigger further re-decomposition — they are marked `failed`.

Re-decomposition does not consume a `max_spec_cycles` budget unit. It is a task-level operation, not a spec-level replan.

### Task terminal states

- **done** — task completed and verified.
- **needs_split** — task too broad; re-decomposed within ExecuteStage if budget allows, otherwise marked `failed`.
- **failed** — task could not be completed within retry budget.

### Task result record

```json
{
  "task_id": "t-003",
  "status": "done",
  "attempts": 2,
  "targeted_checks": {"pass": 3, "fail": 0},
  "always_run_checks": {"pass": 2, "fail": 0},
  "files_changed": ["internal/next/validator/runner.go"],
  "tokens_used": 12400,
  "duration_ms": 34000,
  "model_tier": "medium"
}
```

---

## Validation

Two levels, both use the same runner.

### Task-targeted validation

Checks relevant to the task's touched area, defined in the task's proof plan. Plus always-run checks from execution policy.

### Final validation

All always-run checks (from execution policy) + all project cell validation commands (from Spec 0001's `validation.json`). Runs after all tasks complete.

These are two separate sources with different intent:
- `always_run` = human-declared "always run these" checks
- `validation.json` = auto-extracted "we found these" commands

Both run. No deduplication. A run cannot reach `ready_for_review` if final validation fails.

---

## Worktree Lifecycle

### Creation

InitStage creates a git worktree from the target repo's main branch using a distinct branch name: `gromit/spec-<spec-id>-<run-id>`. This avoids conflicts between concurrent runs.

Concurrent runs of the same spec are allowed — the run-id in the branch name prevents conflicts. The most recent terminal run determines the spec's derived status.

If worktree creation fails (disk full, git lock contention, branch already exists), InitStage transitions immediately to `blocked` with the error details.

### Trust boundary

The executor agent runs with the same filesystem and network permissions as the gromit-next process. The worktree provides git isolation but not filesystem or network sandboxing. This is acceptable for the current model where the human operator runs Gromit against their own repos. Sandboxing is a non-goal for this spec.

### Cleanup

FinalizeStage is responsible for worktree cleanup regardless of terminal state. However, if the terminal state is `ready_for_review` or `needs_human`, the worktree is preserved (not deleted) so the human can inspect it. The worktree path is recorded in `run.json`. Cleanup of old worktrees is manual for now.

---

## Terminal States

### `ready_for_review`

All deterministic validation passes. Evidence bundle complete. Human still decides whether to accept into the product.

In Spec 0002a, this means all final validation passed. In Spec 0002b, this additionally requires review clearance and acceptance criteria passing.

### `needs_human`

Budget exhausted with remaining validation failures, or all tasks failed. Blocker summary populated: what failed, what was tried, recommended next action. Worktree and all artifacts preserved.

### `blocked`

Unrecoverable infrastructure failure. Specific causes:
- InitStage: worktree creation failure, project cell not found
- CompileStage: required artifacts missing or corrupt
- PlanStage: planner produced invalid output after retry, provider unavailable
- Any stage: run timeout exceeded, cost limit exceeded

For `needs_human` and `blocked`, the evidence bundle is still emitted with everything collected so far.

---

## Context Model

Locks in the context relationship from Spec 0001.

### Project packet

Broad stable context for the project.

### Spec packet

A scoped slice of project context plus the approved spec. Includes relevant doctrine, architecture boundaries, source map entries, validation surfaces, glossary terms, and the spec itself.

### Task packet

A scoped slice of spec/project context plus one task objective and its proof surface. Excludes clearly unrelated project context.

Packets are scoped first, budgeted second.

---

## Approved Spec Contract

The minimal approved spec must contain:

- spec_id
- title
- problem / intent
- in-scope behavior
- out-of-scope constraints
- acceptance criteria
- relevant architectural constraints
- validation expectations (if any)

A spec without clear acceptance criteria is not runnable under this spec.

Note: The approved spec uses markdown with structured sections. A frontmatter or sidecar format may be adopted later but is not required for this spec.

---

## Spec Discovery

Specs live in a convention-based directory in the target repo (e.g., `specs/` or `docs/specs/`), configurable via `specs_dir` in `project.json`. Gromit scans that directory.

Status is derived from run history in the project cell:

- **completed** — has a run that ended in `ready_for_review` and was accepted by human.
- **running** — has an active run (currently executing).
- **needs_attention** — has a `needs_human` run with no subsequent run.
- **ready** — approved, no run yet (or prior run was rejected/abandoned).
- **draft** — exists but not yet approved.

---

## Run Storage Model

All run artifacts live in the external workspace, per project.

```
~/.local/share/gromit/projects/<project-id>/
  runs/
    <run-id>/
      run.json                # Canonical run record
      spec.md                 # Copy of approved spec
      spec-packet.md          # Compiled spec context
      plan.md                 # Human-readable plan
      tasks.json              # Machine-readable task list
      events.jsonl            # Append-only event log
      execution-policy.json   # Snapshot of policy used
      tasks/
        <task-id>/
          task-packet.md      # Compiled task context
          result.json         # Task outcome + metrics
          agent-output.txt    # Raw agent stdout
      worktree/               # Path reference to git worktree
      evidence/
        summary.md            # Human-facing run summary
        diff-summary.md       # What changed
        task-results.json     # Aggregated task outcomes
        validation.json       # Final validation results
        review.md             # Decision sheet for human
        metrics.json          # Raw signal: timings, tokens, retries, models
```

Config snapshots (execution policy) are copied at run start so runs are fully reproducible. Provider tier resolution is recorded per-invocation in metrics.json (which model was actually used), not as a separate snapshot.

---

## Evidence Bundle

### review.md — the decision sheet

Contains: terminal state, what changed, cycle history, validation results, known risks, recommended action.

Clearly distinguishes: what was proven, what failed, whether the gap is a fixable execution failure or something requiring human judgment.

### metrics.json — raw signal

Captures per run:
- total cycles, task counts
- success/retry/replan rates
- per-invocation records: phase, tier, model used, tokens in/out, duration, success/failure
- aggregate: total tokens, total cost estimate, wall clock duration
- human intervention flag (was budget exhausted?)

Everything needed for future vision metrics computation. Nothing thrown away.

---

## CLI Commands

### `gromit-next exec spec`

Run a spec against a project.

```
gromit-next exec spec --project payments-api --spec ./specs/spec-0002.md
gromit-next exec spec --project payments-api --spec ./specs/spec-0002.md --dry-run
```

`--dry-run` creates the run record, compiles the spec packet, and runs the planner, but stops before executing tasks. Useful for reviewing the plan before committing budget.

### `gromit-next exec show`

Inspect a run.

```
gromit-next exec show <run-id>
gromit-next exec show <run-id> --full
gromit-next exec show latest --project payments-api
```

Default: terminal state + summary. `--full`: complete evidence bundle.

### `gromit-next exec list`

List runs for a project.

```
gromit-next exec list --project payments-api
```

### `gromit-next spec list`

List available specs with derived status.

```
gromit-next spec list --project payments-api
```

---

## Observability

The event log (`events.jsonl`) records:

- run_started
- spec_packet_compiled
- plan_created (with cycle number and kind: initial/fix)
- plan_validation_result (pass/fail)
- task_created
- task_started
- task_validation_result (targeted + always-run check results)
- task_completed / task_failed / task_needs_split
- redecomposition_triggered (with task_id and sub-task count)
- final_validation_result
- replan_triggered (with failure context summary)
- budget_exceeded (which budget: cycles, time, cost)
- terminal_state (with reason)

Enough to answer: what spec, what task, what context, what validation, why the system stopped, what a human should do next.

---

## Acceptance Criteria

1. **Single-spec execution** — Given an attached project and an approved spec, Gromit can create a run, plan the work, execute task loops, and emit a validation evidence bundle.
2. **Zero repo pollution** — Running a spec does not commit or create Gromit files in the target repo. Tracked files remain untouched except for code changes in the isolated worktree.
3. **Scoped context** — Spec packets and task packets are scoped slices, not full supersets. Task packets exclude clearly unrelated project context.
4. **Bounded retries** — A task receives at most `max_task_retries` repair attempts (default 1). Re-decomposition is bounded to `max_redecomposition_passes` total per run (default 1). Spec-level cycles are bounded by `max_spec_cycles` (default 3).
5. **Deterministic final gate** — `ready_for_review` is impossible if final deterministic validation fails.
6. **Human review preserved** — Final machine success state is `ready_for_review`, not `accepted`.
7. **Artifact durability** — After any terminal state, the run record, task ledger, validation results, metrics, and review artifact remain available in the external workspace.
8. **Multi-project isolation** — Runs for different projects do not share context packets, evidence bundles, or project doctrine.
9. **Useful failure surface** — `needs_human` and `blocked` runs include a concise blocker summary and recommended next action.
10. **Fix plans are scoped** — Re-plan cycles produce only tasks targeting specific validation failures, not fresh replans of the entire spec.
11. **Metrics preserved** — Every run emits metrics.json with token usage, timings, retry counts, cost estimates, and model tiers per invocation.
12. **Plan validation** — Invalid planner output (malformed JSON, empty task list, missing required fields) is caught before execution and retried once, then transitions to `blocked`.
13. **Timeout and cost enforcement** — Runs respect `max_task_duration_seconds`, `max_run_duration_seconds`, and `max_run_cost_usd` budgets.
14. **Worktree isolation** — Each run uses a distinct git worktree with a unique branch name. Concurrent runs do not conflict.

---

## Evidence Required

- Integration test on at least one fixture repo showing end-to-end spec execution through `ready_for_review`.
- Integration test showing a run ending in `needs_human` (validation failures that exhaust budget).
- Integration test showing a run ending in `blocked` (e.g., invalid planner output).
- Proof that run artifacts are stored outside the target repo.
- Example run where a task touching `internal/refund/` receives a task packet that excludes architecture facts about unrelated packages (e.g., `internal/auth/`).
- Example showing fix-plan cycle targeting specific validation failures without replanning completed work.
- Verification that task retry count respects `max_task_retries`.
- Verification that re-decomposition respects `max_redecomposition_passes` (second `needs_split` is marked `failed`).
- Verification that deterministic validation failure prevents `ready_for_review`.
- Test with two fixture projects demonstrating isolation (no shared state).
- Verification that `needs_human` and `blocked` runs include blocker summary with recommended next action.
- Verification that metrics.json contains per-invocation token counts, timings, and model tiers.
- Verification that timeout/cost limits are enforced (run terminates when exceeded).
- Verification that `--dry-run` produces plan without executing tasks.
- Verification that events.jsonl contains all specified event types for a complete run.

---

## Open Questions (Resolved)

1. **Copy spec into run dir or reference?** -> Copy. Runs are self-contained snapshots.
2. **Same provider abstraction for planner and executor?** -> Yes, reuse existing `internal/provider` tier abstraction.
3. **`exec show` in this spec?** -> Yes, along with `exec list` and `spec list`.
4. **Vision metrics now or later?** -> Record raw signal now (metrics.json). Formalize reporting later. Nothing thrown away.
5. **`always_run` vs `validation.json` overlap?** -> Separate sources, different intent. Both run, no deduplication.
6. **`providers.json` file?** -> No. Reuse existing provider interface for tier resolution.
7. **Re-decomposition: spec cycle or internal to ExecuteStage?** -> Internal to ExecuteStage. Does not consume spec cycle budget.
8. **`needs_split` vs `failed` decision?** -> Deterministic heuristic (file spread, package count), not agent self-report.

---

## Recommended Sequencing

**Spec 0002b — LLM Review, Acceptance Evaluation, and Fix-Cycle Replanning**

Adds ReviewStage (2 facets: spec_alignment, code_quality; configurable threshold), AcceptStage (per-criterion evaluation), and extends the fix-cycle replan loop to cover review and acceptance failures. Includes VISION.md review outcome label deferral to Spec 0003.

**Spec 0003 — Review Capture, Cycle Records, and Learning Promotion Boundaries**

Captures human review outcomes with VISION.md labels (`accepted`, `rework_implementation_gap`, `rework_vision_change`), feeds the vision metrics loop, and establishes boundaries for promoting learnings back into project doctrine.
