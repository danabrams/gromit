package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline/execute"
)

// TDDPipelineAdapter bridges the runner's TDD orchestration to the pipeline's
// TDDCycleRunner interface.
type TDDPipelineAdapter struct {
	runner *Runner
}

// RunCycles implements execute.TDDCycleRunner.
func (a *TDDPipelineAdapter) RunCycles(_ context.Context, _ *bead.Bead, _ *config.Config) (execute.TDDCycleResult, error) {
	return execute.TDDCycleResult{}, nil
}
