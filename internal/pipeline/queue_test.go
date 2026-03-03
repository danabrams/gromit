package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
)

func TestPipeline_Queue_ClientCreationError(t *testing.T) {
	t.Parallel()

	deps := &Deps{
		QueueClient: &fakeQueueClient{
			listReadyWorkFn: func(context.Context) ([]*bead.Bead, error) {
				return nil, queueErrBoom
			},
			listFn: func(context.Context) ([]*bead.Bead, error) {
				return nil, nil
			},
			listByStatusFn: func(context.Context, string) ([]*bead.Bead, error) {
				return nil, nil
			},
		},
	}

	p := New(deps, &Paths{})
	if _, err := p.Queue(context.Background(), QueueInput{}); err == nil {
		t.Fatal("expected error when bead client creation fails")
	} else if !errors.Is(err, queueErrBoom) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPipeline_Queue_UsesDepsQueueClient(t *testing.T) {
	t.Parallel()

	called := false
	deps := &Deps{
		QueueClient: &fakeQueueClient{
			listReadyWorkFn: func(context.Context) ([]*bead.Bead, error) {
				called = true
				return nil, nil
			},
			listFn: func(context.Context) ([]*bead.Bead, error) {
				return []*bead.Bead{}, nil
			},
			listByStatusFn: func(context.Context, string) ([]*bead.Bead, error) {
				return []*bead.Bead{}, nil
			},
		},
	}

	p := New(deps, &Paths{})
	if _, err := p.Queue(context.Background(), QueueInput{}); err != nil {
		t.Fatalf("Queue() returned error: %v", err)
	}
	if !called {
		t.Fatal("expected deps queue client to be used")
	}
}

func TestPipeline_Queue_ReturnsPartitionedData(t *testing.T) {
	t.Parallel()

	deps := &Deps{
		QueueClient: &fakeQueueClient{
			listReadyWorkFn: func(context.Context) ([]*bead.Bead, error) {
				return []*bead.Bead{
					{ID: "ready", Priority: 0},
				}, nil
			},
			listFn: func(context.Context) ([]*bead.Bead, error) {
				return []*bead.Bead{
					{ID: "ready", Priority: 0},
					{ID: "in-progress", Priority: 1, Status: "in_progress"},
				}, nil
			},
			listByStatusFn: func(context.Context, string) ([]*bead.Bead, error) {
				return []*bead.Bead{
					{ID: "in-progress", Priority: 1, Status: "in_progress"},
				}, nil
			},
		},
	}

	p := New(deps, &Paths{})
	result, err := p.Queue(context.Background(), QueueInput{LogsDir: "nonexistent", StuckThreshold: 1})
	if err != nil {
		t.Fatalf("Queue() returned error: %v", err)
	}
	if len(result.Ready) != 1 || result.Ready[0].ID != "ready" {
		t.Fatalf("unexpected ready beads: %+v", result.Ready)
	}
	if len(result.Blocked) != 0 {
		t.Fatalf("expected no blocked beads, got %+v", result.Blocked)
	}
	if len(result.Stuck) != 0 {
		t.Fatalf("expected no stuck beads, got %+v", result.Stuck)
	}
}

func TestPipeline_Queue_RespectsRestartPoints(t *testing.T) {
	t.Parallel()

	logsDir := t.TempDir()
	gromitDir := t.TempDir()
	beadID := "ready-after-restart"
	writeFailureLogEntry(t, logsDir, beadID, time.Now().Add(-time.Minute))
	if err := writeManualRestartPoint(gromitDir, beadID); err != nil {
		t.Fatalf("writeManualRestartPoint() error: %v", err)
	}

	deps := &Deps{
		QueueClient: &fakeQueueClient{
			listReadyWorkFn: func(context.Context) ([]*bead.Bead, error) {
				return []*bead.Bead{{ID: beadID, Priority: 0}}, nil
			},
			listFn: func(context.Context) ([]*bead.Bead, error) {
				return []*bead.Bead{{ID: beadID, Priority: 0}}, nil
			},
			listByStatusFn: func(context.Context, string) ([]*bead.Bead, error) {
				return []*bead.Bead{}, nil
			},
		},
	}

	p := New(deps, &Paths{})
	result, err := p.Queue(context.Background(), QueueInput{LogsDir: logsDir, StuckThreshold: 1, GromitDir: gromitDir})
	if err != nil {
		t.Fatalf("Queue() error: %v", err)
	}

	if len(result.Stuck) != 0 {
		t.Fatalf("stuck beads present, want none: %+v", result.Stuck)
	}
	if len(result.Ready) != 1 || result.Ready[0].ID != beadID {
		t.Fatalf("ready beads = %+v, want [ready-after-restart]", result.Ready)
	}
}

func writeFailureLogEntry(t testing.TB, logsDir, beadID string, ts time.Time) {
	t.Helper()
	entry := logger.IterationLog{
		Timestamp: ts,
		Iteration: 1,
		BeadID:    beadID,
		BeadTitle: "Ready After Restart",
		Model:     "test",
		Success:   false,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	path := filepath.Join(logsDir, "run-1.jsonl")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}
}

var queueErrBoom = errors.New("queue boom")

type fakeQueueClient struct {
	listReadyWorkFn func(context.Context) ([]*bead.Bead, error)
	listFn          func(context.Context) ([]*bead.Bead, error)
	listByStatusFn  func(context.Context, string) ([]*bead.Bead, error)
}

func (f *fakeQueueClient) ListReadyWork(ctx context.Context) ([]*bead.Bead, error) {
	if f.listReadyWorkFn == nil {
		return nil, nil
	}
	return f.listReadyWorkFn(ctx)
}

func (f *fakeQueueClient) List(ctx context.Context) ([]*bead.Bead, error) {
	if f.listFn == nil {
		return nil, nil
	}
	return f.listFn(ctx)
}

func (f *fakeQueueClient) ListByStatus(ctx context.Context, status string) ([]*bead.Bead, error) {
	if f.listByStatusFn == nil {
		return nil, nil
	}
	return f.listByStatusFn(ctx, status)
}
