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
