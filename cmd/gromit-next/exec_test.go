package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestExecCmd_AcceptsBothFlags(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md", "--project", "my-project"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecCmd_DryRunFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md", "--project", "my-project", "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if !dryRun {
		t.Fatal("expected dry-run to be true")
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
	name string
	ran  bool
}

func (s *stageRecorder) Name() string { return s.name }
func (s *stageRecorder) Run(_ context.Context, _ *runstore.RunState) (specloop.NextAction, error) {
	s.ran = true
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
	loop := specloop.NewSpecLoop(filtered, specloop.SpecLoopConfig{MaxCycles: 1})
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

	loop := specloop.NewSpecLoop(filtered, specloop.SpecLoopConfig{MaxCycles: 1})
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
