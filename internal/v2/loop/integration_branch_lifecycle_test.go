package loop

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/v2/adapter"
    gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
    "github.com/danabrams/gromit/internal/v2/event"
    "github.com/danabrams/gromit/internal/v2/presentation"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestIntegrationBranchLifecycleFailurePreservesBranch(t *testing.T) {
    t.Parallel()

    if _, err := exec.LookPath("git"); err != nil {
        t.Skip("git not available")
    }

    const specID = "spec-branch-lifecycle-failure"
    repoRoot := t.TempDir()
    initGitRepo(t, repoRoot)

    verifyFailureBranchLifecycle(t, repoRoot, specID)
}

func verifyFailureBranchLifecycle(t *testing.T, repoRoot, specID string) {
    t.Helper()

    acceptStage := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})
    worktree, err := runIntegrationSpecLoop(t, repoRoot, specID, acceptStage)
    if err == nil {
        t.Fatalf("expected spec loop failure, got nil")
    }

    assertBranchExists(t, repoRoot, specID)
    eventsPath := filepath.Join(worktree, ".gromit", "v2", "events.jsonl")
    assertEventsFilePopulated(t, eventsPath)
}

func runIntegrationSpecLoop(t *testing.T, repoRoot, specID string, accept stagepkg.Stage) (string, error) {
    t.Helper()

    if accept == nil {
        t.Fatalf("accept stage required")
    }

    worktreesDir := filepath.Join(repoRoot, ".gromit", "spec-worktrees")
    gitAdapter := newRecordingExecGitAdapter(repoRoot, worktreesDir)
    typedEmitter := event.NewEmitter()

    cfg := &config.Config{}
    presenter := newIntegrationPresenterAdapter(t)
    presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)
    decompose, beadLoop := newIntegrationLoopComponents(t, specID)

    adapters := adapter.AdapterSet{
        Git:         gitAdapter,
        LLM:         newIntegrationLLMAdapter(),
        TaskTracker: newIntegrationTaskTrackerAdapter(),
        Presenter:   presenter,
    }

    loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
        WithTypedEmitter(typedEmitter),
        WithPlanStage(newFakePlanStage(specID)),
        WithPresentStage(presentStage, summaryCtx),
        WithDecomposeStage(decompose),
        WithBeadLoop(beadLoop),
        WithAcceptStage(accept),
    )
    if err != nil {
        t.Fatalf("create spec loop: %v", err)
    }

    runErr := loopInstance.Run(context.Background(), specID, nil)
    typedEmitter.Close()

    if gitAdapter.lastWorktree == "" {
        t.Fatalf("checkout not recorded")
    }

    return gitAdapter.lastWorktree, runErr
}

func assertBranchExists(t *testing.T, repoRoot, specID string) {
    t.Helper()

    branch := presentation.SpecBranchName(specID)
    if _, err := runGitCommand(repoRoot, "rev-parse", "--verify", branch); err != nil {
        t.Fatalf("branch %s not found: %v", branch, err)
    }
}

func assertEventsFilePopulated(t *testing.T, path string) {
    t.Helper()

    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read events log: %v", err)
    }
    if len(data) == 0 {
        t.Fatalf("events log %s is empty", path)
    }
}

func runGitCommand(repoRoot string, args ...string) (string, error) {
    cmd := exec.Command("git", args...)
    cmd.Dir = repoRoot
    out, err := cmd.CombinedOutput()
    if err != nil {
        return string(out), fmt.Errorf("git %v: %s: %w", args, out, err)
    }
    return string(out), nil
}

type recordingExecGitAdapter struct {
    *gitadapter.ExecGitAdapter
    lastWorktree string
}

func newRecordingExecGitAdapter(repoRoot, worktreesDir string) *recordingExecGitAdapter {
    return &recordingExecGitAdapter{ExecGitAdapter: gitadapter.NewExecGitAdapter(repoRoot, worktreesDir)}
}

func (r *recordingExecGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
    worktree, err := r.ExecGitAdapter.Checkout(ctx, specID)
    if err != nil {
        return "", err
    }
    r.lastWorktree = worktree
    return worktree, nil
}
