package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/event"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
)

func TestIntegrationBranchLifecycle_PreservesBranchAndEventLogOnAndonFailure(t *testing.T) {
	t.Parallel()

	assertFailureBranchLifecycle(t, "spec-branch-lifecycle-andon", fmt.Errorf("andon triggered"))
}

func TestIntegrationBranchLifecycle_PreservesBranchAndEventLogOnGenerationCap(t *testing.T) {
	t.Parallel()

	assertGenerationCapBranchLifecycle(t, "spec-branch-lifecycle-generation-cap")
}

func TestIntegrationBranchLifecycle_DeletesWorktreeAndBranchOnSuccess(t *testing.T) {
	t.Parallel()

	assertSuccessBranchLifecycle(t, "spec-branch-lifecycle-success")
}

func assertGenerationCapBranchLifecycle(t *testing.T, specID string) {
	t.Helper()

	assertFailureBranchLifecycle(t, specID, ErrGenerationCapReached)
}

type fixedErrorRemediationRunner struct {
	err error
}

func (r fixedErrorRemediationRunner) Run(_ context.Context, _, _ string) error {
	return r.err
}

func assertFailureBranchLifecycle(t *testing.T, specID string, remediationErr error) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	initGitRepo(t, repoRoot)

	worktreesDir := filepath.Join(repoRoot, ".gromit", "spec-worktrees")
	gitAdapter := gitadapter.NewExecGitAdapter(repoRoot, worktreesDir)

	typedEmitter := event.NewEmitter()
	t.Cleanup(func() {
		typedEmitter.Close()
	})

	adapters := adapter.AdapterSet{
		Git:         gitAdapter,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   newFakePresenterAdapter(t),
	}

	loopInstance, err := NewSpecLoop(adapters, &config.Config{}, noopDependencyGate{},
		WithTypedEmitter(typedEmitter),
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(newFakePresentStage(), &present.SummaryContext{}),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionFail})),
		WithRemediationRunner(fixedErrorRemediationRunner{err: remediationErr}),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	err = loopInstance.Run(context.Background(), specID, nil)
	if err == nil {
		t.Fatal("expected run error")
	}
	if !errors.Is(err, remediationErr) {
		t.Fatalf("run error = %v, want wrapped remediation error %v", err, remediationErr)
	}

	branchName := "gromit/spec/" + specID
	if !branchExistsInRepo(t, repoRoot, branchName) {
		t.Fatalf("branch %q should be preserved on failure", branchName)
	}

	worktreePath := filepath.Join(worktreesDir, specID)
	if !worktreeRegistered(t, repoRoot, worktreePath) {
		t.Fatalf("worktree %s should still be registered for debugging", worktreePath)
	}

	eventsPath := filepath.Join(worktreePath, ".gromit", "v2", "events.jsonl")
	eventsData, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events file: %v", err)
	}
	if !strings.Contains(string(eventsData), "\"type\":\"spec.started\"") {
		t.Fatalf("events file %s missing spec.started event: %s", eventsPath, eventsData)
	}
	if !strings.Contains(string(eventsData), "\"spec_id\":\""+specID+"\"") {
		t.Fatalf("events file %s missing spec id %q: %s", eventsPath, specID, eventsData)
	}

	committedLog := gitCommand(t, repoRoot, "show", branchName+":.gromit/v2/events.jsonl")
	if !strings.Contains(committedLog, "\"spec_id\":\""+specID+"\"") {
		t.Fatalf("preserved branch log missing spec id %q: %s", specID, committedLog)
	}
}
