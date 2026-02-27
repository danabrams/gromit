package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestPipeline_Board_ReturnsSortedOpenBeads(t *testing.T) {
	t.Parallel()

	origClient := newBoardClient
	t.Cleanup(func() { newBoardClient = origClient })

	called := false
	newBoardClient = func() (boardClient, error) {
		return &fakeBoardClient{
			listAllFn: func(ctx context.Context) ([]*bead.Bead, []*bead.Bead, error) {
				called = true
				return []*bead.Bead{
						{ID: "second-open", Priority: 2},
						{ID: "first-open", Priority: 0},
					}, []*bead.Bead{
						{ID: "closed", Priority: 1},
					}, nil
			},
		}, nil
	}

	p := New(&Deps{}, &Paths{})
	result, err := p.Board(context.Background())
	if err != nil {
		t.Fatalf("Board() returned error: %v", err)
	}
	if !called {
		t.Fatal("stub ListAll was not called")
	}
	if len(result.Open) != 2 {
		t.Fatalf("expected 2 open beads, got %d", len(result.Open))
	}
	if result.Open[0].ID != "first-open" {
		t.Fatalf("expected first-open first, got %s", result.Open[0].ID)
	}
	if len(result.Closed) != 1 || result.Closed[0].ID != "closed" {
		t.Fatalf("unexpected closed beads: %+v", result.Closed)
	}
}

func TestPipeline_Board_ClientCreationError(t *testing.T) {
	t.Parallel()

	origClient := newBoardClient
	t.Cleanup(func() { newBoardClient = origClient })

	newBoardClient = func() (boardClient, error) {
		return nil, errBoom
	}

	p := New(&Deps{}, &Paths{})
	if _, err := p.Board(context.Background()); err == nil {
		t.Fatal("expected error when bead client creation fails")
	} else if !strings.Contains(err.Error(), "creating bead client") {
		t.Fatalf("unexpected error: %v", err)
	}
}

var errBoom = errors.New("boom")

type fakeBoardClient struct {
	listAllFn func(context.Context) ([]*bead.Bead, []*bead.Bead, error)
}

func (f *fakeBoardClient) ListAll(ctx context.Context) ([]*bead.Bead, []*bead.Bead, error) {
	if f.listAllFn == nil {
		return nil, nil, nil
	}
	return f.listAllFn(ctx)
}
