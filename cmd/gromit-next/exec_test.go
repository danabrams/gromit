package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/projectcell"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/workspace"
)

func TestExecCmd_RequiresSpecFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--project", "my-project"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --spec flag")
	}
}

func TestExecCmd_RequiresProjectFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --project flag")
	}
}

func TestExecCmd_AcceptsBothFlags_UsesRealProvider(t *testing.T) {
	storeDir := t.TempDir()
	cmd := newExecSpecCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md", "--project", "my-project", "--store-dir", storeDir})
	err := cmd.Execute()
	// The old defaultStageProvider returned "agent provider not configured".
	// With RealStageProvider wired in, the pipeline runs through (noop stages)
	// and produces a Run ID in output.
	if err != nil {
		if strings.Contains(err.Error(), "agent provider not configured") {
			t.Fatalf("still using old defaultStageProvider stub: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Run ID:") {
		t.Errorf("expected Run ID in output, got: %s", buf.String())
	}
}

func TestExecCmd_DryRunFlag(t *testing.T) {
	storeDir := t.TempDir()
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md", "--project", "my-project", "--dry-run", "--store-dir", storeDir})
	_ = cmd.Execute()
	// Verify dry-run flag was parsed correctly (execution may fail due to missing spec file).
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if !dryRun {
		t.Fatal("expected dry-run to be true")
	}
}

// testStageProvider returns the given stages for BuildStages.
type testStageProvider struct {
	stages []specloop.Stage
}

func (p *testStageProvider) BuildStages(_ execpolicy.Policy, _ *runstore.RunState, _ *specloop.Budget, _ *runstore.EventLog) ([]specloop.Stage, error) {
	return p.stages, nil
}

func TestExecSpec_RunsStagesViaPipeline(t *testing.T) {
	tmp := t.TempDir()

	var order []string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "init"},
			&stageRecorder{name: "compile"},
			&stageRecorder{name: "plan"},
			&stageRecorder{name: "execute"},
			&stageRecorder{name: "validate"},
			&stageRecorder{name: "evidence"},
			&stageRecorder{name: "finalize"},
		},
	}
	// Wire recorders to track order
	for _, s := range provider.stages {
		r := s.(*stageRecorder)
		r.orderPtr = &order
	}

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--spec", "test-spec.md", "--project", "test-proj", "--store-dir", tmp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Run ID:") {
		t.Fatal("expected output to contain 'Run ID:'")
	}
	if !strings.Contains(output, "Status:") {
		t.Fatal("expected output to contain 'Status:'")
	}

	// Verify all stages ran
	want := []string{"init", "compile", "plan", "execute", "validate", "evidence", "finalize"}
	if len(order) != len(want) {
		t.Fatalf("expected %d stages to run, got %d: %v", len(want), len(order), order)
	}
	for i, name := range want {
		if order[i] != name {
			t.Errorf("stage %d: want %s, got %s", i, name, order[i])
		}
	}
}

func TestExecSpec_DryRunOnlyRunsEarlyStages(t *testing.T) {
	tmp := t.TempDir()

	var order []string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "init"},
			&stageRecorder{name: "compile"},
			&stageRecorder{name: "plan"},
			&stageRecorder{name: "execute"},
			&stageRecorder{name: "validate"},
			&stageRecorder{name: "evidence"},
			&stageRecorder{name: "finalize"},
		},
	}
	for _, s := range provider.stages {
		r := s.(*stageRecorder)
		r.orderPtr = &order
	}

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--spec", "test-spec.md", "--project", "test-proj", "--store-dir", tmp, "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"init", "compile", "plan"}
	if len(order) != len(want) {
		t.Fatalf("expected %d stages in dry-run, got %d: %v", len(want), len(order), order)
	}
}

func TestExecSpec_SavesRunState(t *testing.T) {
	tmp := t.TempDir()

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "init"},
		},
	}

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--spec", "test-spec.md", "--project", "test-proj", "--store-dir", tmp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify run was persisted
	store := runstore.NewStore(tmp)
	runs, err := store.List("test-proj")
	if err != nil {
		t.Fatalf("unexpected error listing runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run saved, got %d", len(runs))
	}
}

// --- specIDFromPath tests ---

func TestSpecIDFromPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"./specs/add-refund-endpoint.md", "add-refund-endpoint"},
		{"specs/add-refund-endpoint.md", "add-refund-endpoint"},
		{"/absolute/path/to/my-spec.md", "my-spec"},
		{"add-refund-endpoint.md", "add-refund-endpoint"},
		{"add-refund-endpoint", "add-refund-endpoint"},
		{"my-spec.yaml", "my-spec"},
		{"../parent/spec-0002.md", "spec-0002"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := specIDFromPath(tt.input)
			if got != tt.want {
				t.Errorf("specIDFromPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExecSpec_SpecIDIsNormalized(t *testing.T) {
	tmp := t.TempDir()

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "init"},
		},
	}

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--spec", "./specs/add-refund-endpoint.md", "--project", "test-proj", "--store-dir", tmp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify persisted run has normalized SpecID (stem only).
	store := runstore.NewStore(tmp)
	runs, err := store.List("test-proj")
	if err != nil {
		t.Fatalf("unexpected error listing runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].SpecID != "add-refund-endpoint" {
		t.Errorf("SpecID = %q, want %q", runs[0].SpecID, "add-refund-endpoint")
	}
}

// --- Scenario 9: Routing Config Validation ---

// TestScenario_ExecSpec_InvalidRoutingRatio_RejectsBeforeRun is the Scenario 9
// (0002c/0002d) evidence test. It verifies that exec spec rejects a policy whose
// routing.ratio values do not sum to 100 before starting a run.
//
// RED: exec.go does not call policy.Validate() — command succeeds even with an
// invalid ratio. GREEN after: run() calls policy.Validate() and returns an error
// if validation fails, so no run state is created.
func TestScenario_ExecSpec_InvalidRoutingRatio_RejectsBeforeRun(t *testing.T) {
	dir := t.TempDir()

	// Write a policy whose routing.ratio sums to 90, not 100.
	badPolicy := `{
		"always_run": [{"name": "unit-tests", "command": "go test ./...", "type": "test"}],
		"budgets": {
			"max_spec_cycles": 3, "max_task_retries": 1, "max_redecomposition_passes": 1,
			"max_task_duration_seconds": 300, "max_run_duration_seconds": 3600, "max_run_cost_usd": 50.0
		},
		"models": {"planner": "high", "executor": "medium", "evaluator": "high"},
		"review": {"facets": ["spec_alignment"], "tiers": {"spec_alignment": "high"}, "replan_threshold": "warning"},
		"routing": {"preferences": {"plan": "any"}, "ratio": {"claude": 70, "codex": 20}, "cooldown_seconds": 300}
	}`
	policyPath := filepath.Join(dir, "bad-policy.json")
	if err := os.WriteFile(policyPath, []byte(badPolicy), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	storeDir := filepath.Join(dir, "store")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}

	provider := &testStageProvider{stages: []specloop.Stage{}}
	cmd := newExecSpecCmdWithProvider(provider)
	cmd.SetArgs([]string{
		"--spec", "test-spec.md",
		"--project", "fixture-calc",
		"--policy", policyPath,
		"--store-dir", storeDir,
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for routing.ratio not summing to 100, got nil")
	}
	if !strings.Contains(err.Error(), "ratio") && !strings.Contains(err.Error(), "100") {
		t.Errorf("expected error mentioning ratio/100, got: %v", err)
	}

	// No run should be created — validation must fire before run state is saved.
	store := runstore.NewStore(storeDir)
	runs, _ := store.List("fixture-calc")
	if len(runs) > 0 {
		t.Errorf("expected no runs created for invalid policy, got %d", len(runs))
	}
}

// TestScenario_ExecSpec_ValidRoutingRatio_Accepts verifies that a policy with
// routing.ratio summing to exactly 100 is accepted without error.
func TestScenario_ExecSpec_ValidRoutingRatio_Accepts(t *testing.T) {
	dir := t.TempDir()

	goodPolicy := `{
		"always_run": [{"name": "unit-tests", "command": "go test ./...", "type": "test"}],
		"budgets": {
			"max_spec_cycles": 3, "max_task_retries": 1, "max_redecomposition_passes": 1,
			"max_task_duration_seconds": 300, "max_run_duration_seconds": 3600, "max_run_cost_usd": 50.0
		},
		"models": {"planner": "high", "executor": "medium", "evaluator": "high"},
		"review": {"facets": ["spec_alignment"], "tiers": {"spec_alignment": "high"}, "replan_threshold": "warning"},
		"routing": {"preferences": {"plan": "any"}, "ratio": {"claude": 60, "codex": 40}, "cooldown_seconds": 300}
	}`
	policyPath := filepath.Join(dir, "good-policy.json")
	if err := os.WriteFile(policyPath, []byte(goodPolicy), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	storeDir := filepath.Join(dir, "store")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}

	provider := &testStageProvider{stages: []specloop.Stage{}}
	cmd := newExecSpecCmdWithProvider(provider)
	cmd.SetArgs([]string{
		"--spec", "test-spec.md",
		"--project", "fixture-calc",
		"--policy", policyPath,
		"--store-dir", storeDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error for valid routing policy: %v", err)
	}
}

// --- Task 44: exec show tests ---

func TestExecShowCmd_RequiresRunID(t *testing.T) {
	cmd := newExecShowCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no run-id arg provided")
	}
}

func TestExecShowCmd_LatestResolvesToMostRecentRun(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Create two runs with different start times.
	older := &runstore.RunState{
		RunID:     "run-older",
		SpecID:    "spec-001",
		ProjectID: "proj-a",
		Status:    runstore.StatusRunning,
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	newer := &runstore.RunState{
		RunID:     "run-newer",
		SpecID:    "spec-001",
		ProjectID: "proj-a",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	if err := store.Save(older); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(newer); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveRunID("latest", "proj-a", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "run-newer" {
		t.Fatalf("expected run-newer, got %s", resolved)
	}
}

func TestExecShowCmd_FullFlag_ShowsEvidenceBundle(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:     "run-evidence",
		SpecID:    "spec-001",
		ProjectID: "proj-a",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	// Create evidence files.
	evidenceDir := store.RunEvidenceDir("run-evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "summary.txt"), []byte("all tests passed"), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-evidence", store, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Evidence") {
		t.Fatal("expected output to contain Evidence section")
	}
	if !strings.Contains(output, "all tests passed") {
		t.Fatal("expected output to contain evidence content")
	}
}

func TestExecList_EmptyResults_ExitCodeZero(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	output, err := execList("nonexistent-project", store)
	if err != nil {
		t.Fatalf("execList returned error for empty results: %v", err)
	}
	// Should contain header but no data rows
	if !strings.Contains(output, "RUN ID") {
		t.Errorf("expected header row, got: %s", output)
	}
}

// --- Task 45: exec list tests ---

func TestExecListCmd_RequiresProjectFlag(t *testing.T) {
	cmd := newExecListCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --project flag")
	}
}

func TestExecListCmd_PrintsTable(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:     "run-list-test",
		SpecID:    "spec-001",
		ProjectID: "proj-b",
		Status:    runstore.StatusRunning,
		StartedAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execList("proj-b", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "RUN ID") {
		t.Fatal("expected table header with RUN ID")
	}
	if !strings.Contains(output, "run-list-test") {
		t.Fatal("expected output to contain run ID")
	}
}

// --- Task 47: dry-run stage filtering tests ---

// stageRecorder implements specloop.Stage and records when it was run.
type stageRecorder struct {
	name     string
	ran      bool
	orderPtr *[]string // optional: appends name when Run is called
}

func (s *stageRecorder) Name() string { return s.name }
func (s *stageRecorder) Run(_ context.Context, _ *runstore.RunState) (specloop.NextAction, error) {
	s.ran = true
	if s.orderPtr != nil {
		*s.orderPtr = append(*s.orderPtr, s.name)
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

func TestExecSpec_DryRun_StopsAfterPlan(t *testing.T) {
	allNames := []string{"init", "compile", "plan", "execute", "validate", "evidence", "finalize"}
	recorders := make(map[string]*stageRecorder, len(allNames))
	var allStages []specloop.Stage
	for _, name := range allNames {
		r := &stageRecorder{name: name}
		recorders[name] = r
		allStages = append(allStages, r)
	}

	filtered := filterStagesForDryRun(allStages, true)

	// Run the filtered stages via SpecLoop.
	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := specloop.NewSpecLoop(filtered, specloop.SpecLoopConfig{Budget: budget})
	rs := runstore.NewRunState("spec-test", "proj-test")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only init, compile, plan should have run.
	for _, name := range []string{"init", "compile", "plan"} {
		if !recorders[name].ran {
			t.Errorf("expected %s stage to run in dry-run mode", name)
		}
	}
	for _, name := range []string{"execute", "validate", "evidence", "finalize"} {
		if recorders[name].ran {
			t.Errorf("expected %s stage NOT to run in dry-run mode", name)
		}
	}
}

func TestExecSpec_NoDryRun_RunsAllStages(t *testing.T) {
	allNames := []string{"init", "compile", "plan", "execute", "validate", "evidence", "finalize"}
	recorders := make(map[string]*stageRecorder, len(allNames))
	var allStages []specloop.Stage
	for _, name := range allNames {
		r := &stageRecorder{name: name}
		recorders[name] = r
		allStages = append(allStages, r)
	}

	filtered := filterStagesForDryRun(allStages, false)

	budget2 := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := specloop.NewSpecLoop(filtered, specloop.SpecLoopConfig{Budget: budget2})
	rs := runstore.NewRunState("spec-test", "proj-test")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range allNames {
		if !recorders[name].ran {
			t.Errorf("expected %s stage to run when dry-run is false", name)
		}
	}
}

// TestResolveWorkDir_UsesProjectRepoPath verifies that when a project ID is given
// and a cell exists, resolveWorkDir returns the cell's RepoPath — not os.Getwd().
//
// RED: resolveWorkDir does not exist yet; exec.go uses os.Getwd() unconditionally.
// GREEN after: resolveWorkDir looks up cell.RepoPath when projectID is set.
func TestResolveWorkDir_UsesProjectRepoPath(t *testing.T) {
	wsRoot := t.TempDir()

	// Create a fake git repo directory for the project.
	repoDir := filepath.Join(t.TempDir(), "my-repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Register the project in the cell store.
	store := projectcell.NewFSStore(filepath.Join(wsRoot, "projects"))
	if _, err := store.Create("my-project", repoDir); err != nil {
		t.Fatalf("create cell: %v", err)
	}

	root := workspace.Root(wsRoot)
	got := resolveWorkDir("my-project", root)
	if got != repoDir {
		t.Errorf("resolveWorkDir = %q, want project RepoPath %q", got, repoDir)
	}
}

// TestResolveWorkDir_FallsBackToGetwd_WhenNoProject verifies that when no
// project ID is given, resolveWorkDir returns the current working directory.
func TestResolveWorkDir_FallsBackToGetwd_WhenNoProject(t *testing.T) {
	wsRoot := t.TempDir()
	root := workspace.Root(wsRoot)

	cwd, _ := os.Getwd()
	got := resolveWorkDir("", root)
	if got != cwd {
		t.Errorf("resolveWorkDir = %q, want cwd %q", got, cwd)
	}
}

// TestResolveWorkDir_FallsBackToGetwd_WhenProjectNotFound verifies that when
// the project ID is set but the cell doesn't exist, resolveWorkDir falls back
// to os.Getwd() rather than returning an empty string or erroring.
func TestResolveWorkDir_FallsBackToGetwd_WhenProjectNotFound(t *testing.T) {
	wsRoot := t.TempDir() // empty — no projects registered
	root := workspace.Root(wsRoot)

	cwd, _ := os.Getwd()
	got := resolveWorkDir("nonexistent-project", root)
	if got != cwd {
		t.Errorf("resolveWorkDir = %q, want cwd %q", got, cwd)
	}
}

func TestFilterStagesForDryRun_ExcludesReviewAndAccept(t *testing.T) {
	allNames := []string{"init", "compile", "plan", "execute", "validate", "review", "accept", "evidence", "finalize"}
	var allStages []specloop.Stage
	for _, name := range allNames {
		allStages = append(allStages, &stageRecorder{name: name})
	}

	filtered := filterStagesForDryRun(allStages, true)
	for _, s := range filtered {
		if s.Name() == "review" || s.Name() == "accept" {
			t.Errorf("dry-run should not include %q stage", s.Name())
		}
	}
	if len(filtered) != 3 {
		t.Errorf("expected 3 dry-run stages, got %d", len(filtered))
	}
}

// Test that exec show returns a friendly error for an unknown run ID.
func TestExecShowCmd_UnknownRunID_FriendlyError(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	output, err := execShow("nonexistent-run-id", store, false)
	if err == nil {
		t.Fatal("expected error for nonexistent run ID")
	}
	if output != "" {
		t.Fatalf("expected empty output, got %q", output)
	}
	if !strings.Contains(err.Error(), `run "nonexistent-run-id" not found`) {
		t.Fatalf("expected friendly 'not found' error, got: %v", err)
	}
}

// Verify exec show includes the new fields: Cycles, Duration, Tasks done count, Valid, Cost, Evidence path.
func TestExecShowCmd_ShowsExtendedFields(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Second)
	rs := &runstore.RunState{
		RunID:                 "run-extended",
		SpecID:                "spec-001",
		ProjectID:             "proj-a",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 3,
		StartedAt:             start,
		EndedAt:               end,
		AccumulatedCost:       0.1234,
		FinalValidationPassed: true,
		WorktreePath:          "/tmp/worktree-xyz",
		Tasks: []runstore.Task{
			{Status: "done"},
			{Status: "done"},
			{Status: "failed"},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-extended", store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Cycles", "Cycles: 3"},
		{"Duration", "Duration: 1m"},
		{"Tasks", "Tasks (3):"},
		{"Validation", "Validation: \u2713 passed"},
		{"Cost", "$0.12"},
		{"Worktree", "/tmp/worktree-xyz"},
		{"Evidence", store.RunEvidenceDir("run-extended")},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// Verify exec show command uses stdout properly.
func TestExecShowCmd_OutputToStdout(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:     "run-stdout",
		SpecID:    "spec-001",
		ProjectID: "proj-a",
		Status:    runstore.StatusRunning,
		StartedAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Tasks:     []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	cmd := newExecShowCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"run-stdout", "--store-dir", tmp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "run-stdout") {
		t.Fatal("expected output to contain run ID")
	}
}

// --- Scenario 16: Acceptance Fail Triggers Fix Cycle (CLI layer) ---

// seedEvidence creates evidence files directly in the store's evidence directory.
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

// TestScenario_ExecShow_AcceptanceFailFixCycle verifies that exec show correctly
// displays a run that completed via an acceptance-fail fix cycle (cycle 2,
// 1 replan, all three gates passed).
func TestScenario_ExecShow_AcceptanceFailFixCycle(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Seed: a run that went through a fix cycle.
	// Cycle 1: agent implemented Divide without godoc comment (proof checks failed).
	// Cycle 2: fix task added comment + zero-divisor guard, all gates pass → ready_for_review.
	rs := &runstore.RunState{
		RunID:                 "run-accept-fail",
		SpecID:                "divide-float64",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 2,
		TotalReplans:          1,
		StartedAt:             time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 12, 5, 0, 0, time.UTC),
		AccumulatedCost:       0.25,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Attempts: 1, FilesChanged: []string{"calc/calc.go"}},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-accept-fail", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Cycles", "Cycles: 2"},
		{"Status", "ready_for_review"},
		{"Cost", "$0.25"},
		{"Validation", "Validation: \u2713 passed"},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// TestScenario_ExecShow_Full_AcceptanceAllPass verifies exec show --full shows
// acceptance.json with all_pass: true for an acceptance-fail-fixed run.
func TestScenario_ExecShow_Full_AcceptanceAllPass(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:                 "run-accept-full",
		SpecID:                "divide-float64",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 2,
		TotalReplans:          1,
		StartedAt:             time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 12, 5, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	seedEvidence(t, store, "run-accept-full", map[string]string{
		"acceptance.json": `{"all_pass": true, "criteria": [{"description": "Divide(10, 2) returns 5.0", "result": "pass"}, {"description": "Divide(10, 3) returns ~3.333", "result": "pass"}]}`,
		"summary.md":      "# Execution Summary\n\n- **Status:** ready_for_review\n- **Cycles:** 2\n",
	})

	output, err := execShow("run-accept-full", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	if !strings.Contains(output, "acceptance.json") {
		t.Errorf("expected acceptance.json section in full output, got:\n%s", output)
	}
	if !strings.Contains(output, "all_pass") {
		t.Errorf("expected all_pass in acceptance.json, got:\n%s", output)
	}
	if strings.Contains(output, "Status: running") {
		t.Errorf("full output shows stale 'running' status:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review in full output, got:\n%s", output)
	}
}

// TestScenario_ExecList_ShowsAcceptanceFixCycleRun verifies exec list includes a
// run that completed via acceptance-fail fix cycle with correct status.
func TestScenario_ExecList_ShowsAcceptanceFixCycleRun(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	if err := store.Save(&runstore.RunState{
		RunID:                 "run-accept-list",
		SpecID:                "divide-float64",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 2,
		TotalReplans:          1,
		StartedAt:             time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	output, err := execList("fixture-calc", store)
	if err != nil {
		t.Fatalf("execList: %v", err)
	}

	if !strings.Contains(output, "run-accept-list") {
		t.Errorf("expected run-accept-list in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review in output, got:\n%s", output)
	}
}

// TestScenario_ExecShow_BudgetExhaustion_CyclesExhausted verifies that exec show
// displays a run that exhausted its cycle budget with needs_human status and the
// cycles_exhausted terminal reason.
func TestScenario_ExecShow_BudgetExhaustion_CyclesExhausted(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Seed: a run that hit max_spec_cycles=2 during review+acceptance fix cycles.
	// Review found errors on cycle 1 → replan → cycle 2 → acceptance still failing
	// → cycles exhausted → needs_human.
	rs := &runstore.RunState{
		RunID:           "run-budget-cycles",
		SpecID:          "unfixable-conflict",
		ProjectID:       "fixture-calc",
		Status:          runstore.StatusNeedsHuman,
		TerminalReason:  "cycles_exhausted",
		Cycle:           2,
		TotalReplans:    1,
		StartedAt:       time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2026, 3, 15, 14, 8, 0, 0, time.UTC),
		AccumulatedCost: 0.18,
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Attempts: 1, FilesChanged: []string{"calc/calc.go"}},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-budget-cycles", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Status", "needs_human"},
		{"Reason", "cycles_exhausted"},
		{"Cycles", "Cycles: 2"},
		{"Cost", "$0.18"},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// TestScenario_ExecShow_Full_BudgetExhaustion_AcceptanceFailed verifies that
// exec show --full displays acceptance.json (with a failing criterion) and
// review.json for a cycles-exhausted run.
func TestScenario_ExecShow_Full_BudgetExhaustion_AcceptanceFailed(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:          "run-budget-full",
		SpecID:         "unfixable-conflict",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusNeedsHuman,
		TerminalReason: "cycles_exhausted",
		Cycle:          2,
		TotalReplans:   1,
		StartedAt:      time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC),
		EndedAt:        time.Date(2026, 3, 15, 14, 8, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	seedEvidence(t, store, "run-budget-full", map[string]string{
		"acceptance.json": `{"all_pass": false, "criteria": [{"description": "No global mutable state", "result": "fail"}, {"description": "All functions documented", "result": "pass"}]}`,
		"review.json":     `{"findings": [{"facet": "spec_alignment", "severity": "error", "description": "Conflicting requirements unresolvable"}]}`,
		"metrics.json":    `{"total_cost_usd": 0.18, "cycles": 2}`,
	})

	output, err := execShow("run-budget-full", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	// Evidence bundle must include both acceptance.json and review.json
	if !strings.Contains(output, "acceptance.json") {
		t.Errorf("expected acceptance.json section in full output, got:\n%s", output)
	}
	if !strings.Contains(output, "review.json") {
		t.Errorf("expected review.json section in full output, got:\n%s", output)
	}
	// acceptance.json must show all_pass: false (at least one criterion failed)
	if !strings.Contains(output, `"all_pass": false`) {
		t.Errorf("expected all_pass: false in acceptance.json, got:\n%s", output)
	}
	// Must not show stale "running" status
	if strings.Contains(output, "Status: running") {
		t.Errorf("full output shows stale 'running' status:\n%s", output)
	}
}

// TestScenario_ExecList_BudgetExhaustion verifies exec list shows a
// cycles-exhausted run with needs_human status.
func TestScenario_ExecList_BudgetExhaustion(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	if err := store.Save(&runstore.RunState{
		RunID:          "run-budget-list",
		SpecID:         "unfixable-conflict",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusNeedsHuman,
		TerminalReason: "cycles_exhausted",
		Cycle:          2,
		TotalReplans:   1,
		StartedAt:      time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	output, err := execList("fixture-calc", store)
	if err != nil {
		t.Fatalf("execList: %v", err)
	}

	if !strings.Contains(output, "run-budget-list") {
		t.Errorf("expected run-budget-list in output, got:\n%s", output)
	}
	if !strings.Contains(output, "needs_human") {
		t.Errorf("expected needs_human in output, got:\n%s", output)
	}
}

// TestScenario_ExecShow_AcceptanceUnclear_CyclesExhausted verifies that exec show
// displays a run where acceptance criteria were unclear (not pass/fail), exhausted
// the cycle budget, and reached needs_human status.
func TestScenario_ExecShow_AcceptanceUnclear_CyclesExhausted(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Seed: a run that hit max_spec_cycles=2 with repeated unclear acceptance.
	// Cycle 1: acceptance criteria marked unclear → replan → cycle 2
	// Cycle 2: acceptance criteria still unclear → cycles exhausted → needs_human.
	rs := &runstore.RunState{
		RunID:           "run-acceptance-unclear",
		SpecID:          "subjective-criteria",
		ProjectID:       "fixture-calc",
		Status:          runstore.StatusNeedsHuman,
		TerminalReason:  "cycles_exhausted",
		Cycle:           2,
		TotalReplans:    1,
		StartedAt:       time.Date(2026, 3, 15, 15, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2026, 3, 15, 15, 10, 0, 0, time.UTC),
		AccumulatedCost: 0.24,
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Attempts: 1, FilesChanged: []string{"calc/calc.go"}},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-acceptance-unclear", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Status", "needs_human"},
		{"Reason", "cycles_exhausted"},
		{"Cycles", "Cycles: 2"},
		{"Cost", "$0.24"},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// TestScenario_ExecShow_Full_AcceptanceUnclear_CyclesExhausted verifies that
// exec show --full displays acceptance.json with unclear criteria and the
// evidence bundle for an unclear-acceptance-exhausted run.
func TestScenario_ExecShow_Full_AcceptanceUnclear_CyclesExhausted(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:          "run-unclear-full",
		SpecID:         "subjective-criteria",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusNeedsHuman,
		TerminalReason: "cycles_exhausted",
		Cycle:          2,
		TotalReplans:   1,
		StartedAt:      time.Date(2026, 3, 15, 15, 0, 0, 0, time.UTC),
		EndedAt:        time.Date(2026, 3, 15, 15, 10, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	seedEvidence(t, store, "run-unclear-full", map[string]string{
		"acceptance.json": `{"all_pass": false, "criteria": [{"description": "Code is maintainable and follows best practices", "result": "unclear"}, {"description": "Error messages are user-friendly and actionable", "result": "unclear"}]}`,
		"review.json":     `{"findings": []}`,
		"metrics.json":    `{"total_cost_usd": 0.24, "cycles": 2}`,
	})

	output, err := execShow("run-unclear-full", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	// Evidence bundle must include both acceptance.json and review.json
	if !strings.Contains(output, "acceptance.json") {
		t.Errorf("expected acceptance.json section in full output, got:\n%s", output)
	}
	if !strings.Contains(output, "review.json") {
		t.Errorf("expected review.json section in full output, got:\n%s", output)
	}
	// acceptance.json must show all_pass: false and at least one unclear result
	if !strings.Contains(output, `"all_pass": false`) {
		t.Errorf("expected all_pass: false in acceptance.json, got:\n%s", output)
	}
	if !strings.Contains(output, `"result": "unclear"`) {
		t.Errorf("expected at least one 'unclear' result in acceptance.json, got:\n%s", output)
	}
	// Must not show stale "running" status
	if strings.Contains(output, "Status: running") {
		t.Errorf("full output shows stale 'running' status:\n%s", output)
	}
}

// TestScenario_ExecList_AcceptanceUnclear verifies exec list shows an
// acceptance-unclear cycles-exhausted run with needs_human status.
func TestScenario_ExecList_AcceptanceUnclear(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	if err := store.Save(&runstore.RunState{
		RunID:          "run-unclear-list",
		SpecID:         "subjective-criteria",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusNeedsHuman,
		TerminalReason: "cycles_exhausted",
		Cycle:          2,
		TotalReplans:   1,
		StartedAt:      time.Date(2026, 3, 15, 15, 0, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	output, err := execList("fixture-calc", store)
	if err != nil {
		t.Fatalf("execList: %v", err)
	}

	if !strings.Contains(output, "run-unclear-list") {
		t.Errorf("expected run-unclear-list in output, got:\n%s", output)
	}
	if !strings.Contains(output, "needs_human") {
		t.Errorf("expected needs_human in output, got:\n%s", output)
	}
}

// --- Scenario 8b: Enable Additional Facet Via Config (logic_gaps) ---

// TestScenario_ExecShow_LogicGapsFacet verifies that exec show correctly
// displays a run where the policy enabled the logic_gaps review facet.
// The run completes successfully (ready_for_review) — config-only change.
func TestScenario_ExecShow_LogicGapsFacet(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Seed: a run that used logic_gaps facet in review config.
	// Cycle 1: agent implemented Subtract, all three gates passed → ready_for_review.
	// The logic_gaps facet ran and produced suggestion-level findings (non-blocking).
	rs := &runstore.RunState{
		RunID:                 "run-logic-gaps",
		SpecID:                "add-subtract",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 1,
		TotalReplans:          0,
		StartedAt:             time.Date(2026, 3, 15, 16, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 16, 3, 0, 0, time.UTC),
		AccumulatedCost:       0.22,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Attempts: 1, FilesChanged: []string{"calc/calc.go", "calc/calc_test.go"}},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-logic-gaps", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Status", "ready_for_review"},
		{"Cycles", "Cycles: 1"},
		{"Cost", "$0.22"},
		{"Validation", "Validation: \u2713 passed"},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// TestScenario_ExecShow_Full_LogicGapsFacet verifies that exec show --full
// displays review.json with logic_gaps facet findings and execution-policy.json
// showing logic_gaps in the configured facets list.
func TestScenario_ExecShow_Full_LogicGapsFacet(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:                 "run-logic-gaps-full",
		SpecID:                "add-subtract",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 1,
		TotalReplans:          0,
		StartedAt:             time.Date(2026, 3, 15, 16, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 16, 3, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	seedEvidence(t, store, "run-logic-gaps-full", map[string]string{
		"review.json":           `{"findings": [{"facet": "logic_gaps", "severity": "suggestion", "description": "Consider adding overflow checks in arithmetic operations", "disposition": "new"}]}`,
		"execution-policy.json": `{"review": {"facets": ["spec_alignment", "code_quality", "logic_gaps"], "tiers": {"spec_alignment": "high", "code_quality": "medium", "logic_gaps": "medium"}, "replan_threshold": "warning"}}`,
		"acceptance.json":       `{"all_pass": true, "criteria": [{"description": "Subtract(5, 3) returns 2", "result": "pass"}]}`,
	})

	output, err := execShow("run-logic-gaps-full", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	// review.json must appear and contain logic_gaps facet findings
	if !strings.Contains(output, "review.json") {
		t.Errorf("expected review.json section in full output, got:\n%s", output)
	}
	if !strings.Contains(output, "logic_gaps") {
		t.Errorf("expected logic_gaps facet in review.json, got:\n%s", output)
	}
	// execution-policy.json must appear and list logic_gaps in facets
	if !strings.Contains(output, "execution-policy.json") {
		t.Errorf("expected execution-policy.json section in full output, got:\n%s", output)
	}
	// Must not show stale "running" status
	if strings.Contains(output, "Status: running") {
		t.Errorf("full output shows stale 'running' status:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review in full output, got:\n%s", output)
	}
}

// TestScenario_ExecList_LogicGapsFacet verifies exec list shows a run that
// used the logic_gaps facet with ready_for_review status.
func TestScenario_ExecList_LogicGapsFacet(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	if err := store.Save(&runstore.RunState{
		RunID:                 "run-logic-gaps-list",
		SpecID:                "add-subtract",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 1,
		TotalReplans:          0,
		StartedAt:             time.Date(2026, 3, 15, 16, 0, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	output, err := execList("fixture-calc", store)
	if err != nil {
		t.Fatalf("execList: %v", err)
	}

	if !strings.Contains(output, "run-logic-gaps-list") {
		t.Errorf("expected run-logic-gaps-list in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review in output, got:\n%s", output)
	}
}

// TestScenario_ExecShow_NewVsPreexistingFindings verifies exec show correctly
// displays a cycle-2 run where a review finding triggered a fix cycle.
// Scenario 9: after cycle 1 blocked on a spec_alignment error, the agent fixed it
// in cycle 2 and all gates passed → ready_for_review.
func TestScenario_ExecShow_NewVsPreexistingFindings(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:                 "run-new-vs-preexisting",
		SpecID:                "add-refund-endpoint",
		ProjectID:             "fixture-multipackage",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 2,
		TotalReplans:          1,
		StartedAt:             time.Date(2026, 3, 15, 16, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 16, 5, 0, 0, time.UTC),
		AccumulatedCost:       0.35,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Attempts: 1, FilesChanged: []string{"internal/refund/refund.go"}},
			{TaskID: "t-002", Status: "done", Attempts: 1, FilesChanged: []string{"internal/refund/refund.go"}},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-new-vs-preexisting", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Status", "ready_for_review"},
		{"Cycles", "Cycles: 2"},
		{"Cost", "$0.35"},
		{"Validation", "Validation: \u2713 passed"},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// TestScenario_ExecShow_Full_NewVsPreexistingDispositions verifies exec show --full
// renders review.json with findings labeled "new" and "pre-existing".
// In a multi-cycle run, pre-existing findings from cycle 1 reappear with
// disposition "pre-existing" in cycle 2 and do not trigger further replanning.
func TestScenario_ExecShow_Full_NewVsPreexistingDispositions(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:                 "run-dispositions-full",
		SpecID:                "add-refund-endpoint",
		ProjectID:             "fixture-multipackage",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 2,
		TotalReplans:          1,
		StartedAt:             time.Date(2026, 3, 15, 16, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 16, 5, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	// review.json: cycle-2 findings with both "new" and "pre-existing" dispositions.
	// code_quality suggestion from cycle 1 reappears → "pre-existing" (does not reblock).
	// spec_alignment suggestion is new in cycle 2 → "new" (below error threshold, non-blocking).
	seedEvidence(t, store, "run-dispositions-full", map[string]string{
		"review.json": `{
  "code_quality": [
    {
      "facet": "code_quality",
      "severity": "suggestion",
      "file": "internal/refund/refund.go",
      "line": 10,
      "description": "Consider adding error handling for nil Refund input",
      "cycle": 2,
      "disposition": "pre-existing"
    }
  ],
  "spec_alignment": [
    {
      "facet": "spec_alignment",
      "severity": "suggestion",
      "file": "internal/refund/refund.go",
      "line": 25,
      "description": "ProcessPartial could include a comment explaining the percentage semantics",
      "cycle": 2,
      "disposition": "new"
    }
  ]
}`,
	})

	output, err := execShow("run-dispositions-full", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	// review.json must appear in the evidence bundle
	if !strings.Contains(output, "review.json") {
		t.Errorf("expected review.json section in full output, got:\n%s", output)
	}
	// "pre-existing" disposition must appear in review.json content
	if !strings.Contains(output, "pre-existing") {
		t.Errorf("expected pre-existing disposition in review.json, got:\n%s", output)
	}
	// "disposition" field name must appear
	if !strings.Contains(output, "disposition") {
		t.Errorf("expected disposition field in review.json, got:\n%s", output)
	}
	// Both facets must appear
	if !strings.Contains(output, "code_quality") {
		t.Errorf("expected code_quality facet in review.json, got:\n%s", output)
	}
	if !strings.Contains(output, "spec_alignment") {
		t.Errorf("expected spec_alignment facet in review.json, got:\n%s", output)
	}
	// Status must be ready_for_review, not the stale "running"
	if strings.Contains(output, "Status: running") {
		t.Errorf("full output shows stale 'running' status:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review in full output, got:\n%s", output)
	}
}

// TestScenario_ExecList_NewVsPreexisting verifies exec list shows a run that
// completed with new-vs-preexisting finding distinction (cycle 2, ready_for_review).
func TestScenario_ExecList_NewVsPreexisting(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	if err := store.Save(&runstore.RunState{
		RunID:                 "run-nvp-list",
		SpecID:                "add-refund-endpoint",
		ProjectID:             "fixture-multipackage",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 2,
		TotalReplans:          1,
		StartedAt:             time.Date(2026, 3, 15, 16, 0, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	output, err := execList("fixture-multipackage", store)
	if err != nil {
		t.Fatalf("execList: %v", err)
	}

	if !strings.Contains(output, "run-nvp-list") {
		t.Errorf("expected run-nvp-list in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review in output, got:\n%s", output)
	}
}

// --- Scenario 10: Missing Acceptance Criteria → needs_human ---

// TestScenario_ExecShow_MissingAcceptanceCriteria verifies that exec show correctly
// displays a run that terminated with needs_human because the spec had no
// Acceptance Criteria section. The blocker summary must appear in the output
// so the user understands why execution stopped without any fix cycles.
func TestScenario_ExecShow_MissingAcceptanceCriteria(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Seed: a run that hit stage_needs_human in the accept stage because the
	// spec had no ## Acceptance Criteria section. No fix cycles were attempted —
	// the accept stage terminates immediately on missing criteria.
	rs := &runstore.RunState{
		RunID:           "run-no-criteria",
		SpecID:          "no-acceptance-criteria",
		ProjectID:       "fixture-calc",
		Status:          runstore.StatusNeedsHuman,
		TerminalReason:  "stage_needs_human",
		BlockerSummary:  "spec lacks acceptance criteria section — cannot evaluate acceptance. Revise the spec to include acceptance criteria.",
		Cycle:           1,
		TotalReplans:    0,
		StartedAt:       time.Date(2026, 3, 15, 17, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2026, 3, 15, 17, 1, 0, 0, time.UTC),
		AccumulatedCost: 0.05,
		Tasks:           []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-no-criteria", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Status", "needs_human"},
		{"Reason", "stage_needs_human"},
		{"Blocker", "acceptance criteria"},
		{"Cycles", "Cycles: 1"},
		{"Cost", "$0.05"},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// TestScenario_ExecShow_Full_MissingAcceptanceCriteria verifies that exec show --full
// displays the summary.md evidence file for a run that terminated because the
// spec had no Acceptance Criteria section.
func TestScenario_ExecShow_Full_MissingAcceptanceCriteria(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:          "run-no-criteria-full",
		SpecID:         "no-acceptance-criteria",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusNeedsHuman,
		TerminalReason: "stage_needs_human",
		BlockerSummary: "spec lacks acceptance criteria section — cannot evaluate acceptance. Revise the spec to include acceptance criteria.",
		Cycle:          1,
		TotalReplans:   0,
		StartedAt:      time.Date(2026, 3, 15, 17, 0, 0, 0, time.UTC),
		EndedAt:        time.Date(2026, 3, 15, 17, 1, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	seedEvidence(t, store, "run-no-criteria-full", map[string]string{
		"summary.md": "# Execution Summary\n\n- **Status:** needs_human\n- **Reason:** spec lacks acceptance criteria section — cannot evaluate acceptance. Revise the spec to include acceptance criteria.\n",
	})

	output, err := execShow("run-no-criteria-full", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	if !strings.Contains(output, "summary.md") {
		t.Errorf("expected summary.md section in full output, got:\n%s", output)
	}
	if !strings.Contains(output, "needs_human") {
		t.Errorf("expected needs_human in full output, got:\n%s", output)
	}
	if strings.Contains(output, "Status: running") {
		t.Errorf("full output shows stale 'running' status:\n%s", output)
	}
}

// TestScenario_ExecList_MissingAcceptanceCriteria verifies exec list shows a run
// that terminated with needs_human due to missing acceptance criteria.
func TestScenario_ExecList_MissingAcceptanceCriteria(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	if err := store.Save(&runstore.RunState{
		RunID:          "run-no-criteria-list",
		SpecID:         "no-acceptance-criteria",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusNeedsHuman,
		TerminalReason: "stage_needs_human",
		Cycle:          1,
		TotalReplans:   0,
		StartedAt:      time.Date(2026, 3, 15, 17, 0, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	output, err := execList("fixture-calc", store)
	if err != nil {
		t.Fatalf("execList: %v", err)
	}

	if !strings.Contains(output, "run-no-criteria-list") {
		t.Errorf("expected run-no-criteria-list in output, got:\n%s", output)
	}
	if !strings.Contains(output, "needs_human") {
		t.Errorf("expected needs_human in output, got:\n%s", output)
	}
}

// --- Scenario 11: Blocked Worktree Cleanup on Re-run ---

// TestScenario_ExecShow_BlockedWorktreePreserved verifies that exec show displays
// a run that terminated with blocked status (e.g. provider failure) with its
// worktree path preserved — so the user can diagnose what went wrong.
func TestScenario_ExecShow_BlockedWorktreePreserved(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Seed: a blocked run where the provider failed (e.g. invalid API key).
	// FinalizeStage preserves the worktree for blocked runs.
	rs := &runstore.RunState{
		RunID:           "run-blocked-wt",
		SpecID:          "add-subtract",
		ProjectID:       "fixture-calc",
		Status:          runstore.StatusBlocked,
		TerminalReason:  "provider_failure",
		BlockerSummary:  "Claude API returned 401: invalid API key",
		WorktreePath:    "/tmp/gromit-worktree-12345",
		Cycle:           1,
		TotalReplans:    0,
		StartedAt:       time.Date(2026, 3, 15, 18, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2026, 3, 15, 18, 1, 0, 0, time.UTC),
		AccumulatedCost: 0.00,
		Tasks:           []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	output, err := execShow("run-blocked-wt", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	checks := []struct {
		field string
		want  string
	}{
		{"Status", "blocked"},
		{"Reason", "provider_failure"},
		{"Blocker", "invalid API key"},
		{"Worktree", "/tmp/gromit-worktree-12345"},
	}
	for _, c := range checks {
		if !strings.Contains(output, c.want) {
			t.Errorf("%s: want %q in output, got:\n%s", c.field, c.want, output)
		}
	}
}

// TestScenario_ExecShow_Full_BlockedWorktreePreserved verifies that exec show --full
// on a blocked run displays evidence showing the blocked terminal state.
func TestScenario_ExecShow_Full_BlockedWorktreePreserved(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:          "run-blocked-wt-full",
		SpecID:         "add-subtract",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusBlocked,
		TerminalReason: "provider_failure",
		BlockerSummary: "Claude API returned 401: invalid API key",
		WorktreePath:   "/tmp/gromit-worktree-12345",
		Cycle:          1,
		TotalReplans:   0,
		StartedAt:      time.Date(2026, 3, 15, 18, 0, 0, 0, time.UTC),
		EndedAt:        time.Date(2026, 3, 15, 18, 1, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	seedEvidence(t, store, "run-blocked-wt-full", map[string]string{
		"summary.md": "# Execution Summary\n\n- **Status:** blocked\n- **Reason:** Claude API returned 401: invalid API key\n",
	})

	output, err := execShow("run-blocked-wt-full", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	if !strings.Contains(output, "summary.md") {
		t.Errorf("expected summary.md section in full output, got:\n%s", output)
	}
	if !strings.Contains(output, "blocked") {
		t.Errorf("expected blocked status in full output, got:\n%s", output)
	}
	if strings.Contains(output, "Status: running") {
		t.Errorf("full output shows stale 'running' status:\n%s", output)
	}
}

// TestScenario_ExecList_BlockedWorktreeCleanup verifies exec list shows a blocked
// run alongside a subsequent re-run of the same spec, with correct statuses.
// After re-run, the blocked run's worktree is cleaned by InitStage — the store
// reflects both runs, each with the correct status.
func TestScenario_ExecList_BlockedWorktreeCleanup(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// First run: blocked (provider failure). WorktreePath cleared by re-run's InitStage.
	if err := store.Save(&runstore.RunState{
		RunID:          "run-blocked-cleaned",
		SpecID:         "add-subtract",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusBlocked,
		TerminalReason: "provider_failure",
		WorktreePath:   "", // cleared by second run's InitStage
		Cycle:          1,
		TotalReplans:   0,
		StartedAt:      time.Date(2026, 3, 15, 18, 0, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	// Second run: completed successfully after fixing the API key.
	if err := store.Save(&runstore.RunState{
		RunID:                 "run-second-after-blocked",
		SpecID:                "add-subtract",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		WorktreePath:          "/tmp/gromit-worktree-67890",
		Cycle:                 1,
		TotalReplans:          0,
		StartedAt:             time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	output, err := execList("fixture-calc", store)
	if err != nil {
		t.Fatalf("execList: %v", err)
	}

	if !strings.Contains(output, "run-blocked-cleaned") {
		t.Errorf("expected run-blocked-cleaned in output, got:\n%s", output)
	}
	if !strings.Contains(output, "blocked") {
		t.Errorf("expected blocked status in output, got:\n%s", output)
	}
	if !strings.Contains(output, "run-second-after-blocked") {
		t.Errorf("expected run-second-after-blocked in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ready_for_review") {
		t.Errorf("expected ready_for_review status in output, got:\n%s", output)
	}
}

// TestScenario_ExecShow_BlockedWorktreeCleared verifies that after a re-run,
// exec show on the prior blocked run shows no worktree path (cleared by InitStage),
// while exec show on the new run shows its own worktree path.
func TestScenario_ExecShow_BlockedWorktreeCleared(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	// Prior blocked run: WorktreePath cleared by second run's InitStage.
	if err := store.Save(&runstore.RunState{
		RunID:          "run-prior-blocked",
		SpecID:         "add-subtract",
		ProjectID:      "fixture-calc",
		Status:         runstore.StatusBlocked,
		TerminalReason: "provider_failure",
		WorktreePath:   "", // cleared
		Cycle:          1,
		StartedAt:      time.Date(2026, 3, 15, 18, 0, 0, 0, time.UTC),
		EndedAt:        time.Date(2026, 3, 15, 18, 1, 0, 0, time.UTC),
		Tasks:          []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	// New run: has its own worktree.
	if err := store.Save(&runstore.RunState{
		RunID:        "run-new-after-cleanup",
		SpecID:       "add-subtract",
		ProjectID:    "fixture-calc",
		Status:       runstore.StatusReadyForReview,
		WorktreePath: "/tmp/gromit-worktree-new",
		Cycle:        1,
		StartedAt:    time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC),
		EndedAt:      time.Date(2026, 3, 15, 19, 5, 0, 0, time.UTC),
		Tasks:        []runstore.Task{},
	}); err != nil {
		t.Fatal(err)
	}

	// Prior blocked run: worktree path should be absent from output.
	priorOut, err := execShow("run-prior-blocked", store, false)
	if err != nil {
		t.Fatalf("execShow prior: %v", err)
	}
	if strings.Contains(priorOut, "Worktree:") {
		t.Errorf("prior blocked run should have no Worktree line after cleanup, got:\n%s", priorOut)
	}

	// New run: should show its own worktree path.
	newOut, err := execShow("run-new-after-cleanup", store, false)
	if err != nil {
		t.Fatalf("execShow new: %v", err)
	}
	if !strings.Contains(newOut, "/tmp/gromit-worktree-new") {
		t.Errorf("new run should show its own worktree path, got:\n%s", newOut)
	}
}

// --- Scenario: Spec 0002c — Provider-Agnostic Adapter Layer ---

// TestScenario_ExecShow_Full_InvocationsHaveProvider verifies that exec show --full
// displays metrics.json with provider-labeled invocation records.
// Spec 0002c adds a Provider field to InvocationRecord so each invocation
// identifies which LLM provider was used (e.g. "claude").
//
// RED: runstore.InvocationRecord has no Provider field — this test will not compile
// until Spec 0002c wires ProviderName() into InvocationRecord.
func TestScenario_ExecShow_Full_InvocationsHaveProvider(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:                 "run-0002c-provider",
		SpecID:                "add-subtract",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 1,
		TotalReplans:          0,
		StartedAt:             time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 12, 3, 0, 0, time.UTC),
		AccumulatedCost:       0.21,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Attempts: 1, FilesChanged: []string{"calc/calc.go", "calc/calc_test.go"}},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	// Build metrics.json using InvocationRecord struct to confirm Provider field exists.
	// Spec 0002c: LLMAdapter.ProviderName() is recorded in each InvocationRecord.
	invocations := []runstore.InvocationRecord{
		{
			Phase:     "plan",
			Tier:      "high",
			Model:     "opus",
			Provider:  "claude", // Spec 0002c: Provider field — RED until added
			TokensIn:  500,
			TokensOut: 150,
			CostUSD:   0.08,
			Success:   true,
		},
		{
			Phase:     "execute",
			Tier:      "medium",
			Model:     "sonnet",
			Provider:  "claude", // Spec 0002c: Provider field — RED until added
			TokensIn:  1200,
			TokensOut: 400,
			CostUSD:   0.13,
			Success:   true,
		},
	}
	type metricsDoc struct {
		TotalCostUSD float64                     `json:"total_cost_usd"`
		Invocations  []runstore.InvocationRecord `json:"invocations"`
	}
	metricsData, err := json.MarshalIndent(metricsDoc{TotalCostUSD: 0.21, Invocations: invocations}, "", "  ")
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}

	seedEvidence(t, store, "run-0002c-provider", map[string]string{
		"metrics.json": string(metricsData),
	})

	output, err := execShow("run-0002c-provider", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	// Each invocation record must include the provider name.
	if !strings.Contains(output, `"provider": "claude"`) {
		t.Errorf("expected provider 'claude' in invocation records, got:\n%s", output)
	}
	// Both plan and execute invocations should have provider.
	if strings.Count(output, `"provider": "claude"`) < 2 {
		t.Errorf("expected at least 2 invocation records with provider='claude', got:\n%s", output)
	}
	// Cost must be non-zero per invocation (real LLM calls).
	if !strings.Contains(output, `"cost_usd": 0.08`) {
		t.Errorf("expected non-zero cost_usd for plan invocation, got:\n%s", output)
	}
}

// TestScenario_ExecShow_AdapterWiring_InvocationCountShown verifies that exec show
// (brief mode) displays the total LLM invocation count from metrics.json.
// Spec 0002c Scenario 2 (Adapter Wiring Verification): the brief summary must make
// invocation count visible without requiring --full.
//
// RED: exec show currently has no "Invocations:" line — it shows Cost but not count.
// GREEN after: execShow reads metrics.json and emits "Invocations: N".
func TestScenario_ExecShow_AdapterWiring_InvocationCountShown(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:                 "run-0002c-wiring",
		SpecID:                "add-subtract",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 1,
		TotalReplans:          0,
		StartedAt:             time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 12, 5, 0, 0, time.UTC),
		AccumulatedCost:       0.30,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "t-001", Status: "done", Attempts: 1, FilesChanged: []string{"calc/calc.go"}},
		},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	// Build metrics.json with 4 LLM-backed invocations (plan, execute, review, accept).
	// validate and compile are NOT included — they use shell/deterministic adapters.
	invocations := []runstore.InvocationRecord{
		{Phase: "plan", Tier: "high", Model: "opus", Provider: "claude", CostUSD: 0.08, Success: true},
		{Phase: "execute", Tier: "medium", Model: "sonnet", Provider: "claude", CostUSD: 0.13, Success: true},
		{Phase: "review", Tier: "medium", Model: "sonnet", Provider: "claude", CostUSD: 0.05, Success: true},
		{Phase: "accept", Tier: "medium", Model: "sonnet", Provider: "claude", CostUSD: 0.04, Success: true},
	}
	type metricsDoc struct {
		TotalCostUSD float64                     `json:"total_cost_usd"`
		Invocations  []runstore.InvocationRecord `json:"invocations"`
	}
	metricsData, err := json.MarshalIndent(metricsDoc{TotalCostUSD: 0.30, Invocations: invocations}, "", "  ")
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}
	seedEvidence(t, store, "run-0002c-wiring", map[string]string{
		"metrics.json": string(metricsData),
	})

	// Brief mode (not --full).
	output, err := execShow("run-0002c-wiring", store, false)
	if err != nil {
		t.Fatalf("execShow: %v", err)
	}

	// Spec 0002c Scenario 2: invocation count must appear in brief output.
	// RED: exec show currently has no "Invocations:" line.
	if !strings.Contains(output, "Invocations: 4") {
		t.Errorf("expected 'Invocations: 4' in exec show brief output, got:\n%s", output)
	}
}

// TestScenario_ExecShow_Full_AdapterWiring_OnlyLLMPhasesInMetrics verifies that
// exec show --full shows invocations for LLM-backed phases (plan, execute, review, accept)
// and that validate/compile do NOT appear — those stages use shell/deterministic adapters.
// Spec 0002c Scenario 2 (Adapter Wiring Verification).
func TestScenario_ExecShow_Full_AdapterWiring_OnlyLLMPhasesInMetrics(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:                 "run-0002c-phases",
		SpecID:                "add-subtract",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		Cycle:                 1,
		AccumulatedCost:       0.30,
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks:                 []runstore.Task{{TaskID: "t-001", Status: "done"}},
	}
	if err := store.Save(rs); err != nil {
		t.Fatal(err)
	}

	invocations := []runstore.InvocationRecord{
		{Phase: "plan", Tier: "high", Model: "opus", Provider: "claude", CostUSD: 0.08, Success: true},
		{Phase: "execute", Tier: "medium", Model: "sonnet", Provider: "claude", CostUSD: 0.13, Success: true},
		{Phase: "review", Tier: "medium", Model: "sonnet", Provider: "claude", CostUSD: 0.05, Success: true},
		{Phase: "accept", Tier: "medium", Model: "sonnet", Provider: "claude", CostUSD: 0.04, Success: true},
	}
	type metricsDoc struct {
		TotalCostUSD float64                     `json:"total_cost_usd"`
		Invocations  []runstore.InvocationRecord `json:"invocations"`
	}
	metricsData, err := json.MarshalIndent(metricsDoc{TotalCostUSD: 0.30, Invocations: invocations}, "", "  ")
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}
	seedEvidence(t, store, "run-0002c-phases", map[string]string{
		"metrics.json": string(metricsData),
	})

	output, err := execShow("run-0002c-phases", store, true /* full */)
	if err != nil {
		t.Fatalf("execShow --full: %v", err)
	}

	// All 4 LLM-backed phases must appear in invocation records.
	for _, phase := range []string{"plan", "execute", "review", "accept"} {
		if !strings.Contains(output, fmt.Sprintf(`"phase": "%s"`, phase)) {
			t.Errorf("expected phase %q in invocation records, got:\n%s", phase, output)
		}
	}
	// validate and compile must NOT appear — they use shell/deterministic adapters, not LLM.
	for _, phase := range []string{"validate", "compile"} {
		if strings.Contains(output, fmt.Sprintf(`"phase": "%s"`, phase)) {
			t.Errorf("phase %q should not appear in LLM invocation records (uses non-LLM adapter), got:\n%s", phase, output)
		}
	}
}

// TestScenario_ExecSpec_TimeoutEnforcement_ContextReachesStages is the Scenario 5
// (0002c/0002d) scenario-level evidence test for Timeout Enforcement.
//
// It verifies that context deadlines propagate through the exec spec pipeline
// to individual stages. This is the same mechanism used by task-level timeouts:
// taskloop creates a context.WithTimeout from MaxTaskDurationSeconds and passes
// it into each InvokeInDir/InvokeStream call. Here we verify the outer exec
// layer correctly threads the context from r.run(ctx) → SpecLoop.Run(ctx) →
// stage.Run(ctx, rs), so a cancelled context terminates the run promptly.
func TestScenario_ExecSpec_TimeoutEnforcement_ContextReachesStages(t *testing.T) {
	blockingStage := &contextBlockingStage{name: "execute"}
	provider := &testStageProvider{stages: []specloop.Stage{blockingStage}}

	// 50ms deadline simulates a short per-task timeout. The blocking stage
	// waits for ctx.Done(), so the run should complete in ≈50ms, not 5s.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := &execSpecRun{
		specPath:      "test-spec.md",
		projectID:     "test-proj",
		storeDir:      t.TempDir(),
		stageProvider: provider,
	}

	start := time.Now()
	_, _ = r.run(ctx)
	elapsed := time.Since(start)

	// Run must complete within 10× the deadline — not hanging.
	if elapsed > 500*time.Millisecond {
		t.Errorf("run took %v with 50ms timeout — context not propagated through pipeline", elapsed)
	}
	// Stage must have received a context with a deadline set.
	if !blockingStage.receivedDeadline {
		t.Error("stage never received a context with a deadline — timeout cannot enforce")
	}
}

// contextBlockingStage blocks until context is done or 5 seconds (whichever
// first), recording whether it received a context with a deadline. Used to
// verify context propagation through exec spec.
type contextBlockingStage struct {
	name             string
	receivedDeadline bool
}

func (s *contextBlockingStage) Name() string { return s.name }
func (s *contextBlockingStage) Run(ctx context.Context, _ *runstore.RunState) (specloop.NextAction, error) {
	_, s.receivedDeadline = ctx.Deadline()
	select {
	case <-ctx.Done():
		return specloop.NextAction{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}
}

// TestScenario_ExecSpec_InvalidRoutingRatio_ReturnsError is the Scenario 9
// (0002c/0002d) evidence test for Routing Config Validation.
//
// It verifies that exec spec validates the policy before starting any stages:
// when routing.ratio values don't sum to 100, the run fails immediately with
// a descriptive error and no run state is created.
func TestScenario_ExecSpec_InvalidRoutingRatio_ReturnsError(t *testing.T) {
	tmp := t.TempDir()

	// Seed: write a policy JSON with invalid routing ratio (sums to 70, not 100)
	policyPath := filepath.Join(tmp, "invalid-routing.json")
	policyJSON := `{
		"always_run": [{"name": "unit-tests", "command": "go test ./...", "type": "test"}],
		"budgets": {
			"max_spec_cycles": 3,
			"max_task_duration_seconds": 300,
			"max_run_duration_seconds": 3600,
			"max_run_cost_usd": 50.0
		},
		"models": {"planner": "high", "executor": "medium", "evaluator": "high"},
		"review": {"facets": ["spec_alignment"], "replan_threshold": "warning"},
		"routing": {"ratio": {"claude": 60, "codex": 10}}
	}`
	if err := os.WriteFile(policyPath, []byte(policyJSON), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	// Track whether BuildStages is called — it must not be if validation fails early.
	var buildCalled bool
	stageProvider := &countingStageProvider{onBuild: func() { buildCalled = true }}

	// Invoke: run exec spec with the invalid policy.
	storeDir := filepath.Join(tmp, "store")
	r := &execSpecRun{
		specPath:      "test-spec.md",
		projectID:     "test-proj",
		policyPath:    policyPath,
		storeDir:      storeDir,
		stageProvider: stageProvider,
	}
	_, err := r.run(context.Background())

	// Assert: error returned before any stages are built.
	if err == nil {
		t.Fatal("expected error for invalid policy routing ratio, got nil")
	}
	if !strings.Contains(err.Error(), "invalid policy") {
		t.Errorf("expected 'invalid policy' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "sum to 100") {
		t.Errorf("expected ratio-sum error in message, got: %v", err)
	}
	if buildCalled {
		t.Error("BuildStages must not be called when policy validation fails")
	}
}

// countingStageProvider calls onBuild when BuildStages is invoked.
type countingStageProvider struct {
	onBuild func()
}

func (c *countingStageProvider) BuildStages(_ execpolicy.Policy, _ *runstore.RunState, _ *specloop.Budget, _ *runstore.EventLog) ([]specloop.Stage, error) {
	if c.onBuild != nil {
		c.onBuild()
	}
	return nil, nil
}
