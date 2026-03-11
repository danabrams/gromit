package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestSpecListCmd_Exists(t *testing.T) {
	cmd := newSpecListCmd()
	if cmd.Use != "list" {
		t.Fatalf("expected Use to be 'list', got %q", cmd.Use)
	}
}

func TestSpecDiscovery_FindsSpecsFromSpecsDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "spec-001.md"), []byte("# Spec 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "spec-002.md"), []byte("# Spec 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-.md file should be ignored.
	if err := os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	specs, err := DiscoverSpecs(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d: %v", len(specs), specs)
	}
}

func TestSpecStatus_ReadyForReview(t *testing.T) {
	runs := []runstore.RunState{
		{Status: runstore.StatusReadyForReview},
	}
	status := DeriveSpecStatus("spec-001", runs)
	if status != "ready_for_review" {
		t.Fatalf("expected ready_for_review, got %q", status)
	}
}

func TestSpecStatus_Running(t *testing.T) {
	runs := []runstore.RunState{
		{Status: runstore.StatusRunning},
	}
	status := DeriveSpecStatus("spec-001", runs)
	if status != "running" {
		t.Fatalf("expected running, got %q", status)
	}
}

func TestSpecStatus_NeedsAttention(t *testing.T) {
	runs := []runstore.RunState{
		{Status: runstore.StatusNeedsHuman},
	}
	status := DeriveSpecStatus("spec-001", runs)
	if status != "needs_attention" {
		t.Fatalf("expected needs_attention, got %q", status)
	}
}

func TestSpecStatus_Ready(t *testing.T) {
	status := DeriveSpecStatus("spec-001", nil)
	if status != "ready" {
		t.Fatalf("expected ready, got %q", status)
	}
}

func TestSpecStatus_Draft(t *testing.T) {
	status := DeriveSpecStatusFromContent("spec-001", nil, "DRAFT: This is a draft spec")
	if status != "draft" {
		t.Fatalf("expected draft, got %q", status)
	}
}

func TestSpecsDir_ReadFromProjectJSON(t *testing.T) {
	tmp := t.TempDir()
	projectJSON := `{"specs_dir": "my-specs"}`
	if err := os.WriteFile(filepath.Join(tmp, "project.json"), []byte(projectJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProjectConfig(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SpecsDir != "my-specs" {
		t.Fatalf("expected specs_dir=my-specs, got %q", cfg.SpecsDir)
	}
}
