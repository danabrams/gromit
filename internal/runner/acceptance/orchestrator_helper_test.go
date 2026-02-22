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
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner"
)

// OrchestratorTestHelper wraps Orchestrator with test-friendly methods,
// providing a foundation for acceptance tests to migrate from NewRunnerWithDeps
// to Orchestrator.
type OrchestratorTestHelper struct {
	orchestrator *runner.Orchestrator
	labelFilters []string
}

// NewOrchestratorTestHelper creates an OrchestratorTestHelper using runner.NewRunner.
func NewOrchestratorTestHelper(t *testing.T, cfg *config.Config, output io.Writer) *OrchestratorTestHelper {
	t.Helper()
	orch, err := runner.NewRunner(cfg, output)
	if err != nil {
		t.Fatalf("NewOrchestratorTestHelper: NewRunner failed: %v", err)
	}
	return &OrchestratorTestHelper{orchestrator: orch}
}

// NewOrchestratorTestHelperWithDeps creates an OrchestratorTestHelper with
// injected mock BeadClient and Router. The GetBead function respects any label
// filters set via SetLabelFilters.
func NewOrchestratorTestHelperWithDeps(t *testing.T, cfg *config.Config, output io.Writer, beads runner.BeadClient, router *provider.Router) *OrchestratorTestHelper {
	t.Helper()
	h := &OrchestratorTestHelper{}

	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if len(h.labelFilters) > 0 {
			for _, label := range h.labelFilters {
				b, err := beads.ReadyWithLabel(label)
				if err != nil {
					return nil, err
				}
				if b != nil {
					return b, nil
				}
			}
			return nil, nil
		}
		return beads.Ready()
	}

	orch := runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate:     &noopStage{},
		Build:    &noopStage{},
		Validate: &noopStage{},
		Epilogue: &noopStage{},
		GetBead:  getBead,
		Config:   cfg,
		Output:   output,
	})
	h.orchestrator = orch
	return h
}

// SetLabelFilters configures label filters that restrict which beads GetBead returns.
// When filters are set, ReadyWithLabel is used instead of Ready for each label in order.
func (h *OrchestratorTestHelper) SetLabelFilters(labels []string) {
	h.labelFilters = labels
}

// Run executes the Orchestrator pipeline loop, delegating to Orchestrator.Run.
func (h *OrchestratorTestHelper) Run(ctx context.Context, maxIterations int, deadline time.Time, stopCh <-chan struct{}) error {
	return h.orchestrator.Run(ctx, maxIterations, deadline, stopCh)
}

// noopStage is a pipeline.Stage that always returns Proceed without doing anything.
type noopStage struct{}

func (n *noopStage) Run(_ context.Context, _ pipeline.Input) (pipeline.Output, error) {
	return pipeline.Output{Decision: pipeline.Proceed}, nil
}

// Compile-time check: *noopStage must implement pipeline.Stage.
var _ pipeline.Stage = (*noopStage)(nil)
