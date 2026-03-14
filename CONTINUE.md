# Manual Test Plan — Spec 0002a/0002c/0002d End-to-End

## Status
- **Current phase:** Scenario 5 CONFIRMED WORKING — run-407b2101ecccee71 passed. Ready for Scenario 6.
- **Next:** Scenario 6 (Task Repair — task retry on failure)
- **Date:** 2026-03-14
- **Latest commits:**
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

## Remaining Scenarios (not yet run)
6. Task Repair — task retry on failure
7. Task Split / Redecomposition
8. Multi-Project Isolation
9. Cost Limits
10. Timeout
11. CLI Inspection (`exec inspect`)
12. Broad Refactor (multi-package)

## How to Resume
1. Read this file
2. Read the manual test plan: `docs/plans/2026-03-13-spec-0002c-0002d-manual-test-plan.md`
3. Fixture repos are at `/tmp/gromit-fixtures/` (may need to be recreated if `/tmp` was cleaned)
4. Rebuild binary: `go build ./cmd/gromit-next/`
5. Continue with Scenario 5 (Dry Run)
