# Spec 0002c/0002d Manual Test Plan

**Specs**: Provider-Agnostic Adapter Layer (0002c) & Multi-Provider Routing (0002d)
**Date**: 2026-03-13
**Scope**: Human QA walkthrough of all 0002c/0002d acceptance criteria -- adapter wiring, contract tests, cost tracking, timeout enforcement, provider fallback, routing, and ExtractJSON robustness.

---

## 1. Prerequisites

### 1.1 System Requirements

- Go 1.22+ installed (`go version`)
- Git 2.30+ installed (`git --version` -- needs worktree support)
- `ANTHROPIC_API_KEY` set in environment (for Claude contract tests and end-to-end)
- `codex` binary installed and on PATH (for 0002d Codex contract tests; `which codex`)
- `jq` installed for JSON inspection (`jq --version`)
- ~500 MB free disk for worktrees and fixture repos
- Specs 0002a and 0002b must be complete (execution loop and review/accept stages operational)

### 1.2 Build the Binary

```bash
cd /Users/dabrams/gromit
go build -o gromit-next ./cmd/gromit-next/
# Verify:
./gromit-next --help
```

Confirm output shows `exec` and `spec` subcommands.

### 1.3 Run All Unit Tests

Verify the codebase is green before manual testing:

```bash
cd /Users/dabrams/gromit
go test ./internal/next/... ./cmd/gromit-next/ -count=1
```

All tests must pass. If any fail, fix them before proceeding.

### 1.4 Verify Provider Availability

```bash
# Claude: verify API key works
echo '{"prompt": "Say hello"}' | timeout 30 claude --model sonnet --print 2>&1 | head -5
# Expected: some output (not an auth error)

# Codex (0002d only): verify binary exists
which codex
codex --version
```

### 1.5 Create Fixture Repos

Reuse the fixture repos from the 0002a manual test plan (see `docs/plans/2026-03-11-spec-0002a-manual-test-plan.md`, Section 4). You need:
- `fixture-calc` -- a tiny Go calculator package
- `fixture-greeter` -- a tiny Go greeting package

If they do not exist, create them per the 0002a plan instructions.

### 1.6 Attach Fixture Projects and Install Policies

```bash
./gromit-next project attach --name fixture-calc --repo /tmp/gromit-fixtures/fixture-calc
./gromit-next project attach --name fixture-greeter --repo /tmp/gromit-fixtures/fixture-greeter
```

Verify project cells exist:

```bash
ls ~/.local/share/gromit/projects/fixture-calc/
ls ~/.local/share/gromit/projects/fixture-greeter/
```

### 1.7 Install Execution Policies with Routing Config

For 0002c testing, use the standard execution policy from 0002a. For 0002d testing, add routing configuration. See Section 4 for exact policy JSON.

```bash
mkdir -p ~/.local/share/gromit/projects/fixture-calc/policy
cp /tmp/gromit-fixtures/policies/fixture-calc-execution-routing.json \
   ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

### 1.8 Run Inspect/Enrich (Spec 0001 baseline)

```bash
./gromit-next context inspect --project fixture-calc
./gromit-next context enrich --project fixture-calc
```

Confirm `validation.json`, `architecture.json`, etc. exist in each project cell.

---

## 2. Test Scenarios

> **Scenario ordering**: Scenarios 1–5 must be run in order, as each builds on artifacts from prior scenarios. Scenarios 6–12 can be run independently after Scenario 1 completes (they only require the binary and provider access from the prerequisites).

### Notation

- `RUN_DIR` = `~/.local/share/gromit/projects/<project>/runs/<run-id>/`
- All `Verify` steps use the actual run-id printed by the CLI.
- Cleanup: each scenario notes whether to clean up or leave artifacts.
- `GROMIT` = `./gromit-next` (the built binary)

---

### Scenario 1: End-to-End Happy Path with Claude

**Purpose**: Verify that a spec executes through the full pipeline with real Claude-backed adapters (not noops), all stages complete, cost is tracked, and the evidence bundle is produced.

**Setup**:
```bash
# Ensure fixture-calc is attached and enriched (see Prerequisites)
# Use the add-subtract spec from 0002a fixtures:
cp /tmp/gromit-fixtures/specs/add-subtract.md /tmp/gromit-fixtures/fixture-calc/specs/
cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add subtract spec" --allow-empty
```

Install the standard execution policy (no routing -- Claude-only, 0002c mode):

```bash
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
    "evaluator": "medium"
  }
}
POLICY
```

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. CLI output shows stages: `[init]`, `[compile]`, `[plan]`, `[execute]`, `[validate]`, `[review]`, `[accept]`, `[evidence]`.
2. Terminal state printed: `ready_for_review` or `completed`.
3. Run directory exists:
   ```bash
   RUN_ID=<from output>
   ls ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/
   ```
4. **Cost tracking**: Verify cost was tracked through the adapter layer:
   ```bash
   jq '.total_cost_usd' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   - `total_cost_usd` must be > 0 (real LLM calls cost money).
   - Each invocation record must have `cost_usd > 0`:
     ```bash
     jq '.invocations[] | {phase, cost_usd, model}' \
       ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
     ```
5. **Provider identification**: Verify invocation records identify the provider:
   ```bash
   jq '.invocations[] | {phase, model, provider}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   - All invocations should show provider `"claude"` (0002c hardcodes Claude).
   - Model names should match Claude tier mappings (e.g., `"opus"` for high, `"sonnet"` for medium).
6. **Review stage**: Review was performed by real LLM (not noop):
   ```bash
   cat ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/review.md
   ```
   - Should contain substantive findings, not placeholder text.
   - Should reference actual code changes.
7. **Acceptance stage**: If acceptance ran, criterion results should be parseable:
   ```bash
   jq '.' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/acceptance.json 2>/dev/null
   ```
   - Each criterion result should have `pass` (boolean), `rationale` (non-empty string).
8. Evidence bundle complete (same checks as 0002a Scenario 1, Section 3).
9. Worktree exists and is valid.

**Expected**: Full pipeline with real Claude. Cost > 0. Provider = "claude" on all invocations. Review contains substantive findings. Evidence bundle complete.

**Cleanup**: Leave artifacts for later scenarios.

---

### Scenario 2: Adapter Wiring Verification

**Purpose**: Verify that `RealStageProvider` wires real adapters (not noops) for each stage, and each stage invokes the correct adapter type.

**Setup**: Use the artifacts from Scenario 1, or run a new execution.

**Execute**:
```bash
# Run with verbose/debug logging if available:
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md \
  --verbose 2>&1 | tee /tmp/adapter-wiring-output.txt
```

**Verify**:

1. **Plan stage uses ProviderPlanAgent**: Inspect logs or events for planner invocation:
   ```bash
   grep -i 'plan' /tmp/adapter-wiring-output.txt | head -10
   grep 'plan_created' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | \
     jq '{event_type, provider, model}'
   ```
   - Plan was created by a real LLM call (non-empty, well-structured plan).
   - `plan.md` contains tasks with `task_id`, `objective`, `expected_touched_area`.

2. **Execute stage uses ProviderTaskRunner**: Inspect task results:
   ```bash
   for d in ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/tasks/*/; do
     echo "=== $(basename $d) ==="
     jq '{status, model_tier, tokens_used}' "$d/result.json"
     wc -l "$d/agent-output.txt"
   done
   ```
   - Each task has `agent-output.txt` with non-trivial content (real LLM output, not empty/placeholder).
   - `tokens_used > 0` for each task.

3. **Validate stage uses ShellValidator**: Verify validation ran shell commands, not LLM:
   ```bash
   jq '.' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/validation.json
   ```
   - Validation results have `command` fields matching `go test`, `go vet`, `gofmt`.
   - No LLM invocation records for the validate phase in `metrics.json`:
     ```bash
     jq '[.invocations[] | select(.phase == "validate")] | length' \
       ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
     ```
     Expected: 0 (validation does not use LLM).

4. **Review stage uses ProviderReviewAgent**: Verify review used LLM:
   ```bash
   jq '[.invocations[] | select(.phase == "review")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: >= 1 (at least one LLM invocation for review).

5. **Accept stage uses ProviderAcceptAgent**: Verify acceptance used LLM:
   ```bash
   jq '[.invocations[] | select(.phase == "accept")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: >= 1 (at least one LLM invocation for acceptance).

6. **Compile stage uses SpecCompilerAdapter**: Verify compilation is deterministic (no LLM):
   ```bash
   jq '[.invocations[] | select(.phase == "compile")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: 0 (compile does not use LLM).

**Expected**: Each stage uses the correct adapter type. LLM invocations recorded for plan, execute, review, accept. No LLM invocations for compile or validate. All adapters are real (not noops).

**Cleanup**: None needed.

---

### Scenario 3: Contract Tests Against Claude

**Purpose**: Verify all contract test suites pass against real Claude provider, confirming structural compliance of LLM output parsing.

**Setup**:
```bash
# Ensure ANTHROPIC_API_KEY is set
echo $ANTHROPIC_API_KEY | head -c 10
# Should show "sk-ant-..." prefix
```

**Execute**:
```bash
cd /Users/dabrams/gromit

# Run all contract tests against Claude
GROMIT_LLM_CONTRACT=1 go test ./internal/next/planner/ \
  -run TestContract_ProviderPlanAgent_Claude -v -count=1 -timeout 120s

GROMIT_LLM_CONTRACT=1 go test ./internal/next/review/ \
  -run TestContract_ProviderReviewAgent_Claude -v -count=1 -timeout 120s

GROMIT_LLM_CONTRACT=1 go test ./internal/next/acceptor/ \
  -run TestContract_ProviderAcceptAgent_Claude -v -count=1 -timeout 120s

GROMIT_LLM_CONTRACT=1 go test ./internal/next/specloop/ \
  -run TestContract_ProviderTaskRunner_Claude -v -count=1 -timeout 120s

GROMIT_LLM_CONTRACT=1 go test ./internal/next/validator/ \
  -run TestContract_ShellValidator -v -count=1 -timeout 60s
```

Or run all at once:
```bash
GROMIT_LLM_CONTRACT=1 go test ./internal/next/... \
  -run 'TestContract.*Claude|TestContract_ShellValidator' -v -count=1 -timeout 300s
```

**Verify**:

1. **PlanAgent contract**: All subtests pass:
   - `returns valid plan for well-formed prompt` -- plan has tasks, tasks have objectives
   - `tasks have required fields (task_id, objective, expected_touched_area)` -- structural compliance
   - `handles empty prompt gracefully` -- no panic
   - `respects context cancellation` -- returns error on cancelled ctx

2. **ReviewAgent contract**: All subtests pass:
   - `returns valid findings for well-formed prompt` -- findings parse as `[]Finding`
   - `returns empty findings not nil for clean code` -- empty slice, not nil
   - `findings have required fields (File, Description, Severity)` -- structural compliance

3. **AcceptAgent contract**: All subtests pass:
   - `result has pass/fail/unclear` -- valid enum value
   - `rationale non-empty` -- explanatory text present
   - `handles empty prompt gracefully`

4. **TaskRunner contract**: All subtests pass:
   - `RunTask returns status` -- valid status value
   - `RepairTask includes failure context` -- references provided failures

5. **ShellValidator contract**: All subtests pass:
   - `passing checks produce pass result`
   - `failing checks produce failure reported correctly`

6. All assertions are structural (parseable output, required fields, valid enum values). No content quality assertions.

**Expected**: All contract test suites PASS against Claude. This is a hard completion gate for 0002c.

**Cleanup**: None needed.

---

### Scenario 4: Cost Callback Verification

**Purpose**: Verify that `OnCost` callbacks fire correctly through the adapter layer and that budget tracking accumulates cost from all stages.

**Setup**: Use the standard fixture-calc setup with a moderate cost budget.

```bash
# Set a visible but non-blocking cost budget
jq '.budgets.max_run_cost_usd = 50.0' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. **Total cost is sum of invocation costs**: Verify the total matches the sum:
   ```bash
   RUN_ID=<from output>
   METRICS=~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json

   # Total cost
   jq '.total_cost_usd' $METRICS

   # Sum of invocation costs
   jq '[.invocations[].cost_usd] | add' $METRICS

   # These should be approximately equal (floating-point tolerance)
   ```

2. **Each LLM stage contributed cost**: Verify non-zero cost per phase:
   ```bash
   jq '.invocations | group_by(.phase) | map({phase: .[0].phase, total_cost: (map(.cost_usd) | add)})' $METRICS
   ```
   - `plan` phase: cost > 0
   - `execute` phase: cost > 0
   - `review` phase: cost > 0
   - `accept` phase: cost > 0

3. **Budget tracking reflects accumulated cost**: If the run has a `run.json` with budget info:
   ```bash
   jq '{status, accumulated_cost_usd}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/run.json
   ```
   - `accumulated_cost_usd` should be > 0 and match `total_cost_usd` from metrics.

4. **OnCost callback fires per invocation**: The number of invocation records in metrics should match the number of LLM calls made:
   ```bash
   jq '.invocations | length' $METRICS
   ```
   - At minimum: 1 (plan) + N (execute, one per task) + M (review, one per facet) + K (accept, one per criterion) > 1.

**Expected**: Cost accumulates correctly through OnCost callbacks. Total matches sum of individual invocations. All LLM stages contribute cost.

**Cleanup**: None needed.

---

### Scenario 5: Timeout Enforcement

**Purpose**: Verify that adapter-level timeout works and context cancellation propagates through `LLMAdapter`.

**Setup**:
```bash
# Set an extremely short per-task timeout to trigger timeout during execution
jq '.budgets.max_task_duration_seconds = 3' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

Use a spec that requires non-trivial work (broad-refactor.md from 0002a fixtures):
```bash
cp /tmp/gromit-fixtures/specs/broad-refactor.md /tmp/gromit-fixtures/fixture-calc/specs/
cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add broad-refactor spec" --allow-empty
```

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/broad-refactor.md 2>&1 | tee /tmp/timeout-output.txt
```

**Verify**:

1. **Task timeout triggers**: At least one task should fail due to timeout:
   ```bash
   RUN_ID=<from output>
   for d in ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/tasks/*/; do
     echo "=== $(basename $d) ==="
     jq '{status, duration_ms}' "$d/result.json"
   done
   ```
   - At least one task has `duration_ms` near the 3-second limit (3000ms +/- tolerance).
   - Failed tasks should have a timeout-related reason.

   **Note**: If Claude responds in < 3 seconds for a trivial spec, the timeout will not trigger. Use a spec requiring substantial output (e.g., `broad-refactor.md`) to maximize the chance of exceeding the limit. Also, the actual observed `duration_ms` may exceed 3000ms by a margin because context cancellation propagation is asynchronous — the provider process may not terminate instantly.

2. **Context cancellation propagated**: The adapter did not hang indefinitely:
   - The run completed (did not need to be killed manually).
   - CLI output mentions timeout or context cancelled.

3. **Run-level timeout**: Set an extremely short run timeout:
   ```bash
   jq '.budgets.max_run_duration_seconds = 10 | .budgets.max_task_duration_seconds = 300' \
     ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
     && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json

   $GROMIT exec spec --project fixture-calc \
     --spec /tmp/gromit-fixtures/fixture-calc/specs/broad-refactor.md 2>&1 | tee /tmp/run-timeout-output.txt
   ```
   - Terminal state: `blocked`.
   - Run completes within approximately 10 seconds (not hanging).
   - `events.jsonl` contains `budget_exceeded` with `"budget": "time"`.

**If the timeout does not trigger**: If the LLM responds within the timeout for all tasks, mark this scenario as **DEFERRED TO UNIT TEST**. Verify that `TestInvoke_TimeoutEnforcement_CancelsContext` passes in the automated test suite (`go test ./internal/next/llmadapter/ -run TestInvoke_TimeoutEnforcement -v`). Record the unit test output as evidence instead.

**Expected**: Adapter-level timeout cancels context. Provider calls do not hang. Run-level timeout enforced.

**Cleanup**:
```bash
jq '.budgets.max_task_duration_seconds = 300 | .budgets.max_run_duration_seconds = 3600' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 6: Provider Fallback on Usage Limit (0002d)

**Purpose**: Verify that `FallbackAdapter` detects a usage-limit error from the primary provider (Claude) and transparently falls back to Codex.

**Setup**:

This scenario is difficult to trigger deterministically with real providers because usage limits are account-level. Two approaches:

**Approach A -- Unit test verification (recommended)**:
```bash
cd /Users/dabrams/gromit
go test ./internal/next/llmadapter/ -run TestFallbackAdapter -v -count=1
```

Verify these tests pass:
- `TestFallbackAdapter_NormalInvocation_NoFallback`
- `TestFallbackAdapter_UsageLimit_FallsBackToRouter`
- `TestFallbackAdapter_NonUsageLimitError_NoFallback`
- `TestFallbackAdapter_AllProvidersExhausted_ReturnsError`
- `TestFallbackAdapter_SatisfiesProviderAwareInvoker`
- `TestFallbackAdapter_Provider_ReturnsPrimaryProvider`

**Approach B -- Simulated usage limit via env manipulation**:
```bash
# Save real key
REAL_KEY="$ANTHROPIC_API_KEY"

# Install a routing policy with Claude + Codex
cat > ~/.local/share/gromit/projects/fixture-calc/policy/execution.json << 'POLICY'
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
    "evaluator": "medium"
  },
  "routing": {
    "preferences": {"plan": "any", "execute": "any", "review": "any", "accept": "any"},
    "ratio": {"claude": 50, "codex": 50},
    "cooldown_seconds": 60
  }
}
POLICY

# Set an invalid Claude key to simulate usage limit
# (Note: this may produce auth error, not usage-limit error.
# The FallbackAdapter only falls back on IsUsageLimitError, not auth errors.
# This approach may NOT trigger fallback -- see note below.)
export ANTHROPIC_API_KEY="sk-invalid-key-00000000"

$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md 2>&1 | tee /tmp/fallback-output.txt

# Restore real key
export ANTHROPIC_API_KEY="$REAL_KEY"
```

**Note**: An invalid API key produces an authentication error, NOT a usage-limit error. The `FallbackAdapter` only triggers fallback on `IsUsageLimitError()`. For a true usage-limit test, you must either:
1. Exhaust your Claude API quota, or
2. Rely on the unit tests (Approach A), which mock the usage-limit detection.

**Expected behavior for Approach B**: The run will likely FAIL to trigger fallback. The primary provider will fail with an authentication error, and Codex will NOT be invoked. This is correct behavior — it confirms that `FallbackAdapter` correctly distinguishes auth errors from usage-limit errors. If this occurs, record the auth error output and mark Approach B as PASS (auth-error-distinction verified).

**Verify (Approach A)**:

1. All `TestFallbackAdapter_*` tests pass.
2. `TestFallbackAdapter_UsageLimit_FallsBackToRouter` confirms:
   - Primary provider (Claude mock) returns usage-limit error.
   - `router.MarkUnavailable` is called for the primary.
   - Fallback provider (Codex mock) is invoked.
   - Result comes from fallback provider.
3. `TestFallbackAdapter_AllProvidersExhausted_ReturnsError` confirms:
   - When fallback also returns nil, error contains `"all providers exhausted"`.

**Verify (Approach B, if usage limit triggered)**:

1. CLI output logs: `"provider claude hit usage limit, attempting fallback"`.
2. The run continues with Codex as the provider.
3. Metrics show mixed provider invocations:
   ```bash
   RUN_ID=<from output>
   jq '.invocations[] | {phase, provider}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   - Early invocations: `provider: "claude"`.
   - Later invocations after fallback: `provider: "codex"`.

**Expected**: FallbackAdapter correctly detects usage-limit errors and transparently switches to fallback provider. Unit tests prove the mechanism; end-to-end is opportunistic.

**Cleanup**:
```bash
export ANTHROPIC_API_KEY="$REAL_KEY"
```

---

### Scenario 7: Router Phase Preferences (0002d)

**Purpose**: Verify that per-phase provider preferences in `RoutingConfig` cause the correct provider to be selected for each stage.

**Setup**:

Install a routing policy with phase-specific preferences:

```bash
cat > ~/.local/share/gromit/projects/fixture-calc/policy/execution.json << 'POLICY'
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
    "evaluator": "medium"
  },
  "routing": {
    "preferences": {
      "plan": "claude",
      "execute": "codex",
      "review": "claude",
      "accept": "claude"
    },
    "ratio": {"claude": 60, "codex": 40},
    "cooldown_seconds": 300
  }
}
POLICY
```

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md 2>&1 | tee /tmp/routing-output.txt
```

**Verify**:

1. **Plan stage used Claude**: Inspect invocations:
   ```bash
   RUN_ID=<from output>
   jq '.invocations[] | select(.phase == "plan") | {phase, provider, model}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: `provider: "claude"`.

2. **Execute stage used Codex**: Inspect invocations:
   ```bash
   jq '.invocations[] | select(.phase == "execute") | {phase, provider, model}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: `provider: "codex"`.

3. **Review stage used Claude**:
   ```bash
   jq '.invocations[] | select(.phase == "review") | {phase, provider, model}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: `provider: "claude"`.

4. **Accept stage used Claude**:
   ```bash
   jq '.invocations[] | select(.phase == "accept") | {phase, provider, model}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: `provider: "claude"`.

5. **No validate-phase LLM invocations** (ShellValidator is not LLM-backed):
   ```bash
   jq '[.invocations[] | select(.phase == "validate")] | length' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   Expected: 0.

6. **Pipeline completed successfully**: Terminal state is `ready_for_review` or `completed`.

**Determinism Fallback**: If Codex is not installed (`which codex` fails), the Router should fall back to Claude for all phases. In that case:
- All invocations show `provider: "claude"`.
- Log output may mention "codex not available" or similar.
- This is acceptable behavior for single-provider mode (see Scenario 10).
- **Mark this as a DEGRADED PASS** — routing was not actually tested. The scenario must be re-run with Codex available before 0002d can be considered fully verified.

**Expected**: Provider selection respects per-phase preferences. Plan and review use Claude; execute uses Codex (when available).

**Cleanup**: Restore standard policy (without routing):
```bash
cat > ~/.local/share/gromit/projects/fixture-calc/policy/execution.json << 'POLICY'
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
    "evaluator": "medium"
  }
}
POLICY
```

---

### Scenario 8: Contract Tests Against Codex (0002d)

**Purpose**: Verify all contract test suites pass against real Codex provider, confirming structural compliance and prompt compatibility.

**Prerequisite**: `codex` binary must be installed and on PATH.

```bash
which codex || { echo "SKIP: codex not installed"; exit 0; }
```

**Execute**:
```bash
cd /Users/dabrams/gromit

# Run all contract tests against Codex
GROMIT_LLM_CONTRACT=1 go test ./internal/next/planner/ \
  -run TestContract_ProviderPlanAgent_Codex -v -count=1 -timeout 120s

GROMIT_LLM_CONTRACT=1 go test ./internal/next/review/ \
  -run TestContract_ProviderReviewAgent_Codex -v -count=1 -timeout 120s

GROMIT_LLM_CONTRACT=1 go test ./internal/next/acceptor/ \
  -run TestContract_ProviderAcceptAgent_Codex -v -count=1 -timeout 120s

GROMIT_LLM_CONTRACT=1 go test ./internal/next/specloop/ \
  -run TestContract_ProviderTaskRunner_Codex -v -count=1 -timeout 120s
```

Or run all at once:
```bash
GROMIT_LLM_CONTRACT=1 go test ./internal/next/... \
  -run 'TestContract.*Codex' -v -count=1 -timeout 300s
```

**Verify**:

1. **All subtests pass** with the same structural checks as Scenario 3 (Claude):
   - PlanAgent: plan has tasks with required fields.
   - ReviewAgent: findings parse correctly.
   - AcceptAgent: criterion results have pass/fail/unclear and rationale.
   - TaskRunner: RunTask and RepairTask return valid statuses.

2. **Prompt compatibility**: If any contract test fails, investigate:
   - Is the Codex output parseable JSON after `ExtractJSON` processing?
   - Does Codex return markdown-fenced JSON, bare JSON, or prose-prefixed JSON?
   - If parsing fails, note which adapter and which output format Codex produced.

3. **Performance**: Note the latency difference between Claude and Codex contract tests:
   ```bash
   # Compare test durations in the verbose output
   ```

**Expected**: All Codex contract tests PASS. This is a hard completion gate for 0002d. If any fail, Codex is not a supported provider for that stage until prompt compatibility is resolved.

**Cleanup**: None needed.

---

### Scenario 9: Routing Config Validation

**Purpose**: Verify that invalid routing configurations are rejected with clear error messages.

**Setup**: No fixtures needed -- this tests policy validation logic.

**Execute and Verify**:

#### 9a: Invalid ratio sum (does not equal 100)

```bash
cat > /tmp/bad-policy-ratio.json << 'POLICY'
{
  "always_run": [
    {"name": "unit-tests", "command": "go test ./...", "type": "test"}
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
    "evaluator": "medium"
  },
  "routing": {
    "preferences": {"plan": "any"},
    "ratio": {"claude": 70, "codex": 20},
    "cooldown_seconds": 300
  }
}
POLICY

cp /tmp/bad-policy-ratio.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json

$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md 2>&1
```

**Verify**:
- CLI exits with non-zero exit code (`echo $?` → non-zero).
- Error message on stderr mentions ratio values must sum to 100 (got 90).
- Run does NOT start — no run directory created:
  ```bash
  ls ~/.local/share/gromit/projects/fixture-calc/runs/ | tail -1
  # Should NOT show a new run-id created after the failed command
  ```
- No `events.jsonl` created for this attempt.

#### 9b: Valid routing config

```bash
cat > /tmp/good-policy-routing.json << 'POLICY'
{
  "always_run": [
    {"name": "unit-tests", "command": "go test ./...", "type": "test"}
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
    "evaluator": "medium"
  },
  "routing": {
    "preferences": {"plan": "claude", "execute": "any"},
    "ratio": {"claude": 70, "codex": 30},
    "cooldown_seconds": 300
  }
}
POLICY

cp /tmp/good-policy-routing.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

**Verify**: Dry-run succeeds without validation errors:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md --dry-run
```
- No policy validation errors.
- Plan is created successfully.

#### 9c: Unit test verification of routing validation

```bash
cd /Users/dabrams/gromit
go test ./internal/next/execpolicy/ -run TestPolicy_Validate_Routing -v -count=1
```

**Verify**:
- `TestPolicy_Validate_RoutingRatioSumsTo100` -- rejects ratio sum != 100.
- `TestPolicy_Validate_RoutingRatioValid` -- accepts ratio sum == 100.

**Expected**: Invalid ratio sums rejected. Valid configs accepted. Unknown provider names deferred to router construction time.

**Cleanup**: Restore standard policy.

---

### Scenario 10: Single-Provider Mode (0002d)

**Purpose**: Verify the system works correctly with only Claude (no Codex binary, no codex in routing config), producing no nil-pointer errors or fallback failures.

**Setup**:

Ensure Codex is NOT used:

```bash
# Install a Claude-only routing config
cat > ~/.local/share/gromit/projects/fixture-calc/policy/execution.json << 'POLICY'
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
    "evaluator": "medium"
  },
  "routing": {
    "preferences": {"plan": "claude", "execute": "claude", "review": "claude", "accept": "claude"},
    "ratio": {"claude": 100},
    "cooldown_seconds": 300
  }
}
POLICY
```

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md 2>&1 | tee /tmp/single-provider-output.txt
```

**Verify**:

1. **No panics or nil-pointer errors**: Check output:
   ```bash
   grep -i 'panic\|nil pointer\|segfault' /tmp/single-provider-output.txt
   ```
   Expected: no matches.

2. **Run completes normally**: Terminal state is `ready_for_review` or `completed`.

3. **All invocations use Claude**: No Codex invocations:
   ```bash
   RUN_ID=<from output>
   jq '.invocations[] | .provider' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json | sort -u
   ```
   Expected: only `"claude"`.

4. **FallbackAdapter handles nil codex gracefully**: The Router was constructed with only Claude in the providers map. No errors from missing Codex provider.

5. **Unit test verification**:
   ```bash
   cd /Users/dabrams/gromit
   go test ./cmd/gromit-next/ -run TestBuildStages_NilCodexProvider -v -count=1
   ```
   Expected: PASS -- single-provider mode works.

**Expected**: Full pipeline completes with Claude-only routing. No nil pointer or fallback errors. FallbackAdapter degrades gracefully when only one provider is available.

**Cleanup**: None needed.

---

### Scenario 10b: Cost Budget Exceeded Mid-Run

**Purpose**: Verify that when `max_run_cost_usd` is exceeded mid-run, the pipeline halts gracefully via the adapter layer's cost tracking.

**Setup**:
```bash
# Set an extremely low cost budget to trigger mid-run
jq '.budgets.max_run_cost_usd = 0.01' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md 2>&1 | tee /tmp/cost-exceeded-output.txt
```

**Verify**:

1. **Run terminates before completing all stages**:
   ```bash
   RUN_ID=<from output>
   jq '{status, terminal_state}' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/run.json
   ```
   - Terminal state: `blocked` (budget exceeded).

2. **Budget exceeded event recorded**:
   ```bash
   grep 'budget_exceeded' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | \
     jq '{event_type, budget, accumulated_cost_usd}'
   ```
   - `budget` field: `"cost"`.
   - `accumulated_cost_usd` > 0.01 (exceeded the limit).

3. **Cost was tracked through adapter OnCost callbacks**:
   ```bash
   jq '.total_cost_usd' ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/evidence/metrics.json
   ```
   - `total_cost_usd` is positive and near or above the budget limit.

4. **No panic or crash**: CLI output does not contain stack traces.

**Expected**: Pipeline halts gracefully when cost budget exceeded. Cost tracked through adapter layer's OnCost callbacks feeds budget enforcement.

**Cleanup**:
```bash
jq '.budgets.max_run_cost_usd = 50.0' \
  ~/.local/share/gromit/projects/fixture-calc/policy/execution.json > /tmp/ep.json \
  && mv /tmp/ep.json ~/.local/share/gromit/projects/fixture-calc/policy/execution.json
```

---

### Scenario 11: ExtractJSON Robustness

**Purpose**: Verify that `ExtractJSON` correctly handles various LLM output formats (bare JSON, markdown fenced, prose-prefixed, nested objects).

**Setup**: This is primarily a unit test scenario.

**Execute**:
```bash
cd /Users/dabrams/gromit
go test ./internal/next/llmadapter/ -run TestExtractJSON -v -count=1
```

**Verify**:

1. **Bare JSON**: `ExtractJSON('{"key": "value"}')` returns `'{"key": "value"}'`.
2. **Markdown fenced**:
   ```
   ExtractJSON('Here is the result:\n```json\n{"key": "value"}\n```\n')
   ```
   Returns `'{"key": "value"}'`.
3. **Prose-prefixed**: `ExtractJSON('The findings are: {"key": "value"}')` returns `'{"key": "value"}'`.
4. **Array**: `ExtractJSON('[{"a":1},{"b":2}]')` returns `'[{"a":1},{"b":2}]'`.
5. **No JSON**: `ExtractJSON('no json here')` returns `""`.
6. **Nested objects**: `ExtractJSON('{"outer":{"inner":"val"}}')` returns the full nested object.
7. **Multiple JSON objects**: `ExtractJSON('{"first":1} and {"second":2}')` returns `'{"first":1}'` (first match).

All test cases must pass.

**Additionally, verify integration with real LLM output**:
```bash
# Run a single contract test that exercises ExtractJSON through a real adapter:
GROMIT_LLM_CONTRACT=1 go test ./internal/next/review/ \
  -run TestContract_ProviderReviewAgent_Claude -v -count=1 -timeout 120s
```

The review contract test invokes a real LLM and parses findings via `ExtractJSON` + `json.Unmarshal`. If it passes, `ExtractJSON` handles real Claude output.

**Expected**: All `ExtractJSON` unit tests pass. Contract tests that depend on `ExtractJSON` also pass.

**Cleanup**: None needed.

---

### Scenario 12: Review and Acceptance with Real LLM

**Purpose**: Verify that review and acceptance stages produce correctly parsed, substantive output when backed by real LLM adapters.

**Setup**: Use the artifacts from Scenario 1, or run a fresh execution.

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
```

**Verify**:

1. **Discover evidence files**: First, list the evidence directory to find actual file names:
   ```bash
   RUN_ID=<from output>
   RUN_DIR=~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID
   ls $RUN_DIR/evidence/
   ```
   Identify the review findings file(s) and acceptance results file(s) from the listing. The exact names may vary (e.g., `review.md`, `review-findings.json`, `acceptance.json`, or similar). Use the discovered names in the steps below.

2. **Review findings parse correctly**: Inspect the review output file identified above:
   ```bash
   # Use the review file discovered in step 1, e.g.:
   cat $RUN_DIR/evidence/<review-file>
   ```
   - Contains structured findings (not raw JSON dump).
   - Findings have `Severity` (one of: critical, high, medium, low, info).
   - Findings have `Description` (non-empty descriptive text).
   - Findings have `File` (file path or code reference).

3. **Review findings JSON is valid**: If a JSON review file exists in the listing:
   ```bash
   # Use the JSON review file discovered in step 1, e.g.:
   cat $RUN_DIR/evidence/<review-json-file> | jq .
   ```
   - Each finding has required fields.
   - Severity values are valid enum values.

4. **Acceptance criterion results parse correctly**: Inspect the acceptance file identified above:
   ```bash
   # Use the acceptance file discovered in step 1, e.g.:
   cat $RUN_DIR/evidence/<acceptance-file> | jq .
   ```
   - Each criterion result has:
     - `pass` -- boolean (true/false) or string ("pass"/"fail"/"unclear")
     - `rationale` -- non-empty string explaining the judgment
     - `criterion` -- references the acceptance criterion being evaluated
   - No criterion result has empty rationale.

5. **Review-triggered replan check**: If any review finding has severity `critical`:
   ```bash
   grep 'replan_triggered' $RUN_DIR/events.jsonl
   ```
   - If critical findings exist, verify whether replan was triggered.
   - If no critical findings, verify the run proceeded to acceptance.

6. **Acceptance-triggered replan check**: If any acceptance criterion failed:
   ```bash
   grep 'acceptance_failed' $RUN_DIR/events.jsonl
   ```
   - If criteria failed, verify the run transitions appropriately (replan or needs_human).

7. **Review and acceptance used different prompts**: Verify the review and acceptance stages sent distinct prompts (not the same prompt):
   - Review prompt should reference code changes, facets, and review criteria.
   - Acceptance prompt should reference specific acceptance criteria from the spec.

**Expected**: Review produces structured findings with valid severity levels. Acceptance produces criterion results with rationale. Both parse correctly through `ExtractJSON` and domain-specific unmarshalling.

**Cleanup**: None needed.

---

### Scenario 12b: Adapter Parse Error Recovery

**Purpose**: Verify that when an LLM returns output that `ExtractJSON` cannot parse into the expected domain type, the pipeline handles it gracefully (retry, fail task, or mark blocked — not crash).

**Setup**: Use the standard fixture-calc setup. This scenario is best observed opportunistically — it occurs when an LLM returns malformed output. To increase the likelihood, use a vague or minimal spec:

```bash
cat > /tmp/gromit-fixtures/fixture-calc/specs/vague-spec.md << 'SPEC'
# Vague Spec
Do something interesting with the calculator.
SPEC
cd /tmp/gromit-fixtures/fixture-calc && git add specs/ && git commit -m "add vague spec" --allow-empty
```

**Execute**:
```bash
$GROMIT exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/vague-spec.md 2>&1 | tee /tmp/parse-error-output.txt
```

**Verify**:

1. **No panic or crash**: Check output:
   ```bash
   grep -i 'panic\|nil pointer\|segfault' /tmp/parse-error-output.txt
   ```
   Expected: no matches.

2. **If parse error occurred**: Check events for retry or failure:
   ```bash
   RUN_ID=<from output>
   grep -E 'parse_error|retry|failed' \
     ~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID/events.jsonl | \
     jq '{event_type, phase, error}'
   ```
   - Parse errors should result in retry (up to configured limit) or task failure — not an unhandled crash.

3. **If no parse error occurred** (LLM returned valid output): The scenario passes vacuously. Note this in results and rely on the unit tests (`TestProviderReviewAgent_ReviewFacet_InvalidJSON_ReturnsParseError`, etc.) for parse error coverage.

4. **Alternative — force parse error via unit test**:
   ```bash
   cd /Users/dabrams/gromit
   go test ./internal/next/review/ -run TestProviderReviewAgent_ReviewFacet_InvalidJSON -v -count=1
   go test ./internal/next/acceptor/ -run TestProviderAcceptAgent_EvaluateCriterion_InvalidJSON -v -count=1
   ```
   Both must PASS, confirming parse errors are returned (not panicked on).

**Expected**: Parse errors from adapter layer are handled gracefully — retried or failed with a descriptive error, never crashing. Unit tests provide deterministic coverage; end-to-end is opportunistic.

**Cleanup**: None needed.

---

## 3. Artifact Verification Checklist

Use this after any scenario to systematically verify 0002c/0002d-specific artifacts.

### 3.1 Adapter Layer Artifacts

| Artifact | Check |
|----------|-------|
| `metrics.json` invocations | Each LLM invocation has `provider`, `model`, `phase`, `cost_usd`, `tokens_in`, `tokens_out` |
| `metrics.json` total_cost_usd | > 0 for any run with LLM stages |
| `metrics.json` provider field | Present on every invocation; matches configured provider for that phase |
| Review output | Substantive text, not noop placeholder |
| Acceptance output | Parseable criterion results, not noop placeholder |
| Plan output | Structured tasks from real LLM, not empty/mock plan |

### 3.2 Provider Routing Artifacts (0002d)

| Artifact | Check |
|----------|-------|
| `metrics.json` provider field | Matches routing preferences per phase |
| `execution-policy.json` routing section | Contains `preferences`, `ratio`, `cooldown_seconds` |
| Mixed-provider invocations | When routing to multiple providers, invocations show different `provider` values |
| Fallback log messages | When fallback occurs, log contains `"provider X hit usage limit, attempting fallback"` |

### 3.3 Cost Tracking Verification

```bash
# For any completed run:
RUN_DIR=~/.local/share/gromit/projects/fixture-calc/runs/$RUN_ID

# 1. Total cost is positive
jq '.total_cost_usd > 0' $RUN_DIR/evidence/metrics.json
# Expected: true

# 2. Sum of invocations matches total
jq '(.total_cost_usd - ([.invocations[].cost_usd] | add)) | fabs < 0.001' $RUN_DIR/evidence/metrics.json
# Expected: true (within floating-point tolerance)

# 3. Each LLM phase has at least one invocation
jq '.invocations | map(.phase) | unique' $RUN_DIR/evidence/metrics.json
# Expected: includes "plan", "execute", "review", "accept"

# 4. No invocations for non-LLM phases
jq '[.invocations[] | select(.phase == "compile" or .phase == "validate")] | length' $RUN_DIR/evidence/metrics.json
# Expected: 0
```

### 3.4 Contract Test Results Summary

| Domain | Claude | Codex | ShellValidator |
|--------|--------|-------|----------------|
| PlanAgent | Must PASS | Must PASS (0002d) | N/A |
| ReviewAgent | Must PASS | Must PASS (0002d) | N/A |
| AcceptAgent | Must PASS | Must PASS (0002d) | N/A |
| TaskRunner | Must PASS | Must PASS (0002d) | N/A |
| ShellValidator | N/A | N/A | Must PASS |

### 3.5 Events Spot-Checks (0002c/0002d additions)

```bash
# For a successful run with real adapters:
cat $RUN_DIR/events.jsonl | jq -r .event_type | sort | uniq -c | sort -rn
```

New event types to verify (compared to noop 0002a):
- `review_completed` -- review stage completed with real findings
- `acceptance_completed` -- acceptance stage completed with real criterion results
- Provider-related metadata on existing events (e.g., `plan_created` may now include `provider` field)

---

## 4. Fixture Setup

### 4.1 Reuse 0002a Fixtures

The fixture repos (`fixture-calc`, `fixture-greeter`, `fixture-multipackage`) and spec files from the 0002a manual test plan should be reused. See `docs/plans/2026-03-11-spec-0002a-manual-test-plan.md`, Section 4, for creation instructions.

### 4.2 Execution Policy with Routing Config (0002d)

Save to `/tmp/gromit-fixtures/policies/fixture-calc-execution-routing.json`:

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
    "executor": "medium",
    "evaluator": "medium"
  },
  "routing": {
    "preferences": {
      "plan": "claude",
      "execute": "any",
      "review": "claude",
      "accept": "claude"
    },
    "ratio": {"claude": 70, "codex": 30},
    "cooldown_seconds": 300
  }
}
```

### 4.3 Execution Policy without Routing (0002c only)

Save to `/tmp/gromit-fixtures/policies/fixture-calc-execution-adapters.json`:

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
    "executor": "medium",
    "evaluator": "medium"
  }
}
```

### 4.4 Policy Installation Script

```bash
#!/bin/bash
# Install policies for 0002c/0002d testing

POLICY_DIR=/tmp/gromit-fixtures/policies
PROJECTS_DIR=~/.local/share/gromit/projects

# 0002c testing (Claude-only, no routing):
mkdir -p $PROJECTS_DIR/fixture-calc/policy
cp $POLICY_DIR/fixture-calc-execution-adapters.json $PROJECTS_DIR/fixture-calc/policy/execution.json

# 0002d testing (multi-provider routing):
# cp $POLICY_DIR/fixture-calc-execution-routing.json $PROJECTS_DIR/fixture-calc/policy/execution.json
```

---

## 5. Regression Checks

### 5.1 Smoke Test Sequence

After any code change to 0002c/0002d packages, run this minimal sequence:

```bash
# 1. Build
cd /Users/dabrams/gromit
go build -o gromit-next ./cmd/gromit-next/

# 2. Unit tests for all 0002c/0002d packages
go test ./internal/next/llmadapter/... -v -count=1
go test ./internal/next/planner/... -v -count=1
go test ./internal/next/review/... -v -count=1
go test ./internal/next/acceptor/... -v -count=1
go test ./internal/next/specloop/... -v -count=1
go test ./internal/next/validator/... -v -count=1
go test ./internal/next/contextpkt/... -v -count=1
go test ./internal/next/execpolicy/... -v -count=1
go test ./cmd/gromit-next/ -v -count=1

# 3. Contract tests against Claude
GROMIT_LLM_CONTRACT=1 go test ./internal/next/... -run 'TestContract.*Claude' -v -count=1 -timeout 300s

# 4. Quick happy-path end-to-end
./gromit-next exec spec --project fixture-calc \
  --spec /tmp/gromit-fixtures/fixture-calc/specs/add-subtract.md
# Confirm: terminal state is ready_for_review, cost > 0

# 5. Verify routing (0002d only)
# Install routing policy, re-run, check provider fields in metrics
```

### 5.2 What to Re-test After Specific Changes

| Changed Package | Re-run Scenarios |
|----------------|-----------------|
| `llmadapter` | 1, 2, 4, 5, 6, 11 (core adapter, cost, timeout, fallback, ExtractJSON) |
| `planner` (ProviderPlanAgent) | 1, 2, 3 (plan creation, contract tests) |
| `review` (ProviderReviewAgent) | 1, 3, 12 (review findings, contract tests) |
| `acceptor` (ProviderAcceptAgent) | 1, 3, 12 (acceptance, contract tests) |
| `specloop` (ProviderTaskRunner) | 1, 2, 3 (task execution, contract tests) |
| `validator` (ShellValidator) | 1, 2 (validation delegation) |
| `contextpkt` (SpecCompilerAdapter) | 1, 2 (compilation) |
| `execpolicy` (RoutingConfig) | 7, 9, 10 (routing, validation) |
| `cmd/gromit-next` (RealStageProvider wiring) | 1, 2, 7, 10 (all wiring scenarios) |
| `llmadapter/fallback.go` (FallbackAdapter) | 6, 7, 10 (fallback, routing, single-provider) |

### 5.3 Pre-merge Checklist

Before merging Spec 0002c implementation:

- [ ] All unit tests pass: `go test ./internal/next/... ./cmd/gromit-next/`
- [ ] Scenario 1 (Happy Path) passes with real Claude -- cost > 0, substantive output
- [ ] Scenario 2 (Adapter Wiring) -- all stages use correct adapter types
- [ ] Scenario 3 (Claude Contracts) -- all contract test suites PASS
- [ ] Scenario 4 (Cost Callbacks) -- cost accumulates correctly
- [ ] Scenario 5 (Timeout) -- adapter-level timeout works
- [ ] Scenario 11 (ExtractJSON) -- all unit tests pass
- [ ] Scenario 12 (Review/Accept) -- findings and criteria parse correctly
- [ ] Scenario 10b (Cost Budget) -- pipeline halts when cost exceeded
- [ ] Scenario 12b (Parse Errors) -- unit tests pass; end-to-end no crash
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` produces no output

Before merging Spec 0002d implementation (requires 0002c complete):

- [ ] All 0002c checklist items still pass
- [ ] Scenario 6 (Fallback) -- unit tests pass; end-to-end if usage limit available
- [ ] Scenario 7 (Phase Preferences) -- correct provider selected per phase
- [ ] Scenario 8 (Codex Contracts) -- all contract test suites PASS against Codex
- [ ] Scenario 9 (Config Validation) -- invalid ratios rejected, valid accepted
- [ ] Scenario 10 (Single Provider) -- no nil errors with Claude-only config
- [ ] FallbackAdapter unit tests all pass
- [ ] RoutingConfig validation unit tests all pass
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` produces no output
