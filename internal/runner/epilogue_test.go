package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/state"
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

// --- runEpilogueRetro ---

func TestRunEpilogueRetro_SkipsWhenRouterNil(t *testing.T) {
	// When router is nil, runEpilogueRetro should return nil without panicking.
	cfg := &config.Config{}
	cfg.Session.Iterations = 1
	r := newEpilogueTestRunner(t, cfg)
	r.router = nil
	r.gromitDir = t.TempDir()

	err := r.runEpilogueRetro(context.Background())
	if err != nil {
		t.Fatalf("runEpilogueRetro() error = %v, want nil", err)
	}
}

func TestRunEpilogueRetro_RecordsInState(t *testing.T) {
	// When retro runs successfully, the state file should have LastRetro updated.
	dir := t.TempDir()
	// Create minimal .gromit structure needed for retro
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	// Write a minimal retro template
	retroTemplate := `Rules: {{.Rules}}
Learnings: {{.Learnings}}`
	if err := os.WriteFile(filepath.Join(dir, "templates", "PROMPT_retro.md"), []byte(retroTemplate), 0644); err != nil {
		t.Fatal(err)
	}
	// Write empty RULES.md
	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Session.Iterations = 1
	r := newEpilogueTestRunner(t, cfg)
	r.gromitDir = dir

	// Set up mock router that returns a provider returning valid retro output
	r.router = newMockRouter()

	// Wire a state file
	sf, _ := state.NewFile(dir)
	r.stateFile = sf

	err := r.runEpilogueRetro(context.Background())
	if err != nil {
		t.Fatalf("runEpilogueRetro() error = %v, want nil", err)
	}

	// Reload state and verify retro was recorded
	sf2, _ := state.NewFile(dir)
	if loadErr := sf2.Load(); loadErr != nil {
		t.Fatalf("loading state: %v", loadErr)
	}
	if sf2.LastRetro().IsZero() {
		t.Error("runEpilogueRetro() did not record retro in state (LastRetro is zero)")
	}
}

// --- runEpilogueReview ---

func TestRunEpilogueReview_SkipsWhenReviewerNil(t *testing.T) {
	// When reviewer is nil, runEpilogueReview should return nil without panicking.
	cfg := &config.Config{}
	r := newEpilogueTestRunner(t, cfg)
	r.reviewer = nil

	err := r.runEpilogueReview(context.Background(), &runLoopState{})
	if err != nil {
		t.Fatalf("runEpilogueReview() error = %v, want nil", err)
	}
}

// --- runSessionEpilogue ---

// --- finishRun wiring: epilogue called when session.iterations > 0 ---

func TestFinishRun_CallsEpilogueWhenSessionIterationsSet(t *testing.T) {
	// When cfg.Session.Iterations > 0, finishRun should call runSessionEpilogue.
	// We verify this by checking that runEpilogueRetro records in state when
	// Retro is enabled (default nil = true).
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	retroTemplate := `Rules: {{.Rules}} Learnings: {{.Learnings}}`
	if err := os.WriteFile(filepath.Join(dir, "templates", "PROMPT_retro.md"), []byte(retroTemplate), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Session.Iterations = 1

	sf, _ := state.NewFile(dir)

	r := newEpilogueTestRunner(t, cfg)
	r.gromitDir = dir
	r.router = newMockRouter()
	r.stateFile = sf

	st := &runLoopState{sf: sf}

	err := r.finishRun(context.Background(), st)
	if err != nil {
		t.Fatalf("finishRun() error = %v", err)
	}

	// Verify retro was recorded (epilogue ran retro)
	sf2, _ := state.NewFile(dir)
	if loadErr := sf2.Load(); loadErr != nil {
		t.Fatalf("loading state: %v", loadErr)
	}
	if sf2.LastRetro().IsZero() {
		t.Error("finishRun() did not run epilogue retro (LastRetro is zero)")
	}
}

const (
	finishRunCallPush = "push"
	finishRunCallEnd  = "end"
)

func newFinishRunCommandTestRunner(
	t *testing.T,
	endOfLoopCommand string,
	onPush func(),
	onEndCommand func(command string) (stdout string, stderr string, exitCode int, err error),
) *Runner {
	t.Helper()

	autoPush := true
	r := &Runner{
		cfg: &config.Config{
			Git: config.GitConfig{
				AutoPush:    &autoPush,
				PushFailure: "stop",
			},
			Loop: config.LoopConfig{
				EndOfLoopCommand: endOfLoopCommand,
			},
		},
		output: &bytes.Buffer{},
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			if program == "git" && slices.Equal(args, sessionCompletionPushArgv) && onPush != nil {
				onPush()
			}
			return "", "", 0, nil
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if onEndCommand == nil {
				return "", "", 0, nil
			}
			return onEndCommand(command)
		},
	}

	return r
}

func TestFinishRun_RunsEndOfLoopCommandAfterSessionCompletion(t *testing.T) {
	var calls []string

	r := newFinishRunCommandTestRunner(
		t,
		"echo done",
		func() {
			calls = append(calls, finishRunCallPush)
		},
		func(command string) (string, string, int, error) {
			calls = append(calls, finishRunCallEnd)
			if command != "echo done" {
				t.Fatalf("unexpected command: %q", command)
			}
			return "", "", 0, nil
		},
	)

	if err := r.finishRun(context.Background(), &runLoopState{}); err != nil {
		t.Fatalf("finishRun() error = %v", err)
	}

	pushIdx := slices.Index(calls, finishRunCallPush)
	endIdx := slices.Index(calls, finishRunCallEnd)
	if pushIdx < 0 {
		t.Fatalf("expected session completion git push call, got calls %v", calls)
	}
	if endIdx < 0 {
		t.Fatalf("expected end-of-loop command call, got calls %v", calls)
	}
	if endIdx < pushIdx {
		t.Fatalf("expected end-of-loop command after session completion, got calls %v", calls)
	}
}

func TestFinishRun_PropagatesEndOfLoopCommandFailure(t *testing.T) {
	pushCalled := false

	r := newFinishRunCommandTestRunner(
		t,
		"echo fail",
		func() {
			pushCalled = true
		},
		func(command string) (string, string, int, error) {
			return "", "boom", 9, nil
		},
	)

	err := r.finishRun(context.Background(), &runLoopState{})
	if err == nil {
		t.Fatal("finishRun() error = nil, want end-of-loop command error")
	}
	if !strings.Contains(err.Error(), "end-of-loop command failed") {
		t.Fatalf("finishRun() error = %v, want end-of-loop command failure", err)
	}
	if !pushCalled {
		t.Fatal("expected session completion to run before end-of-loop command failure")
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

func TestRunSessionEpilogue_SkipsRetroWhenRetroFalse(t *testing.T) {
	// When cfg.Session.Retro is explicitly false, the retro phase should not run.
	falseVal := false
	cfg := &config.Config{}
	cfg.Session.Iterations = 1
	cfg.Session.Retro = &falseVal

	r := newEpilogueTestRunner(t, cfg)
	// No router set — if retro tried to run it would panic on nil router
	st := &runLoopState{}

	ranRetro, err := r.runSessionEpilogue(context.Background(), st)
	if err != nil {
		t.Fatalf("runSessionEpilogue() error = %v, want nil", err)
	}
	if ranRetro {
		t.Error("runSessionEpilogue() ranRetro = true, want false when Session.Retro is false")
	}
}

func TestRunSessionEpilogue_RunsAllPhasesWhenEnabled(t *testing.T) {
	// When Iterations > 0 and retro is enabled, ranRetro should be true.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	retroTemplate := `Rules: {{.Rules}} Learnings: {{.Learnings}}`
	if err := os.WriteFile(filepath.Join(dir, "templates", "PROMPT_retro.md"), []byte(retroTemplate), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	trueVal := true
	cfg := &config.Config{}
	cfg.Session.Iterations = 1
	cfg.Session.Review = &trueVal
	cfg.Session.Retro = &trueVal

	r := newEpilogueTestRunner(t, cfg)
	r.gromitDir = dir
	r.router = newMockRouter()

	sf, _ := state.NewFile(dir)
	r.stateFile = sf

	st := &runLoopState{}
	ranRetro, err := r.runSessionEpilogue(context.Background(), st)
	if err != nil {
		t.Fatalf("runSessionEpilogue() error = %v, want nil", err)
	}
	if !ranRetro {
		t.Error("runSessionEpilogue() ranRetro = false, want true when retro is enabled")
	}
}
