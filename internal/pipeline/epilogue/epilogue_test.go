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
