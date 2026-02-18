package runner

import (
	"bytes"
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
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

func TestRunTestFixLoop_RetriesOnFailureAndCreatesBeadOnExhaustion(t *testing.T) {
	// When test command fails and retries are exhausted, a P0 bead with
	// "from-epilogue" label should be created.
	cfg := &config.Config{}
	cfg.Session.TestCommand = "go test ./..."
	cfg.Session.MaxFixRetries = 2
	cfg.Session.FixTier = "medium"

	// Track commands called
	var cmdsCalled []string
	r := newEpilogueTestRunner(t, cfg)
	r.cmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdsCalled = append(cmdsCalled, command)
		// Always fail
		return "FAIL", "error", 1, nil
	}

	// Set up mock renderer that returns a simple fix prompt
	r.renderer = &mockPromptRenderer{
		LoadClaudeMDFn: func() (string, error) { return "# project", nil },
		LoadRulesFn:    func() (string, error) { return "rules", nil },
		RenderTestFixFn: func(ctx *prompt.TestFixContext) (string, error) {
			return "fix prompt", nil
		},
	}

	// Set up mock router/provider
	r.router = newMockRouter()

	// Set up mock bead client to track created beads
	var createdBeads []struct {
		title    string
		priority int
		labels   []string
	}
	r.beads = &mockBeadClient{
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			createdBeads = append(createdBeads, struct {
				title    string
				priority int
				labels   []string
			}{title, priority, labels})
			return &bead.Bead{ID: "new-bead"}, nil
		},
	}

	err := r.runTestFixLoop(context.Background())
	if err != nil {
		t.Fatalf("runTestFixLoop() error = %v, want nil", err)
	}

	// Should have called test command: 1 initial + 2 retries = 3 times
	wantCmds := 3 // initial + MaxFixRetries
	if len(cmdsCalled) != wantCmds {
		t.Errorf("runTestFixLoop() test command called %d times, want %d", len(cmdsCalled), wantCmds)
	}

	// Should have created exactly 1 P0 bead with from-epilogue label
	if len(createdBeads) != 1 {
		t.Fatalf("runTestFixLoop() created %d beads, want 1", len(createdBeads))
	}
	if createdBeads[0].priority != 0 {
		t.Errorf("created bead priority = %d, want 0 (P0)", createdBeads[0].priority)
	}
	found := false
	for _, l := range createdBeads[0].labels {
		if l == "from-epilogue" {
			found = true
		}
	}
	if !found {
		t.Errorf("created bead labels = %v, want to contain 'from-epilogue'", createdBeads[0].labels)
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
