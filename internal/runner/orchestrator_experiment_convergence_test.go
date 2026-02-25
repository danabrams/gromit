package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/experiment"
)

// TestOrchestrator_EmitsConvergenceSummaryToStderr verifies that when an experiment
// has been loaded, the orchestrator emits a summary line to stderr after starting the run.
func TestOrchestrator_EmitsConvergenceSummaryToStderr(t *testing.T) {
	// Create experiments
	exp := &experiment.Experiment{
		ID:    "exp-1",
		Phase: "build",
	}

	// Create an experiment manager with one experiment
	expMgr := experiment.NewManager([]*experiment.Experiment{exp}, "")

	// Capture stderr output
	var output bytes.Buffer

	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:          &fakeStage{},
		Build:         &fakeStage{},
		Validate:      &fakeStage{},
		Epilogue:      &fakeStage{},
		GetBead:       getBead,
		Config:        &config.Config{},
		ExperimentMgr: expMgr,
		Output:        &output,
	}

	orchestrator := NewOrchestrator(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := orchestrator.Run(ctx, 0, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() should not error: %v", err)
	}

	// Check that convergence summary was written to stderr
	outputStr := output.String()
	if !strings.Contains(outputStr, "Experiment") || !strings.Contains(outputStr, "converged") {
		t.Fatalf("Expected stderr to contain convergence summary, got: %q", outputStr)
	}
}
