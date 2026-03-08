//go:build integration

package loop

import (
	"context"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/pipeline"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
)

func TestIntegration_CommitPerStageFlow_RealGitLoopHistory(t *testing.T) {
	t.Parallel()

	repoRoot := initLoopCommitHistoryRepo(t)
	worktreesDir := t.TempDir()
	specID := "integration-commit-per-stage"

	gitAdapter := newImmutableGitAdapter(repoRoot, worktreesDir)
	stageCommitter := &pipeline.StageCommitter{Git: gitAdapter}

	typedEmitter := event.NewEmitter()
	t.Cleanup(func() {
		typedEmitter.Close()
	})

	beadLoop, err := NewBeadLoop(BeadLoopConfig{
		Gate:           newNoopStage("gate"),
		Build:          &sleepingNoopStage{name: "build", delay: 25 * time.Millisecond},
		Validate:       &sleepingNoopStage{name: "validate", delay: 25 * time.Millisecond},
		Review:         &sleepingNoopStage{name: "review", delay: 25 * time.Millisecond},
		Epilogue:       newNoopStage("epilogue"),
		Emitter:        typedEmitter,
		StageCommitter: stageCommitter,
	})
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         gitAdapter,
			LLM:         newFakeLLMAdapter(),
			TaskTracker: newFakeTaskTrackerAdapter(),
			Presenter:   newFakePresenterAdapter(t),
		},
		&config.Config{},
		noopDependencyGate{},
		WithTypedEmitter(typedEmitter),
		WithStageCommitter(stageCommitter),
		WithPlanStage(&nonWritingPlanStage{plan: "integration plan"}),
		WithDecomposeStage(&sleepingDecomposeStage{
			delay: 25 * time.Millisecond,
			beads: []*bead.Bead{
				{ID: "bead-001", Title: "bead one"},
			},
		}),
		WithBeadLoop(beadLoop),
		WithAcceptStage(&sleepingAcceptStage{delay: 25 * time.Millisecond}),
		WithPresentStage(&sleepingPresentStage{delay: 25 * time.Millisecond}, &present.SummaryContext{}),
	)
	if err != nil {
		t.Fatalf("NewSpecLoop: %v", err)
	}

	if err := loopInstance.Run(context.Background(), specID, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := gitLogSubjects(t, repoRoot, "gromit/spec/"+specID, 10)
	want := []string{
		"[bead:spec/present/iter:1] proceed",
		"[bead:spec/accept/iter:1] proceed",
		"[bead:bead-001/review/iter:1] proceed",
		"[bead:bead-001/validate/iter:1] proceed",
		"[bead:bead-001/build/iter:1] proceed",
		"[bead:spec/decompose/iter:1] proceed",
		"[bead:spec/plan/iter:1] proceed",
		"initial",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("git log subjects mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func initLoopCommitHistoryRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		runGitInDir(t, repoRoot, args...)
	}
	return repoRoot
}

func gitLogSubjects(t *testing.T, repoRoot, ref string, n int) []string {
	t.Helper()
	out := runGitInDir(t, repoRoot, "log", "--format=%s", "-n", "10", ref)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return lines
}

func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

type sleepingNoopStage struct {
	name  string
	delay time.Duration
}

func (s *sleepingNoopStage) Name() string { return s.name }

func (s *sleepingNoopStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

type sleepingDecomposeStage struct {
	delay time.Duration
	beads []*bead.Bead
}

func (s *sleepingDecomposeStage) Name() string { return "decompose" }

func (s *sleepingDecomposeStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)
	return &stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &stagepkg.DecomposeArtifacts{Beads: append([]*bead.Bead(nil), s.beads...)},
	}, nil
}

type sleepingAcceptStage struct {
	delay time.Duration
}

func (s *sleepingAcceptStage) Name() string { return "accept" }

func (s *sleepingAcceptStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

type sleepingPresentStage struct {
	delay time.Duration
}

func (s *sleepingPresentStage) Name() string { return "present" }

func (s *sleepingPresentStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	time.Sleep(s.delay)
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

var _ stagepkg.Stage = (*sleepingNoopStage)(nil)
var _ stagepkg.Stage = (*sleepingDecomposeStage)(nil)
var _ stagepkg.Stage = (*sleepingAcceptStage)(nil)
var _ stagepkg.Stage = (*sleepingPresentStage)(nil)
var _ stagepkg.Stage = (*nonWritingPlanStage)(nil)
