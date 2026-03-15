# E2E Contract Testing

E2E contract tests verify that a full gromit-next run — real Claude invocations against a real fixture repo — produces correct, consistent outcomes. They test behaviors that synthetic store tests cannot: actual agent decision-making, budget enforcement under real API cost, constraint adherence, and CLI output correctness after a live run.

---

## The two tiers

| Tier | Needs Claude | Speed | Cost | Where |
|------|-------------|-------|------|-------|
| **Scenario (synthetic store)** | No | <1s | $0 | `cmd/gromit-next/*_test.go`, `internal/next/specloop/stages/*_test.go` |
| **E2E contract** | Yes | 1–5 min | $0.05–$1.00 | `contracts/*.yaml` + `e2e/` |

Write synthetic scenario tests by default (see `docs/scenario-tests.md`). Write e2e contracts only for behaviors that require the agent to act:
- Did the agent respect a constraint under real conditions?
- Did the budget gate fire at the right moment with a real cost?
- Did all evidence files get written correctly after a real run?

---

## File layout

```
contracts/
  scenario-01-happy-path.yaml
  scenario-02-unfixable-spec.yaml
  ...
e2e/
  contract.go           # Contract + Assertion type definitions (no build tag)
  runner.go             # Harness functions (//go:build e2e)
  harness_test.go       # TestScenarioContracts + individual TestE2E_* (//go:build e2e)
  testdata/
    divide_test_int_assert.go   # Fixture file copied in by Scenario 2
```

---

## Running

```bash
# All contracts (serially, cost-controlled)
GROMIT_E2E=1 go test ./e2e/ -tags e2e -count=1 -timeout 30m

# Single scenario by name
GROMIT_E2E=1 go test ./e2e/ -tags e2e -count=1 -timeout 30m -run TestScenarioContracts/Scenario01

# Individual named test function
GROMIT_E2E=1 go test ./e2e/ -tags e2e -count=1 -timeout 30m -run TestE2E_Scenario01_HappyPath

# Without GROMIT_E2E=1, all e2e tests skip automatically
go test ./e2e/ -tags e2e   # → skip: set GROMIT_E2E=1 to run e2e tests
```

Tests run serially by default to control cost. Set `concurrent: true` in the contract to opt into `t.Parallel()`.

---

## Contract schema

Every contract is a YAML file in `contracts/`. Field reference:

```yaml
name: "Scenario N — Short description"  # Human-readable name (required)
scenario: N                              # Numeric scenario ID (required)
spec: specs/add-subtract.md             # Spec path relative to fixture dir (required)
fixture: fixture-calc                   # Fixture dir name under fixtureBase (required)
policy: policies/fixture-calc-execution.json  # Policy path relative to fixtureBase (optional)
store_dir: .gromit-next                 # Store dir relative to fixture dir (default: .gromit-next)
extra_flags: []                         # Extra CLI flags appended to exec spec (optional)
concurrent: false                       # Set true to run in parallel (optional)
depends_on_scenario: 0                  # If set, harness ensures this scenario ran first (optional)

inline_policy: |                        # Inline JSON policy — takes precedence over policy field
  { "budgets": { "max_run_cost_usd": 0.001 } }

fixture_reset:                          # State to restore before running (optional sections)
  git_files:
    - commit: "7f6de76"
      files: [calc/calc.go, calc/calc_test.go]
  remove_files:
    - calc/divide_test.go
  add_files:
    - src: e2e/testdata/divide_test_int_assert.go  # relative to gromit repo root
      dst: calc/divide_test.go                     # relative to fixture dir

assertions:
  - status: ready_for_review
  # ... see assertion reference below
```

### Paths

| Field | Resolved relative to |
|-------|---------------------|
| `spec` | `fixtureDir` (e.g. `/tmp/gromit-fixtures/fixture-calc/`) |
| `policy` | `fixtureBase` (e.g. `/tmp/gromit-fixtures/`) |
| `add_files[].src` | Gromit repo root (`/Users/dabrams/gromit/`) |
| `add_files[].dst` | `fixtureDir` |
| `file_contains.path` | `fixtureDir` (or absolute if starting with `/`) |
| `file_not_modified` | `fixtureDir` |

---

## Assertion reference

Assertions are a list of single-key maps. Each entry checks one thing.

### Run state

```yaml
# Terminal status
- status: ready_for_review          # Exact match on rs.Status
- status_one_of: [needs_human]      # rs.Status must be one of these values

# Terminal reason (set when status is blocked or needs_human)
- terminal_reason: budget_exceeded
- terminal_reason: cycles_exhausted

# Boolean run state fields
- final_validation_passed: true
- ended_at_set: true                # rs.EndedAt must be non-zero

# Numeric assertions
- cost_usd_gt: 0                    # rs.AccumulatedCost > this value
- replans_gte: 1                    # rs.TotalReplans >= this value
- replans_eq: 0                     # rs.TotalReplans == this value
- cycle_eq: 1                       # rs.Cycle == this value
```

### Evidence files

These assertions parse JSON evidence files written to `<store>/runs/<run-id>/evidence/`.

```yaml
# acceptance.json — all_pass field
- acceptance_all_pass: true

# validation.json — pass field
- validation_pass: true

# review.json — no findings with severity "error" or "critical"
- no_error_severity_findings: true

# metrics.json — invocations array length
- invocations_count_gte: 1
```

### Task assertions

```yaml
# Every task in rs.Tasks has attempts > 0
- all_tasks_attempted: true

# At least one task has non-empty files_changed
- files_changed_nonempty: true

# No task ever changed a file matching this substring
- files_changed_never_contains: calc/divide_test.go

# At least one task changed a file matching this substring
- any_task_files_changed_contains: calc/calc.go
```

### Filesystem assertions

```yaml
# File contains a substring
- file_contains:
    path: calc/calc.go              # relative to fixture dir
    pattern: "func Subtract"

# File matches its git HEAD version (not modified by the run)
- file_not_modified: calc/divide_test.go
```

### Event assertions

These scan `<store>/runs/<run-id>/events.jsonl` line-by-line.

```yaml
# At least one event with this "type" field exists
- events_contain_type: task_validation_result
- events_contain_type: budget_exceeded
```

Common event types: `task_started`, `task_completed`, `task_validation_result`, `task_needs_split`, `redecomposition_triggered`, `budget_exceeded`.

### CLI assertions

These invoke the actual binary against the completed run's store.

```yaml
# exec show <run-id> --store-dir <storeDir>
- exec_show_contains: "Cycles:"
- exec_show_not_contains: "Cost:    $0.0000"

# exec show <run-id> --full --store-dir <storeDir>
- exec_show_full_contains: "ready_for_review"
- exec_show_full_not_contains: "running"

# exec list --project <fixture> --store-dir <storeDir>
- exec_list_contains: ready_for_review

# spec list --project <fixture> --store-dir <storeDir> --specs-dir <fixtureDir>/specs
- spec_list_contains: ready_for_review
```

---

## Fixture reset patterns

### Restoring files from git

Use `git_files` to reset source files to a known commit. The harness runs `git checkout <commit> -- <file>` in the fixture dir.

```yaml
fixture_reset:
  git_files:
    - commit: "7f6de76"             # Commit where calc.go had only Add()
      files: [calc/calc.go, calc/calc_test.go]
```

Find the right commit with `git log --oneline` in the fixture repo.

### Removing files

Use `remove_files` to clean up files that shouldn't exist for this scenario.

```yaml
fixture_reset:
  remove_files:
    - calc/divide_test.go
    - calc/divide_edge_test.go
```

Silently ignores files that don't exist.

### Adding testdata files

Use `add_files` to inject a file from the gromit repo into the fixture. Source paths are relative to the gromit repo root.

```yaml
fixture_reset:
  add_files:
    - src: e2e/testdata/divide_test_int_assert.go
      dst: calc/divide_test.go
```

Store testdata files that aren't part of any fixture repo in `e2e/testdata/`. These are checked into the gromit repo.

### No reset needed

For scenarios that depend on a prior run's state:

```yaml
fixture_reset:
  git_files: []
  remove_files: []
  add_files: []
```

---

## Inline policy

When a scenario needs a non-standard policy (budget limits, timeouts), use `inline_policy` instead of a policy file. This avoids creating one-off policy files that accumulate in `fixtureBase/policies/`.

```yaml
inline_policy: |
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
```

The harness writes this to a temp file and passes it as `--policy`. If both `inline_policy` and `policy` are set, `inline_policy` wins.

---

## Deterministic vs behavioral contracts

### Deterministic (preferred for nightly)

These run reliably every time because they use budget/timeout tricks to stop the agent early.

| Scenario | Trigger | Terminal state |
|----------|---------|----------------|
| 3 — Budget Exhaustion | `max_spec_cycles: 1` | `needs_human` (cycles_exhausted) |
| 5 — Dry Run | `--dry-run` flag | no finalize, ended_at not set |
| 9 — Cost Limit | `max_run_cost_usd: 0.001` | `blocked` (budget_exceeded) |
| 10 — Timeout | `max_run_duration_seconds: 5` | `blocked` (budget_exceeded) |

These are cheap and fast. Assert on exact terminal reasons and structural properties.

### Behavioral (nightly or pre-release)

These let the agent run freely and assert on outcomes. They can have run-to-run cost variation of ±50%.

| Scenario | What it validates |
|----------|------------------|
| 1 — Happy Path | Agent implements spec, passes all checks |
| 2 — Unfixable Spec | Agent respects constraints, exhausts cycles |
| 4 — Unfixable Conflict | Same, with contradictory review requirements |
| 6 — Task Repair | `ShellTaskInspector` fires, repair loop works |
| 7 — Task Split | `task_needs_split` + redecomposition fires |
| 8 — Multi-Project | Two simultaneous runs don't cross-contaminate |
| 11 — CLI Inspection | All CLI fields present and correct after a run |

**Avoid asserting on non-deterministic counts for behavioral scenarios.** `replans_gte: 1` is safe. `replans_eq: 3` is not — the agent may take different paths.

---

## Adding a new contract

1. Create `contracts/scenario-NN-short-name.yaml` following the schema above.

2. Add a fixture reset that leaves the repo in the right precondition.

3. Write assertions in order: run state → evidence → tasks → filesystem → events → CLI. Start with the most important (terminal status, key constraint).

4. Add an individual test function to `e2e/harness_test.go`:
   ```go
   func TestE2E_Scenario12_BroadRefactor(t *testing.T) {
       e2e.SetBinaryPath(e2e.BuildBinary(t))
       e2e.RunNamedContract(t, 12, contractsDir, fixtureBase)
   }
   ```

5. Run it once manually to verify:
   ```bash
   GROMIT_E2E=1 go test ./e2e/ -tags e2e -count=1 -timeout 30m -run TestE2E_Scenario12
   ```

6. If you need a testdata fixture file (like `divide_test_int_assert.go` for Scenario 2), place it in `e2e/testdata/` and reference it with `add_files`.

---

## Scenario dependencies

Use `depends_on_scenario` when a scenario inspects state left by another run rather than invoking the binary itself (e.g., Scenario 11 inspects the result of Scenario 1).

```yaml
depends_on_scenario: 1
```

The harness does not automatically run the dependency — it is expected to be present from a prior test or from running scenarios in order. When running the full suite (`TestScenarioContracts`), contracts are loaded in filename order, so scenario 11 naturally runs after scenario 1.

---

## What not to assert in contracts

- **Exact cost values.** `cost_usd_gt: 0` is correct. `cost_usd_eq: 0.21` will break on model price changes.
- **Exact replan counts for behavioral scenarios.** Use `replans_gte`, not `replans_eq`, unless the scenario is structurally deterministic.
- **Exact whitespace or column alignment in CLI output.** Tabwriter output shifts with content width. Use `exec_show_contains: "Cycles:"`, not full output equality.
- **Evidence file structure beyond the harness assertions.** The harness already parses `acceptance.json`, `validation.json`, `review.json`, `metrics.json`. Don't add raw substring assertions for JSON content — field order is not guaranteed.

---

## e2e/testdata

Files in `e2e/testdata/` are fixture source files checked into the gromit repo. They are injected into fixture repos via `add_files` during fixture reset.

Current files:

| File | Used by | Purpose |
|------|---------|---------|
| `divide_test_int_assert.go` | Scenario 2 | `TestDivide` asserts `result != 3` — unfixable with a float64 return |

When adding a testdata file, use package `calc` (or the appropriate fixture package) and keep it minimal — just enough to create the precondition the scenario needs.
