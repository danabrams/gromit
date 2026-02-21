package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

// --- fakes for orchestrator tests ---

// orchStageFunc wraps a function as a pipeline.Stage.
type orchStageFunc struct {
	fn func(ctx context.Context, in pipeline.Input) (pipeline.Output, error)
}

func (s *orchStageFunc) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if s.fn != nil {
		return s.fn(ctx, in)
	}
	return pipeline.Output{Decision: pipeline.Proceed}, nil
}

// orchProceedStage returns a stage that always returns Proceed.
func orchProceedStage() pipeline.Stage {
	return &orchStageFunc{}
}

// orchBeadQueue builds a GetBead function from a static list.
func orchBeadQueue(beads ...*bead.Bead) func(context.Context) (*bead.Bead, error) {
	idx := 0
	return func(_ context.Context) (*bead.Bead, error) {
		if idx >= len(beads) {
			return nil, nil
		}
		b := beads[idx]
		idx++
		return b, nil
	}
}

// orchTestBead returns a minimal bead for testing.
func orchTestBead(id string) *bead.Bead {
	return &bead.Bead{
		ID:              id,
		Title:           "Test bead " + id,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}
}

// --- orchestrator unit tests ---

// TestOrchestrator_IterationMonotonicallyIncreasesForBlockedBeads verifies that the
// iteration counter increases for every bead processed, including those blocked at Gate.
func TestOrchestrator_IterationMonotonicallyIncreasesForBlockedBeads(t *testing.T) {
	var gateIterations []int
	gate := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		gateIterations = append(gateIterations, in.Iteration)
		if in.Bead.ID == "bead-1" {
			return pipeline.Output{Decision: pipeline.Block}, nil
		}
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     gate,
		Build:    orchProceedStage(),
		Validate: orchProceedStage(),
		Epilogue: orchProceedStage(),
		GetBead:  orchBeadQueue(orchTestBead("bead-1"), orchTestBead("bead-2")),
		Config:   &config.Config{},
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if len(gateIterations) != 2 {
		t.Fatalf("Expected Gate to be called 2 times, got %d: %v", len(gateIterations), gateIterations)
	}
	if gateIterations[0] >= gateIterations[1] {
		t.Errorf("Iteration must increase monotonically: got %v", gateIterations)
	}
}

// TestOrchestrator_SkippedBeadGetsIncrementedIteration verifies that a bead that
// returns Skip at the Gate stage still increments the iteration counter.
func TestOrchestrator_SkippedBeadGetsIncrementedIteration(t *testing.T) {
	var gateIterations []int
	gate := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		gateIterations = append(gateIterations, in.Iteration)
		if in.Bead.ID == "bead-1" {
			return pipeline.Output{Decision: pipeline.Skip}, nil
		}
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     gate,
		Build:    orchProceedStage(),
		Validate: orchProceedStage(),
		Epilogue: orchProceedStage(),
		GetBead:  orchBeadQueue(orchTestBead("bead-1"), orchTestBead("bead-2")),
		Config:   &config.Config{},
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if len(gateIterations) != 2 {
		t.Fatalf("Expected Gate to be called 2 times, got %d: %v", len(gateIterations), gateIterations)
	}
	if gateIterations[0] >= gateIterations[1] {
		t.Errorf("Iteration must increase monotonically for skipped beads: got %v", gateIterations)
	}
}

// TestOrchestrator_ValidationFailuresFedToNextBuildInput verifies that when Validate
// returns Block with failure summaries, those summaries are included in the next
// Build stage's Input.ValidationFailures.
func TestOrchestrator_ValidationFailuresFedToNextBuildInput(t *testing.T) {
	var buildInputs []pipeline.Input
	build := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildInputs = append(buildInputs, in)
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	validateCallCount := 0
	validate := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		validateCallCount++
		if validateCallCount == 1 {
			return pipeline.Output{
				Decision:           pipeline.Block,
				ValidationFailures: []string{"test: missing function foo"},
			}, nil
		}
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     orchProceedStage(),
		Build:    build,
		Validate: validate,
		Epilogue: orchProceedStage(),
		GetBead:  orchBeadQueue(orchTestBead("bead-1"), orchTestBead("bead-2")),
		Config:   &config.Config{},
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if len(buildInputs) < 2 {
		t.Fatalf("Expected Build to be called at least twice, got %d", len(buildInputs))
	}
	if len(buildInputs[1].ValidationFailures) == 0 {
		t.Error("Expected second Build to receive ValidationFailures from failed Validate")
	}
	if len(buildInputs[1].ValidationFailures) > 0 && buildInputs[1].ValidationFailures[0] != "test: missing function foo" {
		t.Errorf("Unexpected ValidationFailures: %v", buildInputs[1].ValidationFailures)
	}
}

// TestOrchestrator_ValidationFailuresClearedOnSuccess verifies that validation failures
// are cleared after a successful validate, not carried into subsequent iterations.
func TestOrchestrator_ValidationFailuresClearedOnSuccess(t *testing.T) {
	var buildInputs []pipeline.Input
	build := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		buildInputs = append(buildInputs, in)
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	validateCallCount := 0
	validate := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		validateCallCount++
		if validateCallCount == 1 {
			return pipeline.Output{
				Decision:           pipeline.Block,
				ValidationFailures: []string{"test: something failed"},
			}, nil
		}
		// Second validate succeeds
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     orchProceedStage(),
		Build:    build,
		Validate: validate,
		Epilogue: orchProceedStage(),
		// 3 beads: bead-1 fails validate, bead-2 succeeds, bead-3 should have no failures
		GetBead: orchBeadQueue(orchTestBead("bead-1"), orchTestBead("bead-2"), orchTestBead("bead-3")),
		Config:  &config.Config{},
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if len(buildInputs) < 3 {
		t.Fatalf("Expected Build to be called at least 3 times, got %d", len(buildInputs))
	}
	// bead-3's build should have no validation failures (cleared after bead-2 succeeded)
	if len(buildInputs[2].ValidationFailures) > 0 {
		t.Errorf("Expected no ValidationFailures for bead-3 after successful validate, got: %v", buildInputs[2].ValidationFailures)
	}
}

// TestOrchestrator_GlobalStatsMergedNotOverwritten verifies that Run merges the
// global stats file rather than overwriting existing data from zero.
func TestOrchestrator_GlobalStatsMergedNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")
	logsDir := t.TempDir() // empty — no JSONL files

	// Write pre-existing global stats with one model entry.
	existing := &logger.GlobalStats{
		Version: 1,
		Updated: "2026-01-01T00:00:00Z",
		Models: map[string]*logger.GlobalModelStats{
			"existing-model": {Iterations: 5, Successes: 3, Failures: 2},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	runID := "test-run-abc"
	o := NewOrchestrator(OrchestratorConfig{
		Gate:            orchProceedStage(),
		Build:           orchProceedStage(),
		Validate:        orchProceedStage(),
		Epilogue:        orchProceedStage(),
		GetBead:         func(ctx context.Context) (*bead.Bead, error) { return nil, nil },
		Config:          &config.Config{},
		GlobalStatsPath: statsPath,
		GetRunID:        func() string { return runID },
		LogsDir:         logsDir,
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Read the stats back and verify the existing model was preserved.
	result, err := logger.ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}
	if result.Models["existing-model"] == nil {
		t.Error("Expected existing-model to be preserved after merge, but it was removed")
	}
	if result.Models["existing-model"].Iterations != 5 {
		t.Errorf("Expected existing-model Iterations=5, got %d", result.Models["existing-model"].Iterations)
	}
}

// TestOrchestrator_StopsOnContextCancellation verifies that Run returns when the
// context is cancelled.
func TestOrchestrator_StopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     orchProceedStage(),
		Build:    orchProceedStage(),
		Validate: orchProceedStage(),
		Epilogue: orchProceedStage(),
		// This would loop forever if context is not respected
		GetBead: func(_ context.Context) (*bead.Bead, error) {
			return orchTestBead("infinite-bead"), nil
		},
		Config: &config.Config{},
	})

	err := o.Run(ctx, 0, time.Time{}, nil)
	if err == nil {
		t.Error("Expected Run() to return an error when context is cancelled")
	}
}

// TestOrchestrator_ReviewStageSkippedWhenDisabled verifies that the Review stage
// is not called when cfg.Review.Enabled is false.
func TestOrchestrator_ReviewStageSkippedWhenDisabled(t *testing.T) {
	reviewCalled := false
	review := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		reviewCalled = true
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     orchProceedStage(),
		Build:    orchProceedStage(),
		Validate: orchProceedStage(),
		Review:   review,
		Epilogue: orchProceedStage(),
		GetBead:  orchBeadQueue(orchTestBead("bead-1")),
		Config:   &config.Config{Review: config.ReviewConfig{Enabled: false}},
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if reviewCalled {
		t.Error("Expected Review stage NOT to be called when cfg.Review.Enabled is false")
	}
}

// TestOrchestrator_EpilogueReceivesBuildSucceededTrue verifies that when build and
// validate pass, Epilogue is called with BuildSucceeded=true.
func TestOrchestrator_EpilogueReceivesBuildSucceededTrue(t *testing.T) {
	var epilogueInputs []pipeline.Input
	epilogue := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		epilogueInputs = append(epilogueInputs, in)
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     orchProceedStage(),
		Build:    orchProceedStage(),
		Validate: orchProceedStage(),
		Epilogue: epilogue,
		GetBead:  orchBeadQueue(orchTestBead("bead-1")),
		Config:   &config.Config{},
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if len(epilogueInputs) == 0 {
		t.Fatal("Expected Epilogue to be called at least once")
	}
	if !epilogueInputs[len(epilogueInputs)-1].BuildSucceeded {
		t.Error("Expected Epilogue to receive BuildSucceeded=true on success path")
	}
}

// TestOrchestrator_EpilogueReceivesBuildSucceededFalseOnGateBlock verifies that when
// Gate blocks, Epilogue is called with BuildSucceeded=false.
func TestOrchestrator_EpilogueReceivesBuildSucceededFalseOnGateBlock(t *testing.T) {
	var epilogueInputs []pipeline.Input
	epilogue := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		epilogueInputs = append(epilogueInputs, in)
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}}

	gate := &orchStageFunc{fn: func(_ context.Context, in pipeline.Input) (pipeline.Output, error) {
		return pipeline.Output{Decision: pipeline.Block}, nil
	}}

	o := NewOrchestrator(OrchestratorConfig{
		Gate:     gate,
		Build:    orchProceedStage(),
		Validate: orchProceedStage(),
		Epilogue: epilogue,
		GetBead:  orchBeadQueue(orchTestBead("bead-1")),
		Config:   &config.Config{},
	})

	if err := o.Run(context.Background(), 0, time.Time{}, nil); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if len(epilogueInputs) == 0 {
		t.Fatal("Expected Epilogue to be called when Gate blocks")
	}
	if epilogueInputs[0].BuildSucceeded {
		t.Error("Expected Epilogue to receive BuildSucceeded=false when Gate blocks")
	}
}
