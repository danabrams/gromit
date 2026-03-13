# Spec 0002a — Execution Prompt

> **REQUIRED:** Use `superpowers:executing-plans` to implement this plan task-by-task.
> **Plan document:** `docs/plans/2026-03-11-spec-0002a-execution-plan.md`
> **Testing plan:** `docs/plans/2026-03-11-spec-0002a-testing-plan.md`

---

## Project Context

- **Language/runtime:** Go 1.26, CLI built with `github.com/spf13/cobra`
- **Project root:** `/Users/dabrams/gromit`
- **Module path:** `github.com/danabrams/gromit`
- **CLI entry point:** `cmd/gromit-next/main.go` (already has `projectCmd` and `contextCmd`)
- **Existing Spec 0001 packages (reuse, do not modify):** `internal/next/` subtree — `artifact/`, `contextpkt/`, `doctrine/`, `enrich/`, `extract/`, `fact/`, `guide/`, `infer/`, `inspect/`, `projectcell/`, `provenance/`, `sourcemap/`, `validation/`, `workspace/`, `architecture/`
- **Existing v2 packages (reference for patterns, do not modify):** `internal/v2/stage/`, `internal/v2/loop/`, `internal/v2/event/`, `internal/v2/adapter/`, `internal/v2/pipeline/`
- **Provider abstraction:** `internal/provider/` — supports tiers `low`, `medium`, `high` (extend with `xhigh`)

### Conventions

- **Nil-field normalization:** Use exported `NormalizeNilFields()` for cross-package types; use unexported `normalizeNilFields()` for internal-only types. Both map nil slices/maps to empty values.
- **Tests:** Co-located (`foo_test.go` next to `foo.go`), table-driven, fakes over mocks.
- **TDD:** Write a failing test FIRST, then implement to make it pass, then commit.

---

## Architecture Overview

Seven new packages under `internal/next/`:

| Package | Responsibility |
|---------|---------------|
| `execpolicy/` | Execution policy config: always-run checks, budgets, model tiers |
| `runstore/` | Run record CRUD, artifact layout on disk, events log |
| `planner/` | Agent-driven plan generation, task decomposition, plan validation |
| `executor/` | Agent invocation in worktree, inspection, result extraction |
| `validator/` | Validation command runner (targeted, always-run, final) |
| `evidence/` | Evidence bundle assembly (review.md, metrics.json, etc.) |
| `specloop/` | SpecLoop + TaskLoop + stage pipeline orchestration |

### Dependency flow

```
execpolicy   runstore              (leaf — no internal/next deps)
    \          |
     \    planner  executor  validator  evidence
      \      \        |         |        /
       -------\-------+---------+-------/
                -----  specloop  -----
```

All packages may consume from Spec 0001 packages (`contextpkt`, `projectcell`, `artifact`, etc.) and from `internal/provider/`.

---

## Key Design Decisions

Read the full design spec carefully before starting: `docs/plans/2026-03-11-spec-0002a-core-execution-loop-design.md`

### Stage interface

```go
type Stage interface {
    Name() string
    Run(ctx context.Context, run *runstore.RunState) (NextAction, error)
}

type NextAction struct {
    Kind    ActionKind      // Continue, ReplanFrom, NeedsHuman, Blocked
    Context *FailureContext // populated on ReplanFrom only (blocker summaries for needs_human/blocked live in RunState.BlockerSummary)
}
```

- Non-nil error from `Run()` = always treated as `Blocked`.
- EvidenceStage still runs even after a Blocked result from an earlier stage.
- When any stage returns NeedsHuman or when the SpecLoop sets blocked, EvidenceStage MUST still run to capture collected artifacts. Do not short-circuit past evidence assembly.

### LoadPolicy zero-value warning

When loading execution policy, use `json.Unmarshal` onto a pre-populated defaults struct. Do NOT add post-unmarshal zero-value fallbacks — `MaxTaskRetries=0` is a valid value meaning "no repair attempts."

Budget validation: MaxSpecCycles, MaxRunDurationSeconds, and MaxRunCostUSD must be positive. MaxTaskRetries and MaxRedecompositionPasses can be 0 (valid — means no retries/redecomposition). Do not reject zero for these fields.

### Terminal states

- `ready_for_review` — all final validation passed, evidence bundle complete
- `needs_human` — all tasks failed, or remaining validation failures after max spec cycles exhausted
- `blocked` — unrecoverable infrastructure failure, or budget exhausted (cost/timeout via `budget_exceeded` event)

Note the distinction: `max_spec_cycles` exhausted with remaining validation failures produces `needs_human`. `max_run_cost_usd` or `max_run_duration_seconds` exceeded produces `blocked` with a `budget_exceeded` event. These are different paths with different terminal states.

### TaskLoop (inside ExecuteStage)

```
1. Invoke executor agent → agent writes code (IMPLEMENT)
2. Gromit runs targeted checks + always-run checks (INSPECT — this is Gromit, not the agent)
3. If checks pass → done
4. If checks fail → invoke executor again with failure context + check output (REPAIR)
5. Gromit re-runs checks (RE-INSPECT — again Gromit, not the agent)
6. done / failed / needs_split
```

Gromit determines success from deterministic check signals (exit codes, stdout/stderr), not agent self-report.

- `needs_split` heuristic: 3+ packages with failures OR 2x expected file spread
- Re-decomposition is internal to ExecuteStage (does not consume spec cycle budget)
- Before executing sub-tasks from re-decomposition, revert the original task's changes (`git checkout` touched files)
- Sub-tasks that fail cannot trigger further re-decomposition

### Fix plans, not fresh replans

When ValidateStage returns `ReplanFrom`, the planner receives: original plan, completed tasks with results, specific failure output, and current diff. It produces only new tasks to fix failures. Fix tasks are appended to the task ledger, not replacing it. Fix-plan metadata lives at plan level: `cycle`, `kind` (fix), `parent_cycle`, `failures_addressed`.

### Events

Append-only JSONL (`events.jsonl`). All 15 event types for complete reference:

1. `run_started`
2. `spec_packet_compiled`
3. `plan_created`
4. `plan_validation_result`
5. `task_created`
6. `task_started`
7. `task_validation_result`
8. `task_completed`
9. `task_failed`
10. `task_needs_split`
11. `redecomposition_triggered`
12. `final_validation_result`
13. `replan_triggered`
14. `budget_exceeded`
15. `terminal_state`

### Evidence bundle

Always emitted, even on failure. Includes: `summary.md`, `diff-summary.md`, `task-results.json`, `validation.json`, `review.md` (decision sheet: terminal state, changes, cycle history, validation results, risks, recommended action), `metrics.json` (raw signal with per-invocation records).

### Cost enforcement

Accumulate cost from token counts per invocation. Check runs between tasks and between stages. When exceeded, current stage completes but no further stages/tasks execute. Run transitions to `blocked` with `budget_exceeded` event.

The Agent.Invoke interface must return token counts for metrics and cost enforcement. Suggested signature: `(AgentResult, error)` where `AgentResult` contains `Output`, `TokensIn`, `TokensOut`, `Cost`.

The existing `internal/provider` package has a `Result` type with `Output`, `InputTokens`, `OutputTokens`, `CostUSD`, `Model`, `Duration`. The executor's agent result should wrap or reuse this type rather than creating a parallel hierarchy.

Each agent invocation must use a `context.Context` with deadline derived from `max_task_duration_seconds`.

### Worktree

InitStage creates a git worktree with branch `gromit/spec-<spec-id>-<run-id>`. Worktree is preserved for `ready_for_review` and `needs_human` terminal states; cleaned up for `blocked`. Path recorded in `run.json`.

---

## Existing Patterns to Follow

Study these files for patterns before implementing. They show the v2 conventions for stages, events, loops, and adapters:

| File | What to learn |
|------|--------------|
| `internal/v2/stage/stage.go` | Stage interface pattern, Decision type, StageRequest/StageResult, RetryContext, BeadInfo |
| `internal/v2/loop/spec_loop.go` | Outer loop orchestration, option functions, worktree lifecycle, stage commit pattern |
| `internal/v2/loop/bead_loop.go` | Inner loop orchestration, stage pipeline, retry logic, triage, decomposition |
| `internal/v2/event/event.go` | TypedEvent interface, event struct pattern, event type constants |
| `internal/v2/adapter/adapter.go` | Adapter interfaces (Git, LLM, TaskTracker), AdapterSet aggregation |
| `internal/v2/pipeline/stage_committer.go` | Commit-after-stage pattern |
| `internal/provider/provider.go` | Provider interface, tier constants, model mapping |

---

## Implementation Phases

### Phase 1: Leaf packages (no internal/next deps between them)

- `internal/next/execpolicy/` — Load and validate `policy/execution.json` from project cell (snapshotted as `execution-policy.json` in the run directory — note the different filenames). Policy source: `<project-cell>/policy/execution.json`. Snapshot in run directory: `execution-policy.json`. Different filenames — source uses the project cell directory structure, snapshot is a flat copy. Defaults for all fields. Types for AlwaysRunCheck, Budgets, ModelConfig. Note: default AlwaysRun checks (`go test`, `gofmt`, `go vet`) are Go-specific examples. Real projects should always provide their own `execution.json`.
- `internal/next/runstore/` — Run record CRUD. Create run directory structure under `~/.local/share/gromit/projects/<project-id>/runs/<run-id>/`. Read/write `run.json`, list runs, append events to `events.jsonl`.
- `internal/next/validator/` — Execute shell commands, capture stdout/stderr/exit code, return structured results. Runs targeted checks, always-run checks, and final validation.

### Phase 2: Middle packages (depend on Phase 1)

- `internal/next/planner/` — Invoke LLM agent to generate plan. Parse planner JSON output into typed task list. Validate plan (JSON schema, non-empty, required fields, no duplicate IDs). Fix-plan generation with failure context.
- `internal/next/executor/` — Invoke LLM agent with task packet in worktree. Capture agent output. Inspect worktree after execution (git diff, targeted checks). Extract result.

### Phase 3: Orchestration

- `internal/next/specloop/` — SpecLoop (bounded PDSA cycle) + TaskLoop. RunState shared across stages. Stage pipeline: Init, Compile, Plan, Execute, Validate, Evidence, Finalize.

### Phase 4: Evidence bundle assembly

- `internal/next/evidence/` — Assemble evidence from run artifacts. Generate `review.md`, `metrics.json`, `summary.md`, `diff-summary.md`, `task-results.json`, `validation.json`.

### Phase 5: Individual stage implementations

Implement each stage conforming to the Stage interface:
- `InitStage` — Create run record, create git worktree (branch `gromit/spec-<spec-id>-<run-id>`), snapshot execution policy, copy the approved spec into the run directory as `spec.md`
- `CompileStage` — Build spec packet from project cell using `contextpkt`. Read `cmd/gromit-next/context.go` to understand the existing `contextpkt.Compiler` implementation.
- `PlanStage` — Invoke planner, validate output, produce plan.md + tasks.json
- `ExecuteStage` — Run TaskLoop for each task; partial failure -> Continue; all failed -> NeedsHuman
- `ValidateStage` — Run final validation from exactly two sources: (1) always-run checks from `execution-policy.json`, (2) project cell validation commands from Spec 0001's `validation.json`. Task-level proof checks are NOT part of final validation. Both sources run independently, no deduplication.
- `EvidenceStage` — Assemble evidence bundle
- `FinalizeStage` — Determine terminal state from the run's validation results (not just task statuses). If final validation failed, the state is never `ready_for_review` regardless of task outcomes. Log terminal state, cleanup or preserve worktree.

### Phase 6: CLI commands

- `gromit-next exec spec` — Run a spec against a project (with `--dry-run` support)
- `gromit-next exec show` — Inspect a run
- `gromit-next exec list` — List runs for a project
- `gromit-next spec list` — List specs with derived status (reads `specs_dir` from `project.json` to locate specs in the target repo)

### Phase 7: Provider xhigh tier extension

- Add `TierXHigh = "xhigh"` constant to `internal/provider/provider.go`
- Add xhigh model mappings

---

## Checkpoints

After each phase, verify:

```bash
go test ./internal/next/... -v
go vet ./internal/next/...
gofmt -l internal/next/
```

After Phase 6, also verify:

```bash
go build ./cmd/gromit-next/
go vet ./cmd/gromit-next/...
```

---

## Execution Rules

1. **TDD strictly.** Write a failing test first, then implement to pass it, then commit. No implementation without a covering test.
2. **Commit frequently.** After each green test or small logical unit.
3. **DRY, YAGNI.** No speculative abstractions. Build only what the spec requires.
4. **Follow existing patterns** from `internal/v2/` for stage interfaces, event types, adapter patterns.
5. **Run tests:** `go test ./internal/next/... -v`
6. **Build:** `go build ./cmd/gromit-next/`
7. **Interfaces for dependencies.** Use interfaces for LLM providers, git operations, and filesystem access so tests use fakes.
8. **No global state.** All state flows through RunState or function parameters.
9. **Increment RunState.Cycle** at the start of each spec-level cycle so run.json reflects the actual cycle count.
10. **The SpecLoop must call budget.IncrementCycle()** at the start of each Plan-Execute-Validate cycle. The for-loop counter alone is insufficient — the Budget object tracks cycles internally for CyclesExhausted() checks.
11. **NormalizeNilFields convention.** Per project convention, add `NormalizeNilFields()` to types with slice/map fields: `RunState`, `Task`, `Policy`, `Plan`, `Metrics`. Exported for cross-package types, unexported for internal.

---

## What NOT to Do

- **Don't modify existing `internal/v2/` packages.** The new code lives in `internal/next/`.
- **Don't modify existing Spec 0001 packages** (`internal/next/artifact/`, `projectcell/`, etc.) unless absolutely required for integration. Prefer wrapping.
- **Don't create Gromit state files in target repos.** All run artifacts live in the external workspace (`~/.local/share/gromit/projects/`).
- **Don't add LLM-driven review or acceptance evaluation.** That is Spec 0002b. This spec uses deterministic validation only. The `completed` spec status requires human acceptance (Spec 0002b). In 0002a, the highest status a spec can reach is `ready_for_review`. Implement the status derivation with a placeholder for the acceptance signal.
- **Don't add PR creation or merge automation.**
- **Don't add parallel spec execution.**
- **Don't add resume/recovery** beyond artifact preservation for post-mortem inspection.
- **Don't add indefinite retries.** All retry loops are bounded by budget config.
- **Don't add cross-project learning or doctrine rewriting.** Each run is isolated; no feedback loops that modify project cell doctrine.
- **Don't over-engineer.** Simple implementations that pass tests are better than clever abstractions.

---

## Run Storage Layout

All run artifacts go under the external workspace. Never in the target repo.

```
~/.local/share/gromit/projects/<project-id>/
  runs/
    <run-id>/
      run.json                # Canonical run record (state, timestamps, spec-id, worktree_path)
      spec.md                 # Copy of approved spec
      spec-packet.md          # Compiled spec context
      plan.md                 # Human-readable plan
      tasks.json              # Machine-readable task list (all cycles)
      events.jsonl            # Append-only event log
      execution-policy.json   # Snapshot of policy used
      tasks/
        <task-id>/
          task-packet.md      # Compiled task context
          result.json         # Task outcome + metrics
          agent-output.txt    # Raw agent stdout
      evidence/
        summary.md            # Human-facing run summary
        diff-summary.md       # What changed
        task-results.json     # Aggregated task outcomes
        validation.json       # Final validation results
        review.md             # Decision sheet for human
        metrics.json          # Raw signal: per-invocation records (phase, tier, model, tokens_in, tokens_out, duration, success/failure) + aggregates
```

---

## Success Criteria

All of these must be satisfied before the implementation is complete:

1. **Single-spec execution** through `ready_for_review` with plan, task loops, and evidence bundle
2. **Zero repo pollution** — no Gromit files committed to target repo
3. **Scoped context packets** — task packets exclude clearly unrelated project context
4. **Bounded retries** — task (max_task_retries, default 1), redecomposition (max_redecomposition_passes, default 1), spec cycles (max_spec_cycles, default 3)
5. **Deterministic final gate** — `ready_for_review` impossible if final validation fails
6. **Human review preserved** — final success state is `ready_for_review`, not `accepted`
7. **Artifact durability** — run record, task ledger, validation results, metrics, and review artifact available after any terminal state
8. **Multi-project isolation** — runs for different projects share no state
9. **Useful failure surface** — `needs_human` and `blocked` include blocker summary + recommended action
10. **Fix plans are scoped** — re-plan cycles produce only tasks targeting specific failures, not fresh replans
11. **Metrics preserved** — `metrics.json` with per-invocation token counts, timings, cost estimates, model tiers
12. **Plan validation** — invalid planner output caught before execution, retried once, then `blocked`
13. **Timeout and cost enforcement** — `max_task_duration_seconds`, `max_run_duration_seconds`, `max_run_cost_usd` respected
14. **Worktree isolation** — unique branch names (`gromit/spec-<spec-id>-<run-id>`), concurrent runs don't conflict

---

## Final Verification

After all phases are complete, run the full verification:

```bash
go test ./internal/next/... -v
go test -race ./internal/next/...
go vet ./internal/next/... && go vet ./cmd/gromit-next/...
go mod tidy && git diff --exit-code go.mod go.sum
go build ./cmd/gromit-next/
gofmt -l internal/next/
```

Only push after all checks pass.
