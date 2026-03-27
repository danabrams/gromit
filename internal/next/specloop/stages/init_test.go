package stages

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/testutil"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestInitStage_CleansBlockedWorktrees(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

	// Create a prior run with the SAME spec+project so store.List finds it
	priorRS := runstore.NewRunState("test-spec", "test-project")
	priorRS.Status = runstore.StatusBlocked
	priorRS.WorktreePath = filepath.Join(t.TempDir(), "old-worktree")
	os.MkdirAll(priorRS.WorktreePath, 0o755)
	store.Save(priorRS)

	eventLog := runstore.NewEventLog(filepath.Join(storeDir, "events.jsonl"))

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

	// Verify blocked_worktree_cleaned event was emitted with correct path
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var foundCleanEvent bool
	for _, ev := range events {
		if bwc, ok := ev.(*runstore.BlockedWorktreeCleanedEvent); ok {
			foundCleanEvent = true
			if bwc.PriorRunID != priorRS.RunID {
				t.Errorf("event PriorRunID = %q, want %q", bwc.PriorRunID, priorRS.RunID)
			}
			if bwc.WorktreePath == "" {
				t.Error("event WorktreePath should not be empty")
			}
		}
	}
	if !foundCleanEvent {
		t.Error("expected blocked_worktree_cleaned event to be emitted")
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

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

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

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

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
	createdBranch       string
	worktreePath        string
	removedPath         string
	removedPaths        []string
	createErr           error
	removeErr           error
	recoverBranch       string
	recoverWorktreePath string
	recoverErr          error
}

func (f *fakeGitOps) CreateWorktree(repoDir, branch string) (string, error) {
	f.createdBranch = branch
	return f.worktreePath, f.createErr
}

func (f *fakeGitOps) RemoveWorktree(path string) error {
	f.removedPath = path
	f.removedPaths = append(f.removedPaths, path)
	if f.removeErr != nil {
		return f.removeErr
	}
	// Actually remove the directory so os.Stat checks pass
	return os.RemoveAll(path)
}

func (f *fakeGitOps) RecoverWorktree(repoDir, branch string) (string, error) {
	f.recoverBranch = branch
	f.recoverWorktreePath = f.worktreePath
	return f.worktreePath, f.recoverErr
}

func (f *fakeGitOps) CommitAll(workDir, message string) error {
	return nil
}

type fakeBaselineRunner struct {
	results     validator.CheckResults
	err         error
	runCount    int
	lastWorkDir string
}

func (f *fakeBaselineRunner) RunAlwaysRun(ctx context.Context, checks []validator.Check, workDir string) (validator.CheckResults, error) {
	f.runCount++
	f.lastWorkDir = workDir
	return f.results, f.err
}

// TestInitStage_CleansBlockedWorktrees_EventWrittenToRunDir verifies that
// blocked_worktree_cleaned is emitted even when the eventLog path is inside
// the new run's directory — which does not exist yet when cleanBlockedWorktrees
// runs. The fix requires creating the run dir before calling cleanBlockedWorktrees.
func TestInitStage_CleansBlockedWorktrees_EventWrittenToRunDir(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	priorRS := runstore.NewRunState("test-spec", "test-project")
	priorRS.Status = runstore.StatusBlocked
	priorRS.WorktreePath = filepath.Join(t.TempDir(), "old-worktree")
	os.MkdirAll(priorRS.WorktreePath, 0o755)
	store.Save(priorRS)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	newRS := runstore.NewRunState("test-spec", "test-project")

	// Use the same path pattern exec.go uses: store.RunDir(rs.RunID)/events.jsonl.
	// This directory does NOT exist yet when cleanBlockedWorktrees runs.
	eventLogPath := filepath.Join(store.RunDir(newRS.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    storeDir,
		GitOps:     gitOps,
	}, store, eventLog)

	_, err := stage.Run(context.Background(), newRS)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var found bool
	for _, ev := range events {
		if _, ok := ev.(*runstore.BlockedWorktreeCleanedEvent); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("blocked_worktree_cleaned event not written — cleanBlockedWorktrees must run after run dir is created")
	}
}

// Verify InitStage satisfies the Stage interface.
var _ specloop.Stage = (*InitStage)(nil)

func TestInitStage_CreatesRunDir(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	fixtures := testutil.WriteMinimalProjectFixtures(t, tmp)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

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
	fixtures := testutil.WriteMinimalProjectFixtures(t, tmp)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

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
	fixtures := testutil.WriteMinimalProjectFixtures(t, tmp)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, specContent)
	policyFile := fixtures.PolicyPath

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
	fixtures := testutil.WriteMinimalProjectFixtures(t, tmp)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyContent := `{"always_run":[],"budgets":{"max_spec_cycles":3},"models":{"planner":"high"}}`
	policyFile := fixtures.PolicyPath
	if err := os.WriteFile(policyFile, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("write policy override: %v", err)
	}

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

func TestInitStage_CleansBlockedWorktrees_UsesGitOps(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a prior blocked run
	priorRS := runstore.NewRunState("test-spec", "test-project")
	priorRS.Status = runstore.StatusBlocked
	priorRS.WorktreePath = filepath.Join(t.TempDir(), "old-worktree")
	os.MkdirAll(priorRS.WorktreePath, 0o755)
	store.Save(priorRS)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

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

	// Verify GitOps.RemoveWorktree was called with the blocked worktree path
	found := false
	for _, p := range gitOps.removedPaths {
		if p == priorRS.WorktreePath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GitOps.RemoveWorktree should have been called with %q, got calls: %v",
			priorRS.WorktreePath, gitOps.removedPaths)
	}
}

func TestInit_CapturesBaselineFailures(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	rs := runstore.NewRunState("baseline-spec", "baseline-project")
	rs.BaselineFailures = map[string]string{"unit-tests": "stale baseline fail"}
	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	baselineRunner := &fakeBaselineRunner{
		results: validator.CheckResults{
			Results: []validator.CheckResult{
				{Name: "unit-tests", Pass: false, Output: "already failing"},
			},
		},
	}

	stage := NewInitStage(InitStageConfig{
		SpecPath:       specFile,
		PolicyPath:     policyFile,
		RepoDir:        storeDir,
		GitOps:         gitOps,
		BaselineRunner: baselineRunner,
		AlwaysRun: []validator.Check{
			{Name: "unit-tests", Command: "go test ./..."},
		},
	}, store, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if baselineRunner.runCount != 1 {
		t.Fatalf("baseline runner run count = %d, want 1", baselineRunner.runCount)
	}
	if baselineRunner.lastWorkDir != gitOps.worktreePath {
		t.Fatalf("baseline runner workdir = %q, want %q", baselineRunner.lastWorkDir, gitOps.worktreePath)
	}
	if len(rs.BaselineFailures) != 1 {
		t.Fatalf("BaselineFailures len = %d, want 1", len(rs.BaselineFailures))
	}
	if got := rs.BaselineFailures["unit-tests"]; got != "already failing" {
		t.Fatalf("baseline failure output = %q, want %q", got, "already failing")
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var baselineEvent *runstore.BaselineCapturedEvent
	for _, ev := range events {
		if be, ok := ev.(*runstore.BaselineCapturedEvent); ok {
			baselineEvent = be
			break
		}
	}
	if baselineEvent == nil {
		t.Fatalf("baseline_captured event not emitted")
	}
	if baselineEvent.FailureCount != 1 {
		t.Fatalf("baseline event failure_count = %d, want 1", baselineEvent.FailureCount)
	}
	if len(baselineEvent.CheckNames) != 1 || baselineEvent.CheckNames[0] != "unit-tests" {
		t.Fatalf("baseline event check_names = %v, want [unit-tests]", baselineEvent.CheckNames)
	}
}

func TestInit_BaselineRunnerAbsentNonFatal(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	rs := runstore.NewRunState("baseline-spec", "baseline-project")
	rs.BaselineFailures = map[string]string{"unit-tests": "stale baseline fail"}
	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	stage := NewInitStage(InitStageConfig{
		SpecPath:   specFile,
		PolicyPath: policyFile,
		RepoDir:    storeDir,
		GitOps:     gitOps,
		AlwaysRun: []validator.Check{
			{Name: "unit-tests", Command: "go test ./..."},
		},
	}, store, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if len(rs.BaselineFailures) != 0 {
		t.Fatalf("BaselineFailures len = %d, want 0", len(rs.BaselineFailures))
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	for _, ev := range events {
		if _, ok := ev.(*runstore.BaselineCapturedEvent); ok {
			t.Fatal("baseline_captured event should not be emitted when runner is absent")
		}
	}
}

func TestInit_BaselineRunnerErrorNonFatal(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "new-worktree")}
	os.MkdirAll(gitOps.worktreePath, 0o755)

	rs := runstore.NewRunState("baseline-spec", "baseline-project")
	rs.BaselineFailures = map[string]string{"unit-tests": "stale baseline fail"}
	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	baselineRunner := &fakeBaselineRunner{
		err: errors.New("boom"),
	}

	stage := NewInitStage(InitStageConfig{
		SpecPath:       specFile,
		PolicyPath:     policyFile,
		RepoDir:        storeDir,
		GitOps:         gitOps,
		BaselineRunner: baselineRunner,
		AlwaysRun: []validator.Check{
			{Name: "unit-tests", Command: "go test ./..."},
		},
	}, store, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if len(rs.BaselineFailures) != 0 {
		t.Fatalf("BaselineFailures len = %d, want 0", len(rs.BaselineFailures))
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var captureErrEvent *runstore.BaselineCaptureErrorEvent
	for _, ev := range events {
		switch e := ev.(type) {
		case *runstore.BaselineCaptureErrorEvent:
			captureErrEvent = e
		case *runstore.BaselineCapturedEvent:
			t.Fatalf("baseline_captured event should not be emitted when baseline runner errors")
		}
	}
	if captureErrEvent == nil {
		t.Fatalf("baseline_capture_error event not emitted")
	}
	if captureErrEvent.Error != "boom" {
		t.Fatalf("baseline capture error = %q, want boom", captureErrEvent.Error)
	}
}

func TestInit_BaselineRunnerSucceeds_NoFailures(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

	gitOps := &fakeGitOps{worktreePath: t.TempDir()}

	rs := runstore.NewRunState("baseline-spec", "baseline-project")
	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	baselineRunner := &fakeBaselineRunner{
		results: validator.CheckResults{
			Results: []validator.CheckResult{
				{Name: "unit-tests", Pass: true, Output: "ok"},
			},
		},
	}

	stage := NewInitStage(InitStageConfig{
		SpecPath:       specFile,
		PolicyPath:     policyFile,
		RepoDir:        storeDir,
		GitOps:         gitOps,
		BaselineRunner: baselineRunner,
		AlwaysRun: []validator.Check{
			{Name: "unit-tests", Command: "go test ./..."},
		},
	}, store, eventLog)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if len(rs.BaselineFailures) != 0 {
		t.Fatalf("BaselineFailures len = %d, want 0 (empty map)", len(rs.BaselineFailures))
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	var baselineEvent *runstore.BaselineCapturedEvent
	for _, ev := range events {
		if be, ok := ev.(*runstore.BaselineCapturedEvent); ok {
			baselineEvent = be
			break
		}
	}
	if baselineEvent == nil {
		t.Fatalf("baseline_captured event not emitted")
	}
	if baselineEvent.FailureCount != 0 {
		t.Fatalf("baseline event failure_count = %d, want 0", baselineEvent.FailureCount)
	}
	if len(baselineEvent.CheckNames) != 0 {
		t.Fatalf("baseline event check_names = %v, want empty", baselineEvent.CheckNames)
	}
}

func TestInit_BaselineFailuresPersisted(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	fixtures := testutil.WriteMinimalProjectFixtures(t, storeDir)
	specFile := writeSpec(t, fixtures.Config.SpecsDir, "# Test Spec")
	policyFile := fixtures.PolicyPath

	gitOps := &fakeGitOps{worktreePath: t.TempDir()}

	rs := runstore.NewRunState("baseline-spec", "baseline-project")
	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	baselineRunner := &fakeBaselineRunner{
		results: validator.CheckResults{
			Results: []validator.CheckResult{
				{Name: "unit-tests", Pass: false, Output: "pre-existing failure"},
			},
		},
	}

	stage := NewInitStage(InitStageConfig{
		SpecPath:       specFile,
		PolicyPath:     policyFile,
		RepoDir:        storeDir,
		GitOps:         gitOps,
		BaselineRunner: baselineRunner,
		AlwaysRun: []validator.Check{
			{Name: "unit-tests", Command: "go test ./..."},
		},
	}, store, eventLog)

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if err := store.Save(rs); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if len(loaded.BaselineFailures) != len(rs.BaselineFailures) {
		t.Fatalf("loaded BaselineFailures len = %d, want %d", len(loaded.BaselineFailures), len(rs.BaselineFailures))
	}
	for k, v := range rs.BaselineFailures {
		if loaded.BaselineFailures[k] != v {
			t.Fatalf("loaded BaselineFailures[%q] = %q, want %q", k, loaded.BaselineFailures[k], v)
		}
	}
}

func writeSpec(t testing.TB, specsDir, content string) string {
	t.Helper()
	if specsDir == "" {
		t.Fatal("specsDir must be set")
	}
	if content == "" {
		content = "# Test Spec"
	}
	specPath := filepath.Join(specsDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec %q: %v", specPath, err)
	}
	return specPath
}
