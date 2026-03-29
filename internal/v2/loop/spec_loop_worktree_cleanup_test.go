package loop

import (
	"context"
	"fmt"
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
		ExecGitAdapter: gitadapter.NewExecGitAdapter(repoRoot, worktreesDir),
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
		WithPreserveOnFailure(false),
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

func TestCleanupWorktreeSuccessOptionSkipsPartialCommit(t *testing.T) {
	t.Parallel()

	gitAdapter := &ctxCheckingGitAdapter{}
	gitAdapter.t = t

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPreserveOnFailure(false),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	successOpts := cleanupOptions{specID: "test-spec", worktree: "/tmp/fake-worktree", success: true}
	if err := loopInstance.cleanupWorktree(context.Background(), successOpts); err != nil {
		t.Fatalf("cleanupWorktree success should not error: %v", err)
	}
	if len(gitAdapter.commitMessages) != 0 {
		t.Fatalf("unexpected commits when success true: %d", len(gitAdapter.commitMessages))
	}
	if len(gitAdapter.removedWorktrees) != 1 {
		t.Fatalf("expected removal on success, got %d", len(gitAdapter.removedWorktrees))
	}

	failOpts := cleanupOptions{specID: "test-spec", worktree: "/tmp/fake-worktree", success: false}
	if err := loopInstance.cleanupWorktree(context.Background(), failOpts); err != nil {
		t.Fatalf("cleanupWorktree failure should not error: %v", err)
	}
	if len(gitAdapter.commitMessages) != 1 {
		t.Fatalf("expected 1 commit after failure, got %d", len(gitAdapter.commitMessages))
	}
	if len(gitAdapter.removedWorktrees) != 2 {
		t.Fatalf("expected second removal after failure, got %d", len(gitAdapter.removedWorktrees))
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

// failingRemoveGitAdapter returns an error from RemoveWorktree.
type failingRemoveGitAdapter struct {
	fakeGitAdapter
	removeErr    error
	removeCalled bool
}

func (f *failingRemoveGitAdapter) RemoveWorktree(_ context.Context, worktree string) error {
	f.removeCalled = true
	f.removedWorktrees = append(f.removedWorktrees, worktree)
	return f.removeErr
}

// TestCleanupWorktreePreservesWorktreeWhenPreserveOnFailureIsDefault verifies that
// cleanupWorktree does NOT call RemoveWorktree on failure when PreserveOnFailure
// is not set (default true).
func TestCleanupWorktreePreservesWorktreeWhenPreserveOnFailureIsDefault(t *testing.T) {
	t.Parallel()

	gitAdapter := &failingRemoveGitAdapter{removeErr: fmt.Errorf("should not be called")}
	gitAdapter.t = t

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{})
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	opts := cleanupOptions{specID: "test-spec", worktree: "/tmp/fake-worktree", success: false}
	if err := loopInstance.cleanupWorktree(context.Background(), opts); err != nil {
		t.Fatalf("cleanupWorktree should not error when preserving on failure, got: %v", err)
	}
	if gitAdapter.removeCalled {
		t.Fatal("RemoveWorktree should not be called when PreserveOnFailure is true (default)")
	}
}

// TestCleanupWorktreeReturnsErrorWhenRemoveWorktreeFails verifies that the
// cleanupWorktree method surfaces the RemoveWorktree error to the caller.
func TestCleanupWorktreeReturnsErrorWhenRemoveWorktreeFails(t *testing.T) {
	t.Parallel()

	removeErr := fmt.Errorf("permission denied removing worktree")
	gitAdapter := &failingRemoveGitAdapter{removeErr: removeErr}
	gitAdapter.t = t

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{})
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.cleanupWorktree(context.Background(), cleanupOptions{specID: "test-spec", worktree: "/tmp/fake-worktree", success: true})
	if err == nil {
		t.Fatal("expected error when RemoveWorktree fails")
	}
	if !strings.Contains(err.Error(), "remove worktree") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "remove worktree")
	}
	if !gitAdapter.removeCalled {
		t.Fatal("expected RemoveWorktree to be called")
	}
}

// TestDeferredCleanupSilentlyDiscardsRemoveWorktreeError verifies the behavior
// of the deferred cleanup in Run: when a non-checkout error causes Run to fail
// and neither handleFailure nor success paths ran, the deferred cleanup invokes
// RemoveWorktree but silently discards its error with `_ = ...`.
//
// NOTE: The current production code (spec_loop.go, deferred cleanup in Run)
// uses `_ = s.adapters.Git.RemoveWorktree(...)`, meaning any RemoveWorktree
// error in the deferred path is intentionally discarded. This test documents
// that behavior.
func TestDeferredCleanupSilentlyDiscardsRemoveWorktreeError(t *testing.T) {
	t.Parallel()

	removeErr := fmt.Errorf("deferred cleanup removal failed")
	gitAdapter := &failingRemoveGitAdapter{removeErr: removeErr}
	gitAdapter.t = t

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		// No plan stage — Run will fail at plan stage check, triggering deferred cleanup.
		WithPreserveOnFailure(false),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(context.Background(), "spec-deferred-cleanup", nil)
	if err == nil {
		t.Fatal("expected error from Run (nil plan stage)")
	}
	// The Run error should be about the plan stage, NOT about RemoveWorktree,
	// because the deferred cleanup discards the RemoveWorktree error.
	if !strings.Contains(err.Error(), "plan stage") {
		t.Fatalf("error = %q, want it to reference plan stage failure not worktree removal", err.Error())
	}
	// The deferred cleanup should still have attempted RemoveWorktree.
	if !gitAdapter.removeCalled {
		t.Fatal("expected deferred cleanup to call RemoveWorktree even though error is discarded")
	}
}

// TestWorktreeCheckoutUsesNamedBranch verifies that ExecGitAdapter.Checkout creates
// a worktree on a named branch (gromit/spec/<specID>), not a detached HEAD.
func TestWorktreeCheckoutUsesNamedBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git not available")
	}

	repoRoot := t.TempDir()
	initGitRepo(t, repoRoot)

	worktreesDir := filepath.Join(repoRoot, ".gromit", "spec-worktrees")
	gitAdapter := gitadapter.NewExecGitAdapter(repoRoot, worktreesDir)

	wtPath, err := gitAdapter.Checkout(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	branchName := strings.TrimSpace(gitCommand(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD"))
	if branchName == "HEAD" {
		t.Fatal("worktree is on detached HEAD, expected a named branch")
	}
	if branchName != "gromit/spec/test-spec" {
		t.Fatalf("branch = %q, want %q", branchName, "gromit/spec/test-spec")
	}
}

// ctxCheckingGitAdapter records whether RemoveWorktree received a non-cancelled context.
type ctxCheckingGitAdapter struct {
	fakeGitAdapter
	removeCtxErr error
}

func (c *ctxCheckingGitAdapter) RemoveWorktree(ctx context.Context, worktree string) error {
	c.removeCtxErr = ctx.Err()
	c.removedWorktrees = append(c.removedWorktrees, worktree)
	return nil
}

func (c *ctxCheckingGitAdapter) Status(ctx context.Context, worktree string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return "M dirty-file.go", nil
}

func (c *ctxCheckingGitAdapter) Commit(ctx context.Context, worktree, message string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	c.commitMessages = append(c.commitMessages, message)
	return "fake-sha", nil
}

func TestCleanupWorktreeUsesNonCancelledContext(t *testing.T) {
	t.Parallel()

	gitAdapter := &ctxCheckingGitAdapter{}
	gitAdapter.t = t

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithPreserveOnFailure(false),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	// Cancel the parent context before calling cleanupWorktree.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// cleanupWorktree should succeed because it creates a fresh context internally.
	opts := cleanupOptions{specID: "test-spec", worktree: "/tmp/fake-worktree", success: false}
	if err := loopInstance.cleanupWorktree(ctx, opts); err != nil {
		t.Fatalf("cleanupWorktree should succeed with cancelled context, got: %v", err)
	}

	// The git adapter should have received a non-cancelled context for RemoveWorktree.
	if gitAdapter.removeCtxErr != nil {
		t.Fatalf("RemoveWorktree received cancelled context: %v", gitAdapter.removeCtxErr)
	}

	// Verify partial work was committed (status returned dirty).
	if len(gitAdapter.commitMessages) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(gitAdapter.commitMessages))
	}
	if !strings.Contains(gitAdapter.commitMessages[0], "test-spec") {
		t.Fatalf("commit message should contain spec ID, got: %q", gitAdapter.commitMessages[0])
	}

	// Verify worktree was removed.
	if len(gitAdapter.removedWorktrees) != 1 {
		t.Fatalf("expected 1 removed worktree, got %d", len(gitAdapter.removedWorktrees))
	}
}
