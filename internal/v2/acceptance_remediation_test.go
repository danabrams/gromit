//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRemediationCycleTriggersGapAnalysis(t *testing.T) {
	t.Parallel()

	order := []string{}
	accept := &flakyAcceptStage{order: &order}
	gap := &recordingStage{name: "gap-analysis", order: &order}
	decompose := &recordingDecomposeStage{
		order: &order,
		beads: []*bead.Bead{{ID: "missing-123"}},
	}
	beadLoop := &fakeBeadRunner{order: &order}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		GapStage:       gap,
		DecomposeStage: decompose,
		BeadRunner:     beadLoop,
		GenerationCap:  3,
	})

	if err := runner.Run(context.Background(), "spec-remediate"); err != nil {
		t.Fatalf("remediation run failed: %v", err)
	}

	want := []string{"accept", "gap-analysis", "decompose", "bead-loop", "accept"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
}

func TestGenerationCapStopsRemediation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	emitter := events.NewEmitter()
	ch := emitter.Subscribe()
	t.Cleanup(func() {
		emitter.Unsubscribe(ch)
	})

	accept := &alwaysFailingStage{}
	gap := &recordingStage{name: "gap-analysis"}
	decompose := &recordingDecomposeStage{beads: []*bead.Bead{{ID: "unused"}}}
	beadLoop := &fakeBeadRunner{}
	presenter := &spyPresenter{}
	cleaner := &spyWorktreeCleaner{}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:     accept,
		GapStage:        gap,
		DecomposeStage:  decompose,
		BeadRunner:      beadLoop,
		GenerationCap:   0,
		Presenter:       presenter,
		Emitter:         emitter,
		WorktreeCleaner: cleaner,
	})

	if err := runner.Run(ctx, "spec-cap"); err == nil {
		t.Fatal("expected remediation to fail when cap reached")
	}

	if gap.calls != 0 {
		t.Fatalf("gap called = %d, want 0", gap.calls)
	}
	if decompose.calls != 0 {
		t.Fatalf("decompose called = %d, want 0", decompose.calls)
	}
	if beadLoop.runs != 0 {
		t.Fatalf("bead loop runs = %d, want 0", beadLoop.runs)
	}

	eventsSeen := map[string]bool{}
	deadline := time.After(time.Second)
	for len(eventsSeen) < 2 {
		select {
		case evt := <-ch:
			switch evt.(type) {
			case *events.GenerationCapReachedEvent:
				eventsSeen["generationCap"] = true
			case *events.AndonTriggeredEvent:
				eventsSeen["andon"] = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for events, seen=%v", eventsSeen)
		}
	}

	if !presenter.called || presenter.lastSpec != "spec-cap" {
		t.Fatalf("unexpected presenter calls: %#v", presenter)
	}
	if presenter.lastSummary == "" {
		t.Fatal("expected presenter to display failure summary")
	}
	if cleaner.calls != 0 {
		t.Fatalf("worktree cleaner calls = %d, want 0", cleaner.calls)
	}
}

// Helpers used in remediation acceptance tests.
type flakyAcceptStage struct {
	order    *[]string
	attempts int
}

type alwaysFailingStage struct{}

func (f *flakyAcceptStage) Name() string { return "accept" }

func (f *flakyAcceptStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	f.attempts++
	if f.order != nil {
		*f.order = append(*f.order, "accept")
	}
	if f.attempts == 1 {
		return &stage.Result{Decision: stage.DecisionFail}, nil
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

func (alwaysFailingStage) Name() string { return "accept" }

func (alwaysFailingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionFail}, nil
}

var _ stage.Stage = (*flakyAcceptStage)(nil)
var _ stage.Stage = (*alwaysFailingStage)(nil)

type recordingStage struct {
	name  string
	order *[]string
	calls int
}

func (r *recordingStage) Name() string { return r.name }

func (r *recordingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	r.calls++
	if r.order != nil {
		*r.order = append(*r.order, r.name)
	}
	return nil, nil
}

var _ stage.Stage = (*recordingStage)(nil)

type recordingDecomposeStage struct {
	order *[]string
	beads []*bead.Bead
	calls int
}

func (r *recordingDecomposeStage) Name() string { return "decompose" }

func (r *recordingDecomposeStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	r.calls++
	if r.order != nil {
		*r.order = append(*r.order, "decompose")
	}
	return &stage.Result{Artifacts: &DecomposeArtifacts{Beads: r.beads}}, nil
}

var _ stage.Stage = (*recordingDecomposeStage)(nil)

type fakeBeadRunner struct {
	order *[]string
	runs  int
}

func (f *fakeBeadRunner) Run(ctx context.Context, beads []*bead.Bead) error {
	f.runs++
	if f.order != nil {
		*f.order = append(*f.order, "bead-loop")
	}
	return nil
}

type spyPresenter struct {
	called      bool
	lastSpec    string
	lastSummary string
}

func (s *spyPresenter) PresentSummary(ctx context.Context, specID, summary string) error {
	s.called = true
	s.lastSpec = specID
	s.lastSummary = summary
	return nil
}

type spyWorktreeCleaner struct {
	calls int
}

func (s *spyWorktreeCleaner) Cleanup(ctx context.Context, specID string) error {
	s.calls++
	return nil
}
