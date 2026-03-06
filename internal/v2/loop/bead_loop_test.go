package loop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRecordsStartGeneration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	// Request with a bead that has gen:5 label
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:5"},
		},
	}

	result, err := beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	if result == nil {
		t.Fatalf("result is nil")
	}

	if result.StartGeneration != 5 {
		t.Fatalf("StartGeneration = %d, want 5", result.StartGeneration)
	}
}

// successStage is a test stage that always succeeds
func TestBeadLoopHasDefaultGenerationCap(t *testing.T) {
	t.Parallel()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	// Default generation cap should be 3
	if beadLoop.GenerationCap != 3 {
		t.Fatalf("GenerationCap = %d, want 3", beadLoop.GenerationCap)
	}
}

func TestBeadLoopCanSetCustomGenerationCap(t *testing.T) {
	t.Parallel()

	successStage := &successStage{
		name: "test",
	}

	beadLoop, err := NewBeadLoop([]StageSpec{
		{
			Stage: successStage,
			Retry: stage.RetryConfig{MaxRetries: 0},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	beadLoop.GenerationCap = 5

	if beadLoop.GenerationCap != 5 {
		t.Fatalf("GenerationCap = %d, want 5", beadLoop.GenerationCap)
	}
}

// successStage is a test stage that always succeeds
type successStage struct {
	name string
}

func (s *successStage) Name() string {
	return s.name
}

func (s *successStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}
