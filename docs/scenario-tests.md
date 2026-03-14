# Scenario Tests

Scenario tests verify that a sequence of CLI commands produces correct, consistent output when operating on a known store state. They sit between unit tests (single function, mocked deps) and full end-to-end tests (real Claude invocation, real fixture repo).

Most scenario bugs — wrong field in `exec show`, stale status in evidence files, spec list derivation errors — are **CLI layer bugs**, not agent-behavior bugs. They don't need Claude to reproduce or catch.

---

## The two tiers

| Tier | Needs Claude | Needs fixture repo | Speed | Cost |
|------|-------------|-------------------|-------|------|
| **Scenario (synthetic store)** | No | No | <1s | $0 |
| **E2E (real run)** | Yes | Yes | 2–5 min | $0.10–$1.00 |

Write scenario tests by default. Only write E2E tests for behaviors that genuinely require the agent to act (constraint enforcement, fix cycles, task splitting).

---

## Anatomy of a scenario test

Every scenario test follows the same three-phase structure:

```
Seed → Invoke → Assert
```

### Phase 1: Seed

Create a `runstore.Store` in `t.TempDir()` and populate it with `RunState` objects that represent the preconditions your scenario requires. No CLI invocation needed yet.

```go
func TestScenario_ExecList_ShowsMultipleStatuses(t *testing.T) {
    tmp := t.TempDir()
    store := runstore.NewStore(tmp)

    // Seed: one passing run, one blocked run
    mustSave(t, store, &runstore.RunState{
        RunID:     "run-pass",
        SpecID:    "add-subtract",
        ProjectID: "fixture-calc",
        Status:    runstore.StatusReadyForReview,
        StartedAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
        Tasks:     []runstore.Task{{Status: "done"}},
    })
    mustSave(t, store, &runstore.RunState{
        RunID:     "run-blocked",
        SpecID:    "add-subtract",
        ProjectID: "fixture-calc",
        Status:    runstore.StatusBlocked,
        StartedAt: time.Date(2026, 3, 1, 13, 0, 0, 0, time.UTC),
        Tasks:     []runstore.Task{{Status: "pending"}},
    })
```

### Phase 2: Invoke

Call the internal function directly (preferred) or via cobra. Direct calls are simpler and avoid flag-parsing noise.

```go
    // Direct call — preferred
    output, err := execList("fixture-calc", store)
    if err != nil {
        t.Fatalf("execList: %v", err)
    }
```

When you need to test flag parsing or stdout routing, use the cobra command:

```go
    // Cobra call — use when testing flag behavior
    cmd := newExecListCmd()
    var buf bytes.Buffer
    cmd.SetOut(&buf)
    cmd.SetArgs([]string{"--project", "fixture-calc", "--store-dir", tmp})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("cmd.Execute: %v", err)
    }
    output := buf.String()
```

### Phase 3: Assert

Use `strings.Contains` for presence checks. Avoid asserting on exact whitespace — tabwriter alignment changes with content width.

```go
    // Column headers present
    if !strings.Contains(output, "RUN ID") {
        t.Error("expected RUN ID header")
    }
    // Both statuses appear
    if !strings.Contains(output, "ready_for_review") {
        t.Error("expected ready_for_review in output")
    }
    if !strings.Contains(output, "blocked") {
        t.Error("expected blocked in output")
    }
    // Correct run IDs present
    if !strings.Contains(output, "run-pass") {
        t.Error("expected run-pass in output")
    }
}
```

---

## Helper: mustSave

Define this once per test file to reduce boilerplate:

```go
func mustSave(t *testing.T, store *runstore.Store, rs *runstore.RunState) {
    t.Helper()
    if err := store.Save(rs); err != nil {
        t.Fatalf("save %s: %v", rs.RunID, err)
    }
}
```

---

## Seeding evidence files

For `exec show --full` tests, create evidence files directly in the store's evidence directory. No bundler needed — the CLI just reads whatever files are there.

```go
func seedEvidence(t *testing.T, store *runstore.Store, runID string, files map[string]string) {
    t.Helper()
    dir := store.RunEvidenceDir(runID)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        t.Fatalf("mkdir evidence: %v", err)
    }
    for name, content := range files {
        path := filepath.Join(dir, name)
        if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
            t.Fatalf("write %s: %v", name, err)
        }
    }
}

// Usage:
seedEvidence(t, store, "run-pass", map[string]string{
    "summary.md":    "# Execution Summary\n\n- **Status:** ready_for_review\n",
    "review.md":     "# Review Decision Sheet\n\n## Terminal State\n\nready_for_review\n",
    "metrics.json":  `{"total_cost_usd": 0.21, "cycles": 2}`,
    "validation.json": `{"pass": true}`,
})
```

---

## Asserting JSON evidence files

Parse evidence JSON directly when checking structured fields. Don't grep for substrings in JSON — field order is not guaranteed.

```go
func TestScenario_ExecShow_Full_MetricsPopulated(t *testing.T) {
    // ... seed setup ...

    output, err := execShow("run-pass", store, true /* full */)
    if err != nil {
        t.Fatalf("execShow: %v", err)
    }

    // Extract the metrics.json block from the output and parse it
    start := strings.Index(output, "=== metrics.json ===")
    end := strings.Index(output[start:], "\n===")
    block := output[start+len("=== metrics.json ===\n") : start+end]

    var m struct {
        TotalCostUSD float64 `json:"total_cost_usd"`
        Cycles       int     `json:"cycles"`
    }
    if err := json.Unmarshal([]byte(strings.TrimSpace(block)), &m); err != nil {
        t.Fatalf("parse metrics.json: %v", err)
    }
    if m.TotalCostUSD == 0 {
        t.Error("expected non-zero cost")
    }
    if m.Cycles != 2 {
        t.Errorf("expected 2 cycles, got %d", m.Cycles)
    }
}
```

For simpler checks, `strings.Contains` on the full `--full` output is fine:

```go
if !strings.Contains(output, `"pass": true`) {
    t.Error("expected validation pass in output")
}
```

---

## Testing status derivation

`spec list` derives status from run history. Test every derivation path:

| Run history | Expected spec status |
|-------------|---------------------|
| No runs | `ready` |
| Latest run: `ready_for_review` | `ready_for_review` |
| Latest run: `needs_human` | `needs_attention` |
| Latest run: `blocked` | `needs_attention` |
| Latest run: `running` | `running` |

```go
func TestScenario_SpecList_StatusDerivation(t *testing.T) {
    cases := []struct {
        name       string
        runStatus  string
        wantStatus string
    }{
        {"ready_for_review", runstore.StatusReadyForReview, "ready_for_review"},
        {"needs_human",      runstore.StatusNeedsHuman,     "needs_attention"},
        {"blocked",          runstore.StatusBlocked,        "needs_attention"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            tmp := t.TempDir()
            store := runstore.NewStore(tmp)
            mustSave(t, store, &runstore.RunState{
                RunID:     "run-x",
                SpecID:    "my-spec",
                ProjectID: "proj",
                Status:    tc.runStatus,
                StartedAt: time.Now(),
                Tasks:     []runstore.Task{},
            })
            out, err := execSpecList("proj", store, "/path/to/specs")
            if err != nil {
                t.Fatalf("execSpecList: %v", err)
            }
            if !strings.Contains(out, tc.wantStatus) {
                t.Errorf("want %q in output, got:\n%s", tc.wantStatus, out)
            }
        })
    }
}
```

---

## The stage-ordering trap

Some bugs only appear when stage A writes output that depends on state set by stage B, but A runs before B. The `effectiveStatus` bug is the canonical example: `EvidenceStage` ran before `FinalizeStage`, so `rs.Status` was still `"running"` when `summary.md` and `review.md` were written.

This class of bug cannot be caught by unit-testing `EvidenceStage` in isolation because you'd naturally pass a terminal `rs.Status`. It requires a test that simulates the pre-finalize RunState:

```go
func TestScenario_EvidenceStage_StatusBeforeFinalize(t *testing.T) {
    tmp := t.TempDir()
    store := runstore.NewStore(tmp)

    // Simulate the state at the moment EvidenceStage runs:
    // status is still "running" because FinalizeStage hasn't fired yet,
    // but all the gating fields are set to passing.
    rs := runstore.NewRunState("spec-001", "proj")
    rs.Status = runstore.StatusRunning  // <-- key: not yet finalized
    rs.FinalValidationPassed = true
    rs.FinalReviewPassed = true
    rs.FinalAcceptancePassed = true
    rs.Tasks = []runstore.Task{{Status: "done"}, {Status: "done"}}
    mustSave(t, store, rs)

    stage := stages.NewEvidenceStage(store, stages.EvidenceStageConfig{
        StartTime: time.Now().Add(-30 * time.Second),
    })
    _, err := stage.Run(context.Background(), rs)
    if err != nil {
        t.Fatalf("EvidenceStage.Run: %v", err)
    }

    // summary.md must show the correct terminal state, not "running"
    summaryPath := filepath.Join(store.RunEvidenceDir(rs.RunID), "summary.md")
    data, err := os.ReadFile(summaryPath)
    if err != nil {
        t.Fatalf("read summary.md: %v", err)
    }
    if strings.Contains(string(data), "running") {
        t.Error("summary.md shows 'running' — effectiveStatus not applied")
    }
    if !strings.Contains(string(data), "ready_for_review") {
        t.Errorf("summary.md should show ready_for_review, got:\n%s", data)
    }
}
```

**Rule:** whenever a stage writes files that reference `rs.Status`, write a test that passes `rs.Status = "running"` with passing gate fields, and assert the output reflects the derived terminal state, not `"running"`.

---

## Multi-command consistency tests

The most valuable scenario tests verify that multiple commands agree on the same data. If `exec show` says cost is `$0.21` but `exec show --full` shows `metrics.json.total_cost_usd: 0`, something is wrong.

```go
func TestScenario_ShowAndFullAgree(t *testing.T) {
    tmp := t.TempDir()
    store := runstore.NewStore(tmp)

    mustSave(t, store, &runstore.RunState{
        RunID:           "run-x",
        SpecID:          "spec-001",
        ProjectID:       "proj",
        Status:          runstore.StatusReadyForReview,
        AccumulatedCost: 0.42,
        StartedAt:       time.Now().Add(-2 * time.Minute),
        EndedAt:         time.Now(),
        Tasks:           []runstore.Task{{Status: "done"}},
    })
    seedEvidence(t, store, "run-x", map[string]string{
        "metrics.json": `{"total_cost_usd": 0.42}`,
    })

    brief, _ := execShow("run-x", store, false)
    full, _  := execShow("run-x", store, true)

    // Both show the same cost
    if !strings.Contains(brief, "0.4200") {
        t.Errorf("brief output missing cost: %s", brief)
    }
    if !strings.Contains(full, "0.42") {
        t.Errorf("full output missing cost in metrics.json: %s", full)
    }

    // Status agrees between summary line and evidence file
    if !strings.Contains(brief, "ready_for_review") {
        t.Errorf("brief status wrong: %s", brief)
    }
    if strings.Contains(full, "Status: running") {
        t.Errorf("full output shows stale 'running' status: %s", full)
    }
}
```

---

## What not to test here

- **Agent behavior**: Whether Claude writes correct code, respects constraints, or produces valid proof checks. Use E2E tests with real runs for this.
- **Store persistence correctness**: `Save`/`Get` round-trips are unit-tested in the `runstore` package.
- **Stage internals beyond their output files**: Unit-test stages directly with mocked deps. Scenario tests verify what users see, not how stages work internally.
- **Exact whitespace or column alignment**: Tabwriter output shifts with content. Assert on substrings, not full output equality.

---

## File placement

| File | Purpose |
|------|---------|
| `cmd/gromit-next/exec_test.go` | Scenario tests for `exec list`, `exec show`, `exec show --full` |
| `cmd/gromit-next/spec_test.go` | Scenario tests for `spec list` |
| `internal/next/specloop/stages/evidence_test.go` | Stage-ordering tests (effectiveStatus, pre-finalize status) |

Scenario tests live in the same package as the code under test (`package main` for cmd, `package stages` for stage tests). No separate `scenario_test.go` file is needed until a package has more than ~5 scenario tests.

---

## Running

```bash
# All scenario tests (fast, no Claude needed)
go test ./cmd/gromit-next/ ./internal/next/specloop/stages/ -count=1

# Just scenario tests by name convention
go test ./... -run TestScenario -count=1

# Full suite
go test ./... -count=1
```

No build tags, no environment variables required. Scenario tests must always be runnable with a plain `go test`.
