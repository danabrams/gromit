package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
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

func (p *testStageProvider) BuildStages(_ execpolicy.Policy, _ *runstore.RunState) ([]specloop.Stage, error) {
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
