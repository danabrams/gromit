package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitWorktreeStagesCodeAndEvents(t *testing.T) {
	t.Parallel()

	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	ga := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()
	wtPath, err := ga.Checkout(ctx, "commit-helper")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	codePath := filepath.Join(wtPath, "internal", "helper.go")
	if err := os.MkdirAll(filepath.Dir(codePath), 0o755); err != nil {
		t.Fatalf("MkdirAll code dir: %v", err)
	}
	if err := os.WriteFile(codePath, []byte("package helper\n"), 0o644); err != nil {
		t.Fatalf("WriteFile code: %v", err)
	}

	gitignorePath := filepath.Join(wtPath, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(".gromit/v2/events.jsonl\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .gitignore: %v", err)
	}

	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll events dir: %v", err)
	}
	if err := os.WriteFile(eventsPath, []byte("{\"type\":\"helper\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile events: %v", err)
	}

	commitHash, err := CommitWorktree(ctx, wtPath, "helper commit")
	if err != nil {
		t.Fatalf("CommitWorktree: %v", err)
	}

	diff, err := ga.Show(ctx, wtPath, commitHash)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if !strings.Contains(diff, "internal/helper.go") {
		t.Fatalf("commit diff missing code artifact: %s", diff)
	}
	if !strings.Contains(diff, "diff --git a/.gromit/v2/events.jsonl b/.gromit/v2/events.jsonl") {
		t.Fatalf("commit diff missing events log update: %s", diff)
	}
}

func TestStageCommitterCommitStageCommitsCodeAndEventsWithStructuredMessage(t *testing.T) {
	t.Parallel()

	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	ga := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()
	wtPath, err := ga.Checkout(ctx, "spec-stage-commit")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	codePath := filepath.Join(wtPath, "internal", "foo.go")
	if err := os.MkdirAll(filepath.Dir(codePath), 0o755); err != nil {
		t.Fatalf("MkdirAll code dir: %v", err)
	}
	if err := os.WriteFile(codePath, []byte("package internal\n"), 0o644); err != nil {
		t.Fatalf("WriteFile code: %v", err)
	}

	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll events dir: %v", err)
	}
	if err := os.WriteFile(eventsPath, []byte("{\"type\":\"stage.completed\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile events: %v", err)
	}

	sc := &StageCommitter{Git: ga}
	if err := sc.CommitStage(ctx, wtPath, "", "plan", 1, "proceed"); err != nil {
		t.Fatalf("CommitStage: %v", err)
	}

	entries, err := ga.Log(ctx, wtPath, 1)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	if got, want := entries[0].Message, "[spec/plan/iter:1] proceed"; got != want {
		t.Fatalf("commit message = %q, want %q", got, want)
	}

	diff, err := ga.Show(ctx, wtPath, entries[0].Hash)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if !strings.Contains(diff, "internal/foo.go") {
		t.Fatalf("commit diff missing code artifact: %s", diff)
	}
	if !strings.Contains(diff, "diff --git a/.gromit/v2/events.jsonl b/.gromit/v2/events.jsonl") {
		t.Fatalf("commit diff missing events log update: %s", diff)
	}
}

func TestStageCommitterCommitStageAddsIgnoredEventsFile(t *testing.T) {
	t.Parallel()

	repoDir := initTestRepo(t)
	worktreesDir := t.TempDir()

	ga := NewExecGitAdapter(repoDir, worktreesDir)
	ctx := context.Background()
	wtPath, err := ga.Checkout(ctx, "spec-stage-commit-ignored-events")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	gitignorePath := filepath.Join(wtPath, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(".gromit/v2/events.jsonl\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .gitignore: %v", err)
	}

	codePath := filepath.Join(wtPath, "internal", "foo.go")
	if err := os.MkdirAll(filepath.Dir(codePath), 0o755); err != nil {
		t.Fatalf("MkdirAll code dir: %v", err)
	}
	if err := os.WriteFile(codePath, []byte("package internal\n"), 0o644); err != nil {
		t.Fatalf("WriteFile code: %v", err)
	}

	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll events dir: %v", err)
	}
	if err := os.WriteFile(eventsPath, []byte("{\"type\":\"stage.completed\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile events: %v", err)
	}

	sc := &StageCommitter{Git: ga}
	if err := sc.CommitStage(ctx, wtPath, "", "plan", 1, "proceed"); err != nil {
		t.Fatalf("CommitStage: %v", err)
	}

	entries, err := ga.Log(ctx, wtPath, 1)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}

	diff, err := ga.Show(ctx, wtPath, entries[0].Hash)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if !strings.Contains(diff, "diff --git a/.gromit/v2/events.jsonl b/.gromit/v2/events.jsonl") {
		t.Fatalf("commit diff missing events log update: %s", diff)
	}
}
