package stages

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestScenario_RecoveryFails verifies that when worktree recovery fails,
// the stage sets rs.BlockerSummary with an "infrastructure: " prefix and
// returns Blocked. No contract or shell check failures are generated,
// no replan is triggered, and a WorktreeRecoveryEvent is emitted with
// RecoverySucceeded: false.
func TestScenario_RecoveryFails(t *testing.T) {
	cases := []struct {
		name            string
		setupWorktree   func(t *testing.T, dir string) string // returns worktree path
		wantHealthMsg   string                                // substring expected in blocker summary
		wantEventHealth string                                // substring expected in event HealthCheckFailure
	}{
		{
			name: "git_file_missing",
			setupWorktree: func(t *testing.T, dir string) string {
				t.Helper()
				wt := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
				if err := os.MkdirAll(wt, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				// go.mod present but .git missing
				if err := os.WriteFile(filepath.Join(wt, "go.mod"), []byte("module test\n"), 0o644); err != nil {
					t.Fatalf("write go.mod: %v", err)
				}
				return wt
			},
			wantHealthMsg:   ".git file missing in",
			wantEventHealth: ".git file missing",
		},
		{
			name: "gomod_missing",
			setupWorktree: func(t *testing.T, dir string) string {
				t.Helper()
				wt := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
				if err := os.MkdirAll(wt, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				// .git present but go.mod missing
				if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
					t.Fatalf("write .git: %v", err)
				}
				return wt
			},
			wantHealthMsg:   "go.mod not found in",
			wantEventHealth: "go.mod not found",
		},
		{
			name: "directory_missing",
			setupWorktree: func(t *testing.T, dir string) string {
				t.Helper()
				// Return a path that does not exist
				return filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
			},
			wantHealthMsg:   "directory does not exist:",
			wantEventHealth: "directory does not exist",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			// Seed: broken worktree per sub-case
			brokenWorktree := tc.setupWorktree(t, dir)

			// Seed: event log to capture WorktreeRecoveryEvent
			eventLogPath := filepath.Join(dir, "events.jsonl")
			eventLog := runstore.NewEventLog(eventLogPath)

			// Seed: fakeGitOps where RecoverWorktree returns an error
			fakeGit := &validateScenarioFakeGitOps{
				recoverErr: errors.New("disk full: cannot create worktree"),
			}

			// Use a passing validator — recovery failure should prevent it from running
			v := &fakeValidator{
				result: validator.FinalResult{
					Pass: true,
					AlwaysRun: validator.CheckResults{
						Results: []validator.CheckResult{{Name: "go test ./...", Pass: true}},
					},
					ProjectChecks: validator.CheckResults{
						Results: []validator.CheckResult{{Name: "lint", Pass: true}},
					},
				},
			}

			stage := NewValidateStage(v, ValidateStageConfig{
				WorkDir: "/tmp/work",
				RepoDir: dir,
			}, eventLog, nil, fakeGit)

			rs := runstore.NewRunState("spec-001", "proj-001")
			rs.WorktreePath = brokenWorktree
			rs.SpecID = "spec-001"
			rs.RunID = "run-001"

			// Invoke
			action, err := stage.Run(context.Background(), rs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Assert: stage returns Blocked (not ReplanFrom)
			if action.Kind != specloop.Blocked {
				t.Fatalf("expected Blocked, got %v", action.Kind)
			}

			// Assert: no replan context (no failures list)
			if action.Context != nil {
				t.Fatalf("expected nil FailureContext (no replan), got %+v", action.Context)
			}

			// Assert: BlockerSummary starts with "infrastructure: "
			if !strings.HasPrefix(rs.BlockerSummary, "infrastructure: ") {
				t.Fatalf("expected BlockerSummary to start with 'infrastructure: ', got %q", rs.BlockerSummary)
			}

			// Assert: BlockerSummary contains the health check failure description
			if !strings.Contains(rs.BlockerSummary, tc.wantHealthMsg) {
				t.Fatalf("expected BlockerSummary to contain %q, got %q", tc.wantHealthMsg, rs.BlockerSummary)
			}

			// Assert: BlockerSummary contains the recovery error detail
			if !strings.Contains(rs.BlockerSummary, "recovery error:") {
				t.Fatalf("expected BlockerSummary to contain 'recovery error:', got %q", rs.BlockerSummary)
			}

			// Assert: WorktreeRecoveryEvent emitted with RecoverySucceeded=false
			events, err := eventLog.ReadAll()
			if err != nil {
				t.Fatalf("ReadAll events: %v", err)
			}
			var foundRecoveryEvent bool
			for _, ev := range events {
				if wre, ok := ev.(*runstore.WorktreeRecoveryEvent); ok {
					foundRecoveryEvent = true
					if wre.RecoverySucceeded {
						t.Error("expected RecoverySucceeded=false in WorktreeRecoveryEvent")
					}
					if wre.NewWorktreePath != "" {
						t.Errorf("expected empty NewWorktreePath on failure, got %q", wre.NewWorktreePath)
					}
					if !strings.Contains(wre.HealthCheckFailure, tc.wantEventHealth) {
						t.Errorf("expected HealthCheckFailure to contain %q, got %q", tc.wantEventHealth, wre.HealthCheckFailure)
					}
				}
			}
			if !foundRecoveryEvent {
				t.Error("expected WorktreeRecoveryEvent to be emitted")
			}

			// Assert: no contract or shell check failures generated (no ReplanFrom)
			// Already verified by action.Kind == Blocked and action.Context == nil above.

			// Assert: FinalValidationPassed was NOT set (validation never ran)
			if rs.FinalValidationPassed {
				t.Error("expected FinalValidationPassed to be false since recovery failed before validation")
			}
		})
	}
}
