package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/testutil"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestValidate_BaselineRunnerAbsent_ReturnsReplanFrom(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	validWorktree := filepath.Join(t.TempDir(), "valid-worktree")
	if err := os.MkdirAll(validWorktree, 0o755); err != nil {
		t.Fatalf("create valid worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validWorktree, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("write .git in valid worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validWorktree, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod in valid worktree: %v", err)
	}
	testutil.WriteMinimalProjectFixtures(t, validWorktree)

	brokenWorktree := filepath.Join(t.TempDir(), "broken-worktree")
	if err := os.MkdirAll(brokenWorktree, 0o755); err != nil {
		t.Fatalf("create broken worktree: %v", err)
	}

	rs := runstore.NewRunState("baseline-spec", "baseline-project")
	rs.SpecID = "baseline-spec"
	rs.ProjectID = "baseline-project"
	rs.RunID = "run-001"
	rs.Cycle = 1
	rs.WorktreePath = brokenWorktree

	eventLog := runstore.NewEventLog(filepath.Join(repoDir, "events.jsonl"))
	gitOps := &recoveringGitOps{recoverPath: validWorktree}
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "FAIL"},
				},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: validWorktree,
		RepoDir: repoDir,
	}, eventLog, nil, gitOps)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("run validate: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if rs.BlockerSummary != "" {
		t.Fatalf("unexpected BlockerSummary: %q", rs.BlockerSummary)
	}
	if rs.WorktreePath != validWorktree {
		t.Fatalf("worktree path not updated to repaired value: got %q", rs.WorktreePath)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var recovered bool
	for _, ev := range events {
		if wre, ok := ev.(*runstore.WorktreeRecoveryEvent); ok && wre.RecoverySucceeded {
			recovered = true
			if wre.NewWorktreePath != validWorktree {
				t.Fatalf("worktree recovery event references wrong path: %q", wre.NewWorktreePath)
			}
		}
	}
	if !recovered {
		t.Fatal("expected successful worktree_recovery event")
	}
}

// recoveringGitOps is a GitOps fake that returns a pre-configured recovery path.
// It differs from fakeGitOps (in init_test.go) in that RecoverWorktree returns
// recoverPath, allowing tests to simulate recovery to a different worktree.
type recoveringGitOps struct {
	recoverPath  string
	recoverErr   error
	removedPaths []string
}

func (g *recoveringGitOps) CreateWorktree(repoDir, branch string) (string, error) {
	return "", nil
}

func (g *recoveringGitOps) RemoveWorktree(path string) error {
	g.removedPaths = append(g.removedPaths, path)
	return os.RemoveAll(path)
}

func (g *recoveringGitOps) RecoverWorktree(repoDir, branch string) (string, error) {
	return g.recoverPath, g.recoverErr
}

func (g *recoveringGitOps) CommitAll(workDir, message string) error {
	return nil
}
