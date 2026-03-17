diff --git a/cmd/gromit-next/real_git_ops.go b/cmd/gromit-next/real_git_ops.go
index 6708c1d72..0075f992b 100644
--- a/cmd/gromit-next/real_git_ops.go
+++ b/cmd/gromit-next/real_git_ops.go
@@ -52,6 +52,12 @@ func forceRemoveAll(path string) error {
 	return os.RemoveAll(path)
 }
 
+// RecoverWorktree creates a fresh git worktree after a broken one is removed.
+// It creates a new worktree on the same branch using CreateWorktree.
+func (r *realGitOps) RecoverWorktree(repoDir, branch string) (string, error) {
+	return r.CreateWorktree(repoDir, branch)
+}
+
 // RemoveWorktree removes a git worktree. It first attempts git worktree remove --force,
 // falling back to forceRemoveAll if the git command fails.
 func (r *realGitOps) RemoveWorktree(path string) error {
diff --git a/cmd/gromit-next/stage_provider.go b/cmd/gromit-next/stage_provider.go
index 79d1283e3..cb49f2476 100644
--- a/cmd/gromit-next/stage_provider.go
+++ b/cmd/gromit-next/stage_provider.go
@@ -267,7 +267,9 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		AlwaysRun:   alwaysRun,
 		WorkDir:     p.cfg.WorkDir,
 		EvidenceDir: evidenceDir,
-	}, nil, contractEvaluator)
+		RepoDir:     p.cfg.WorkDir,
+		GitOps:      gitOps,
+	}, eventLog, contractEvaluator)
 
 	reviewStage := stages.NewReviewStage(reviewRunner, stages.ReviewStageConfig{
 		SpecContent:  string(specContent),
diff --git a/internal/next/runstore/events.go b/internal/next/runstore/events.go
index 87a50c0ca..d18a0c6c5 100644
--- a/internal/next/runstore/events.go
+++ b/internal/next/runstore/events.go
@@ -178,6 +178,13 @@ type TerminalStateEvent struct {
 	Reason string `json:"reason,omitempty"`
 }
 
+type WorktreeRecoveryEvent struct {
+	BaseEvent
+	HealthCheckFailure string `json:"health_check_failure"`
+	RecoverySucceeded  bool   `json:"recovery_succeeded"`
+	NewWorktreePath    string `json:"new_worktree_path"`
+}
+
 // --- EventLog ---
 
 // EventLog manages append-only event logging to a JSONL file.
@@ -329,6 +336,9 @@ func unmarshalEvent(data []byte) (TypedEvent, error) {
 	case "scenario_tests_blocked":
 		var e ScenarioTestsBlockedEvent
 		ev = &e
+	case "worktree_recovery":
+		var e WorktreeRecoveryEvent
+		ev = &e
 	default:
 		return nil, fmt.Errorf("unknown event type: %s", peek.Type)
 	}
diff --git a/internal/next/specloop/stages/init.go b/internal/next/specloop/stages/init.go
index 1f27c6b16..79b8f9c23 100644
--- a/internal/next/specloop/stages/init.go
+++ b/internal/next/specloop/stages/init.go
@@ -14,6 +14,7 @@ import (
 // GitOps abstracts git worktree operations for stage implementations.
 type GitOps interface {
 	CreateWorktree(repoDir, branch string) (worktreePath string, err error)
+	RecoverWorktree(repoDir, branch string) (worktreePath string, err error)
 	RemoveWorktree(path string) error
 }
 
diff --git a/internal/next/specloop/stages/init_test.go b/internal/next/specloop/stages/init_test.go
index e8cbea36d..d736fd516 100644
--- a/internal/next/specloop/stages/init_test.go
+++ b/internal/next/specloop/stages/init_test.go
@@ -157,12 +157,15 @@ func TestInitStage_SkipsDifferentSpecBlockedWorktrees(t *testing.T) {
 }
 
 type fakeGitOps struct {
-	createdBranch string
-	worktreePath  string
-	removedPath   string
-	removedPaths  []string
-	createErr     error
-	removeErr     error
+	createdBranch    string
+	recoveredBranch  string
+	worktreePath     string
+	recoverWorktreePath string // separate return path for RecoverWorktree
+	removedPath      string
+	removedPaths     []string
+	createErr        error
+	recoverErr       error // separate error for RecoverWorktree
+	removeErr        error
 }
 
 func (f *fakeGitOps) CreateWorktree(repoDir, branch string) (string, error) {
@@ -170,6 +173,21 @@ func (f *fakeGitOps) CreateWorktree(repoDir, branch string) (string, error) {
 	return f.worktreePath, f.createErr
 }
 
+func (f *fakeGitOps) RecoverWorktree(repoDir, branch string) (string, error) {
+	f.recoveredBranch = branch
+	// Use recoverWorktreePath if set, otherwise fall back to worktreePath
+	path := f.recoverWorktreePath
+	if path == "" {
+		path = f.worktreePath
+	}
+	// Use recoverErr if set, otherwise fall back to createErr for backward compatibility
+	err := f.recoverErr
+	if err == nil {
+		err = f.createErr
+	}
+	return path, err
+}
+
 func (f *fakeGitOps) RemoveWorktree(path string) error {
 	f.removedPath = path
 	f.removedPaths = append(f.removedPaths, path)
diff --git a/internal/next/specloop/stages/validate.go b/internal/next/specloop/stages/validate.go
index 0eee5e722..f7b5cfb2e 100644
--- a/internal/next/specloop/stages/validate.go
+++ b/internal/next/specloop/stages/validate.go
@@ -25,6 +25,8 @@ type ValidateStageConfig struct {
 	ProjectChecks []validator.Check
 	WorkDir       string
 	EvidenceDir   string
+	RepoDir       string
+	GitOps        GitOps
 }
 
 // ValidateStage runs final validation checks.
@@ -33,6 +35,7 @@ type ValidateStage struct {
 	contractEvaluator contract.ContractEvaluator
 	cfg               ValidateStageConfig
 	eventLog          *runstore.EventLog
+	gitOps            GitOps // injected for testing; if set, used instead of cfg.GitOps
 }
 
 // NewValidateStage creates a new ValidateStage. An optional ContractEvaluator may be
@@ -41,9 +44,92 @@ func NewValidateStage(v FinalValidator, cfg ValidateStageConfig, eventLog *runst
 	return &ValidateStage{validator: v, cfg: cfg, eventLog: eventLog, contractEvaluator: evaluator}
 }
 
+// recoverWorktree removes a broken worktree and recovers a new one.
+// It constructs the branch name as 'gromit/spec-{SpecID}-{RunID}' matching InitStage pattern,
+// and updates rs.WorktreePath with the new path if successful.
+func (s *ValidateStage) recoverWorktree(ctx context.Context, workDir string, healthErr error, rs *runstore.RunState) (string, error) {
+	// Use injected gitOps if set (for testing), otherwise use cfg.GitOps
+	gitOps := s.gitOps
+	if gitOps == nil {
+		gitOps = s.cfg.GitOps
+	}
+
+	// Remove the broken worktree
+	removeErr := gitOps.RemoveWorktree(workDir)
+	if removeErr != nil {
+		// Log event before returning
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.WorktreeRecoveryEvent{
+				BaseEvent: runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
+				HealthCheckFailure: healthErr.Error(),
+				RecoverySucceeded:  false,
+			})
+		}
+		return "", fmt.Errorf("worktree cleanup failed: %w", removeErr)
+	}
+
+	// Attempt recovery with branch name matching InitStage pattern
+	branch := fmt.Sprintf("gromit/spec-%s-%s", rs.SpecID, rs.RunID)
+	newWorktreePath, recoverErr := gitOps.RecoverWorktree(s.cfg.RepoDir, branch)
+
+	// Log event
+	if s.eventLog != nil {
+		s.eventLog.Append(runstore.WorktreeRecoveryEvent{
+			BaseEvent: runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
+			HealthCheckFailure: healthErr.Error(),
+			RecoverySucceeded:  recoverErr == nil,
+			NewWorktreePath:    newWorktreePath,
+		})
+	}
+
+	if recoverErr != nil {
+		return "", fmt.Errorf("failed to recover worktree: %w", recoverErr)
+	}
+
+	// Update RunState with new path
+	rs.WorktreePath = newWorktreePath
+
+	return newWorktreePath, nil
+}
+
 // Name returns the stage name.
 func (s *ValidateStage) Name() string { return "validate" }
 
+// checkWorktreeHealth verifies that a directory is a healthy worktree by checking:
+// 1. Directory exists
+// 2. .git file exists in directory
+// 3. go.mod file exists in directory
+// Returns nil if healthy, descriptive error if not.
+func (s *ValidateStage) checkWorktreeHealth(workDir string) error {
+	// Check if directory exists
+	if _, err := os.Stat(workDir); err != nil {
+		if errors.Is(err, os.ErrNotExist) {
+			return fmt.Errorf("worktree directory does not exist: %s", workDir)
+		}
+		return fmt.Errorf("failed to stat worktree directory: %w", err)
+	}
+
+	// Check if .git exists
+	gitPath := filepath.Join(workDir, ".git")
+	if _, err := os.Stat(gitPath); err != nil {
+		if errors.Is(err, os.ErrNotExist) {
+			return fmt.Errorf(".git does not exist in worktree: %s", gitPath)
+		}
+		return fmt.Errorf("failed to stat .git in worktree: %w", err)
+	}
+
+	// Check if go.mod exists
+	modPath := filepath.Join(workDir, "go.mod")
+	if _, err := os.Stat(modPath); err != nil {
+		if errors.Is(err, os.ErrNotExist) {
+			return fmt.Errorf("go.mod does not exist in worktree: %s", modPath)
+		}
+		return fmt.Errorf("failed to stat go.mod in worktree: %w", err)
+	}
+
+	return nil
+}
+
 // Run executes final validation.
 func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
 	workDir := s.cfg.WorkDir
@@ -51,6 +137,70 @@ func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloo
 		workDir = rs.WorktreePath
 	}
 
+	// Use injected gitOps if set (for testing), otherwise use cfg.GitOps
+	gitOps := s.gitOps
+	if gitOps == nil {
+		gitOps = s.cfg.GitOps
+	}
+
+	// Check worktree health every cycle (not gated by prior success)
+	// This ensures broken worktrees are detected and recovered even after previous successful cycles
+	if healthErr := s.checkWorktreeHealth(workDir); healthErr != nil && gitOps != nil {
+		// Health check failed - attempt recovery
+		// First remove the broken worktree
+		removeErr := gitOps.RemoveWorktree(workDir)
+		if removeErr != nil {
+			// RemoveWorktree failed - return Blocked with cleanup error
+			errMsg := fmt.Sprintf("infrastructure: failed to remove broken worktree: %v", removeErr)
+			if s.eventLog != nil {
+				s.eventLog.Append(runstore.WorktreeRecoveryEvent{
+					BaseEvent: runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
+					HealthCheckFailure: healthErr.Error(),
+					RecoverySucceeded:  false,
+				})
+			}
+			return specloop.NextAction{
+				Kind: specloop.Blocked,
+				Context: &specloop.FailureContext{
+					Failures: []string{errMsg},
+					Cycle:    rs.Cycle,
+				},
+			}, nil
+		}
+
+		// Now attempt recovery
+		branch := fmt.Sprintf("gromit/spec-%s-%s", rs.SpecID, rs.RunID)
+		newWorktreePath, recoverErr := gitOps.RecoverWorktree(s.cfg.RepoDir, branch)
+
+		recoverySucceeded := recoverErr == nil
+
+		// Emit WorktreeRecoveryEvent
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.WorktreeRecoveryEvent{
+				BaseEvent: runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
+				HealthCheckFailure: healthErr.Error(),
+				RecoverySucceeded:  recoverySucceeded,
+				NewWorktreePath:    newWorktreePath,
+			})
+		}
+
+		if recoverErr != nil {
+			// Recovery failed - return Blocked with infrastructure error
+			errMsg := fmt.Sprintf("infrastructure: failed to recover worktree: %v", recoverErr)
+			return specloop.NextAction{
+				Kind: specloop.Blocked,
+				Context: &specloop.FailureContext{
+					Failures: []string{errMsg},
+					Cycle:    rs.Cycle,
+				},
+			}, nil
+		}
+
+		// Recovery succeeded - update workDir to the recovered path
+		workDir = newWorktreePath
+		rs.WorktreePath = newWorktreePath
+	}
+
 	// Collect contract failures first (if EvidenceDir is configured and file exists).
 	var failures []string
 	if s.cfg.EvidenceDir != "" && s.contractEvaluator != nil {
diff --git a/internal/next/specloop/stages/validate_test.go b/internal/next/specloop/stages/validate_test.go
index 97e80f16c..664e495da 100644
--- a/internal/next/specloop/stages/validate_test.go
+++ b/internal/next/specloop/stages/validate_test.go
@@ -2,6 +2,7 @@ package stages
 
 import (
 	"context"
+	"fmt"
 	"os"
 	"path/filepath"
 	"strings"
@@ -232,3 +233,364 @@ func TestValidateStage_ContractAndShellFailures(t *testing.T) {
 		t.Fatalf("expected first failure to be contract failure, got %q", action.Context.Failures[0])
 	}
 }
+
+// TestValidateStage_HealthCheck verifies that checkWorktreeHealth detects directory health.
+func TestValidateStage_HealthCheck(t *testing.T) {
+	// Test healthy worktree with .git and go.mod
+	healthyDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(healthyDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	if err := os.WriteFile(filepath.Join(healthyDir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
+		t.Fatalf("write go.mod: %v", err)
+	}
+
+	stage := NewValidateStage(nil, ValidateStageConfig{WorkDir: healthyDir}, nil, nil)
+	err := stage.checkWorktreeHealth(healthyDir)
+	if err != nil {
+		t.Fatalf("expected healthy worktree, got error: %v", err)
+	}
+
+	// Test missing .git
+	missingGitDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(missingGitDir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
+		t.Fatalf("write go.mod: %v", err)
+	}
+	err = stage.checkWorktreeHealth(missingGitDir)
+	if err == nil {
+		t.Fatal("expected error for missing .git")
+	}
+	if !strings.Contains(err.Error(), ".git") {
+		t.Fatalf("expected error mentioning .git, got: %v", err)
+	}
+
+	// Test missing go.mod
+	missingModDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(missingModDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	err = stage.checkWorktreeHealth(missingModDir)
+	if err == nil {
+		t.Fatal("expected error for missing go.mod")
+	}
+	if !strings.Contains(err.Error(), "go.mod") {
+		t.Fatalf("expected error mentioning go.mod, got: %v", err)
+	}
+
+	// Test nonexistent directory
+	err = stage.checkWorktreeHealth("/nonexistent/path/12345")
+	if err == nil {
+		t.Fatal("expected error for nonexistent directory")
+	}
+}
+
+// TestValidateStage_HealthCheckFail_RecoverySucceeds verifies that when worktree health
+// check fails, recovery is attempted via GitOps, and if successful, validation proceeds normally.
+func TestValidateStage_HealthCheckFail_RecoverySucceeds(t *testing.T) {
+	// Create a broken worktree directory (missing go.mod)
+	brokenDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(brokenDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	// Missing go.mod to trigger health check failure
+
+	// Setup healthy recovery directory
+	healthyDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(healthyDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	if err := os.WriteFile(filepath.Join(healthyDir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
+		t.Fatalf("write go.mod: %v", err)
+	}
+
+	v := &fakeValidator{
+		result: validator.FinalResult{
+			Pass: true,
+			AlwaysRun: validator.CheckResults{
+				Results: []validator.CheckResult{{Name: "test", Pass: true}},
+			},
+			ProjectChecks: validator.CheckResults{
+				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
+			},
+		},
+	}
+
+	gitOps := &fakeGitOps{
+		recoverWorktreePath: healthyDir,
+		recoverErr:          nil,
+	}
+
+	eventLog := runstore.NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
+
+	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: brokenDir, RepoDir: "/fake/repo"}, eventLog, nil)
+	stage.gitOps = gitOps
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	rs.WorktreePath = brokenDir
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	// After successful recovery, validation should proceed and return Continue
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue after successful recovery, got %v", action.Kind)
+	}
+
+	// WorktreePath should be updated to recovered directory
+	if rs.WorktreePath != healthyDir {
+		t.Fatalf("expected WorktreePath to be updated to %s, got %s", healthyDir, rs.WorktreePath)
+	}
+
+	// RemoveWorktree should have been called with the broken directory
+	if len(gitOps.removedPaths) != 1 || gitOps.removedPaths[0] != brokenDir {
+		t.Fatalf("expected RemoveWorktree to be called with broken directory, got %v", gitOps.removedPaths)
+	}
+}
+
+// TestValidateStage_HealthCheckFail_RecoveryFails_Blocked verifies that when health check
+// fails and recovery fails, a Blocked action is returned with 'infrastructure: ' prefix.
+func TestValidateStage_HealthCheckFail_RecoveryFails_Blocked(t *testing.T) {
+	// Create a broken worktree directory (missing go.mod)
+	brokenDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(brokenDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	// Missing go.mod to trigger health check failure
+
+	gitOps := &fakeGitOps{
+		recoverWorktreePath: "",
+		recoverErr:          fmt.Errorf("recovery failed: git error"),
+	}
+
+	eventLog := runstore.NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
+
+	stage := NewValidateStage(nil, ValidateStageConfig{WorkDir: brokenDir, RepoDir: "/fake/repo"}, eventLog, nil)
+	stage.gitOps = gitOps
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	rs.WorktreePath = brokenDir
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	// Should return Blocked action when recovery fails
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked action, got %v", action.Kind)
+	}
+
+	// BlockerSummary should have 'infrastructure: ' prefix
+	if action.Context == nil || len(action.Context.Failures) == 0 {
+		t.Fatal("expected FailureContext with failures")
+	}
+
+	failure := action.Context.Failures[0]
+	if !strings.HasPrefix(failure, "infrastructure: ") {
+		t.Fatalf("expected failure to start with 'infrastructure: ', got %q", failure)
+	}
+
+	// RemoveWorktree should have been called before recovery attempt
+	if len(gitOps.removedPaths) != 1 || gitOps.removedPaths[0] != brokenDir {
+		t.Fatalf("expected RemoveWorktree to be called with broken directory")
+	}
+}
+
+// TestValidateStage_RecoveryEmitsEvent verifies that WorktreeRecoveryEvent is emitted
+// when recovery is attempted, with correct fields populated.
+func TestValidateStage_RecoveryEmitsEvent(t *testing.T) {
+	// Create a broken worktree directory (missing go.mod)
+	brokenDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(brokenDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	// Missing go.mod to trigger health check failure
+
+	// Setup healthy recovery directory
+	healthyDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(healthyDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	if err := os.WriteFile(filepath.Join(healthyDir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
+		t.Fatalf("write go.mod: %v", err)
+	}
+
+	v := &fakeValidator{
+		result: validator.FinalResult{
+			Pass:          true,
+			AlwaysRun:     validator.CheckResults{},
+			ProjectChecks: validator.CheckResults{},
+		},
+	}
+
+	gitOps := &fakeGitOps{
+		recoverWorktreePath: healthyDir,
+		recoverErr:          nil,
+	}
+
+	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+
+	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: brokenDir, RepoDir: "/fake/repo"}, eventLog, nil)
+	stage.gitOps = gitOps
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	rs.WorktreePath = brokenDir
+
+	_, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	// Read the event log and verify WorktreeRecoveryEvent was emitted
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("failed to read event log: %v", err)
+	}
+
+	var recoveryEvent *runstore.WorktreeRecoveryEvent
+	for _, ev := range events {
+		if r, ok := ev.(*runstore.WorktreeRecoveryEvent); ok {
+			recoveryEvent = r
+			break
+		}
+	}
+
+	if recoveryEvent == nil {
+		t.Fatal("expected WorktreeRecoveryEvent to be emitted")
+	}
+
+	// Verify event fields
+	if recoveryEvent.RecoverySucceeded != true {
+		t.Errorf("expected RecoverySucceeded=true, got %v", recoveryEvent.RecoverySucceeded)
+	}
+
+	if recoveryEvent.NewWorktreePath != healthyDir {
+		t.Errorf("expected NewWorktreePath=%s, got %s", healthyDir, recoveryEvent.NewWorktreePath)
+	}
+
+	// HealthCheckFailure should mention missing go.mod
+	if !strings.Contains(recoveryEvent.HealthCheckFailure, "go.mod") {
+		t.Errorf("expected HealthCheckFailure to mention go.mod, got %q", recoveryEvent.HealthCheckFailure)
+	}
+}
+
+// TestValidateStage_RemoveWorktreeFails_Blocked verifies that when RemoveWorktree
+// fails during recovery attempt, a Blocked action is returned with cleanup error.
+func TestValidateStage_RemoveWorktreeFails_Blocked(t *testing.T) {
+	// Create a broken worktree directory (missing go.mod)
+	brokenDir := t.TempDir()
+	if err := os.WriteFile(filepath.Join(brokenDir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	// Missing go.mod to trigger health check failure
+
+	gitOps := &fakeGitOps{
+		recoverWorktreePath: "",
+		recoverErr:          nil,
+		removeErr:           fmt.Errorf("permission denied"),
+	}
+
+	eventLog := runstore.NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
+
+	stage := NewValidateStage(nil, ValidateStageConfig{WorkDir: brokenDir, RepoDir: "/fake/repo"}, eventLog, nil)
+	stage.gitOps = gitOps
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	rs.WorktreePath = brokenDir
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	// Should return Blocked action when RemoveWorktree fails
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked action, got %v", action.Kind)
+	}
+
+	// Failures should mention the cleanup error
+	if action.Context == nil || len(action.Context.Failures) == 0 {
+		t.Fatal("expected FailureContext with failures")
+	}
+
+	failure := action.Context.Failures[0]
+	if !strings.Contains(failure, "infrastructure: ") {
+		t.Fatalf("expected failure to contain 'infrastructure: ', got %q", failure)
+	}
+}
+
+// TestCheckWorktreeHealth_Healthy verifies that checkWorktreeHealth returns nil when
+// .git and go.mod exist in a valid directory.
+func TestCheckWorktreeHealth_Healthy(t *testing.T) {
+	dir := t.TempDir()
+
+	// Create .git and go.mod files
+	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
+		t.Fatalf("write go.mod: %v", err)
+	}
+
+	stage := NewValidateStage(nil, ValidateStageConfig{}, nil, nil)
+	err := stage.checkWorktreeHealth(dir)
+	if err != nil {
+		t.Fatalf("expected healthy worktree, got error: %v", err)
+	}
+}
+
+// TestCheckWorktreeHealth_MissingDirectory verifies that checkWorktreeHealth returns
+// a descriptive error when the directory does not exist.
+func TestCheckWorktreeHealth_MissingDirectory(t *testing.T) {
+	stage := NewValidateStage(nil, ValidateStageConfig{}, nil, nil)
+	err := stage.checkWorktreeHealth("/nonexistent/path/12345")
+	if err == nil {
+		t.Fatal("expected error for missing directory")
+	}
+	if !strings.Contains(err.Error(), "does not exist") {
+		t.Fatalf("expected error mentioning 'does not exist', got: %v", err)
+	}
+}
+
+// TestCheckWorktreeHealth_MissingGitFile verifies that checkWorktreeHealth returns
+// an error mentioning .git when the .git file is missing.
+func TestCheckWorktreeHealth_MissingGitFile(t *testing.T) {
+	dir := t.TempDir()
+
+	// Create only go.mod, missing .git
+	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
+		t.Fatalf("write go.mod: %v", err)
+	}
+
+	stage := NewValidateStage(nil, ValidateStageConfig{}, nil, nil)
+	err := stage.checkWorktreeHealth(dir)
+	if err == nil {
+		t.Fatal("expected error for missing .git file")
+	}
+	if !strings.Contains(err.Error(), ".git") {
+		t.Fatalf("expected error mentioning '.git', got: %v", err)
+	}
+}
+
+// TestCheckWorktreeHealth_MissingGoMod verifies that checkWorktreeHealth returns
+// an error mentioning go.mod when the go.mod file is missing.
+func TestCheckWorktreeHealth_MissingGoMod(t *testing.T) {
+	dir := t.TempDir()
+
+	// Create only .git, missing go.mod
+	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte(""), 0o644); err != nil {
+		t.Fatalf("write .git: %v", err)
+	}
+
+	stage := NewValidateStage(nil, ValidateStageConfig{}, nil, nil)
+	err := stage.checkWorktreeHealth(dir)
+	if err == nil {
+		t.Fatal("expected error for missing go.mod file")
+	}
+	if !strings.Contains(err.Error(), "go.mod") {
+		t.Fatalf("expected error mentioning 'go.mod', got: %v", err)
+	}
+}