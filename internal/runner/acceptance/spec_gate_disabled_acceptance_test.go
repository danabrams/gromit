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
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/runner"
)

// TestEpilogue_SpecGateTriggerDisabledInSpecMode verifies that the old epilogue
// spec-gate auto-trigger is not executed when spec-level methodology is active.
// The new merge pipeline should be the only completion mechanism for spec work.
func TestEpilogue_SpecGateTriggerDisabledInSpecMode(t *testing.T) {
	t.Parallel()

	// Track whether specgate.Run was called
	var specGateRunCalled bool
	mockSpecGate := &mockSpecGateRunner{
		RunFn: func(ctx context.Context, beadID string, labels []string) error {
			specGateRunCalled = true
			return nil
		},
	}

	// Create epilogue with spec gate configured
	epilogueStage := epilogue.New(
		&mockBeadLifecycle{},
		&mockStatusWriter{},
		io.Discard,
	)
	epilogueStage.WithSpecGate(mockSpecGate)

	// Create input for a successful spec bead with spec-level methodology
	trueVal := true
	input := pipeline.Input{
		BuildSucceeded: true,
		Bead: &bead.Bead{
			ID:    "test-bead-1",
			Title: "Test Bead",
			Labels: []string{
				"spec:auth",
			},
		},
		Config: &config.Config{
			Methodology: config.MethodologyConfig{
				Granularity: config.MethodologyGranularitySpec,
			},
			SpecGate: config.SpecGateConfig{
				Enabled:     &trueVal,
				AutoTrigger: &trueVal,
				// Note: AutoTrigger should no longer be checked in the new pipeline model
			},
		},
	}

	// Run epilogue
	_, err := epilogueStage.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("epilogue.Run failed: %v", err)
	}

	// Verify that specgate.Run was NOT called
	// This test will fail with current code, which is expected for RED phase
	if specGateRunCalled {
		t.Error("specgate.Run() should not be called when spec methodology is active - " +
			"merge pipeline should be the only completion mechanism")
	}
}

// mockSpecGateRunner implements epilogue.SpecGateRunner for testing
type mockSpecGateRunner struct {
	RunFn func(ctx context.Context, beadID string, labels []string) error
}

func (m *mockSpecGateRunner) Run(ctx context.Context, beadID string, labels []string) error {
	if m.RunFn != nil {
		return m.RunFn(ctx, beadID, labels)
	}
	return nil
}

// mockBeadLifecycle implements epilogue.BeadLifecycle for testing
type mockBeadLifecycle struct {
	CloseFn func(id string) error
	SyncFn  func() error
}

func (m *mockBeadLifecycle) Close(id string) error {
	if m.CloseFn != nil {
		return m.CloseFn(id)
	}
	return nil
}

func (m *mockBeadLifecycle) Sync() error {
	if m.SyncFn != nil {
		return m.SyncFn()
	}
	return nil
}

// mockStatusWriter implements epilogue.StatusWriter for testing
type mockStatusWriter struct {
	WriteFn func(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error
}

func (m *mockStatusWriter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	if m.WriteFn != nil {
		return m.WriteFn(iteration, beadID, beadTitle, model, maxIterations, timeBudgetMinutes)
	}
	return nil
}

// TestOrchestratorSpecModeWithMixedQueue verifies correct behavior with mixed
// spec and non-spec beads when spec-level methodology is active.
// Spec beads should complete via merge pipeline, not legacy epilogue spec gate.
func TestOrchestratorSpecModeWithMixedQueue(t *testing.T) {
	t.Parallel()

	trueVal := true
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
		SpecGate: config.SpecGateConfig{
			Enabled:     &trueVal,
			AutoTrigger: &trueVal,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Track which beads were executed
	var executedBeadIDs []string
	closedBeadIDs := make(map[string]bool)

	allBeads := []*bead.Bead{
		{ID: "spec-1", Title: "Spec task 1", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "regular-1", Title: "Regular task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
		{ID: "spec-2", Title: "Spec task 2", Priority: 0, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			for _, b := range allBeads {
				if !closedBeadIDs[b.ID] && (len(b.Labels) == 0 || b.Labels[0] != "spec:auth") {
					return b, nil
				}
			}
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			for _, b := range allBeads {
				if !closedBeadIDs[b.ID] {
					for _, l := range b.Labels {
						if l == label {
							return b, nil
						}
					}
				}
			}
			return nil, nil
		},
		CloseFn: func(id string) error {
			closedBeadIDs[id] = true
			return nil
		},
	}

	// Create test stage that tracks execution and closes beads
	executingStage := &testStage{
		fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			executedBeadIDs = append(executedBeadIDs, in.Bead.ID)
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		},
	}

	h := &OrchestratorTestHelper{}
	h.orchestrator = runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate:     &noopStage{},
		Build:    executingStage,
		Validate: &noopStage{},
		Epilogue: &testEpilogueStage{
			fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
				// Close the bead to allow the queue to progress
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

	ctx := context.Background()
	err := h.Run(ctx, 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify all beads were executed
	if len(executedBeadIDs) != 3 {
		t.Errorf("Expected 3 beads to be executed, got %d: %v", len(executedBeadIDs), executedBeadIDs)
	}
}

// testStage is a simple pipeline stage for testing
type testStage struct {
	fn func(ctx context.Context, in pipeline.Input) (pipeline.Output, error)
}

func (s *testStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if s.fn != nil {
		return s.fn(ctx, in)
	}
	return pipeline.Output{Decision: pipeline.Proceed}, nil
}

// testEpilogueStage is an epilogue stage for testing
type testEpilogueStage struct {
	fn func(ctx context.Context, in pipeline.Input) (pipeline.Output, error)
}

func (s *testEpilogueStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if s.fn != nil {
		return s.fn(ctx, in)
	}
	return pipeline.Output{Decision: pipeline.Proceed}, nil
}
