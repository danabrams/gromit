package stages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_InitStage_FixturesScaffoldedInTempDir verifies the Seed/Invoke/Assert
// scenario where tests scaffold required project fixtures in an isolated temp dir
// before invoking InitStage.
func TestScenario_InitStage_FixturesScaffoldedInTempDir(t *testing.T) {
	// Seed
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	prior := runstore.NewRunState("fixture-spec", "fixture-project")
	prior.Status = runstore.StatusReadyForReview
	prior.WorktreePath = filepath.Join(t.TempDir(), "existing-worktree")
	if err := os.MkdirAll(prior.WorktreePath, 0o755); err != nil {
		t.Fatalf("mkdir prior worktree: %v", err)
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	specsDir := filepath.Join(storeDir, "docs", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}
	projectConfigPath := filepath.Join(storeDir, "project.json")
	projectConfig := map[string]string{
		"name":      "fixture-project",
		"repo_path": storeDir,
		"specs_dir": specsDir,
	}
	projectData, err := json.Marshal(projectConfig)
	if err != nil {
		t.Fatalf("marshal project config: %v", err)
	}
	if err := os.WriteFile(projectConfigPath, projectData, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	specPath := filepath.Join(specsDir, "spec.md")
	if err := os.WriteFile(specPath, []byte("# Scenario Spec"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	policyPath := filepath.Join(storeDir, "policy", "execution.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatalf("mkdir policy dir: %v", err)
	}
	if err := os.WriteFile(policyPath, []byte(`{"budgets":{}}`), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	if _, err := os.Stat(projectConfigPath); err != nil {
		t.Fatalf("project config fixture missing: %v", err)
	}
	if _, err := os.Stat(policyPath); err != nil {
		t.Fatalf("policy fixture missing: %v", err)
	}

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	if err := os.MkdirAll(gitOps.worktreePath, 0o755); err != nil {
		t.Fatalf("mkdir new worktree: %v", err)
	}

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specPath,
		PolicyPath: policyPath,
		RepoDir:    storeDir,
		GitOps:     gitOps,
	}, store, nil)

	rs := runstore.NewRunState("fixture-spec", "fixture-project")

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert
	if err != nil {
		if strings.Contains(err.Error(), "project.json") || strings.Contains(err.Error(), "execution.json") {
			t.Fatalf("InitStage failed due to missing project-level fixtures: %v", err)
		}
		t.Fatalf("InitStage.Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if rs.WorktreePath == "" {
		t.Fatal("expected WorktreePath to be set")
	}
	if _, statErr := os.Stat(store.RunDir(rs.RunID)); statErr != nil {
		t.Fatalf("expected run dir to exist: %v", statErr)
	}
}
