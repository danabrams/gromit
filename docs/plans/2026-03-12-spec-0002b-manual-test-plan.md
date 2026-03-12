# Spec 0002b Manual Test Plan

**Spec**: LLM Review, Acceptance Evaluation, and Fix-Cycle Replanning
**Date**: 2026-03-12
**Scope**: Human QA walkthrough of all Spec 0002b acceptance criteria
**Prerequisite**: Spec 0002a fully implemented and passing (all 0002a manual test scenarios green)

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

Confirm output shows `exec` and `spec` subcommands. The `exec spec` command should accept `--project`, `--spec`, `--policy`, `--dry-run`, and `--store-dir` flags.

### 1.3 Verify 0002a Bug Fixes (Blocking Prerequisites)

Before proceeding with 0002b scenarios, verify the three bugs discovered during 0002a manual testing are fixed. If any fail, stop -- 0002b scenarios depend on these.

See Section 6 for the full bug fix verification steps.

### 1.4 Reuse Fixture Repos from 0002a

This plan reuses the three fixture repos from the 0002a manual test plan:
- `fixture-calc` -- tiny Go calculator package
- `fixture-greeter` -- tiny Go greeting package
- `fixture-multipackage` -- multi-package Go module with `internal/auth/`, `internal/refund/`, `internal/billing/`

If they do not already exist, set them up per Section 4 of the 0002a manual test plan.

Verify they are attached:

```bash
ls ~/.local/share/gromit/projects/fixture-calc/
ls ~/.local/share/gromit/projects/fixture-greeter/
ls ~/.local/share/gromit/projects/fixture-multipackage/
```

### 1.5 Install 0002b Execution Policies

The 0002b execution policies extend the 0002a policies with `review` configuration and the `evaluator` model tier. Replace the existing policies:

```bash
# fixture-calc
cat > ~/.local/share/gromit/projects/fixture-calc/policy/execution.json << 'POLICY'
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
    "executor": "medium",
    "evaluator": "high"
  },
  "review": {
    "facets": ["spec_alignment", "code_quality"],
    "tiers": {
      "spec_alignment": "high",
      "code_quality": "medium"
    },
    "replan_threshold": "warning"
  }
}
POLICY

# fixture-greeter
cat > ~/.local/share/gromit/projects/fixture-greeter/policy/execution.json << 'POLICY'
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
    "executor": "medium",
    "evaluator": "high"
  },
  "review": {
    "facets": ["spec_alignment", "code_quality"],
    "tiers": {
      "spec_alignment": "high",
      "code_quality": "medium"
    },
    "replan_threshold": "warning"
  }
}
POLICY

# fixture-multipackage
cat > ~/.local/share/gromit/projects/fixture-multipackage/policy/execution.json << 'POLICY'
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
    "executor": "medium",
    "evaluator": "high"
  },
  "review": {
    "facets": ["spec_alignment", "code_quality"],
    "tiers": {
      "spec_alignment": "high",
      "code_quality": "medium"
    },
    "replan_threshold": "warning"
  }
}
POLICY
```

### 1.6 Ensure Context is Enriched

If not already done from 0002a testing:

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
- `PROJ_DIR` = `~/.local/share/gromit/projects/<project>/`
- Cleanup: each scenario notes whether to clean up or leave artifacts.

---

### Scenario 1: Review + Acceptance Happy Path -- `ready_for_review`

**Purpose**: Verify that a straightforward spec passes review (empty findings), passes all acceptance criteria, and reaches `ready_for_review` with the complete 0002b evidence bundle.

**Setup**:
```bash
# Ensure fixture-calc is attached, enriched, and has the add-subtract spec committed
# (should already exist from 0002a testing)
ls /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
# If missing, copy it:
# cp /tmp/gromit-fixtures/specs/add-subtract.md /tmp/gromit-fixtures/fixture-calc/specs/
# cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add subtract spec"
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Expected CLI output** (approximate):
```
[init] Created run 20260312-XXXXXX-XXXXXX
[compile] Spec packet compiled
[plan] Planner produced N tasks
[execute] All N tasks completed
[validate] Final validation: N/N passed
[review] spec_alignment: 0 findings
[review] code_quality: 0 findings (or info-only)
[accept] N/N acceptance criteria: pass
[evidence] Bundle written

Terminal state: ready_for_review
```

**Verify**:

1. CLI output shows the new stages: `[review]` and `[accept]` appear between `[validate]` and `[evidence]`.
2. Terminal state printed: `ready_for_review`.
3. Run directory exists:
   ```bash
   RUN_ID=<from output>
   ls ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/
   ```
4. `run.json` shows correct state:
   ```bash
   jq .state ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/run.json
   # Expected: "ready_for_review"
   ```
5. **FinalizeStage three-gate condition verified** -- all three gates must be true for `ready_for_review`:
   ```bash
   jq '{final_validation_passed, final_review_passed, final_acceptance_passed}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/run.json
   # Expected: {"final_validation_passed": true, "final_review_passed": true, "final_acceptance_passed": true}
   ```
   If any gate is false, the terminal state should NOT be `ready_for_review`.

6. **review.json exists and is clean** (0002b artifact):
   ```bash
   cat ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json | jq .
   ```
   Expected structure -- object keyed by facet name, each containing a findings array:
   ```json
   {
     "spec_alignment": [],
     "code_quality": []
   }
   ```
   Or with info-level findings only (info never blocks):
   ```json
   {
     "spec_alignment": [],
     "code_quality": [
       {
         "severity": "info",
         "file": "calc/calc.go",
         "line": 10,
         "description": "consider extracting helper",
         "suggested_fix": "...",
         "cycle": 1,
         "disposition": "new"
       }
     ]
   }
   ```
   Verify: no findings with severity `error`, `warning`, or `suggestion`.
   ```bash
   jq '[.[] | .[] | select(.severity != "info")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   # Expected: 0
   ```

7. **acceptance.json exists and all pass** (0002b artifact):
   ```bash
   cat ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json | jq .
   ```
   Expected structure -- object with `.results` array, `.all_pass`, and `.has_fail_or_unclear`:
   ```json
   {
     "results": [
       {
         "criterion": "calc.Subtract(5, 3) returns 2",
         "status": "pass",
         "rationale": "Test TestSubtract verifies Subtract(5, 3) == 2 and passes.",
         "evidence_refs": ["evidence/validation.json", "evidence/diff-summary.md"]
       },
       {
         "criterion": "All existing tests continue to pass",
         "status": "pass",
         "rationale": "go test ./calc/... exits 0 with all tests passing.",
         "evidence_refs": ["evidence/validation.json"]
       }
     ],
     "all_pass": true,
     "has_fail_or_unclear": false
   }
   ```
   Verify: all criteria have `"status": "pass"`:
   ```bash
   jq '[.results[] | select(.status != "pass")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: 0
   ```
   Verify: convenience fields are consistent:
   ```bash
   jq '{all_pass, has_fail_or_unclear}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: {"all_pass": true, "has_fail_or_unclear": false}
   ```
   Verify: each criterion has non-empty `rationale` and `evidence_refs`:
   ```bash
   jq '[.results[] | select(.rationale == "" or (.evidence_refs | length) == 0)] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: 0
   ```

8. **review.md includes 0002b sections**:
   ```bash
   grep -c "Review Findings" ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.md
   # Expected: >= 1
   grep -c "Acceptance" ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.md
   # Expected: >= 1
   ```
   The review.md should contain:
   - A section for review findings by facet (even if empty)
   - A per-criterion acceptance table with status/rationale/evidence

9. **events.jsonl contains review and accept events**:
   ```bash
   grep 'review_result' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq .
   grep 'acceptance_result' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq .
   ```
   Both event types should be present. The review event should reference the facets evaluated. The acceptance event should reference criteria count and pass count.

10. **Execution policy snapshot includes review config**:
   ```bash
   jq '.review' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/execution-policy.json
   # Expected: {"facets": ["spec_alignment", "code_quality"], "tiers": {...}, "replan_threshold": "warning"}
   jq '.models.evaluator' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/execution-policy.json
   # Expected: "high"
   ```

11. **Pipeline ordering**: Verify that in `events.jsonl`, the review event occurs after `final_validation_result` and before `acceptance_result`:
    ```bash
    grep -n 'final_validation_result\|review_result\|acceptance_result' \
      ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl
    ```
    Line numbers should be in order: final_validation_result < review_result < acceptance_result.

12. All 0002a artifacts still present: `summary.md`, `diff-summary.md`, `task-results.json`, `validation.json`, `metrics.json`, `review.md`.

**Pass/Fail Criteria**:
- [ ] Terminal state is `ready_for_review`
- [ ] All three FinalizeStage gates are true: `final_validation_passed`, `final_review_passed`, `final_acceptance_passed`
- [ ] `evidence/review.json` exists with per-facet structure
- [ ] No blocking findings (severity != info) in review.json
- [ ] `evidence/acceptance.json` exists with `.results` array, `.all_pass` true, `.has_fail_or_unclear` false
- [ ] All acceptance criteria have `"status": "pass"`
- [ ] Each criterion has non-empty `rationale` and `evidence_refs`
- [ ] `evidence/review.md` contains review findings and acceptance sections
- [ ] events.jsonl shows review and accept stages in correct order
- [ ] Execution policy snapshot includes `review` and `models.evaluator`

**Cleanup**: Leave artifacts for later inspection.

---

### Scenario 2: Review Finding Triggers Fix Cycle

**Purpose**: Verify that a review finding at or above the configured threshold triggers a fix-cycle replan, and the second cycle's review is clean.

**Setup**:

This scenario needs a spec that will produce working code that passes deterministic validation but has a reviewable gap. Use the add-subtract spec on fixture-calc (simple enough that the code works, but the reviewer may find suggestions). If the reviewer produces no findings, use fixture-multipackage with add-refund-endpoint.md as a fallback (more code surface for review).

```bash
# Ensure replan_threshold is "warning" (default) so warnings and errors trigger a fix cycle
jq '.review.replan_threshold' ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
# Expected: "warning"

# Ensure max_spec_cycles allows at least 2 cycles
jq '.budgets.max_spec_cycles' ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
# Expected: 3
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Expected CLI output** (approximate):
```
[init] Created run 20260312-XXXXXX-XXXXXX
[compile] Spec packet compiled
[plan] Planner produced N tasks
[execute] All N tasks completed
[validate] Final validation: N/N passed
[review] spec_alignment: 0 findings
[review] code_quality: 1 warning finding
  "Missing edge case test for negative overflow"
[replan] Cycle 2 (fix): 1 task targeting review finding
[execute] Task t-00N: add edge case test ... done
[validate] Final validation: N/N passed
[review] spec_alignment: 0 findings
[review] code_quality: 0 findings
[accept] N/N acceptance criteria: pass

Terminal state: ready_for_review
```

**Verify**:

1. CLI output shows at least 2 cycles (initial + fix triggered by review).
2. Terminal state: `ready_for_review`.
3. **review.json contains cycle-1 finding**:
   ```bash
   jq '.. | objects | select(.cycle == 1)' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   ```
   At least one finding should have `"cycle": 1` and severity at or above `"warning"` (the default threshold).

4. **Fix-cycle plan references review finding**:
   ```bash
   grep 'plan_created' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | \
     jq 'select(.kind == "fix") | {cycle, kind, failures_addressed}'
   ```
   The fix plan's `failures_addressed` should reference the review finding (facet, file, description).

5. **Cycle-2 review is clean** (no new blocking findings):
   ```bash
   jq '[.. | objects | select(.cycle == 2 and .severity != "info")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   # Expected: 0 (no new blocking findings in cycle 2)
   ```

6. **acceptance.json shows all pass**:
   ```bash
   jq '[.results[] | select(.status != "pass")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: 0
   jq '{all_pass, has_fail_or_unclear}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: {"all_pass": true, "has_fail_or_unclear": false}
   ```

7. `metrics.json` shows `cycles >= 2`:
   ```bash
   jq .cycles ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   # Expected: >= 2
   ```

8. **events.jsonl contains replan_triggered referencing review**:
   ```bash
   grep 'replan_triggered' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq .
   ```
   Should reference the review stage as the source of the replan.

**Determinism Fallback**: The LLM reviewer may not produce blocking findings on simple code. If cycle 1 review is clean and the run completes in a single cycle:
1. Note the outcome as "reviewer found no issues on simple code" -- this is valid behavior.
2. To force a review finding, use the fixture-multipackage project with a more complex spec, or temporarily lower the threshold and use a spec that produces slightly suboptimal code.
3. The fix-cycle-from-review machinery is also verified via automated integration tests.

**Pass/Fail Criteria**:
- [ ] Run completes in 2+ cycles (or see Determinism Fallback)
- [ ] review.json contains at least one cycle-1 finding at or above threshold
- [ ] Fix-cycle plan references the review finding
- [ ] Cycle-2 review is clean (no new blocking findings)
- [ ] Terminal state is `ready_for_review`
- [ ] acceptance.json shows all criteria pass

**Cleanup**: Leave artifacts for inspection.

---

### Scenario 3: Configurable Threshold -- Suggestions Non-Blocking at Default

**Purpose**: Verify that the default `"warning"` threshold causes suggestion findings to be recorded but not trigger fix cycles, while warnings still trigger replanning. Also verify that overriding to `"error"` causes both warnings and suggestions to be non-blocking.

**Setup (Part A -- default "warning" threshold, suggestions non-blocking)**:
```bash
# Verify default threshold is "warning" -- suggestions should NOT trigger replanning
jq '.review.replan_threshold' ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
# Expected: "warning"
```

Run the spec. If the reviewer produces only suggestion-level findings, verify they are recorded but do not trigger a fix cycle. Then proceed to Part B:

**Setup (Part B -- "error" threshold, warnings also non-blocking)**:
```bash
# Set replan_threshold to "error" -- only error-severity findings trigger replanning
jq '.review.replan_threshold = "error"' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json

# Verify:
jq '.review.replan_threshold' ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
# Expected: "error"
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Expected CLI output** (approximate):
```
[init] Created run 20260312-XXXXXX-XXXXXX
[compile] Spec packet compiled
[plan] Planner produced N tasks
[execute] All N tasks completed
[validate] Final validation: N/N passed
[review] code_quality: 2 warning findings (recorded, not blocking per threshold)
[review] spec_alignment: 0 findings
[accept] N/N acceptance criteria: pass
[evidence] Bundle written

Terminal state: ready_for_review
```

**Verify**:

1. Terminal state: `ready_for_review` (not `needs_human`).
2. Run completes in a **single cycle** (no fix-cycle triggered):
   ```bash
   jq .cycles ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   # Expected: 1
   ```

3. **review.json contains warning/suggestion findings that did NOT block**:
   ```bash
   jq '[.. | objects | select(.severity == "warning" or .severity == "suggestion")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   ```
   This may be 0 if the reviewer found nothing. The key verification is that NO replan was triggered.

4. **No replan_triggered event in events.jsonl** (unless validation failed):
   ```bash
   grep -c 'replan_triggered' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl
   # Expected: 0
   ```

5. **Execution policy snapshot reflects the threshold**:
   ```bash
   jq '.review.replan_threshold' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/execution-policy.json
   # Expected: "error"
   ```

6. **acceptance.json shows all pass**:
   ```bash
   jq '[.results[] | select(.status != "pass")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: 0
   jq '.all_pass' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: true
   ```

7. **review.md records the non-blocking findings** (if any exist):
   ```bash
   cat ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.md
   ```
   Warnings/suggestions should appear in the review findings section with a note that they did not trigger replanning.

**Pass/Fail Criteria**:
- [ ] Terminal state is `ready_for_review`
- [ ] Run completes in 1 cycle (no replan triggered by review)
- [ ] Execution policy snapshot shows `replan_threshold: "error"`
- [ ] Any warning/suggestion findings are recorded in review.json but did not block
- [ ] acceptance.json shows all criteria pass

**Cleanup**:
```bash
# Restore replan_threshold to default
jq '.review.replan_threshold = "warning"' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 4: Acceptance Fail Triggers Fix Cycle

**Purpose**: Verify that an acceptance criterion `fail` triggers a fix-cycle replan, and the second cycle passes acceptance.

**Setup**:

Use the divide-float64 spec on fixture-calc. The spec has clear acceptance criteria (exact return values) that may partially fail if the implementation has edge cases wrong.

```bash
# Ensure the divide-float64 spec is committed
ls /tmp/gromit-fixtures/fixture-calc/specs/divide-float64.md
# If missing, copy from 0002a fixtures
```

Alternatively, use a spec with acceptance criteria that are hard to verify deterministically on the first pass, such as a spec requiring specific error messages.

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/divide-float64.md
```

**Expected CLI output** (approximate):
```
[init] Created run 20260312-XXXXXX-XXXXXX
...
[validate] Final validation: N/N passed
[review] clean
[accept] Criterion 1 "Divide(10, 2) returns 5.0": pass
[accept] Criterion 2 "Divide(10, 3) returns ~3.333": fail
  "Implementation returns integer division result, not float64"
[replan] Cycle 2 (fix): 1 task targeting acceptance gap
[execute] Task t-00N: fix float64 division ... done
[validate] N/N passed
[review] clean
[accept] N/N acceptance criteria: pass

Terminal state: ready_for_review
```

**Verify**:

1. Terminal state: `ready_for_review` (fix cycle resolved the acceptance failure).
2. **Cycle-1 acceptance failure existed** (use events.jsonl as primary verification):
   acceptance.json contains only the final evaluation (all-pass after the fix cycle). To confirm that cycle-1 failures existed, check events.jsonl:
   ```bash
   grep 'acceptance_result' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq .
   ```
   There should be at least two acceptance evaluation events (one per cycle). The cycle-1 event should contain a `fail` status for at least one criterion.

3. **Fix-cycle plan references acceptance failure**:
   ```bash
   grep 'plan_created' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | \
     jq 'select(.kind == "fix") | {cycle, failures_addressed}'
   ```
   The `failures_addressed` should reference the specific criterion that failed and its rationale.

4. **Final acceptance is all pass**:
   ```bash
   jq '[.results[] | select(.status != "pass")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: 0
   jq '{all_pass, has_fail_or_unclear}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: {"all_pass": true, "has_fail_or_unclear": false}
   ```

5. `metrics.json` shows `cycles >= 2`:
   ```bash
   jq .cycles ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   # Expected: >= 2
   ```

**Determinism Fallback**: The acceptance evaluator may pass all criteria on cycle 1 if the implementation is correct. If this happens:
1. Note the outcome as "implementation satisfied all acceptance criteria on first pass."
2. The fix-cycle-from-acceptance machinery is also verified via automated integration tests.
3. To force an acceptance failure, use the unfixable-conflict spec (contradictory criteria) and verify that acceptance fails trigger replanning before budget exhaustion leads to `needs_human`.

**Pass/Fail Criteria**:
- [ ] Run completes in 2+ cycles (or see Determinism Fallback)
- [ ] At least one acceptance criterion failed in an earlier cycle
- [ ] Fix-cycle plan references the specific acceptance failure
- [ ] Final acceptance.json shows all criteria pass
- [ ] Terminal state is `ready_for_review`

**Cleanup**: Leave artifacts for inspection.

---

### Scenario 5: Acceptance Unclear -- Adds Evidence

**Purpose**: Verify that an `unclear` acceptance criterion triggers a fix cycle that targets adding tests or evidence (not re-implementing the feature), and the criterion resolves.

**Setup**:

Create a spec with an acceptance criterion that is hard to evaluate without explicit test evidence. Place it in fixture-calc:

```bash
cat > /tmp/gromit-fixtures/fixture-calc/specs/multiply-with-logging.md << 'SPEC'
# Add Multiply Function With Logging

## spec_id
multiply-with-logging

## Title
Add a Multiply function that logs its inputs

## Problem
The calculator needs a Multiply function that records its inputs for audit purposes.

## In-Scope
- Add a `Multiply(a, b int) int` function to `calc/calc.go`
- The function must record each invocation (inputs and result) to a package-level slice `var AuditLog []string`
- Add tests for Multiply correctness in `calc/calc_test.go`

## Out-of-Scope
- No changes to existing functions
- No external logging libraries

## Acceptance Criteria
1. `calc.Multiply(3, 4)` returns `12`
2. `calc.Multiply(0, 5)` returns `0`
3. After calling Multiply(3, 4), AuditLog contains an entry recording the inputs and result
4. All existing tests continue to pass
5. `go vet ./...` passes

## Architectural Constraints
- All code stays in the `calc` package

## Validation
- `go test ./calc/...`
- `go vet ./...`
SPEC

cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add multiply-with-logging spec"
```

Criterion 3 (AuditLog verification) may be evaluated as `unclear` if no explicit test asserts the audit log content.

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/multiply-with-logging.md
```

**Expected CLI output** (approximate):
```
...
[accept] Criterion 1 "Multiply(3, 4) returns 12": pass
[accept] Criterion 2 "Multiply(0, 5) returns 0": pass
[accept] Criterion 3 "AuditLog contains entry": unclear
  "No test explicitly verifies AuditLog content after Multiply call."
[replan] Cycle 2 (fix): 1 task targeting acceptance gap
[execute] Task t-00N: add AuditLog verification test ... done
[validate] N/N passed
[review] clean
[accept] Criterion 3 "AuditLog contains entry": pass

Terminal state: ready_for_review
```

**Verify**:

1. Terminal state: `ready_for_review`.
2. **acceptance.json shows the unclear-to-pass transition**:
   ```bash
   grep 'acceptance_result' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq .
   ```
   The first evaluation should contain a criterion with `"status": "unclear"`. The final evaluation should show all pass.

3. **Fix-cycle task targets evidence, not re-implementation**:
   ```bash
   # Check fix plan description
   grep 'plan_created' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | \
     jq 'select(.kind == "fix")'
   ```
   The fix task should mention "add test" or "add evidence" rather than "re-implement" or "rewrite".

4. **Final acceptance.json all pass**:
   ```bash
   jq '[.results[] | select(.status != "pass")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: 0
   jq '.all_pass' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: true
   ```

**Determinism Fallback**: The LLM may produce a test for AuditLog on the first pass, making criterion 3 pass immediately. If this happens:
1. Note the outcome as "implementation included evidence on first pass."
2. The unclear-triggers-fix machinery is verified via automated integration tests.

**Pass/Fail Criteria**:
- [ ] At least one criterion was `unclear` in an earlier cycle (or see Determinism Fallback)
- [ ] Fix-cycle task targets adding evidence/tests, not re-implementing
- [ ] Final acceptance.json shows all criteria pass
- [ ] Terminal state is `ready_for_review`

**Cleanup**: Leave artifacts for inspection.

---

### Scenario 6: Budget Exhaustion Across Review + Acceptance

**Purpose**: Verify that combined review and acceptance fix cycles consume from the shared `max_spec_cycles` budget, and budget exhaustion produces `needs_human` with a blocker summary.

**Setup**:
```bash
# Set max_spec_cycles to 2 for faster exhaustion
jq '.budgets.max_spec_cycles = 2' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json

# Use the unfixable-conflict spec (contradictory acceptance criteria guarantee repeated failure)
ls /tmp/gromit-fixtures/fixture-calc/specs/unfixable-conflict.md
# If missing, copy from 0002a fixtures
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/unfixable-conflict.md
```

**Expected CLI output** (approximate):
```
[init] Created run 20260312-XXXXXX-XXXXXX
...
[validate] ...
[review] ...
[accept] Criterion 1 "Add(2,3) returns 6": fail OR pass (depending on implementation)
[accept] Criterion 2 "TestAdd still passes": fail OR pass (contradictory with criterion 1)
[replan] Cycle 2 (fix): ...
...
[accept] ... still failing ...
[budget] max_spec_cycles (2) exhausted

Terminal state: needs_human
Blocker: Acceptance/review failures remain after 2 cycles.
```

**Verify**:

1. Terminal state: `needs_human`.
   ```bash
   jq .state ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/run.json
   # Expected: "needs_human"
   ```

2. **Budget exhaustion is correctly signaled**:
   ```bash
   grep 'terminal_state' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq '{state, reason}'
   ```
   Should show `"needs_human"` with a reason referencing cycle exhaustion.

3. **Blocker summary is present and actionable**:
   ```bash
   jq '.blocker_summary // .blocker // .reason' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/run.json
   ```
   Should describe what failed, what was tried, and recommended next action.

4. **Shared budget accounting**: Verify that validation, review, and acceptance fix cycles all consumed from the same budget:
   ```bash
   jq .cycles ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   # Expected: 2 (equals max_spec_cycles)
   ```

5. **Evidence bundle still emitted** (even on failure):
   ```bash
   ls ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/
   # Should contain: review.json, acceptance.json, review.md, metrics.json, validation.json
   ```

6. **acceptance.json shows remaining failures**:
   ```bash
   jq '[.results[] | select(.status != "pass")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: >= 1
   jq '.has_fail_or_unclear' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json
   # Expected: true
   ```

7. **review.json is present** (may be empty or have findings):
   ```bash
   jq 'keys' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   # Expected: ["spec_alignment", "code_quality"] (at minimum)
   ```

**Pass/Fail Criteria**:
- [ ] Terminal state is `needs_human` (not `blocked` -- cycle exhaustion is task-progress failure)
- [ ] Blocker summary describes what failed and recommends action
- [ ] cycles equals max_spec_cycles (2)
- [ ] Evidence bundle is complete (review.json, acceptance.json, review.md, metrics.json)
- [ ] acceptance.json shows at least one remaining failure

**Cleanup**:
```bash
# Restore max_spec_cycles
jq '.budgets.max_spec_cycles = 3' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 7: Enable Additional Facet Via Config

**Purpose**: Verify that enabling a built-in review facet via execution policy config causes it to run without code changes.

**Setup**:
```bash
# Add "logic_gaps" to the review facets list
jq '.review.facets = ["spec_alignment", "code_quality", "logic_gaps"] | .review.tiers.logic_gaps = "high"' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json

# Verify:
jq '.review.facets' ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
# Expected: ["spec_alignment", "code_quality", "logic_gaps"]
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Expected CLI output** (approximate):
```
...
[review] spec_alignment: 0 findings
[review] code_quality: 0 findings
[review] logic_gaps: 0 findings (or N findings)
[accept] N/N acceptance criteria: pass

Terminal state: ready_for_review
```

**Verify**:

1. CLI output shows `[review] logic_gaps:` line (the new facet ran).
2. **review.json contains the logic_gaps facet**:
   ```bash
   jq 'keys' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   # Expected: includes "logic_gaps"
   ```
   ```bash
   jq '.logic_gaps' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   # Expected: array (may be empty or contain findings)
   ```

3. **Execution policy snapshot reflects the new facet**:
   ```bash
   jq '.review.facets' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/execution-policy.json
   # Expected: ["spec_alignment", "code_quality", "logic_gaps"]
   ```

4. **events.jsonl shows logic_gaps was evaluated**:
   ```bash
   grep 'review_result' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | jq .
   ```
   Should reference all three facets.

5. If logic_gaps produced findings, verify they follow the standard finding format:
   ```bash
   jq '.logic_gaps[] | {severity, file, description}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.json
   ```
   Each finding should have `severity`, `file`, `description` fields.

**Pass/Fail Criteria**:
- [ ] CLI output shows `logic_gaps` facet was evaluated
- [ ] review.json contains `logic_gaps` key with findings array
- [ ] Execution policy snapshot includes `logic_gaps` in facets list
- [ ] No code changes were needed to enable the facet (config-only)
- [ ] Terminal state is `ready_for_review` or `needs_human` (depending on findings)

**Cleanup**:
```bash
# Remove logic_gaps from the facets list
jq '.review.facets = ["spec_alignment", "code_quality"] | del(.review.tiers.logic_gaps)' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 8: New-vs-Preexisting Finding Distinction

**Purpose**: Verify that on fix cycles, the review stage distinguishes new findings from pre-existing ones, and that pre-existing info-level notes do not re-trigger replanning.

**Setup**:

This scenario requires a spec that will produce review findings in cycle 1, then after the fix cycle, the cycle-2 review should label residual findings as `"pre-existing"` and any new ones as `"new"`. Only new findings above threshold should trigger further replanning.

Use the fixture-multipackage project with the add-refund-endpoint spec (more code surface for review):

```bash
# Ensure replan_threshold is "warning" (default)
jq '.review.replan_threshold' ~/.local/share/gromit/projects/fixture-multipackage/policy/execution.json
# Expected: "warning"

# Ensure max_spec_cycles is 3
jq '.budgets.max_spec_cycles' ~/.local/share/gromit/projects/fixture-multipackage/policy/execution.json
# Expected: 3
```

**Execute**:
```bash
./gromit-next exec spec --project fixture-multipackage \
  --spec /tmp/gromit-fixtures/fixture-multipackage/specs/add-refund-endpoint.md
```

**Expected CLI output** (approximate):
```
...
[review] spec_alignment: 1 warning finding
[review] code_quality: 1 info finding
[replan] Cycle 2 (fix): 1 task targeting review finding
[execute] ...
[review] spec_alignment: 0 new findings (1 pre-existing info note)
[review] code_quality: 1 info finding (pre-existing)
[accept] N/N acceptance criteria: pass

Terminal state: ready_for_review
```

**Verify**:

1. Terminal state: `ready_for_review`.
2. **review.json contains findings with disposition labels**:
   ```bash
   jq '.. | objects | select(.disposition != null) | {facet: .facet, severity, disposition, cycle}' \
     ~/.local/share/gromit/projects/fixture-multipackage/runs/$RUN_ID/evidence/review.json
   ```
   Look for findings with `"disposition": "pre-existing"` (matched from a prior cycle) and `"disposition": "new"`.

3. **Pre-existing findings have cycle references**:
   ```bash
   jq '.. | objects | select(.disposition == "pre-existing") | {file, description, cycle}' \
     ~/.local/share/gromit/projects/fixture-multipackage/runs/$RUN_ID/evidence/review.json
   ```
   Pre-existing findings should reference the cycle they were originally found in.

4. **Info-level findings do not trigger replanning**:
   ```bash
   # Count replans
   grep -c 'replan_triggered' ~/.local/share/gromit/projects/fixture-multipackage/runs/$RUN_ID/events.jsonl
   ```
   If a fix cycle resolves targeted findings but surfaces new info-level notes, no additional replan should occur.

5. **Only new findings above threshold trigger replanning**: Cross-reference the `replan_triggered` events with the findings in review.json. Only findings with `"disposition": "new"` and severity >= threshold should have triggered replanning.

**Determinism Fallback**: The reviewer may not produce findings that span multiple cycles with disposition labeling. If the run completes in a single cycle:
1. Note the outcome as "reviewer found no blocking issues."
2. The new-vs-preexisting distinction is also verified via automated integration tests.
3. If the run completes in 2 cycles, manually inspect review.json for disposition labels -- their presence confirms the feature works.

**Pass/Fail Criteria**:
- [ ] review.json findings include `disposition` field (`"new"` or `"pre-existing"`)
- [ ] Pre-existing findings have correct cycle references
- [ ] Info-level findings do not trigger replanning
- [ ] Only new findings above threshold trigger fix cycles
- [ ] Terminal state is `ready_for_review` (or see Determinism Fallback)

**Cleanup**: Leave artifacts for inspection.

---

### Scenario 9: Blocked Worktree Cleanup on Re-run

**Purpose**: Verify that FinalizeStage preserves worktrees for terminal states, and that InitStage auto-cleans `blocked` worktrees from prior runs when re-running the same spec.

**Important distinction**: Per the design spec, InitStage only cleans worktrees from prior runs with `status: blocked`. The `blocked` state is produced by infrastructure-level failures (planner failure, provider unavailability), NOT by budget exhaustion (which produces `needs_human`). This scenario must use a setup that produces `blocked` specifically.

**Note on `needs_human` worktrees**: Worktrees from `needs_human` runs are preserved but NOT auto-cleaned by InitStage on re-run. A `needs_human` run contains partial work product that a human may want to inspect or continue from. Only `blocked` runs (which represent infrastructure failures with no useful work product) are auto-cleaned.

**Setup**:

Trigger a `blocked` state by making the LLM provider unavailable. Set `ANTHROPIC_API_KEY` to an invalid value so the planner call fails as an infrastructure error:

```bash
# Use any existing spec -- the spec content does not matter since the provider call will fail
# Save the real key for restoration later
REAL_KEY="$ANTHROPIC_API_KEY"

export ANTHROPIC_API_KEY="sk-ant-invalid-key-for-blocked-test"
```

**Execute (first run -- expect `blocked` from provider failure)**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/divide-float64.md
```

**Verify worktree preserved**:
1. Note the run-id from output.
2. Terminal state must be `blocked` (not `needs_human`):
   ```bash
   RUN_ID_1=<from output>
   jq .state ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID_1/run.json
   # Expected: "blocked"
   ```
3. Confirm the worktree directory still exists after the run completes:
   ```bash
   jq '.worktree_path // .worktree' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID_1/run.json
   # Verify the directory exists on disk
   ls -la <worktree_path_from_above>
   ```

**Execute (second run -- same spec, verify cleanup of blocked worktree)**:
```bash
# Restore or keep the invalid key depending on whether you want this run to also block
# To test cleanup only, restore the real key so the second run proceeds normally:
export ANTHROPIC_API_KEY="$REAL_KEY"

./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/divide-float64.md
```

**Verify old worktree cleaned**:
1. The first run's worktree should be removed:
   ```bash
   ls <worktree_path_from_run_1> 2>&1
   # Expected: "No such file or directory"
   ```
2. The first run's `run.json` should have its worktree path cleared:
   ```bash
   jq '.worktree_path // .worktree' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID_1/run.json
   # Expected: "" or null
   ```
3. **`blocked_worktree_cleaned` event emitted** by the second run:
   ```bash
   grep 'blocked_worktree_cleaned' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID_2/events.jsonl | jq .
   # Expected: event referencing the first run's ID
   ```
4. The second run should have its own worktree:
   ```bash
   RUN_ID_2=<from output>
   jq '.worktree_path // .worktree' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID_2/run.json
   ls -la <worktree_path_from_run_2>
   ```

**Pass/Fail Criteria**:
- [ ] First run terminal state is `blocked` (provider failure, not budget exhaustion)
- [ ] First run's worktree is preserved after `blocked` terminal state
- [ ] Second run of the same spec auto-cleans the first run's `blocked` worktree
- [ ] Second run emits `blocked_worktree_cleaned` event referencing first run
- [ ] Second run creates its own new worktree successfully

**Cleanup**:
```bash
# Ensure the real API key is restored
export ANTHROPIC_API_KEY="$REAL_KEY"
```

---

## 3. Artifact Verification Checklist (0002b Extensions)

Use this after any scenario to systematically verify the 0002b-specific artifacts.

### 3.1 Extended Evidence Directory

For `RUN_DIR/evidence/`:

| File | Must Exist | Check |
|------|-----------|-------|
| `review.json` | Always (0002b) | Valid JSON. Object keyed by facet name. Each value is an array of findings. Each finding has `severity`, `file`, `description`, `cycle`, `disposition` (required), and `line`, `suggested_fix` (optional, omitempty). |
| `acceptance.json` | Always (0002b) | Valid JSON object with `.results` array (each element has `criterion`, `status`, `rationale`, `evidence_refs`), plus `.all_pass` bool and `.has_fail_or_unclear` bool. |
| `review.md` | Always (updated) | Contains review findings by facet section. Contains per-criterion acceptance table with status/rationale/evidence. |
| `summary.md` | Always (0002a) | Unchanged. |
| `diff-summary.md` | If code changed (0002a) | Unchanged. |
| `task-results.json` | Always (0002a) | Unchanged. |
| `validation.json` | If validation ran (0002a) | Unchanged. |
| `metrics.json` | Always (0002a) | Unchanged. |

### 3.2 review.json Spot-Checks

```bash
# Verify structure: object keyed by facet names
jq 'keys' RUN_DIR/evidence/review.json
# Expected: ["spec_alignment", "code_quality"] (or more if additional facets enabled)

# Verify finding schema (for each finding)
jq '.. | objects | select(.severity != null) | keys' RUN_DIR/evidence/review.json | sort -u
# Each finding should have: severity, file, description, cycle, disposition (required); line, suggested_fix (optional, omitempty)

# Verify severity values are valid
jq '[.. | objects | select(.severity != null) | .severity] | unique' RUN_DIR/evidence/review.json
# Expected: subset of ["error", "warning", "suggestion", "info"]

# Verify disposition values are valid
jq '[.. | objects | select(.disposition != null) | .disposition] | unique' RUN_DIR/evidence/review.json
# Expected: subset of ["new", "pre-existing"]
```

### 3.3 acceptance.json Spot-Checks

```bash
# Verify it's an object with .results array
jq '.results | type' RUN_DIR/evidence/acceptance.json
# Expected: "array"

# Verify top-level convenience fields
jq '{all_pass, has_fail_or_unclear}' RUN_DIR/evidence/acceptance.json
# Expected: {"all_pass": true/false, "has_fail_or_unclear": true/false}

# Verify each element has required fields
jq '[.results[] | select(.criterion == null or .status == null or .rationale == null or .evidence_refs == null)] | length' \
  RUN_DIR/evidence/acceptance.json
# Expected: 0

# Verify status values
jq '[.results[] | .status] | unique' RUN_DIR/evidence/acceptance.json
# Expected: subset of ["pass", "fail", "unclear"]

# Verify evidence_refs are non-empty arrays
jq '[.results[] | select((.evidence_refs | length) == 0)] | length' RUN_DIR/evidence/acceptance.json
# Expected: 0

# Verify all_pass consistency
jq '(.all_pass == ([.results[] | .status] | all(. == "pass")))' RUN_DIR/evidence/acceptance.json
# Expected: true

# Verify has_fail_or_unclear consistency
jq '(.has_fail_or_unclear == ([.results[] | .status] | any(. == "fail" or . == "unclear")))' RUN_DIR/evidence/acceptance.json
# Expected: true
```

### 3.4 Extended Events Spot-Checks

For a successful 0002b run, expect all 0002a events plus:
- `review_result` (>= 1, one per cycle that reaches review)
- `acceptance_result` (>= 1, one per cycle that reaches acceptance)

Conditional events:
- `replan_triggered` with review or acceptance source -- only when review/acceptance triggers fix cycle

```bash
# Count 0002b event types
grep -E 'review_result|acceptance_result' RUN_DIR/events.jsonl | \
  jq -r .event_type | sort | uniq -c
```

### 3.5 Extended Pipeline Ordering

Verify stage ordering in events.jsonl follows the 0002b pipeline:
```
Init -> Compile -> Plan -> Execute -> Validate -> Review -> Accept -> Evidence -> Finalize
```

```bash
grep -E 'stage_started|stage_completed' RUN_DIR/events.jsonl | jq -r .stage
```

The sequence should show: validate before review, review before accept, accept before evidence.

---

## 4. Extended Execution Policy Schema

The 0002b execution policy extends the 0002a schema with:

```json
{
  "always_run": [...],
  "budgets": {...},
  "models": {
    "planner": "high",
    "executor": "medium",
    "evaluator": "high"
  },
  "review": {
    "facets": ["spec_alignment", "code_quality"],
    "tiers": {
      "spec_alignment": "high",
      "code_quality": "medium"
    },
    "replan_threshold": "warning"
  }
}
```

### Key fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `models.evaluator` | string | `"high"` | Model tier for review and acceptance stages |
| `review.facets` | string[] | `["spec_alignment", "code_quality"]` | Which built-in facets to run |
| `review.tiers` | object | per-facet defaults | Model tier override per facet |
| `review.replan_threshold` | string | `"warning"` | Minimum severity to trigger replanning. Default `"warning"` prevents churn from subjective suggestions while catching real bugs. |

### Available built-in facets

| Facet | What it checks |
|-------|---------------|
| `spec_alignment` | Does the diff implement what the spec asked for? |
| `code_quality` | Naming, structure, duplication, readability |
| `logic_gaps` | Off-by-one, nil handling, missing error paths |
| `test_coverage` | Are new code paths tested? Missing edge cases? |
| `architecture_drift` | Does the change respect boundaries from the project cell? |

---

## 5. New Spec Files for 0002b

### multiply-with-logging.md (Scenario 5)

Already created in Scenario 5 setup. If not yet committed:

```bash
cat > /tmp/gromit-fixtures/fixture-calc/specs/multiply-with-logging.md << 'SPEC'
# Add Multiply Function With Logging

## spec_id
multiply-with-logging

## Title
Add a Multiply function that logs its inputs

## Problem
The calculator needs a Multiply function that records its inputs for audit purposes.

## In-Scope
- Add a `Multiply(a, b int) int` function to `calc/calc.go`
- The function must record each invocation (inputs and result) to a package-level slice `var AuditLog []string`
- Add tests for Multiply correctness in `calc/calc_test.go`

## Out-of-Scope
- No changes to existing functions
- No external logging libraries

## Acceptance Criteria
1. `calc.Multiply(3, 4)` returns `12`
2. `calc.Multiply(0, 5)` returns `0`
3. After calling Multiply(3, 4), AuditLog contains an entry recording the inputs and result
4. All existing tests continue to pass
5. `go vet ./...` passes

## Architectural Constraints
- All code stays in the `calc` package

## Validation
- `go test ./calc/...`
- `go vet ./...`
SPEC

cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add multiply-with-logging spec"
```

---

## 6. Bug Fix Verification (0002a Prerequisite Checks)

These three bugs were discovered during 0002a manual testing. They must be fixed for 0002b scenarios to work.

### Bug Fix 1: Agent Provider Wiring

**What was broken**: `cmd/gromit-next/exec.go:57-63` -- `defaultStageProvider.BuildStages()` returned a stub error: `"agent provider not configured: real stage wiring requires LLM agent dependencies (see Spec 0002b)"`.

**Verify the fix**:
```bash
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md 2>&1 | head -5
```

**Pass**: CLI output shows `[init]`, `[compile]`, `[plan]` stages progressing (not an immediate error about "agent provider not configured").

**Fail**: CLI immediately prints `Error: agent provider not configured: real stage wiring requires LLM agent dependencies`.

- [ ] Bug Fix 1 passes -- BuildStages no longer returns stub error

### Bug Fix 2: `spec list` Path Resolution

**What was broken**: `cmd/gromit-next/spec.go:119` -- `LoadProjectConfig(".")` looked for `project.json` in cwd instead of resolving from `--project` flag.

**Verify the fix**:
```bash
# Run from a directory that is NOT a project cell (e.g., /tmp)
cd /tmp
/Users/dabrams/gromit/gromit-next spec list --project fixture-calc
```

**Pass**: Command outputs a spec list table (or empty table) without error.

**Fail**: Command prints `Error: load project config: read project config: open ./project.json: no such file or directory`.

```bash
# Return to gromit directory
cd /Users/dabrams/gromit
```

- [ ] Bug Fix 2 passes -- spec list resolves project path from --project flag

### Bug Fix 3: `exec list` Exit Code on Empty Results

**What was broken**: `exec list --project <name>` returned exit code 1 when no runs exist.

**Verify the fix**:
```bash
# Use a project with no runs (or create a fresh one)
./gromit-next exec list --project fixture-greeter 2>/dev/null
echo "Exit code: $?"
```

**Pass**: Exit code is 0 (empty result is not an error).

**Fail**: Exit code is 1.

- [ ] Bug Fix 3 passes -- exec list returns 0 on empty results

---

## 7. Regression Checks

### 7.1 Smoke Test Sequence

After any code change to Spec 0002b packages, run this minimal sequence:

```bash
# 1. Build
cd /Users/dabrams/gromit
go build -o gromit-next ./cmd/gromit-next/

# 2. Unit tests for all Spec 0002a + 0002b packages
go test ./internal/next/execpolicy/...
go test ./internal/next/runstore/...
go test ./internal/next/planner/...
go test ./internal/next/executor/...
go test ./internal/next/validator/...
go test ./internal/next/evidence/...
go test ./internal/next/specloop/...
go test ./internal/next/review/...
go test ./internal/next/acceptor/...

# 3. Quick happy-path end-to-end (Scenario 1)
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
# Confirm: terminal state is ready_for_review
# Confirm: evidence/review.json and evidence/acceptance.json exist

# 4. Quick dry-run (verify review/accept stages are filtered out)
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md --dry-run
# Confirm: plan output, no review or acceptance stages

# 5. CLI inspection
./gromit-next exec list --project fixture-calc
./gromit-next exec show latest --project fixture-calc
./gromit-next spec list --project fixture-calc
# Confirm: commands return without error, output is formatted
```

### 7.2 What to Re-test After Specific Changes

| Changed Package | Re-run Scenarios |
|----------------|-----------------|
| `review` | 1, 2, 3, 7, 8 |
| `acceptor` | 1, 4, 5, 6 |
| `specloop` (replan wiring) | 2, 4, 5, 6 |
| `evidence` (bundle generation) | 1, 6 (check review.json, acceptance.json, review.md) |
| `execpolicy` (review config) | 3, 7 |
| `specloop/stages` (init, finalize) | 1, 9 |
| CLI wiring (exec.go) | All scenarios |

### 7.3 Pre-merge Checklist

Before merging Spec 0002b implementation:

- [ ] All unit tests pass: `go test ./internal/next/...`
- [ ] Bug Fix 1 (Agent Provider Wiring) verified
- [ ] Bug Fix 2 (spec list Path Resolution) verified
- [ ] Bug Fix 3 (exec list Exit Code) verified
- [ ] Scenario 1 (Happy Path) -- review.json empty/info-only, acceptance.json all pass, `ready_for_review`
- [ ] Scenario 2 (Review Finding Fix Cycle) -- finding triggers fix, cycle 2 clean
- [ ] Scenario 3 (Configurable Threshold) -- warnings non-blocking with `replan_threshold: "error"`
- [ ] Scenario 4 (Acceptance Fail Fix Cycle) -- fail triggers fix, cycle 2 passes
- [ ] Scenario 5 (Acceptance Unclear) -- unclear triggers evidence-adding fix
- [ ] Scenario 6 (Budget Exhaustion) -- `needs_human` with blocker summary
- [ ] Scenario 7 (Enable Additional Facet) -- `logic_gaps` runs from config alone
- [ ] Scenario 8 (New-vs-Preexisting) -- disposition labels present, info notes non-blocking
- [ ] Scenario 9 (Blocked Worktree Cleanup) -- `blocked` worktree preserved, auto-cleaned on re-run with `blocked_worktree_cleaned` event
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` produces no output
- [ ] No Gromit files committed to fixture repos' main branches
- [ ] evidence/review.json follows the documented schema (Section 3.2)
- [ ] evidence/acceptance.json follows the documented schema (Section 3.3)
- [ ] Pipeline ordering is correct: Validate -> Review -> Accept -> Evidence (Section 3.5)
