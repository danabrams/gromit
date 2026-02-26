package epilogue_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
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
	writeFn               func(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error
	called                bool
	lastIteration         int
	lastBeadID            string
	lastBeadTitle         string
	lastModel             string
	lastMaxIterations     int
	lastTimeBudgetMinutes int
}

func (f *fakeStatusWriter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	f.called = true
	f.lastIteration = iteration
	f.lastBeadID = beadID
	f.lastBeadTitle = beadTitle
	f.lastModel = model
	f.lastMaxIterations = maxIterations
	f.lastTimeBudgetMinutes = timeBudgetMinutes
	if f.writeFn != nil {
		return f.writeFn(iteration, beadID, beadTitle, model, maxIterations, timeBudgetMinutes)
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
	pendingCallCount int
	mergedBranches []string
}

func (f *fakeWorktreeMerger) PendingBranches() ([]string, error) {
	f.pendingCalled = true
	f.pendingCallCount++
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

func TestEpilogue_DeduplicatesPendingBranchesBeforeMerge(t *testing.T) {
	merger := &fakeWorktreeMerger{
		branches: []string{"gromit/review-1", "gromit/review-1", "gromit/review-2"},
	}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithWorktree(merger)

	in := makeInput("bead-1", "Test", true)
	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if merger.pendingCallCount != 1 {
		t.Fatalf("PendingBranches() calls = %d, want 1", merger.pendingCallCount)
	}
	if len(merger.mergedBranches) != 2 {
		t.Fatalf("merged branches = %v, want 2 unique merges", merger.mergedBranches)
	}
	if merger.mergedBranches[0] != "gromit/review-1" || merger.mergedBranches[1] != "gromit/review-2" {
		t.Fatalf("merged branches order = %v, want [gromit/review-1 gromit/review-2]", merger.mergedBranches)
	}
}

func TestEpilogue_DeduplicatesRepeatedMergeWarningsAcrossIterations(t *testing.T) {
	merger := &fakeWorktreeMerger{
		branches:   []string{"gromit/review-1"},
		mergeErr:   errors.New("merge conflict for branch gromit/review-1"),
	}
	var out bytes.Buffer
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, &out).
		WithWorktree(merger)

	in := makeInput("bead-1", "Test", true)
	if _, err := stage.Run(context.Background(), in); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	if _, err := stage.Run(context.Background(), in); err != nil {
		t.Fatalf("second Run() error = %v, want nil", err)
	}

	got := strings.Count(out.String(), "Warning: failed to merge branch gromit/review-1")
	if got != 1 {
		t.Fatalf("warning count = %d, want 1; output:\n%s", got, out.String())
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

// fakeFailureLearner is a test double for epilogue.FailureLearner.
type fakeFailureLearner struct {
	called        bool
	beadID        string
	failureOutput string
	callFn        func(ctx context.Context, beadID, beadTitle, failureOutput string) error
}

func (f *fakeFailureLearner) ExtractFailureLearning(ctx context.Context, beadID, beadTitle, failureOutput string) error {
	f.called = true
	f.beadID = beadID
	f.failureOutput = failureOutput
	if f.callFn != nil {
		return f.callFn(ctx, beadID, beadTitle, failureOutput)
	}
	return nil
}

// TestEpilogue_FailurePath_CallsFailureLearner verifies that failure-path learning is extracted
// unconditionally when the build did not succeed.
func TestEpilogue_FailurePath_CallsFailureLearner(t *testing.T) {
	learner := &fakeFailureLearner{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithFailureLearner(learner)

	in := makeInput("bead-1", "Implement feature", false) // failure path

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !learner.called {
		t.Error("FailureLearner.ExtractFailureLearning() was not called; want failure-path learning extracted unconditionally")
	}
	if learner.beadID != "bead-1" {
		t.Errorf("FailureLearner.ExtractFailureLearning() beadID = %q, want %q", learner.beadID, "bead-1")
	}
}

// TestEpilogue_SuccessPath_DoesNotCallFailureLearner verifies that failure-path learning
// is not extracted on the success path.
func TestEpilogue_SuccessPath_DoesNotCallFailureLearner(t *testing.T) {
	learner := &fakeFailureLearner{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithFailureLearner(learner)

	in := makeInput("bead-1", "Implement feature", true) // success path

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if learner.called {
		t.Error("FailureLearner.ExtractFailureLearning() was called on success path; want no failure-path learning")
	}
}

// TestEpilogue_FailureLearner_CalledRegardlessOfTierOrPackageNovelty verifies that the
// failure learner is called even for low-tier beads touching previously-seen packages.
func TestEpilogue_FailureLearner_CalledRegardlessOfTierOrPackageNovelty(t *testing.T) {
	learner := &fakeFailureLearner{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithFailureLearner(learner)

	in := makeInput("bead-2", "Low-tier bead", false)
	// Simulate low-tier bead with previously-seen packages via TouchedPackages
	in.TouchedPackages = []string{"internal/pipeline"} // packages already seen

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !learner.called {
		t.Error("FailureLearner.ExtractFailureLearning() was not called; want failure-path learning regardless of tier/novelty")
	}
}

// TestEpilogue_FailurePath_PassesFailureOutputToLearner verifies that the raw
// FailureOutput from Input reaches the failure learner's failureOutput parameter.
func TestEpilogue_FailurePath_PassesFailureOutputToLearner(t *testing.T) {
	learner := &fakeFailureLearner{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithFailureLearner(learner)

	in := makeInput("bead-1", "Implement feature", false)
	in.FailureOutput = "--- FAIL: TestFoo\n    expected 1 got 2"

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !learner.called {
		t.Fatal("FailureLearner was not called on failure path")
	}
	if learner.failureOutput != "--- FAIL: TestFoo\n    expected 1 got 2" {
		t.Errorf("FailureLearner received failureOutput %q, want %q", learner.failureOutput, "--- FAIL: TestFoo\n    expected 1 got 2")
	}
}

// TestEpilogue_OutputContainsTouchedPackages verifies that the Epilogue returns
// Input.TouchedPackages in Output.TouchedPackages for orchestrator accumulation.
func TestEpilogue_OutputContainsTouchedPackages(t *testing.T) {
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard)

	in := makeInput("bead-1", "Test", true)
	in.TouchedPackages = []string{"internal/pipeline", "internal/runner"}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(out.TouchedPackages) != 2 {
		t.Errorf("Output.TouchedPackages: want 2 items, got %d", len(out.TouchedPackages))
	}
	if len(out.TouchedPackages) > 0 && out.TouchedPackages[0] != "internal/pipeline" {
		t.Errorf("Output.TouchedPackages[0]: want %q, got %q", "internal/pipeline", out.TouchedPackages[0])
	}
}

// TestEpilogue_NilFailureLearner_NoopOnFailure verifies that a nil FailureLearner
// does not panic when the build fails.
func TestEpilogue_NilFailureLearner_NoopOnFailure(t *testing.T) {
	// No WithFailureLearner call — nil by default
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard)

	in := makeInput("bead-1", "Test", false)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
}

// fakeIterationLogWriter is a test double for epilogue.IterationLogWriter.
type fakeIterationLogWriter struct {
	called  bool
	lastLog *logger.IterationLog
	err     error
}

func (f *fakeIterationLogWriter) Write(log *logger.IterationLog) error {
	f.called = true
	f.lastLog = log
	return f.err
}

// TestEpilogue_WritesIterationLog_WhenResultPresent verifies that the iteration log
// is written when Input.Result is set.
func TestEpilogue_WritesIterationLog_WhenResultPresent(t *testing.T) {
	logWriter := &fakeIterationLogWriter{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithIterationLogWriter(logWriter)

	in := makeInput("bead-1", "Test", true)
	in.Result = &logger.IterationLog{
		BeadID:  "bead-1",
		Model:   "sonnet",
		Success: true,
	}

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !logWriter.called {
		t.Error("IterationLogWriter.Write was not called; want iteration log written when Result is present")
	}
}

// TestEpilogue_WritesIterationLog_UsageLimitedTrue verifies that when
// Input.Result.UsageLimited=true, the log writer receives an entry with UsageLimited=true.
func TestEpilogue_WritesIterationLog_UsageLimitedTrue(t *testing.T) {
	logWriter := &fakeIterationLogWriter{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithIterationLogWriter(logWriter)

	in := makeInput("bead-1", "Test usage limit", false)
	in.Result = &logger.IterationLog{
		BeadID:       "bead-1",
		Model:        "sonnet",
		Success:      false,
		UsageLimited: true,
	}

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !logWriter.called {
		t.Error("IterationLogWriter.Write was not called")
	}
	if logWriter.lastLog == nil || !logWriter.lastLog.UsageLimited {
		t.Error("expected UsageLimited=true in written log entry")
	}
}

// TestEpilogue_WritesIterationLog_UsageLimitedFalse verifies that when
// Input.Result.UsageLimited=false, the log writer receives an entry with UsageLimited=false.
func TestEpilogue_WritesIterationLog_UsageLimitedFalse(t *testing.T) {
	logWriter := &fakeIterationLogWriter{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithIterationLogWriter(logWriter)

	in := makeInput("bead-1", "Test no usage limit", true)
	in.Result = &logger.IterationLog{
		BeadID:       "bead-1",
		Model:        "sonnet",
		Success:      true,
		UsageLimited: false,
	}

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !logWriter.called {
		t.Error("IterationLogWriter.Write was not called")
	}
	if logWriter.lastLog != nil && logWriter.lastLog.UsageLimited {
		t.Error("expected UsageLimited=false in written log entry")
	}
}

// TestEpilogue_SkipsIterationLog_WhenNoResult verifies that the log writer is not
// called when Input.Result is nil.
func TestEpilogue_SkipsIterationLog_WhenNoResult(t *testing.T) {
	logWriter := &fakeIterationLogWriter{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithIterationLogWriter(logWriter)

	in := makeInput("bead-1", "Test", true)
	// in.Result is nil — no log entry to write

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if logWriter.called {
		t.Error("IterationLogWriter.Write was called; want no call when Result is nil")
	}
}

// TestEpilogue_StatusWrite_PassesModelFromResult verifies that when Input.Result
// is non-nil, the StatusWriter receives Result.Model instead of an empty string.
func TestEpilogue_StatusWrite_PassesModelFromResult(t *testing.T) {
	status := &fakeStatusWriter{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, status, io.Discard)

	in := makeInput("bead-1", "Implement feature", true)
	in.Result = &logger.IterationLog{
		BeadID: "bead-1",
		Model:  "sonnet",
	}

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !status.called {
		t.Fatal("StatusWriter.Write() was not called")
	}
	if status.lastModel != "sonnet" {
		t.Errorf("StatusWriter.Write() model = %q, want %q", status.lastModel, "sonnet")
	}
}

// TestEpilogue_SkipsIterationLog_WhenNoWriter verifies that when no log writer is
// configured, epilogue does not panic when Result is present.
func TestEpilogue_SkipsIterationLog_WhenNoWriter(t *testing.T) {
	// WithIterationLogWriter is not called — no log writer configured
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard)

	in := makeInput("bead-1", "Test", true)
	in.Result = &logger.IterationLog{
		BeadID:       "bead-1",
		Model:        "sonnet",
		UsageLimited: true,
	}

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (log write skipped when no writer configured)", err)
	}
}

// fakePendingBranchRemover is a test double for epilogue.PendingBranchRemover.
type fakePendingBranchRemover struct {
	removedBranches []string
	removeErr       error
}

func (f *fakePendingBranchRemover) RemovePendingWorktreeBranch(branch string) error {
	f.removedBranches = append(f.removedBranches, branch)
	return f.removeErr
}

// TestEpilogue_RemovesPendingBranchesAfterMerge verifies that pending worktree branches
// are removed from interactive state after successful merge.
func TestEpilogue_RemovesPendingBranchesAfterMerge(t *testing.T) {
	merger := &fakeWorktreeMerger{
		branches: []string{"gromit/review-1", "gromit/debug-1"},
	}
	remover := &fakePendingBranchRemover{}
	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard).
		WithWorktree(merger).
		WithPendingBranchRemover(remover)

	in := makeInput("bead-1", "Test", true)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(remover.removedBranches) != 2 {
		t.Errorf("removed branches = %v, want 2 branches removed", remover.removedBranches)
	}
	if remover.removedBranches[0] != "gromit/review-1" {
		t.Errorf("first removed branch = %q, want %q", remover.removedBranches[0], "gromit/review-1")
	}
	if remover.removedBranches[1] != "gromit/debug-1" {
		t.Errorf("second removed branch = %q, want %q", remover.removedBranches[1], "gromit/debug-1")
	}
}
