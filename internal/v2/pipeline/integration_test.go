//go:build integration

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	execgit "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/presentation"
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

func TestIntegration_EventsJSONLCumulativeValidLines(t *testing.T) {
	t.Parallel()
	eventsPath := filepath.Join(t.TempDir(), "events", "events.jsonl")

	emitter := event.NewEmitter()
	sub := event.NewFileSubscriber(eventsPath)
	sub.SubscribeTo(emitter)

	emitter.Emit(event.SpecStartedEvent{
		Event:  event.Event{SchemaVersion: event.SchemaVersion, Type: event.EventTypeSpecStarted},
		SpecID: "test-spec",
	})
	emitter.Emit(event.BeadStartedEvent{
		Event:  event.Event{SchemaVersion: event.SchemaVersion, Type: event.EventTypeBeadStarted},
		BeadID: "nd56b",
	})

	emitter.Close()
	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close: %v", err)
	}

	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile events.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in events.jsonl, got %d: %q", len(lines), data)
	}
	for i, line := range lines {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("line %d not valid JSON: %v: %q", i, err, line)
		}
	}
}

func TestIntegration_ImmutablePipeline_RealRepoEndToEnd(t *testing.T) {
	t.Parallel()
	repoDir := initIntegrationRepo(t)
	worktreesDir := t.TempDir()
	ctx := context.Background()
	specID := "integration-immutable-pipeline"

	ga := execgit.NewExecGitAdapter(repoDir, worktreesDir)
	wtPath, err := ga.Checkout(ctx, specID)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	stageCommitter := &StageCommitter{Git: ga}
	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	fileSub := event.NewFileSubscriber(eventsPath)
	t.Cleanup(func() {
		_ = fileSub.Close()
	})

	type stageStep struct {
		beadID    string
		stageName string
		iteration int
		decision  string
		filePath  string
		content   string
		success   bool
	}
	steps := []stageStep{
		{stageName: "plan", iteration: 1, decision: "Proceed", filePath: "spec/plan.md", content: "plan", success: true},
		{beadID: "001", stageName: "build", iteration: 1, decision: "Proceed", filePath: "beads/001.txt", content: "build-1", success: true},
		{beadID: "001", stageName: "validate", iteration: 1, decision: "Fail", filePath: "beads/001.txt", content: "validate-1-fail", success: false},
		{beadID: "001", stageName: "build", iteration: 2, decision: "Proceed", filePath: "beads/001.txt", content: "build-2", success: true},
		{beadID: "001", stageName: "validate", iteration: 2, decision: "Proceed", filePath: "beads/001.txt", content: "validate-2", success: true},
		{beadID: "002", stageName: "build", iteration: 1, decision: "Proceed", filePath: "beads/002.txt", content: "build-1", success: true},
		{beadID: "002", stageName: "validate", iteration: 1, decision: "Proceed", filePath: "beads/002.txt", content: "validate-1", success: true},
	}

	for _, step := range steps {
		writePath := filepath.Join(wtPath, step.filePath)
		if err := appendLine(writePath, fmt.Sprintf("%s:%s", step.stageName, step.content)); err != nil {
			t.Fatalf("appendLine %s: %v", step.filePath, err)
		}

		fileSub.Handle(event.StageCompletedEvent{
			Event: event.Event{
				SchemaVersion: event.SchemaVersion,
				Type:          event.EventTypeStageCompleted,
			},
			StageName: step.stageName,
			BeadID:    step.beadID,
			Iteration: step.iteration,
			Success:   step.success,
		})

		if err := stageCommitter.CommitStage(ctx, wtPath, step.beadID, step.stageName, step.iteration, step.decision); err != nil {
			t.Fatalf("CommitStage bead=%q stage=%q iter=%d: %v", step.beadID, step.stageName, step.iteration, err)
		}
	}

	if err := fileSub.Close(); err != nil {
		t.Fatalf("fileSub.Close: %v", err)
	}

	entries, err := ga.Log(ctx, wtPath, len(steps))
	if err != nil {
		t.Fatalf("Log before squash: %v", err)
	}
	if len(entries) != len(steps) {
		t.Fatalf("expected %d structured commits, got %d", len(steps), len(entries))
	}

	for i, entry := range entries {
		info, ok := ParseCommitMessage(entry.Message)
		if !ok {
			t.Fatalf("entry %d message not parseable: %q", i, entry.Message)
		}
		want := steps[len(steps)-1-i]
		if info.BeadID != want.beadID || info.StageName != want.stageName || info.Iteration != want.iteration || info.Decision != want.decision {
			t.Fatalf("entry %d parsed info = %+v, want bead=%q stage=%q iter=%d decision=%q", i, info, want.beadID, want.stageName, want.iteration, want.decision)
		}
		diff, err := ga.Show(ctx, wtPath, entry.Hash)
		if err != nil {
			t.Fatalf("Show %s: %v", entry.Hash, err)
		}
		if !strings.Contains(diff, ".gromit/v2/events.jsonl") {
			t.Fatalf("commit %s does not include events log update", entry.Hash)
		}
	}

	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("ReadFile events.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != len(steps) {
		t.Fatalf("events.jsonl lines = %d, want %d", len(lines), len(steps))
	}
	for i, line := range lines {
		var decoded struct {
			Type      string `json:"type"`
			StageName string `json:"stage_name"`
			BeadID    string `json:"bead_id"`
			Iteration int    `json:"iteration"`
		}
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("json line %d decode: %v", i, err)
		}
		want := steps[i]
		if decoded.Type != event.EventTypeStageCompleted || decoded.StageName != want.stageName || decoded.BeadID != want.beadID || decoded.Iteration != want.iteration {
			t.Fatalf("events line %d = %+v, want type=%q stage=%q bead=%q iter=%d", i, decoded, event.EventTypeStageCompleted, want.stageName, want.beadID, want.iteration)
		}
	}

	branch := strings.TrimSpace(runGit(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD"))
	wantBranch := "gromit/spec/" + specID
	if branch != wantBranch {
		t.Fatalf("worktree branch = %q, want %q", branch, wantBranch)
	}
	if !worktreeListed(t, repoDir, wtPath) {
		t.Fatalf("worktree %q should be registered before cleanup", wtPath)
	}

	if err := SquashPerBead(ctx, ga, wtPath, []presentation.BeadSummary{
		{ID: "001", Title: "First bead"},
		{ID: "002", Title: "Second bead"},
	}); err != nil {
		t.Fatalf("SquashPerBead: %v", err)
	}

	squashedEntries, err := ga.Log(ctx, wtPath, 10)
	if err != nil {
		t.Fatalf("Log after squash: %v", err)
	}
	wantMessages := []string{
		"bead 002: Second bead",
		"bead 001: First bead",
		"initial",
	}
	if len(squashedEntries) != len(wantMessages) {
		t.Fatalf("squashed commit count = %d, want %d (%v)", len(squashedEntries), len(wantMessages), squashedEntries)
	}
	for i, want := range wantMessages {
		if squashedEntries[i].Message != want {
			t.Fatalf("squashedEntries[%d].Message = %q, want %q", i, squashedEntries[i].Message, want)
		}
		if _, ok := ParseCommitMessage(squashedEntries[i].Message); ok {
			t.Fatalf("structured commit left after squash: %q", squashedEntries[i].Message)
		}
	}

	if err := ga.RemoveWorktree(ctx, wtPath); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree removed after success cleanup, stat err = %v", err)
	}
	if worktreeListed(t, repoDir, wtPath) {
		t.Fatalf("worktree %q should not be registered after cleanup", wtPath)
	}
}

func appendLine(path, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func worktreeListed(t *testing.T, repoDir, wtPath string) bool {
	t.Helper()
	return strings.Contains(runGit(t, repoDir, "worktree", "list", "--porcelain"), wtPath)
}
