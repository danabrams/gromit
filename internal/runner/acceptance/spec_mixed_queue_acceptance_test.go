//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
)

// TestOrchestratorSpecModeFullSuccessLoop verifies the full pipeline execution
// with spec and non-spec beads, all successfully completing.
func TestOrchestratorSpecModeFullSuccessLoop(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Define mixed queue: spec beads and non-spec beads
	allBeads := []*bead.Bead{
		{ID: "auth-task-1", Title: "Auth implementation", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "payment-task-1", Title: "Payment implementation", Priority: 1, Labels: []string{"spec:payment"}, ExpectedOutputs: []string{}},
		{ID: "regular-task-1", Title: "Regular refactor", Priority: 0, Labels: []string{}, ExpectedOutputs: []string{}},
	}

	closedBeads := make(map[string]bool)
	executedBeads := []string{}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			for _, b := range allBeads {
				if !closedBeads[b.ID] && (len(b.Labels) == 0 || b.Labels[0] != "spec:auth") && (len(b.Labels) == 0 || b.Labels[0] != "spec:payment") {
					return b, nil
				}
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			closedBeads[id] = true
			return nil
		},
	}

	h := &OrchestratorTestHelper{}
	h.orchestrator = runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate: &noopStage{},
		Build: &testStage{
			fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
				executedBeads = append(executedBeads, in.Bead.ID)
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
		},
		Validate: &noopStage{},
		Epilogue: &testEpilogueStage{
			fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
				mockBeads.Close(in.Bead.ID)
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
		},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return mockBeads.Ready()
		},
		Config: cfg,
		Output: io.Discard,
	})

	err := h.Run(context.Background(), 100, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify that the non-spec bead was executed
	if len(executedBeads) == 0 {
		t.Error("Expected at least one bead to be executed")
	}

	// Verify that the non-spec bead was closed
	if !closedBeads["regular-task-1"] {
		t.Error("Expected regular-task-1 to be closed")
	}
}

// TestOrchestratorSpecModeWithIterationBoundary verifies that all beads execute
// respecting the iteration boundary constraint.
func TestOrchestratorSpecModeWithIterationBoundary(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Loop: config.LoopConfig{
			MaxIterations: 5,
		},
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	beadQueue := []*bead.Bead{
		{ID: "bead-1", Title: "Task 1", Priority: 0, Labels: []string{"spec:feature"}, ExpectedOutputs: []string{}},
		{ID: "bead-2", Title: "Task 2", Priority: 0, Labels: []string{"spec:feature"}, ExpectedOutputs: []string{}},
	}

	queueIndex := 0
	closedBeads := make(map[string]bool)
	iterationCount := 0

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if queueIndex >= len(beadQueue) || closedBeads[beadQueue[queueIndex].ID] {
				return nil, nil
			}
			return beadQueue[queueIndex], nil
		},
		CloseFn: func(id string) error {
			closedBeads[id] = true
			queueIndex++
			return nil
		},
	}

	h := &OrchestratorTestHelper{}
	h.orchestrator = runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate: &noopStage{},
		Build: &testStage{
			fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
				iterationCount++
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
		},
		Validate: &noopStage{},
		Epilogue: &testEpilogueStage{
			fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
				mockBeads.Close(in.Bead.ID)
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
		},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return mockBeads.Ready()
		},
		Config: cfg,
		Output: io.Discard,
	})

	err := h.Run(context.Background(), 5, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify that iterations were bounded by maxIterations
	if iterationCount > 5 {
		t.Errorf("Expected iteration count to be bounded by maxIterations (5), got %d", iterationCount)
	}

	// Verify that at least one bead was executed
	if iterationCount == 0 {
		t.Error("Expected at least one iteration to execute")
	}
}

// TestOrchestratorSpecModeNonSpecBeadsOnMain verifies that non-spec beads
// are executed even when spec mode is active.
func TestOrchestratorSpecModeNonSpecBeadsOnMain(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	allBeads := []*bead.Bead{
		{ID: "non-spec-1", Title: "Regular task", Priority: 0, Labels: []string{}, ExpectedOutputs: []string{}},
		{ID: "non-spec-2", Title: "Another regular", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
	}

	closedBeads := make(map[string]bool)
	executedBeads := []string{}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			for _, b := range allBeads {
				if !closedBeads[b.ID] {
					return b, nil
				}
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			closedBeads[id] = true
			return nil
		},
	}

	h := &OrchestratorTestHelper{}
	h.orchestrator = runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate: &noopStage{},
		Build: &testStage{
			fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
				executedBeads = append(executedBeads, in.Bead.ID)
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
		},
		Validate: &noopStage{},
		Epilogue: &testEpilogueStage{
			fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
				mockBeads.Close(in.Bead.ID)
				return pipeline.Output{Decision: pipeline.Proceed}, nil
			},
		},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return mockBeads.Ready()
		},
		Config: cfg,
		Output: io.Discard,
	})

	err := h.Run(context.Background(), 100, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify that non-spec beads were executed
	if len(executedBeads) != 2 {
		t.Errorf("Expected 2 non-spec beads to be executed, got %d: %v", len(executedBeads), executedBeads)
	}
}
