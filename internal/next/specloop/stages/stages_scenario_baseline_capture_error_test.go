package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_BaselineCaptureError_RunProceedsWithEmptyBaseline(t *testing.T) {
	// Seed
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)
	rs := runstore.NewRunState("spec-scenario-baseline-error", "project-scenario")
	if err := store.Save(rs); err != nil {
		t.Fatalf("seed save run state: %v", err)
	}

	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	planRan := false
	loop := specloop.NewSpecLoop([]specloop.Stage{
		&baselineCaptureErrorInitStage{eventLog: eventLog},
		&scenarioPlanStage{ran: &planRan},
	}, specloop.SpecLoopConfig{EventLog: eventLog})

	// Invoke
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("spec loop run: %v", err)
	}

	// Assert
	if len(rs.BaselineFailures) != 0 {
		t.Fatalf("BaselineFailures len = %d, want 0", len(rs.BaselineFailures))
	}
	if !planRan {
		t.Fatal("expected run to continue to planning stage")
	}

	rawEvents, err := os.ReadFile(eventLogPath)
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if !strings.Contains(string(rawEvents), `"type":"baseline_capture_error"`) {
		t.Fatal("expected baseline_capture_error event type in event log")
	}
	if !strings.Contains(string(rawEvents), "baseline capture failed: worktree not ready") {
		t.Fatal("expected concrete baseline capture error message in event log")
	}
}

type baselineCaptureErrorInitStage struct {
	eventLog *runstore.EventLog
}

func (s *baselineCaptureErrorInitStage) Name() string { return "init" }

func (s *baselineCaptureErrorInitStage) Run(_ context.Context, _ *runstore.RunState) (specloop.NextAction, error) {
	if s.eventLog != nil {
		s.eventLog.Append(runstore.BaselineCaptureErrorEvent{
			BaseEvent: runstore.BaseEvent{Type: "baseline_capture_error", Timestamp: time.Now()},
			Error:     "baseline capture failed: worktree not ready",
		})
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

type scenarioPlanStage struct {
	ran *bool
}

func (s *scenarioPlanStage) Name() string { return "plan" }

func (s *scenarioPlanStage) Run(_ context.Context, _ *runstore.RunState) (specloop.NextAction, error) {
	if s.ran != nil {
		*s.ran = true
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}
