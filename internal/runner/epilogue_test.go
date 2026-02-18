package runner

import (
	"bytes"
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// newEpilogueTestRunner returns a minimal Runner suitable for epilogue tests.
// cfg is applied as-is; defaults must be set by the caller if needed.
func newEpilogueTestRunner(t *testing.T, cfg *config.Config) *Runner {
	t.Helper()
	var buf bytes.Buffer
	r := &Runner{
		cfg:    cfg,
		output: &buf,
	}
	return r
}

// --- runTestFixLoop ---

func TestRunTestFixLoop_SkipsWhenTestCommandEmpty(t *testing.T) {
	// When cfg.Session.TestCommand == "", runTestFixLoop should return nil immediately.
	cfg := &config.Config{}
	cfg.Session.TestCommand = ""

	var cmdsCalled []string
	r := newEpilogueTestRunner(t, cfg)
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdsCalled = append(cmdsCalled, command)
		return "", "", 0, nil
	}

	err := r.runTestFixLoop(context.Background())
	if err != nil {
		t.Fatalf("runTestFixLoop() error = %v, want nil", err)
	}
	if len(cmdsCalled) != 0 {
		t.Errorf("runTestFixLoop() called commands = %v, want none", cmdsCalled)
	}
}

func TestRunTestFixLoop_SucceedsWhenTestCommandPasses(t *testing.T) {
	// When test command exits 0, runTestFixLoop returns nil and doesn't retry.
	cfg := &config.Config{}
	cfg.Session.TestCommand = "go test ./..."
	cfg.Session.MaxFixRetries = 3

	var cmdsCalled []string
	r := newEpilogueTestRunner(t, cfg)
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdsCalled = append(cmdsCalled, command)
		return "ok", "", 0, nil
	}

	err := r.runTestFixLoop(context.Background())
	if err != nil {
		t.Fatalf("runTestFixLoop() error = %v, want nil", err)
	}
	if len(cmdsCalled) != 1 {
		t.Errorf("runTestFixLoop() called commands %d times, want 1", len(cmdsCalled))
	}
}

// --- runSessionEpilogue ---

func TestRunSessionEpilogue_SkipsWhenIterationsZero(t *testing.T) {
	// When cfg.Session.Iterations == 0, epilogue should be a no-op.
	cfg := &config.Config{}
	cfg.Session.Iterations = 0

	r := newEpilogueTestRunner(t, cfg)
	st := &runLoopState{}

	ranRetro, err := r.runSessionEpilogue(context.Background(), st)
	if err != nil {
		t.Fatalf("runSessionEpilogue() error = %v, want nil", err)
	}
	if ranRetro {
		t.Error("runSessionEpilogue() ranRetro = true, want false when Iterations == 0")
	}
}
