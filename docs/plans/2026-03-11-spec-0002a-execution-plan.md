# Spec 0002a Core Execution Loop — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the core execution kernel for Gromit Next per Spec 0002a.

**Architecture:** Seven packages under `internal/next/` implementing a stage-pipeline SpecLoop with bounded TaskLoop, deterministic validation gates, and evidence bundle generation. CLI commands under `cmd/gromit-next/`.

**Tech Stack:** Go 1.26, cobra CLI, yaml.v3, existing provider/contextpkt packages.

**Module:** `github.com/danabrams/gromit`

**Existing packages used:**
- `internal/provider` — Provider interface, tier constants (`TierHigh`, `TierMedium`, `TierLow`)
- `internal/next/contextpkt` — Context packet compilation (`Compiler`, `Level`, `CompileOpts`)
- `internal/next/workspace` — `Root`, `EnvResolver` for `~/.local/share/gromit/`
- `internal/next/artifact` — `Store` interface for artifact I/O

---

## Phase 1: Leaf Packages (no `internal/next/` cross-deps)

### Task 1: `execpolicy/` — Core types and defaults

- **Files:**
  - Create: `internal/next/execpolicy/policy.go`
  - Create: `internal/next/execpolicy/policy_test.go`

- **Step 1: Write failing test** in `policy_test.go`:
```go
package execpolicy

import "testing"

func TestDefaultPolicy_HasExpectedBudgets(t *testing.T) {
	p := DefaultPolicy()
	if p.Budgets.MaxSpecCycles != 3 {
		t.Fatalf("want MaxSpecCycles=3, got %d", p.Budgets.MaxSpecCycles)
	}
	if p.Budgets.MaxTaskRetries != 1 {
		t.Fatalf("want MaxTaskRetries=1, got %d", p.Budgets.MaxTaskRetries)
	}
	if p.Budgets.MaxRunDurationSeconds != 3600 {
		t.Fatalf("want MaxRunDurationSeconds=3600, got %d", p.Budgets.MaxRunDurationSeconds)
	}
	if p.Models.Planner != "high" {
		t.Fatalf("want Planner=high, got %s", p.Models.Planner)
	}
}

func TestDefaultPolicy_AlwaysRunChecksNonEmpty(t *testing.T) {
	p := DefaultPolicy()
	if len(p.AlwaysRun) == 0 {
		t.Fatal("default policy must include at least one always-run check")
	}
}
```

- **Step 2:** `go test ./internal/next/execpolicy/ -run TestDefault -v` — expect FAIL (package missing)

- **Step 3: Implement** in `policy.go`:
```go
package execpolicy

type Policy struct {
	AlwaysRun []Check  `json:"always_run"`
	Budgets   Budgets  `json:"budgets"`
	Models    Models   `json:"models"`
}

type Check struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Type    string `json:"type"` // "test" or "lint"
}

type Budgets struct {
	MaxSpecCycles             int     `json:"max_spec_cycles"`
	MaxTaskRetries            int     `json:"max_task_retries"`
	MaxRedecompositionPasses  int     `json:"max_redecomposition_passes"`
	MaxTaskDurationSeconds    int     `json:"max_task_duration_seconds"`
	MaxRunDurationSeconds     int     `json:"max_run_duration_seconds"`
	MaxRunCostUSD             float64 `json:"max_run_cost_usd"`
}

type Models struct {
	Planner  string `json:"planner"`
	Executor string `json:"executor"`
}

func DefaultPolicy() Policy {
	return Policy{
		AlwaysRun: []Check{
			{Name: "unit-tests", Command: "go test ./...", Type: "test"},
			{Name: "format", Command: "gofmt -l .", Type: "lint"},
			{Name: "vet", Command: "go vet ./...", Type: "lint"},
		},
		Budgets: Budgets{
			MaxSpecCycles:            3,
			MaxTaskRetries:           1,
			MaxRedecompositionPasses: 1,
			MaxTaskDurationSeconds:   300,
			MaxRunDurationSeconds:    3600,
			MaxRunCostUSD:            50.0,
		},
		Models: Models{Planner: "high", Executor: "medium"},
	}
}
```

- **Step 4:** `go test ./internal/next/execpolicy/ -v` — expect PASS
- **Step 4a (I5):** Add `NormalizeNilFields()` method to `Policy` type — maps nil `AlwaysRun` slice to empty `[]Check{}`. Per project convention, exported since `Policy` is cross-package.
- **Step 5:** Commit `"feat(next): add execpolicy package with core types and defaults"`

---

### Task 2: `execpolicy/` — Load from JSON file

- **Files:**
  - Modify: `internal/next/execpolicy/policy.go`
  - Modify: `internal/next/execpolicy/policy_test.go`

- **Step 1: Write failing test:**
```go
func TestLoadPolicy_FromJSON(t *testing.T) {
	dir := t.TempDir()
	data := `{"budgets":{"max_spec_cycles":5},"models":{"planner":"xhigh","executor":"high"},"always_run":[]}`
	os.WriteFile(filepath.Join(dir, "execution-policy.json"), []byte(data), 0644)

	p, err := LoadPolicy(filepath.Join(dir, "execution-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Budgets.MaxSpecCycles != 5 {
		t.Fatalf("want 5, got %d", p.Budgets.MaxSpecCycles)
	}
	if p.Models.Planner != "xhigh" {
		t.Fatalf("want xhigh, got %s", p.Models.Planner)
	}
}

func TestLoadPolicy_FileNotFound_ReturnsDefault(t *testing.T) {
	p, err := LoadPolicy("/nonexistent/policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if p.Budgets.MaxSpecCycles != 3 {
		t.Fatal("expected default when file missing")
	}
}
```

- **Step 2:** Run, expect FAIL (LoadPolicy undefined)

- **Step 3: Implement:**
```go
func LoadPolicy(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPolicy(), nil
		}
		return Policy{}, fmt.Errorf("read policy: %w", err)
	}
	p := DefaultPolicy() // start from defaults
	if err := json.Unmarshal(data, &p); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	// NOTE (I1): Do NOT add zero-value fallback lines here. The unmarshal-into-defaults
	// approach above already handles partial configs correctly: fields present in JSON
	// overwrite the default, fields absent retain the default. Explicit zero-value
	// checks (e.g., `if p.Budgets.MaxTaskRetries == 0`) would make it impossible to
	// intentionally set a field to 0 (e.g., MaxTaskRetries=0 means "no repair attempts").
	return p, nil
}
```

- **Step 4:** Run, expect PASS
- **Step 5:** Commit `"feat(next): add execpolicy JSON loading with fallback to defaults"`

---

### Task 3: `execpolicy/` — Validation

- **Files:**
  - Modify: `internal/next/execpolicy/policy.go`
  - Modify: `internal/next/execpolicy/policy_test.go`

- **Step 1: Write failing test:**
```go
func TestValidate_RejectsZeroRequiredBudgets(t *testing.T) {
	p := Policy{} // all zeroes
	err := p.Validate()
	if err == nil {
		t.Fatal("expected validation error for zero budgets")
	}
	// Must reject: MaxSpecCycles, MaxRunDurationSeconds, MaxRunCostUSD must be positive
	// Must NOT reject: MaxTaskRetries=0 and MaxRedecompositionPasses=0 are valid (means "no retries/redecomposition")
}

func TestValidate_AcceptsDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("default policy should be valid: %v", err)
	}
}

func TestValidate_AcceptsZeroTaskRetriesAndRedecomposition(t *testing.T) {
	p := DefaultPolicy()
	p.Budgets.MaxTaskRetries = 0
	p.Budgets.MaxRedecompositionPasses = 0
	if err := p.Validate(); err != nil {
		t.Fatalf("MaxTaskRetries=0 and MaxRedecompositionPasses=0 should be valid: %v", err)
	}
}
```

- **Step 2:** Run, expect FAIL
- **Step 3: Implement** `Validate()` method — check MaxSpecCycles>0, MaxRunDurationSeconds>0, MaxRunCostUSD>0, Models non-empty. MaxTaskRetries and MaxRedecompositionPasses can be 0 (valid — means "no retries" / "no redecomposition"). Return joined errors.
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 4: `runstore/` — Run record types

- **Files:**
  - Create: `internal/next/runstore/types.go`
  - Create: `internal/next/runstore/types_test.go`

- **Step 1: Write failing test:**
```go
package runstore

import "testing"

func TestRunState_IsTerminal(t *testing.T) {
	cases := []struct{ status string; want bool }{
		{"running", false},
		{"ready_for_review", true},
		{"needs_human", true},
		{"blocked", true},
	}
	for _, tc := range cases {
		rs := RunState{Status: tc.status}
		if rs.IsTerminal() != tc.want {
			t.Errorf("status=%s: want IsTerminal=%v", tc.status, tc.want)
		}
	}
}

func TestNewRunState_GeneratesID(t *testing.T) {
	rs := NewRunState("spec-001", "proj-1")
	if rs.RunID == "" {
		t.Fatal("RunID must not be empty")
	}
	if rs.Status != "running" {
		t.Fatalf("want running, got %s", rs.Status)
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
package runstore

import "time"

const (
	StatusRunning        = "running"
	StatusReadyForReview = "ready_for_review"
	StatusNeedsHuman     = "needs_human"
	StatusBlocked        = "blocked"
)

type RunState struct {
	RunID           string    `json:"run_id"`
	SpecID          string    `json:"spec_id"`
	ProjectID       string    `json:"project_id"`
	Status          string    `json:"status"`
	Cycle           int       `json:"cycle"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at,omitempty"`
	Tasks           []Task    `json:"tasks"`
	WorktreePath    string    `json:"worktree_path,omitempty"`
	BlockerSummary  string    `json:"blocker_summary,omitempty"`
	AccumulatedCost        float64   `json:"accumulated_cost"`
	TerminalReason         string    `json:"terminal_reason,omitempty"` // e.g. "all_passed", "budget_exceeded", "cycles_exhausted", "infra_failure"
	FinalValidationPassed  bool      `json:"final_validation_passed"`  // (I4) set by ValidateStage, checked by FinalizeStage
}

type Task struct {
	TaskID              string   `json:"task_id"`
	Objective           string   `json:"objective"`
	Status              string   `json:"status"` // pending, running, done, failed, needs_split
	Attempts            int      `json:"attempts"`
	ExpectedTouchedArea []string `json:"expected_touched_area"`
	ProofChecks         []string `json:"proof_checks"`
	FilesChanged        []string `json:"files_changed"`
	TokensUsed          int      `json:"tokens_used"`
	DurationMs          int64    `json:"duration_ms"`
	ModelTier           string   `json:"model_tier"`
	Cycle               int      `json:"cycle"`
	Kind                string   `json:"kind"` // "original" or "fix"
	ParentCycle         int      `json:"parent_cycle,omitempty"`
	FailuresAddressed   []string `json:"failures_addressed,omitempty"` // fix-cycle metadata
}

func NewRunState(specID, projectID string) *RunState {
	return &RunState{
		RunID:     generateID("run"),
		SpecID:    specID,
		ProjectID: projectID,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		Tasks:     []Task{},
	}
}

func (rs *RunState) IsTerminal() bool {
	switch rs.Status {
	case StatusReadyForReview, StatusNeedsHuman, StatusBlocked:
		return true
	}
	return false
}

// generateID produces a prefixed unique ID (e.g., "run-abc123")
func generateID(prefix string) string {
	// Use crypto/rand hex or time-based — keep simple
}
```

- **Step 4:** PASS
- **Step 4a (I5):** Add `NormalizeNilFields()` to `RunState` (maps nil `Tasks` to `[]Task{}`) and `Task` (maps nil `ExpectedTouchedArea`, `ProofChecks`, `FilesChanged`, `FailuresAddressed` to empty slices). Exported since these are cross-package types.
- **Step 5:** Commit `"feat(next): add runstore types with RunState and Task"`

---

### Task 5: `runstore/` — Store CRUD (create + get)

- **Files:**
  - Create: `internal/next/runstore/store.go`
  - Create: `internal/next/runstore/store_test.go`

- **Step 1: Write failing test:**
```go
func TestStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")

	if err := s.Save(rs); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Get(rs.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SpecID != "spec-1" {
		t.Fatalf("want spec-1, got %s", loaded.SpecID)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — `Store` struct with `rootDir`, `Save` writes `runs/<run-id>/run.json`, `Get` reads it back.
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 6: `runstore/` — List runs

- **Files:**
  - Modify: `internal/next/runstore/store.go`
  - Modify: `internal/next/runstore/store_test.go`

- **Step 1: Write failing test:**
```go
func TestStore_List(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.Save(NewRunState("spec-1", "proj-1"))
	s.Save(NewRunState("spec-2", "proj-1"))

	runs, err := s.List("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(runs))
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `List(projectID string)` — scan `runs/` dirs, filter by projectID
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 7: `runstore/` — Events log (append + read)

- **Files:**
  - Create: `internal/next/runstore/events.go`
  - Create: `internal/next/runstore/events_test.go`

- **Step 1: Write failing test:**
```go
func TestEvents_AppendAndRead(t *testing.T) {
	dir := t.TempDir()
	el := NewEventLog(filepath.Join(dir, "events.jsonl"))

	el.Append(RunStartedEvent{
		BaseEvent: BaseEvent{Type: "run_started", Timestamp: time.Now()},
		SpecID: "spec-1", ProjectID: "proj-1",
	})
	el.Append(TaskStartedEvent{
		BaseEvent: BaseEvent{Type: "task_started", Timestamp: time.Now()},
		TaskID: "t-001", Cycle: 1,
	})

	events, err := el.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0].EventType() != "run_started" {
		t.Fatalf("want run_started, got %s", events[0].EventType())
	}
	// Verify typed deserialization
	if started, ok := events[0].(RunStartedEvent); !ok {
		t.Fatal("expected RunStartedEvent type")
	} else if started.SpecID != "spec-1" {
		t.Fatalf("want spec-1, got %s", started.SpecID)
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
// TypedEvent interface — follows the pattern from internal/v2/event/event.go
type TypedEvent interface {
	EventType() string
	EventTimestamp() time.Time
}

// Base event fields embedded by all typed events
type BaseEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

func (b BaseEvent) EventType() string        { return b.Type }
func (b BaseEvent) EventTimestamp() time.Time { return b.Timestamp }

// Typed event structs
type RunStartedEvent struct {
	BaseEvent
	SpecID    string `json:"spec_id"`
	ProjectID string `json:"project_id"`
}

type TaskStartedEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
	Cycle  int    `json:"cycle"`
}

type TaskCompletedEvent struct {
	BaseEvent
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
}

type SpecPacketCompiledEvent struct {
	BaseEvent
	PacketPath string `json:"packet_path"`
}

type PlanCreatedEvent struct {
	BaseEvent
	Cycle int    `json:"cycle"`
	Kind  string `json:"kind"` // "original" or "fix"
}

type PlanValidationResultEvent struct {
	BaseEvent
	Pass   bool     `json:"pass"`
	Errors []string `json:"errors,omitempty"`
}

type TaskCreatedEvent struct {
	BaseEvent
	TaskID    string `json:"task_id"`
	Objective string `json:"objective"`
}

// ValidationCheckResult is a lightweight event-specific struct to avoid importing
// validator types into the runstore leaf package (which would create a circular dep).
type ValidationCheckResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
}

type TaskValidationResultEvent struct {
	BaseEvent
	TaskID string                  `json:"task_id"`
	Checks []ValidationCheckResult `json:"checks"`
	Passed bool                    `json:"passed"`
}

type TaskFailedEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

type TaskNeedsSplitEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

type RedecompositionTriggeredEvent struct {
	BaseEvent
	TaskID       string `json:"task_id"`
	SubTaskCount int    `json:"sub_task_count"`
}

type FinalValidationResultEvent struct {
	BaseEvent
	Passed  bool                    `json:"passed"`
	Results []ValidationCheckResult `json:"results"`
}

type ReplanTriggeredEvent struct {
	BaseEvent
	Cycle          int    `json:"cycle"`
	FailureContext string `json:"failure_context"`
}

type BudgetExceededEvent struct {
	BaseEvent
	Reason string  `json:"reason"`
	Cost   float64 `json:"cost,omitempty"`
}

type TerminalStateEvent struct {
	BaseEvent
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// EventLog supports typed events with JSON discriminator on "type" field
type EventLog struct{ path string }

func NewEventLog(path string) *EventLog { return &EventLog{path: path} }

func (el *EventLog) Append(e TypedEvent) error {
	// Marshal typed event + append line to file
}

func (el *EventLog) ReadAll() ([]TypedEvent, error) {
	// Read file, split lines, peek "type" field, unmarshal to correct typed struct
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 8: `runstore/` — Artifact directory helpers

- **Files:**
  - Modify: `internal/next/runstore/store.go`
  - Modify: `internal/next/runstore/store_test.go`

- **Step 1: Write failing test:**
```go
func TestStore_RunDir_Layout(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")
	s.Save(rs)

	runDir := s.RunDir(rs.RunID)
	if _, err := os.Stat(filepath.Join(runDir, "run.json")); err != nil {
		t.Fatal("run.json must exist in run dir")
	}

	taskDir := s.TaskDir(rs.RunID, "t-001")
	// Just verify path is correct
	if !strings.Contains(taskDir, "tasks/t-001") {
		t.Fatalf("unexpected task dir: %s", taskDir)
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `RunDir`, `TaskDir`, `EvidenceDir` path helpers. `Save` creates dirs as needed.
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 9: `runstore/` — Write/read task artifacts

- **Files:**
  - Modify: `internal/next/runstore/store.go`
  - Modify: `internal/next/runstore/store_test.go`

- **Step 1: Write failing test:**
```go
func TestStore_WriteAndReadTaskArtifact(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")
	s.Save(rs)

	err := s.WriteTaskArtifact(rs.RunID, "t-001", "result.json", map[string]string{"status": "done"})
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	err = s.ReadTaskArtifact(rs.RunID, "t-001", "result.json", &result)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "done" {
		t.Fatalf("want done, got %s", result["status"])
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `WriteTaskArtifact` / `ReadTaskArtifact` — JSON marshal to `tasks/<task-id>/<filename>`
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 10: `validator/` — Check runner (single command execution)

- **Files:**
  - Create: `internal/next/validator/runner.go`
  - Create: `internal/next/validator/runner_test.go`

- **Step 1: Write failing test:**
```go
package validator

import "testing"

func TestRunCheck_PassingCommand(t *testing.T) {
	r := NewRunner()
	result, err := r.RunCheck(context.Background(), Check{
		Name: "echo", Command: "echo hello", Type: "test",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatal("expected pass")
	}
}

func TestRunCheck_FailingCommand(t *testing.T) {
	r := NewRunner()
	result, err := r.RunCheck(context.Background(), Check{
		Name: "fail", Command: "false", Type: "test",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Fatal("expected fail")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
package validator

import (
	"context"
	"os/exec"
	"time"
)

type Check struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Type    string `json:"type"`
}

type CheckResult struct {
	Name     string        `json:"name"`
	Pass     bool          `json:"pass"`
	Output   string        `json:"output"`
	Duration time.Duration `json:"duration"`
	Type     string        `json:"type"`
}

type Runner struct{}

func NewRunner() *Runner { return &Runner{} }

func (r *Runner) RunCheck(ctx context.Context, c Check, workDir string) (CheckResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	pass := err == nil
	return CheckResult{
		Name: c.Name, Pass: pass, Output: string(out),
		Duration: time.Since(start), Type: c.Type,
	}, nil
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 11: `validator/` — Always-run check execution

- **Files:**
  - Create: `internal/next/validator/always_run.go`
  - Modify: `internal/next/validator/runner_test.go`

- **Step 1: Write failing test:**
```go
func TestRunAlwaysRun_AllPass(t *testing.T) {
	r := NewRunner()
	checks := []Check{
		{Name: "echo1", Command: "echo a", Type: "test"},
		{Name: "echo2", Command: "echo b", Type: "lint"},
	}
	results, err := r.RunAlwaysRun(context.Background(), checks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results.AllPass() {
		t.Fatal("expected all pass")
	}
	if results.PassCount() != 2 {
		t.Fatalf("want 2, got %d", results.PassCount())
	}
}

func TestRunAlwaysRun_SomeFail(t *testing.T) {
	r := NewRunner()
	checks := []Check{
		{Name: "pass", Command: "true", Type: "test"},
		{Name: "fail", Command: "false", Type: "lint"},
	}
	results, err := r.RunAlwaysRun(context.Background(), checks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if results.AllPass() {
		t.Fatal("expected some failures")
	}
	if results.FailCount() != 1 {
		t.Fatalf("want 1 failure, got %d", results.FailCount())
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type CheckResults struct {
	Results []CheckResult `json:"results"`
}

func (cr CheckResults) AllPass() bool { /* iterate */ }
func (cr CheckResults) PassCount() int { /* count */ }
func (cr CheckResults) FailCount() int { /* count */ }
func (cr CheckResults) FailedChecks() []CheckResult { /* filter */ }

func (r *Runner) RunAlwaysRun(ctx context.Context, checks []Check, workDir string) (CheckResults, error) {
	var results []CheckResult
	for _, c := range checks {
		res, err := r.RunCheck(ctx, c, workDir)
		if err != nil {
			return CheckResults{}, err
		}
		results = append(results, res)
	}
	return CheckResults{Results: results}, nil
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 12: `validator/` — Targeted checks (proof checks per task)

- **Files:**
  - Create: `internal/next/validator/targeted.go`
  - Create: `internal/next/validator/targeted_test.go`

- **Step 1: Write failing test:**
```go
func TestRunTargeted_ExecutesProofChecks(t *testing.T) {
	r := NewRunner()
	proofChecks := []string{"echo proof1", "echo proof2"}
	results, err := r.RunTargeted(context.Background(), proofChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results.AllPass() {
		t.Fatal("expected all pass")
	}
	if len(results.Results) != 2 {
		t.Fatalf("want 2, got %d", len(results.Results))
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `RunTargeted` — converts proof check strings to Check objects, runs each
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 13: `validator/` — Final validation (always-run + project checks only)

> **NOTE (I7):** The spec says final validation is only always-run checks + project cell
> `validation.json`. Proof checks are task-level targeted checks and are NOT part of
> final validation. They are run during the TaskLoop inspect step instead.

- **Files:**
  - Create: `internal/next/validator/final.go`
  - Create: `internal/next/validator/final_test.go`

- **Step 1: Write failing test:**
```go
func TestRunFinal_CombinesAlwaysRunAndProjectChecks(t *testing.T) {
	r := NewRunner()
	alwaysRun := []Check{{Name: "vet", Command: "true", Type: "lint"}}
	projectChecks := []Check{{Name: "integration", Command: "true", Type: "test"}}

	result, err := r.RunFinal(context.Background(), alwaysRun, projectChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatal("expected pass")
	}
	if result.AlwaysRun.PassCount() != 1 || result.ProjectChecks.PassCount() != 1 {
		t.Fatal("unexpected counts")
	}
}

func TestRunFinal_ProjectChecksFail_OverallFails(t *testing.T) {
	r := NewRunner()
	alwaysRun := []Check{{Name: "vet", Command: "true", Type: "lint"}}
	projectChecks := []Check{{Name: "integration", Command: "false", Type: "test"}}

	result, err := r.RunFinal(context.Background(), alwaysRun, projectChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Fatal("expected fail when project checks fail")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type FinalResult struct {
	Pass          bool         `json:"pass"`
	AlwaysRun     CheckResults `json:"always_run"`
	ProjectChecks CheckResults `json:"project_checks"`
	// NOTE (I7): No Targeted/proofChecks field — proof checks are task-level,
	// run during TaskLoop inspect step, not during final validation.
}

// RunFinal runs two check sources (NO proof checks — those are task-level):
// - alwaysRun: from execution policy's always_run field
// - projectChecks: from project cell's validation.json (Spec 0001)
func (r *Runner) RunFinal(ctx context.Context, alwaysRun []Check, projectChecks []Check, workDir string) (FinalResult, error) {
	ar, err := r.RunAlwaysRun(ctx, alwaysRun, workDir)
	if err != nil { return FinalResult{}, err }
	pc, err := r.RunAlwaysRun(ctx, projectChecks, workDir)
	if err != nil { return FinalResult{}, err }
	return FinalResult{
		Pass: ar.AllPass() && pc.AllPass(),
		AlwaysRun: ar, ProjectChecks: pc,
	}, nil
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

## Phase 2: Middle Packages (depend on Phase 1)

### Task 14: `planner/` — Plan and task types

- **Files:**
  - Create: `internal/next/planner/types.go`
  - Create: `internal/next/planner/types_test.go`

- **Step 1: Write failing test:**
```go
package planner

import "testing"

func TestPlan_TaskByID(t *testing.T) {
	p := Plan{Tasks: []TaskDef{
		{TaskID: "t-001", Objective: "first"},
		{TaskID: "t-002", Objective: "second"},
	}}
	task, ok := p.TaskByID("t-002")
	if !ok || task.Objective != "second" {
		t.Fatal("expected to find t-002")
	}
	_, ok = p.TaskByID("t-999")
	if ok {
		t.Fatal("should not find nonexistent task")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
package planner

type Plan struct {
	SpecID            string    `json:"spec_id"`
	Cycle             int       `json:"cycle"`
	Tasks             []TaskDef `json:"tasks"`
	Kind              string    `json:"kind"` // "original" or "fix"
	ParentCycle       int       `json:"parent_cycle,omitempty"`       // for fix plans: which cycle this fixes
	FailuresAddressed []string  `json:"failures_addressed,omitempty"` // for fix plans: summary of failures being addressed
}

type TaskDef struct {
	TaskID              string   `json:"task_id"`
	Objective           string   `json:"objective"`
	ExpectedTouchedArea []string `json:"expected_touched_area"`
	ProofChecks         []string `json:"proof_checks"`
	// Fix-plan metadata (no Dependencies field — spec mandates sequential execution)
	ParentCycle       int      `json:"parent_cycle,omitempty"`
	FailuresAddressed []string `json:"failures_addressed,omitempty"`
}

func (p Plan) TaskByID(id string) (TaskDef, bool) {
	for _, t := range p.Tasks {
		if t.TaskID == id { return t, true }
	}
	return TaskDef{}, false
}
```

- **Step 4:** PASS
- **Step 4a (I5):** Add `NormalizeNilFields()` to `Plan` (maps nil `Tasks` to `[]TaskDef{}`, nil `FailuresAddressed` to `[]string{}`) and `TaskDef` (maps nil `ExpectedTouchedArea`, `ProofChecks`, `FailuresAddressed` to empty slices). Exported since these are cross-package types.
- **Step 5:** Commit

---

### Task 15: `planner/` — Plan validation rules

- **Files:**
  - Create: `internal/next/planner/validate.go`
  - Create: `internal/next/planner/validate_test.go`

- **Step 1: Write failing test:**
```go
func TestValidatePlan_RejectsEmptyTasks(t *testing.T) {
	p := Plan{Tasks: nil}
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected error for empty tasks")
	}
}

func TestValidatePlan_RejectsDuplicateIDs(t *testing.T) {
	p := Plan{Tasks: []TaskDef{
		{TaskID: "t-001", Objective: "a", ExpectedTouchedArea: []string{"x"}, ProofChecks: []string{"y"}},
		{TaskID: "t-001", Objective: "b", ExpectedTouchedArea: []string{"x"}, ProofChecks: []string{"y"}},
	}}
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
}

func TestValidatePlan_RejectsMissingFields(t *testing.T) {
	p := Plan{Tasks: []TaskDef{{TaskID: "t-001"}}} // missing objective, area, checks
	if err := ValidatePlan(p); err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestValidatePlan_AcceptsValid(t *testing.T) {
	p := Plan{Tasks: []TaskDef{{
		TaskID: "t-001", Objective: "do thing",
		ExpectedTouchedArea: []string{"pkg/a"}, ProofChecks: []string{"go test ./pkg/a/"},
	}}}
	if err := ValidatePlan(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlan_CrossCycleSequentialIDs(t *testing.T) {
	// Cycle 1 used t-001..t-004, fix-cycle 2 must continue from t-005
	priorMaxID := "t-004"
	p := Plan{
		Cycle: 2,
		Kind:  "fix",
		Tasks: []TaskDef{
			{TaskID: "t-005", Objective: "fix lint", ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"}},
			{TaskID: "t-006", Objective: "fix test", ExpectedTouchedArea: []string{"b/"}, ProofChecks: []string{"true"}},
		},
	}
	if err := ValidatePlanWithPrior(p, priorMaxID); err != nil {
		t.Fatalf("sequential IDs should be accepted: %v", err)
	}
}

func TestValidatePlan_CrossCycleNonSequentialIDs_Rejected(t *testing.T) {
	// Cycle 1 used t-001..t-004, fix-cycle reuses t-001 — invalid
	priorMaxID := "t-004"
	p := Plan{
		Cycle: 2,
		Kind:  "fix",
		Tasks: []TaskDef{
			{TaskID: "t-001", Objective: "fix lint", ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"}},
		},
	}
	if err := ValidatePlanWithPrior(p, priorMaxID); err == nil {
		t.Fatal("reusing prior cycle IDs should be rejected")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `ValidatePlan(Plan) error` — check non-empty tasks, unique IDs, required fields (TaskID, Objective, ExpectedTouchedArea, ProofChecks). Also implement `ValidatePlanWithPrior(Plan, priorMaxID string) error` — calls ValidatePlan and additionally checks that all task IDs are globally sequential (numerically greater than priorMaxID) to enforce cross-cycle ID continuity
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 16: `planner/` — Plan parsing from agent output

- **Files:**
  - Create: `internal/next/planner/parse.go`
  - Create: `internal/next/planner/parse_test.go`

- **Step 1: Write failing test:**
```go
func TestParsePlan_ExtractsJSONFromMarkdown(t *testing.T) {
	raw := "Here is the plan:\n```json\n" +
		`{"spec_id":"s1","cycle":1,"tasks":[{"task_id":"t-001","objective":"do it","expected_touched_area":["a/"],"proof_checks":["go test ./a/"]}]}` +
		"\n```\nDone."
	plan, err := ParsePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
}

func TestParsePlan_BareJSON(t *testing.T) {
	raw := `{"spec_id":"s1","cycle":1,"tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["b/"],"proof_checks":["true"]}]}`
	plan, err := ParsePlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SpecID != "s1" {
		t.Fatalf("want s1, got %s", plan.SpecID)
	}
}

func TestParsePlan_InvalidJSON(t *testing.T) {
	_, err := ParsePlan("not json at all")
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `ParsePlan(raw string) (Plan, error)` — extract JSON from markdown fences or bare, unmarshal, then `ValidatePlan`
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 17: `planner/` — Agent invocation interface

- **Files:**
  - Create: `internal/next/planner/planner.go`
  - Create: `internal/next/planner/planner_test.go`

- **Step 1: Write failing test:**
```go
func TestPlanner_InvokesAgentAndParsesPlan(t *testing.T) {
	validJSON := `{"spec_id":"s1","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"x","expected_touched_area":["a/"],"proof_checks":["true"]}]}`
	agent := &fakeAgent{output: validJSON}
	p := NewPlanner(agent, "high") // tier from execution policy

	plan, err := p.CreatePlan(context.Background(), PlanRequest{
		SpecPacket: "build a thing", Cycle: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(plan.Tasks))
	}
	if !agent.called {
		t.Fatal("agent not called")
	}
}

type fakeAgent struct {
	output string
	called bool
}

func (f *fakeAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	f.called = true
	return AgentResult{Output: f.output, TokensIn: 100, TokensOut: 50, Cost: 0.01, Model: "fake-model"}, nil
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
// NOTE (I6): AgentResult should wrap or align with the existing internal/provider.Result
// type (which has Output, InputTokens, OutputTokens, CostUSD, Model, Duration).
// Avoid creating a parallel hierarchy — either embed provider.Result or delegate to it.
type AgentResult struct {
	Output    string  `json:"output"`
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	Cost      float64 `json:"cost"`
	Model     string  `json:"model"`    // (I2) actual model resolved from tier, for per-invocation metrics
	Duration  int64   `json:"duration_ms,omitempty"`
}

type Agent interface {
	Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error)
}

type PlanRequest struct {
	SpecPacket string
	Cycle      int
	// For fix plans:
	CompletedTasks []string
	Failures       []string
	CurrentDiff    string
}

type Planner struct{
	agent       Agent
	plannerTier string // read from execution policy Models.Planner
}

func NewPlanner(agent Agent, plannerTier string) *Planner {
	return &Planner{agent: agent, plannerTier: plannerTier}
}

func (p *Planner) CreatePlan(ctx context.Context, req PlanRequest) (Plan, error) {
	prompt := buildPlanPrompt(req) // internal helper
	result, err := p.agent.Invoke(ctx, prompt, p.plannerTier)
	if err != nil { return Plan{}, err }
	return ParsePlan(result.Output)
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 18: `planner/` — Fix-plan generation

- **Files:**
  - Modify: `internal/next/planner/planner.go`
  - Modify: `internal/next/planner/planner_test.go`

- **Step 1: Write failing test:**
```go
func TestPlanner_CreateFixPlan(t *testing.T) {
	fixJSON := `{"spec_id":"s1","cycle":2,"kind":"fix","tasks":[{"task_id":"t-002","objective":"fix lint","expected_touched_area":["a/"],"proof_checks":["true"],"parent_cycle":1,"failures_addressed":["lint failure"]}]}`
	agent := &fakeAgent{output: fixJSON}
	p := NewPlanner(agent, "high")

	plan, err := p.CreateFixPlan(context.Background(), FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		CompletedTasks: []CompletedTask{{
			TaskID:       "t-001",
			Attempts:     1,
			FilesChanged: []string{"a/foo.go"},
			ValidationOutcome: "passed",
		}},
		Failures:    []string{"lint failure in a/"},
		CurrentDiff: "diff --git ...",
		Cycle:       2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Kind != "fix" {
		t.Fatalf("want fix, got %s", plan.Kind)
	}
	if plan.Tasks[0].ParentCycle != 1 {
		t.Fatal("expected parent_cycle=1")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type CompletedTask struct {
	TaskID            string   `json:"task_id"`
	Attempts          int      `json:"attempts"`
	FilesChanged      []string `json:"files_changed"`
	ValidationOutcome string   `json:"validation_outcome"` // "passed", "failed"
}

type FixPlanRequest struct {
	OriginalPlan   Plan            `json:"original_plan"`
	CompletedTasks []CompletedTask `json:"completed_tasks"`
	Failures       []string        `json:"failures"`
	CurrentDiff    string          `json:"current_diff"`
	Cycle          int             `json:"cycle"`
}
```
Implement `CreateFixPlan` method — builds fix-plan prompt including task results (attempts, files changed, validation outcomes), invokes agent, parses result
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 19: `executor/` — Task packet compilation

- **Files:**
  - Create: `internal/next/executor/packet.go`
  - Create: `internal/next/executor/packet_test.go`

- **Step 1: Write failing test:**
```go
package executor

import "testing"

func TestCompileTaskPacket(t *testing.T) {
	pkt, err := CompileTaskPacket(TaskPacketInput{
		SpecPacket: "build feature X",
		Task: TaskDef{TaskID: "t-001", Objective: "implement parser", ProofChecks: []string{"go test ./parser/"}},
		PriorContext: "tasks t-000 completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pkt == "" {
		t.Fatal("packet must not be empty")
	}
	// Should contain objective and proof checks
	if !strings.Contains(pkt, "implement parser") {
		t.Fatal("packet must contain objective")
	}
	if !strings.Contains(pkt, "go test ./parser/") {
		t.Fatal("packet must contain proof checks")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `CompileTaskPacket` — template-based, includes spec context, objective, proof checks, expected area
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 20: `executor/` — Agent invocation and result extraction

- **Files:**
  - Create: `internal/next/executor/executor.go`
  - Create: `internal/next/executor/executor_test.go`

- **Step 1: Write failing test:**
```go
func TestExecutor_RunTask_Success(t *testing.T) {
	agent := &fakeAgent{output: "Done. Changed files: parser.go"}
	exec := NewExecutor(agent)

	result, err := exec.RunTask(context.Background(), RunTaskInput{
		Packet:   "implement parser",
		WorkDir:  t.TempDir(),
		ModelTier: "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AgentOutput == "" {
		t.Fatal("expected agent output")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type Executor struct{ agent Agent }

type RunTaskInput struct {
	Packet                string
	WorkDir               string
	ModelTier             string
	MaxTaskDurationSeconds int // from budget; 0 means no limit
}

type RunTaskResult struct {
	AgentOutput  string
	TokensUsed   int
	Cost         float64
	DurationMs   int64
	FilesChanged []string
	Model        string
	Tier         string
}

func (e *Executor) RunTask(ctx context.Context, input RunTaskInput) (RunTaskResult, error) {
	start := time.Now()
	if input.MaxTaskDurationSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(input.MaxTaskDurationSeconds)*time.Second)
		defer cancel()
	}
	result, err := e.agent.Invoke(ctx, input.Packet, input.ModelTier)
	if err != nil { return RunTaskResult{}, err }
	return RunTaskResult{
		AgentOutput: result.Output,
		TokensUsed:  result.TokensIn + result.TokensOut,
		Cost:        result.Cost,
		DurationMs:  time.Since(start).Milliseconds(),
		Model:       result.Model,
		Tier:        result.Tier,
	}, nil
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 21: `executor/` — Worktree file inspection (detect changed files)

- **Files:**
  - Create: `internal/next/executor/inspect.go`
  - Create: `internal/next/executor/inspect_test.go`

- **Step 1: Write failing test** — create a temp git repo, modify a file, call `InspectChanges`, verify it returns the modified file
- **Step 2:** FAIL
- **Step 3: Implement** `InspectChanges(workDir string) ([]string, error)` — runs `git diff --name-only HEAD` in workDir
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 22: `executor/` — needs_split heuristic

- **Files:**
  - Create: `internal/next/executor/split.go`
  - Create: `internal/next/executor/split_test.go`

- **Step 1: Write failing test:**
```go
func TestNeedsSplit_ThreeDistinctPackagesFromChangedFiles(t *testing.T) {
	failures := []CheckResult{
		{Name: "test", Pass: false, Output: "some error output"},
	}
	changedFiles := []string{"pkg/a/x.go", "pkg/b/y.go", "pkg/c/z.go"}
	if !NeedsSplit(failures, changedFiles, []string{"pkg/a/"}) {
		t.Fatal("3+ distinct parent directories from changedFiles should trigger split")
	}
}

func TestNeedsSplit_AreaExceeded(t *testing.T) {
	failures := []CheckResult{{Name: "test", Pass: false}}
	changed := []string{"a/1.go", "a/2.go", "b/1.go", "b/2.go", "c/1.go"}
	expected := []string{"a/"}
	if !NeedsSplit(failures, changed, expected) {
		t.Fatal("files exceeding 2x expected area should trigger split")
	}
}

func TestNeedsSplit_SmallFailure(t *testing.T) {
	failures := []CheckResult{{Name: "test", Pass: false}}
	if NeedsSplit(failures, []string{"a/x.go"}, []string{"a/"}) {
		t.Fatal("single package failure should not trigger split")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `NeedsSplit(failures []CheckResult, changedFiles, expectedArea []string) bool` — count distinct parent directories from `changedFiles` (NOT from error output); >=3 distinct dirs triggers split. Also triggers if changed file count exceeds 2x the expected area directory count
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 23: `executor/` — Repair loop (one retry attempt)

- **Files:**
  - Create: `internal/next/executor/repair.go`
  - Create: `internal/next/executor/repair_test.go`

- **Step 1: Write failing test:**
```go
func TestRepairLoop_SucceedsOnRetry(t *testing.T) {
	callCount := 0
	agent := &countingAgent{fn: func() string {
		callCount++
		if callCount == 1 { return "still broken" }
		return "fixed"
	}}
	validator := &fakeValidator{passOnCall: 2}

	result, err := RepairLoop(context.Background(), RepairInput{
		Agent:     agent,
		Validator: validator,
		MaxRetries: 1,
		Packet:    "fix the bug",
		WorkDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Attempts != 2 {
		t.Fatalf("want 2 attempts, got %d", result.Attempts)
	}
}

func TestRepairLoop_ExhaustsRetries(t *testing.T) {
	agent := &countingAgent{fn: func() string { return "still broken" }}
	validator := &fakeValidator{passOnCall: 999} // never passes

	result, err := RepairLoop(context.Background(), RepairInput{
		Agent: agent, Validator: validator, MaxRetries: 1,
		Packet: "fix", WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" {
		t.Fatalf("want failed, got %s", result.Status)
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `RepairLoop` — implement -> validate -> if fail and retries left, retry once -> return result with attempt count and final status
- **Step 4:** PASS
- **Step 5:** Commit

---

## Phase 3: Orchestration

### Task 24: `specloop/` — Stage interface and NextAction types

- **Files:**
  - Create: `internal/next/specloop/stage.go`
  - Create: `internal/next/specloop/stage_test.go`

- **Step 1: Write failing test:**
```go
package specloop

import "testing"

func TestNextAction_ContinueIsDefault(t *testing.T) {
	na := NextAction{}
	if na.Kind != Continue {
		t.Fatalf("want Continue, got %v", na.Kind)
	}
}

func TestActionKind_String(t *testing.T) {
	cases := map[ActionKind]string{
		Continue:   "continue",
		ReplanFrom: "replan_from",
		NeedsHuman: "needs_human",
		Blocked:    "blocked",
	}
	for k, want := range cases {
		if k.String() != want {
			t.Fatalf("want %s, got %s", want, k.String())
		}
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
package specloop

import "context"

type ActionKind int

const (
	Continue   ActionKind = iota
	ReplanFrom
	NeedsHuman
	Blocked
)

func (k ActionKind) String() string {
	// switch on value
}

type FailureContext struct {
	Failures []string `json:"failures"`
	Cycle    int      `json:"cycle"`
	Diff     string   `json:"diff,omitempty"`
}

type NextAction struct {
	Kind    ActionKind
	Context *FailureContext
}

type Stage interface {
	Name() string
	Run(ctx context.Context, run *RunState) (NextAction, error)
}
```
(Note: `RunState` here is imported from `runstore` package — use the type alias or import.)

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 25: `specloop/` — SpecLoop runner (stage pipeline)

- **Files:**
  - Create: `internal/next/specloop/specloop.go`
  - Create: `internal/next/specloop/specloop_test.go`

- **Step 1: Write failing test:**
```go
func TestSpecLoop_RunsStagesInOrder(t *testing.T) {
	var order []string
	stages := []Stage{
		&recordStage{name: "init", order: &order},
		&recordStage{name: "compile", order: &order},
		&recordStage{name: "plan", order: &order},
		&recordStage{name: "finalize", order: &order},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 1})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"init", "compile", "plan", "finalize"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("want %v, got %v", want, order)
	}
}

type recordStage struct {
	name  string
	order *[]string
}

func (s *recordStage) Name() string { return s.name }
func (s *recordStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	*s.order = append(*s.order, s.name)
	return NextAction{Kind: Continue}, nil
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type SpecLoopConfig struct {
	MaxCycles  int
	Budget     *Budget     // full budget reference for cost/time/cycle enforcement
	ReplanStage string     // stage name to restart from on ReplanFrom
}

type SpecLoop struct {
	stages []Stage
	config SpecLoopConfig
}

func NewSpecLoop(stages []Stage, cfg SpecLoopConfig) *SpecLoop {
	return &SpecLoop{stages: stages, config: cfg}
}

func (sl *SpecLoop) Run(ctx context.Context, rs *runstore.RunState) error {
	for cycle := 0; cycle < sl.config.MaxCycles; cycle++ {
		// Track cycle in budget so CyclesExhausted() reflects actual count
		if sl.config.Budget != nil {
			sl.config.Budget.IncrementCycle()
		}
		// Wire cycle into RunState so run.json reflects actual cycle count
		rs.Cycle = cycle + 1
		for _, stage := range sl.stages {
			// (C1) Check hard budget (cost/time) between stages → blocked
			if sl.config.Budget != nil && sl.config.Budget.HardBudgetExceeded() {
				rs.Status = runstore.StatusBlocked
				rs.TerminalReason = "budget_exceeded"
				rs.BlockerSummary = sl.config.Budget.Reason()
				// Emit budget_exceeded event
				return nil
			}

			action, err := stage.Run(ctx, rs)
			if err != nil {
				rs.Status = runstore.StatusBlocked
				rs.BlockerSummary = err.Error()
				// Still run EvidenceStage to capture partial results
				if evidenceStage := sl.findStage("evidence"); evidenceStage != nil {
					evidenceStage.Run(ctx, rs)
				}
				// Emit blocked event
				return nil
			}
			switch action.Kind {
			case Continue:
				continue
			case ReplanFrom:
				break // restart from plan stage (next cycle)
			case NeedsHuman:
				rs.Status = runstore.StatusNeedsHuman
				// Spec: "For needs_human and blocked, the evidence bundle is still emitted."
				if evidenceStage := sl.findStage("evidence"); evidenceStage != nil {
					evidenceStage.Run(ctx, rs)
				}
				return nil
			case Blocked:
				rs.Status = runstore.StatusBlocked
				// Spec: "For needs_human and blocked, the evidence bundle is still emitted."
				if evidenceStage := sl.findStage("evidence"); evidenceStage != nil {
					evidenceStage.Run(ctx, rs)
				}
				return nil
			}
		}
		if rs.IsTerminal() { return nil }
	}
	// (C1) Cycle exhaustion with remaining failures → needs_human (NOT blocked)
	if sl.config.Budget != nil && sl.config.Budget.CyclesExhausted() && !rs.IsTerminal() {
		rs.Status = runstore.StatusNeedsHuman
		rs.TerminalReason = "cycles_exhausted"
	}
	return nil
}

func (sl *SpecLoop) findStage(name string) Stage {
	for _, s := range sl.stages {
		if s.Name() == name { return s }
	}
	return nil
}
```

- **Step 4: Add error-handling test:**
```go
func TestSpecLoop_StageError_SetsBlockedAndRunsEvidence(t *testing.T) {
	evidenceRan := false
	stages := []Stage{
		&recordStage{name: "init", order: new([]string)},
		&errorStage{name: "plan", err: fmt.Errorf("infra failure")},
		&recordStage{name: "execute", order: new([]string)},
		&callbackStage{name: "evidence", fn: func() { evidenceRan = true }},
		&recordStage{name: "finalize", order: new([]string)},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 1})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("SpecLoop should not propagate stage errors, got: %v", err)
	}
	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("want blocked, got %s", rs.Status)
	}
	if !evidenceRan {
		t.Fatal("evidence stage should still run on error")
	}
}
```

- **Step 5:** PASS
- **Step 6:** Commit

---

### Task 26: `specloop/` — ReplanFrom loops back to Plan stage

- **Files:**
  - Modify: `internal/next/specloop/specloop.go`
  - Modify: `internal/next/specloop/specloop_test.go`

- **Step 1: Write failing test:**
```go
func TestSpecLoop_ReplanFromLoopsBack(t *testing.T) {
	callCounts := map[string]int{}
	validate := &actionStage{name: "validate", actionFn: func() NextAction {
		callCounts["validate"]++
		if callCounts["validate"] == 1 {
			return NextAction{Kind: ReplanFrom, Context: &FailureContext{Failures: []string{"lint fail"}}}
		}
		return NextAction{Kind: Continue}
	}}
	stages := []Stage{
		&countStage{name: "init", counts: callCounts},
		&countStage{name: "plan", counts: callCounts},
		&countStage{name: "execute", counts: callCounts},
		validate,
		&countStage{name: "finalize", counts: callCounts},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 3, ReplanStage: "plan"})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	if callCounts["plan"] != 2 {
		t.Fatalf("plan should run twice, got %d", callCounts["plan"])
	}
	if callCounts["finalize"] != 1 {
		t.Fatal("finalize should run once after second pass succeeds")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — add `ReplanStage` to config, on `ReplanFrom` break inner loop and restart from that stage index on next cycle
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 27: `specloop/` — TaskLoop

- **Files:**
  - Create: `internal/next/specloop/taskloop.go`
  - Create: `internal/next/specloop/taskloop_test.go`

- **Step 1: Write failing test:**
```go
func TestTaskLoop_ExecutesAllTasks(t *testing.T) {
	executed := []string{}
	runner := &fakeTaskRunner{fn: func(id string) TaskResult {
		executed = append(executed, id)
		return TaskResult{Status: "done"}
	}}
	// fakeInspector that always passes — all tasks succeed on first try
	inspector := &fakeInspector{fn: func(ctx context.Context, task runstore.Task) InspectResult {
		return InspectResult{Pass: true}
	}}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
	}

	results, err := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{MaxRetries: 1, Inspector: inspector})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2, got %d", len(results))
	}
	if !reflect.DeepEqual(executed, []string{"t-001", "t-002"}) {
		t.Fatalf("unexpected order: %v", executed)
	}
}

func TestTaskLoop_RetriesOnFailure(t *testing.T) {
	calls := 0
	runner := &fakeTaskRunner{fn: func(id string) TaskResult {
		calls++
		if calls == 1 { return TaskResult{Status: "failed"} }
		return TaskResult{Status: "done"}
	}}
	// fakeInspector: first call fails (triggering repair), second call passes
	inspectCalls := 0
	inspector := &fakeInspector{fn: func(ctx context.Context, task runstore.Task) InspectResult {
		inspectCalls++
		if inspectCalls == 1 { return InspectResult{Pass: false, Failures: []string{"test failed"}} }
		return InspectResult{Pass: true}
	}}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{MaxRetries: 1, Inspector: inspector})
	if results[0].Status != "done" {
		t.Fatalf("expected done after retry, got %s", results[0].Status)
	}
	if results[0].Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", results[0].Attempts)
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type TaskRunner interface {
	RunTask(ctx context.Context, task runstore.Task) (TaskResult, error)
	RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error) // (I3) re-invoke with failure context
}

type CheckSummary struct {
	Pass int
	Fail int
}

type TaskResult struct {
	Status          string // done, failed, needs_split
	Attempts        int
	TokensUsed      int
	Cost            float64
	DurationMs      int64
	FilesChanged    []string
	Model           string
	Tier            string
	TargetedChecks  CheckSummary
	AlwaysRunChecks CheckSummary
}

type TaskInspector interface {
	// Inspect runs targeted checks (proof checks) + always-run checks for a task
	Inspect(ctx context.Context, task runstore.Task) InspectResult
}

type InspectResult struct {
	Pass     bool
	Failures []string // failure descriptions for repair context
}

type TaskLoopConfig struct {
	MaxRetries int
	Inspector  TaskInspector // (I3) runs checks between implement and repair steps
}

// NOTE (I3): The spec's TaskLoop is: implement → inspect (targeted + always-run checks)
// → if fail → repair (invoke executor with failure context) → re-inspect → done/failed/needs_split.
// This is NOT a generic retry loop — the inspect step between implement and repair is critical.
func RunTaskLoop(ctx context.Context, tasks []runstore.Task, runner TaskRunner, cfg TaskLoopConfig) ([]TaskResult, error) {
	var results []TaskResult
	for _, task := range tasks {
		// Step 1: Implement — invoke executor
		attempts := 1
		result, _ := runner.RunTask(ctx, task)

		// Step 2: Inspect — run targeted checks (proof checks) + always-run checks
		inspectResult := cfg.Inspector.Inspect(ctx, task)

		if inspectResult.Pass {
			result.Status = "done"
			result.Attempts = attempts
			results = append(results, result)
			continue
		}

		// Step 3: Repair loop — retry with failure context from inspection
		// NOTE: We track attempts in a local variable because RepairTask returns
		// a fresh TaskResult with Attempts=0, which would lose the count.
		for repair := 0; repair < cfg.MaxRetries; repair++ {
			attempts++
			// Invoke executor again with inspection failure context
			repairResult, _ := runner.RepairTask(ctx, task, inspectResult.Failures)
			result = repairResult

			// Step 4: Re-inspect
			inspectResult = cfg.Inspector.Inspect(ctx, task)
			if inspectResult.Pass {
				result.Status = "done"
				break
			}
		}

		if !inspectResult.Pass {
			result.Status = "failed" // or "needs_split" based on NeedsSplit heuristic
		}
		result.Attempts = attempts
		results = append(results, result)
	}
	return results, nil
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 28: `specloop/` — Re-decomposition on needs_split

- **Files:**
  - Modify: `internal/next/specloop/taskloop.go`
  - Modify: `internal/next/specloop/taskloop_test.go`

- **Step 1: Write failing test:**
```go
func TestTaskLoop_RedecomposesOnNeedsSplit(t *testing.T) {
	runner := &fakeTaskRunner{fn: func(id string) TaskResult {
		if id == "t-001" { return TaskResult{Status: "needs_split"} }
		return TaskResult{Status: "done"}
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001a", Status: "pending"},
		{TaskID: "t-001b", Status: "pending"},
	}}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Decomposer: decomposer, MaxRedecompositions: 1,
	})

	// Should have 3 results: original failed + 2 sub-tasks
	doneCount := 0
	for _, r := range results {
		if r.Status == "done" { doneCount++ }
	}
	if doneCount != 2 {
		t.Fatalf("expected 2 done sub-tasks, got %d done", doneCount)
	}
	if !decomposer.called {
		t.Fatal("decomposer should have been called")
	}
}
```

- **Step 2:** FAIL
- **Step 2a: Write revert test:**
```go
func TestTaskLoop_RevertBeforeRedecompose(t *testing.T) {
	gitOps := &fakeGitOps{}
	runner := &fakeTaskRunner{fn: func(id string) TaskResult {
		if id == "t-001" { return TaskResult{Status: "needs_split", FilesChanged: []string{"a.go", "b.go"}} }
		return TaskResult{Status: "done"}
	}}
	decomposer := &fakeDecomposer{subTasks: []runstore.Task{
		{TaskID: "t-001a", Status: "pending"},
	}}
	tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}

	RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
		MaxRetries: 1, Decomposer: decomposer, MaxRedecompositions: 1, GitOps: gitOps,
	})

	if !gitOps.checkoutCalled {
		t.Fatal("git checkout should be called on touched files before sub-task execution")
	}
	if !reflect.DeepEqual(gitOps.checkoutFiles, []string{"a.go", "b.go"}) {
		t.Fatalf("checkout should revert touched files, got %v", gitOps.checkoutFiles)
	}
}
```

- **Step 3: Implement** — add `Decomposer` interface, `MaxRedecompositions`, and `GitOps` interface to config. On `needs_split`: call `GitOps.Checkout` on touched files to revert changes before sub-task execution, invoke decomposer, append sub-tasks, consume budget. Sub-tasks cannot further decompose.
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 29: `specloop/` — Budget enforcement (time + cost + cycles)

- **Files:**
  - Create: `internal/next/specloop/budget.go`
  - Create: `internal/next/specloop/budget_test.go`

- **Step 1: Write failing test:**
```go
func TestBudget_ExceedsMaxCycles(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2})
	b.IncrementCycle()
	b.IncrementCycle()
	if !b.Exceeded() {
		t.Fatal("should be exceeded after 2 cycles with max=2")
	}
}

func TestBudget_ExceedsCost(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 10.0, MaxSpecCycles: 99})
	b.AddCost(11.0)
	if !b.Exceeded() {
		t.Fatal("should be exceeded when cost > max")
	}
}

func TestBudget_NotExceeded(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 5, MaxRunCostUSD: 50.0, MaxRunDurationSeconds: 3600})
	b.IncrementCycle()
	b.AddCost(5.0)
	if b.Exceeded() {
		t.Fatal("should not be exceeded yet")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type Budget struct {
	limits   execpolicy.Budgets
	cycles   int
	cost     float64
	startedAt time.Time
}

func NewBudget(limits execpolicy.Budgets) *Budget {
	return &Budget{limits: limits, startedAt: time.Now()}
}

func (b *Budget) IncrementCycle()     { b.cycles++ }
func (b *Budget) AddCost(usd float64) { b.cost += usd }

// NOTE (C1): Exceeded() returns true for ANY budget exceeded, but callers must
// distinguish cycle exhaustion (→ needs_human) from cost/time (→ blocked).
// Use CyclesExhausted() and HardBudgetExceeded() for distinct terminal states.

func (b *Budget) Exceeded() bool {
	return b.CyclesExhausted() || b.HardBudgetExceeded()
}

// CyclesExhausted returns true when max_spec_cycles is used up.
// Terminal state: needs_human (validation failures remain but cycles are spent).
func (b *Budget) CyclesExhausted() bool {
	return b.cycles >= b.limits.MaxSpecCycles
}

// HardBudgetExceeded returns true when cost or time limits are exceeded.
// Terminal state: blocked with budget_exceeded event.
func (b *Budget) HardBudgetExceeded() bool {
	if b.limits.MaxRunCostUSD > 0 && b.cost >= b.limits.MaxRunCostUSD { return true }
	if b.limits.MaxRunDurationSeconds > 0 &&
		time.Since(b.startedAt).Seconds() >= float64(b.limits.MaxRunDurationSeconds) { return true }
	return false
}

func (b *Budget) Reason() string { /* return which limit hit */ }
```

- **Step 4:** PASS

- **Step 4a: Write additional wiring tests:**
```go
func TestTaskLoop_ChecksBudgetBetweenTasks(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	budget.AddCost(1.5) // already exceeded
	runner := &fakeTaskRunner{fn: func(id string) TaskResult {
		return TaskResult{Status: "done"}
	}}
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending"},
		{TaskID: "t-002", Status: "pending"},
	}

	results, _ := RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{MaxRetries: 1, Budget: budget})
	// Only t-001 should have been attempted (budget check before t-002)
	doneCount := 0
	for _, r := range results {
		if r.Status == "done" { doneCount++ }
	}
	if doneCount != 1 {
		t.Fatalf("want 1 done task (budget should stop t-002), got %d", doneCount)
	}
}

func TestSpecLoop_ChecksBudgetBetweenStages(t *testing.T) {
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	executed := []string{}
	stages := []Stage{
		&recordStage{name: "plan", order: &executed},
		&costStage{name: "execute", order: &executed, budget: budget, costToAdd: 2.0},
		&recordStage{name: "validate", order: &executed},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 1, Budget: budget})
	rs := runstore.NewRunState("s1", "p1")

	loop.Run(context.Background(), rs)

	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("want blocked, got %s", rs.Status)
	}
	// validate should not have run
	for _, name := range executed {
		if name == "validate" {
			t.Fatal("validate should not run after budget exceeded")
		}
	}
}

func TestBudget_CyclesExhausted_IsDistinctFromHardBudget(t *testing.T) {
	// (C1) Cycle exhaustion → needs_human; cost/time → blocked
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxRunCostUSD: 50.0})
	b.IncrementCycle()
	b.IncrementCycle()
	if !b.CyclesExhausted() {
		t.Fatal("cycles should be exhausted")
	}
	if b.HardBudgetExceeded() {
		t.Fatal("hard budget (cost/time) should NOT be exceeded")
	}
}

func TestBudget_HardBudgetExceeded_CostExceeded(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	b.AddCost(1.5)
	if !b.HardBudgetExceeded() {
		t.Fatal("hard budget should be exceeded when cost > max")
	}
	if b.CyclesExhausted() {
		t.Fatal("cycles should NOT be exhausted")
	}
}

func TestSpecLoop_CycleExhaustion_SetsNeedsHuman(t *testing.T) {
	// (C1) When cycles run out with remaining failures, status = needs_human
	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 999})
	stages := []Stage{
		&countStage{name: "plan", counts: map[string]int{}},
		&countStage{name: "execute", counts: map[string]int{}},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 1, Budget: budget})
	rs := runstore.NewRunState("s1", "p1")
	budget.IncrementCycle()

	loop.Run(context.Background(), rs)
	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("cycle exhaustion should produce needs_human, got %s", rs.Status)
	}
}

func TestSpecLoop_CostExceeded_SetsBlocked(t *testing.T) {
	// (C1) When cost exceeded, status = blocked with budget_exceeded
	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	budget.AddCost(1.5)
	stages := []Stage{
		&countStage{name: "plan", counts: map[string]int{}},
	}
	loop := NewSpecLoop(stages, SpecLoopConfig{MaxCycles: 1, Budget: budget})
	rs := runstore.NewRunState("s1", "p1")

	loop.Run(context.Background(), rs)
	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("cost exceeded should produce blocked, got %s", rs.Status)
	}
	if rs.TerminalReason != "budget_exceeded" {
		t.Fatalf("want terminal_reason=budget_exceeded, got %s", rs.TerminalReason)
	}
}
```

- **Step 4b: Update TaskLoop** — add `Budget *Budget` field to `TaskLoopConfig`. In `RunTaskLoop`, check `budget.Exceeded()` before each task execution; if exceeded, mark remaining tasks as "blocked" and return.

- **Step 4c: Update SpecLoop** — add `Budget *Budget` field to `SpecLoopConfig`. In `SpecLoop.Run`, check `budget.HardBudgetExceeded()` (cost/time) between stage executions; if exceeded, set `rs.Status = StatusBlocked`, `rs.TerminalReason = "budget_exceeded"`, emit `budget_exceeded` event, and return. After the cycle loop, check `budget.CyclesExhausted()`; if exhausted with remaining failures, set `rs.Status = StatusNeedsHuman`, `rs.TerminalReason = "cycles_exhausted"` (C1).

- **Step 5:** Commit

---

## Phase 4: Evidence and Finalize

### Task 30: `evidence/` — Bundle assembly (evidence dir creation)

- **Files:**
  - Create: `internal/next/evidence/bundle.go`
  - Create: `internal/next/evidence/bundle_test.go`

- **Step 1: Write failing test:**
```go
package evidence

import "testing"

func TestAssembleBundle_CreatesEvidenceDir(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	err := b.Init()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("evidence dir should exist")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `Bundler` struct with `Init()` that creates evidence directory
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 31: `evidence/` — Task results summary (task-results.json)

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify: `internal/next/evidence/bundle_test.go`

- **Step 1: Write failing test:**
```go
func TestBundler_WriteTaskResults(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "done", Attempts: 1},
		{TaskID: "t-002", Status: "done", Attempts: 2},
	}
	err := b.WriteTaskResults(tasks)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "task-results.json"))
	if !strings.Contains(string(data), "t-001") {
		t.Fatal("task-results.json should contain task IDs")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `WriteTaskResults` — marshal tasks to `task-results.json`
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 32: `evidence/` — Validation summary (validation.json)

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify: `internal/next/evidence/bundle_test.go`

- **Step 1: Write failing test:**
```go
func TestBundler_WriteValidation(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()

	result := validator.FinalResult{Pass: true}
	err := b.WriteValidation(result)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "validation.json"))
	if !strings.Contains(string(data), `"pass":true`) {
		t.Fatal("validation.json should contain pass status")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `WriteValidation` — marshal to `validation.json`
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 33: `evidence/` — Metrics and diff summary

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify: `internal/next/evidence/bundle_test.go`

- **Step 1: Write failing test:**
```go
func TestBundler_WriteMetrics(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()

	m := Metrics{
		TotalTokens:  5000,
		TotalCostUSD: 1.23,
		TotalTasks:   3,
		PassedTasks:  2,
		FailedTasks:  1,
		DurationMs:   45000,
		Cycles:       1,
		Invocations: []InvocationRecord{
			{Phase: "plan", Tier: "high", Model: "opus", TokensIn: 2000, TokensOut: 1000, DurationMs: 15000, Success: true},
			{Phase: "execute", Tier: "medium", Model: "sonnet", TokensIn: 1500, TokensOut: 500, DurationMs: 30000, Success: false},
		},
	}
	err := b.WriteMetrics(m)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "metrics.json"))
	if !strings.Contains(string(data), "5000") {
		t.Fatal("metrics.json should contain token count")
	}
}

func TestBundler_WriteDiffSummary(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()

	err := b.WriteDiffSummary("3 files changed, 120 insertions, 5 deletions")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "diff-summary.md"))
	if !strings.Contains(string(data), "120 insertions") {
		t.Fatal("diff-summary should contain stats")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type InvocationRecord struct {
	Phase      string  `json:"phase"`       // "plan", "execute", "validate"
	Tier       string  `json:"tier"`        // "high", "medium", "low"
	Model      string  `json:"model"`       // "opus", "sonnet", "haiku"
	TokensIn   int     `json:"tokens_in"`
	TokensOut  int     `json:"tokens_out"`
	DurationMs int64   `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
	Success    bool    `json:"success"`
}

type Metrics struct {
	TotalTokens       int                `json:"total_tokens"`
	TotalCostUSD      float64            `json:"total_cost_usd"`
	TotalTasks        int                `json:"total_tasks"`
	PassedTasks       int                `json:"passed_tasks"`
	FailedTasks       int                `json:"failed_tasks"`
	DurationMs        int64              `json:"duration_ms"`
	Cycles            int                `json:"cycles"`
	TotalRetries      int                `json:"total_retries"`
	TotalReplans      int                `json:"total_replans"`
	HumanIntervention bool               `json:"human_intervention"`
	Invocations       []InvocationRecord `json:"invocations"`
}
```
Implement `WriteMetrics`, `WriteDiffSummary`
- **Step 4:** PASS
- **Step 4a (I5):** Add `NormalizeNilFields()` to `Metrics` (maps nil `Invocations` to `[]InvocationRecord{}`). Exported since `Metrics` is cross-package.
- **Step 5:** Commit

---

### Task 34: `evidence/` — Summary markdown generation

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify: `internal/next/evidence/bundle_test.go`

- **Step 1: Write failing test:**
```go
func TestBundler_WriteSummary(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()

	err := b.WriteSummary(SummaryInput{
		SpecID:    "spec-001",
		Status:    "ready_for_review",
		TaskCount: 3,
		PassCount: 3,
		Cycles:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
	if !strings.Contains(string(data), "spec-001") {
		t.Fatal("summary should contain spec ID")
	}
	if !strings.Contains(string(data), "ready_for_review") {
		t.Fatal("summary should contain terminal status")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** `WriteSummary` — markdown template with status, task counts, cycle info
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 35: `evidence/` — Review decision sheet (review.md)

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify: `internal/next/evidence/bundle_test.go`

- **Step 1: Write failing test:**
```go
func TestBundler_WriteReview(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()

	err := b.WriteReview(ReviewInput{
		TerminalState:     "ready_for_review",
		WhatChanged:       "Implemented parser package with 3 files",
		CycleHistory:      []CycleRecord{{Cycle: 1, TaskCount: 3, PassCount: 3}},
		ValidationResults: "All 3 checks passed",
		KnownRisks:        []string{"No error handling for malformed input"},
		RecommendedAction: "Merge after manual review of edge cases",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "review.md"))
	content := string(data)
	if !strings.Contains(content, "ready_for_review") {
		t.Fatal("review.md should contain terminal state")
	}
	if !strings.Contains(content, "Recommended Action") {
		t.Fatal("review.md should contain recommended action section")
	}
	if !strings.Contains(content, "Known Risks") {
		t.Fatal("review.md should contain known risks section")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
type ReviewInput struct {
	TerminalState     string        `json:"terminal_state"`
	WhatChanged       string        `json:"what_changed"`
	CycleHistory      []CycleRecord `json:"cycle_history"`
	ValidationResults string        `json:"validation_results"`
	KnownRisks        []string      `json:"known_risks"`
	RecommendedAction string        `json:"recommended_action"`
}

type CycleRecord struct {
	Cycle     int `json:"cycle"`
	TaskCount int `json:"task_count"`
	PassCount int `json:"pass_count"`
}

func (b *Bundler) WriteReview(input ReviewInput) error {
	// Generate markdown decision sheet with sections:
	// - Terminal State
	// - What Changed
	// - Cycle History (table)
	// - Validation Results
	// - Known Risks (bulleted list)
	// - Recommended Action
	// Write to review.md
}
```
- **Step 4:** PASS
- **Step 5:** Commit `"feat(next): add review.md decision sheet to evidence bundle"`

---

## Phase 5: Individual Stages

### Task 36: InitStage

- **Files:**
  - Create: `internal/next/specloop/stages/init.go`
  - Create: `internal/next/specloop/stages/init_test.go`

- **Step 1: Write failing test:**
```go
package stages

import "testing"

func TestInitStage_CreatesRunDir(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	stage := NewInitStage(store, execpolicy.DefaultPolicy(), InitStageConfig{
		SpecPath:   "/tmp/specs/spec-001.md",
		PolicyPath: "/tmp/execution-policy.json",
		RepoDir:    dir,
	})

	rs := runstore.NewRunState("spec-1", "proj-1")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("want Continue, got %v", action.Kind)
	}
	// Verify run dir was created
	if _, err := os.Stat(store.RunDir(rs.RunID)); os.IsNotExist(err) {
		t.Fatal("run dir should exist after init")
	}
}

func TestInitStage_CreatesWorktreeWithCorrectBranch(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	gitOps := &fakeGitOps{}
	stage := NewInitStage(store, execpolicy.DefaultPolicy(), InitStageConfig{
		SpecPath:   "/tmp/specs/spec-001.md",
		PolicyPath: "/tmp/execution-policy.json",
		RepoDir:    dir,
		GitOps:     gitOps,
	})

	rs := runstore.NewRunState("spec-001", "proj-1")
	stage.Run(context.Background(), rs)

	expectedBranch := fmt.Sprintf("gromit/spec-%s-%s", rs.SpecID, rs.RunID)
	if gitOps.createdBranch != expectedBranch {
		t.Fatalf("want branch %s, got %s", expectedBranch, gitOps.createdBranch)
	}
	if gitOps.worktreePath == "" {
		t.Fatal("worktree path should be set")
	}
}

func TestInitStage_CopiesSpecIntoRunDir(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "spec-001.md")
	os.WriteFile(specPath, []byte("# My Spec"), 0644)

	stage := NewInitStage(store, execpolicy.DefaultPolicy(), InitStageConfig{
		SpecPath: specPath,
		RepoDir:  dir,
	})

	rs := runstore.NewRunState("spec-001", "proj-1")
	stage.Run(context.Background(), rs)

	copied, err := os.ReadFile(filepath.Join(store.RunDir(rs.RunID), "spec.md"))
	if err != nil {
		t.Fatal("spec.md should be copied into run dir")
	}
	if string(copied) != "# My Spec" {
		t.Fatal("spec.md content mismatch")
	}
}

func TestInitStage_SnapshotsPolicyIntoRunDir(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	policyDir := t.TempDir()
	policyPath := filepath.Join(policyDir, "execution-policy.json")
	os.WriteFile(policyPath, []byte(`{"budgets":{"max_spec_cycles":5}}`), 0644)

	stage := NewInitStage(store, execpolicy.DefaultPolicy(), InitStageConfig{
		PolicyPath: policyPath,
		RepoDir:    dir,
	})

	rs := runstore.NewRunState("spec-001", "proj-1")
	stage.Run(context.Background(), rs)

	snapshotted, err := os.ReadFile(filepath.Join(store.RunDir(rs.RunID), "execution-policy.json"))
	if err != nil {
		t.Fatal("execution-policy.json should be snapshotted into run dir")
	}
	if !strings.Contains(string(snapshotted), "max_spec_cycles") {
		t.Fatal("policy snapshot content mismatch")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — creates git worktree with branch `gromit/spec-<spec-id>-<run-id>`, creates run directory, copies spec file into run dir as `spec.md`, snapshots execution policy into run dir as `execution-policy.json`, writes initial `run.json`, sets `rs.WorktreePath`, emits `run_started` event. **Note:** The source policy is at `<project-cell>/policy/execution.json` and the snapshot is written as `execution-policy.json` in the run directory.
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 37: CompileStage

- **Files:**
  - Create: `internal/next/specloop/stages/compile.go`
  - Create: `internal/next/specloop/stages/compile_test.go`

- **Step 1: Write failing test:**
```go
func TestCompileStage_WritesSpecPacket(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	compiler := &fakeCompiler{output: "compiled spec packet content"}
	stage := NewCompileStage(store, compiler)

	rs := runstore.NewRunState("spec-1", "proj-1")
	store.Save(rs)
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatal("expected continue")
	}

	data, _ := os.ReadFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"))
	if string(data) != "compiled spec packet content" {
		t.Fatal("spec packet not written")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — uses `contextpkt.Compiler` (or interface) to compile spec packet, writes to run dir, emits `spec_packet_compiled` event
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 38: PlanStage

- **Files:**
  - Create: `internal/next/specloop/stages/plan.go`
  - Create: `internal/next/specloop/stages/plan_test.go`

- **Step 1: Write failing test:**
```go
func TestPlanStage_CreatesPlanAndTasks(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	plannerAgent := &fakePlanner{plan: planner.Plan{
		Tasks: []planner.TaskDef{
			{TaskID: "t-001", Objective: "do thing", ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"}},
		},
	}}
	stage := NewPlanStage(store, plannerAgent)

	rs := runstore.NewRunState("spec-1", "proj-1")
	store.Save(rs)
	// Write a spec-packet so PlanStage can read it
	os.WriteFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"), []byte("spec"), 0644)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatal("expected continue")
	}
	if len(rs.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(rs.Tasks))
	}
}
```

- **Step 2:** FAIL
- **Step 2a: Write plan validation retry test:**
```go
func TestPlanStage_RetryOnPlanValidationFailure(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	callCount := 0
	plannerAgent := &fakePlanner{planFn: func() planner.Plan {
		callCount++
		if callCount == 1 {
			// First attempt: invalid plan (missing proof checks)
			return planner.Plan{Tasks: []planner.TaskDef{
				{TaskID: "t-001", Objective: "do thing", ExpectedTouchedArea: []string{"a/"}},
			}}
		}
		// Second attempt: valid plan
		return planner.Plan{Tasks: []planner.TaskDef{
			{TaskID: "t-001", Objective: "do thing", ExpectedTouchedArea: []string{"a/"}, ProofChecks: []string{"true"}},
		}}
	}}
	stage := NewPlanStage(store, plannerAgent)

	rs := runstore.NewRunState("spec-1", "proj-1")
	store.Save(rs)
	os.WriteFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"), []byte("spec"), 0644)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatal("expected continue after retry succeeds")
	}
	if callCount != 2 {
		t.Fatalf("planner should be called twice, got %d", callCount)
	}
}

func TestPlanStage_BothRetriesFail_Blocked(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	plannerAgent := &fakePlanner{plan: planner.Plan{Tasks: []planner.TaskDef{
		{TaskID: "t-001", Objective: "do thing"}, // always invalid
	}}}
	stage := NewPlanStage(store, plannerAgent)

	rs := runstore.NewRunState("spec-1", "proj-1")
	store.Save(rs)
	os.WriteFile(filepath.Join(store.RunDir(rs.RunID), "spec-packet.md"), []byte("spec"), 0644)

	action, _ := stage.Run(context.Background(), rs)
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked when both retries fail, got %v", action.Kind)
	}
}
```

- **Step 3: Implement** — reads spec-packet, invokes planner, validates plan. On validation failure, retry once with validation errors included in prompt. If retry also fails, return Blocked. On success, writes `plan.md` + `tasks.json`, populates `rs.Tasks`, emits `plan_created` event, and emits `TaskCreatedEvent` for each task (C3):
```go
// After populating rs.Tasks from plan:
for _, task := range rs.Tasks {
	eventLog.Append(TaskCreatedEvent{
		BaseEvent: BaseEvent{Type: "task_created", Timestamp: time.Now()},
		TaskID:    task.TaskID,
		Objective: task.Objective,
	})
}
```
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 39: ExecuteStage

- **Files:**
  - Create: `internal/next/specloop/stages/execute.go`
  - Create: `internal/next/specloop/stages/execute_test.go`

- **Step 1: Write failing test:**
```go
func TestExecuteStage_RunsTaskLoop(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	exec := &fakeExecutor{result: executor.RunTaskResult{AgentOutput: "done"}}
	val := &fakeValidator{pass: true}
	stage := NewExecuteStage(store, exec, val, execpolicy.DefaultPolicy())

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "pending", ProofChecks: []string{"true"}}}
	store.Save(rs)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatal("expected continue")
	}
	if rs.Tasks[0].Status != "done" {
		t.Fatalf("want done, got %s", rs.Tasks[0].Status)
	}
}
```

- **Step 2:** FAIL
- **Step 2a: Write all-tasks-failed and partial-failure tests:**
```go
func TestExecuteStage_AllTasksFailed_NeedsHuman(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	exec := &fakeExecutor{result: executor.RunTaskResult{AgentOutput: "failed"}}
	val := &fakeValidator{pass: false}
	stage := NewExecuteStage(store, exec, val, execpolicy.DefaultPolicy())

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", ProofChecks: []string{"false"}},
		{TaskID: "t-002", Status: "pending", ProofChecks: []string{"false"}},
	}
	store.Save(rs)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.NeedsHuman {
		t.Fatalf("all tasks failed should yield NeedsHuman, got %v", action.Kind)
	}
}

func TestExecuteStage_PartialFailure_Continue(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	callCount := 0
	exec := &fakeExecutor{resultFn: func() executor.RunTaskResult {
		callCount++
		return executor.RunTaskResult{AgentOutput: "output"}
	}}
	val := &fakeValidator{passFn: func(id string) bool { return id == "t-001" }}
	stage := NewExecuteStage(store, exec, val, execpolicy.DefaultPolicy())

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "pending", ProofChecks: []string{"true"}},
		{TaskID: "t-002", Status: "pending", ProofChecks: []string{"false"}},
	}
	store.Save(rs)

	action, _ := stage.Run(context.Background(), rs)
	if action.Kind != specloop.Continue {
		t.Fatalf("partial failure should continue (for fix cycle), got %v", action.Kind)
	}
}
```

- **Step 3: Implement** — compiles task packets, runs TaskLoop, updates task statuses in RunState, emits task events. If ALL tasks failed, return `NeedsHuman` action. If partial failure, return `Continue` (fix cycle will handle remaining failures)
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 40: ValidateStage

- **Files:**
  - Create: `internal/next/specloop/stages/validate.go`
  - Create: `internal/next/specloop/stages/validate_test.go`

- **Step 1: Write failing test:**
```go
func TestValidateStage_AllPass_Continue(t *testing.T) {
	val := &fakeValidator{finalResult: validator.FinalResult{Pass: true}}
	stage := NewValidateStage(val, execpolicy.DefaultPolicy())

	rs := runstore.NewRunState("spec-1", "proj-1")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatal("expected continue on pass")
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed=true after all checks pass")
	}
}

func TestValidateStage_Failure_ReplanFrom(t *testing.T) {
	val := &fakeValidator{finalResult: validator.FinalResult{Pass: false}}
	stage := NewValidateStage(val, execpolicy.DefaultPolicy())

	rs := runstore.NewRunState("spec-1", "proj-1")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected failure context")
	}
	if rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed=false after validation failure")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — runs final validation. On pass, set `rs.FinalValidationPassed = true` and return Continue. On failure, ensure `rs.FinalValidationPassed` stays false and return ReplanFrom with FailureContext
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 41: EvidenceStage

- **Files:**
  - Create: `internal/next/specloop/stages/evidence.go`
  - Create: `internal/next/specloop/stages/evidence_test.go`

- **Step 1: Write failing test:**
```go
func TestEvidenceStage_AssemblesBundle(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	stage := NewEvidenceStage(store)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done", Attempts: 1}}
	store.Save(rs)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatal("expected continue")
	}
	// Verify evidence files exist
	evidenceDir := store.EvidenceDir(rs.RunID)
	if _, err := os.Stat(filepath.Join(evidenceDir, "task-results.json")); os.IsNotExist(err) {
		t.Fatal("task-results.json should exist")
	}
	if _, err := os.Stat(filepath.Join(evidenceDir, "review.md")); os.IsNotExist(err) {
		t.Fatal("review.md should exist")
	}
	if _, err := os.Stat(filepath.Join(evidenceDir, "summary.md")); os.IsNotExist(err) {
		t.Fatal("summary.md should exist")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — creates bundler, writes task-results, validation, metrics, diff-summary, summary
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 42: FinalizeStage

- **Files:**
  - Create: `internal/next/specloop/stages/finalize.go`
  - Create: `internal/next/specloop/stages/finalize_test.go`

- **Step 1: Write failing test:**
```go
func TestFinalizeStage_SetsReadyForReview(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	stage := NewFinalizeStage(store)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.FinalValidationPassed = true // (I4) must check validation, not just tasks
	store.Save(rs)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != specloop.Continue {
		t.Fatal("expected continue")
	}
	if rs.Status != runstore.StatusReadyForReview {
		t.Fatalf("want ready_for_review, got %s", rs.Status)
	}
}

// (I4) All tasks passed but final validation failed → needs_human, NOT ready_for_review
func TestFinalizeStage_AllTasksDoneButValidationFailed_NeedsHuman(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	stage := NewFinalizeStage(store)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.FinalValidationPassed = false
	store.Save(rs)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("all tasks done but validation failed: want needs_human, got %s", rs.Status)
	}
}

func TestFinalizeStage_SetsNeedsHumanWhenTasksFailed(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	stage := NewFinalizeStage(store)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "failed"}}
	store.Save(rs)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("want needs_human, got %s", rs.Status)
	}
}
```

- **Step 2:** FAIL
- **Step 2a: Write worktree cleanup tests:**
```go
func TestFinalizeStage_PreservesWorktreeForReadyForReview(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	gitOps := &fakeGitOps{}
	stage := NewFinalizeStage(store, gitOps)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.WorktreePath = "/tmp/worktree-1"
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	store.Save(rs)

	stage.Run(context.Background(), rs)
	if gitOps.removedWorktree {
		t.Fatal("worktree should be preserved for ready_for_review")
	}
}

func TestFinalizeStage_PreservesWorktreeForNeedsHuman(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	gitOps := &fakeGitOps{}
	stage := NewFinalizeStage(store, gitOps)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.WorktreePath = "/tmp/worktree-1"
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "failed"}}
	store.Save(rs)

	stage.Run(context.Background(), rs)
	if gitOps.removedWorktree {
		t.Fatal("worktree should be preserved for needs_human")
	}
}

func TestFinalizeStage_CleansWorktreeForBlocked(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	gitOps := &fakeGitOps{}
	stage := NewFinalizeStage(store, gitOps)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.WorktreePath = "/tmp/worktree-1"
	rs.Status = runstore.StatusBlocked
	rs.Tasks = []runstore.Task{}
	store.Save(rs)

	stage.Run(context.Background(), rs)
	if !gitOps.removedWorktree {
		t.Fatal("worktree should be cleaned for blocked")
	}
}

func TestFinalizeStage_RecordsWorktreePathInRunJSON(t *testing.T) {
	store := runstore.NewStore(t.TempDir())
	stage := NewFinalizeStage(store, nil)

	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.WorktreePath = "/tmp/worktree-1"
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	store.Save(rs)

	stage.Run(context.Background(), rs)

	loaded, _ := store.Get(rs.RunID)
	if loaded.WorktreePath != "/tmp/worktree-1" {
		t.Fatal("worktree path should be recorded in run.json")
	}
}
```

- **Step 3: Implement** — (I4) checks BOTH final validation result AND task statuses: all tasks done AND final validation passed -> `ready_for_review`; otherwise -> `needs_human`. A run where all tasks passed but final validation failed must NOT be `ready_for_review`. FinalizeStage receives the `FinalResult` from ValidateStage (passed via RunState or stage context) and uses `FinalResult.Pass` as the primary gate. Preserves worktree for `ready_for_review` and `needs_human`; cleans worktree for `blocked`. Records `WorktreePath` in run.json. Saves final run state, emits `terminal_state` event.
- **Step 4:** PASS
- **Step 5:** Commit

---

## Phase 6: CLI Commands

### Task 43: `exec spec` command

- **Files:**
  - Modify: `cmd/gromit-next/main.go` (add exec subcommand)
  - Create: `cmd/gromit-next/exec.go`

- **Step 1: Write failing test:**
```go
package main

import "testing"

func TestExecCmd_RequiresSpecFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--project", "my-project"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --spec flag provided")
	}
}

func TestExecCmd_RequiresProjectFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --project flag provided")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execution commands",
}

var execSpecCmd = &cobra.Command{
	Use:   "spec",
	Short: "Execute a spec through the full pipeline",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		specPath, _ := cmd.Flags().GetString("spec")
		projectName, _ := cmd.Flags().GetString("project")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		policyPath, _ := cmd.Flags().GetString("policy")

		// 1. Resolve project store
		// 2. Load execution policy
		// 3. Build stage pipeline
		// 4. Create RunState
		// 5. Run SpecLoop
		// 6. Print terminal status
		_, _ = specPath, projectName
		_, _ = dryRun, policyPath
		return nil // stub for now, wired in integration
	},
}

func init() {
	execSpecCmd.Flags().String("spec", "", "path to spec markdown file (required)")
	execSpecCmd.Flags().String("project", "", "project name (required)")
	execSpecCmd.Flags().String("policy", "", "execution policy JSON path")
	execSpecCmd.Flags().Bool("dry-run", false, "compile plan but do not execute")
	execSpecCmd.MarkFlagRequired("spec")
	execSpecCmd.MarkFlagRequired("project")
	execCmd.AddCommand(execSpecCmd)
}
```

- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 44: `exec show` command

- **Files:**
  - Create: `cmd/gromit-next/exec_show.go`

- **Step 1: Write failing test:**
```go
func TestExecShowCmd_RequiresRunID(t *testing.T) {
	cmd := newExecShowCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no run ID provided")
	}
}

func TestExecShowCmd_LatestResolvesToMostRecentRun(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	rs1 := runstore.NewRunState("spec-1", "proj-1")
	rs1.StartedAt = time.Now().Add(-1 * time.Hour)
	store.Save(rs1)
	rs2 := runstore.NewRunState("spec-2", "proj-1")
	rs2.StartedAt = time.Now()
	store.Save(rs2)

	resolved, err := resolveRunID("latest", "proj-1", store)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != rs2.RunID {
		t.Fatalf("want latest run %s, got %s", rs2.RunID, resolved)
	}
}

func TestExecShowCmd_ProjectFlag(t *testing.T) {
	cmd := newExecShowCmd()
	cmd.SetArgs([]string{"latest", "--project", "my-project"})
	// Should not error on missing project flag when provided
	// (actual store lookup tested separately)
}

func TestExecShowCmd_FullFlag_ShowsEvidenceBundle(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewStore(dir)
	rs := runstore.NewRunState("spec-1", "proj-1")
	store.Save(rs)
	// Write evidence files
	evidenceDir := store.EvidenceDir(rs.RunID)
	os.MkdirAll(evidenceDir, 0755)
	os.WriteFile(filepath.Join(evidenceDir, "summary.md"), []byte("# Summary"), 0644)
	os.WriteFile(filepath.Join(evidenceDir, "review.md"), []byte("# Review"), 0644)

	output, err := execShow(rs.RunID, store, true /* full */)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "# Summary") {
		t.Fatal("--full should include summary.md content")
	}
	if !strings.Contains(output, "# Review") {
		t.Fatal("--full should include review.md content")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — `exec show [run-id|latest]` loads RunState from store, prints status, tasks, evidence path. Supports `latest` keyword that resolves to most recent run for `--project`. Add `--project` flag for project scoping and `--full` flag that prints the complete evidence bundle (summary.md, review.md, metrics.json, etc.)
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 45: `exec list` command

- **Files:**
  - Create: `cmd/gromit-next/exec_list.go`

- **Step 1: Write failing test:**
```go
func TestExecListCmd_RequiresProjectFlag(t *testing.T) {
	cmd := newExecListCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no project flag")
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — `exec list --project <name>` lists runs from store, prints table (run-id, spec, status, started)
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 46: `spec list` command

- **Files:**
  - Create: `cmd/gromit-next/spec.go`

- **Step 1: Write failing test:**
```go
func TestSpecListCmd_Exists(t *testing.T) {
	cmd := newSpecListCmd()
	if cmd.Use != "list" {
		t.Fatalf("unexpected Use: %s", cmd.Use)
	}
}

func TestSpecDiscovery_FindsSpecsFromSpecsDir(t *testing.T) {
	dir := t.TempDir()
	specsDir := filepath.Join(dir, "specs")
	os.MkdirAll(specsDir, 0755)
	os.WriteFile(filepath.Join(specsDir, "spec-001.md"), []byte("# Spec 1"), 0644)
	os.WriteFile(filepath.Join(specsDir, "spec-002.md"), []byte("# Spec 2"), 0644)

	specs, err := DiscoverSpecs(specsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("want 2 specs, got %d", len(specs))
	}
}

// NOTE (M1): In Spec 0002a scope, `completed` requires human acceptance (Spec 0002b),
// so no spec can reach `completed` yet. The best status achievable is `ready_for_review`.
// This test documents the current behavior; `completed` will be added when 0002b ships.
func TestSpecStatus_ReadyForReview(t *testing.T) {
	runs := []runstore.RunState{{Status: "ready_for_review"}}
	status := DeriveSpecStatus("spec-001", runs)
	if status != "ready_for_review" {
		t.Fatalf("want ready_for_review, got %s", status)
	}
}

func TestSpecStatus_Running(t *testing.T) {
	runs := []runstore.RunState{{Status: "running"}}
	status := DeriveSpecStatus("spec-001", runs)
	if status != "running" {
		t.Fatalf("want running, got %s", status)
	}
}

func TestSpecStatus_NeedsAttention(t *testing.T) {
	runs := []runstore.RunState{{Status: "needs_human"}}
	status := DeriveSpecStatus("spec-001", runs)
	if status != "needs_attention" {
		t.Fatalf("want needs_attention, got %s", status)
	}
}

func TestSpecStatus_Ready(t *testing.T) {
	// Spec exists but has no runs
	status := DeriveSpecStatus("spec-001", nil)
	if status != "ready" {
		t.Fatalf("want ready, got %s", status)
	}
}

func TestSpecStatus_Draft(t *testing.T) {
	// Spec file starts with "DRAFT" marker
	status := DeriveSpecStatusFromContent("spec-001", nil, "DRAFT: # Spec 1")
	if status != "draft" {
		t.Fatalf("want draft, got %s", status)
	}
}

func TestSpecsDir_ReadFromProjectJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "project.json"), []byte(`{"specs_dir":"my-specs"}`), 0644)

	cfg, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpecsDir != "my-specs" {
		t.Fatalf("want my-specs, got %s", cfg.SpecsDir)
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — `spec list --project <name>` reads `specs_dir` from `project.json`, scans for `.md` spec files, derives status for each spec from run history using 5 states: `ready_for_review` (has ready_for_review run — NOTE (M1): `completed` requires human acceptance from Spec 0002b, so is not reachable in 0002a scope), `running` (has running run), `needs_attention` (has needs_human or blocked run), `ready` (no runs yet), `draft` (content starts with DRAFT marker). Prints table with spec ID, status, last run info.
- **Step 4:** PASS
- **Step 5:** Commit

---

### Task 47: `--dry-run` flag wiring

- **Files:**
  - Modify: `cmd/gromit-next/exec.go`
  - Create: `cmd/gromit-next/exec_test.go` (if not already created in Task 43)

- **Step 1: Write failing test:**
```go
func TestExecSpec_DryRun_StopsAfterPlan(t *testing.T) {
	// Integration-style test: verify that with --dry-run, only Init+Compile+Plan stages run
	// Use a stage recorder to track which stages executed
	recorder := &stageRecorder{}
	// Build pipeline with recorder stages
	// Assert execute/validate/evidence/finalize are NOT called
}
```

- **Step 2:** FAIL
- **Step 3: Implement** — when `--dry-run` is set, build SpecLoop with only Init, Compile, Plan stages. Print plan JSON to stdout instead of executing.
- **Step 4:** PASS
- **Step 5:** Commit

---

## Phase 7: Provider Extension

### Task 48: Add `xhigh` tier to provider

- **Files:**
  - Modify: `internal/provider/provider.go`
  - Modify: `internal/provider/provider_test.go`

- **Step 1: Write failing test:**
```go
func TestTierFromLegacyModel_XHigh(t *testing.T) {
	// No model maps to xhigh yet, but xhigh should be a valid tier constant
	if TierXHigh != "xhigh" {
		t.Fatalf("want xhigh, got %s", TierXHigh)
	}
}

func TestTierToLegacyModel_XHigh(t *testing.T) {
	model := TierToLegacyModel(TierXHigh)
	if model != "opus" {
		t.Fatalf("xhigh should map to opus, got %s", model)
	}
}
```

- **Step 2:** FAIL
- **Step 3: Implement:**
```go
const TierXHigh = "xhigh"
```
Add to `tierToLegacyModel`: `TierXHigh: "opus"`. The xhigh tier uses the same backing model (opus) but signals maximum reasoning effort / extended thinking in the executor.

- **Step 4:** PASS
- **Step 5:** Commit

---

## Summary

| Phase | Tasks | Packages |
|-------|-------|----------|
| 1: Leaf | 1-13 | `execpolicy/`, `runstore/`, `validator/` |
| 2: Middle | 14-23 | `planner/`, `executor/` |
| 3: Orchestration | 24-29 | `specloop/` (core loop, taskloop, budget) |
| 4: Evidence | 30-35 | `evidence/` (includes review.md decision sheet) |
| 5: Stages | 36-42 | `specloop/stages/` (init, compile, plan, execute, validate, evidence, finalize) |
| 6: CLI | 43-47 | `cmd/gromit-next/` |
| 7: Provider | 48 | `internal/provider/` |

**Total: 48 tasks across 7 phases.**

Each task follows strict TDD: write failing test, verify red, implement, verify green, commit. Tasks are ordered by dependency — each phase only depends on packages from prior phases.

**Key integration points with existing code:**
- `internal/provider.Provider` — used by executor agent implementation
- `internal/next/contextpkt.Compiler` — used by CompileStage
- `internal/next/workspace.Root` — used by runstore for data directory resolution
- `internal/next/artifact.Store` — potential use for artifact I/O

**Event types emitted** (for `events.jsonl`): `run_started`, `spec_packet_compiled`, `plan_created`, `plan_validation_result`, `task_created`, `task_started`, `task_validation_result`, `task_completed`, `task_failed`, `task_needs_split`, `redecomposition_triggered`, `final_validation_result`, `replan_triggered`, `budget_exceeded`, `terminal_state`

**Terminal states:** `ready_for_review` (all tasks pass AND final validation passes), `needs_human` (task failures OR cycle exhaustion with remaining validation failures), `blocked` (infra failure OR cost/time budget_exceeded). Note: `completed` requires human acceptance (Spec 0002b) and is not reachable in 0002a scope.
