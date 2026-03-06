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

func TestGenerationCapReachedMarksCapHit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Start at generation 5, cap of 3 means generation 8 would be capped
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

	beadLoop.GenerationCap = 3

	// Request with generation 5, cap of 3, so start + cap = 8
	// Current bead is generation 5, which is <= start + cap, so cap not hit yet
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

	// Generation 5 is not >= start (5) + cap (3), so CapHit should be false
	if result.CapHit {
		t.Fatalf("CapHit = true, want false for generation 5 with start=5, cap=3")
	}
}

func TestCapHitDetectsWhenGenerationThresholdExceeded(t *testing.T) {
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

	beadLoop.GenerationCap = 3

	// Process a bead at generation 0
	req := stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead",
			Labels: []string{"gen:0"},
		},
	}

	result, err := beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// StartGeneration = 0, cap = 3, so threshold = 3
	// Current generation 0 < 3, so CapHit should be false
	if result.CapHit {
		t.Fatalf("CapHit = true, want false for generation 0 with cap 3")
	}

	// Process a bead at generation 3 (at threshold: start=0, cap=3, threshold=3)
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead-2",
			Labels: []string{"gen:3"},
		},
	}

	result, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// StartGeneration = 0 (from first run), cap = 3, threshold = 3
	// Current generation 3 >= 3, so CapHit should be true
	if !result.CapHit {
		t.Fatalf("CapHit = false, want true for generation 3 with start=0, cap=3")
	}

	// Process a bead at generation 2 (below threshold)
	req = stage.Request{
		Bead: stage.BeadInfo{
			ID:     "test-bead-3",
			Labels: []string{"gen:2"},
		},
	}

	result, err = beadLoop.Run(ctx, req)
	if err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	// StartGeneration = 0, cap = 3, threshold = 3
	// Current generation 2 < 3, so CapHit should be false
	if result.CapHit {
		t.Fatalf("CapHit = true, want false for generation 2")
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

// capHitTestStage is a test stage that creates beads at a specific generation
type capHitTestStage struct {
	name           string
	nextGeneration int
}

func (s *capHitTestStage) Name() string {
	return s.name
}

func (s *capHitTestStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	// This stage would normally enqueue new beads at nextGeneration
	// For now, just return success
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}
