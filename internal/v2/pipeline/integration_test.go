//go:build integration

package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	execgit "github.com/danabrams/gromit/internal/v2/adapter/git"
)

func initIntegrationRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repoDir
}

func TestIntegration_CommitPerStageFlowGitLogParseable(t *testing.T) {
	t.Parallel()
	repoDir := initIntegrationRepo(t)
	worktreesDir := t.TempDir()
	ctx := context.Background()

	ga := execgit.NewExecGitAdapter(repoDir, worktreesDir)
	wtPath, err := ga.Checkout(ctx, "integration-git-log")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	sc := &StageCommitter{Git: ga}

	// Spec-level stage: write a file and commit.
	if err := os.WriteFile(filepath.Join(wtPath, "spec.txt"), []byte("spec"), 0o644); err != nil {
		t.Fatalf("WriteFile spec.txt: %v", err)
	}
	if err := sc.CommitStage(ctx, wtPath, "", "triage", 1, "Proceed"); err != nil {
		t.Fatalf("CommitStage spec-level: %v", err)
	}

	// Bead-level stage: write another file and commit.
	if err := os.WriteFile(filepath.Join(wtPath, "bead.txt"), []byte("bead"), 0o644); err != nil {
		t.Fatalf("WriteFile bead.txt: %v", err)
	}
	if err := sc.CommitStage(ctx, wtPath, "nd56b", "build", 1, "Pass"); err != nil {
		t.Fatalf("CommitStage bead-level: %v", err)
	}

	entries, err := ga.Log(ctx, wtPath, 2)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}
	for _, entry := range entries {
		_, ok := ParseCommitMessage(entry.Message)
		if !ok {
			t.Errorf("commit message not parseable: %q", entry.Message)
		}
	}
}
