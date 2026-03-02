package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
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
