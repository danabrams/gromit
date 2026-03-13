package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/workspace"
)

func TestResolveProjectConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "projects", "myapp")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	specsDir := filepath.Join(projectDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{"specs_dir": specsDir}
	cfgData, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(projectDir, "project.json"), cfgData, 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveProjectConfigPath(workspace.Root(tmpDir), "myapp")
	if err != nil {
		t.Fatalf("ResolveProjectConfigPath failed: %v", err)
	}
	if resolved != projectDir {
		t.Errorf("got %q, want %q", resolved, projectDir)
	}
}

func TestResolveProjectConfigPath_MissingProject(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := ResolveProjectConfigPath(workspace.Root(tmpDir), "nonexistent")
	if err == nil {
		t.Error("expected error for missing project")
	}
}
