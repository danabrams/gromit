//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/loop"
	"github.com/danabrams/gromit/internal/v2/presentation"
	"github.com/danabrams/gromit/internal/v2/stage"
	specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
)

func TestSpecLoopExecutesCanonicalStageChain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "acceptance-spec"

	recorder := newRecordingStageRecorder()

	fixture := newAcceptanceAdapters(t)
	adapters := fixture.AdapterSet()

	cfg := &config.Config{}
	gate := newDependencyGate()

	planStage := newAcceptancePlanStage(specID + "-plan")
	presentStage, summaryCtx := newAcceptancePresentStage()

	specLoop, err := loop.NewSpecLoop(
		adapters,
		cfg,
		gate,
		loop.WithStageRecorder(recorder),
		loop.WithPlanStage(planStage),
		loop.WithPresentStage(presentStage, summaryCtx),
		loop.WithDecomposeStage(newFakeDecomposeStage(specID)),
		loop.WithBeadLoop(newFakeBeadRunner()),
		loop.WithAcceptStage(newFakeAcceptStage()),
		loop.WithSpecReviewStage(acceptanceSpecReviewStage{}),
	)
	if err != nil {
		t.Fatalf("NewSpecLoop error = %v", err)
	}

	if err := specLoop.Run(ctx, specID, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	requireCanonicalStageSequence(t, recorder)
	verifyPlanFileJustInTime(t, fixture.Worktree(specID), specID, recorder)
}

func TestSpecLoopRunsWithStages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "acceptance-stage"

	recorder := newRecordingStageRecorder()

	fixture := newAcceptanceAdapters(t)
	adapters := fixture.AdapterSet()

	cfg := &config.Config{}
	gate := newDependencyGate()

	planStage := newAcceptancePlanStage(specID + "-plan")
	presentStage, summaryCtx := newAcceptancePresentStage()

	specLoop, err := loop.NewSpecLoop(
		adapters,
		cfg,
		gate,
		loop.WithStageRecorder(recorder),
		loop.WithPlanStage(planStage),
		loop.WithPresentStage(presentStage, summaryCtx),
		loop.WithDecomposeStage(newFakeDecomposeStage(specID)),
		loop.WithBeadLoop(newFakeBeadRunner()),
		loop.WithAcceptStage(newFakeAcceptStage()),
	)
	if err != nil {
		t.Fatalf("NewSpecLoop error = %v", err)
	}

	if err := specLoop.Run(ctx, specID, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestStageResultCarriesDecisionArtifactsEvents(t *testing.T) {
	t.Parallel()

	evt := sampleStageEvent("plan", "spec-acceptance")
	res := newStageResult(evt)

	if res.Decision != stage.DecisionProceed {
		t.Fatalf("decision = %v", res.Decision)
	}
	if _, ok := res.Artifacts.(map[string]string); !ok {
		t.Fatalf("expected artifacts to survive type assertion")
	}
	if len(res.Events) != 1 {
		t.Fatalf("events preserved = %v", res.Events)
	}

	got := res.Events[0]
	if got.EventType() != evt.EventType() {
		t.Fatalf("event types differ: got %s want %s", got.EventType(), evt.EventType())
	}
}

// --- helpers ---

type fixtureStageRecorder struct {
	mu      sync.Mutex
	records []stageRecord
}

type stageRecord struct {
	name string
	time time.Time
}

func newRecordingStageRecorder() *fixtureStageRecorder {
	return &fixtureStageRecorder{}
}

func (r *fixtureStageRecorder) RecordStage(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, stageRecord{name: name, time: time.Now()})
}

func (r *fixtureStageRecorder) stageNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.records))
	for _, rec := range r.records {
		names = append(names, rec.name)
	}
	return names
}

func (r *fixtureStageRecorder) stageTime(name string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.records {
		if rec.name == name {
			return rec.time
		}
	}
	return time.Time{}
}

func requireCanonicalStageSequence(t *testing.T, recorder *fixtureStageRecorder) {
	t.Helper()
	if got := recorder.stageNames(); !reflect.DeepEqual(got, loop.StageSequence) {
		t.Fatalf("stage order = %v, want %v", got, loop.StageSequence)
	}
}

func verifyPlanFileJustInTime(t *testing.T, worktree, specID string, recorder *fixtureStageRecorder) {
	t.Helper()
	planPath := filepath.Join(worktree, ".gromit", "v2", "plan.md")
	info, err := os.Stat(planPath)
	if err != nil {
		t.Fatalf("stat plan file: %v", err)
	}

	planStart := recorder.stageTime("plan")
	if planStart.IsZero() {
		t.Fatalf("no plan stage recorded")
	}
	if info.ModTime().Before(planStart) {
		t.Fatalf("plan file mod time %v should not be before plan stage start %v", info.ModTime(), planStart)
	}

	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan file: %v", err)
	}
	if got := string(planData); got != specID+"-plan" {
		t.Fatalf("plan contents = %q, want %q", got, specID+"-plan")
	}
}

func sampleStageEvent(stageName, beadID string) event.TypedEvent {
	return &event.StageStartedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeStageStarted,
		},
		StageName: stageName,
		BeadID:    beadID,
		Iteration: 1,
	}
}

func newStageResult(evt event.TypedEvent) stage.Result {
	return stage.Result{
		Decision: stage.DecisionProceed,
		Artifacts: map[string]string{
			"phase": "acceptance",
		},
		Events: []event.TypedEvent{evt},
	}
}

type fakeDecomposeStage struct {
	producedBeads []*bead.Bead
}

func newFakeDecomposeStage(specID string) *fakeDecomposeStage {
	return &fakeDecomposeStage{
		producedBeads: []*bead.Bead{
			{ID: specID + "-bead"},
		},
	}
}

func (f *fakeDecomposeStage) Name() string { return "decompose" }

func (f *fakeDecomposeStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{
		Decision:  stage.DecisionProceed,
		Artifacts: &stage.DecomposeArtifacts{Beads: append([]*bead.Bead(nil), f.producedBeads...)},
	}, nil
}

type acceptanceFakeBeadRunner struct{}

func newFakeBeadRunner() *acceptanceFakeBeadRunner {
	return &acceptanceFakeBeadRunner{}
}

func (f *acceptanceFakeBeadRunner) Run(ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) (loop.BeadLoopResult, error) {
	_ = ctx
	_ = beads
	_ = stopCh
	return loop.BeadLoopResult{}, nil
}

type fakeAcceptStage struct {
	results []presentation.AcceptanceResult
}

func newFakeAcceptStage() *fakeAcceptStage {
	return &fakeAcceptStage{
		results: []presentation.AcceptanceResult{
			{Title: "criterion", Description: "desc"},
		},
	}
}

func (f *fakeAcceptStage) Name() string { return "accept" }

func (f *fakeAcceptStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{
		Decision: stage.DecisionProceed,
		Artifacts: &stageaccept.AcceptArtifacts{
			Results: append([]presentation.AcceptanceResult(nil), f.results...),
		},
	}, nil
}

type acceptanceSpecReviewStage struct{}

func (acceptanceSpecReviewStage) Name() string { return "spec-review" }

func (acceptanceSpecReviewStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{
		Decision: stage.DecisionProceed,
		Artifacts: &specreview.SpecReviewArtifacts{
			Verdict: "pass",
		},
	}, nil
}

var _ stage.Stage = (*acceptanceSpecReviewStage)(nil)
