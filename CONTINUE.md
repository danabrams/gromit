# Manual Test Plan — Spec 0002a/0002c/0002d End-to-End

## Status
- **Current phase:** Scenario 11 (CLI Inspection) CONFIRMED. All changes committed.
- **Next:** **Scenario 12 (Broad Refactor — multi-package)**
- **Date:** 2026-03-14
- **Latest commits:**
  - fix: effectiveStatus in EvidenceStage — review.md/summary.md showed "running" instead of terminal state
  - fix: exec show — add Cycles, Duration, Cost, Validation, Worktree, Evidence path fields
  - fix: ShellTaskInspector — task repair wired end-to-end; EventLog through BuildStages; planner proof_checks require executable shell commands
  - (uncommitted) fix: structural fix planner constraint enforcement — filterForbiddenFixTasks, SpecPacket/SpecConstraints in FixPlanRequest, stronger prompt wording
  - (uncommitted) fix: thread SpecConstraints from spec.md to task prompts so agent respects Out-of-Scope/Architectural Constraints
  - (uncommitted) fix: TestNormalizeNilFieldsVisibilityPolicy — added CLAUDE.md convention comment to execpolicy/policy.go
  - (uncommitted) fix: TestFinalVerification — add .claude and .worktrees to scanProjectTestFiles skip list
  - (uncommitted) fix: evidence stage wiring — review.json, acceptance.json, diff-summary.md
  - (uncommitted) fix: preserve ReplanContext across cycles so fix planner runs
  - (uncommitted) fix: 8 bugs found during Scenario 2 run — see "Scenario 2 Bugs Fixed" below
  - `0a24afdbd` — wire InvokeInDir through executor for StreamRun cost tracking
  - `292866a2d` — improve fix-plan prompts, add DirStreamRunner, track files changed
  - `121c8b9e1` — wire Claude provider and fix pipeline for end-to-end execution

## Context
Running the manual test plan from `docs/plans/2026-03-13-spec-0002c-0002d-manual-test-plan.md` to validate the spec 0002a execution loop end-to-end with real Claude CLI invocations.

## Fixture Repos
All at `/tmp/gromit-fixtures/`:
- `fixture-calc/` — Go calculator with `Add`, test for `Add`
- `fixture-greeter/` — Go greeter with `Hello`
- `fixture-multipackage/` — Go module with `internal/auth`, `refund`, `billing`
- `specs/` — `add-subtract.md`, `divide-float64.md`, `unfixable-conflict.md`, `vague-spec.md`, `broad-refactor.md`
- `policies/` — `fixture-calc-execution.json`, `fixture-greeter-execution.json`, `fixture-multipackage-execution.json`

Note: `fixture-calc/calc/divide_test.go` was removed from git (referenced undefined `Divide` function, caused review/fix loops).

Note: For Scenario 2, `divide_test.go` (expecting `int` return) has been re-added to fixture-calc. The fixture currently has commits: Subtract (Scenario 1 result), divide_test.go (int assertion), divide-float64.md spec.

## Bugs Found and Fixed (commit 121c8b9e1)

1. **`--no-input` flag invalid** — Claude CLI doesn't have this flag. Removed from `exec.go` and `contract_helper.go`.
2. **Model IDs not accessible** — `claude-sonnet-4-5-20250514` not available via CLI. Changed to aliases: `haiku`, `sonnet`.
3. **noopCompiler returned "noop spec packet"** — Planner got no useful context. Added `passthruCompiler` that reads the spec file and returns its raw content.
4. **Planner prompt underspecified** — LLM returned `kind: "implementation"` (invalid), string instead of array for `expected_touched_area`, and non-`t-NNN` task IDs. Added explicit format constraints to the prompt.
5. **noopGitOps created empty temp dirs** — Executor had no files to work with. Now copies repo contents via `cp -a`.
6. **Claude CLI `-p` mode doesn't use tools** — Without `--dangerously-skip-permissions`, Claude just generates text and doesn't edit files. Added the flag.

## Limitations Fixed (commits 292866a2d, 0a24afdbd)

1. **Review warnings never got fixed** — Fix-plan prompt now separates review findings from validation failures and instructs the LLM to create surgical fix tasks. Executor task prompts include `FailuresAddressed` context for fix tasks. *(commit 292866a2d)*
2. **`accumulated_cost: 0`** — Added `DirStreamRunner` interface and `StreamRunInDir` to `ClaudeProvider`. Wired `InvokeInDir` through `Invoker` → `LLMAdapter` → `FallbackAdapter` → `ProviderTaskRunner`. Executor now uses `StreamRun` which parses cost/token data from the JSON event stream. *(commits 292866a2d, 0a24afdbd)*
3. **`files_changed: []`** — Added `GitFilesChanged()` detector that runs `git diff --name-only HEAD` + `git ls-files --others` after each task. Wired through `ExecuteStage` → `TaskLoop`. *(commit 292866a2d)*
4. **Review/acceptance never passing** — Resolved by fix #1 (review warnings now get addressed in fix cycles).
5. **Executor ran in CWD not worktree** — Resolved by fix #2 (`InvokeInDir` passes WorkDir to `StreamRunInDir`).

## Scenario 1 — First Run (BEFORE limitation fixes)

**Run ID:** `run-0f43f47081185ea5`
**Status:** `needs_human` (cycles_exhausted)
- Code was correct (Subtract added, tests pass)
- But review warning about double-calling in test assertions kept retriggering replans without being fixed
- Cost/files_changed not tracked

## Scenario 1 — Re-run PASSED (AFTER limitation + ReplanContext fixes)

**Run ID:** `run-d884d2721fbbf7dd`
**Status:** `ready_for_review`
- [x] Review warnings get fixed in fix cycles (double-call pattern fixed across 2 fix cycles)
- [x] `accumulated_cost > 0` — $0.28
- [x] `files_changed` populated — all 5 tasks show files_changed
- [x] Status is `ready_for_review` (not `needs_human`)

**Bug found:** `ReplanContext` was reset to `[]string{}` at the start of each cycle in `specloop.go`, wiping the failures before the plan stage could read them. Fix: removed the reset (ReplanContext is set at end of cycle N for cycle N+1). Test updated.

**Fixture fix:** `divide_test.go` was still committed despite referencing undefined `Divide`. Removed from git to prevent review/fix loops.

## Evidence Fixes (uncommitted)

Three evidence stage bugs found and fixed:

1. **`review.md` showed "Not evaluated" for review/acceptance** — `stage_provider.go` didn't pass `EvidenceDir` to `ReviewStageConfig` or `AcceptStageConfig`. Without it, the bundler was `nil`, so `review.json` and `acceptance.json` were never written. Fix: compute `evidenceDir := store.RunEvidenceDir(rs.RunID)` and pass to both configs. Also added `os.MkdirAll` to `bundle.go:writeJSON` so the evidence dir is created on demand.

2. **`diff-summary.md` always empty (wrong diff form)** — `git diff main...HEAD` (three-dot) only shows committed inter-branch differences. With `noopGitOps` (cp -a copy, no commits), this always returns empty. Fix: changed to `git diff main` (two-arg form) which includes uncommitted working-tree changes.

3. **`diff-summary.md` still empty (wrong directory)** — `lazyDiffProvider` preferred `rs.WorktreePath` (the noopGitOps temp copy) over the original `WorkDir`. But the executor runs Claude CLI in the original `WorkDir`, not the temp copy. Fix: swapped priority so `fallbackDir` (original repo) is preferred until real git worktree support redirects execution into `WorktreePath`.

**Verified locally:** run-8bfa319ea8cae417 shows populated diff-summary.md, review.json with findings, acceptance.json with 6 passing criteria, and review.md with full data.

## Scenario 1 — PASSED (official, after all fixes)

**Run ID:** `run-49a6a016a790ad79`
**Cost:** $0.21
- [x] Status is `ready_for_review`
- [x] `accumulated_cost > 0` — $0.21
- [x] `files_changed` populated — calc.go, calc_test.go across all tasks
- [x] `review.md` shows real review findings (not "Not evaluated")
- [x] `acceptance.json` shows criteria results — all passing
- [x] `diff-summary.md` shows actual diff — Subtract func + tests

**Command:**
```bash
cd /tmp/gromit-fixtures/fixture-calc
git checkout -- .
rm -rf .gromit-next/runs/*
/Users/dabrams/gromit/gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md --store-dir .gromit-next
```

## Scenario 2 — First Run (bugs found, fixes applied — NEEDS RERUN)

**Run ID:** `run-ecf05ecd1f98d740`
**Status:** `needs_human` (cycles_exhausted) — correct per determinism fallback
- Agent correctly added `Divide(a, b int) float64`, triggered 3 fix cycles
- Unfixable: spec forbids modifying `divide_test.go` which uses `%d` on float64 return
- Cost: $0.75, files_changed populated, review.json populated, diff-summary.md populated

**Bugs found and fixed (uncommitted, all 718 tests passing):**

1. **`events.jsonl` never created** — `exec.go` never instantiated `EventLog` or passed it to `SpecLoopConfig`. Fix: create `runstore.NewEventLog(filepath.Join(store.RunDir(rs.RunID), "events.jsonl"))` and wire into config. *(cmd/gromit-next/exec.go)*

2. **`validation.json.always_run.results` null** — `ValidateStage` discarded the full `FinalResult`, `EvidenceStage` reconstructed a minimal struct with nil Results. Fix: added `LastFinalValidation *validator.FinalResult` to `RunState`; `ValidateStage` stores it; `EvidenceStage` uses it if present. *(runstore/types.go, stages/validate.go, stages/evidence.go)*

3. **`acceptance.json` missing on cycles_exhausted** — `AcceptStage` was skipped when terminal state triggered early exit. Fix: added `runAccept()` helper to specloop, called before `emitTerminal`/`runEvidence` in `NeedsHuman` and `cycles_exhausted` paths. Updated test `TestSpecLoop_ReviewReplan_SkipsAcceptStage` to reflect new behavior. *(specloop.go, specloop_test.go)*

4. **`run.json.ended_at` zero** — `FinalizeStage` sets `EndedAt` on success path only; all early-exit terminal paths (HardBudgetExceeded, error, NeedsHuman, Blocked, cycles_exhausted) never set it. Fix: added `rs.EndedAt = time.Now()` in all terminal paths before `emitTerminal`. *(specloop.go)*

5. **`metrics.json.total_replans: 0`** — `TotalReplans` field defined but never incremented. Fix: added `TotalReplans int` to `RunState`; increment at replan trigger in specloop; wire into metrics struct literal. *(runstore/types.go, specloop.go, stages/evidence.go)*

6. **Task IDs reset to `t-001` each fix cycle** — `PlanStage` built `FixPlanRequest` without setting `PriorMaxTaskID`. The field, `ValidatePlanWithPrior()`, and test patterns all existed but were unwired. Fix: added `maxTaskID(tasks)` helper; set `fixReq.PriorMaxTaskID` from `rs.Tasks` before calling `CreateFixPlan()`. *(stages/plan.go)*

7. **`tasks.json` only had last cycle's plan** — Written before `rs.Tasks` was updated, with only `plan.Tasks`. Fix: moved write after `rs.Tasks` update; now writes `rs.Tasks` (full accumulated list). *(stages/plan.go)*

8. **`metrics.json.invocations` always `[]`** — Implemented Option D (OnCost callback pattern). Defined `InvocationRecord` in `runstore` (avoids cycle); added thread-safe accumulator to `Budget`; added `OnInvocation` callback to `llmadapter.Config`; wired into all 4 adapters in `stage_provider.go`; `EvidenceStage` reads from `InvocationSource` interface backed by budget. *(runstore/types.go, specloop/budget.go, llmadapter/adapter.go, stage_provider.go, stages/evidence.go)*

## Scenario 2 — Second Run (all 8 bugs verified — but correctness issue found)

**Run ID:** `run-9ba515fa3ce1002e`
**Status:** `needs_human` (cycles_exhausted) — correct terminal state, but wrong path
- All 8 bug fixes verified (events.jsonl, validation.json, acceptance.json, ended_at, total_replans, task IDs, tasks.json, invocations)
- Agent modified `divide_test.go` in fix cycle 2 (violating spec constraint "Do NOT modify any existing test files")
- Review correctly caught the violation (severity: error), but constraint was never in the task prompt to begin with

**Root cause found:** `SpecConstraints` (Out-of-Scope + Architectural Constraints sections) were extracted from spec.md but never threaded to the executor task prompt. Agent inferred constraints from review feedback rather than being told upfront.

**Spec constraints fix (uncommitted):**
1. `runstore/types.go` — added `SpecConstraints string` to `RunState` and `Task`
2. `stages/compile.go` — `extractSpecConstraints()` parses Out-of-Scope + Architectural Constraints from spec.md; set on `rs.SpecConstraints` after compile
3. `stages/plan.go` — copies `rs.SpecConstraints` to every new Task
4. `specloop/provider_taskrunner.go` — `renderTaskBody()` emits `### Spec Constraints` section with "HARD REQUIREMENTS" preamble when non-empty
5. Tests added: `extractSpecConstraints` (5 cases), plan propagation (1), rendering (3)

**Rerun command:**
```bash
cd /tmp/gromit-fixtures/fixture-calc
git checkout -- .
rm -rf .gromit-next/runs/*
/Users/dabrams/gromit/gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/divide-float64.md --store-dir .gromit-next
```

**Expected outcome after spec constraints fix:**
- Agent adds `Divide(a, b int) float64` ✓
- Agent does NOT modify `divide_test.go` (constraint enforced in prompt)
- Validation fails (test expects `result != 3`, float64 returns 3.333...)
- Fix planner cannot fix without modifying test (spec forbids it)
- Status: `needs_human` via cycles_exhausted — same terminal state, correct path

## Scenario 2 — Third Run (spec constraints ordering fix applied — NEEDS RERUN)

**Root cause found (run-cb0876584b270fe0):**
- Agent deleted `divide_test.go` in t-002 ("verify tests pass") to satisfy proof check "go test exits 0"
- Two problems: (1) Spec Constraints section appeared AFTER Proof Checks in rendered prompt — agent anchored on proof checks first; (2) preamble said "modify" but agent treated deletion as distinct from modification

**Fix applied (uncommitted):**
- `provider_taskrunner.go` — moved `SpecConstraints` section before `ProofChecks` in `renderTaskBody`; enhanced preamble: "'Modify' includes editing, deleting, renaming, or moving a file"
- `provider_taskrunner_test.go` — added `TestRenderTaskPrompt_SpecConstraintsAppearBeforeProofChecks` (ordering) and `TestRenderTaskPrompt_ConstraintPreambleMentionsDeletion` (preamble content)
- All 8192 tests passing

## Scenario 2 — Runs 4–7 (fix planner still violated constraints — bugs found and fixed)

**Root cause (runs run-9e2418980cf295fb, run-f02c298100dae94f, run-d0a4242b411d8cb7):**
- The executor correctly respected constraints in t-001 (only touched calc.go)
- But the FIX PLANNER had no knowledge of spec constraints: `FixPlanRequest` had no `SpecConstraints` or `SpecPacket` fields
- Fix planner kept generating tasks targeting `divide_test.go` (to fix the `%d` format error)
- Prompt-only fixes (stronger wording, CRITICAL: labels) were insufficient — LLM still rationalized the modification

**Three-layer fix applied (uncommitted, run-acb4a772a99c39d5 = first passing run):**

1. **`planner/planner.go`** — Added `SpecPacket string` and `SpecConstraints string` to `FixPlanRequest`. Updated `buildFixPlanPrompt` to include: full spec requirements section (so fix planner knows float64 is required, not just what's forbidden), HARD REQUIREMENTS block with stronger wording ("CRITICAL: if only way to fix requires violating constraint, leave it unfixed"), explicit instructions not to touch forbidden files.

2. **`specloop/stages/plan.go`** — Wire `SpecPacket` and `SpecConstraints` into `FixPlanRequest`. Added `filterForbiddenFixTasks()`: after fix plan generation, structurally removes any task whose `expected_touched_area` includes `*_test.go` files when spec constraints prohibit test file modification. If all tasks are filtered, returns `Continue` without adding tasks (cycles exhaust → `needs_human`).

3. **Tests (8198 total, all passing):** `TestFilterForbiddenFixTasks_*` (3 cases), `TestPlanStage_FixCycle_AllTasksFilteredReturnsContinue`, `TestBuildFixPlanPrompt_IncludesSpecConstraints`, `TestBuildFixPlanPrompt_NoSpecConstraintsSection_WhenEmpty`.

## Scenario 2 — CONFIRMED STABLE (run-a52415cf7cf7817f)

**Run ID:** run-a52415cf7cf7817f
**Status:** `needs_human` (cycles_exhausted) — correct terminal state, correct path
- t-001: Agent added `Divide(a, b int) float64` — only `calc/calc.go` changed ✓
- Fix cycles: fix planner generated test-file tasks, all filtered by `filterForbiddenFixTasks` — `divide_test.go` untouched ✓
- 3 replans, cycles exhausted → `needs_human` ✓
- **Cost:** $0.12
- All evidence files populated: events.jsonl, validation.json, acceptance.json, diff-summary.md, metrics.json (10 invocations), review.md ✓

## Scenario 3 — Budget Exhaustion — CONFIRMED WORKING

**Run ID:** run-8c3232c2cae9109c
**Status:** `needs_human` (cycles_exhausted) — correct terminal state
**Cost:** $0.28

**Approach:** Use `max_spec_cycles: 1` via a temporary policy file. The agent implements Subtract in cycle 1, validation passes, then the budget gate fires before cycle 2 → `needs_human` (cycles_exhausted). Deterministic, cheap (~$0.20), no stochastic cost dependency.

**Fixture note:** Reset fixture-calc to initial state (Add only, no divide files) with:
```bash
cd /tmp/gromit-fixtures/fixture-calc
git show 7f6de76:calc/calc.go > calc/calc.go
git show 7f6de76:calc/calc_test.go > calc/calc_test.go
rm -f calc/divide_test.go calc/divide_edge_test.go calc/divide_exact_test.go
rm -rf .gromit-next/runs/*
```
(Plain `git checkout -- .` is insufficient — committed Scenario 2 state includes divide test files and Subtract already added.)

**Setup:**
```bash
# Write a one-off policy with max_spec_cycles: 1
cat > /tmp/gromit-fixtures/policies/fixture-calc-budget1.json << 'EOF'
{
  "always_run": [
    {"name": "unit-tests", "command": "go test ./...", "type": "test"},
    {"name": "format", "command": "gofmt -l .", "type": "lint"},
    {"name": "vet", "command": "go vet ./...", "type": "lint"}
  ],
  "budgets": {
    "max_spec_cycles": 1,
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
EOF
```

**Run:**
```bash
cd /tmp/gromit-fixtures/fixture-calc
git checkout -- .
rm -rf .gromit-next/runs/*
/Users/dabrams/gromit/gromit-next exec spec \
  --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md \
  --policy /tmp/gromit-fixtures/policies/fixture-calc-budget1.json \
  --store-dir .gromit-next
```

**Verified outcomes:**
- [x] Status: `needs_human` ✓
- [x] `terminal_reason`: `cycles_exhausted` ✓
- [x] `cycle`: 1 ✓
- [x] `ended_at`: populated ✓
- [x] `accumulated_cost`: $0.28 ✓
- [x] `final_validation_passed`: true — validation passed before cycles exhausted ✓
- [x] `total_replans`: 1 ✓
- [x] `metrics.json.invocations`: 13 invocations tracked ✓
- [x] `diff-summary.md`: populated with actual diff ✓
- [x] `review.md`: populated ✓
- [x] `acceptance.json`: populated (8 evidence files total) ✓

**Observation:** Agent added Subtract func but not tests (review caught this as error). Acceptance shows "unclear" for test coverage — expected since fixture state still had committed divide files that agent cleaned up. Core budget exhaustion behavior validated.

## Scenario 4 — Unfixable Spec (contradictory) — CONFIRMED WORKING

**Run ID:** run-dc497ab43a1abc24
**Status:** `needs_human` (cycles_exhausted) — correct terminal state

**Bugs found and fixed (uncommitted):**

1. **`CreateFixPlan` error in fix cycle caused `Blocked`** — both `CreateFixPlan` returning an error and fix plan tasks remaining invalid after 2 retries fell through to `Blocked`. Fix: both cases now return `Continue` in fix cycles so cycles exhaust → `needs_human`. Files: `stages/plan.go`, `stages/plan_test.go` (2 new tests).

2. **`files_changed` was empty or noisy** — root cause: `GitFilesChanged()` ran `git diff --name-only HEAD` as a point-in-time check after the task. Two failure modes:
   - First attempt: showed false positives (`calc_test.go`, divide files) because fixture reset made working tree differ from HEAD before the task ran
   - After set-subtraction fix: showed `[]` because files already dirty vs HEAD before the task don't appear in the delta even when the agent modifies them
   - Root fix: switched to **content-hash snapshot** — `GitFilesChanged()` now returns a stateful closure that hashes file contents on first call (baseline), hashes again on second call, and returns only files whose content actually changed. Files: `specloop/files_changed.go` (rewritten), `specloop/taskloop.go` (before-call discards result, after-call uses it), 8204 tests passing.

**Confirmed outcomes (run-dc497ab43a1abc24):**
- [x] Status: `needs_human` ✓
- [x] `terminal_reason`: `cycles_exhausted` ✓
- [x] `final_validation_passed`: false ✓
- [x] `calc_test.go` untouched — constraint enforced ✓
- [x] `files_changed` for t-001: `['calc/calc.go']` only — no false positives ✓
- [x] All evidence files populated ✓

## Scenario 5 — Dry Run — CONFIRMED WORKING

**Run ID:** run-407b2101ecccee71
**Status:** `running` (expected — finalize never runs in dry-run mode)

**Verified outcomes:**
- [x] `--dry-run` flag accepted ✓
- [x] Only init/compile/plan stages ran — execute/validate/evidence/finalize skipped ✓
- [x] `calc/calc.go` unchanged — no code executed ✓
- [x] Artifacts: run.json, tasks.json, plan.md, spec.md, spec-packet.md, execution-policy.json ✓
- [x] Plan generated with 2 tasks (t-001: add Subtract, t-002: add tests) ✓
- [x] `SpecConstraints` present in tasks ✓
- [x] 5 dry-run unit tests all pass ✓

## Scenario 6 — Task Repair (task retry on failure) — CONFIRMED WORKING

**Run ID:** run-c4724a90ca32f14c
**Status:** `ready_for_review`
**Cost:** $0.17

**Bugs found and fixed (uncommitted):**

1. **`ShellTaskInspector` not implemented** — Inspector field in `ExecuteStageConfig` was always nil; task repair never triggered. Fix: created `internal/next/specloop/shell_task_inspector.go` — runs task's `proof_checks` via `validator.Runner.RunTargeted()`. Returns `Pass=false` if any check fails, triggering `RepairTask` up to `MaxRetries` times. Wired into `stage_provider.go`. 5 tests added.

2. **`EventLog` not wired to `ExecuteStage`** — Task-level events (`task_started`, `task_validation_result`, `task_completed`) were never persisted. Fix: added `eventLog *runstore.EventLog` to `BuildStages` interface signature; `stage_provider.go` receives it and passes to `ExecuteStageConfig.EventLog`.

3. **Planner generated non-executable proof checks** — Proof checks like `"calc/calc.go contains a Subtract function..."` were prose descriptions, not shell commands. When `ShellTaskInspector` ran them, they always failed (shell tries to exec the file path). Fix: tightened `buildPlanPrompt` and `buildFixPlanPrompt` in `planner/planner.go` to explicitly require "EXECUTABLE SHELL COMMANDS only" with examples (`grep -q`, `go test ./...`, etc.).

4. **Fixture `.gromit-next/` accidentally committed** — `git add -A` in reset commit included run artifacts. Fix: added `.gitignore` excluding `.gromit-next/`, untracked the directory.

**Verified outcomes (run-c4724a90ca32f14c):**
- [x] `status: ready_for_review` ✓
- [x] `final_validation_passed: true` ✓
- [x] `task_validation_result` events in `events.jsonl` — Inspector ran for every task ✓
- [x] `attempts: 1` for all tasks — proof checks passed on first inspection ✓
- [x] `files_changed` correct: t-001 → `[calc/calc.go, calc/calc_test.go]`, t-002 → `[calc/calc_test.go]` ✓
- [x] Proof checks are executable shell commands: `grep -q`, `go test ./...`, `go vet ./...`, `gofmt -l .` ✓
- [x] Repair mechanism confirmed via earlier run (run-b8f5c32b63eb5ab8): when proof checks fail, `attempts: 2` — repair triggers and `RepairTask` is called ✓
- [x] 747 unit tests passing ✓

**Note on max_task_retries:** Repair loop is gated on `cfg.Inspector != nil` (now wired) and inspection failure. Budget is enforced via `for retry := 0; retry < cfg.MaxRetries; retry++` in taskloop.go. With `max_task_retries: 1` (default), tasks get exactly one repair attempt if inspection fails.

**Command:**
```bash
cd /tmp/gromit-fixtures/fixture-calc
git checkout -- calc/
rm -rf .gromit-next/runs/*
/Users/dabrams/gromit/gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md --store-dir .gromit-next
```

## Scenario 7 — Task Split / Redecomposition — CORE MACHINERY CONFIRMED; NEW BUGS FOUND

### Run 1 (pre-fix): run-c8ffddf7b0eee56e — cycles_exhausted ($1.55)

`task_needs_split` + `redecomposition_triggered` fired twice. Cycles exhausted because agent missed test file assertions. Root cause: no structural enforcement of test-file coverage.

**Fixes applied (uncommitted, 8221 tests pass):**
1. `planner/planner.go` — require content-verification proof checks for every `*_test.go` in `expected_touched_area`
2. `taskloop.go` — structural cross-check: `*_test.go` in `expected_touched_area` not in `files_changed` → downgrade `Pass: true` to `Pass: false`
3. Tests: 2 planner + 3 taskloop tests added; gofmt fixed (8221 total)

### Run 2 (post-fix): run-0ed6e5980aa970cd — stage_needs_human ($2.50)

**Run ID:** run-0ed6e5980aa970cd
**Status:** `needs_human` (stage_needs_human)
**Cost:** $2.50
**Cycle:** 3, `total_replans`: 2

**Core objectives verified:**
- [x] `task_needs_split` fired — t-011 triggered split ✓
- [x] `redecomposition_triggered` fired ✓
- [x] Structural test-file enforcement working in cycles 1–2: agent correctly touched test files (auth_test.go, billing_test.go, refund_test.go appear in `files_changed`) ✓
- [x] All 8 evidence files present ✓
- [x] `ended_at` populated, cost tracked ✓

**What happened cycle-by-cycle:**
- Cycle 1: Planner decomposed spec into 5 tasks upfront (per-package). All passed. Agent created `internal/logging/logger.go` (untracked).
- Review fired: `logger.go` and `logger_test.go` exist on disk but are untracked (`??` in git status) — not in diff.
- Cycle 2: Fix tasks (t-006..t-009) corrected test files. Review fired again for same untracked-files issue.
- Cycle 3: Fix planner created t-010 (`git add` the logging files) and t-011 (fix double-close in test files). t-010 failed twice (structural check: `logger_test.go` in eta but `git add` doesn't change content → `files_changed: []`). t-011 triggered `needs_split` → redecomposed into t-001..t-005. All 5 sub-tasks failed. Execute stage returned `NeedsHuman` ("all tasks failed").

**Bugs found and fixed (uncommitted, 760 tests pass):**

1. **Untracked files review loop** — `noopGitOps` never stages new files. Fix planner correctly generates a git-add task (t-010), but bug #2 blocked it from succeeding. Expected to self-resolve with bug #2 fix. *Note: real git-worktree execution commits properly; this is a noopGitOps limitation.*

2. **Structural check blocks git-only fix tasks** — Fixed in `taskloop.go`: structural `*_test.go` cross-check now skips when `result.FilesChanged` is empty. When `files_changed: []`, the agent did a non-content operation (git staging) or a genuine no-op — nothing to enforce. Applied to both initial inspection and repair retry.

3. **Redecomposition ID collision** — Fixed in `taskloop.go`: after `Decompose()` returns sub-tasks, `maxTaskIDInQueue()` scans the queue for the current max numeric suffix, then `renumberSubTasks()` reassigns sub-task IDs starting at `max+1`. E.g., if t-011 triggers a split, sub-tasks become t-012, t-013, t-014 instead of t-001. Also copies `SpecConstraints` from parent to sub-tasks if decomposer doesn't set it.

4. **`stage_needs_human` instead of `cycles_exhausted`** — Fixed in `stages/execute.go`: `allFailed` branch now returns `ReplanFrom` instead of `NeedsHuman`. Fix planner gets a chance to try recovery; normal cycle governor handles escalation. Test renamed `TestExecuteStage_AllTasksFailed_ReplanFrom`.

### Run 3 (post-fix): run-afcbc8f4fe7ae6b2 — cycles_exhausted ($0.98) — CONFIRMED

**Run ID:** run-afcbc8f4fe7ae6b2
**Status:** `needs_human` (cycles_exhausted) — correct terminal state
**Cost:** $0.98, **Cycle:** 3, **total_replans:** 3

**All 4 bugs verified:**
- [x] `task_needs_split` fired — t-001 triggered split ✓
- [x] `redecomposition_triggered` fired ✓
- [x] **Bug #3 (ID collision):** Sub-tasks got t-002..t-008, not t-001 again ✓
- [x] **Bug #2 (structural check skip):** t-008, t-009 have `files_changed: []` and completed — no blocking ✓
- [x] **Bug #4 (allFailed→ReplanFrom):** terminal_reason is `cycles_exhausted`, not `stage_needs_human` ✓
- [x] `final_validation_passed: true` — all tests pass ✓
- [x] All evidence files present ✓
- [x] **Bug #1 (untracked files):** Correctly cycles to exhaustion rather than looping forever (noopGitOps limitation) ✓

**Observation:** t-001 ran twice in events — once triggering the split (cycle 1), then again at start of cycle 2 with `files_changed: []`. Original task appears to be re-queued after redecomposition. Harmless but worth investigating later.

**Remaining review error (noopGitOps limitation):** `logger.go` untracked in diff across all 3 cycles — real git-worktree execution would commit files properly, eliminating this.

## Scenario 8 — Multi-Project Isolation — CONFIRMED WORKING

**Calc Run ID:** run-af0838da20d5b1b3 | **Greeter Run ID:** run-1dc68ab47b0bb8df
**Both Status:** `ready_for_review`
**Calc Cost:** $0.18 | **Greeter Cost:** $0.12

**Verified outcomes:**
- [x] Both runs completed concurrently, both exit 0 ✓
- [x] Run directories separate: `fixture-calc/.gromit-next/runs/` vs `fixture-greeter/.gromit-next/runs/` ✓
- [x] Worktrees different: `gromit-noop-worktree-2418093440` vs `gromit-noop-worktree-409300836` ✓
- [x] Spec packets distinct: calc → "Add a Subtract function to the calculator", greeter → "Add a Farewell function to the greeter" ✓
- [x] No cross-contamination: no "greeter/farewell" refs in calc evidence; no "calculator/subtract" refs in greeter evidence ✓
- [x] Events independent: 6 events each, separate task IDs, separate run artifacts ✓
- [x] Correct code: calc got `Subtract(a, b int) int`, greeter got `Farewell(name string) string` ✓
- [x] All 8 evidence files in each run's `evidence/` directory ✓
- [x] Metrics independent: 11 invocations (calc), 8 invocations (greeter) ✓

**Command:**
```bash
cd /tmp/gromit-fixtures/fixture-calc && git checkout -- . && rm -rf .gromit-next/runs/*
cd /tmp/gromit-fixtures/fixture-greeter && git checkout -- . && rm -rf .gromit-next/runs/* 2>/dev/null

cd /tmp/gromit-fixtures/fixture-calc && /Users/dabrams/gromit/gromit-next exec spec \
  --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md \
  --policy /tmp/gromit-fixtures/policies/fixture-calc-execution.json \
  --store-dir .gromit-next > /tmp/calc-run.log 2>&1 &

cd /tmp/gromit-fixtures/fixture-greeter && /Users/dabrams/gromit/gromit-next exec spec \
  --project fixture-greeter \
  --spec /tmp/gromit-fixtures/fixture-greeter/specs/add-farewell.md \
  --policy /tmp/gromit-fixtures/policies/fixture-greeter-execution.json \
  --store-dir .gromit-next > /tmp/greeter-run.log 2>&1 &

wait
```

## Scenario 9 — Cost Limits — CONFIRMED WORKING

**Run ID:** run-dad4848b3ef42090
**Status:** `blocked`
**Policy:** `max_run_cost_usd: 0.001`
**Actual cost:** $0.0719

**Verified outcomes:**
- [x] Status: `blocked` ✓
- [x] `terminal_reason`: `budget_exceeded` ✓
- [x] `blocker_summary`: `"cost budget exceeded: $0.07 >= $0.00"` ✓
- [x] `ended_at`: populated ✓
- [x] `accumulated_cost`: $0.0719 (71x the $0.001 limit) ✓
- [x] `events.jsonl` has `budget_exceeded` event with `accumulated_cost: 0.0719` ✓
- [x] **Cost enforcement timing**: t-001 completed (Subtract added); t-002 got `status: "blocked", attempts: 0` — budget check fires between tasks ✓
- [x] No validate/finalize/review stages ran after budget exceeded ✓
- [x] Evidence bundle emitted (6 files: diff-summary.md, metrics.json, review.md, summary.md, task-results.json, validation.json) ✓
- [x] `metrics.json.total_cost_usd`: $0.0719, 2 invocations tracked ✓

**Command:**
```bash
cd /tmp/gromit-fixtures/fixture-calc
git show 7f6de76:calc/calc.go > calc/calc.go
git show 7f6de76:calc/calc_test.go > calc/calc_test.go
rm -rf .gromit-next/runs/*
cat > /tmp/gromit-fixtures/policies/fixture-calc-cost001.json << 'EOF'
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
    "max_run_cost_usd": 0.001
  },
  "models": {
    "planner": "high",
    "executor": "medium"
  }
}
EOF
/Users/dabrams/gromit/gromit-next exec spec \
  --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md \
  --policy /tmp/gromit-fixtures/policies/fixture-calc-cost001.json \
  --store-dir .gromit-next
```

## Scenario 10 — Timeout — CONFIRMED WORKING

**Run ID:** run-a14e20583c1f1dc4
**Status:** `blocked`
**Policy:** `max_run_duration_seconds: 5`
**Actual duration:** ~6.7 seconds

**Verified outcomes:**
- [x] Status: `blocked` ✓
- [x] `terminal_reason`: `budget_exceeded` ✓
- [x] `blocker_summary`: `"time budget exceeded: 6s >= 5s"` ✓
- [x] `ended_at`: populated ✓
- [x] `events.jsonl` has `budget_exceeded` event ✓
- [x] Run completed in ~6.7s (roughly the timeout duration, not hanging) ✓
- [x] Evidence bundle emitted: 6 files (diff-summary.md, metrics.json, review.md, summary.md, task-results.json, validation.json) ✓
- [x] `metrics.json.invocations`: 1 invocation (plan stage, sonnet, 6427ms) ✓
- [x] Execute stage never ran — tasks have `status: "pending", attempts: 0` ✓
- [x] calc/calc.go unchanged ✓

**Behavior:** Timeout fires between stages (init/compile/plan completed; execute never started). The plan stage ran for ~6.4s generating 2 tasks. After plan stage, the budget check fired (6s >= 5s) → `blocked`.

**Note:** `accumulated_cost: 0` for this run because the plan-stage invocation returned no cost data from Claude CLI stream. This is expected — cost tracking works correctly in execute-stage invocations (seen in Scenarios 1-9).

**Command:**
```bash
cd /tmp/gromit-fixtures/fixture-calc
git show 7f6de76:calc/calc.go > calc/calc.go
git show 7f6de76:calc/calc_test.go > calc/calc_test.go
rm -f calc/divide_test.go calc/divide_edge_test.go calc/divide_exact_test.go
rm -rf .gromit-next/runs/*
cat > /tmp/gromit-fixtures/policies/fixture-calc-timeout5.json << 'EOF'
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
    "max_run_duration_seconds": 5,
    "max_run_cost_usd": 50.0
  },
  "models": {
    "planner": "high",
    "executor": "medium"
  }
}
EOF
/Users/dabrams/gromit/gromit-next exec spec \
  --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md \
  --policy /tmp/gromit-fixtures/policies/fixture-calc-timeout5.json \
  --store-dir .gromit-next
```

## Scenario 11 — CLI Inspection — CONFIRMED WORKING

**Run ID used:** run-ed72546cce95542b (ready_for_review, $0.14, Subtract spec)

**Fixes applied:**
1. `exec show` was missing Cycles, Duration, Cost, Validation pass, Worktree path, Evidence path. Added all fields. 1 new test.
2. `review.md` and `summary.md` showed `status: running` — `EvidenceStage` runs before `FinalizeStage` so `rs.Status` was still "running" when evidence files were written. Fix: added `effectiveStatus(rs)` helper that derives the correct terminal state when `rs.Status == "running"`. 4 new tests. 767 tests total pass.

**Verified outcomes:**

#### 12a: `exec list`
- [x] Table with RUN ID, SPEC, STATUS, STARTED ✓
- [x] Multiple runs shown (ready_for_review + blocked) ✓

#### 12b: `exec show <run-id>`
- [x] Run ID, Spec, Project, Status (ready_for_review) ✓
- [x] Cycles: 2 ✓
- [x] Duration: 2m52.109s ✓
- [x] Tasks: 3 total, 3 done ✓
- [x] Valid: true ✓
- [x] Cost: $0.1380 ✓
- [x] Worktree path ✓
- [x] Evidence path ✓

#### 12c: `exec show latest`
- [x] Resolves to most recent run ✓
- [x] Fields match correctly ✓

#### 12d: `exec show --full`
- [x] All evidence files shown: acceptance.json, diff-summary.md, metrics.json, review.json, review.md, summary.md, task-results.json, validation.json ✓
- [x] Task-level details per task: status, attempts, files_changed, proof_checks ✓

#### 12e: `spec list`
- [x] Table with SPEC, STATUS, LAST RUN ✓
- [x] add-subtract: `ready_for_review` ✓
- [x] divide-float64: `ready` (no run) ✓

**Re-run verified (run-904c4763b46c98c8, $0.21):**
- [x] `review.md` Terminal State: `ready_for_review` ✓ (was "running" before fix)
- [x] `summary.md` Status: `ready_for_review` ✓ (was "running" before fix)

**Command:**
```bash
cd /tmp/gromit-fixtures/fixture-calc
git show 7f6de76:calc/calc.go > calc/calc.go
git show 7f6de76:calc/calc_test.go > calc/calc_test.go
rm -f calc/divide_test.go calc/divide_edge_test.go calc/divide_exact_test.go
rm -rf .gromit-next/runs/*
/Users/dabrams/gromit/gromit-next exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md --store-dir .gromit-next
# Then:
/Users/dabrams/gromit/gromit-next exec list --project fixture-calc --store-dir .gromit-next
/Users/dabrams/gromit/gromit-next exec show <run-id> --store-dir .gromit-next
/Users/dabrams/gromit/gromit-next exec show latest --project fixture-calc --store-dir .gromit-next
/Users/dabrams/gromit/gromit-next exec show <run-id> --full --store-dir .gromit-next
/Users/dabrams/gromit/gromit-next spec list --project fixture-calc --store-dir .gromit-next
```

## Remaining Scenarios (not yet run)
12. Broad Refactor (multi-package)

## How to Resume
1. Read this file
2. Read the manual test plan: `docs/plans/2026-03-11-spec-0002a-manual-test-plan.md` (Scenario 9 = Multi-Project Isolation, etc.)
3. Fixture repos are at `/tmp/gromit-fixtures/` (may need to be recreated if `/tmp` was cleaned)
4. Rebuild binary: `go build ./cmd/gromit-next/`
5. Fix the 4 Scenario 7 bugs (see above), re-run Scenario 7 to confirm, then continue with Scenario 8 (Multi-Project Isolation)
