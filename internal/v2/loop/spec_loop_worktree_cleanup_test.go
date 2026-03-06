package loop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
)

func TestSpecLoopFailureCommitsPartialWorkAndRemovesWorktree(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	initGitRepo(t, repoRoot)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}
	defer func() {
		if cdErr := os.Chdir(oldWd); cdErr != nil {
			t.Fatalf("restore cwd: %v", cdErr)
		}
	}()

	specID := "spec-loop-worktree-cleanup"
	worktreesDir := filepath.Join(repoRoot, ".gromit", "spec-worktrees")
	gitAdapter := &recordingGitAdapter{
		ExecGitAdapter: gitadapter.NewExecGitAdapter(worktreesDir),
		t:              t,
		specID:         specID,
	}

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	planStage := newFakePlanStage(specID)
	presentStage := newFakePresentStage()
	summaryCtx := &present.SummaryContext{}
	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(context.Background(), specID, nil); err == nil {
		t.Fatal("expected accept failure")
	}

	pattern := "[gromit: partial work] spec " + specID
	if !strings.Contains(strings.TrimSpace(gitAdapter.lastCommitLog), pattern) {
		t.Fatalf("expected partial work commit, log=%q", gitAdapter.lastCommitLog)
	}

	worktreePath := filepath.Join(worktreesDir, specID)
	if worktreeRegistered(t, repoRoot, worktreePath) {
		t.Fatalf("worktree %s should have been removed from git worktree list", worktreePath)
	}
}

func initGitRepo(t *testing.T, repoRoot string) {
	t.Helper()
	gitCommand(t, repoRoot, "init")
	gitCommand(t, repoRoot, "config", "user.name", "gromit-test")
	gitCommand(t, repoRoot, "config", "user.email", "gromit@example.com")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("initial"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitCommand(t, repoRoot, "add", "README.md")
	gitCommand(t, repoRoot, "commit", "-m", "initial commit")
}

func gitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s: %v", args, out, err)
	}
	return string(out)
}

func worktreeRegistered(t *testing.T, repoRoot, worktree string) bool {
	t.Helper()
	out := gitCommand(t, repoRoot, "worktree", "list", "--porcelain")
	return strings.Contains(out, worktree)
}

type recordingGitAdapter struct {
	*gitadapter.ExecGitAdapter
	t             *testing.T
	specID        string
	lastCommitLog string
}

func (r *recordingGitAdapter) RemoveWorktree(ctx context.Context, worktree string) error {
	r.t.Helper()
	r.lastCommitLog = gitCommand(r.t, worktree, "log", "-1", "--pretty=%B")
	return r.ExecGitAdapter.RemoveWorktree(ctx, worktree)
}
