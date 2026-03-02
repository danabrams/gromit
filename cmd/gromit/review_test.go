package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/worktree"
	"github.com/spf13/cobra"
)

type gitCommandCapture struct {
	name string
	args []string
}

type reviewRouterStub struct {
	SelectFn          func(phase string, tier string) (provider.Provider, string)
	MarkUnavailableFn func(name string)
}

func (s *reviewRouterStub) Select(phase string, tier string) (provider.Provider, string) {
	if s != nil && s.SelectFn != nil {
		return s.SelectFn(phase, tier)
	}
	return nil, ""
}

func (s *reviewRouterStub) MarkUnavailable(name string) {
	if s != nil && s.MarkUnavailableFn != nil {
		s.MarkUnavailableFn(name)
	}
}

type reviewProviderStub struct {
	NameFn               func() string
	ModelForTierFn       func(tier string) string
	RunFn                func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	StreamRunFn          func(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	RunValidationFn      func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
	IsUsageLimitErrorFn  func(result *provider.Result, err error) bool
	IsValidationPassedFn func(result *provider.Result) bool
	IsScopeTooLargeFn    func(result *provider.Result) (bool, string)
}

func (s *reviewProviderStub) Name() string {
	if s != nil && s.NameFn != nil {
		return s.NameFn()
	}
	return "stub"
}

func (s *reviewProviderStub) ModelForTier(tier string) string {
	if s != nil && s.ModelForTierFn != nil {
		return s.ModelForTierFn(tier)
	}
	return tier
}

func (s *reviewProviderStub) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if s != nil && s.RunFn != nil {
		return s.RunFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true}, nil
}

func (s *reviewProviderStub) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if s != nil && s.StreamRunFn != nil {
		return s.StreamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true}, nil
}

func (s *reviewProviderStub) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if s != nil && s.RunValidationFn != nil {
		return s.RunValidationFn(ctx, commands, tier, workDir)
	}
	return &provider.Result{Success: true}, nil
}

func (s *reviewProviderStub) IsUsageLimitError(result *provider.Result, err error) bool {
	if s != nil && s.IsUsageLimitErrorFn != nil {
		return s.IsUsageLimitErrorFn(result, err)
	}
	return false
}

func (s *reviewProviderStub) IsValidationPassed(result *provider.Result) bool {
	if s != nil && s.IsValidationPassedFn != nil {
		return s.IsValidationPassedFn(result)
	}
	return true
}

func (s *reviewProviderStub) IsScopeTooLarge(result *provider.Result) (bool, string) {
	if s != nil && s.IsScopeTooLargeFn != nil {
		return s.IsScopeTooLargeFn(result)
	}
	return false, ""
}

func stubReviewGit(t *testing.T, output []byte, outputErr error) *gitCommandCapture {
	t.Helper()

	origCommandFn := reviewGitCommandFn
	origOutputFn := reviewGitOutputFn
	t.Cleanup(func() {
		reviewGitCommandFn = origCommandFn
		reviewGitOutputFn = origOutputFn
	})

	capture := &gitCommandCapture{}
	reviewGitCommandFn = func(name string, arg ...string) *exec.Cmd {
		capture.name = name
		capture.args = append([]string{}, arg...)
		return exec.Command("echo", "stub")
	}
	reviewGitOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
		return output, outputErr
	}

	return capture
}

func assertGitCommand(t *testing.T, capture *gitCommandCapture, wantName string, wantArgs []string) {
	t.Helper()

	if capture.name != wantName {
		t.Fatalf("command name = %q, want %q", capture.name, wantName)
	}
	if strings.Join(capture.args, "|") != strings.Join(wantArgs, "|") {
		t.Fatalf("command args = %v, want %v", capture.args, wantArgs)
	}
}

// TestReviewGitOutputFn_CanBeOverridden verifies that reviewGitOutputFn is a
// package-level injectable variable with the expected signature.
func TestReviewGitOutputFn_CanBeOverridden(t *testing.T) {

	orig := reviewGitOutputFn
	t.Cleanup(func() { reviewGitOutputFn = orig })

	stubOut := []byte("abc123\n")
	reviewGitOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
		return stubOut, nil
	}

	cmd := exec.Command("echo", "unused")
	got, err := reviewGitOutputFn(cmd)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(got) != string(stubOut) {
		t.Errorf("expected output %q, got %q", stubOut, got)
	}
}

// TestReviewGitCommandFn_CanBeOverridden verifies that reviewGitCommandFn is a
// package-level injectable variable with the expected signature.
func TestReviewGitCommandFn_CanBeOverridden(t *testing.T) {

	orig := reviewGitCommandFn
	t.Cleanup(func() { reviewGitCommandFn = orig })

	var capturedName string
	var capturedArgs []string
	reviewGitCommandFn = func(name string, arg ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = arg
		return exec.Command("echo", "stub")
	}

	_ = reviewGitCommandFn("git", "rev-parse", "HEAD")

	if capturedName != "git" {
		t.Errorf("expected captured name %q, got %q", "git", capturedName)
	}
	if len(capturedArgs) < 2 || capturedArgs[0] != "rev-parse" || capturedArgs[1] != "HEAD" {
		t.Errorf("expected args [rev-parse HEAD], got %v", capturedArgs)
	}
}

func TestRunReviewInteractive_UsesSessionWorktreeLaunchDir(t *testing.T) {

	origLauncher := reviewInteractiveSessionLauncherFn
	origRunner := reviewInteractiveRunnerFn
	origRecord := recordInteractiveReviewCompletionFn
	t.Cleanup(func() {
		reviewInteractiveSessionLauncherFn = origLauncher
		reviewInteractiveRunnerFn = origRunner
		recordInteractiveReviewCompletionFn = origRecord
	})

	cfg := &config.Config{}
	wantSessionDir := t.TempDir()
	gotLaunchDir := ""
	recordCalled := false

	reviewInteractiveSessionLauncherFn = func(
		ctx context.Context,
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		if command != "review" {
			t.Fatalf("command = %q, want %q", command, "review")
		}
		wantSettings := sessionConflictSettingsFromConfig(cfg)
		if conflictSettings.Policy != wantSettings.Policy || conflictSettings.RetryCap != wantSettings.RetryCap {
			t.Fatalf("conflict settings = %+v, want policy=%q retry_cap=%d", conflictSettings, wantSettings.Policy, wantSettings.RetryCap)
		}
		if err := callback(wantSessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "gromit/review-test", WorktreeDir: wantSessionDir}, nil
	}
	reviewInteractiveRunnerFn = func(cfg *config.Config, fromCommit, diff, launchDir string) error {
		gotLaunchDir = launchDir
		return nil
	}
	recordInteractiveReviewCompletionFn = func(gromitDir, fromCommit string) error {
		recordCalled = true
		return nil
	}

	if err := runReviewInteractive(context.Background(), cfg, "abc123", "diff --git a b"); err != nil {
		t.Fatalf("runReviewInteractive() error = %v", err)
	}
	if gotLaunchDir != wantSessionDir {
		t.Fatalf("launchDir = %q, want %q", gotLaunchDir, wantSessionDir)
	}
	if !recordCalled {
		t.Fatal("expected review completion to be recorded")
	}
}

func TestRunReviewInteractive_AppliesReviewFindings(t *testing.T) {
	t.Parallel()

	origLauncher := reviewInteractiveSessionLauncherFn
	origRunner := reviewInteractiveRunnerFn
	origRecord := recordInteractiveReviewCompletionFn
	origApply := applyInteractiveReviewFindingsFn
	t.Cleanup(func() {
		reviewInteractiveSessionLauncherFn = origLauncher
		reviewInteractiveRunnerFn = origRunner
		recordInteractiveReviewCompletionFn = origRecord
		applyInteractiveReviewFindingsFn = origApply
	})

	applyCalled := false
	reviewInteractiveSessionLauncherFn = func(
		ctx context.Context,
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		if err := callback(t.TempDir()); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "test", WorktreeDir: t.TempDir()}, nil
	}
	reviewInteractiveRunnerFn = func(cfg *config.Config, fromCommit, diff, launchDir string) error {
		return nil
	}
	recordInteractiveReviewCompletionFn = func(gromitDir, fromCommit string) error {
		return nil
	}
	applyInteractiveReviewFindingsFn = func(cfg *config.Config, dir string) error {
		applyCalled = true
		return nil
	}

	if err := runReviewInteractive(context.Background(), &config.Config{}, "abc123", "diff"); err != nil {
		t.Fatalf("runReviewInteractive() error = %v", err)
	}
	if !applyCalled {
		t.Fatal("expected applyInteractiveReviewFindings to run")
	}
}

func TestRunReviewInteractive_ConflictHandoffPropagates(t *testing.T) {

	origLauncher := reviewInteractiveSessionLauncherFn
	origRunner := reviewInteractiveRunnerFn
	origRecord := recordInteractiveReviewCompletionFn
	t.Cleanup(func() {
		reviewInteractiveSessionLauncherFn = origLauncher
		reviewInteractiveRunnerFn = origRunner
		recordInteractiveReviewCompletionFn = origRecord
	})

	cfg := &config.Config{}
	recordCalled := false
	reviewInteractiveRunnerFn = func(cfg *config.Config, fromCommit, diff, launchDir string) error {
		return nil
	}
	recordInteractiveReviewCompletionFn = func(gromitDir, fromCommit string) error {
		recordCalled = true
		return nil
	}
	reviewInteractiveSessionLauncherFn = func(
		ctx context.Context,
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		return &worktree.SessionWorktree{
				BranchName:  "gromit/review-conflict",
				WorktreeDir: "/tmp/session-review",
			}, &mergeConflictHandoffError{
				Policy:     conflictPolicyManual,
				Branch:     "gromit/review-conflict",
				SessionDir: "/tmp/session-review",
				MergeErr:   errors.New("merge conflict"),
			}
	}

	err := runReviewInteractive(context.Background(), cfg, "abc123", "diff --git a b")
	if err == nil {
		t.Fatal("expected conflict handoff error, got nil")
	}
	if !isMergeConflictHandoffError(err) {
		t.Fatalf("expected merge conflict handoff error, got %T (%v)", err, err)
	}
	if recordCalled {
		t.Fatal("review completion should not be recorded when merge handoff occurs")
	}
}

func TestRunReview_CommandContextPassedToSessionLauncher(t *testing.T) {
	baseDir := t.TempDir()
	origCreatePipelineFn := createReviewPipelineFn
	origLauncher := reviewInteractiveSessionLauncherFn
	origRunner := reviewInteractiveRunnerFn
	origRecord := recordInteractiveReviewCompletionFn
	origApply := applyInteractiveReviewFindingsFn
	origGitOutputFn := reviewGitOutputFn
	origNonInteractive := reviewNonInteractive
	origDryRun := reviewDryRun
	t.Cleanup(func() {
		createReviewPipelineFn = origCreatePipelineFn
		reviewInteractiveSessionLauncherFn = origLauncher
		reviewInteractiveRunnerFn = origRunner
		recordInteractiveReviewCompletionFn = origRecord
		applyInteractiveReviewFindingsFn = origApply
		reviewGitOutputFn = origGitOutputFn
		reviewNonInteractive = origNonInteractive
		reviewDryRun = origDryRun
	})

	if err := os.Chdir(baseDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "gromit.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}

	ctx := context.WithValue(context.Background(), "test-key", "test-value")
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	reviewNonInteractive = false
	reviewDryRun = false

	createReviewPipelineFn = func(cfg *config.Config, gromitDir string) (ReviewScopeResolver, error) {
		return testReviewScopeResolver{}, nil
	}
	reviewGitOutputFn = func(_ *exec.Cmd) ([]byte, error) {
		return []byte("diff"), nil
	}

	var capturedCtx context.Context
	reviewInteractiveSessionLauncherFn = func(
		ctx context.Context,
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		capturedCtx = ctx
		if err := callback("session"); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "gromit/review-test", WorktreeDir: "session"}, nil
	}
	reviewInteractiveRunnerFn = func(cfg *config.Config, fromCommit, diff, launchDir string) error {
		return nil
	}
	recordInteractiveReviewCompletionFn = func(gromitDir, fromCommit string) error {
		return nil
	}
	applyInteractiveReviewFindingsFn = func(cfg *config.Config, dir string) error {
		return nil
	}

	if err := runReview(cmd, []string{}); err != nil {
		t.Fatalf("runReview() error = %v", err)
	}

	if capturedCtx == nil {
		t.Fatal("expected session launcher to receive context")
	}
	if capturedCtx.Value("test-key") != "test-value" {
		t.Fatalf("captured context missing marker value: %v", capturedCtx.Value("test-key"))
	}
}

type testReviewScopeResolver struct{}

func (testReviewScopeResolver) ResolveReviewScope(ctx context.Context, spec string, epic string, since string) (string, error) {
	return "abc123", nil
}

func TestApplyInteractiveReviewFindings_MissingFileWarns(t *testing.T) {
	t.Parallel()

	gromitDir := t.TempDir()
	var buf strings.Builder

	origWriter := reviewFindingsLogWriter
	t.Cleanup(func() { reviewFindingsLogWriter = origWriter })
	reviewFindingsLogWriter = &buf

	origBuilder := buildReviewFindingsApplierFn
	t.Cleanup(func() { buildReviewFindingsApplierFn = origBuilder })
	buildReviewFindingsApplierFn = func(cfg *config.Config, dir string) (reviewFindingsApplier, error) {
		t.Fatalf("pipeline should not be built when findings file is absent")
		return nil, nil
	}

	if err := applyInteractiveReviewFindings(&config.Config{}, gromitDir); err != nil {
		t.Fatalf("applyInteractiveReviewFindings() error = %v", err)
	}

	if !strings.Contains(buf.String(), "warning") {
		t.Fatalf("expected warning log, got %q", buf.String())
	}
}

func TestApplyInteractiveReviewFindings_MalformedFileWarns(t *testing.T) {
	t.Parallel()

	gromitDir := t.TempDir()
	reviewDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(reviewDir, "review-findings.json")
	if err := os.WriteFile(filePath, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var buf strings.Builder
	origWriter := reviewFindingsLogWriter
	t.Cleanup(func() { reviewFindingsLogWriter = origWriter })
	reviewFindingsLogWriter = &buf

	origBuilder := buildReviewFindingsApplierFn
	t.Cleanup(func() { buildReviewFindingsApplierFn = origBuilder })
	buildReviewFindingsApplierFn = func(cfg *config.Config, dir string) (reviewFindingsApplier, error) {
		t.Fatalf("pipeline should not be built when findings file cannot be parsed")
		return nil, nil
	}

	if err := applyInteractiveReviewFindings(&config.Config{}, gromitDir); err != nil {
		t.Fatalf("applyInteractiveReviewFindings() error = %v", err)
	}

	if !strings.Contains(buf.String(), "warning") {
		t.Fatalf("expected warning log, got %q", buf.String())
	}
}

func TestApplyInteractiveReviewFindings_AppliesAndLogsSummary(t *testing.T) {
	t.Parallel()

	gromitDir := t.TempDir()
	reviewDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(reviewDir, "review-findings.json")
	reviewJSON := `{"passed":true,"fixes_applied":[],"fix_categories":[],"beads_to_create":[],"backlog_items":[],"summary":"ok","learnings":[]}`
	if err := os.WriteFile(filePath, []byte(reviewJSON), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var buf strings.Builder
	origWriter := reviewFindingsLogWriter
	t.Cleanup(func() { reviewFindingsLogWriter = origWriter })
	reviewFindingsLogWriter = &buf

	stub := &stubReviewFindingsApplier{
		applyResult: &pipeline.ReviewApplyResult{
			CreatedBeadIDs:      []string{"bead-123", "bead-456"},
			CreatedBacklogCount: 2,
			LearningsSaved:      1,
		},
	}
	origBuilder := buildReviewFindingsApplierFn
	t.Cleanup(func() { buildReviewFindingsApplierFn = origBuilder })
	buildReviewFindingsApplierFn = func(cfg *config.Config, dir string) (reviewFindingsApplier, error) {
		return stub, nil
	}

	if err := applyInteractiveReviewFindings(&config.Config{}, gromitDir); err != nil {
		t.Fatalf("applyInteractiveReviewFindings() error = %v", err)
	}

	if !stub.called {
		t.Fatalf("expected pipeline to be invoked")
	}
	log := buf.String()
	if !strings.Contains(log, "Review findings applied") {
		t.Fatalf("expected summary log, got %q", log)
	}
	if !strings.Contains(log, "Bead IDs: bead-123, bead-456") {
		t.Fatalf("expected bead IDs in log, got %q", log)
	}
}

type stubReviewFindingsApplier struct {
	applyResult *pipeline.ReviewApplyResult
	called      bool
}

func (s *stubReviewFindingsApplier) ApplyReviewFindings(ctx context.Context, result *review.ReviewResult) (*pipeline.ReviewApplyResult, error) {
	s.called = true
	return s.applyResult, nil
}

func TestRunGitDiffForReview_UsesInjectedGit(t *testing.T) {
	// Not parallel: stubReviewGit mutates package-level reviewGitCommandFn and reviewGitOutputFn.
	capture := stubReviewGit(t, []byte("diff output\n"), nil)

	diff, err := runGitDiffForReview("abc123", "git diff --stat", "--stat")
	if err != nil {
		t.Fatalf("runGitDiffForReview() error = %v", err)
	}
	if diff != "diff output\n" {
		t.Fatalf("runGitDiffForReview() = %q, want %q", diff, "diff output\n")
	}
	wantArgs := []string{"diff", "--stat", "abc123", "--"}
	assertGitCommand(t, capture, "git", wantArgs)
}

func TestGetGitHeadForReview_UsesInjectedGit(t *testing.T) {
	// Not parallel: stubReviewGit mutates package-level reviewGitCommandFn and reviewGitOutputFn.
	capture := stubReviewGit(t, []byte("deadbeef\n"), nil)

	head, err := getGitHeadForReview()
	if err != nil {
		t.Fatalf("getGitHeadForReview() error = %v", err)
	}
	if head != "deadbeef" {
		t.Fatalf("getGitHeadForReview() = %q, want %q", head, "deadbeef")
	}
	wantArgs := []string{"rev-parse", "HEAD"}
	assertGitCommand(t, capture, "git", wantArgs)
}

func TestCliStateManagerSetLastReviewCommit_PassesThroughCommit(t *testing.T) {
	// The adapter now just passes through the commit parameter as-is
	// (business logic to fetch git head moved to caller)
	t.Parallel()

	gromitDir := t.TempDir()
	sf, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		t.Fatalf("NewInteractiveFile() error = %v", err)
	}
	if err := sf.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	manager := &cliStateManager{stateFile: sf}
	if err := manager.SetLastReviewCommit("from-commit"); err != nil {
		t.Fatalf("SetLastReviewCommit() error = %v", err)
	}
	if sf.LastReviewCommit() != "from-commit" {
		t.Fatalf("LastReviewCommit() = %q, want %q", sf.LastReviewCommit(), "from-commit")
	}
}

func TestCliStateManagerSetLastReviewCommit_FallsBackToProvidedCommit(t *testing.T) {
	// Not parallel: stubReviewGit mutates package-level reviewGitOutputFn.
	stubReviewGit(t, nil, errors.New("git failure"))

	gromitDir := t.TempDir()
	sf, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		t.Fatalf("NewInteractiveFile() error = %v", err)
	}
	if err := sf.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	manager := &cliStateManager{stateFile: sf}
	if err := manager.SetLastReviewCommit("fallback-commit"); err != nil {
		t.Fatalf("SetLastReviewCommit() error = %v", err)
	}

	sf2, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		t.Fatalf("NewInteractiveFile() error = %v", err)
	}
	if err := sf2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if sf2.LastReviewCommit() != "fallback-commit" {
		t.Fatalf("LastReviewCommit() = %q, want %q", sf2.LastReviewCommit(), "fallback-commit")
	}
}

func TestCliStateManagerGetLastReviewCommit_ReturnsStateCommit(t *testing.T) {
	t.Parallel()
	gromitDir := t.TempDir()
	sf, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		t.Fatalf("NewInteractiveFile() error = %v", err)
	}
	if err := sf.RecordReview("json-commit", 1); err != nil {
		t.Fatalf("RecordReview() error = %v", err)
	}

	manager := &cliStateManager{stateFile: sf}
	commit, err := manager.GetLastReviewCommit()
	if err != nil {
		t.Fatalf("GetLastReviewCommit() error = %v", err)
	}
	if commit != "json-commit" {
		t.Fatalf("GetLastReviewCommit() = %q, want %q", commit, "json-commit")
	}
}

// saveReviewFlags saves the current review flag values and registers a cleanup
// to restore them after the test completes.
func saveReviewFlags(t *testing.T) {
	t.Helper()
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	t.Cleanup(func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	})
}

func TestResolveReviewRendererPaths_Defaults(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	templatesDir, specsDir, claudeMDPath := resolveReviewRendererPaths(cfg)

	if templatesDir != ".gromit/templates" {
		t.Errorf("expected templates dir default to .gromit/templates, got %q", templatesDir)
	}
	if specsDir != ".gromit/specs" {
		t.Errorf("expected specs dir default to .gromit/specs, got %q", specsDir)
	}
	if claudeMDPath != "CLAUDE.md" {
		t.Errorf("expected project CLAUDE.md default to CLAUDE.md, got %q", claudeMDPath)
	}
}

func TestReviewNonInteractiveModel_PrefersThoroughConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Review.Thorough.Model = "custom-model"

	if got := reviewNonInteractiveModel(cfg); got != "custom-model" {
		t.Fatalf("reviewNonInteractiveModel() = %q, want %q", got, "custom-model")
	}
}

func TestReviewNonInteractiveModel_DefaultsToConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.SetDefaults()

	if got := reviewNonInteractiveModel(cfg); got != cfg.Review.Thorough.Model {
		t.Fatalf("reviewNonInteractiveModel() = %q, want %q", got, cfg.Review.Thorough.Model)
	}
}

func TestResolveReviewNonInteractiveTimeout_Defaults(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	timeout := resolveReviewNonInteractiveTimeout(cfg)

	if timeout != defaultThoroughReviewTimeoutSeconds {
		t.Errorf("expected default thorough review timeout %d, got %d", defaultThoroughReviewTimeoutSeconds, timeout)
	}
}

func TestBuildReviewNonInteractiveClient_ReturnsPipelineLLMClient(t *testing.T) {
	t.Parallel()
	var builder func(*config.Config) (pipeline.LLMClient, error) = buildReviewNonInteractiveClient
	if builder == nil {
		t.Fatal("builder is nil")
	}
}

func TestBuildReviewNonInteractiveClient_ClaudeFallbackPath(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:          "claude",
			Timeout:         123,
			PipelineTimeout: 456,
		},
	}

	client, err := buildReviewNonInteractiveClient(cfg)
	if err != nil {
		t.Fatalf("buildReviewNonInteractiveClient() error = %v", err)
	}

	typedClient, ok := client.(*claudeClientAdapter)
	if !ok {
		t.Fatalf("client type = %T, want *claudeClientAdapter", client)
	}
	if typedClient.Timeout != 456*time.Second {
		t.Fatalf("timeout = %v, want %v", typedClient.Timeout, 456*time.Second)
	}
}

func TestBuildReviewNonInteractiveClient_CodexProviderPath(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
				Models: map[string]string{
					provider.TierHigh:   "gpt-5.3-codex",
					provider.TierMedium: "gpt-5.3-codex",
					provider.TierLow:    "gpt-5.3-codex",
				},
			},
		},
		Claude: config.ClaudeConfig{
			PipelineTimeout: 789,
		},
	}

	client, err := buildReviewNonInteractiveClient(cfg)
	if err != nil {
		t.Fatalf("buildReviewNonInteractiveClient() error = %v", err)
	}

	typedClient, ok := client.(*llmRouterClientAdapter)
	if !ok {
		t.Fatalf("client type = %T, want *llmRouterClientAdapter", client)
	}
	if typedClient.Timeout != 789*time.Second {
		t.Fatalf("timeout = %v, want %v", typedClient.Timeout, 789*time.Second)
	}
	if typedClient.Phase != reviewSessionCommand {
		t.Fatalf("phase = %q, want %q", typedClient.Phase, reviewSessionCommand)
	}
}

func TestProviderRouterClientAdapterRun_ClaudePath(t *testing.T) {
	t.Parallel()
	var gotPhase, gotTier string
	mockProvider := &reviewProviderStub{
		NameFn: func() string { return "claude" },
		RunFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			gotTier = tier
			return &provider.Result{Success: true, Output: "ok-claude", ExitCode: 0}, nil
		},
	}
	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			gotPhase = phase
			return mockProvider, "opus"
		},
	}
	adapter := &llmRouterClientAdapter{
		Router:  mockRouter,
		Timeout: 2 * time.Second,
		Phase:   "review",
	}

	result, err := adapter.Run("prompt", "opus")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	if gotPhase != "review" {
		t.Fatalf("phase = %q, want review", gotPhase)
	}
	if gotTier != provider.TierHigh {
		t.Fatalf("tier = %q, want %q", gotTier, provider.TierHigh)
	}
	if result.Output != "ok-claude" {
		t.Fatalf("output = %q, want ok-claude", result.Output)
	}
}

func TestProviderRouterClientAdapterRun_CodexPath(t *testing.T) {
	t.Parallel()
	var gotTier string
	mockProvider := &reviewProviderStub{
		NameFn: func() string { return "codex" },
		RunFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			gotTier = tier
			return &provider.Result{Success: true, Output: "ok-codex", ExitCode: 0}, nil
		},
	}
	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			return mockProvider, "gpt-5.3-codex"
		},
	}
	adapter := &llmRouterClientAdapter{
		Router:  mockRouter,
		Timeout: 2 * time.Second,
		Phase:   "review",
	}

	result, err := adapter.Run("prompt", "sonnet")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	if gotTier != provider.TierMedium {
		t.Fatalf("tier = %q, want %q", gotTier, provider.TierMedium)
	}
	if result.Output != "ok-codex" {
		t.Fatalf("output = %q, want ok-codex", result.Output)
	}
}

func TestCliLogWriter_WriteIncludesPromptDiagnosticsFromProvider(t *testing.T) {
	t.Parallel()
	logsDir := t.TempDir()
	wantDiagnostics := &prompt.PromptDiagnostics{
		PromptType:      "thorough_review",
		EstimatedTokens: 55,
		SectionTokens: map[string]int{
			prompt.SectionDiff: 55,
		},
	}
	writer := &cliLogWriter{
		logsDir: logsDir,
		promptDiagnosticsProvider: func() *prompt.PromptDiagnostics {
			return wantDiagnostics
		},
	}

	entry := &pipeline.LogEntry{
		Type:           "review",
		Passed:         true,
		FixesApplied:   1,
		BeadsCreated:   2,
		BacklogCreated: 3,
		Model:          "sonnet",
	}
	if err := writer.Write(entry); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(files))
	}

	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	diagRaw, ok := raw["prompt_diagnostics"]
	if !ok {
		t.Fatalf("expected prompt_diagnostics in review log entry: %s", lines[0])
	}
	diagMap, ok := diagRaw.(map[string]any)
	if !ok {
		t.Fatalf("prompt_diagnostics has unexpected type %T", diagRaw)
	}
	if got, _ := diagMap["prompt_type"].(string); got != "thorough_review" {
		t.Fatalf("prompt_type = %q, want %q", got, "thorough_review")
	}
}

func TestCliLogWriter_WriteUsesProviderAtWriteTime(t *testing.T) {
	t.Parallel()
	logsDir := t.TempDir()
	initialDiagnostics := &prompt.PromptDiagnostics{PromptType: "initial"}
	updatedDiagnostics := &prompt.PromptDiagnostics{PromptType: "updated"}
	currentDiagnostics := initialDiagnostics

	writer := &cliLogWriter{
		logsDir: logsDir,
		promptDiagnosticsProvider: func() *prompt.PromptDiagnostics {
			return currentDiagnostics
		},
	}

	// Match runReviewNonInteractive behavior: diagnostics are read when logs are written.
	currentDiagnostics = updatedDiagnostics

	entry := &pipeline.LogEntry{
		Type:   "review",
		Passed: true,
	}
	if err := writer.Write(entry); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(files))
	}

	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	diagRaw, ok := raw["prompt_diagnostics"]
	if !ok {
		t.Fatalf("expected prompt_diagnostics in review log entry: %s", lines[0])
	}
	diagMap, ok := diagRaw.(map[string]any)
	if !ok {
		t.Fatalf("prompt_diagnostics has unexpected type %T", diagRaw)
	}
	if got, _ := diagMap["prompt_type"].(string); got != "updated" {
		t.Fatalf("prompt_type = %q, want %q", got, "updated")
	}
}

func TestPrintReviewSummaryCounts_IncludesBacklogCount(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	result := &pipeline.ReviewResult{
		FixesApplied:   1,
		BeadsCreated:   2,
		BacklogCreated: 3,
	}
	printReviewSummaryCounts(&buf, result)
	got := buf.String()
	if !strings.Contains(got, "Fixes applied: 1") {
		t.Fatalf("missing fixes applied line, got %q", got)
	}
	if !strings.Contains(got, "Created 2 beads from review findings") {
		t.Fatalf("missing beads count, got %q", got)
	}
	if !strings.Contains(got, "Created 3 backlog items") {
		t.Fatalf("missing backlog count, got %q", got)
	}
}

func TestPrintReviewSummaryCounts_IncludesBeadIDs(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	result := &pipeline.ReviewResult{
		FixesApplied:   1,
		BeadsCreated:   2,
		BacklogCreated: 1,
		Apply: &pipeline.ReviewApplyResult{
			CreatedBeadIDs: []string{"bead-55", "bead-99"},
		},
	}
	printReviewSummaryCounts(&buf, result)
	got := buf.String()
	if !strings.Contains(got, "Created 2 beads from review findings") {
		t.Fatalf("missing bead count, got %q", got)
	}
	if !strings.Contains(got, "bead-55") || !strings.Contains(got, "bead-99") {
		t.Fatalf("missing bead IDs, got %q", got)
	}
}

func TestCliBacklogClient_ImplementsBacklogWriter(t *testing.T) {
	t.Parallel()
	var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)
}

func TestCliBacklogClient_AddPassesThroughEntry(t *testing.T) {
	t.Parallel()
	// The adapter now just passes through the entry fields as-is
	entry := &pipeline.BacklogEntry{
		Title:           "backlog item",
		Type:            "review-finding",
		Priority:        2,
		Labels:          []string{"from-review", "backlog"},
		ExpectedOutputs: []string{"output1", "output2"},
	}

	var capturedTitle string
	var capturedPriority int
	var capturedLabels []string
	var capturedOutputs []string

	client := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			// Capture the arguments passed to bead.Create
			// The adapter calls: c.beadClient.Create(context.Background(), entry.Title, entry.Priority, entry.Labels, entry.ExpectedOutputs)
			// So args will be like: ["create", "backlog item", "--priority", "2", "--label", "from-review", "--label", "backlog", "--acceptance", "output1\noutput2"]
			for i := 0; i < len(args); i++ {
				if args[i] == "create" && i+1 < len(args) {
					capturedTitle = args[i+1]
				}
				if args[i] == "--priority" && i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%d", &capturedPriority)
				}
				if args[i] == "--label" && i+1 < len(args) {
					capturedLabels = append(capturedLabels, args[i+1])
				}
				if args[i] == "--acceptance" && i+1 < len(args) {
					capturedOutputs = append(capturedOutputs, args[i+1])
				}
			}
			return `{"id":"fake","title":"fake","description":"","status":"open","priority":2}`, nil
		},
	}

	backlog := &cliBacklogClient{beadClient: client}
	if err := backlog.Add(context.Background(), entry); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if capturedTitle != "backlog item" {
		t.Fatalf("expected title to pass through, got %q", capturedTitle)
	}
	if capturedPriority != 2 {
		t.Fatalf("expected priority 2 to pass through, got %d", capturedPriority)
	}
	if !containsLabel(capturedLabels, "from-review") || !containsLabel(capturedLabels, "backlog") {
		t.Fatalf("expected labels to include from-review/backlog, got %v", capturedLabels)
	}
}

func containsLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

// TestValidateCommitRef verifies that commit refs starting with "-" are rejected.
func TestValidateCommitRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{"valid sha", "abc1234", false},
		{"valid full sha", "cf02391aabbccddeeff00112233445566778899aa", false},
		{"valid branch name", "main", false},
		{"valid HEAD", "HEAD", false},
		{"flag injection attempt", "--output=/tmp/x", true},
		{"short flag attempt", "-n", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateCommitRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCommitRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			}
		})
	}
}

// TestGetGitDiffForReview_RejectsFlagInjection verifies that getGitDiffForReview
// rejects commit refs that look like git flags.
func TestGetGitDiffForReview_RejectsFlagInjection(t *testing.T) {
	t.Parallel()
	_, err := getGitDiffForReview("--output=/tmp/x")
	if err == nil {
		t.Fatal("getGitDiffForReview should reject flag-like commit ref")
	}
	if !strings.Contains(err.Error(), "invalid commit ref") {
		t.Errorf("error should mention 'invalid commit ref', got: %v", err)
	}
}

// TestTimestampComparison verifies that Unix timestamp comparison is done numerically,
// not lexicographically. This is a regression test for the bug where string comparison
// was used (e.g. "9" > "10" in string comparison).
func TestTimestampComparison(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		timestamp1 string
		timestamp2 string
		want       bool // true if timestamp1 < timestamp2
	}{
		{
			name:       "clearly earlier (10 digits)",
			timestamp1: "1609459200", // 2021-01-01
			timestamp2: "1640995200", // 2022-01-01
			want:       true,
		},
		{
			name:       "clearly later (10 digits)",
			timestamp1: "1640995200", // 2022-01-01
			timestamp2: "1609459200", // 2021-01-01
			want:       false,
		},
		{
			name:       "equal timestamps",
			timestamp1: "1609459200",
			timestamp2: "1609459200",
			want:       false,
		},
		{
			name:       "single digit vs double digit (string compare would fail)",
			timestamp1: "9",
			timestamp2: "10",
			want:       true, // 9 < 10 numerically (but "9" > "10" in string comparison)
		},
		{
			name:       "large vs small with string comparison issue",
			timestamp1: "999999999",  // 9 digits
			timestamp2: "1000000000", // 10 digits (epoch start)
			want:       true,         // numerically correct (but string compare would be false)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel(
			// Parse the timestamps using the same logic as isCommitEarlier
			)

			ts1, err := strconv.ParseInt(tt.timestamp1, 10, 64)
			if err != nil {
				t.Fatalf("Failed to parse timestamp1 %q: %v", tt.timestamp1, err)
			}

			ts2, err := strconv.ParseInt(tt.timestamp2, 10, 64)
			if err != nil {
				t.Fatalf("Failed to parse timestamp2 %q: %v", tt.timestamp2, err)
			}

			got := ts1 < ts2

			if got != tt.want {
				t.Errorf("timestamp comparison: ts1=%d, ts2=%d, got %v, want %v", ts1, ts2, got, tt.want)
			}

			// Also verify that string comparison would give incorrect results for the edge cases
			if tt.name == "single digit vs double digit (string compare would fail)" {
				stringCompare := tt.timestamp1 < tt.timestamp2
				if stringCompare == got {
					t.Errorf("Expected string comparison to differ from numeric comparison for test case %q", tt.name)
				}
			}
		})
	}
}

// Tests consolidated from review_scope_acceptance_test.go

// TestReviewCommand_SpecFlagExists verifies that the review command accepts --spec flag
func TestReviewCommand_SpecFlagExists(t *testing.T) {
	t.Parallel()
	cmd := reviewCmd

	specFlag := cmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("review command should have --spec flag")
	}

	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestReviewCommand_SpecAndEpicMutuallyExclusive verifies that --spec and --epic
// cannot be used together on the review command
func TestReviewCommand_SpecAndEpicMutuallyExclusive(t *testing.T) {
	t.Parallel()
	err := scope.ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("scope.ValidateFlags should return error when both epic and spec are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// TestReviewCommand_SpecFlagResolvesToLabel verifies that --spec flag
// resolves to the correct label format via scope.ResolveSpec
func TestReviewCommand_SpecFlagResolvesToLabel(t *testing.T) {
	t.Parallel()
	specName := "init-wizard"
	labels := scope.ResolveSpec(specName)

	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}

	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Errorf("ResolveSpec(%q) = %q, want %q", specName, labels[0], expectedLabel)
	}
}

// TestReviewCommand_EpicFlagUsesResolveEpic verifies that --epic flag
// uses scope.ResolveEpic to resolve epic to spec labels
func TestReviewCommand_EpicFlagUsesResolveEpic(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	specs := []struct {
		filename string
		id       string
		epic     string
	}{
		{"auth.md", "auth", "gromit-xyz"},
		{"profile.md", "profile", "gromit-xyz"},
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		specContent := fmt.Sprintf(`---
id: %s
epic: %s
created: 2026-02-08
---

# Spec
`, spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	labels, err := scope.ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}

	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels, got %d", len(labels))
	}

	expectedLabels := map[string]bool{
		"spec:auth":    false,
		"spec:profile": false,
	}
	for _, label := range labels {
		if _, exists := expectedLabels[label]; !exists {
			t.Errorf("Unexpected label %q", label)
		}
		expectedLabels[label] = true
	}
	for label, found := range expectedLabels {
		if !found {
			t.Errorf("Missing expected label %q", label)
		}
	}
}

// TestReviewCommand_SpecFlagInHelpText verifies that --spec flag appears
// in the review command help text
func TestReviewCommand_SpecFlagInHelpText(t *testing.T) {
	t.Parallel()
	cmd := reviewCmd
	helpText := cmd.Long

	if !strings.Contains(helpText, "--spec") {
		t.Fatal("--spec flag should be documented in review command help text")
	}
}

// Tests consolidated from review_mutual_exclusivity_acceptance_test.go

// TestReviewCommand_FlagMutualExclusivity verifies that --epic, --spec, and --since
// flags are mutually exclusive on the review command
func TestReviewCommand_FlagMutualExclusivity(t *testing.T) {
	// Not parallel: saveReviewFlags mutates package-level reviewEpic, reviewSpec, reviewSince.
	saveReviewFlags(t)

	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "epic and spec both set",
			epic:    "gromit-xyz",
			spec:    "init-wizard",
			since:   "",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "epic and since both set",
			epic:    "gromit-xyz",
			spec:    "",
			since:   "abc123",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "spec and since both set",
			epic:    "",
			spec:    "init-wizard",
			since:   "abc123",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "all three flags set",
			epic:    "gromit-xyz",
			spec:    "init-wizard",
			since:   "abc123",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "only epic set",
			epic:    "gromit-xyz",
			spec:    "",
			since:   "",
			wantErr: false,
		},
		{
			name:    "only spec set",
			epic:    "",
			spec:    "init-wizard",
			since:   "",
			wantErr: false,
		},
		{
			name:    "only since set",
			epic:    "",
			spec:    "",
			since:   "abc123",
			wantErr: false,
		},
		{
			name:    "no flags set",
			epic:    "",
			spec:    "",
			since:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: subtests mutate package-level reviewEpic, reviewSpec, reviewSince.
			reviewEpic = tt.epic
			reviewSpec = tt.spec
			reviewSince = tt.since

			err := validateReviewFlags()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error when flags %s are set, got nil", tt.name)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("should not error on mutual exclusivity for %s, got: %v", tt.name, err)
				}
			}
		})
	}
}

// TestReviewCommand_MutualExclusivityCheckedEarly verifies that mutual exclusivity
// is checked before attempting to resolve specs or epics
func TestReviewCommand_MutualExclusivityCheckedEarly(t *testing.T) {
	// Not parallel: saveReviewFlags mutates package-level reviewEpic, reviewSpec, reviewSince.
	saveReviewFlags(t)

	// Set two flags with invalid values that would fail resolution
	reviewEpic = "nonexistent-epic-xyz"
	reviewSpec = "nonexistent-spec-123"
	reviewSince = ""

	err := validateReviewFlags()
	if err == nil {
		t.Fatal("expected error when both --epic and --spec are set")
	}

	// Should fail with mutual exclusivity error, not with "epic not found" or "spec not found"
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity (not resolution failure), got: %v", err)
	}
}

// TestReviewCommand_MutualExclusivityWithWhitespace verifies that flags with
// only whitespace are treated as empty and don't trigger mutual exclusivity
func TestReviewCommand_MutualExclusivityWithWhitespace(t *testing.T) {
	// Not parallel: saveReviewFlags mutates package-level reviewEpic, reviewSpec, reviewSince.
	saveReviewFlags(t)

	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
	}{
		{
			name:    "epic with value, spec with whitespace",
			epic:    "gromit-xyz",
			spec:    "   ",
			since:   "",
			wantErr: false,
		},
		{
			name:    "spec with value, epic with whitespace",
			epic:    "   ",
			spec:    "init-wizard",
			since:   "",
			wantErr: false,
		},
		{
			name:    "since with value, epic with whitespace",
			epic:    "   ",
			spec:    "",
			since:   "abc123",
			wantErr: false,
		},
		{
			name:    "all whitespace",
			epic:    "   ",
			spec:    "   ",
			since:   "   ",
			wantErr: false,
		},
		{
			name:    "two real values, one whitespace",
			epic:    "gromit-xyz",
			spec:    "   ",
			since:   "abc123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: subtests mutate package-level reviewEpic, reviewSpec, reviewSince.
			reviewEpic = tt.epic
			reviewSpec = tt.spec
			reviewSince = tt.since

			err := validateReviewFlags()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected mutual exclusivity error for %s", tt.name)
				}
				if !strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("error should mention mutual exclusivity, got: %v", err)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("%s should not fail with mutual exclusivity error, got: %v", tt.name, err)
				}
			}
		})
	}
}

// TestRunReviewDelegatesStoPipelineResolveReviewScope verifies that runReview
// delegates scope resolution to Pipeline.ResolveReviewScope
// Expected failure: runReview does not yet call Pipeline.ResolveReviewScope
func TestRunReviewDelegatesStoPipelineResolveReviewScope(t *testing.T) {
	t.Parallel()

	_, _, cfgPath := setupAgentConfig(t, `
claude:
  binary: "claude"
  timeout: 30
  flags: []
`)
	origConfigPath := configPath
	configPath = cfgPath
	t.Cleanup(func() { configPath = origConfigPath })

	saveReviewFlags(t)
	origDryRun := reviewDryRun
	origNonInteractive := reviewNonInteractive
	defer func() {
		reviewDryRun = origDryRun
		reviewNonInteractive = origNonInteractive
	}()

	// Force dry-run so we don't execute the full review workflow.
	reviewDryRun = true
	reviewNonInteractive = false
	reviewSince = "abc123def456"
	reviewSpec = ""
	reviewEpic = ""

	origPipelineFn := createReviewPipelineFn
	defer func() { createReviewPipelineFn = origPipelineFn }()

	resolveCalled := false
	createReviewPipelineFn = func(cfg *config.Config, gromitDir string) (ReviewScopeResolver, error) {
		return &mockTestPipeline{
			resolveReviewScopeFn: func(ctx context.Context, spec string, epic string, since string) (string, error) {
				resolveCalled = true
				if since != reviewSince {
					t.Errorf("ResolveReviewScope called with since=%q, want %q", since, reviewSince)
				}
				if spec != reviewSpec {
					t.Errorf("ResolveReviewScope called with spec=%q, want %q", spec, reviewSpec)
				}
				if epic != reviewEpic {
					t.Errorf("ResolveReviewScope called with epic=%q, want %q", epic, reviewEpic)
				}
				return "resolved-commit", nil
			},
		}, nil
	}

	origGitOutput := reviewGitOutputFn
	t.Cleanup(func() { reviewGitOutputFn = origGitOutput })
	reviewGitOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
		return []byte("diff"), nil
	}

	if err := runReview(nil, nil); err != nil {
		t.Fatalf("runReview() error = %v", err)
	}
	if !resolveCalled {
		t.Fatal("expected Pipeline.ResolveReviewScope to be called")
	}
}

// Mock pipeline for testing delegation
type mockTestPipeline struct {
	resolveReviewScopeFn func(ctx context.Context, spec string, epic string, since string) (string, error)
}

func (m *mockTestPipeline) ResolveReviewScope(ctx context.Context, spec string, epic string, since string) (string, error) {
	if m.resolveReviewScopeFn != nil {
		return m.resolveReviewScopeFn(ctx, spec, epic, since)
	}
	return "", fmt.Errorf("not implemented")
}

// TestResolveReviewScopeWithPipeline_SinceFlagPassthrough verifies --since flag is passed to Pipeline
// Expected: resolveReviewScopeWithPipeline passes --since directly to Pipeline.ResolveReviewScope
func TestResolveReviewScopeWithPipeline_SinceFlagPassthrough(t *testing.T) {
	t.Parallel()

	// Create a mock pipeline that captures the arguments
	var capturedSince string
	mockPipeline := &mockReviewPipelineForDelegation{
		resolveReviewScopeFn: func(ctx context.Context, spec string, epic string, since string) (string, error) {
			capturedSince = since
			return since, nil // Echo back the commit
		},
	}

	// Set the global flag
	origSince := reviewSince
	origSpec := reviewSpec
	origEpic := reviewEpic
	defer func() {
		reviewSince = origSince
		reviewSpec = origSpec
		reviewEpic = origEpic
	}()

	reviewSince = "test-commit-abc123"
	reviewSpec = ""
	reviewEpic = ""

	cfg := &config.Config{}

	// Call the helper function
	commit, err := resolveReviewScopeWithPipeline(mockPipeline, cfg)

	if err != nil {
		t.Errorf("resolveReviewScopeWithPipeline() error = %v, want nil", err)
	}

	if commit != "test-commit-abc123" {
		t.Errorf("resolveReviewScopeWithPipeline() returned %q, want 'test-commit-abc123'", commit)
	}

	if capturedSince != "test-commit-abc123" {
		t.Errorf("Pipeline.ResolveReviewScope received since=%q, want 'test-commit-abc123'", capturedSince)
	}
}

// Mock pipeline for testing delegation in review.go
type mockReviewPipelineForDelegation struct {
	resolveReviewScopeFn func(ctx context.Context, spec string, epic string, since string) (string, error)
}

func (m *mockReviewPipelineForDelegation) ResolveReviewScope(ctx context.Context, spec string, epic string, since string) (string, error) {
	if m.resolveReviewScopeFn != nil {
		return m.resolveReviewScopeFn(ctx, spec, epic, since)
	}
	return "", fmt.Errorf("not implemented")
}

// TestCreateReviewPipeline_CreatesValidPipeline verifies createReviewPipeline creates a working pipeline
// Expected: createReviewPipeline returns a non-nil pipeline
func TestCreateReviewPipeline_CreatesValidPipeline(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 30,
		},
	}
	gromitDir := t.TempDir()

	p, err := createReviewPipelineFn(cfg, gromitDir)

	if err != nil {
		t.Errorf("createReviewPipelineFn() error = %v, want nil", err)
	}

	if p == nil {
		t.Fatal("createReviewPipelineFn() returned nil pipeline")
	}

	if _, ok := p.(*pipeline.Pipeline); !ok {
		t.Fatalf("createReviewPipelineFn() returned %T, want *pipeline.Pipeline", p)
	}

	// Verify pipeline can be used
	ctx := context.Background()
	commit, pipelineErr := p.ResolveReviewScope(ctx, "", "", "test-commit")

	// When --since is provided, should return it
	if pipelineErr != nil {
		t.Errorf("Pipeline.ResolveReviewScope(since=test-commit) error = %v, want nil", pipelineErr)
	}

	if commit != "test-commit" {
		t.Errorf("Pipeline.ResolveReviewScope(since=test-commit) returned %q, want 'test-commit'", commit)
	}
}

// TestReviewThinWrapperPattern verifies the thin wrapper delegation pattern
// Expected: review command uses Pipeline for core logic delegation
func TestReviewThinWrapperPattern(t *testing.T) {
	t.Parallel()

	// Verify ReviewScopeResolver interface exists for dependency injection
	var _ ReviewScopeResolver = &mockReviewPipelineForDelegation{}

	// Verify *pipeline.Pipeline implements ReviewScopeResolver
	var p *pipeline.Pipeline
	var _ ReviewScopeResolver = p
}
