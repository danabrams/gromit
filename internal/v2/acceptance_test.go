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

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/loop"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestSpecLoopExecutesCanonicalStageChain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "acceptance-spec"

	recorder := newRecordingStageRecorder()

	git := newRecordingGitAdapter(t)
	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         &fakeLLMAdapter{},
		TaskTracker: &fakeTaskTrackerAdapter{},
		Presenter:   &fakePresenterAdapter{},
	}

	cfg := &config.Config{}
	gate := newDependencyGate()

	specLoop, err := loop.NewSpecLoop(adapters, cfg, gate, loop.WithStageRecorder(recorder))
	if err != nil {
		t.Fatalf("NewSpecLoop error = %v", err)
	}

	if err := specLoop.Run(ctx, specID, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	requireCanonicalStageSequence(t, recorder)
	verifyPlanFileJustInTime(t, git.lastWorktree, specID, recorder)
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
	planPath := filepath.Join(worktree, "plan.md")
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

// fake adapters

type recordingGitAdapter struct {
	t            *testing.T
	lastWorktree string
}

var _ adapter.GitAdapter = (*recordingGitAdapter)(nil)

func newRecordingGitAdapter(t *testing.T) *recordingGitAdapter {
	return &recordingGitAdapter{t: t}
}

func (g *recordingGitAdapter) Checkout(_ context.Context, specID string) (string, error) {
	worktree := filepath.Join(g.t.TempDir(), specID)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		g.t.Fatalf("mkdir worktree: %v", err)
	}
	g.lastWorktree = worktree
	return worktree, nil
}
