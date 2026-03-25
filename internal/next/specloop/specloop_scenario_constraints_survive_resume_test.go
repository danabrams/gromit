package specloop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ConstraintsSurviveResume(t *testing.T) {
	// Seed: create a store and write a RunState with ArchitectureConstraints
	// to run.json, simulating a run paused mid-execution.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	constraints := []string{
		"use NormalizeNilFields for cross-package types",
		"separate validation in haiku invocation",
		"Path semantics: always relative to project root",
	}

	paused := runstore.NewRunState("spec-resume-001", "proj-resume")
	paused.Status = runstore.StatusRunning
	paused.Tasks = []runstore.Task{
		{
			TaskID:    "t-001",
			Objective: "implement feature A",
			Status:    "pending",
			Kind:      "original",
			Cycle:     1,
		},
	}
	paused.ArchitectureConstraints = constraints
	paused.NormalizeNilFields()

	runDir := store.RunDir(paused.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	data, err := json.Marshal(paused)
	if err != nil {
		t.Fatalf("marshal RunState: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), data, 0o644); err != nil {
		t.Fatalf("write run.json: %v", err)
	}

	// Invoke: load RunState from run.json (simulating resume) and mark as resumed.
	resumed, err := store.Get(paused.RunID)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	resumed.Resumed = true

	// Assert: ArchitectureConstraints are restored from run.json.
	if len(resumed.ArchitectureConstraints) != len(constraints) {
		t.Fatalf("ArchitectureConstraints not restored: got %d entries, want %d", len(resumed.ArchitectureConstraints), len(constraints))
	}
	for i, want := range constraints {
		if i < len(resumed.ArchitectureConstraints) && resumed.ArchitectureConstraints[i] != want {
			t.Errorf("ArchitectureConstraints[%d]: got %q, want %q", i, resumed.ArchitectureConstraints[i], want)
		}
	}

	// Assert: subsequent executor task prompt includes the constraints when
	// passed via TaskContext (derived from resumed RunState).
	if len(resumed.Tasks) == 0 {
		t.Fatal("expected tasks to be present after resume")
	}
	task := resumed.Tasks[0]

	// Pass constraints via TaskContext (derived from resumed RunState)
	prompt := renderTaskPrompt(task, TaskContext{ArchitectureConstraints: resumed.ArchitectureConstraints}, "")

	if !strings.Contains(prompt, "### Architecture Conventions") {
		t.Error("prompt does not contain '### Architecture Conventions' section header")
	}
	for _, constraint := range resumed.ArchitectureConstraints {
		if !strings.Contains(prompt, "- "+constraint) {
			t.Errorf("prompt does not contain constraint %q", constraint)
		}
	}
}
