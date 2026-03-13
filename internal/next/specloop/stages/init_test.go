package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestInitStage_CleansBlockedWorktrees(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a prior run with the SAME spec+project so store.List finds it
	priorRS := runstore.NewRunState("test-spec", "test-project")
	priorRS.Status = runstore.StatusBlocked
	priorRS.WorktreePath = filepath.Join(t.TempDir(), "old-worktree")
	os.MkdirAll(priorRS.WorktreePath, 0o755)
	store.Save(priorRS)

	eventLog := runstore.NewEventLog(filepath.Join(storeDir, "events.jsonl"))

	specFile := filepath.Join(storeDir, "spec.md")
	os.WriteFile(specFile, []byte("# Test Spec"), 0o644)
	policyFile := filepath.Join(storeDir, "policy.json")
	os.WriteFile(policyFile, []byte(`{"budgets":{}}`), 0o644)

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    storeDir,
		GitOps:     gitOps,
	}, store, eventLog)
	newRS := runstore.NewRunState("test-spec", "test-project")

	_, err := stage.Run(context.Background(), newRS)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Prior blocked run's worktree should be removed
	if _, err := os.Stat(priorRS.WorktreePath); !os.IsNotExist(err) {
		t.Error("blocked worktree should have been removed")
	}

	// Prior run's worktree_path should be cleared in store
	reloaded, err := store.Get(priorRS.RunID)
	if err != nil {
		t.Fatalf("Get prior run: %v", err)
	}
	if reloaded.WorktreePath != "" {
		t.Error("worktree_path should be cleared in run.json")
	}
}

func TestInitStage_SkipsNonBlockedWorktrees(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a prior run that is ready_for_review (not blocked)
	priorRS := runstore.NewRunState("test-spec", "test-project")
	priorRS.Status = runstore.StatusReadyForReview
	priorRS.WorktreePath = filepath.Join(t.TempDir(), "good-worktree")
	os.MkdirAll(priorRS.WorktreePath, 0o755)
	store.Save(priorRS)

	specFile := filepath.Join(storeDir, "spec.md")
	os.WriteFile(specFile, []byte("# Test Spec"), 0o644)
	policyFile := filepath.Join(storeDir, "policy.json")
	os.WriteFile(policyFile, []byte(`{"budgets":{}}`), 0o644)

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    storeDir,
		GitOps:     gitOps,
	}, store, nil)
	newRS := runstore.NewRunState("test-spec", "test-project")

	_, err := stage.Run(context.Background(), newRS)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Non-blocked worktree should still exist
	if _, err := os.Stat(priorRS.WorktreePath); os.IsNotExist(err) {
		t.Error("non-blocked worktree should NOT be removed")
	}
}

func TestInitStage_SkipsDifferentSpecBlockedWorktrees(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a blocked run with a DIFFERENT spec
	priorRS := runstore.NewRunState("other-spec", "test-project")
	priorRS.Status = runstore.StatusBlocked
	priorRS.WorktreePath = filepath.Join(t.TempDir(), "other-worktree")
	os.MkdirAll(priorRS.WorktreePath, 0o755)
	store.Save(priorRS)

	specFile := filepath.Join(storeDir, "spec.md")
	os.WriteFile(specFile, []byte("# Test Spec"), 0o644)
	policyFile := filepath.Join(storeDir, "policy.json")
	os.WriteFile(policyFile, []byte(`{"budgets":{}}`), 0o644)

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    storeDir,
		GitOps:     gitOps,
	}, store, nil)
	newRS := runstore.NewRunState("test-spec", "test-project")

	_, err := stage.Run(context.Background(), newRS)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Different-spec blocked worktree should still exist
	if _, err := os.Stat(priorRS.WorktreePath); os.IsNotExist(err) {
		t.Error("different-spec blocked worktree should NOT be removed")
	}
}

type fakeGitOps struct {
	createdBranch string
	worktreePath  string
	removedPath   string
	createErr     error
	removeErr     error
}

func (f *fakeGitOps) CreateWorktree(repoDir, branch string) (string, error) {
	f.createdBranch = branch
	return f.worktreePath, f.createErr
}

func (f *fakeGitOps) RemoveWorktree(path string) error {
	f.removedPath = path
	return f.removeErr
}

// Verify InitStage satisfies the Stage interface.
var _ specloop.Stage = (*InitStage)(nil)

func TestInitStage_CreatesRunDir(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	specFile := filepath.Join(tmp, "spec.md")
	os.WriteFile(specFile, []byte("# Test Spec"), 0o644)
	policyFile := filepath.Join(tmp, "policy.json")
	os.WriteFile(policyFile, []byte(`{"budgets":{}}`), 0o644)

	gitOps := &fakeGitOps{worktreePath: filepath.Join(tmp, "worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    tmp,
		GitOps:     gitOps,
	}, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	runDir := store.RunDir(rs.RunID)
	if _, err := os.Stat(runDir); os.IsNotExist(err) {
		t.Fatalf("run dir not created: %s", runDir)
	}
}

func TestInitStage_CreatesWorktreeWithCorrectBranch(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	specFile := filepath.Join(tmp, "spec.md")
	os.WriteFile(specFile, []byte("# Test Spec"), 0o644)
	policyFile := filepath.Join(tmp, "policy.json")
	os.WriteFile(policyFile, []byte(`{"budgets":{}}`), 0o644)

	gitOps := &fakeGitOps{worktreePath: filepath.Join(tmp, "worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    tmp,
		GitOps:     gitOps,
	}, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBranch := "gromit/spec-spec-001-" + rs.RunID
	if gitOps.createdBranch != expectedBranch {
		t.Fatalf("expected branch %q, got %q", expectedBranch, gitOps.createdBranch)
	}
}

func TestInitStage_CopiesSpecIntoRunDir(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	specContent := "# My Spec\nSome content"
	specFile := filepath.Join(tmp, "spec.md")
	os.WriteFile(specFile, []byte(specContent), 0o644)
	policyFile := filepath.Join(tmp, "policy.json")
	os.WriteFile(policyFile, []byte(`{"budgets":{}}`), 0o644)

	gitOps := &fakeGitOps{worktreePath: filepath.Join(tmp, "worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    tmp,
		GitOps:     gitOps,
	}, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copiedSpec := filepath.Join(store.RunDir(rs.RunID), "spec.md")
	data, err := os.ReadFile(copiedSpec)
	if err != nil {
		t.Fatalf("spec not copied: %v", err)
	}
	if string(data) != specContent {
		t.Fatalf("spec content mismatch: got %q", string(data))
	}
}

func TestInitStage_SnapshotsPolicyIntoRunDir(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	specFile := filepath.Join(tmp, "spec.md")
	os.WriteFile(specFile, []byte("# Test Spec"), 0o644)
	policyContent := `{"always_run":[],"budgets":{"max_spec_cycles":3},"models":{"planner":"high"}}`
	policyFile := filepath.Join(tmp, "policy.json")
	os.WriteFile(policyFile, []byte(policyContent), 0o644)

	gitOps := &fakeGitOps{worktreePath: filepath.Join(tmp, "worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    tmp,
		GitOps:     gitOps,
	}, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	copiedPolicy := filepath.Join(store.RunDir(rs.RunID), "execution-policy.json")
	data, err := os.ReadFile(copiedPolicy)
	if err != nil {
		t.Fatalf("policy not copied: %v", err)
	}
	if string(data) != policyContent {
		t.Fatalf("policy content mismatch: got %q", string(data))
	}
}
