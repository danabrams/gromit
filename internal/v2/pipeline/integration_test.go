//go:build integration

package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	execgit "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/event"
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
