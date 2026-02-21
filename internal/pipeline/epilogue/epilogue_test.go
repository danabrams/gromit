package epilogue_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	epiloguepkg "github.com/danabrams/gromit/internal/pipeline/epilogue"
)

// fakeBeadLifecycle is a test double for epilogue.BeadLifecycle.
type fakeBeadLifecycle struct {
	closeFn     func(id string) error
	syncFn      func() error
	closeCalled bool
	syncCalled  bool
	closeID     string
	callOrder   []string
}

func (f *fakeBeadLifecycle) Close(id string) error {
	f.closeCalled = true
	f.closeID = id
	f.callOrder = append(f.callOrder, "close")
	if f.closeFn != nil {
		return f.closeFn(id)
	}
	return nil
}

func (f *fakeBeadLifecycle) Sync() error {
	f.syncCalled = true
	f.callOrder = append(f.callOrder, "sync")
	if f.syncFn != nil {
		return f.syncFn()
	}
	return nil
}

// fakeStatusWriter is a test double for epilogue.StatusWriter.
type fakeStatusWriter struct {
	writeFn       func(iteration int, beadID, beadTitle string) error
	called        bool
	lastIteration int
	lastBeadID    string
	lastBeadTitle string
}

func (f *fakeStatusWriter) Write(iteration int, beadID, beadTitle string) error {
	f.called = true
	f.lastIteration = iteration
	f.lastBeadID = beadID
	f.lastBeadTitle = beadTitle
	if f.writeFn != nil {
		return f.writeFn(iteration, beadID, beadTitle)
	}
	return nil
}

func makeInput(beadID, beadTitle string, succeeded bool) pipeline.Input {
	return pipeline.Input{
		Bead:           &bead.Bead{ID: beadID, Title: beadTitle},
		Config:         &config.Config{},
		Iteration:      1,
		Deadline:       time.Now().Add(time.Minute),
		BuildSucceeded: succeeded,
	}
}

// TestEpilogue_SuccessPath_ClosesBead verifies that on the success path, the bead is closed.
func TestEpilogue_SuccessPath_ClosesBead(t *testing.T) {
	beads := &fakeBeadLifecycle{}
	status := &fakeStatusWriter{}

	stage := epiloguepkg.New(beads, status, io.Discard)
	in := makeInput("bead-1", "Implement feature", true)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !beads.closeCalled {
		t.Error("Close() was not called; want bead closed on success path")
	}
	if beads.closeID != "bead-1" {
		t.Errorf("Close() called with ID %q, want %q", beads.closeID, "bead-1")
	}
}

// TestEpilogue_SuccessPath_SyncsAfterClose verifies that on the success path,
// Sync is called after Close.
func TestEpilogue_SuccessPath_SyncsAfterClose(t *testing.T) {
	beads := &fakeBeadLifecycle{}
	status := &fakeStatusWriter{}

	stage := epiloguepkg.New(beads, status, io.Discard)
	in := makeInput("bead-1", "Implement feature", true)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !beads.syncCalled {
		t.Error("Sync() was not called; want sync after close on success path")
	}
	if len(beads.callOrder) < 2 || beads.callOrder[0] != "close" || beads.callOrder[1] != "sync" {
		t.Errorf("call order = %v, want [close, sync]", beads.callOrder)
	}
}

// TestEpilogue_AlwaysWritesStatus verifies that status is written after each iteration.
func TestEpilogue_AlwaysWritesStatus(t *testing.T) {
	beads := &fakeBeadLifecycle{}
	status := &fakeStatusWriter{}

	stage := epiloguepkg.New(beads, status, io.Discard)
	in := makeInput("bead-1", "Implement feature", true)
	in.Iteration = 3

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !status.called {
		t.Error("StatusWriter.Write() was not called; want status written after each iteration")
	}
	if status.lastIteration != 3 {
		t.Errorf("StatusWriter.Write() iteration = %d, want 3", status.lastIteration)
	}
	if status.lastBeadID != "bead-1" {
		t.Errorf("StatusWriter.Write() beadID = %q, want %q", status.lastBeadID, "bead-1")
	}
}

// TestEpilogue_FailurePath_DoesNotCloseBead verifies that on the failure path,
// Close and Sync are not called.
func TestEpilogue_FailurePath_DoesNotCloseBead(t *testing.T) {
	beads := &fakeBeadLifecycle{}
	status := &fakeStatusWriter{}

	stage := epiloguepkg.New(beads, status, io.Discard)
	in := makeInput("bead-1", "Implement feature", false)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if beads.closeCalled {
		t.Error("Close() was called; want no bead close on failure path")
	}
	if beads.syncCalled {
		t.Error("Sync() was called; want no sync on failure path")
	}
}

// TestEpilogue_FailurePath_WritesStatus verifies that status is written even on the failure path.
func TestEpilogue_FailurePath_WritesStatus(t *testing.T) {
	beads := &fakeBeadLifecycle{}
	status := &fakeStatusWriter{}

	stage := epiloguepkg.New(beads, status, io.Discard)
	in := makeInput("bead-1", "Implement feature", false)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !status.called {
		t.Error("StatusWriter.Write() was not called; want status written even on failure path")
	}
}

// TestEpilogue_ReturnsProceed verifies that Epilogue always returns a Proceed decision.
func TestEpilogue_ReturnsProceed(t *testing.T) {
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard)
	in := makeInput("bead-1", "Test", true)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
}

// fakeWorktreeMerger is a test double for epilogue.WorktreeMerger.
type fakeWorktreeMerger struct {
	branches       []string
	pendingErr     error
	mergeErr       error
	pendingCalled  bool
	mergedBranches []string
}

func (f *fakeWorktreeMerger) PendingBranches() ([]string, error) {
	f.pendingCalled = true
	return f.branches, f.pendingErr
}

func (f *fakeWorktreeMerger) MergeBack(branch string) error {
	f.mergedBranches = append(f.mergedBranches, branch)
	return f.mergeErr
}

// fakeCommandRunner is a test double for epilogue.CommandRunner.
type fakeCommandRunner struct {
	called      bool
	lastCommand string
	stdout      string
	stderr      string
	exitCode    int
	err         error
}

func (f *fakeCommandRunner) Run(_ context.Context, command string) (string, string, int, error) {
	f.called = true
	f.lastCommand = command
	return f.stdout, f.stderr, f.exitCode, f.err
}

func boolPtr(b bool) *bool { return &b }

// TestEpilogue_MergesWorktreeBranches verifies that pending worktree branches are merged when
// worktree is enabled and auto-merge is enabled.
func TestEpilogue_MergesWorktreeBranches(t *testing.T) {
	merger := &fakeWorktreeMerger{
		branches: []string{"interactive/branch-1", "interactive/branch-2"},
	}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithWorktree(merger)

	// Config zero-value: Worktree.Enabled=nil → IsEnabled()=true, AutoMerge=nil → IsAutoMergeEnabled()=true
	in := makeInput("bead-1", "Test", true)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !merger.pendingCalled {
		t.Error("PendingBranches() was not called; want worktree merge attempted")
	}
	if len(merger.mergedBranches) != 2 {
		t.Errorf("merged branches = %v, want 2 branches merged", merger.mergedBranches)
	}
}

// TestEpilogue_SkipsMergeWhenWorktreeDisabled verifies that merge is skipped when worktree is disabled.
func TestEpilogue_SkipsMergeWhenWorktreeDisabled(t *testing.T) {
	merger := &fakeWorktreeMerger{
		branches: []string{"interactive/branch-1"},
	}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithWorktree(merger)

	in := makeInput("bead-1", "Test", true)
	in.Config.Worktree.Enabled = boolPtr(false)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if merger.pendingCalled {
		t.Error("PendingBranches() was called; want no merge when worktree disabled")
	}
}

// TestEpilogue_SkipsMergeWhenAutoMergeDisabled verifies that merge is skipped when auto-merge is
// disabled.
func TestEpilogue_SkipsMergeWhenAutoMergeDisabled(t *testing.T) {
	merger := &fakeWorktreeMerger{
		branches: []string{"interactive/branch-1"},
	}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithWorktree(merger)

	in := makeInput("bead-1", "Test", true)
	in.Config.Worktree.AutoMerge = boolPtr(false)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if merger.pendingCalled {
		t.Error("PendingBranches() was called; want no merge when auto-merge disabled")
	}
}

// TestEpilogue_RunsBetweenIterationsCommand verifies the between-iterations command is executed.
func TestEpilogue_RunsBetweenIterationsCommand(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithCommandRunner(runner)

	in := makeInput("bead-1", "Test", true)
	in.Config.Loop.BetweenIterationsCommand = "echo done"

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !runner.called {
		t.Error("CommandRunner.Run() was not called; want between-iterations command executed")
	}
	if runner.lastCommand != "echo done" {
		t.Errorf("command = %q, want %q", runner.lastCommand, "echo done")
	}
}

// TestEpilogue_SkipsCommandWhenEmpty verifies no command runs when BetweenIterationsCommand is empty.
func TestEpilogue_SkipsCommandWhenEmpty(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithCommandRunner(runner)

	in := makeInput("bead-1", "Test", true)
	// BetweenIterationsCommand defaults to ""

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if runner.called {
		t.Error("CommandRunner.Run() was called; want no command when BetweenIterationsCommand is empty")
	}
}

// TestEpilogue_CommandFailureIsWarning verifies that between-iterations command failure is non-fatal.
func TestEpilogue_CommandFailureIsWarning(t *testing.T) {
	runner := &fakeCommandRunner{exitCode: 1, stderr: "something failed"}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithCommandRunner(runner)

	in := makeInput("bead-1", "Test", true)
	in.Config.Loop.BetweenIterationsCommand = "failing-cmd"

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() returned error %v; want nil (command failures are warnings)", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed even after command failure", out.Decision)
	}
}
